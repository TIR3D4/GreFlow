package console

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"net/http"
	"time"
)

type MonitorSettings struct {
	Enabled       bool `json:"enabled"`
	Minutes       int  `json:"minutes"`
	RetentionDays int  `json:"retention_days"`
}

func (a *App) monitorConfig() MonitorSettings {
	v := MonitorSettings{Minutes: 1, RetentionDays: 30}
	var body []byte
	if a.Store.DB.QueryRow("SELECT body FROM settings WHERE id='monitor'").Scan(&body) == nil {
		json.Unmarshal(body, &v)
	}
	return v
}
func (a *App) monitoringSettings(w http.ResponseWriter, r *http.Request, u User) {
	if u.Role != "admin" {
		apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
		return
	}
	if r.Method == "GET" {
		reply(w, 200, a.monitorConfig())
		return
	}
	var v MonitorSettings
	if !decode(w, r, &v) {
		return
	}
	if v.Minutes < 1 || v.Minutes > 60 || v.RetentionDays < 7 || v.RetentionDays > 365 {
		apiError(w, 400, "VALIDATION", "فاصله ۱ تا ۶۰ دقیقه و نگهداری ۷ تا ۳۶۵ روز")
		return
	}
	if _, e := a.Store.DB.Exec("INSERT INTO settings(id,body) VALUES('monitor',?) ON CONFLICT(id) DO UPDATE SET body=excluded.body", []byte(marshal(v))); e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	a.Store.Audit(u.ID, "monitor_settings", "")
	reply(w, 200, v)
}
func (a *App) monitor(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	next := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		cfg := a.monitorConfig()
		now := time.Now().UTC()
		// Claimed work is never automatically resent when its result is unknown.
		a.Store.DB.Exec("UPDATE operations SET state='uncertain' WHERE state='running' AND updated_at<?", now.Add(-5*time.Minute).Format(time.RFC3339Nano))
		a.Store.DB.Exec("DELETE FROM sessions WHERE expires<?", now.Unix())
		if cfg.Enabled && now.After(next) {
			a.queueMonitoring()
			next = now.Add(time.Duration(cfg.Minutes) * time.Minute)
		}
		a.deliverNotification()
		// Preserve idempotency tombstones and audit; prune large result bodies/events.
		cutoff := now.AddDate(0, 0, -cfg.RetentionDays).Format(time.RFC3339Nano)
		a.Store.DB.Exec("DELETE FROM events WHERE at<? AND operation_id IN (SELECT id FROM operations WHERE state IN ('succeeded','failed','rolled_back'))", cutoff)
		a.Store.DB.Exec("UPDATE operations SET result=NULL WHERE updated_at<? AND state IN ('succeeded','failed','rolled_back') AND result IS NOT NULL", cutoff)
	}
}
func (a *App) queueMonitoring() {
	rows, e := a.Store.DB.Query("SELECT id FROM hosts WHERE id NOT IN (SELECT host_id FROM agents WHERE revoked=1)")
	if e != nil {
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		var count int
		a.Store.DB.QueryRow("SELECT count(*) FROM operations WHERE host_id=? AND state IN ('queued','running')", id).Scan(&count)
		if count > 0 {
			continue
		}
		h, _, e := a.host(id)
		if e != nil {
			continue
		}
		requests := []protocol.Request{}
		if h.Inventory == nil || time.Since(h.Inventory.ObservedAt) > 10*time.Minute {
			requests = append(requests, protocol.Request{Action: "discover"})
		} else {
			for _, t := range h.Inventory.Tunnels {
				requests = append(requests, protocol.Request{Action: "probe", Name: t.Name})
			}
		}
		for _, req := range requests {
			req.Version = 1
			req.ID = ID()
			now := time.Now().UTC().Format(time.RFC3339Nano)
			a.Store.DB.Exec("INSERT INTO operations(id,actor,host_id,idem,request,state,created_at,updated_at) VALUES(?,'monitor',?,?,?,'queued',?,?)", req.ID, id, req.ID, marshal(req), now, now)
		}
	}
}
func incidentState(t protocol.Tunnel) string {
	peer := "unknown"
	tcpOK := false
	failed := false
	for _, e := range t.Checks {
		if e.Kind == "user_verified" {
			continue
		}
		if time.Now().After(e.ExpiresAt) {
			return "stale"
		}
		if e.Kind == "gre_peer" {
			peer = e.Status
		}
		if len(e.Kind) > 12 && e.Kind[:12] == "destination:" {
			if e.Status == "ok" {
				tcpOK = true
			}
			if e.Status == "failed" {
				failed = true
			}
		}
		if (e.Kind == "interface" || e.Kind == "rules") && e.Status == "failed" {
			failed = true
		}
	}
	if failed {
		return "down"
	}
	if peer == "failed" && tcpOK {
		return "degraded"
	}
	if peer == "failed" {
		return "down"
	}
	if peer == "ok" {
		return "reachable"
	}
	return "unknown"
}
func (a *App) recordIncident(host string, t protocol.Tunnel) {
	state := incidentState(t)
	if state == "unknown" || state == "stale" {
		return
	}
	id := host + ":" + t.Name
	tx, e := a.Store.DB.Begin()
	if e != nil {
		return
	}
	defer tx.Rollback()
	var previous string
	var generation int
	e = tx.QueryRow("SELECT state,generation FROM incidents WHERE id=?", id).Scan(&previous, &generation)
	if e == nil && previous == state {
		return
	}
	generation++
	if _, e = tx.Exec("INSERT INTO incidents(id,state,generation) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET state=excluded.state,generation=excluded.generation", id, state, generation); e != nil {
		return
	}
	if state != "reachable" || previous != "" {
		incident := fmt.Sprintf("%s:%d", id, generation)
		body := fmt.Sprintf("GreFlow: %s / %s → %s (GRE/TCP evidence; not client verification)", host, t.Name, state)
		if _, e = tx.Exec("INSERT OR IGNORE INTO deliveries(id,incident,body,state,next_at) VALUES(?,?,?,'pending',?)", ID(), incident, body, time.Now().Unix()); e != nil {
			return
		}
	}
	tx.Commit()
}
func (a *App) deliverNotification() {
	var encrypted []byte
	if a.Store.DB.QueryRow("SELECT body FROM settings WHERE id='telegram'").Scan(&encrypted) != nil {
		return
	}
	plain, e := a.Store.Open(encrypted)
	if e != nil {
		return
	}
	var n Notification
	if json.Unmarshal(plain, &n) != nil || !n.Enabled {
		return
	}
	var id, body string
	var attempts int
	if a.Store.DB.QueryRow("SELECT id,body,attempts FROM deliveries WHERE state='pending' AND next_at<=? ORDER BY next_at LIMIT 1", time.Now().Unix()).Scan(&id, &body, &attempts) != nil {
		return
	}
	e = sendTelegram(n, body)
	state := "sent"
	if e != nil {
		state = "pending"
		if attempts >= 7 {
			state = "failed"
		}
	}
	delay := time.Duration(1<<uint(attempts)) * 15 * time.Second
	a.Store.DB.Exec("UPDATE deliveries SET state=?,attempts=attempts+1,next_at=? WHERE id=?", state, time.Now().Add(delay).Unix(), id)
}
