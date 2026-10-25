package console

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	s, e := OpenStore(filepath.Join(dir, "data"), filepath.Join(dir, "keys", "master"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Close() })
	a := New(s, "http://localhost")
	if e = a.Bootstrap(); e != nil {
		t.Fatal(e)
	}
	return a
}
func request(t *testing.T, a *App, path string, body any, cookie, csrf, key string) *httptest.ResponseRecorder {
	t.Helper()
	method := "GET"
	var b io.Reader
	if body != nil {
		method = "POST"
		b = strings.NewReader(marshal(body))
	}
	r := httptest.NewRequest(method, "http://localhost/api/v1/"+path, b)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("Idempotency-Key", key)
	if cookie != "" {
		r.Header.Set("Cookie", cookie)
	}
	w := httptest.NewRecorder()
	a.Handler(http.NotFoundHandler()).ServeHTTP(w, r)
	return w
}
func loginTest(t *testing.T, a *App) (string, string) {
	t.Helper()
	w := request(t, a, "bootstrap", map[string]string{"name": "owner", "password": "test-only-long-password"}, "", "", "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(t, a, "auth/login", map[string]string{"name": "owner", "password": "test-only-long-password"}, "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	return w.Result().Cookies()[0].String(), v["csrf"]
}
func TestBootstrapSessionCSRFAndScope(t *testing.T) {
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	if w := request(t, a, "servers", nil, "", "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "users", map[string]string{}, cookie, "wrong", ID()); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "users", map[string]string{}, cookie, csrf, ""); w.Code != 400 {
		t.Fatal(w.Code)
	}
	w := request(t, a, "users", map[string]any{"name": "viewer", "password": "viewer-password-long", "role": "viewer", "scope": []string{}}, cookie, csrf, ID())
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = request(t, a, "auth/login", map[string]string{"name": "viewer", "password": "viewer-password-long"}, "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	vc := w.Result().Cookies()[0].String()
	if w = request(t, a, "servers", map[string]string{}, vc, v["csrf"], ID()); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w = request(t, a, "auth/revoke", map[string]string{}, cookie, csrf, ID()); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = request(t, a, "session", nil, cookie, "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
}

type brokenTransport struct{ calls int }

func (b *brokenTransport) Call(context.Context, Host, []byte, protocol.Request) (protocol.Response, error) {
	b.calls++
	return protocol.Response{}, fmt.Errorf("connection lost")
}
func addTestHost(t *testing.T, a *App) string {
	t.Helper()
	id := ID()
	key, e := a.Store.Seal([]byte("test-only-key"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = a.Store.DB.Exec("INSERT INTO hosts(id,name,kind,addresses,hostname,port,username,fingerprint,credential) VALUES(?,?,'iran','[]','127.0.0.1',22,'root',?,?)", id, "test", "SHA256:"+strings.Repeat("a", 43), key)
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func TestDurableIdempotencyAndUncertain(t *testing.T) {
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	id := addTestHost(t, a)
	b := &brokenTransport{}
	a.Transport = b
	key := ID()
	body := map[string]any{"host_id": id, "request": protocol.Request{Action: "discover"}}
	w := request(t, a, "operations", body, cookie, csrf, key)
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var op map[string]string
	json.Unmarshal(w.Body.Bytes(), &op)
	w = request(t, a, "operations", body, cookie, csrf, key)
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var n int
	a.Store.DB.QueryRow("SELECT count(*) FROM operations").Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
	if e := a.work(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := a.work(context.Background()); e != nil {
		t.Fatal(e)
	}
	if b.calls != 1 {
		t.Fatal(b.calls)
	}
	var state string
	a.Store.DB.QueryRow("SELECT state FROM operations WHERE id=?", op["id"]).Scan(&state)
	if state != "uncertain" {
		t.Fatal(state)
	}
	body["request"] = protocol.Request{Action: "logs"}
	if w = request(t, a, "operations", body, cookie, csrf, key); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestBackupEncryptionTamperingAndFreshRestore(t *testing.T) {
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	addTestHost(t, a)
	w := request(t, a, "backups", map[string]any{"password": "separate-backup-passphrase", "include_secrets": true}, cookie, csrf, ID())
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	if strings.Contains(v["data"], "test-only-key") {
		t.Fatal("plaintext key")
	}
	if _, e := openBackup(v["data"], "wrong-password"); e == nil {
		t.Fatal("wrong password accepted")
	}
	d, e := openBackup(v["data"], "separate-backup-passphrase")
	if e != nil || len(d.Hosts) != 1 || string(d.Hosts[0].Key) != "test-only-key" {
		t.Fatal(d, e)
	}
	a2 := testApp(t)
	c2, s2 := loginTest(t, a2)
	body := map[string]any{"password": "separate-backup-passphrase", "data": v["data"], "revision": hash(v["data"])}
	w = request(t, a2, "restore", body, c2, s2, ID())
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request(t, a2, "restore", body, c2, s2, ID())
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	var count int
	a2.Store.DB.QueryRow("SELECT count(*) FROM hosts").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
}
func TestLoginRateLimit(t *testing.T) {
	a := testApp(t)
	for i := 0; i < 10; i++ {
		if !a.allow("127.0.0.1:1") {
			t.Fatal(i)
		}
	}
	if a.allow("127.0.0.1:2") {
		t.Fatal("rate bypass")
	}
}
func TestVaultCipher(t *testing.T) {
	a := testApp(t)
	b, e := a.Store.Seal([]byte("secret"))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(b, []byte("secret")) {
		t.Fatal("not encrypted")
	}
	b[len(b)-1] ^= 1
	if _, e = a.Store.Open(b); e == nil {
		t.Fatal("tamper accepted")
	}
}

func TestFullSnapshotRestoresVaultAndAccountsWithoutReplayingWork(t *testing.T) {
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	host := addTestHost(t, a)
	queued := request(t, a, "operations", map[string]any{"host_id": host, "request": protocol.Request{Action: "discover"}}, cookie, csrf, ID())
	if queued.Code != 202 {
		t.Fatal(queued.Body.String())
	}
	w := request(t, a, "backups", map[string]any{"password": "full-snapshot-passphrase", "scope": "full"}, cookie, csrf, ID())
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var data map[string]string
	json.Unmarshal(w.Body.Bytes(), &data)
	b, e := openBackup(data["data"], "full-snapshot-passphrase")
	if e != nil {
		t.Fatal(e)
	}
	target := testApp(t)
	c2, s2 := loginTest(t, target)
	w = request(t, target, "restore", map[string]string{"password": "full-snapshot-passphrase", "data": data["data"], "revision": hash(data["data"])}, c2, s2, ID())
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	_, encrypted, e := target.host(host)
	if e != nil {
		t.Fatal(e)
	}
	plain, e := target.Store.Open(encrypted)
	if e != nil || string(plain) != "test-only-key" {
		t.Fatal(string(plain), e)
	}
	var n int
	target.Store.DB.QueryRow("SELECT count(*) FROM sessions").Scan(&n)
	if n != 0 {
		t.Fatal("sessions restored")
	}
	target.Store.DB.QueryRow("SELECT count(*) FROM operations WHERE state='uncertain'").Scan(&n)
	if n != 1 {
		t.Fatal("queued work could replay", n)
	}
	if len(b.Snapshot["users"].Rows) != 1 {
		t.Fatal("users missing")
	}
}

func TestSynchronousMutationReceiptPreventsDuplicateAndInterruptedReplay(t *testing.T) {
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	key := ID()
	body := map[string]any{"name": "readonly", "password": "readonly-long-password", "role": "viewer", "scope": []string{}}
	first := request(t, a, "users", body, cookie, csrf, key)
	second := request(t, a, "users", body, cookie, csrf, key)
	if first.Code != 201 || second.Code != 201 || first.Body.String() != second.Body.String() {
		t.Fatal(first.Code, second.Code, second.Body.String())
	}
	var n int
	a.Store.DB.QueryRow("SELECT count(*) FROM users WHERE name='readonly'").Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
	body["name"] = "different"
	if w := request(t, a, "users", body, cookie, csrf, key); w.Code != 409 {
		t.Fatal(w.Code)
	}
	var actor string
	a.Store.DB.QueryRow("SELECT id FROM users WHERE name='owner'").Scan(&actor)
	interrupted := ID()
	payload := map[string]any{"enabled": true, "minutes": 1, "retention_days": 30}
	digest := hash("/api/v1/monitoring\n" + marshal(payload))
	a.Store.DB.Exec("INSERT INTO mutation_receipts(actor,idem,digest,created_at) VALUES(?,?,?,?)", actor, interrupted, digest, time.Now().UTC().Format(time.RFC3339Nano))
	if w := request(t, a, "monitoring", payload, cookie, csrf, interrupted); w.Code != 409 || !strings.Contains(w.Body.String(), "UNCERTAIN") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestRealSSHHandshakePinningAndFixedDispatcher(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, e := ssh.NewSignerFromKey(priv)
	if e != nil {
		t.Fatal(e)
	}
	pk, e := ssh.MarshalPrivateKey(priv, "test")
	if e != nil {
		t.Fatal(e)
	}
	private := pem.EncodeToMemory(pk)
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(k.Marshal(), signer.PublicKey().Marshal()) {
			return nil, fmt.Errorf("wrong key")
		}
		return nil, nil
	}}
	cfg.AddHostKey(signer)
	cfg.PasswordCallback = func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if meta.User() != "root" || string(password) != "test-only-root-password" {
			return nil, fmt.Errorf("invalid login")
		}
		return nil, nil
	}
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				sc, chs, reqs, e := ssh.NewServerConn(c, cfg)
				if e != nil {
					return
				}
				defer sc.Close()
				go ssh.DiscardRequests(reqs)
				probeResult := ""
				for ch := range chs {
					if ch.ChannelType() != "session" {
						ch.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, requests, e := ch.Accept()
					if e != nil {
						return
					}
					for r := range requests {
						if r.Type != "exec" {
							r.Reply(false, nil)
							continue
						}
						var command struct{ Command string }
						ssh.Unmarshal(r.Payload, &command)
						if command.Command == helperProbeCommand && probeResult != "" {
							r.Reply(true, nil)
							io.WriteString(channel, probeResult)
							channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
							channel.Close()
							continue
						}
						if command.Command == "/usr/local/bin/greflow-agent version" {
							r.Reply(true, nil)
							io.WriteString(channel, "GreFlow dispatcher protocol 1\n")
							channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
							channel.Close()
							continue
						}
						if command.Command != dispatcherCommand(sc.User()) {
							r.Reply(false, nil)
							channel.Close()
							return
						}
						r.Reply(true, nil)
						var request protocol.Request
						json.NewDecoder(channel).Decode(&request)
						switch request.Name {
						case "missing-helper":
							probeResult = "GREFLOW_HELPER_MISSING"
						case "disabled-helper":
							probeResult = "GREFLOW_HELPER_NOT_EXECUTABLE"
						case "old-helper":
							probeResult = "GreFlow dispatcher protocol 0"
						case "failed-helper":
							probeResult = "GreFlow dispatcher protocol 1"
						}
						if probeResult != "" {
							io.WriteString(channel.Stderr(), "sensitive-shell-output-must-not-leak")
							channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{127}))
							channel.Close()
							continue
						}
						json.NewEncoder(channel).Encode(protocol.Response{Version: 1, State: "succeeded", Logs: request.Name})
						channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
						channel.Close()
					}
				}
			}(c)
		}
	}()
	h := Host{Name: "isolated test", Kind: "iran", Hostname: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Username: "root", Fingerprint: ssh.FingerprintSHA256(signer.PublicKey())}
	res, e := (SSHTransport{}).Call(context.Background(), h, private, protocol.Request{Version: 1, ID: ID(), Action: "logs", Name: "exit2"})
	if e != nil || res.Logs != "exit2" {
		t.Fatal(res, e)
	}
	for _, tc := range []struct{ name, user, want string }{
		{"missing-helper", "root", "نصب نیست"},
		{"disabled-helper", "root", "اجازهٔ اجرا ندارد"},
		{"old-helper", "root", "سازگار نیست"},
		{"failed-helper", "root", "کد 127"},
		{"failed-helper", "operator", "sudo"},
	} {
		h.Username = tc.user
		_, e := (SSHTransport{}).Call(context.Background(), h, private, protocol.Request{Version: 1, ID: ID(), Action: "logs", Name: tc.name})
		if e == nil || !strings.Contains(e.Error(), tc.want) || strings.Contains(e.Error(), "sensitive-shell-output") {
			t.Fatalf("%s/%s: %v", tc.name, tc.user, e)
		}
	}
	h.Username = "root"
	passwordCredential, e := credentialFromHost(Host{AuthMethod: "password", Password: "test-only-root-password"})
	if e != nil {
		t.Fatal(e)
	}
	res, e = (SSHTransport{}).Call(context.Background(), h, passwordCredential, protocol.Request{Version: 1, ID: ID(), Action: "logs", Name: "exit2"})
	if e != nil || res.Logs != "exit2" {
		t.Fatal("password SSH", res, e)
	}
	h.Username = "operator"
	res, e = (SSHTransport{}).Call(context.Background(), h, private, protocol.Request{Version: 1, ID: ID(), Action: "logs", Name: "exit2"})
	if e != nil || res.Logs != "exit2" {
		t.Fatal("non-root sudo", res, e)
	}
	h.Username = "root"
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	h.Password = "test-only-root-password"
	h.AuthMethod = "password"
	h.Fingerprint = ""
	w := request(t, a, "ssh/check", h, cookie, csrf, ID())
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"helper_ready":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	var checked struct {
		Fingerprint string `json:"fingerprint"`
	}
	json.Unmarshal(w.Body.Bytes(), &checked)
	h.Fingerprint = checked.Fingerprint
	w = request(t, a, "servers", h, cookie, csrf, ID())
	if w.Code != 201 || strings.Contains(w.Body.String(), h.Password) {
		t.Fatal("create leaked credential or failed", w.Code, w.Body.String())
	}
	var created Host
	if e := json.Unmarshal(w.Body.Bytes(), &created); e != nil || created.PreparationID == "" {
		t.Fatal("automatic preparation not returned", e)
	}
	var queuedAction string
	if e := a.Store.DB.QueryRow("SELECT json_extract(request,'$.action') FROM operations WHERE id=? AND host_id=? AND state='queued'", created.PreparationID, created.ID).Scan(&queuedAction); e != nil || queuedAction != "prepare" {
		t.Fatal("automatic preparation not queued atomically", e)
	}
	w = request(t, a, "servers", nil, cookie, "", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), h.Password) || !strings.Contains(w.Body.String(), `"auth_method":"password"`) {
		t.Fatal("inventory leaked secret", w.Body.String())
	}
	var encrypted []byte
	a.Store.DB.QueryRow("SELECT credential FROM hosts LIMIT 1").Scan(&encrypted)
	if bytes.Contains(encrypted, []byte(h.Password)) {
		t.Fatal("password stored in plaintext")
	}
	h.Fingerprint = "SHA256:" + strings.Repeat("a", 43)
	if _, e = (SSHTransport{}).Call(context.Background(), h, private, protocol.Request{}); e == nil {
		t.Fatal("changed key accepted")
	}
}

func TestSimpleSetupNoCodeAndRetiredTOTP(t *testing.T) {
	a := testApp(t)
	if _, e := os.Stat(filepath.Join(a.Store.Dir, "bootstrap-token")); !os.IsNotExist(e) {
		t.Fatal("obsolete setup code created")
	}
	w := request(t, a, "bootstrap", map[string]string{"password": "simple-test-password"}, "", "", "")
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = request(t, a, "bootstrap", map[string]string{"password": "another-test-password"}, "", "", "")
	if w.Code != 409 {
		t.Fatal("setup replaced existing account")
	}
	w = request(t, a, "auth/login", map[string]string{"password": "simple-test-password"}, "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var v map[string]string
	json.Unmarshal(w.Body.Bytes(), &v)
	cookie := w.Result().Cookies()[0].String()
	w = request(t, a, "auth/totp", map[string]string{}, cookie, v["csrf"], ID())
	if w.Code != 404 {
		t.Fatal("TOTP remains enabled")
	}
	a.Store.DB.Exec("DELETE FROM schema_migrations WHERE version=4")
	a.Store.DB.Exec("UPDATE users SET totp=?,last_totp=99", []byte("legacy-value"))
	a.Store.DB.Exec("INSERT INTO recovery(user_id,code) SELECT id,'legacy-recovery-code' FROM users")
	if e := a.Store.migrate(); e != nil {
		t.Fatal(e)
	}
	var n int
	a.Store.DB.QueryRow("SELECT count(*) FROM users WHERE totp IS NOT NULL OR last_totp!=0").Scan(&n)
	if n != 0 {
		t.Fatal("legacy TOTP retained")
	}
	a.Store.DB.QueryRow("SELECT count(*) FROM recovery").Scan(&n)
	if n != 0 {
		t.Fatal("legacy recovery codes retained")
	}
}
