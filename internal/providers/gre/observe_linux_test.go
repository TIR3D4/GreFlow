package gre

import (
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"os"
	"strings"
	"testing"
	"time"
)

type readOnlyRunner struct{ calls []string }

func (r *readOnlyRunner) Run(name string, args ...string) (string, error) {
	s := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, s)
	if name == "systemctl" {
		return "active\nexited\n", nil
	}
	if name == "ip" {
		return `[{"ifalias":"greflow:v0.1","flags":["UP"],"stats64":{"rx":{"bytes":10},"tx":{"bytes":20}}}]`, nil
	}
	if name == "iptables" {
		return "", nil
	}
	return "", fmt.Errorf("unexpected command")
}

func TestClientVerificationNeverRefreshesOnDiscovery(t *testing.T) {
	m := manager.Manager{Root: t.TempDir()}
	old := time.Now().UTC().Add(-time.Hour)
	v := verification{Revision: "revision", Generation: "generation", ObservedAt: old}
	if e := config.Save(verificationPath(m), v); e != nil {
		t.Fatal(e)
	}
	tunnel := protocol.Tunnel{Revision: v.Revision, Generation: v.Generation}
	first := verificationEvidence(m, tunnel, true)
	second := verificationEvidence(m, tunnel, true)
	if !first.ObservedAt.Equal(old) || !second.ObservedAt.Equal(old) || !first.ExpiresAt.Before(time.Now()) {
		t.Fatal(first, second)
	}
	tunnel.Generation = "new"
	if verificationEvidence(m, tunnel, true).Status != "unknown" {
		t.Fatal("old generation accepted")
	}
}
func TestStructuredDiscoveryNeverMutatesAndKeepsLayersIndependent(t *testing.T) {
	r := &readOnlyRunner{}
	m := manager.Manager{Root: t.TempDir(), R: r}
	c := config.Config{Role: "entry", Local: "192.0.2.1", Remote: "192.0.2.2", Interface: "greflow0", Network: "10.77.0.0/30", MTU: 1476}
	if e := config.Save(m.ConfigPath(), c); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(m.Path("/etc/machine-id"), []byte("isolated-test"), 0600); e != nil {
		t.Fatal(e)
	}
	i, e := Discover(m)
	if e != nil {
		t.Fatal(e)
	}
	if len(i.Tunnels) != 1 {
		t.Fatal(i)
	}
	checks := map[string]string{}
	for _, v := range i.Tunnels[0].Checks {
		checks[v.Kind] = v.Status
	}
	if checks["service"] != "ok" || checks["gre_peer"] != "unknown" || checks["user_verified"] != "unknown" {
		t.Fatal(checks)
	}
	for _, call := range r.calls {
		if strings.Contains(call, " -A ") || strings.Contains(call, " -D ") || strings.Contains(call, " restart ") || strings.Contains(call, " apply ") {
			t.Fatal(call)
		}
	}
}
