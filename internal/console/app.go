package console

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type App struct {
	BuildVersion string
	Geo          *IPLookup
	AgentURL     string

	Store     *Store
	Transport Transport
	Origin    string
	Secure    bool
	mu        sync.Mutex
	rates     map[string][]time.Time
	wake      chan struct{}
}

func New(s *Store, origin string) *App {
	return &App{Geo: newIPLookup(s), Store: s, Transport: SSHTransport{}, Origin: origin, Secure: strings.HasPrefix(origin, "https://"), rates: map[string][]time.Time{}, wake: make(chan struct{}, 1)}
}

// Bootstrap removes the obsolete setup-code file after upgrading. First run now
// creates the sole initial administrator directly, under a database transaction.
func (a *App) Bootstrap() error {
	e := os.Remove(filepath.Join(a.Store.Dir, "bootstrap-token"))
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
func (a *App) allow(addr string) bool {
	host, _, _ := net.SplitHostPort(addr)
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	recent := []time.Time{}
	for _, t := range a.rates[host] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 10 {
		return false
	}
	a.rates[host] = append(recent, now)
	if len(a.rates) > 10000 {
		a.rates = map[string][]time.Time{host: recent}
	}
	return true
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, status int, code, message string) {
	reply(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "retryable": status >= 500}})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		apiError(w, 400, "JSON", "درخواست JSON معتبر نیست")
		return false
	}
	var x any
	if d.Decode(&x) != io.EOF {
		apiError(w, 400, "JSON", "دادهٔ اضافی در درخواست")
		return false
	}
	return true
}
func (a *App) Handler(static http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			if r.Method != "GET" && r.Method != "HEAD" {
				http.Error(w, "method", 405)
				return
			}
			static.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		mut := r.Method != "GET"
		if r.Method != "GET" && r.Method != "POST" {
			apiError(w, 405, "METHOD", "روش درخواست پشتیبانی نمی‌شود")
			return
		}
		if mut {
			if r.Header.Get("Origin") != a.Origin {
				apiError(w, 403, "ORIGIN", "مبدأ درخواست معتبر نیست")
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				apiError(w, 415, "CONTENT_TYPE", "JSON لازم است")
				return
			}
		}
		if path == "status" && r.Method == "GET" {
			var n int
			if e := a.Store.DB.QueryRow("SELECT count(*) FROM users").Scan(&n); e != nil {
				apiError(w, 503, "STORAGE", "پایگاه داده در دسترس نیست")
				return
			}
			reply(w, 200, map[string]any{"bootstrap_required": n == 0, "protocol": 1})
			return
		}
		if path == "bootstrap" || path == "auth/login" {
			if !mut {
				apiError(w, 405, "METHOD", "POST لازم است")
				return
			}
			a.auth(w, r, path)
			return
		}
		u, e := a.user(r)
		if e != nil {
			apiError(w, 401, "SESSION", "ابتدا وارد شوید")
			return
		}
		if mut && r.Header.Get("X-CSRF-Token") != u.CSRF {
			apiError(w, 403, "CSRF", "نشست یا کد امنیتی معتبر نیست")
			return
		}
		if mut && r.Header.Get("Idempotency-Key") == "" {
			apiError(w, 400, "IDEMPOTENCY", "شناسهٔ یکتای درخواست لازم است")
			return
		}
		if mut && path != "operations" {
			capture, done, ok := a.reserveReply(w, r, u)
			if !ok {
				return
			}
			w = capture
			defer done()
		}
		if path == "session" && !mut {
			reply(w, 200, u)
			return
		}
		if path == "capabilities" && !mut {
			reply(w, 200, map[string]any{"ssh": true, "push_agent": a.AgentURL != "", "push_agent_reason": "Requires configured native HTTPS agent listener, public reachability and a valid TLS certificate.", "providers": []string{"gre"}, "transfer": "single TCP mapping on same entry endpoint", "automatic_recovery": true, "build": a.BuildVersion})
			return
		}
		if path == "users" || path == "users/manage" || strings.HasPrefix(path, "auth/") {
			if !mut && path != "users" && path != "auth/sessions" {
				apiError(w, 405, "METHOD", "POST لازم است")
				return
			}
			a.security(w, r, u, path)
			return
		}
		if path == "events" && !mut {
			a.events(w, r, u)
			return
		}
		if mut && u.Role == "viewer" && path != "operations" {
			apiError(w, 403, "ROLE", "حساب مشاهده‌گر اجازهٔ تغییر ندارد")
			return
		}
		switch path {
		case "ip-info":
			a.ipInfo(w, r, u)
		case "ssh/check":
			a.checkSSH(w, r, u)
		case "servers/credentials":
			a.hostCredentials(w, r, u)
		case "enrollments":
			a.enrollmentAdmin(w, r, u)
		case "servers":
			if mut {
				a.createHost(w, r, u)
			} else {
				a.listHosts(w, r, u)
			}
		case "ssh/fingerprint":
			if !mut || u.Role != "admin" {
				apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
				return
			}
			var v struct {
				Hostname string `json:"hostname"`
				Port     int    `json:"port"`
			}
			if !decode(w, r, &v) {
				return
			}
			fp, e := Fingerprint(r.Context(), v.Hostname, v.Port)
			if e != nil {
				apiError(w, 502, "SSH", e.Error())
				return
			}
			reply(w, 200, map[string]string{"fingerprint": fp, "warning": "Compare through an independent trusted channel before saving."})
		case "operations":
			if mut {
				a.enqueue(w, r, u)
			} else {
				a.operations(w, r, u)
			}
		case "audit":
			if mut || u.Role != "admin" {
				apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
				return
			}
			rows, e := a.Store.DB.Query("SELECT actor,action,object_id,at FROM audit ORDER BY id DESC LIMIT 500")
			if e != nil {
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
			defer rows.Close()
			out := []map[string]string{}
			for rows.Next() {
				var actor, action, id, at string
				rows.Scan(&actor, &action, &id, &at)
				out = append(out, map[string]string{"actor": actor, "action": action, "object_id": id, "at": at})
			}
			reply(w, 200, out)
		case "schedules":
			if mut {
				apiError(w, 405, "METHOD", "از عملیات زمان‌بندی استفاده کنید")
				return
			}
			a.schedules(w, r, u)
		case "backups", "restore-plans", "restore":
			if !mut || u.Role != "admin" {
				apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
				return
			}
			a.backup(w, r, u, path)
		case "monitoring":
			a.monitoringSettings(w, r, u)
		case "notifications":
			a.notifications(w, r, u)
		default:
			apiError(w, 404, "NOT_FOUND", "مسیر پیدا نشد")
		}
	})
}
func (a *App) createHost(w http.ResponseWriter, r *http.Request, u User) {
	if u.Role != "admin" {
		apiError(w, 403, "ROLE", "افزودن سرور به مدیر نیاز دارد")
		return
	}
	var h Host
	if !decode(w, r, &h) {
		return
	}
	credential, e := credentialFromHost(h)
	if e != nil {
		apiError(w, 400, "CREDENTIAL", e.Error())
		return
	}
	if h.Fingerprint == "" {
		h.Fingerprint, e = Fingerprint(r.Context(), h.Hostname, h.Port)
		if e != nil {
			apiError(w, 502, "SSH", "دریافت هویت سرور ممکن نشد")
			return
		}
	}
	if e := h.Validate(); e != nil {
		apiError(w, 400, "VALIDATION", e.Error())
		return
	}
	key, e := a.Store.Seal(credential)
	if e != nil {
		apiError(w, 500, "KEY", e.Error())
		return
	}
	h.PrivateKey = ""
	h.Password = ""
	h.KeyPassphrase = ""
	h.ID = ID()
	h.Inventory = nil
	h.PreparationID = ID()
	tx, e := a.Store.DB.Begin()
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec("INSERT INTO hosts(id,name,kind,addresses,hostname,port,username,fingerprint,credential) VALUES(?,?,?,?,?,?,?,?,?)", h.ID, h.Name, h.Kind, marshal(h.Addresses), h.Hostname, h.Port, h.Username, h.Fingerprint, key)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e == nil {
		_, e = tx.Exec("INSERT INTO operations(id,actor,host_id,idem,request,state,created_at,updated_at) VALUES(?,?,?,?,?,'queued',?,?)", h.PreparationID, u.ID, h.ID, "prepare-"+h.ID, marshal(map[string]any{"version": 1, "id": h.PreparationID, "action": "prepare", "name": ""}), now, now)
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO events(operation_id,at,state) VALUES(?,?,'queued')", h.PreparationID, now)
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO audit(actor,action,object_id,at) VALUES(?,'create_host_and_prepare',?,?)", u.ID, h.ID, now)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	select {
	case a.wake <- struct{}{}:
	default:
	}
	reply(w, 201, h)
}
func (a *App) host(id string) (Host, []byte, error) {
	var h Host
	var addresses string
	var encrypted []byte
	var inventory, errText, at *string
	e := a.Store.DB.QueryRow("SELECT id,name,kind,addresses,hostname,port,username,fingerprint,credential,inventory,last_error,observed_at FROM hosts WHERE id=?", id).Scan(&h.ID, &h.Name, &h.Kind, &addresses, &h.Hostname, &h.Port, &h.Username, &h.Fingerprint, &encrypted, &inventory, &errText, &at)
	if e != nil {
		return h, nil, e
	}
	json.Unmarshal([]byte(addresses), &h.Addresses)
	if inventory != nil {
		json.Unmarshal([]byte(*inventory), &h.Inventory)
	}
	if errText != nil {
		h.LastError = *errText
	}
	if at != nil {
		h.ObservedAt = *at
	}
	if plain, err := a.Store.Open(encrypted); err == nil {
		if c, err := readCredential(plain); err == nil {
			h.AuthMethod = c.Method
		}
	}
	return h, encrypted, nil
}
func (a *App) listHosts(w http.ResponseWriter, r *http.Request, u User) {
	rows, e := a.Store.DB.Query("SELECT id FROM hosts ORDER BY name")
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		if u.Can(id) {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []Host{}
	for _, id := range ids {
		h, _, e := a.host(id)
		if e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		out = append(out, h)
	}
	reply(w, 200, out)
}
func (a *App) Run(ctx context.Context) error {
	if _, e := a.Store.DB.Exec("UPDATE operations SET state='uncertain',updated_at=? WHERE state='running'", time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
		return e
	}
	go a.monitor(ctx)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-a.wake:
		}
		if e := a.work(ctx); e != nil {
			return e
		}
	}
}
