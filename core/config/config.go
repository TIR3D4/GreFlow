// Package config validates declarative configuration before any host changes.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Forward struct {
	Protocol    string `json:"protocol"`
	Public      string `json:"public"`
	Destination string `json:"destination"`
}
type Config struct {
	Role      string    `json:"role"`
	Local     string    `json:"local"`
	Remote    string    `json:"remote"`
	Interface string    `json:"interface"`
	Network   string    `json:"network"`
	MTU       int       `json:"mtu"`
	Forwards  []Forward `json:"forwards"`
}

func Range(s string) (int, int, error) {
	p := strings.Split(s, "-")
	if len(p) > 2 {
		return 0, 0, fmt.Errorf("invalid port range")
	}
	a, e := strconv.Atoi(p[0])
	if e != nil || a < 1 || a > 65535 {
		return 0, 0, fmt.Errorf("invalid port: %s", s)
	}
	b := a
	if len(p) == 2 {
		b, e = strconv.Atoi(p[1])
	}
	if e != nil || b < a || b > 65535 {
		return 0, 0, fmt.Errorf("invalid range: %s", s)
	}
	return a, b, nil
}
func (c Config) Validate() error {
	if c.Role != "entry" && c.Role != "exit" {
		return fmt.Errorf("role must be entry or exit")
	}
	for _, s := range []string{c.Local, c.Remote} {
		a, e := netip.ParseAddr(s)
		if e != nil || !a.Is4() || !a.IsGlobalUnicast() {
			return fmt.Errorf("invalid IPv4 endpoint: %s", s)
		}
	}
	if c.Local == c.Remote {
		return fmt.Errorf("endpoints must differ")
	}
	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,14}$`).MatchString(c.Interface) || c.Interface == "lo" {
		return fmt.Errorf("invalid interface")
	}
	n, e := netip.ParsePrefix(c.Network)
	if e != nil || !n.Addr().Is4() || n.Bits() != 30 || n != n.Masked() || !n.Addr().IsPrivate() {
		return fmt.Errorf("network must be an aligned private IPv4 /30")
	}
	if n.Contains(netip.MustParseAddr(c.Local)) || n.Contains(netip.MustParseAddr(c.Remote)) {
		return fmt.Errorf("GRE network overlaps endpoint")
	}
	if c.MTU < 576 || c.MTU > 1476 {
		return fmt.Errorf("MTU must be 576..1476")
	}
	for i, f := range c.Forwards {
		if f.Protocol != "tcp" && f.Protocol != "udp" {
			return fmt.Errorf("protocol must be tcp or udp")
		}
		a, b, e := Range(f.Public)
		if e != nil {
			return e
		}
		x, y, e := Range(f.Destination)
		if e != nil {
			return e
		}
		if b-a != y-x {
			return fmt.Errorf("ranges must have equal width")
		}
		if b > a && a != x {
			return fmt.Errorf("v0.1 supports identity range mapping only")
		}
		for _, g := range c.Forwards[:i] {
			u, v, _ := Range(g.Public)
			if f.Protocol == g.Protocol && a <= v && u <= b {
				return fmt.Errorf("overlapping public ports")
			}
		}
	}
	return nil
}
func (c Config) Addresses() (string, string) {
	n := netip.MustParsePrefix(c.Network)
	a := n.Addr().Next()
	b := a.Next()
	if c.Role == "exit" {
		return b.String(), a.String()
	}
	return a.String(), b.String()
}
func Load(path string) (Config, error) {
	var c Config
	f, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return c, fmt.Errorf("trailing JSON")
	}
	return c, c.Validate()
}
func Save(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".greflow-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(append(b, '\n'))
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(name, path)
}
