package dockerhosts

import (
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
	c.Next() // 'dockerhosts'
	dockerHosts := c.RemainingArgs()
	fmt.Printf("Remaining args %v\n", dockerHosts)
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		dp := &CoreDNSDockerPlugin{
			Next:             next, // Set the Next field, so the plugin chaining works.
			clients:          []*client.Client{},
			containerToIPMap: map[string]string{},
			network:          "overlay",
			errorCount:       0,
		}
		err := dp.initClients(dockerHosts)
		if err != nil {
			panic(err)
		}
		go dp.updateContainers()
		return dp
	})

	return nil
}