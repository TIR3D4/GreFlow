package main

import (
	"encoding/json"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"github.com/TIR3D4/GreFlow/internal/providers/gre"
	"os"
	"time"
)

type recoveryState struct {
	Failures   int         `json:"failures"`
	Attempts   []time.Time `json:"attempts"`
	LastResult string      `json:"last_result"`
}

func recoveryNeeded(t protocol.Tunnel) bool {
	failed := false
	tcpOK := false
	for _, e := range t.Checks {
		if e.Kind == "user_verified" {
			continue
		}
		if e.ExpiresAt.Before(time.Now()) {
			return false
		}
		if len(e.Kind) > 12 && e.Kind[:12] == "destination:" {
			if e.Status == "ok" {
				tcpOK = true
			}
			if e.Status == "failed" {
				failed = true
			}
		}
		if e.Kind == "interface" && e.Status == "failed" {
			failed = true
		}
	}
	return failed && !tcpOK
}
func runRecovery(m manager.Manager, r protocol.Request, id string) (protocol.Response, error) {
	s := r.Schedule
	if s == nil || s.FailureThreshold < 2 || s.CooldownMinutes < 5 || s.MaxPerHour < 1 {
		return protocol.Response{}, fmt.Errorf("invalid recovery policy")
	}
	result := protocol.Response{Version: 1, State: "succeeded"}
	maintenance, e := s.InMaintenance(time.Now())
	if e != nil {
		return result, e
	}
	if maintenance {
		result.Logs = "maintenance enabled; no probe or restart"
		return result, nil
	}
	// Share the CLI host lock while reserving budgets across every recovery timer.
	unlock, e := m.Lock()
	if e != nil {
		return result, e
	}
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()
	t, e := gre.Observe(m, true)
	if e != nil {
		return result, e
	}
	result.Tunnel = &t
	path := m.Path("/var/lib/greflow/console-recovery/" + id + ".json")
	var state recoveryState
	b, e := os.ReadFile(path)
	if e == nil {
		if e = json.Unmarshal(b, &state); e != nil {
			return result, e
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	if !recoveryNeeded(t) {
		state.Failures = 0
		state.LastResult = "no restart: healthy TCP, no actionable failure, or unsupported evidence"
		result.Logs = state.LastResult
		return result, config.Save(path, state)
	}
	state.Failures++
	now := time.Now().UTC()
	attempts := []time.Time{}
	for _, at := range state.Attempts {
		if now.Sub(at) < time.Hour {
			attempts = append(attempts, at)
		}
	}
	state.Attempts = attempts
	globalPath := m.Path("/var/lib/greflow/console-recovery/host-budget.json")
	var global recoveryState
	if b, err := os.ReadFile(globalPath); err == nil {
		if err = json.Unmarshal(b, &global); err != nil {
			return result, err
		}
	} else if !os.IsNotExist(err) {
		return result, err
	}
	globalAttempts := []time.Time{}
	for _, at := range global.Attempts {
		if now.Sub(at) < time.Hour {
			globalAttempts = append(globalAttempts, at)
		}
	}
	global.Attempts = globalAttempts
	hostCooldown := len(globalAttempts) > 0 && now.Sub(globalAttempts[len(globalAttempts)-1]) < 5*time.Minute
	cooldown := len(attempts) > 0 && now.Sub(attempts[len(attempts)-1]) < time.Duration(s.CooldownMinutes)*time.Minute
	if state.Failures < s.FailureThreshold || cooldown || len(attempts) >= s.MaxPerHour || hostCooldown || len(globalAttempts) >= 6 {
		state.LastResult = "restart withheld: threshold, cooldown or hourly cap"
		result.Logs = state.LastResult
		return result, config.Save(path, state)
	}
	// Persist the attempt before invoking systemd; a crash consumes this attempt.
	global.Attempts = append(global.Attempts, now)
	if e = config.Save(globalPath, global); e != nil {
		return result, e
	}
	state.Attempts = append(state.Attempts, now)
	state.LastResult = "restart attempt reserved"
	if e = config.Save(path, state); e != nil {
		return result, e
	}
	r.Action = "restart"
	r.Schedule = nil
	unlock()
	locked = false
	result = gre.Dispatch(m, r)
	state.LastResult = result.State
	if result.State == "succeeded" {
		state.Failures = 0
	}
	return result, config.Save(path, state)
}
