package config

import (
	"os"
	"path/filepath"
	"testing"
)

func valid() Config {
	return Config{Role: "entry", Local: "192.0.2.1", Remote: "192.0.2.2", Interface: "greflow0", Network: "10.77.0.0/30", MTU: 1476}
}
func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{{"role", func(c *Config) { c.Role = "other" }}, {"injection", func(c *Config) { c.Interface = "gre;rm" }}, {"ipv6", func(c *Config) { c.Local = "::1" }}, {"public network", func(c *Config) { c.Network = "15.0.0.0/30" }}, {"alignment", func(c *Config) { c.Network = "10.77.0.1/30" }}, {"mtu", func(c *Config) { c.MTU = 9000 }}, {"overlap", func(c *Config) { c.Forwards = []Forward{{"tcp", "443", "2020"}, {"tcp", "440-450", "440-450"}} }}, {"offset range", func(c *Config) { c.Forwards = []Forward{{"udp", "100-110", "200-210"}} }}, {"bad protocol", func(c *Config) { c.Forwards = []Forward{{"gre", "80", "80"}} }}}
	if e := valid().Validate(); e != nil {
		t.Fatal(e)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.edit(&c)
			if c.Validate() == nil {
				t.Fatal("accepted invalid configuration")
			}
		})
	}
}
func TestAddressesAndRanges(t *testing.T) {
	c := valid()
	a, b := c.Addresses()
	if a != "10.77.0.1" || b != "10.77.0.2" {
		t.Fatal(a, b)
	}
	c.Role = "exit"
	a, b = c.Addresses()
	if a != "10.77.0.2" || b != "10.77.0.1" {
		t.Fatal(a, b)
	}
	for _, s := range []string{"0", "65536", "20-10", "1-2-3", "x"} {
		if _, _, e := Range(s); e == nil {
			t.Fatal(s)
		}
	}
	c.Forwards = []Forward{{"tcp", "443", "2020"}, {"udp", "443", "443"}, {"udp", "10000-20000", "10000-20000"}}
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
}
func TestStorage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c := valid()
	if e := Save(p, c); e != nil {
		t.Fatal(e)
	}
	v, e := Load(p)
	if e != nil || v.Local != c.Local {
		t.Fatal(v, e)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	for _, s := range []string{`{"unknown":1}`, `{} {}`, `{} garbage`} {
		_ = os.WriteFile(p, []byte(s), 0600)
		if _, e = Load(p); e == nil {
			t.Fatal("accepted", s)
		}
	}
}

func TestTunnelNames(t *testing.T) {
	for _, s := range []string{"", "exit2", "a-1", "a_1"} {
		if e := ValidName(s); e != nil {
			t.Fatal(s, e)
		}
	}
	for _, s := range []string{"default", "../x", "UPPER", "bad;name", "a1234567890"} {
		if ValidName(s) == nil {
			t.Fatal(s)
		}
	}
}
