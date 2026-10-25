package console

import (
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"testing"
	"time"
)

func TestUnperformedChecksAreNotExpiredResults(t *testing.T) {
	tunnel := protocol.Tunnel{Checks: []protocol.Evidence{{Kind: "gre_peer", Status: "unknown", ExpiresAt: time.Now().Add(-time.Hour)}}}
	if got := incidentState(tunnel); got != "unknown" {
		t.Fatalf("unperformed check became %s", got)
	}
	tunnel.Checks[0].Status = "ok"
	if got := incidentState(tunnel); got != "stale" {
		t.Fatalf("old positive check became %s", got)
	}
	tunnel.Checks[0].Status = "failed"
	if got := incidentState(tunnel); got != "stale" {
		t.Fatalf("old failed check became %s", got)
	}
}
