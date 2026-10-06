// Package manager serializes host changes and records rollback state on disk.
package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/system"
	"github.com/TIR3D4/GreFlow/core/tunnel"
	"github.com/TIR3D4/GreFlow/firewall"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type State struct {
	Config   *config.Config    `json:"config,omitempty"`
	Original map[string]string `json:"original_sysctl"`
	Written  map[string]string `json:"written_sysctl"`
	Active   bool              `json:"active"`
	Since    time.Time         `json:"since"`
	Verified bool              `json:"user_verified"`
}
type Manager struct {
	Root string
	R    system.Runner
}

func (m Manager) Path(s string) string {
	if m.Root == "" {
		return s
	}
	return filepath.Join(m.Root, strings.TrimPrefix(s, "/"))
}
func (m Manager) ConfigPath() string { return m.Path("/etc/greflow/config.json") }
func (m Manager) StatePath() string  { return m.Path("/var/lib/greflow/state.json") }
func (m Manager) Lock() (func(), error) {
	p := m.Path("/run/greflow.lock")
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("another GreFlow operation is running")
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
func (m Manager) State() (State, error) {
	s := State{Original: map[string]string{}, Written: map[string]string{}}
	b, e := os.ReadFile(m.StatePath())
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	e = json.Unmarshal(b, &s)
	if s.Original == nil {
		s.Original = map[string]string{}
	}
	if s.Written == nil {
		s.Written = map[string]string{}
	}
	return s, e
}
func (m Manager) log(msg string) {
	p := m.Path("/var/log/greflow/events.jsonl")
	_ = os.MkdirAll(filepath.Dir(p), 0700)
	f, e := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if e == nil {
		defer f.Close()
		_ = json.NewEncoder(f).Encode(map[string]any{"time": time.Now().UTC(), "message": msg})
	}
}
func (m Manager) backup(s State) (string, error) {
	p := m.Path("/var/lib/greflow/backups/" + time.Now().UTC().Format("20060102T150405.000000000"))
	if e := config.Save(filepath.Join(p, "state.json"), s); e != nil {
		return "", e
	}
	if c, e := os.ReadFile(m.ConfigPath()); e == nil {
		if e = os.WriteFile(filepath.Join(p, "config.json"), c, 0600); e != nil {
			return "", e
		}
	}
	v, e := m.R.Run("iptables-save", "-c")
	if e != nil {
		return "", e
	}
	if e = os.WriteFile(filepath.Join(p, "iptables.txt"), []byte(v), 0600); e != nil {
		return "", e
	}
	return p, nil
}
func (m Manager) sysctl(s *State, k, v string) error {
	cur, e := m.R.Run("sysctl", "-n", k)
	if e != nil {
		return e
	}
	if _, ok := s.Original[k]; !ok {
		s.Original[k] = strings.TrimSpace(cur)
	}
	s.Written[k] = v
	// Persist recovery information before changing a global kernel setting.
	if e = config.Save(m.StatePath(), s); e != nil {
		return e
	}
	_, e = m.R.Run("sysctl", "-w", k+"="+v)
	return e
}
func (m Manager) restore(s State) error {
	var errs []error
	var keys []string
	for k := range s.Original {
		cur, e := m.R.Run("sysctl", "-n", k)
		if e != nil {
			errs = append(errs, e)
			continue
		}
		if strings.TrimSpace(cur) == s.Written[k] {
			keys = append(keys, k)
		}
	}
	// ip_forward can reset other IPv4 defaults: restore it before the remaining values.
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "net.ipv4.ip_forward" {
			return true
		}
		if keys[j] == "net.ipv4.ip_forward" {
			return false
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		if _, e := m.R.Run("sysctl", "-w", k+"="+s.Original[k]); e != nil {
			errs = append(errs, e)
		}
	}
	return errors.Join(errs...)
}
func (m Manager) host(c config.Config, s *State) error {
	if e := tunnel.Apply(m.R, c); e != nil {
		return e
	}
	if c.Role == "entry" {
		// Capture all global originals before the first kernel mutation.
		for _, k := range []string{"net.ipv4.ip_forward", "net.ipv4.conf.all.rp_filter"} {
			if _, ok := s.Original[k]; !ok {
				v, e := m.R.Run("sysctl", "-n", k)
				if e != nil {
					return e
				}
				s.Original[k] = strings.TrimSpace(v)
			}
		}
		for _, x := range [][2]string{{"net.ipv4.ip_forward", "1"}, {"net.ipv4.conf.all.rp_filter", "0"}} {
			if e := m.sysctl(s, x[0], x[1]); e != nil {
				return e
			}
		}
	}
	for _, k := range []string{"net.ipv4.conf." + c.Interface + ".rp_filter", "net.ipv4.conf." + c.Interface + ".accept_local"} {
		v := "0"
		if _, e := m.R.Run("sysctl", "-w", k+"="+v); e != nil {
			return e
		}
	}
	return (firewall.IPTables{R: m.R}).Apply(c)
}
func (m Manager) Apply(c config.Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	old, e := m.State()
	if e != nil {
		return e
	}
	if old.Config != nil && old.Config.Interface != c.Interface {
		return fmt.Errorf("interface name cannot change; uninstall first")
	}
	if old.Config != nil && (old.Config.Role != c.Role || old.Config.Local != c.Local || old.Config.Remote != c.Remote || old.Config.Network != c.Network) {
		return fmt.Errorf("endpoint/role/network cannot change; uninstall first")
	}
	if _, e = m.R.Run("ip", "route", "get", c.Remote); e != nil {
		return e
	}
	v, routeErr := m.R.Run("ip", "-j", "route", "get", c.Remote)
	if routeErr != nil {
		return routeErr
	}
	var routes []struct {
		Dev string `json:"dev"`
	}
	if e = json.Unmarshal([]byte(v), &routes); e != nil {
		return e
	}
	if len(routes) == 0 || routes[0].Dev == c.Interface {
		return fmt.Errorf("invalid or recursive underlay route")
	}
	v, e = m.R.Run("ip", "-j", "-4", "addr", "show")
	if e != nil {
		return e
	}
	var addresses []struct {
		AddrInfo []struct {
			Local string `json:"local"`
		} `json:"addr_info"`
	}
	if e = json.Unmarshal([]byte(v), &addresses); e != nil {
		return e
	}
	assigned := false
	for _, link := range addresses {
		for _, a := range link.AddrInfo {
			if a.Local == c.Local {
				assigned = true
			}
		}
	}
	if !assigned {
		return fmt.Errorf("local endpoint %s is not assigned on this host (NAT endpoints require a separate design)", c.Local)
	}
	if _, e = m.backup(old); e != nil {
		return e
	}
	next := old
	next.Original = map[string]string{}
	next.Written = map[string]string{}
	for k, v := range old.Original {
		next.Original[k] = v
	}
	for k, v := range old.Written {
		next.Written[k] = v
	}
	next.Verified = false
	next.Config = &c
	if e = m.host(c, &next); e != nil {
		return m.failed(e, old, next, c)
	}
	next.Active = true
	next.Since = time.Now().UTC()
	if e = config.Save(m.StatePath(), next); e != nil {
		return m.failed(e, old, next, c)
	}
	if e = config.Save(m.ConfigPath(), c); e != nil {
		return m.failed(e, old, next, c)
	}
	m.log("configuration applied")
	return nil
}
func (m Manager) failed(cause error, old, next State, c config.Config) error {
	m.log("apply failed: " + cause.Error())
	var cleanup []error
	cleanup = append(cleanup, (firewall.IPTables{R: m.R}).Remove(), tunnel.Down(m.R, c), m.restore(next))
	if old.Active && old.Config != nil {
		cleanup = append(cleanup, m.host(*old.Config, &old))
	}
	cleanup = append(cleanup, config.Save(m.StatePath(), old))
	if old.Config != nil {
		cleanup = append(cleanup, config.Save(m.ConfigPath(), old.Config))
	} else {
		_ = os.Remove(m.ConfigPath())
	}
	return fmt.Errorf("apply failed: %w; rollback: %v", cause, errors.Join(cleanup...))
}
func (m Manager) Down() error {
	s, e := m.State()
	if e != nil {
		return e
	}
	if s.Config == nil {
		return nil
	}
	if e = (firewall.IPTables{R: m.R}).Remove(); e != nil {
		return e
	}
	if e = tunnel.Down(m.R, *s.Config); e != nil {
		return e
	}
	if e = m.restore(s); e != nil {
		return e
	}
	s.Active = false
	s.Verified = false
	if e = config.Save(m.StatePath(), s); e != nil {
		return e
	}
	m.log("tunnel stopped")
	return nil
}

const Unit = `[Unit]
Description=GreFlow GRE tunnel and port forwarding
Wants=network-online.target
After=network-online.target
ConditionPathExists=/etc/greflow/config.json

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/local/bin/greflow apply
ExecStop=/usr/local/bin/greflow down
TimeoutStartSec=120
TimeoutStopSec=120

[Install]
WantedBy=multi-user.target
`

func (m Manager) Persist() error {
	if m.Root != "" {
		return fmt.Errorf("systemd persistence unavailable with GREFLOW_ROOT")
	}
	if e := os.WriteFile("/etc/systemd/system/greflow.service", []byte(Unit), 0644); e != nil {
		return e
	}
	if _, e := m.R.Run("systemctl", "daemon-reload"); e != nil {
		return e
	}
	_, e := m.R.Run("systemctl", "enable", "greflow.service")
	return e
}
func (m Manager) Uninstall() error {
	if e := m.Down(); e != nil {
		return e
	}
	if m.Root == "" {
		if _, e := os.Stat("/etc/systemd/system/greflow.service"); e == nil {
			if _, e = m.R.Run("systemctl", "disable", "greflow.service"); e != nil {
				return e
			}
			if e = os.Remove("/etc/systemd/system/greflow.service"); e != nil {
				return e
			}
			if _, e = m.R.Run("systemctl", "daemon-reload"); e != nil {
				return e
			}
		}
	}
	for _, p := range []string{"/etc/greflow", "/var/lib/greflow/state.json"} {
		if e := os.RemoveAll(m.Path(p)); e != nil {
			return e
		}
	}
	if m.Root == "" {
		if e := os.Remove("/usr/local/bin/greflow"); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	m.log("uninstalled; recovery backups and audit logs retained")
	return nil
}
func (m Manager) Rollback(dir string) error {
	b, e := os.ReadFile(filepath.Join(dir, "state.json"))
	if e != nil {
		return e
	}
	var s State
	if e = json.Unmarshal(b, &s); e != nil {
		return e
	}
	if s.Config == nil {
		return fmt.Errorf("backup has no previous configuration; use down")
	}
	return m.Apply(*s.Config)
}
