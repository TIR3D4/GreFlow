package console

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"net/http"
	"strconv"
	"time"
)

type Operation struct {
	ID        string             `json:"id"`
	Actor     string             `json:"actor"`
	HostID    string             `json:"host_id"`
	Request   protocol.Request   `json:"request"`
	State     string             `json:"state"`
	Result    *protocol.Response `json:"result,omitempty"`
	CreatedAt string             `json:"created_at"`
	UpdatedAt string             `json:"updated_at"`
}

func (a *App) enqueue(w http.ResponseWriter, r *http.Request, u User) {
	var v struct {
		HostID  string           `json:"host_id"`
		Request protocol.Request `json:"request"`
	}
	if !decode(w, r, &v) {
		return
	}
	if !u.Can(v.HostID) {
		apiError(w, 403, "SCOPE", "سرور در محدودهٔ دسترسی شما نیست")
		return
	}
	if u.Role == "viewer" && v.Request.Action != "logs" {
		apiError(w, 403, "ROLE", "مشاهده‌گر فقط اجازهٔ خواندن لاگ دارد")
		return
	}
	if u.Role != "admin" && (v.Request.Action == "server_reboot" || v.Request.Action == "prepare" || v.Request.Action == "create" || v.Request.Action == "remove" || v.Request.Action == "recover") {
		apiError(w, 403, "ROLE", "این عملیات به مدیر نیاز دارد")
		return
	}
	v.Request.Version = 1
	v.Request.ID = ID()
	validated := v.Request
	if consoleAction(validated.Action) {
		// Console-only action, never forwarded to the tunnel protocol dispatcher.
		v.Request = protocol.Request{Version: 1, ID: v.Request.ID, Action: validated.Action}
		validated = v.Request
		validated.Action = "discover"
	}
	if e := validated.Validate(); e != nil {
		apiError(w, 400, "VALIDATION", e.Error())
		return
	}
	if h, _, e := a.host(v.HostID); e != nil {
		apiError(w, 404, "HOST", "سرور پیدا نشد")
		return
	} else if consoleAction(v.Request.Action) && h.Hostname == "agent" {
		apiError(w, 400, "TRANSPORT", "این عملیات فقط برای سرور SSH است")
		return
	}
	var existing, req, host string
	e := a.Store.DB.QueryRow("SELECT id,request,host_id FROM operations WHERE actor=? AND idem=?", u.ID, r.Header.Get("Idempotency-Key")).Scan(&existing, &req, &host)
	if e == nil {
		var old protocol.Request
		json.Unmarshal([]byte(req), &old)
		v.Request.ID = old.ID
		if marshal(v.Request) != req || host != v.HostID {
			apiError(w, 409, "IDEMPOTENCY", "شناسه برای درخواست دیگری استفاده شده است")
			return
		}
		reply(w, 202, map[string]string{"id": existing})
		return
	}
	if e != sql.ErrNoRows {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := v.Request.ID
	tx, e := a.Store.DB.Begin()
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec("INSERT INTO operations(id,actor,host_id,idem,request,state,created_at,updated_at) VALUES(?,?,?,?,?,'queued',?,?)", id, u.ID, v.HostID, r.Header.Get("Idempotency-Key"), marshal(v.Request), now, now)
	if e == nil {
		_, e = tx.Exec("INSERT INTO audit(actor,action,object_id,at) VALUES(?,?,?,?)", u.ID, v.Request.Action, v.HostID, now)
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO events(operation_id,at,state) VALUES(?,?,'queued')", id, now)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		apiError(w, 409, "STORAGE", e.Error())
		return
	}
	select {
	case a.wake <- struct{}{}:
	default:
	}
	reply(w, 202, map[string]string{"id": id})
}
func (a *App) transition(id, state string, result *protocol.Response) error {
	tx, e := a.Store.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	at := time.Now().UTC().Format(time.RFC3339Nano)
	var body any
	if result != nil {
		body = marshal(result)
	}
	if _, e = tx.Exec("UPDATE operations SET state=?,result=?,updated_at=? WHERE id=?", state, body, at, id); e != nil {
		return e
	}
	if _, e = tx.Exec("INSERT INTO events(operation_id,at,state) VALUES(?,?,?)", id, at, state); e != nil {
		return e
	}
	return tx.Commit()
}
func (a *App) work(ctx context.Context) error {
	var id, host, body string
	e := a.Store.DB.QueryRow("SELECT id,host_id,request FROM operations WHERE state='queued' AND host_id NOT IN (SELECT host_id FROM agents) ORDER BY created_at LIMIT 1").Scan(&id, &host, &body)
	if e == sql.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	var req protocol.Request
	if e = json.Unmarshal([]byte(body), &req); e != nil {
		return e
	}
	if e = a.transition(id, "running", nil); e != nil {
		return e
	}
	h, encrypted, e := a.host(host)
	if e != nil {
		return e
	}
	key, e := a.Store.Open(encrypted)
	if e != nil {
		return a.transition(id, "failed", &protocol.Response{Version: 1, State: "failed", Error: "credential decryption failed"})
	}
	timeout := 150 * time.Second
	if req.Action == "prepare" {
		timeout = 300 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var res protocol.Response
	if req.Action == "prepare" {
		if p, ok := a.Transport.(ProvisioningTransport); ok {
			res, e = p.Prepare(callCtx, h, key, req, func(steps []protocol.Diagnostic) error {
				return a.transition(id, "running", &protocol.Response{Version: 1, State: "running", Diagnostics: steps})
			})
		} else {
			res = protocol.Response{Version: 1, State: "failed", Error: "نصب خودکار در این روش اتصال پشتیبانی نمی‌شود"}
		}
	} else if consoleAction(req.Action) {
		if transport, ok := a.Transport.(ServerTransport); ok {
			res, e = transport.ServerAction(callCtx, h, key, req)
		} else {
			res = protocol.Response{Version: 1, State: "failed", Error: "این روش اتصال عملیات سرور را پشتیبانی نمی‌کند"}
		}
	} else {
		res, e = a.Transport.Call(callCtx, h, key, req)
	}
	for i := range key {
		key[i] = 0
	}
	return a.complete(id, host, req, res, e)
}
func (a *App) complete(id, host string, req protocol.Request, res protocol.Response, e error) error {
	h, _, he := a.host(host)
	if he != nil {
		return he
	}
	if e != nil {
		if res.State != "failed" {
			res.State = "uncertain"
		}
		res.Version = 1
		res.Error = e.Error()
		if _, e = a.Store.DB.Exec("UPDATE hosts SET last_error=? WHERE id=?", res.Error, host); e != nil {
			return e
		}
	} else {
		if res.Inventory != nil {
			b := res.Inventory
			if b.HostID == "" {
				res.State = "failed"
				res.Error = "missing host identity"
			} else {
				result, err := a.Store.DB.Exec("UPDATE hosts SET host_identity=?,inventory=?,addresses=?,observed_at=?,last_error='' WHERE id=? AND (host_identity IS NULL OR host_identity=?)", b.HostID, marshal(b), marshal(b.Addresses), b.ObservedAt.Format(time.RFC3339Nano), host, b.HostID)
				if err != nil {
					res.State = "failed"
					res.Error = "HOST_IDENTITY_CONFLICT: this machine is already registered"
				} else {
					n, _ := result.RowsAffected()
					if n != 1 {
						res.State = "failed"
						res.Error = "HOST_IDENTITY_CHANGED: stored identity does not match"
					}
				}
			}
		}
		if res.Tunnel != nil && h.Inventory != nil {
			found := false
			for i, t := range h.Inventory.Tunnels {
				if t.Name == res.Tunnel.Name {
					h.Inventory.Tunnels[i] = *res.Tunnel
					found = true
				}
			}
			if !found {
				h.Inventory.Tunnels = append(h.Inventory.Tunnels, *res.Tunnel)
			}
			if _, e = a.Store.DB.Exec("UPDATE hosts SET inventory=?,last_error='' WHERE id=?", marshal(h.Inventory), host); e != nil {
				return e
			}
		}
		if req.Action == "remove" && res.State == "succeeded" && h.Inventory != nil {
			ts := []protocol.Tunnel{}
			for _, t := range h.Inventory.Tunnels {
				if t.Name != req.Name {
					ts = append(ts, t)
				}
			}
			h.Inventory.Tunnels = ts
			if _, e = a.Store.DB.Exec("UPDATE hosts SET inventory=? WHERE id=?", marshal(h.Inventory), host); e != nil {
				return e
			}
		}
		if res.State == "succeeded" && req.Schedule != nil {
			if req.Action == "schedule" {
				_, e = a.Store.DB.Exec("INSERT INTO schedules(id,host_id,body,operation_id) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body,operation_id=excluded.operation_id", req.Schedule.ID, host, marshal(req.Schedule), id)
			} else if req.Action == "unschedule" {
				_, e = a.Store.DB.Exec("DELETE FROM schedules WHERE id=? AND host_id=?", req.Schedule.ID, host)
			}
			if e != nil {
				res.State = "uncertain"
				res.Error = "remote acknowledgement received; local schedule storage failed"
			}
		}
	}
	if res.State == "" {
		res.State = "uncertain"
		res.Error = "missing result state"
	}
	if res.Error != "" {
		if _, e = a.Store.DB.Exec("UPDATE hosts SET last_error=? WHERE id=?", res.Error, host); e != nil {
			return e
		}
	}
	if res.Tunnel != nil {
		a.recordIncident(host, *res.Tunnel)
	}
	return a.transition(id, res.State, &res)
}
func (a *App) operations(w http.ResponseWriter, r *http.Request, u User) {
	rows, e := a.Store.DB.Query("SELECT id,actor,host_id,request,state,result,created_at,updated_at FROM operations ORDER BY created_at DESC LIMIT 500")
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	defer rows.Close()
	out := []Operation{}
	for rows.Next() {
		var o Operation
		var req string
		var res *string
		if e = rows.Scan(&o.ID, &o.Actor, &o.HostID, &req, &o.State, &res, &o.CreatedAt, &o.UpdatedAt); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		if !u.Can(o.HostID) {
			continue
		}
		json.Unmarshal([]byte(req), &o.Request)
		if res != nil {
			json.Unmarshal([]byte(*res), &o.Result)
		}
		out = append(out, o)
	}
	reply(w, 200, out)
}
func (a *App) schedules(w http.ResponseWriter, r *http.Request, u User) {
	rows, e := a.Store.DB.Query("SELECT host_id,body,operation_id FROM schedules")
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var host, body, op string
		rows.Scan(&host, &body, &op)
		if u.Can(host) {
			var s protocol.Schedule
			json.Unmarshal([]byte(body), &s)
			out = append(out, map[string]any{"host_id": host, "schedule": s, "operation_id": op})
		}
	}
	reply(w, 200, out)
}
func (a *App) events(w http.ResponseWriter, r *http.Request, u User) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	if v, e := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64); e == nil && v > cursor {
		cursor = v
	}
	f, ok := w.(http.Flusher)
	if !ok {
		apiError(w, 500, "SSE", "stream unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		rows, e := a.Store.DB.Query("SELECT e.id,e.operation_id,e.at,e.state,o.host_id FROM events e JOIN operations o ON o.id=e.operation_id WHERE e.id>? ORDER BY e.id LIMIT 200", cursor)
		if e != nil {
			return
		}
		for rows.Next() {
			var id int64
			var op, at, state, host string
			rows.Scan(&id, &op, &at, &state, &host)
			cursor = id
			if u.Can(host) {
				fmt.Fprintf(w, "id: %d\ndata: %s\n\n", id, marshal(map[string]string{"id": op, "state": state, "at": at}))
			}
		}
		rows.Close()
		fmt.Fprint(w, ": heartbeat\n\n")
		f.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
		u, e = a.user(r)
		if e != nil {
			return
		}
	}
}
