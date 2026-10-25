package gre

import (
	"encoding/json"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func RecordScheduleRun(m manager.Manager, id string, res protocol.Response) error {
	if !protocol.IDPattern.MatchString(id) {
		return fmt.Errorf("invalid schedule ID")
	}
	path := m.Path("/var/lib/greflow/console-schedule-runs/" + id + ".json")
	runs := []protocol.ScheduleRun{}
	b, e := os.ReadFile(path)
	if e == nil {
		if e = json.Unmarshal(b, &runs); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	detail := res.Logs
	if res.Error != "" {
		detail = res.Error
	}
	if len(detail) > 4096 {
		detail = detail[:4096]
	}
	runs = append(runs, protocol.ScheduleRun{At: time.Now().UTC(), State: res.State, Detail: detail})
	if len(runs) > 50 {
		runs = runs[len(runs)-50:]
	}
	return config.Save(path, runs)
}

func ObserveSchedules(m manager.Manager) []protocol.ScheduleObservation {
	out := []protocol.ScheduleObservation{}
	files, _ := filepath.Glob(m.Path("/etc/greflow/console-schedules/*.json"))
	next := map[string]time.Time{}
	// JSON timer output is optional on older systemd. Unknown is never estimated.
	if raw, e := m.R.Run("systemctl", "list-timers", "--all", "--output=json", "--no-pager", "greflow-console-*.timer"); e == nil {
		var timers []struct {
			Unit string `json:"unit"`
			Next uint64 `json:"next"`
		}
		if json.Unmarshal([]byte(raw), &timers) == nil {
			for _, t := range timers {
				if t.Next > 0 && t.Next < 1<<63 {
					next[t.Unit] = time.UnixMicro(int64(t.Next)).UTC()
				}
			}
		}
	}
	for _, p := range files {
		id := strings.TrimSuffix(filepath.Base(p), ".json")
		if !protocol.IDPattern.MatchString(id) {
			continue
		}
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		var req protocol.Request
		if json.Unmarshal(b, &req) != nil || req.Schedule == nil {
			continue
		}
		o := protocol.ScheduleObservation{Schedule: *req.Schedule, TimerState: "unknown", ObservedAt: time.Now().UTC(), Runs: []protocol.ScheduleRun{}}
		unit := "greflow-console-" + id + ".timer"
		if raw, e := m.R.Run("systemctl", "show", unit, "--property=ActiveState", "--value"); e == nil {
			switch strings.TrimSpace(raw) {
			case "active":
				o.TimerState = "active"
			case "inactive", "failed":
				o.TimerState = strings.TrimSpace(raw)
			}
		}
		if n, ok := next[unit]; ok {
			o.NextRunAt = &n
		}
		if b, e := os.ReadFile(m.Path("/var/lib/greflow/console-schedule-runs/" + id + ".json")); e == nil {
			json.Unmarshal(b, &o.Runs)
		}
		out = append(out, o)
	}
	return out
}
