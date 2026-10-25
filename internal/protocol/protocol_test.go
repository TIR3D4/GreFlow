package protocol

import (
	"strings"
	"testing"
)

func TestDispatcherRejectsUntrustedFields(t *testing.T) {
	for _, r := range []Request{{Version: 1, ID: strings.Repeat("a", 32), Action: "shell"}, {Version: 1, ID: "../file", Action: "probe"}, {Version: 1, ID: strings.Repeat("a", 32), Action: "probe", Name: "x;reboot"}, {Version: 1, ID: strings.Repeat("a", 32), Action: "create"}, {Version: 1, ID: strings.Repeat("a", 32), Action: "schedule", Schedule: &Schedule{ID: strings.Repeat("a", 32), Action: "restart", Minutes: 0}}} {
		if r.Validate() == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
}
