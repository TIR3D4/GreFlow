package console

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

type Notification struct {
	Token   string `json:"token,omitempty"`
	ChatID  string `json:"chat_id"`
	Enabled bool   `json:"enabled"`
	Test    bool   `json:"test,omitempty"`
}

func (a *App) notifications(w http.ResponseWriter, r *http.Request, u User) {
	if u.Role != "admin" {
		apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
		return
	}
	var stored []byte
	var n Notification
	e := a.Store.DB.QueryRow("SELECT body FROM settings WHERE id='telegram'").Scan(&stored)
	if e == nil {
		plain, e := a.Store.Open(stored)
		if e != nil {
			apiError(w, 500, "KEY", e.Error())
			return
		}
		json.Unmarshal(plain, &n)
	}
	if r.Method == "GET" {
		n.Token = ""
		reply(w, 200, n)
		return
	}
	var v Notification
	if !decode(w, r, &v) {
		return
	}
	if v.Token == "" {
		v.Token = n.Token
	}
	if !regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`).MatchString(v.Token) || !regexp.MustCompile(`^-?[0-9]+$`).MatchString(v.ChatID) {
		apiError(w, 400, "TELEGRAM", "توکن و شناسهٔ مقصد معتبر لازم است")
		return
	}
	if v.Test {
		if e := sendTelegram(v, "GreFlow: پیام آزمایش اتصال هشدارها"); e != nil {
			apiError(w, 502, "TELEGRAM", e.Error())
			return
		}
		a.Store.Audit(u.ID, "telegram_test", v.ChatID)
		reply(w, 200, map[string]bool{"sent": true})
		return
	}
	b, e := a.Store.Seal([]byte(marshal(v)))
	if e != nil {
		apiError(w, 500, "KEY", e.Error())
		return
	}
	if _, e = a.Store.DB.Exec("INSERT INTO settings(id,body) VALUES('telegram',?) ON CONFLICT(id) DO UPDATE SET body=excluded.body", b); e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	a.Store.Audit(u.ID, "telegram_settings", v.ChatID)
	reply(w, 200, map[string]bool{"saved": true})
}
func sendTelegram(n Notification, message string) error {
	body := []byte(marshal(map[string]string{"chat_id": n.ChatID, "text": message}))
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("redirect refused") }}
	res, e := client.Post("https://api.telegram.org/bot"+n.Token+"/sendMessage", "application/json", bytes.NewReader(body))
	if e != nil {
		return fmt.Errorf("Telegram connection failed")
	}
	defer res.Body.Close()
	var result struct {
		OK bool `json:"ok"`
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&result); e != nil || res.StatusCode != 200 || !result.OK {
		return fmt.Errorf("Telegram rejected notification")
	}
	return nil
}
