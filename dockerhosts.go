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
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/miekg/dns"
)

const name = "dockerhosts"

type IPInfo struct {
	ip           string
	creationTime time.Time
}

type CoreDNSDockerPlugin struct {
	Next           plugin.Handler
	clients        []*client.Client
	network        string
	containerIPMap map[string]IPInfo
	mutex          sync.RWMutex
}

func (dp *CoreDNSDockerPlugin) initClients(hosts []string) error {
	if len(hosts) == 0 {
		return errors.New("Expected at least 1 docker host, but got 0!")
	}
	for _, dockerHost := range hosts {
		dockerClient, err := client.NewClientWithOpts(client.WithHost(dockerHost))
		if err != nil {
			return err
		}
		dp.clients = append(dp.clients, dockerClient)
	}
	return nil
}

func (dp *CoreDNSDockerPlugin) getIP(name string) (string, error) {
	name = strings.ToLower(name)
	if ipInfo, hasKey := dp.containerIPMap[name]; hasKey {
		if time.Now().Sub(ipInfo.creationTime) < (60 * time.Second) {
			fmt.Printf("Got ip %s for container %s from cache!\n", ipInfo.ip, name)
			return ipInfo.ip, nil
		}
	}
	resIP := ""
	for _, cli := range dp.clients {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
		cancel()
		if err != nil {
			return "", err
		}
		for _, container := range containers {
			var networkSummary *types.SummaryNetworkSettings
			if networkSummary = container.NetworkSettings; networkSummary != nil {
				var networks map[string]*network.EndpointSettings
				if networks = networkSummary.Networks; networks != nil {
					if endpoint, hasKey := networks[dp.network]; hasKey {
						if containerNames := container.Names; len(containerNames) > 0 {
							dp.mutex.Lock()
							fmt.Printf("Updating ip info for container %s\n", strings.ToLower(containerNames[0][1:]))
							dp.containerIPMap[strings.ToLower(containerNames[0][1:])] = IPInfo{(*endpoint).IPAddress, time.Now()}
							if strings.ToLower(containerNames[0][1:]) == name {
								resIP = dp.containerIPMap[name].ip
								fmt.Printf("Got ip %s for container %s from reqeust!\n", resIP, name)
							}
							dp.mutex.Unlock()
						}
					}
				}
			}
		}
	}
	return resIP, nil
}

// ServeDNS implements the plugin.Handler interface.
func (dp *CoreDNSDockerPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	fmt.Printf("Dockerhosts plugin received DNS query %v\n", r)
	state := request.Request{W: w, Req: r}
	a := new(dns.Msg)
	a.SetReply(r)
	a.Authoritative = true
	requestName := state.Name()
	if strings.Count(requestName, ".") > 1 {
		fmt.Printf("Dockerhosts plugin received a DNS question for an actual domain, but not a simple container name.\n")
		retCode, err := plugin.NextOrFailure(dp.Name(), dp.Next, ctx, w, r)
		return retCode, err
	}
	resIp, warn := dp.getIP(requestName[:len(requestName)-1])
	fmt.Printf("%s %s %s %s %v\n", name, resIp, state.Name(), state.QName(), warn)
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
