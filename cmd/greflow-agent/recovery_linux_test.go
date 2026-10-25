package main

import (
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"testing"
	"time"
)

func TestRecoveryDoesNotRestartForPingWhenTCPWorks(t *testing.T) {
	now := time.Now()
	v := protocol.Tunnel{Checks: []protocol.Evidence{{Kind: "gre_peer", Status: "failed", ExpiresAt: now.Add(time.Minute)}, {Kind: "destination:tcp:443", Status: "ok", ExpiresAt: now.Add(time.Minute)}}}
	if recoveryNeeded(v) {
		t.Fatal("ping-only restart")
	}
	v.Checks[1].Status = "failed"
	if !recoveryNeeded(v) {
		t.Fatal("missing actionable failure")
	}
	v.Checks[1].ExpiresAt = now.Add(-time.Second)
	if recoveryNeeded(v) {
		t.Fatal("stale evidence triggers restart")
	}
}
