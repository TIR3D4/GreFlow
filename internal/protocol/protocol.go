// Package protocol is the versioned, platform-independent dispatcher contract.
package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"regexp"
	"time"
)

const Version = 1

type Evidence struct {
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
	ObservedAt time.Time `json:"observed_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Detail     string    `json:"detail,omitempty"`
	LatencyMS  *float64  `json:"latency_ms,omitempty"`
}
type Tunnel struct {
	Name       string         `json:"name"`
	Revision   string         `json:"revision"`
	Desired    config.Config  `json:"desired"`
	Applied    *config.Config `json:"applied,omitempty"`
	Checks     []Evidence     `json:"checks"`
	RX         *uint64        `json:"rx_bytes,omitempty"`
	TX         *uint64        `json:"tx_bytes,omitempty"`
	Generation string         `json:"generation"`
	Drift      bool           `json:"drift"`
}
type Inventory struct {
	Schedules  []ScheduleObservation `json:"schedules,omitempty"`
	Version    int                   `json:"version"`
	HostID     string                `json:"host_id"`
	Addresses  []string              `json:"addresses"`
	Tunnels    []Tunnel              `json:"tunnels"`
	ObservedAt time.Time             `json:"observed_at"`
}

type ScheduleRun struct {
	At     time.Time `json:"at"`
	State  string    `json:"state"`
	Detail string    `json:"detail,omitempty"`
}
type ScheduleObservation struct {
	Schedule   Schedule      `json:"schedule"`
	TimerState string        `json:"timer_state"`
	NextRunAt  *time.Time    `json:"next_run_at,omitempty"`
	ObservedAt time.Time     `json:"observed_at"`
	Runs       []ScheduleRun `json:"runs"`
}
type Schedule struct {
	MaintenanceWindows []MaintenanceWindow `json:"maintenance_windows,omitempty"`
	FailureThreshold   int                 `json:"failure_threshold,omitempty"`
	CooldownMinutes    int                 `json:"cooldown_minutes,omitempty"`
	MaxPerHour         int                 `json:"max_per_hour,omitempty"`
	Maintenance        bool                `json:"maintenance,omitempty"`
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Action             string              `json:"action"`
	Minutes            int                 `json:"minutes"`
	Enabled            bool                `json:"enabled"`
	Timezone           string              `json:"timezone"`
}
type Request struct {
	Version        int             `json:"version"`
	ID             string          `json:"id"`
	Action         string          `json:"action"`
	Name           string          `json:"name"`
	Revision       string          `json:"revision,omitempty"`
	Config         *config.Config  `json:"config,omitempty"`
	Target         string          `json:"target,omitempty"`
	TargetRevision string          `json:"target_revision,omitempty"`
	Forward        *config.Forward `json:"forward,omitempty"`
	Destination    string          `json:"destination,omitempty"`
	Schedule       *Schedule       `json:"schedule,omitempty"`
}
type Response struct {
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Version     int          `json:"version"`
	State       string       `json:"state"`
	Error       string       `json:"error,omitempty"`
	Inventory   *Inventory   `json:"inventory,omitempty"`
	Tunnel      *Tunnel      `json:"tunnel,omitempty"`
	Logs        string       `json:"logs,omitempty"`
}

// Diagnostic records control-plane steps separately from tunnel evidence.
type Diagnostic struct {
	At       time.Time `json:"at"`
	Stage    string    `json:"stage"`
	State    string    `json:"state"`
	Code     string    `json:"code,omitempty"`
	Message  string    `json:"message"`
	ExitCode *int      `json:"exit_code,omitempty"`
}

func Revision(c config.Config) string {
	b, _ := json.Marshal(c)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

var IDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func (r Request) Validate() error {
	if r.Version != Version || !IDPattern.MatchString(r.ID) {
		return fmt.Errorf("invalid protocol version or request ID")
	}
	if e := config.ValidName(r.Name); e != nil {
		return e
	}
	if e := config.ValidName(r.Target); e != nil {
		return e
	}
	switch r.Action {
	case "discover", "observe", "probe", "logs", "plan", "create", "start", "stop", "restart", "remove", "mappings", "transfer", "recover", "schedule", "unschedule", "verify":
	default:
		return fmt.Errorf("unsupported action")
	}
	if r.Config != nil {
		if e := r.Config.Validate(); e != nil {
			return e
		}
	}
	if r.Action == "create" || r.Action == "plan" || r.Action == "mappings" {
		if r.Config == nil {
			return fmt.Errorf("config required")
		}
	}
	if r.Action == "transfer" {
		if r.Forward == nil || r.Target == r.Name {
			return fmt.Errorf("distinct target and mapping required")
		}
		f := *r.Forward
		c := config.Config{Role: "entry", Local: "192.0.2.1", Remote: "192.0.2.2", Interface: "gf", Network: "10.0.0.0/30", MTU: 1476, Forwards: []config.Forward{f}}
		if e := c.Validate(); e != nil {
			return e
		}
		if _, _, e := config.Range(r.Destination); e != nil {
			return e
		}
	}
	if r.Action == "schedule" || r.Action == "unschedule" {
		s := r.Schedule
		if s == nil || !IDPattern.MatchString(s.ID) {
			return fmt.Errorf("invalid schedule")
		}
		if r.Action == "schedule" {
			if e := s.ValidateWindows(); e != nil {
				return e
			}
			if s.Minutes < 1 || s.Minutes > 10080 || (s.Action != "probe" && s.Action != "restart" && s.Action != "recovery") {
				return fmt.Errorf("invalid schedule interval/action")
			}
			if s.Action == "recovery" && (s.FailureThreshold < 2 || s.FailureThreshold > 20 || s.CooldownMinutes < 5 || s.CooldownMinutes > 1440 || s.MaxPerHour < 1 || s.MaxPerHour > 6) {
				return fmt.Errorf("recovery requires threshold 2..20, cooldown 5..1440 minutes and hourly cap 1..6")
			}
			if _, e := time.LoadLocation(s.Timezone); e != nil {
				return e
			}
		}
	}
	return nil
}
