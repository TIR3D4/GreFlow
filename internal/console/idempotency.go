package console

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type capturedReply struct {
	headers http.Header
	status  int
	body    bytes.Buffer
}

func (c *capturedReply) Header() http.Header { return c.headers }
func (c *capturedReply) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}
func (c *capturedReply) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = 200
	}
	return c.body.Write(b)
}

type savedReply struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body"`
}

// Reserve before entering a mutation. An interrupted reservation is never replayed:
// the client gets an explicit uncertain response instead of a duplicate side effect.
func (a *App) reserveReply(w http.ResponseWriter, r *http.Request, u User) (http.ResponseWriter, func(), bool) {
	body, e := io.ReadAll(io.LimitReader(r.Body, (8<<20)+1))
	if e != nil || len(body) > 8<<20 {
		apiError(w, 413, "BODY", "درخواست بیش از حد بزرگ است")
		return nil, nil, false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	key := r.Header.Get("Idempotency-Key")
	if len(key) > 128 {
		apiError(w, 400, "IDEMPOTENCY", "شناسه بیش از حد طولانی است")
		return nil, nil, false
	}
	digest := hash(r.URL.Path + "\n" + string(body))
	result, e := a.Store.DB.Exec("INSERT OR IGNORE INTO mutation_receipts(actor,idem,digest,created_at) VALUES(?,?,?,?)", u.ID, key, digest, time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		apiError(w, 503, "STORAGE", "ثبت امن درخواست ممکن نیست")
		return nil, nil, false
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var previous string
		var encrypted []byte
		if e = a.Store.DB.QueryRow("SELECT digest,response FROM mutation_receipts WHERE actor=? AND idem=?", u.ID, key).Scan(&previous, &encrypted); e != nil {
			apiError(w, 503, "STORAGE", e.Error())
			return nil, nil, false
		}
		if previous != digest {
			apiError(w, 409, "IDEMPOTENCY", "شناسه برای درخواست دیگری استفاده شده است")
			return nil, nil, false
		}
		if len(encrypted) == 0 {
			apiError(w, 409, "UNCERTAIN", "درخواست قبلی در حال اجرا یا نتیجهٔ آن نامشخص است؛ پیش از تغییر تازه وضعیت را تطبیق دهید")
			return nil, nil, false
		}
		plain, e := a.Store.Open(encrypted)
		if e != nil {
			apiError(w, 500, "KEY", "بازیابی نتیجه ممکن نیست")
			return nil, nil, false
		}
		var reply savedReply
		if e = json.Unmarshal(plain, &reply); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return nil, nil, false
		}
		writeSaved(w, reply)
		return nil, nil, false
	}
	capture := &capturedReply{headers: make(http.Header)}
	done := func() {
		status := capture.status
		if status == 0 {
			status = 200
		}
		response := savedReply{Status: status, Headers: capture.headers, Body: capture.body.Bytes()}
		encrypted, e := a.Store.Seal([]byte(marshal(response)))
		if e == nil {
			_, e = a.Store.DB.Exec("UPDATE mutation_receipts SET response=? WHERE actor=? AND idem=?", encrypted, u.ID, key)
		}
		if e != nil {
			apiError(w, 503, "UNCERTAIN", "عملیات ممکن است انجام شده باشد؛ ذخیرهٔ نتیجه ناموفق بود. دوباره اجرا نکنید.")
			return
		}
		writeSaved(w, response)
	}
	return capture, done, true
}
func writeSaved(w http.ResponseWriter, r savedReply) {
	for k, values := range r.Headers {
		w.Header()[k] = values
	}
	w.WriteHeader(r.Status)
	w.Write(r.Body)
}
