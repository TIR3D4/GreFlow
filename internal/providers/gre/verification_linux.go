package gre

import (
	"encoding/json"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"os"
	"time"
)

type verification struct {
	Revision   string    `json:"revision"`
	Generation string    `json:"generation"`
	ObservedAt time.Time `json:"observed_at"`
}

func verificationPath(m manager.Manager) string {
	return m.Path("/var/lib/greflow/console-verifications/" + m.Label() + ".json")
}
func recordVerification(m manager.Manager, s manager.State) error {
	c, e := config.Load(m.ConfigPath())
	if e != nil {
		return e
	}
	return config.Save(verificationPath(m), verification{protocol.Revision(c), s.Since.Format(time.RFC3339Nano), time.Now().UTC()})
}
func verificationEvidence(m manager.Manager, t protocol.Tunnel, active bool) protocol.Evidence {
	e := protocol.Evidence{Kind: "user_verified", Status: "unknown", Detail: "No timestamped client confirmation for this configuration and generation", ObservedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(180 * time.Second)}
	var v verification
	b, err := os.ReadFile(verificationPath(m))
	if err == nil && json.Unmarshal(b, &v) == nil && active && !t.Drift && v.Revision == t.Revision && v.Generation == t.Generation && !v.ObservedAt.IsZero() {
		e.Status = "ok"
		e.Detail = "Explicit user statement; not inferred from probes. Timestamp is never refreshed by discovery."
		e.ObservedAt = v.ObservedAt
		e.ExpiresAt = v.ObservedAt.Add(180 * time.Second)
	}
	return e
}
