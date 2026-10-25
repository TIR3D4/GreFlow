package console

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/internal/protocol"
)

const infoFixture = `<div id="ip_info-dbip"><table><tr><td>IP address</td><td><strong>1.1.1.1</strong></td></tr><tr><td>Country</td><td><img src="/x"><strong>Australia</strong> (AU)</td></tr><tr><td>City</td><td>Sydney</td></tr><tr><td>Region</td><td>New South Wales</td></tr><tr><td>ISP / Org</td><td>Cloudflare, Inc.</td></tr><tr><td>ASN</td><td>13335</td></tr></table></div>`

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIPInfoParsingScopeCachingAndExitRole(t *testing.T) {
	var info IPInfo
	if e := parseIPInfo([]byte(infoFixture), "1.1.1.1", &info); e != nil || info.CountryCode != "au" || info.Organization != "Cloudflare, Inc." || info.Source != "Check-Host / dbip" {
		t.Fatal(info, e)
	}
	if e := parseIPInfo([]byte(infoFixture), "8.8.8.8", &IPInfo{}); e == nil {
		t.Fatal("mismatched IP accepted")
	}
	conflict := infoFixture + strings.Replace(infoFixture, "Australia</strong> (AU)", "Germany</strong> (DE)", 1)
	if e := parseIPInfo([]byte(conflict), "1.1.1.1", &info); e != nil || !info.CountryConflict {
		t.Fatal("country conflict hidden")
	}
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "224.0.0.1", "::1", "1.1.1.1/?host=evil"} {
		if publicIP(ip) {
			t.Fatal("non-public IP accepted", ip)
		}
	}
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	host := addTestHost(t, a)
	inventory := protocol.Inventory{HostID: "test", Tunnels: []protocol.Tunnel{{Name: "exit", Desired: config.Config{Role: "exit", Local: "1.1.1.1", Remote: "8.8.8.8"}}}}
	a.Store.DB.Exec("UPDATE hosts SET inventory=? WHERE id=?", marshal(inventory), host)
	calls := 0
	a.Geo.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "check-host.net" || r.URL.Query().Get("host") != "1.1.1.1" {
			t.Error("wrong outside endpoint", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(infoFixture))}, nil
	})}
	for i := 0; i < 2; i++ {
		w := request(t, a, "ip-info?host_id="+host+"&tunnel=exit", nil, cookie, "", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"country_code":"au"`) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatal("lookup not cached", calls)
	}
	w := request(t, a, "users", map[string]any{"name": "viewer", "password": "viewer-test-password", "role": "viewer", "scope": []string{}}, cookie, csrf, ID())
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = request(t, a, "auth/login", map[string]string{"name": "viewer", "password": "viewer-test-password"}, "", "", "")
	w = request(t, a, "ip-info?host_id="+host+"&tunnel=exit", nil, w.Result().Cookies()[0].String(), "", "")
	if w.Code != 403 || calls != 1 {
		t.Fatal("scope bypass")
	}
	var cached []byte
	a.Store.DB.QueryRow("SELECT body FROM settings WHERE id='ipinfo:1.1.1.1'").Scan(&cached)
	json.Unmarshal(cached, &info)
	info.ExpiresAt = time.Now().Add(-time.Hour)
	a.Store.DB.Exec("UPDATE settings SET body=? WHERE id='ipinfo:1.1.1.1'", []byte(marshal(info)))
	a.Geo.Client = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader("rate limited"))}, nil
	})}
	oldObserved := info.ObservedAt
	info, e := a.Geo.lookup(context.Background(), "1.1.1.1")
	if e != nil || info.Status != "stale" || info.ObservedAt != oldObserved || !strings.Contains(info.Error, "429") {
		t.Fatal("old geography passed as fresh", info, e)
	}
}

func TestServerActionsAndLogRedaction(t *testing.T) {
	if _, e := serverCommand("root", "server_reboot", "bad;touch /tmp/unsafe"); e == nil {
		t.Fatal("unsafe id accepted")
	}
	command, e := serverCommand("operator", "server_reboot", ID())
	if e != nil || !strings.HasPrefix(command, "sudo -n systemd-run ") || !strings.Contains(command, "--on-active=5s") {
		t.Fatal(command, e)
	}
	key, _ := credentialFromHost(Host{AuthMethod: "password", Password: "private-example-password"})
	log := safeServerLog("private-example-password token=hidden\nnormal line", key)
	if strings.Contains(log, "private-example-password") || strings.Contains(log, "hidden") || !strings.Contains(log, "normal line") {
		t.Fatal("redaction failed")
	}
}
