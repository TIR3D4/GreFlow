package gre

import (
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/core/tunnel"
	"github.com/TIR3D4/GreFlow/firewall"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"net"
	"os"
	"strings"
	"time"
)

func Observe(m manager.Manager, probe bool) (protocol.Tunnel, error) {
	c, e := config.Load(m.ConfigPath())
	if e != nil {
		return protocol.Tunnel{}, e
	}
	now := time.Now().UTC()
	t := protocol.Tunnel{Name: m.Name, Desired: c, Revision: protocol.Revision(c), Checks: []protocol.Evidence{}}
	add := func(kind, status, detail string) {
		t.Checks = append(t.Checks, protocol.Evidence{Kind: kind, Status: status, Detail: detail, ObservedAt: now, ExpiresAt: now.Add(180 * time.Second)})
	}
	s, e := m.State()
	if e != nil {
		return t, e
	}
	t.Applied = s.Config
	t.Generation = s.Since.Format(time.RFC3339Nano)
	t.Drift = s.Config == nil || protocol.Revision(*s.Config) != t.Revision
	v, se := m.R.Run("systemctl", "show", m.Service(), "--property=ActiveState,SubState", "--value")
	status := "unknown"
	if se == nil {
		status = "failed"
		if strings.HasPrefix(v, "active\n") {
			status = "ok"
		}
	}
	add("service", status, strings.TrimSpace(v))
	l, le := tunnel.Inspect(m.R, c.Interface)
	status = "failed"
	if le == nil && l.Alias == m.Alias() && strings.Contains(strings.Join(l.Flags, " "), "UP") {
		status = "ok"
		t.RX = &l.Stats.RX.Bytes
		t.TX = &l.Stats.TX.Bytes
	}
	add("interface", status, "owned link and UP flag")
	fe := (firewall.IPTables{R: m.R, Name: m.Name}).Check(c)
	status = "ok"
	if fe != nil {
		status = "failed"
	}
	add("rules", status, "expected owned rules and hooks")
	if probe {
		_, peer := c.Addresses()
		start := time.Now()
		_, pe := m.R.Run("ping", "-n", "-c", "1", "-W", "2", "-I", c.Interface, peer)
		status = "ok"
		if pe != nil {
			status = "failed"
		}
		add("gre_peer", status, peer)
		ms := float64(time.Since(start).Microseconds()) / 1000
		t.Checks[len(t.Checks)-1].LatencyMS = &ms
		for _, f := range c.Forwards {
			kind := "destination:" + f.Protocol + ":" + f.Public
			if c.Role != "entry" || f.Protocol != "tcp" || strings.Contains(f.Destination, "-") {
				add(kind, "unsupported", "application probe required")
				continue
			}
			pe := Probe(c, f.Destination)
			status = "ok"
			if pe != nil {
				status = "failed"
			}
			add(kind, status, "TCP connect only; no application handshake")
		}
	} else {
		add("gre_peer", "unknown", "active probe not requested")
	}
	t.Checks = append(t.Checks, verificationEvidence(m, t, s.Active && s.Verified))
	return t, nil
}
func Probe(c config.Config, port string) error {
	if _, _, e := config.Range(port); e != nil || strings.Contains(port, "-") {
		return fmt.Errorf("single TCP destination required")
	}
	local, peer := c.Addresses()
	d := net.Dialer{Timeout: 3 * time.Second, LocalAddr: &net.TCPAddr{IP: net.ParseIP(local)}}
	conn, e := d.Dial("tcp", net.JoinHostPort(peer, port))
	if e == nil {
		conn.Close()
	}
	return e
}
func Discover(m manager.Manager) (protocol.Inventory, error) {
	out := protocol.Inventory{Version: 1, Tunnels: []protocol.Tunnel{}, Addresses: []string{}, ObservedAt: time.Now().UTC()}
	id, e := os.ReadFile(m.Path("/etc/machine-id"))
	if e != nil {
		return out, e
	}
	out.HostID = strings.TrimSpace(string(id))
	addresses, e := net.InterfaceAddrs()
	if e != nil {
		return out, e
	}
	for _, a := range addresses {
		out.Addresses = append(out.Addresses, a.String())
	}
	instances, e := m.Instances()
	if e != nil {
		return out, e
	}
	for _, v := range instances {
		t, e := Observe(v, false)
		if e != nil {
			return out, e
		}
		out.Tunnels = append(out.Tunnels, t)
	}
	out.Schedules = ObserveSchedules(m)
	return out, nil
}
