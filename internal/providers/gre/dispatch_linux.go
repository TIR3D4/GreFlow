package gre

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/firewall"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type journal struct {
	Request  protocol.Request         `json:"request"`
	Before   map[string]config.Config `json:"before"`
	After    map[string]config.Config `json:"after"`
	Response protocol.Response        `json:"response"`
}

func revision(m manager.Manager, want string) error {
	c, e := config.Load(m.ConfigPath())
	if e != nil {
		return e
	}
	if want == "" || protocol.Revision(c) != want {
		return fmt.Errorf("REVISION_CONFLICT: rediscover and review plan")
	}
	return nil
}
func saveMapping(m manager.Manager, c config.Config) error {
	s, e := m.State()
	if e != nil {
		return e
	}
	s.Config = &c
	s.Verified = false
	if e = config.Save(m.ConfigPath(), c); e != nil {
		return e
	}
	return config.Save(m.StatePath(), s)
}
func sameLink(a, b config.Config) bool {
	a.Forwards = nil
	b.Forwards = nil
	return reflect.DeepEqual(a, b)
}
func preflight(m manager.Manager, c config.Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	if e := m.ValidatePeers(c); e != nil {
		return e
	}
	v, e := m.R.Run("ip", "-j", "-4", "addr", "show")
	if e != nil {
		return e
	}
	var links []struct {
		AddrInfo []struct {
			Local string `json:"local"`
		} `json:"addr_info"`
	}
	if e = json.Unmarshal([]byte(v), &links); e != nil {
		return e
	}
	assigned := false
	for _, l := range links {
		for _, a := range l.AddrInfo {
			if a.Local == c.Local {
				assigned = true
			}
		}
	}
	if !assigned {
		return fmt.Errorf("local IPv4 is not assigned; NAT endpoints unsupported")
	}
	v, e = m.R.Run("ip", "-j", "route", "get", c.Remote)
	if e != nil {
		return e
	}
	var routes []struct {
		Dev string `json:"dev"`
	}
	if e = json.Unmarshal([]byte(v), &routes); e != nil {
		return e
	}
	if len(routes) == 0 || routes[0].Dev == c.Interface {
		return fmt.Errorf("invalid underlay route")
	}
	return nil
}

// Dispatch is reachable only through a root-owned helper over authenticated SSH.
// All remote data is typed; no request can name an executable or arbitrary path.
func Dispatch(m manager.Manager, r protocol.Request) protocol.Response {
	out := protocol.Response{Version: 1, State: "failed"}
	fail := func(e error) protocol.Response { out.Error = e.Error(); return out }
	if e := r.Validate(); e != nil {
		return fail(e)
	}
	m.Name = r.Name
	if r.Action == "discover" {
		i, e := Discover(m)
		if e != nil {
			return fail(e)
		}
		out.State = "succeeded"
		out.Inventory = &i
		return out
	}
	if r.Action == "observe" || r.Action == "probe" {
		t, e := Observe(m, r.Action == "probe")
		if e != nil {
			return fail(e)
		}
		out.State = "succeeded"
		out.Tunnel = &t
		return out
	}
	if r.Action == "logs" {
		b, e := os.ReadFile(m.Path("/var/log/greflow/events.jsonl"))
		if e != nil && !os.IsNotExist(e) {
			return fail(e)
		}
		lines := strings.Split(string(b), "\n")
		if len(lines) > 1000 {
			lines = lines[len(lines)-1000:]
		}
		var kept []string
		for _, line := range lines {
			var row map[string]any
			if json.Unmarshal([]byte(line), &row) == nil && row["tunnel"] == m.Label() {
				kept = append(kept, line)
			}
		}
		out.Logs = strings.Join(kept, "\n")
		out.State = "succeeded"
		return out
	}
	unlock, e := m.Lock()
	if e != nil {
		return fail(e)
	}
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()
	jp := m.Path("/var/lib/greflow/console-journal/" + r.ID + ".json")
	if b, e := os.ReadFile(jp); e == nil {
		var previous journal
		if e = json.Unmarshal(b, &previous); e != nil {
			return fail(e)
		}
		if !reflect.DeepEqual(previous.Request, r) {
			return fail(fmt.Errorf("REQUEST_ID_CONFLICT"))
		}
		if previous.Response.State == "running" {
			previous.Response.State = "uncertain"
			previous.Response.Error = "interrupted request; recover journal before retry"
		}
		return previous.Response
	}
	pending, _ := filepath.Glob(m.Path("/var/lib/greflow/console-journal/*.json"))
	for _, p := range pending {
		b, e := os.ReadFile(p)
		if e != nil {
			return fail(e)
		}
		var j journal
		if e = json.Unmarshal(b, &j); e != nil {
			return fail(e)
		}
		if j.Response.State == "running" || j.Response.State == "rollback_failed" || j.Response.State == "uncertain" {
			if r.Action != "recover" {
				return fail(fmt.Errorf("UNRESOLVED_JOURNAL: %s; use recover", filepath.Base(p)))
			}
			if len(j.Before) == 0 {
				return fail(fmt.Errorf("lifecycle interrupted; manual inspection required: %s", filepath.Base(p)))
			}
			if e = restoreMappings(m, j); e != nil {
				j.Response.State = "rollback_failed"
				j.Response.Error = e.Error()
			} else {
				j.Response.State = "rolled_back"
				j.Response.Error = ""
			}
			if e = config.Save(p, j); e != nil {
				return fail(e)
			}
			if j.Response.State == "rollback_failed" {
				return j.Response
			}
		}
	}
	if r.Action == "recover" {
		out.State = "succeeded"
		return out
	}
	if r.Action == "plan" {
		if e = preflight(m, *r.Config); e != nil {
			return fail(e)
		}
		if _, e = os.Stat(m.ConfigPath()); e == nil {
			return fail(fmt.Errorf("name already reserved"))
		}
		out.State = "succeeded"
		return out
	}
	if r.Action != "create" && r.Action != "schedule" && r.Action != "unschedule" {
		if e = revision(m, r.Revision); e != nil {
			return fail(e)
		}
	}
	j := journal{Request: r, Before: map[string]config.Config{}, After: map[string]config.Config{}, Response: protocol.Response{Version: 1, State: "running"}}
	if r.Action == "mappings" || r.Action == "transfer" {
		old, err := config.Load(m.ConfigPath())
		if err != nil {
			return fail(err)
		}
		s, err := m.State()
		if err != nil || !s.Active {
			return fail(fmt.Errorf("active source required"))
		}
		j.Before[m.Name] = old
		if err = (firewall.IPTables{R: m.R, Name: m.Name}).Check(old); err != nil {
			return fail(fmt.Errorf("SOURCE_DRIFT: rule preflight failed: %w", err))
		}
		if r.Action == "mappings" {
			if !sameLink(old, *r.Config) {
				return fail(fmt.Errorf("only forwards can change in mapping plan"))
			}
			if e = m.ValidatePeers(*r.Config); e != nil {
				return fail(e)
			}
			j.After[m.Name] = *r.Config
		} else {
			target := m
			target.Name = r.Target
			if e = revision(target, r.TargetRevision); e != nil {
				return fail(e)
			}
			tc, err := config.Load(target.ConfigPath())
			if err != nil {
				return fail(err)
			}
			ts, err := target.State()
			if err != nil || !ts.Active {
				return fail(fmt.Errorf("active target required"))
			}
			if old.Role != "entry" || tc.Role != "entry" || tc.Local != old.Local {
				return fail(fmt.Errorf("transfer requires same entry endpoint"))
			}
			if err = (firewall.IPTables{R: m.R, Name: target.Name}).Check(tc); err != nil {
				return fail(fmt.Errorf("TARGET_DRIFT: rule preflight failed: %w", err))
			}
			if r.Forward.Protocol != "tcp" || strings.Contains(r.Forward.Public, "-") || strings.Contains(r.Destination, "-") {
				return fail(fmt.Errorf("transfer requires single TCP mapping with destination preflight"))
			}
			if e = Probe(tc, r.Destination); e != nil {
				return fail(fmt.Errorf("PREFLIGHT_FAILED: old mapping unchanged: %w", e))
			}
			next := old
			next.Forwards = nil
			found := false
			for _, f := range old.Forwards {
				if f == *r.Forward {
					found = true
				} else {
					next.Forwards = append(next.Forwards, f)
				}
			}
			if !found {
				return fail(fmt.Errorf("source mapping missing"))
			}
			nt := tc
			nt.Forwards = append(append([]config.Forward{}, tc.Forwards...), config.Forward{Protocol: r.Forward.Protocol, Public: r.Forward.Public, Destination: r.Destination})
			if e = nt.Validate(); e != nil {
				return fail(e)
			}
			j.Before[target.Name] = tc
			j.After[m.Name] = next
			j.After[target.Name] = nt
		}
	}
	if r.Action == "create" {
		if _, e = os.Stat(m.ConfigPath()); !os.IsNotExist(e) {
			return fail(fmt.Errorf("name exists or unreadable"))
		}
		if e = preflight(m, *r.Config); e != nil {
			return fail(e)
		}
	}
	if e = config.Save(jp, j); e != nil {
		return fail(e)
	}
	switch r.Action {
	case "mappings", "transfer":
		names := []string{m.Name}
		if r.Action == "transfer" {
			names = append(names, r.Target)
		}
		for _, name := range names {
			mm := m
			mm.Name = name
			if e = (firewall.IPTables{R: m.R, Name: name}).Update(j.Before[name], j.After[name]); e != nil {
				break
			}
			if e = saveMapping(mm, j.After[name]); e != nil {
				break
			}
		}
		if e == nil && r.Action == "transfer" {
			e = Probe(j.After[r.Target], r.Destination)
		}
		if e != nil {
			cause := e
			if re := restoreMappings(m, j); re != nil {
				out.State = "rollback_failed"
				out.Error = fmt.Sprintf("%v; recovery: %v", cause, re)
			} else {
				out.State = "rolled_back"
				out.Error = cause.Error()
			}
		} else {
			out.State = "succeeded"
		}
	case "create":
		e = m.Apply(*r.Config)
		if e == nil && m.Root == "" {
			e = m.Persist()
		}
	case "verify":
		s, err := m.State()
		e = err
		if e == nil {
			if !s.Active {
				e = fmt.Errorf("inactive tunnel")
			} else {
				s.Verified = true
				e = recordVerification(m, s)
				if e == nil {
					e = config.Save(m.StatePath(), s)
				}
			}
		}
	case "start", "stop", "restart":
		// systemd invokes the CLI, which acquires the same host lock.
		unlock()
		locked = false
		if m.Root != "" {
			e = fmt.Errorf("systemd unavailable with test root")
		} else {
			_, e = m.R.Run("systemctl", r.Action, m.Service())
		}
	case "remove":
		// Explicitly preserve the installed CLI even when removing the last instance.
		e = m.Down()
		if e == nil && m.Root == "" {
			_, e = m.R.Run("systemctl", "disable", m.Service())
			if e == nil {
				e = os.Remove("/etc/systemd/system/" + m.Service())
				if os.IsNotExist(e) {
					e = nil
				}
			}
			if e == nil {
				_, e = m.R.Run("systemctl", "daemon-reload")
			}
		}
		if e == nil {
			e = os.Remove(m.ConfigPath())
			if e == nil {
				e = os.Remove(m.StatePath())
				if os.IsNotExist(e) {
					e = nil
				}
			}
		}
	case "schedule", "unschedule":
		e = Schedule(m, r)
	}
	if r.Action != "mappings" && r.Action != "transfer" {
		if e == nil {
			out.State = "succeeded"
		} else {
			out.State = "uncertain"
			out.Error = e.Error()
		}
	}
	if out.State == "succeeded" && r.Action != "remove" && r.Action != "unschedule" {
		t, err := Observe(m, false)
		if err == nil {
			out.Tunnel = &t
			if r.Action == "create" || r.Action == "start" || r.Action == "restart" {
				for _, check := range t.Checks {
					if (check.Kind == "interface" || check.Kind == "rules") && check.Status != "ok" {
						out.State = "uncertain"
						out.Error = "operation returned but owned interface/rule readback failed"
					}
				}
			}
		} else if r.Action != "schedule" {
			out.State = "uncertain"
			out.Error = err.Error()
		}
	}
	j.Response = out
	if e = config.Save(jp, j); e != nil {
		out.State = "uncertain"
		out.Error = "journal completion could not be persisted"
	}
	return out
}
func restoreMappings(m manager.Manager, j journal) error {
	var errs []error
	// Remove target additions before restoring the old source reservation.
	names := []string{j.Request.Name}
	if j.Request.Action == "transfer" {
		names = []string{j.Request.Target, j.Request.Name}
	}
	for _, name := range names {
		old, ok := j.Before[name]
		if !ok {
			continue
		}
		mm := m
		mm.Name = name
		next := j.After[name]
		current, e := config.Load(mm.ConfigPath())
		if e != nil {
			return e
		}
		revision := protocol.Revision(current)
		if revision != protocol.Revision(old) && revision != protocol.Revision(next) {
			return fmt.Errorf("RECOVERY_REVISION_CONFLICT: %s changed outside journal; inspect manually", mm.Label())
		}
		errs = append(errs, (firewall.IPTables{R: m.R, Name: name}).Update(next, old))
		errs = append(errs, saveMapping(mm, old))
	}
	return errors.Join(errs...)
}

func Schedule(m manager.Manager, r protocol.Request) error {
	if m.Root != "" {
		return fmt.Errorf("systemd scheduling requires live Linux host")
	}
	s := r.Schedule
	if r.Action == "schedule" {
		if _, e := config.Load(m.ConfigPath()); e != nil {
			return e
		}
		s.Name = m.Name
	}
	name := "greflow-console-" + s.ID
	base := "/etc/systemd/system/" + name
	if r.Action == "unschedule" {
		if _, e := m.R.Run("systemctl", "disable", "--now", name+".timer"); e != nil {
			return e
		}
		for _, p := range []string{base + ".timer", base + ".service", "/etc/greflow/console-schedules/" + s.ID + ".json"} {
			if e := os.Remove(p); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
		_, e := m.R.Run("systemctl", "daemon-reload")
		return e
	}
	request := protocol.Request{Version: 1, Action: s.Action, Name: m.Name, Schedule: s}
	if e := config.Save("/etc/greflow/console-schedules/"+s.ID+".json", request); e != nil {
		return e
	}
	unit := "[Unit]\nDescription=GreFlow Console owned schedule\n[Service]\nType=oneshot\nExecStart=/usr/local/bin/greflow-agent schedule-run " + s.ID + "\nTimeoutStartSec=180\n"
	timer := fmt.Sprintf("[Unit]\nDescription=GreFlow Console owned timer\n[Timer]\nOnBootSec=%dm\nOnUnitActiveSec=%dm\nRandomizedDelaySec=15\nUnit=%s.service\n[Install]\nWantedBy=timers.target\n", s.Minutes, s.Minutes, name)
	if e := os.WriteFile(base+".service", []byte(unit), 0644); e != nil {
		return e
	}
	if e := os.WriteFile(base+".timer", []byte(timer), 0644); e != nil {
		return e
	}
	if _, e := m.R.Run("systemctl", "daemon-reload"); e != nil {
		return e
	}
	if s.Enabled {
		_, e := m.R.Run("systemctl", "enable", "--now", name+".timer")
		return e
	}
	_, e := m.R.Run("systemctl", "disable", "--now", name+".timer")
	return e
}
