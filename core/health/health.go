// Package health reports independently observed layers; UDP is never inferred from TCP.
package health

import (
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/core/tunnel"
	"github.com/TIR3D4/GreFlow/firewall"
	"net"
	"os"
	"strings"
	"time"
)

func result(name string, e error) {
	if e != nil {
		fmt.Printf("%-28s FAIL: %v\n", name, e)
	} else {
		fmt.Printf("%-28s OK\n", name)
	}
}
func Status(m manager.Manager, c config.Config, probe bool) error {
	failures := 0
	check := func(n string, e error) {
		result(n, e)
		if e != nil {
			failures++
		}
	}
	s, e := m.State()
	if e != nil {
		return e
	}
	if m.Root == "" {
		_, e = m.R.Run("systemctl", "is-enabled", "greflow.service")
		check("H0 persistence enabled", e)
		v, _ := m.R.Run("systemctl", "is-active", "greflow.service")
		fmt.Printf("Service state: %s", v)
	} else {
		fmt.Println("H0 persistence UNKNOWN (test root)")
	}
	l, e := tunnel.Inspect(m.R, c.Interface)
	if e == nil {
		if l.Alias != tunnel.Alias || !strings.Contains(strings.Join(l.Flags, " "), "UP") {
			e = fmt.Errorf("interface ownership/state mismatch")
		}
	}
	check("H1 GRE interface", e)
	local, peer := c.Addresses()
	v, e := m.R.Run("ip", "-4", "addr", "show", "dev", c.Interface)
	if e == nil && !strings.Contains(v, local+"/30") {
		e = fmt.Errorf("GRE address missing")
	}
	check("H1 GRE address", e)
	if probe {
		v, e = m.R.Run("ping", "-n", "-c", "3", "-W", "2", "-I", c.Interface, peer)
		check("H2 GRE peer ping", e)
		if v != "" {
			fmt.Print(v)
		}
		if c.Role == "entry" {
			for _, f := range c.Forwards {
				if f.Protocol == "udp" {
					fmt.Printf("H3 UDP %s UNKNOWN (application test required)\n", f.Destination)
					continue
				}
				a, b, _ := config.Range(f.Destination)
				if a != b {
					fmt.Printf("H3 TCP range %s UNKNOWN (sample/application test required)\n", f.Destination)
					continue
				}
				conn, ce := net.DialTimeout("tcp", net.JoinHostPort(peer, f.Destination), 3*time.Second)
				if ce == nil {
					_ = conn.Close()
				}
				check("H3 TCP destination "+f.Destination, ce)
			}
		} else {
			fmt.Println("H3 destination: run ss -lntup and verify service binding")
		}
	} else {
		fmt.Println("H2/H3 UNKNOWN (run greflow test)")
	}
	check("H4 firewall rules", (firewall.IPTables{R: m.R}).Check(c))
	fmt.Printf("H5 traffic RX=%d TX=%d bytes\n", l.Stats.RX.Bytes, l.Stats.TX.Bytes)
	if s.Verified && s.Active {
		fmt.Println("H6 user verified YES")
	} else {
		fmt.Println("H6 user verified UNKNOWN")
	}
	if failures > 0 {
		return fmt.Errorf("%d health checks failed", failures)
	}
	return nil
}
func Doctor(m manager.Manager, c config.Config) error {
	e := Status(m, c, true)
	for _, cmd := range [][]string{{"ip", "route", "get", c.Remote}, {"ip", "-d", "link", "show", "dev", c.Interface}, {"sysctl", "net.ipv4.ip_forward", "net.ipv4.conf.all.rp_filter", "net.ipv4.conf." + c.Interface + ".rp_filter", "net.netfilter.nf_conntrack_count", "net.netfilter.nf_conntrack_max"}, {"iptables", "--version"}, {"ss", "-lntup"}, {"iptables", "-w", "5", "-t", "nat", "-nvxL", "GREFLOW_PREROUTING"}} {
		v, ce := m.R.Run(cmd[0], cmd[1:]...)
		fmt.Printf("\n$ %s\n%s", strings.Join(cmd, " "), v)
		if ce != nil {
			fmt.Println(ce)
		}
	}
	if c.MTU != 1476 {
		fmt.Printf("Configured MTU: %d (check underlay path MTU)\n", c.MTU)
	}
	return e
}
func Stats(m manager.Manager, c config.Config) error {
	l, e := tunnel.Inspect(m.R, c.Interface)
	if e != nil {
		return e
	}
	s, e := m.State()
	if e != nil {
		return e
	}
	fmt.Printf("Tunnel %s age=%s RX=%d TX=%d bytes\n", c.Interface, time.Since(s.Since).Round(time.Second), l.Stats.RX.Bytes, l.Stats.TX.Bytes)
	for _, cmd := range [][]string{{"iptables", "-w", "5", "-t", "nat", "-nvxL", "GREFLOW_PREROUTING"}, {"iptables", "-w", "5", "-t", "filter", "-nvxL", "GREFLOW_FORWARD"}, {"sysctl", "net.netfilter.nf_conntrack_count", "net.netfilter.nf_conntrack_max"}, {"ss", "-s"}} {
		v, ce := m.R.Run(cmd[0], cmd[1:]...)
		fmt.Print(v)
		if ce != nil {
			fmt.Println(ce)
		}
	}
	fmt.Println("NAT counters count initial packets; FORWARD counters count traffic. ss reports local sockets, not forwarded connections.")
	if v, ce := m.R.Run("conntrack", "-L", "-o", "extended"); ce == nil {
		counts := CountForwarded(v, c)
		fmt.Printf("Forwarded conntrack entries: %v\n", counts)
	} else {
		fmt.Println("Per-port connections UNKNOWN (install conntrack)")
	}
	return nil
}
func Tune(m manager.Manager) error {
	v, _ := os.ReadFile("/proc/meminfo")
	lines := strings.Split(string(v), "\n")
	if len(lines) > 0 {
		fmt.Println(lines[0])
	}
	for _, cmd := range [][]string{{"getconf", "_NPROCESSORS_ONLN"}, {"sysctl", "net.netfilter.nf_conntrack_count", "net.netfilter.nf_conntrack_max", "net.netfilter.nf_conntrack_tcp_timeout_established", "net.netfilter.nf_conntrack_udp_timeout", "net.core.somaxconn", "net.core.rmem_max", "net.core.wmem_max", "net.core.netdev_max_backlog", "net.ipv4.tcp_max_syn_backlog", "net.ipv4.tcp_fin_timeout", "fs.file-max"}} {
		v, e := m.R.Run(cmd[0], cmd[1:]...)
		fmt.Print(v)
		if e != nil {
			fmt.Println(e)
		}
	}
	fmt.Println("Advisory only. Increase conntrack capacity only for measured saturation with RAM headroom; shorten timeouts only after testing long-lived connections. No kernel values changed.")
	return nil
}

// CountForwarded matches conntrack's original public tuple and translated reply tuple.
func CountForwarded(output string, c config.Config) map[string]int {
	counts := map[string]int{}
	if c.Role != "entry" {
		return counts
	}
	local, peer := c.Addresses()
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		var src, dst, dport []string
		for _, field := range fields {
			if strings.HasPrefix(field, "src=") {
				src = append(src, strings.TrimPrefix(field, "src="))
			}
			if strings.HasPrefix(field, "dst=") {
				dst = append(dst, strings.TrimPrefix(field, "dst="))
			}
			if strings.HasPrefix(field, "dport=") {
				dport = append(dport, strings.TrimPrefix(field, "dport="))
			}
		}
		if len(src) != 2 || len(dst) != 2 || len(dport) != 2 || dst[0] != c.Local || src[1] != peer || dst[1] != local {
			continue
		}
		var port int
		if _, e := fmt.Sscanf(dport[0], "%d", &port); e != nil {
			continue
		}
		for _, f := range c.Forwards {
			a, b, _ := config.Range(f.Public)
			if fields[0] == f.Protocol && port >= a && port <= b {
				counts[f.Protocol+":"+f.Public]++
			}
		}
	}
	return counts
}
