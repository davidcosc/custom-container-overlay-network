package dockerhosts

import (
	"errors"
	"fmt"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/docker/docker/client"
)

func init() {
	plugin.Register("dockerhosts", setup)
}

func setup(c *caddy.Controller) error {
	fmt.Printf("Setting up dockerhosts pugin.\n")
	c.Next() // 'dockerhosts'
	hosts := c.RemainingArgs()
	if len(hosts) < 1 {
		return errors.New("Expected at least 1 docker host, but got 0!")
	}
	dp := &CoreDNSDockerPlugin{
		clients:        []*client.Client{},
		containerIPMap: map[string]IPInfo{},
		network:        "overlay",
	}
	if err := dp.initClients(hosts); err != nil {
		return err
	}
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		dp.Next = next // Set the Next field, so the plugin chaining works.
		return dp
	})
	return nil
}
