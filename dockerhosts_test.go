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
		clients:        []*client.Client{},
		containerIPMap: map[string]IPInfo{},
		network:        "overlay",
	}
	if err := dp.initClients([]string{"tcp://172.17.0.2:2375", "tcp://172.17.0.1:2375"}); err != nil {
		t.Errorf("Init clients error %v", err)
	}
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
		wait          time.Duration
	}{
		{
			qname:        "unknown_uakari",
			qtype:        dns.TypeA,
			expectedCode: 2,
			wait:         0 * time.Second,
		},
		{
			qname:         "cadvisorB",
			qtype:         dns.TypeA,
			expectedCode:  dns.RcodeSuccess,
			expectedReply: []string{"cadvisorB."},
			wait:          0 * time.Second,
		},
		{
			qname:        "blubb.example.com",
			qtype:        dns.TypeA,
			expectedCode: 2,
			wait:         0 * time.Second,
		},
		{
			qname:         "cadvisorB",
			qtype:         dns.TypeA,
			expectedCode:  dns.RcodeSuccess,
			expectedReply: []string{"cadvisorB."},
			wait:          70 * time.Second,
		},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		if tc.wait > 0*time.Second {
			time.Sleep(tc.wait)
		}
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
