package manager

import (
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type runner struct {
	values map[string]string
	fail   string
}

func (r *runner) Run(n string, a ...string) (string, error) {
	if n == "sysctl" {
		if a[0] == "-n" {
			return r.values[a[1]], nil
		}
		if a[0] == "-w" {
			p := strings.SplitN(a[1], "=", 2)
			r.values[p[0]] = p[1]
			return "", nil
		}
	}
	if n == "iptables-save" {
		return "# snapshot", nil
	}
	if n == "ip" {
		s := strings.Join(a, " ")
		if s == "-j route get 192.0.2.2" {
			return `[{"dev":"eth0"}]`, nil
		}
		if s == "-j -4 addr show" {
			return `[{"addr_info":[{"local":"192.0.2.1"}]}]`, nil
		}
		if strings.HasPrefix(s, "-j -s link show") {
			return "", fmt.Errorf("absent")
		}
		if strings.HasPrefix(s, "tunnel add") {
			return "", fmt.Errorf("injected tunnel failure")
		}
	}
	if n == "iptables" {
		return "", fmt.Errorf("absent")
	}
	return "[]", nil
}
func TestRestoreRespectsSubsequentUserChanges(t *testing.T) {
	r := &runner{values: map[string]string{"one": "1", "two": "9"}}
	m := Manager{Root: t.TempDir(), R: r}
	s := State{Original: map[string]string{"one": "0", "two": "0"}, Written: map[string]string{"one": "1", "two": "1"}}
	if e := m.restore(s); e != nil {
		t.Fatal(e)
	}
	if r.values["one"] != "0" || r.values["two"] != "9" {
		t.Fatal(r.values)
	}
}
func TestBackupAndFailedApply(t *testing.T) {
	root := t.TempDir()
	r := &runner{values: map[string]string{}}
	m := Manager{Root: root, R: r}
	c := config.Config{Role: "entry", Local: "192.0.2.1", Remote: "192.0.2.2", Interface: "greflow0", Network: "10.77.0.0/30", MTU: 1476}
	if e := m.Apply(c); e == nil || !strings.Contains(e.Error(), "rollback") {
		t.Fatal(e)
	}
	s, e := m.State()
	if e != nil || s.Active || s.Config != nil {
		t.Fatal(s, e)
	}
	files, _ := filepath.Glob(m.Path("/var/lib/greflow/backups/*/iptables.txt"))
	if len(files) != 1 {
		t.Fatal(files)
	}
	if _, e = os.Stat(m.ConfigPath()); !os.IsNotExist(e) {
		t.Fatal("failed config persisted", e)
	}
}
func TestLock(t *testing.T) {
	m := Manager{Root: t.TempDir()}
	unlock, e := m.Lock()
	if e != nil {
		t.Fatal(e)
	}
	defer unlock()
	if u, e := m.Lock(); e == nil {
		u()
		t.Fatal("concurrent operation allowed")
	}
}
