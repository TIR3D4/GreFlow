package tunnel

import (
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"strings"
	"testing"
)

type recorder struct {
	alias  string
	exists bool
	calls  []string
}

func (r *recorder) Run(n string, a ...string) (string, error) {
	s := strings.Join(a, " ")
	r.calls = append(r.calls, n+" "+s)
	if strings.HasPrefix(s, "-j -s link show") {
		if !r.exists {
			return "", fmt.Errorf("missing")
		}
		return `[{"ifalias":"` + r.alias + `","flags":["UP"],"mtu":1476}]`, nil
	}
	if strings.HasPrefix(s, "tunnel add") {
		r.exists = true
	}
	if strings.Contains(s, "alias ") {
		r.alias = Alias
	}
	if strings.HasPrefix(s, "link delete") {
		r.exists = false
	}
	return "[]", nil
}
func TestRefuseUnowned(t *testing.T) {
	r := &recorder{exists: true, alias: "docker"}
	c := config.Config{Interface: "greflow0"}
	if Apply(r, c) == nil || Down(r, c) == nil {
		t.Fatal("accepted unowned interface")
	}
	for _, s := range r.calls {
		if strings.Contains(s, "link delete") {
			t.Fatal("deleted unrelated interface")
		}
	}
}
func TestCreateAndDestroy(t *testing.T) {
	r := &recorder{}
	c := config.Config{Role: "entry", Local: "192.0.2.1", Remote: "192.0.2.2", Interface: "greflow0", Network: "10.77.0.0/30", MTU: 1476}
	if e := Apply(r, c); e != nil {
		t.Fatal(e)
	}
	s := strings.Join(r.calls, "\n")
	for _, want := range []string{"mode gre local 192.0.2.1 remote 192.0.2.2 ttl 255", "alias greflow:v0.1", "10.77.0.1/30", "mtu 1476"} {
		if !strings.Contains(s, want) {
			t.Fatal(want, s)
		}
	}
	if e := Down(r, c); e != nil {
		t.Fatal(e)
	}
	if r.exists {
		t.Fatal("still exists")
	}
}
