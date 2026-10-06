// Named instances share one lock and kernel baseline but own their link/rules/service.
package manager

import (
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/tunnel"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
)

func (m Manager) Label() string {
	if m.Name == "" {
		return "default"
	}
	return m.Name
}
func (m Manager) Alias() string {
	if m.Name == "" {
		return tunnel.Alias
	}
	return "greflow:instance:" + m.Name
}
func (m Manager) Service() string {
	if m.Name == "" {
		return "greflow.service"
	}
	return "greflow-" + m.Name + ".service"
}
func (m Manager) BackupPrefix() string {
	if m.Name == "" {
		return ""
	}
	return m.Name + "/"
}
func (m Manager) Instances() ([]Manager, error) {
	var out []Manager
	base := m
	base.Name = ""
	if _, e := os.Stat(base.ConfigPath()); e == nil {
		out = append(out, base)
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	entries, e := os.ReadDir(m.Path("/etc/greflow/tunnels"))
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if e := config.ValidName(entry.Name()); e != nil {
			return nil, e
		}
		other := m
		other.Name = entry.Name()
		if _, e := os.Stat(other.ConfigPath()); e == nil {
			out = append(out, other)
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label() < out[j].Label() })
	return out, nil
}
func (m Manager) Siblings() ([]Manager, error) {
	all, e := m.Instances()
	if e != nil {
		return nil, e
	}
	var out []Manager
	for _, other := range all {
		if other.Name != m.Name {
			out = append(out, other)
		}
	}
	return out, nil
}
func (m Manager) ValidatePeers(c config.Config) error {
	siblings, e := m.Siblings()
	if e != nil {
		return e
	}
	for _, other := range siblings {
		v, e := config.Load(other.ConfigPath())
		if e != nil {
			return fmt.Errorf("tunnel %s: %w", other.Label(), e)
		}
		if v.Interface == c.Interface {
			return fmt.Errorf("interface already owned by tunnel %s", other.Label())
		}
		a := netip.MustParsePrefix(c.Network)
		b := netip.MustParsePrefix(v.Network)
		if a.Overlaps(b) {
			return fmt.Errorf("GRE network overlaps tunnel %s", other.Label())
		}
		if c.Local == v.Local && c.Remote == v.Remote {
			return fmt.Errorf("unkeyed GRE endpoint pair already used by tunnel %s", other.Label())
		}
		if c.Role == "entry" && v.Role == "entry" && c.Local == v.Local {
			for _, f := range c.Forwards {
				for _, g := range v.Forwards {
					a, b, _ := config.Range(f.Public)
					x, y, _ := config.Range(g.Public)
					if f.Protocol == g.Protocol && a <= y && x <= b {
						return fmt.Errorf("public %s %s conflicts with tunnel %s; remove its mapping first", f.Protocol, f.Public, other.Label())
					}
				}
			}
		}
	}
	return nil
}

// Inherit the original pre-GreFlow kernel values from an active sibling. Every
// active entry keeps the same baseline, so stopping one cannot disable another.
func (m Manager) inheritOriginal(s *State) error {
	siblings, e := m.Siblings()
	if e != nil {
		return e
	}
	for _, other := range siblings {
		v, e := other.State()
		if e != nil {
			return e
		}
		if v.Active && v.Config != nil && v.Config.Role == "entry" {
			for k, value := range v.Original {
				s.Original[k] = value
			}
		}
	}
	return nil
}

// DefaultInterface is deterministic and fits Linux's 15-character limit.
func (m Manager) DefaultInterface() string {
	if m.Name == "" {
		return "greflow0"
	}
	return "gf-" + m.Name
}
func (m Manager) ConfigDirectory() string { return filepath.Dir(m.ConfigPath()) }
