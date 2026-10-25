package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/core/system"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"github.com/TIR3D4/GreFlow/internal/providers/gre"
	"io"
	"os"
	"time"
	_ "time/tzdata"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println("GreFlow dispatcher protocol 1")
		return nil
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("dispatcher requires root")
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("use dispatch or schedule-run")
	}
	if len(os.Args) == 3 && os.Args[1] == "enroll" {
		return enroll(os.Args[2])
	}
	if len(os.Args) == 2 && os.Args[1] == "run" {
		return outbound()
	}
	m := manager.Manager{Root: os.Getenv("GREFLOW_ROOT"), R: system.Exec{}}
	var r protocol.Request
	switch os.Args[1] {
	case "dispatch":
		if len(os.Args) != 2 {
			return fmt.Errorf("unexpected arguments")
		}
		d := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20))
		d.DisallowUnknownFields()
		if e := d.Decode(&r); e != nil {
			return e
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return fmt.Errorf("trailing JSON")
		}
	case "schedule-run":
		if len(os.Args) != 3 || !protocol.IDPattern.MatchString(os.Args[2]) {
			return fmt.Errorf("invalid schedule ID")
		}
		b, e := os.ReadFile(m.Path("/etc/greflow/console-schedules/" + os.Args[2] + ".json"))
		if e != nil {
			return e
		}
		if e = json.Unmarshal(b, &r); e != nil {
			return e
		}
		id := make([]byte, 16)
		if _, e = rand.Read(id); e != nil {
			return e
		}
		r.ID = hex.EncodeToString(id)
		m.Name = r.Name
		c, e := config.Load(m.ConfigPath())
		if e != nil {
			return e
		}
		r.Revision = protocol.Revision(c)
		if r.Schedule != nil {
			maintenance, err := r.Schedule.InMaintenance(time.Now())
			if err != nil {
				return err
			}
			if maintenance {
				return scheduleOutput(m, os.Args[2], protocol.Response{Version: 1, State: "succeeded", Logs: "maintenance window active; scheduled action skipped"})
			}
		}
		if r.Action == "recovery" {
			res, e := runRecovery(m, r, os.Args[2])
			if e != nil {
				res = protocol.Response{Version: 1, State: "failed", Error: e.Error()}
			}
			return scheduleOutput(m, os.Args[2], res)
		}
		return scheduleOutput(m, os.Args[2], gre.Dispatch(m, r))
	default:
		return fmt.Errorf("unsupported command")
	}
	return json.NewEncoder(os.Stdout).Encode(gre.Dispatch(m, r))
}

func scheduleOutput(m manager.Manager, id string, res protocol.Response) error {
	if e := gre.RecordScheduleRun(m, id, res); e != nil {
		return fmt.Errorf("schedule outcome %s; history persistence failed: %w", res.State, e)
	}
	if e := json.NewEncoder(os.Stdout).Encode(res); e != nil {
		return e
	}
	if res.State != "succeeded" {
		return fmt.Errorf("scheduled operation: %s", res.State)
	}
	return nil
}
