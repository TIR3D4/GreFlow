// Package firewall isolates host rules in dedicated chains and tagged hooks.
package firewall

import (
	"crypto/sha256"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/system"
	"strings"
)

type Rule struct {
	Table, Chain string
	Args         []string
}
type Backend interface {
	Apply(config.Config) error
	Remove() error
	Check(config.Config) error
}
type IPTables struct {
	R    system.Runner
	Name string
}

var chains = [][3]string{{"filter", "INPUT", "GREFLOW_INPUT"}, {"filter", "FORWARD", "GREFLOW_FORWARD"}, {"nat", "PREROUTING", "GREFLOW_PREROUTING"}, {"nat", "POSTROUTING", "GREFLOW_POSTROUTING"}, {"mangle", "FORWARD", "GREFLOW_MANGLE"}}

// ChainName keeps legacy names intact and gives named tunnels stable short chains.
func ChainName(name, legacy string) string {
	if name == "" {
		return legacy
	}
	sum := sha256.Sum256([]byte(name))
	suffix := map[string]string{"GREFLOW_INPUT": "IN", "GREFLOW_FORWARD": "FW", "GREFLOW_PREROUTING": "PRE", "GREFLOW_POSTROUTING": "POST", "GREFLOW_MANGLE": "MSS"}[legacy]
	return fmt.Sprintf("GF_%x_%s", sum[:6], suffix)
}
func (b IPTables) chains() [][3]string {
	out := make([][3]string, len(chains))
	copy(out, chains)
	for i := range out {
		out[i][2] = ChainName(b.Name, out[i][2])
	}
	return out
}
func (b IPTables) marker(kind string) string {
	if b.Name == "" {
		return "greflow:" + kind
	}
	return "greflow:" + b.Name + ":" + kind
}
func (b IPTables) hook(chain string) []string {
	return []string{"-m", "comment", "--comment", b.marker("hook"), "-j", chain}
}
func (b IPTables) ruleArgs(a []string) []string {
	return append(append([]string{}, a...), "-m", "comment", "--comment", b.marker("rule"))
}
func (b IPTables) Rules(c config.Config) []Rule {
	rules := Rules(c)
	for i := range rules {
		rules[i].Chain = ChainName(b.Name, rules[i].Chain)
	}
	return rules
}
func (b IPTables) run(t string, a ...string) (string, error) {
	return b.R.Run("iptables", append([]string{"-w", "5", "-t", t}, a...)...)
}
func hook(chain string) []string {
	return []string{"-m", "comment", "--comment", "greflow:hook", "-j", chain}
}
func Rules(c config.Config) []Rule {
	local, peer := c.Addresses()
	var rules []Rule
	add := func(t, ch string, a ...string) { rules = append(rules, Rule{t, ch, a}) }
	add("filter", "GREFLOW_INPUT", "-s", c.Remote, "-d", c.Local, "-p", "gre", "-j", "ACCEPT")
	add("filter", "GREFLOW_INPUT", "-i", c.Interface, "-s", peer, "-d", local, "-p", "icmp", "-j", "ACCEPT")
	for _, f := range c.Forwards {
		pub := strings.ReplaceAll(f.Public, "-", ":")
		dst := strings.ReplaceAll(f.Destination, "-", ":")
		if c.Role == "exit" {
			add("filter", "GREFLOW_INPUT", "-i", c.Interface, "-s", peer, "-d", local, "-p", f.Protocol, "--dport", dst, "-j", "ACCEPT")
			continue
		}
		to := peer + ":" + f.Destination
		if strings.Contains(f.Public, "-") {
			to = peer
		}
		add("nat", "GREFLOW_PREROUTING", "-d", c.Local, "-p", f.Protocol, "--dport", pub, "-j", "DNAT", "--to-destination", to)
		add("nat", "GREFLOW_POSTROUTING", "-o", c.Interface, "-d", peer, "-p", f.Protocol, "--dport", dst, "-j", "SNAT", "--to-source", local)
		add("filter", "GREFLOW_FORWARD", "-o", c.Interface, "-d", peer, "-p", f.Protocol, "--dport", dst, "-m", "conntrack", "--ctstate", "NEW,ESTABLISHED,RELATED", "-j", "ACCEPT")
		add("filter", "GREFLOW_FORWARD", "-i", c.Interface, "-s", peer, "-p", f.Protocol, "--sport", dst, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")
	}
	if c.Role == "entry" {
		add("filter", "GREFLOW_FORWARD", "-o", c.Interface, "-p", "icmp", "-m", "conntrack", "--ctstate", "RELATED", "-j", "ACCEPT")
		add("filter", "GREFLOW_FORWARD", "-i", c.Interface, "-p", "icmp", "-m", "conntrack", "--ctstate", "RELATED", "-j", "ACCEPT")
		add("mangle", "GREFLOW_MANGLE", "-o", c.Interface, "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")
	}
	add("mangle", "GREFLOW_MANGLE", "-i", c.Interface, "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")
	// Multiple public ports may target one destination; share identical SNAT/filter rules.
	seen := map[string]bool{}
	unique := make([]Rule, 0, len(rules))
	for _, r := range rules {
		key := r.Table + "\x00" + r.Chain + "\x00" + strings.Join(r.Args, "\x00")
		if !seen[key] {
			seen[key] = true
			unique = append(unique, r)
		}
	}
	return unique
}
func (b IPTables) owned(t, ch string) error {
	s, e := b.run(t, "-S", ch)
	if e != nil {
		return e
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "-A ") && !strings.Contains(line, b.marker("rule")) {
			return fmt.Errorf("unowned rules in %s", ch)
		}
	}
	return nil
}

// DNAT/SNAT have a trailing option: append comments instead of inspecting jump position.
func ruleArgs(a []string) []string {
	return append(append([]string{}, a...), "-m", "comment", "--comment", "greflow:rule")
}
func (b IPTables) Apply(c config.Config) error {
	for _, x := range b.chains() {
		if _, e := b.run(x[0], "-S", x[2]); e != nil {
			if _, e = b.run(x[0], "-N", x[2]); e != nil {
				return e
			}
		} else if e = b.owned(x[0], x[2]); e != nil {
			return e
		}
	}
	if e := b.RemoveHooks(); e != nil {
		return e
	}
	for _, x := range b.chains() {
		if _, e := b.run(x[0], "-F", x[2]); e != nil {
			return e
		}
	}
	for _, r := range b.Rules(c) {
		if _, e := b.run(r.Table, append([]string{"-A", r.Chain}, b.ruleArgs(r.Args)...)...); e != nil {
			return e
		}
	}
	for _, x := range b.chains() {
		if _, e := b.run(x[0], append([]string{"-I", x[1], "1"}, b.hook(x[2])...)...); e != nil {
			return e
		}
	}
	return nil
}
func (b IPTables) RemoveHooks() error {
	for _, x := range b.chains() {
		for n := 0; ; n++ {
			if n > 1000 {
				return fmt.Errorf("too many duplicate hooks")
			}
			if _, e := b.run(x[0], append([]string{"-C", x[1]}, b.hook(x[2])...)...); e != nil {
				break
			}
			if _, e := b.run(x[0], append([]string{"-D", x[1]}, b.hook(x[2])...)...); e != nil {
				return e
			}
		}
	}
	return nil
}
func (b IPTables) Remove() error {
	for _, x := range b.chains() {
		if _, e := b.run(x[0], "-S", x[2]); e == nil {
			if e = b.owned(x[0], x[2]); e != nil {
				return e
			}
		}
	}
	if e := b.RemoveHooks(); e != nil {
		return e
	}
	for _, x := range b.chains() {
		if _, e := b.run(x[0], "-S", x[2]); e == nil {
			if _, e = b.run(x[0], "-F", x[2]); e != nil {
				return e
			}
			if _, e = b.run(x[0], "-X", x[2]); e != nil {
				return e
			}
		}
	}
	return nil
}
func (b IPTables) Check(c config.Config) error {
	for _, x := range b.chains() {
		if _, e := b.run(x[0], append([]string{"-C", x[1]}, b.hook(x[2])...)...); e != nil {
			return e
		}
	}
	for _, r := range b.Rules(c) {
		if _, e := b.run(r.Table, append([]string{"-C", r.Chain}, b.ruleArgs(r.Args)...)...); e != nil {
			return e
		}
	}
	return nil
}
