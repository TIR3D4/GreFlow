package console

import (
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
)

func (a *App) accountManagement(w http.ResponseWriter, r *http.Request, u User, path string) {
	if path == "auth/sessions" {
		current, _ := r.Cookie("greflow_session")
		if r.Method == "GET" {
			rows, e := a.Store.DB.Query("SELECT id,expires FROM sessions WHERE user_id=? AND expires>? ORDER BY expires DESC", u.ID, time.Now().Unix())
			if e != nil {
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var id string
				var expires int64
				if e = rows.Scan(&id, &expires); e != nil {
					apiError(w, 500, "STORAGE", e.Error())
					return
				}
				out = append(out, map[string]any{"id": id, "expires": time.Unix(expires, 0).UTC(), "current": current != nil && id == hash(current.Value)})
			}
			reply(w, 200, out)
			return
		}
		var v struct {
			ID string `json:"id"`
		}
		if !decode(w, r, &v) {
			return
		}
		if _, e := a.Store.DB.Exec("DELETE FROM sessions WHERE id=? AND user_id=?", v.ID, u.ID); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		a.Store.Audit(u.ID, "revoke_session", v.ID)
		reply(w, 200, map[string]bool{"revoked": true})
		return
	}
	if path == "auth/password" {
		var v struct {
			Current  string `json:"current"`
			Password string `json:"password"`
		}
		if !decode(w, r, &v) {
			return
		}
		var pwd []byte
		if e := a.Store.DB.QueryRow("SELECT password FROM users WHERE id=?", u.ID).Scan(&pwd); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		if bcrypt.CompareHashAndPassword(pwd, []byte(v.Current)) != nil {
			apiError(w, 403, "PASSWORD", "رمز فعلی معتبر نیست")
			return
		}
		if len(v.Password) < 8 || len(v.Password) > 72 {
			apiError(w, 400, "PASSWORD", "رمز جدید باید ۸ تا ۷۲ بایت باشد")
			return
		}
		encoded, e := bcrypt.GenerateFromPassword([]byte(v.Password), 12)
		if e != nil {
			apiError(w, 500, "PASSWORD", e.Error())
			return
		}
		tx, e := a.Store.DB.Begin()
		if e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		defer tx.Rollback()
		if _, e = tx.Exec("UPDATE users SET password=? WHERE id=?", encoded, u.ID); e == nil {
			_, e = tx.Exec("DELETE FROM sessions WHERE user_id=?", u.ID)
		}
		if e == nil {
			_, e = tx.Exec("INSERT INTO audit(actor,action,object_id,at) VALUES(?,?,?,?)", u.ID, "change_password", u.ID, time.Now().UTC().Format(time.RFC3339Nano))
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		reply(w, 200, map[string]bool{"logout_required": true})
		return
	}
	if u.Role != "admin" {
		apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
		return
	}
	var v struct {
		ID       string   `json:"id"`
		Action   string   `json:"action"`
		Role     string   `json:"role"`
		Scope    []string `json:"scope"`
		Password string   `json:"password"`
	}
	if !decode(w, r, &v) {
		return
	}
	var role string
	if e := a.Store.DB.QueryRow("SELECT role FROM users WHERE id=?", v.ID).Scan(&role); e != nil {
		apiError(w, 404, "USER", "کاربر پیدا نشد")
		return
	}
	if role == "admin" || v.ID == u.ID {
		apiError(w, 409, "OWNER", "مالک از این مسیر قابل حذف یا تغییر نیست")
		return
	}
	if v.Action != "delete" && v.Action != "update" {
		apiError(w, 400, "ACTION", "عملیات معتبر نیست")
		return
	}
	var pwd []byte
	if v.Action == "update" {
		if v.Role != "operator" && v.Role != "viewer" {
			apiError(w, 400, "ROLE", "نقش معتبر نیست")
			return
		}
		for _, id := range v.Scope {
			var n int
			if a.Store.DB.QueryRow("SELECT count(*) FROM hosts WHERE id=?", id).Scan(&n) != nil || n != 1 {
				apiError(w, 400, "SCOPE", "سرور انتخاب‌شده وجود ندارد")
				return
			}
		}
		if v.Password != "" {
			if len(v.Password) < 8 || len(v.Password) > 72 {
				apiError(w, 400, "PASSWORD", "رمز جدید باید ۸ تا ۷۲ بایت باشد")
				return
			}
			var e error
			pwd, e = bcrypt.GenerateFromPassword([]byte(v.Password), 12)
			if e != nil {
				apiError(w, 500, "PASSWORD", e.Error())
				return
			}
		}
	}
	tx, e := a.Store.DB.Begin()
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	defer tx.Rollback()
	if _, e = tx.Exec("DELETE FROM sessions WHERE user_id=?", v.ID); e == nil {
		if v.Action == "delete" {
			if _, e = tx.Exec("DELETE FROM recovery WHERE user_id=?", v.ID); e == nil {
				_, e = tx.Exec("DELETE FROM users WHERE id=?", v.ID)
			}
		} else {
			_, e = tx.Exec("UPDATE users SET role=?,scope=? WHERE id=?", v.Role, marshal(v.Scope), v.ID)
			if e == nil && pwd != nil {
				_, e = tx.Exec("UPDATE users SET password=? WHERE id=?", pwd, v.ID)
			}
		}
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO audit(actor,action,object_id,at) VALUES(?,?,?,?)", u.ID, v.Action+"_user", v.ID, time.Now().UTC().Format(time.RFC3339Nano))
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	reply(w, 200, map[string]bool{"changed": true})
}

func (a *App) hostCredentials(w http.ResponseWriter, r *http.Request, u User) {
	if r.Method != "POST" || u.Role != "admin" {
		apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
		return
	}
	var v struct {
		ID            string `json:"id"`
		Username      string `json:"username"`
		Fingerprint   string `json:"fingerprint"`
		PrivateKey    string `json:"private_key"`
		AuthMethod    string `json:"auth_method"`
		Password      string `json:"password"`
		KeyPassphrase string `json:"key_passphrase"`
	}
	if !decode(w, r, &v) {
		return
	}
	h, _, e := a.host(v.ID)
	if e != nil {
		apiError(w, 404, "HOST", "سرور پیدا نشد")
		return
	}
	if h.Hostname == "agent" {
		apiError(w, 409, "AGENT", "برای عامل از لغو دسترسی و ثبت مجدد استفاده کنید")
		return
	}
	h.Username = v.Username
	if v.Fingerprint != "" {
		h.Fingerprint = v.Fingerprint
	}
	h.AuthMethod = v.AuthMethod
	h.Password = v.Password
	h.PrivateKey = v.PrivateKey
	h.KeyPassphrase = v.KeyPassphrase
	if e = h.Validate(); e != nil {
		apiError(w, 400, "VALIDATION", e.Error())
		return
	}
	credential, e := credentialFromHost(h)
	if e != nil {
		apiError(w, 400, "CREDENTIAL", e.Error())
		return
	}
	sealed, e := a.Store.Seal(credential)
	if e != nil {
		apiError(w, 500, "KEY", e.Error())
		return
	}
	tx, e := a.Store.DB.Begin()
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	defer tx.Rollback()
	var n int
	if e = tx.QueryRow("SELECT count(*) FROM operations WHERE host_id=? AND state IN ('queued','running')", v.ID).Scan(&n); e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	if n > 0 {
		apiError(w, 409, "BUSY", "تا پایان عملیات در صف و در حال اجرا صبر کنید")
		return
	}
	if _, e = tx.Exec("UPDATE hosts SET username=?,fingerprint=?,credential=?,last_error='' WHERE id=?", v.Username, h.Fingerprint, sealed, v.ID); e == nil {
		_, e = tx.Exec("INSERT INTO audit(actor,action,object_id,at) VALUES(?,?,?,?)", u.ID, "rotate_ssh_credential", v.ID, time.Now().UTC().Format(time.RFC3339Nano))
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	reply(w, 200, map[string]bool{"updated": true})
}
