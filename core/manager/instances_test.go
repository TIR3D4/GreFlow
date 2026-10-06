package manager

import (
	"github.com/TIR3D4/GreFlow/core/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func baseConfig() config.Config {
	return config.Config{Role: "entry", Local: "192.0.2.1", Remote: "192.0.2.2", Interface: "greflow0", Network: "10.77.0.0/30", MTU: 1476, Forwards: []config.Forward{{Protocol: "tcp", Public: "443", Destination: "443"}}}
}
func TestInstanceConflicts(t *testing.T) {
	base := Manager{Root: t.TempDir()}
	first := baseConfig()
	if e := config.Save(base.ConfigPath(), first); e != nil {
		t.Fatal(e)
	}
	next := base
	next.Name = "exit2"
	c := baseConfig()
	c.Remote = "192.0.2.4"
	c.Interface = "gf-exit2"
	c.Network = "10.77.0.4/30"
	c.Forwards[0].Public = "3212"
	if e := next.ValidatePeers(c); e != nil {
		t.Fatal(e)
	}
	tests := []struct {
		name string
		edit func(*config.Config)
	}{{"interface", func(c *config.Config) { c.Interface = "greflow0" }}, {"network", func(c *config.Config) { c.Network = "10.77.0.0/30" }}, {"endpoint", func(c *config.Config) { c.Remote = "192.0.2.2" }}, {"public port", func(c *config.Config) {
		c.Forwards = []config.Forward{{Protocol: "tcp", Public: "440-445", Destination: "440-445"}}
	}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := c
			tt.edit(&v)
			if next.ValidatePeers(v) == nil {
				t.Fatal("conflict accepted")
			}
		})
	}
}
func TestPathsAndUnitsPreserveLegacy(t *testing.T) {
	m := Manager{}
	if m.Service() != "greflow.service" || m.ConfigPath() != "/etc/greflow/config.json" || m.UnitText() != Unit {
		t.Fatal("legacy changed")
	}
	m.Name = "exit2"
	if m.Service() != "greflow-exit2.service" || !strings.Contains(m.ConfigPath(), "tunnels/exit2/config.json") || !strings.Contains(m.UnitText(), "greflow --tunnel exit2 apply") {
		t.Fatal(m.UnitText())
	}
	if strings.Contains(m.UnitText(), "ExecStart=/usr/local/bin/greflow apply") {
		t.Fatal("unscoped unit")
	}
}
func TestSharedKernelBaselineAndUninstallIsolation(t *testing.T) {
	root := t.TempDir()
	r := &runner{values: map[string]string{"net.ipv4.ip_forward": "1", "net.ipv4.conf.all.rp_filter": "0"}}
	first := Manager{Root: root, R: r}
	c := baseConfig()
	original := map[string]string{"net.ipv4.ip_forward": "0", "net.ipv4.conf.all.rp_filter": "2"}
	written := map[string]string{"net.ipv4.ip_forward": "1", "net.ipv4.conf.all.rp_filter": "0"}
	state := State{Config: &c, Active: true, Original: original, Written: written}
	if e := config.Save(first.ConfigPath(), c); e != nil {
		t.Fatal(e)
	}
	if e := config.Save(first.StatePath(), state); e != nil {
		t.Fatal(e)
	}
	second := first
	second.Name = "exit2"
	s := State{Original: map[string]string{}}
	if e := second.inheritOriginal(&s); e != nil {
		t.Fatal(e)
	}
	if s.Original["net.ipv4.ip_forward"] != "0" {
		t.Fatal(s)
	}
	c2 := c
	c2.Interface = "gf-exit2"
	c2.Network = "10.77.0.4/30"
	state2 := State{Config: &c2, Active: true, Original: s.Original, Written: written}
	_ = config.Save(second.ConfigPath(), c2)
	_ = config.Save(second.StatePath(), state2)
	if e := first.restore(state); e != nil {
		t.Fatal(e)
	}
	if r.values["net.ipv4.ip_forward"] != "1" {
		t.Fatal("disabled sibling forwarding")
	}
	state.Active = false
	_ = config.Save(first.StatePath(), state)
	if e := second.restore(state2); e != nil {
		t.Fatal(e)
	}
	if r.values["net.ipv4.ip_forward"] != "0" || r.values["net.ipv4.conf.all.rp_filter"] != "2" {
		t.Fatal(r.values)
	}
	// Removing an inactive default config must leave a named config and all backups.
	state.Config = nil
	_ = config.Save(first.StatePath(), state)
	backup := filepath.Join(root, "var/lib/greflow/backups/keep")
	_ = os.MkdirAll(backup, 0700)
	if e := first.Uninstall(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(second.ConfigPath()); e != nil {
		t.Fatal("sibling config removed", e)
	}
	if _, e := os.Stat(backup); e != nil {
		t.Fatal(e)
	}
}
