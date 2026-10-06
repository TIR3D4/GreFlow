// Package tunnel owns only interfaces carrying the GreFlow alias.
package tunnel

import (
	"encoding/json"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/system"
)

const Alias = "greflow:v0.1"

type Link struct {
	Alias string   `json:"ifalias"`
	Flags []string `json:"flags"`
	MTU   int      `json:"mtu"`
	Stats struct {
		RX struct {
			Bytes uint64 `json:"bytes"`
		} `json:"rx"`
		TX struct {
			Bytes uint64 `json:"bytes"`
		} `json:"tx"`
	} `json:"stats64"`
}

func Inspect(r system.Runner, name string) (Link, error) {
	var l []Link
	s, e := r.Run("ip", "-j", "-s", "link", "show", "dev", name)
	if e != nil {
		return Link{}, e
	}
	if e = json.Unmarshal([]byte(s), &l); e != nil || len(l) != 1 {
		return Link{}, fmt.Errorf("invalid ip link response")
	}
	return l[0], nil
}
func Down(r system.Runner, c config.Config) error { return DownOwned(r, c, Alias) }
func DownOwned(r system.Runner, c config.Config, alias string) error {
	l, e := Inspect(r, c.Interface)
	if e != nil {
		if _, allErr := r.Run("ip", "-j", "link", "show"); allErr != nil {
			return allErr
		}
		return nil
	}
	if l.Alias != alias {
		return fmt.Errorf("refusing unowned interface %s", c.Interface)
	}
	_, e = r.Run("ip", "link", "delete", c.Interface)
	return e
}
func Apply(r system.Runner, c config.Config) error { return ApplyOwned(r, c, Alias) }
func ApplyOwned(r system.Runner, c config.Config, alias string) error {
	if l, e := Inspect(r, c.Interface); e == nil {
		if l.Alias != alias {
			return fmt.Errorf("interface %s belongs to another application", c.Interface)
		}
		if e = DownOwned(r, c, alias); e != nil {
			return e
		}
	}
	_, e := r.Run("ip", "tunnel", "add", c.Interface, "mode", "gre", "local", c.Local, "remote", c.Remote, "ttl", "255")
	if e != nil {
		return e
	}
	// Set ownership immediately; cleanup is safe even if subsequent steps fail.
	if _, e = r.Run("ip", "link", "set", "dev", c.Interface, "alias", alias); e != nil {
		_, _ = r.Run("ip", "link", "delete", c.Interface)
		return e
	}
	local, _ := c.Addresses()
	for _, a := range [][]string{{"link", "set", "dev", c.Interface, "mtu", fmt.Sprint(c.MTU)}, {"addr", "add", local + "/30", "dev", c.Interface}, {"link", "set", "dev", c.Interface, "up"}} {
		if _, e = r.Run("ip", a...); e != nil {
			return e
		}
	}
	return nil
}
