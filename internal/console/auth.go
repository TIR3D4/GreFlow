package console

import (
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
)

type User struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Role  string   `json:"role"`
	Scope []string `json:"scope"`
	CSRF  string   `json:"csrf,omitempty"`
}

func (u User) Can(host string) bool {
	if u.Role == "admin" {
		return true
	}
	for _, v := range u.Scope {
		if v == host {
			return true
		}
	}
	return false
}
func (a *App) user(r *http.Request) (User, error) {
	var u User
	var scopes string
	c, e := r.Cookie("greflow_session")
	if e != nil {
		return u, e
	}
	e = a.Store.DB.QueryRow(`SELECT u.id,u.name,u.role,u.scope,s.csrf FROM users u JOIN sessions s ON s.user_id=u.id WHERE s.id=? AND s.expires>?`, hash(c.Value), time.Now().Unix()).Scan(&u.ID, &u.Name, &u.Role, &scopes, &u.CSRF)
	if e != nil {
		return u, e
	}
	json.Unmarshal([]byte(scopes), &u.Scope)
	return u, nil
}
func (a *App) auth(w http.ResponseWriter, r *http.Request, path string) {
	if !a.allow(r.RemoteAddr) {
		apiError(w, 429, "RATE_LIMIT", "کمی صبر کنید و دوباره تلاش کنید")
		return
	}
	var v struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decode(w, r, &v) {
		return
	}
	if v.Name == "" {
		v.Name = "admin"
	}
	if path == "bootstrap" {
		if len(v.Password) < 8 || len(v.Password) > 72 || len(v.Name) < 2 || len(v.Name) > 80 {
			apiError(w, 400, "VALIDATION", "نام و رمز حداقل ۸ نویسه لازم است")
			return
		}
		pwd, e := bcrypt.GenerateFromPassword([]byte(v.Password), 12)
		if e != nil {
			apiError(w, 500, "PASSWORD", "خطا در ساخت حساب")
			return
		}
		tx, e := a.Store.DB.Begin()
		if e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		defer tx.Rollback()
		var n int
		if e = tx.QueryRow("SELECT count(*) FROM users").Scan(&n); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		if n != 0 {
			apiError(w, 409, "BOOTSTRAP", "پنل قبلاً راه‌اندازی شده است")
			return
		}
		id := ID()
		if _, e = tx.Exec("INSERT INTO users(id,name,password,role) VALUES(?,?,?,'admin')", id, v.Name, pwd); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		if e = tx.Commit(); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		a.Store.Audit(id, "bootstrap", id)
		reply(w, 201, map[string]bool{"created": true})
		return
	}
	var id string
	var pwd []byte
	e := a.Store.DB.QueryRow("SELECT id,password FROM users WHERE name=?", v.Name).Scan(&id, &pwd)
	if e != nil || bcrypt.CompareHashAndPassword(pwd, []byte(v.Password)) != nil {
		apiError(w, 401, "LOGIN", "نام کاربری یا رمز معتبر نیست")
		return
	}
	token, csrf := ID()+ID(), ID()+ID()
	if _, e = a.Store.DB.Exec("INSERT INTO sessions(id,user_id,csrf,expires) VALUES(?,?,?,?)", hash(token), id, csrf, time.Now().Add(12*time.Hour).Unix()); e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "greflow_session", Value: token, Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	a.Store.Audit(id, "login", id)
	reply(w, 200, map[string]string{"csrf": csrf})
}
func (a *App) security(w http.ResponseWriter, r *http.Request, u User, path string) {
	switch path {
	case "users/manage", "auth/sessions", "auth/password":
		a.accountManagement(w, r, u, path)
	case "auth/logout":
		c, _ := r.Cookie("greflow_session")
		if c != nil {
			if _, e := a.Store.DB.Exec("DELETE FROM sessions WHERE id=?", hash(c.Value)); e != nil {
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
		}
		http.SetCookie(w, &http.Cookie{Name: "greflow_session", Path: "/", Value: "", MaxAge: -1, HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode})
		reply(w, 200, map[string]bool{"logged_out": true})
	case "auth/revoke":
		if _, e := a.Store.DB.Exec("DELETE FROM sessions WHERE user_id=?", u.ID); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		a.Store.Audit(u.ID, "revoke_sessions", u.ID)
		reply(w, 200, map[string]bool{"revoked": true})
	case "users":
		if u.Role != "admin" {
			apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
			return
		}
		if r.Method == "GET" {
			rows, e := a.Store.DB.Query("SELECT id,name,role,scope FROM users")
			if e != nil {
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
			defer rows.Close()
			out := []User{}
			for rows.Next() {
				var v User
				var scope string
				rows.Scan(&v.ID, &v.Name, &v.Role, &scope)
				json.Unmarshal([]byte(scope), &v.Scope)
				out = append(out, v)
			}
			reply(w, 200, out)
			return
		}
		var v struct {
			Name     string   `json:"name"`
			Password string   `json:"password"`
			Role     string   `json:"role"`
			Scope    []string `json:"scope"`
		}
		if !decode(w, r, &v) {
			return
		}
		if len(v.Name) < 2 || len(v.Password) < 8 || len(v.Password) > 72 || (v.Role != "operator" && v.Role != "viewer") {
			apiError(w, 400, "VALIDATION", "نقش مشاهده‌گر یا اپراتور و رمز حداقل ۸ نویسه لازم است")
			return
		}
		pwd, e := bcrypt.GenerateFromPassword([]byte(v.Password), 12)
		if e != nil {
			apiError(w, 400, "PASSWORD", e.Error())
			return
		}
		id := ID()
		if _, e = a.Store.DB.Exec("INSERT INTO users(id,name,password,role,scope) VALUES(?,?,?,?,?)", id, v.Name, pwd, v.Role, marshal(v.Scope)); e != nil {
			apiError(w, 409, "USER", "نام تکراری یا خطای ذخیره‌سازی")
			return
		}
		a.Store.Audit(u.ID, "create_user", id)
		reply(w, 201, map[string]string{"id": id})
	default:
		apiError(w, 404, "NOT_FOUND", "مسیر پیدا نشد")
	}
}
