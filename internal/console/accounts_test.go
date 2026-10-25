package console

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAccountChangesRevokeSessionsAndProtectOwner(t *testing.T) {
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	w := request(t, a, "users", map[string]any{"name": "limited", "password": "long-test-password", "role": "operator", "scope": []string{}}, cookie, csrf, ID())
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var created map[string]string
	json.Unmarshal(w.Body.Bytes(), &created)
	w = request(t, a, "auth/login", map[string]string{"name": "limited", "password": "long-test-password"}, "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	otherCookie := w.Result().Cookies()[0].String()
	var session map[string]string
	json.Unmarshal(w.Body.Bytes(), &session)
	w = request(t, a, "users/manage", map[string]any{"id": created["id"], "action": "update", "role": "viewer", "scope": []string{}}, cookie, csrf, ID())
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(t, a, "session", nil, otherCookie, "", ""); w.Code != 401 {
		t.Fatal("old session survived scope change")
	}
	var owner string
	a.Store.DB.QueryRow("SELECT id FROM users WHERE role='admin'").Scan(&owner)
	if w = request(t, a, "users/manage", map[string]string{"id": owner, "action": "delete"}, cookie, csrf, ID()); w.Code != 409 {
		t.Fatal("owner deletion allowed", w.Body.String())
	}
	if w = request(t, a, "users/manage", map[string]string{"id": created["id"], "action": "delete"}, cookie, csrf, ID()); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request(t, a, "auth/sessions", nil, cookie, "", "")
	var sessions []map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &sessions) != nil || len(sessions) != 1 || sessions[0]["current"] != true {
		t.Fatal(w.Body.String())
	}
	w = request(t, a, "auth/password", map[string]string{"current": "test-only-long-password", "password": "changed-long-test-password"}, cookie, csrf, ID())
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(t, a, "session", nil, cookie, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatal("password change retained session")
	}
	w = request(t, a, "auth/login", map[string]string{"name": "owner", "password": "changed-long-test-password"}, "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}
