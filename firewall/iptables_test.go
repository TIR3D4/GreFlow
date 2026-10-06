package firewall

import (
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"strings"
	"testing"
)

type fake struct {
	tables map[string]map[string][]string
	fail   string
	failed bool
}

func newFake() *fake {
	f := &fake{tables: map[string]map[string][]string{}}
	for _, x := range chains {
		if f.tables[x[0]] == nil {
			f.tables[x[0]] = map[string][]string{}
		}
		f.tables[x[0]][x[1]] = nil
	}
	return f
}
func (f *fake) Run(_ string, a ...string) (string, error) {
	if f.fail != "" && !f.failed && strings.Contains(strings.Join(a, " "), f.fail) {
		f.failed = true
		return "", fmt.Errorf("injected")
	}
	t := f.tables[a[3]]
	op, ch := a[4], a[5]
	rest := a[6:]
	if op == "-I" {
		rest = rest[1:]
	}
	key := strings.Join(rest, " ")
	v, exists := t[ch]
	switch op {
	case "-N":
		if exists {
			return "", fmt.Errorf("exists")
		}
		t[ch] = nil
	case "-S":
		if !exists {
			return "", fmt.Errorf("absent")
		}
		s := "-N " + ch + "\n"
		for _, r := range v {
			s += "-A " + ch + " " + r + "\n"
		}
		return s, nil
	case "-A":
		t[ch] = append(v, key)
	case "-I":
		t[ch] = append([]string{key}, v...)
	case "-C", "-D":
		for i, r := range v {
			if r == key {
				if op == "-D" {
					t[ch] = append(v[:i], v[i+1:]...)
				}
				return "", nil
			}
		}
		return "", fmt.Errorf("missing rule")
	case "-F":
		if !exists {
			return "", fmt.Errorf("absent")
		}
		t[ch] = nil
	case "-X":
		delete(t, ch)
	default:
		return "", fmt.Errorf("unsupported %v", a)
	}
	return "", nil
}
func cfg() config.Config {
	return config.Config{Role: "entry", Local: "192.0.2.1", Remote: "192.0.2.2", Interface: "greflow0", Network: "10.77.0.0/30", MTU: 1476, Forwards: []config.Forward{{Protocol: "tcp", Public: "443", Destination: "2020"}, {Protocol: "udp", Public: "10000-20000", Destination: "10000-20000"}}}
}
func TestApplyRepairAndOwnership(t *testing.T) {
	f := newFake()
	f.tables["filter"]["FORWARD"] = []string{"-j USER_RULE"}
	b := IPTables{R: f}
	c := cfg()
	for i := 0; i < 3; i++ {
		if e := b.Apply(c); e != nil {
			t.Fatal(e)
		}
		if e := b.Check(c); e != nil {
			t.Fatal(e)
		}
	}
	if len(f.tables["filter"]["FORWARD"]) != 2 {
		t.Fatal("duplicates")
	}
	f.tables["nat"]["GREFLOW_PREROUTING"] = nil
	f.tables["filter"]["FORWARD"] = append(f.tables["filter"]["FORWARD"], strings.Join(hook("GREFLOW_FORWARD"), " "))
	if e := b.Apply(c); e != nil {
		t.Fatal(e)
	}
	if len(f.tables["filter"]["FORWARD"]) != 2 {
		t.Fatal("repair duplicate")
	}
	if e := b.Remove(); e != nil {
		t.Fatal(e)
	}
	if strings.Join(f.tables["filter"]["FORWARD"], "") != "-j USER_RULE" {
		t.Fatal("user firewall altered")
	}
}
func TestRefuseForeignRules(t *testing.T) {
	f := newFake()
	f.tables["filter"]["GREFLOW_FORWARD"] = []string{"-j FOREIGN"}
	b := IPTables{R: f}
	if b.Apply(cfg()) == nil {
		t.Fatal("accepted foreign rules")
	}
	if b.Remove() == nil {
		t.Fatal("deleted foreign rules")
	}
}
func TestScopedGeneration(t *testing.T) {
	for _, r := range Rules(cfg()) {
		s := strings.Join(r.Args, " ")
		if r.Chain == "GREFLOW_PREROUTING" && !strings.Contains(s, "-d 192.0.2.1") {
			t.Fatal(s)
		}
		if strings.Contains(s, "--dport 10000:20000") && strings.Contains(s, "DNAT") && !strings.HasSuffix(s, "--to-destination 10.77.0.2") {
			t.Fatal("range must preserve ports", s)
		}
	}
	c := cfg()
	c.Role = "exit"
	for _, r := range Rules(c) {
		if r.Table == "nat" {
			t.Fatal("exit has NAT")
		}
	}
}

func TestSharedDestinationDoesNotDuplicateRules(t *testing.T) {
	c := cfg()
	c.Forwards = append(c.Forwards, config.Forward{Protocol: "tcp", Public: "8443", Destination: "2020"})
	n := 0
	for _, r := range Rules(c) {
		if r.Chain == "GREFLOW_POSTROUTING" && strings.Contains(strings.Join(r.Args, " "), "-p tcp") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected one shared SNAT rule, got %d", n)
	}
}

func TestNamedChainsAreIsolated(t *testing.T) {
	f := newFake()
	legacy := IPTables{R: f}
	second := IPTables{R: f, Name: "exit2"}
	third := IPTables{R: f, Name: "exit3"}
	c := cfg()
	if e := legacy.Apply(c); e != nil {
		t.Fatal(e)
	}
	if e := second.Apply(c); e != nil {
		t.Fatal(e)
	}
	if e := third.Apply(c); e != nil {
		t.Fatal(e)
	}
	for _, b := range []IPTables{legacy, second, third} {
		for _, x := range b.chains() {
			if len(x[2]) > 28 {
				t.Fatal(x[2])
			}
			if e := b.Check(c); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := second.Remove(); e != nil {
		t.Fatal(e)
	}
	if e := legacy.Check(c); e != nil {
		t.Fatal("legacy removed", e)
	}
	if e := third.Check(c); e != nil {
		t.Fatal("other tunnel removed", e)
	}
	if e := second.Apply(c); e != nil {
		t.Fatal(e)
	}
	if e := second.Apply(c); e != nil {
		t.Fatal(e)
	}
	hooks := 0
	for _, rule := range f.tables["filter"]["FORWARD"] {
		if strings.Contains(rule, "greflow:exit2:hook") {
			hooks++
		}
	}
	if hooks != 1 {
		t.Fatal("duplicate instance hook", hooks)
	}
}
