package dockerhosts

import (
	"testing"

	"github.com/coredns/caddy"
)

func TestSetup(t *testing.T) {
	c := caddy.NewTestController("dns", `dockerhosts`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected errors, but got nil")
	}

	c = caddy.NewTestController("dns", `dockerhosts tcp://172.17.0.2:2375 tcp://172.17.0.1:2375`)
	if err := setup(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}
}
