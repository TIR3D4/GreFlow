package health

import (
	"github.com/TIR3D4/GreFlow/core/config"
	"testing"
)

func TestTranslatedConntrackTuples(t *testing.T) {
	c := config.Config{Role: "entry", Local: "192.0.2.1", Network: "10.77.0.0/30", Forwards: []config.Forward{{Protocol: "tcp", Public: "443", Destination: "2020"}, {Protocol: "udp", Public: "12000-12002", Destination: "12000-12002"}}}
	output := `tcp 6 100 ESTABLISHED src=192.0.2.3 dst=192.0.2.1 sport=50000 dport=443 src=10.77.0.2 dst=10.77.0.1 sport=2020 dport=50000 [ASSURED]
udp 17 30 src=192.0.2.3 dst=192.0.2.1 sport=50001 dport=12001 src=10.77.0.2 dst=10.77.0.1 sport=12001 dport=50001
 tcp 6 100 src=10.77.0.1 dst=10.77.0.2 sport=50002 dport=2020 src=10.77.0.2 dst=10.77.0.1 sport=2020 dport=50002
 tcp 6 100 src=192.0.2.3 dst=192.0.2.1 sport=50003 dport=443 src=10.99.0.2 dst=10.77.0.1 sport=2020 dport=50003`
	got := CountForwarded(output, c)
	if got["tcp:443"] != 1 || got["udp:12000-12002"] != 1 || len(got) != 2 {
		t.Fatal(got)
	}
	c.Role = "exit"
	if len(CountForwarded(output, c)) != 0 {
		t.Fatal("exit has no NAT forwards")
	}
}
