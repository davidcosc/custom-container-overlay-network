package dockerhosts

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/miekg/dns"
)

const name = "dockerhosts"

type DockerEventsMessage struct {
	client  *client.Client
	message events.Message
}

type DockerEventsError struct {
	client *client.Client
	err    error
}

type CoreDNSDockerPlugin struct {
	Next             plugin.Handler
	clients          []*client.Client
	network          string
	containerToIPMap map[string]string
	mutex            sync.RWMutex
	errorCount       int
}

func (dp *CoreDNSDockerPlugin) initClients(dockerHosts []string) error {
	if len(dockerHosts) == 0 {
		return errors.New("Expected at least 1 docker host, but got 0!")
	}
	for _, dockerHost := range dockerHosts {
		dockerClient, err := client.NewClientWithOpts(client.WithHost(dockerHost))
		if err != nil {
			return err
		}
		dp.clients = append(dp.clients, dockerClient)
	}
	return nil
}

func (dp *CoreDNSDockerPlugin) updateContainers() {
	fmt.Printf("Starting updateContainers iteration %d\n", dp.errorCount)
	defer dp.updateContainers()
	ctx, cancel := context.WithCancelCause(context.Background())
	mergedMsgChan := make(chan DockerEventsMessage)
	mergedErrChan := make(chan DockerEventsError)
	var wg sync.WaitGroup
	for _, cli := range dp.clients {
		wg.Add(1)
		go dp.updateContainersForClient(ctx, cli, mergedMsgChan, mergedErrChan, &wg)
	}
	for {
		select {
		case msg := <-mergedMsgChan:
			inspectInfo, err := msg.client.ContainerInspect(ctx, msg.message.Actor.ID)
			if err != nil {
				fmt.Printf("Client %v inspect message failed with %v\n", msg.client, err)
				break
			}
			if networkSummary := inspectInfo.NetworkSettings; networkSummary != nil {
				if networks := networkSummary.Networks; networks != nil {
					if endpoint, hasKey := networks[dp.network]; hasKey {
						if containerName, hasKey := msg.message.Actor.Attributes["name"]; hasKey {
							dp.mutex.Lock()
							if msg.message.Action == "stop" {
								delete(dp.containerToIPMap, containerName)
							} else {
								dp.containerToIPMap[containerName] = (*endpoint).IPAddress
							}
							dp.mutex.Unlock()
						}
					}
				}
			}
		case err := <-mergedErrChan:
			fmt.Printf("Client %v received error %v\n", err.client, err.err)
			cancel(err.err)
		case <-ctx.Done():
			fmt.Printf("Context was cancelled with %v\n", context.Cause(ctx))
			wg.Wait()
			fmt.Printf("Finished waiting.\n")
			close(mergedMsgChan)
			close(mergedErrChan)
			dp.mutex.Lock()
			dp.containerToIPMap = map[string]string{}
			dp.errorCount += 1
			dp.mutex.Unlock()
			return
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func (dp *CoreDNSDockerPlugin) updateContainersForClient(ctx context.Context, cli *client.Client, outMessageChan chan<- DockerEventsMessage, outErrorChan chan<- DockerEventsError, wg *sync.WaitGroup) {
	defer wg.Done()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		outErrorChan <- DockerEventsError{client: cli, err: err}
		return
	}
	for _, container := range containers {
		networkSummary := *container.NetworkSettings
		networks := networkSummary.Networks
		if endpoint, hasKey := networks[dp.network]; hasKey {
			if containerNames := container.Names; len(containerNames) > 0 {
				dp.mutex.Lock()
				dp.containerToIPMap[containerNames[0][1:]] = (*endpoint).IPAddress
				dp.mutex.Unlock()
			}
		}
	}
	messageChan, errorChan := cli.Events(ctx, types.EventsOptions{
		Filters: filters.NewArgs(filters.Arg("event", "start"), filters.Arg("event", "stop")),
	})
	for {
		select {
		case msg := <-messageChan:
			outMessageChan <- DockerEventsMessage{client: cli, message: msg}
		case err := <-errorChan:
			outErrorChan <- DockerEventsError{client: cli, err: err}
			return
		case <-ctx.Done():
			fmt.Printf("Client %v received context cancelled with %v\n", cli, context.Cause(ctx))
			return
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

// ServeDNS implements the plugin.Handler interface.
func (dp *CoreDNSDockerPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	a := new(dns.Msg)
	a.SetReply(r)
	a.Authoritative = true
	resIp := ""
	for name, ip := range dp.containerToIPMap {
		fmt.Printf("%s %s %s %s\n", name, ip, state.Name(), state.QName())
		requestName := state.Name()
		if requestName[:len(requestName)-1] == strings.ToLower(name) {
			resIp = ip
		}
	}
	fmt.Printf("Res ip: %v\n", resIp)
	if resIp == "" || state.Family() != 1 {
		retCode, err := plugin.NextOrFailure(dp.Name(), dp.Next, ctx, w, r)
		return retCode, err
	}
	var rr dns.RR
	rr = new(dns.A)
	rr.(*dns.A).Hdr = dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: dns.ClassINET}
	rr.(*dns.A).A = net.ParseIP(resIp).To4()
	a.Answer = []dns.RR{rr}
	w.WriteMsg(a)
	return 0, nil
}

// Name implements the Handler interface.
func (dp *CoreDNSDockerPlugin) Name() string { return name }