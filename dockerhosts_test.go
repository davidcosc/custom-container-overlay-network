package dockerhosts

import (
	"context"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/docker/docker/client"
	"github.com/miekg/dns"
)

func TestDockerhosts(t *testing.T) {
	dp := &CoreDNSDockerPlugin{
		clients:          []*client.Client{},
		containerToIPMap: map[string]string{},
		network:          "overlay",
		errorCount:       0,
	}
	err := dp.initClients([]string{"tcp://172.17.0.2:2375", "tcp://172.17.0.1:2375"})
	if err != nil {
		panic(err)
	}
	go dp.updateContainers()
	if dp.Name() != name {
		t.Errorf("expected plugin name: %s, got %s", dp.Name(), name)
	}
	time.Sleep(5 * time.Second) // Wait for updateContainers to query once.
	tests := []struct {
		qname         string
		qtype         uint16
		remote        string
		expectedCode  int
		expectedReply []string // ownernames for the records in the additional section.
	}{
		{
			qname:        "unknown_uakari",
			qtype:        dns.TypeA,
			expectedCode: 2,
		},
		// Case insensitive and case preserving
		{
			qname:         "cadvisorB",
			qtype:         dns.TypeA,
			expectedCode:  dns.RcodeSuccess,
			expectedReply: []string{"cadvisorB."},
		},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion(dns.Fqdn(tc.qname), tc.qtype)
		rec := dnstest.NewRecorder(&test.ResponseWriter{RemoteIP: tc.remote})
		code, _ := dp.ServeDNS(ctx, rec, req)
		if code != tc.expectedCode {
			t.Errorf("Test %d: Expected status code %d, but got %d", i, tc.expectedCode, code)
		}
		if len(tc.expectedReply) != 0 {
			if len(tc.expectedReply) != len(rec.Msg.Answer) {
				t.Errorf("Test %d: Unexpected number of answers", i)
				continue
			}
			for i, expected := range tc.expectedReply {
				actual := rec.Msg.Answer[i].String()
				actual = actual[:len(actual)-18]
				if actual != expected {
					t.Errorf("Test %d: Expected answer %s, but got %s", i, expected, actual)
				}
			}
		}
	}
}