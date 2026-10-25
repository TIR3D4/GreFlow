package console

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMTLSEnrollmentSingleUseRevocationAndQueueIsolation(t *testing.T) {
	a := testApp(t)
	a.AgentURL = "https://agent.example.test"
	cookie, csrf := loginTest(t, a)
	w := request(t, a, "enrollments", map[string]string{"name": "isolated agent", "kind": "iran"}, cookie, csrf, ID())
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var ticket struct {
		Token  string `json:"token"`
		HostID string `json:"host_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &ticket)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	csr, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "untrusted-subject"}}, key)
	config, e := a.AgentTLS()
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewUnstartedServer(a.AgentHandler())
	server.TLS = config
	server.StartTLS()
	defer server.Close()
	client := server.Client()
	call := func(c *http.Client, path string, v any) (int, []byte) {
		r, e := c.Post(server.URL+path, "application/json", strings.NewReader(marshal(v)))
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		var b bytes.Buffer
		b.ReadFrom(r.Body)
		return r.StatusCode, b.Bytes()
	}
	body := map[string]any{"token": ticket.Token, "csr": csr}
	status, b := call(client, "/agent/v1/enroll", body)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	if status, _ = call(client, "/agent/v1/enroll", body); status != 403 {
		t.Fatal("reused token", status)
	}
	var issued struct {
		Certificate []byte `json:"certificate"`
	}
	json.Unmarshal(b, &issued)
	pk, _ := x509.MarshalPKCS8PrivateKey(key)
	cert, e := tls.X509KeyPair(issued.Certificate, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}))
	if e != nil {
		t.Fatal(e)
	}
	transport := client.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	authenticated := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	if status, _ = call(client, "/agent/v1/poll", map[string]string{}); status != 401 {
		t.Fatal("missing certificate", status)
	}
	op := request(t, a, "operations", map[string]any{"host_id": ticket.HostID, "request": protocol.Request{Action: "discover"}}, cookie, csrf, ID())
	if op.Code != 202 {
		t.Fatal(op.Body.String())
	}
	status, b = call(authenticated, "/agent/v1/poll", map[string]string{})
	if status != 200 || !bytes.Contains(b, []byte("discover")) {
		t.Fatal(status, string(b))
	}
	status, b = call(authenticated, "/agent/v1/poll", map[string]string{})
	if status != 200 || !bytes.Contains(b, []byte("null")) {
		t.Fatal("duplicate claim", string(b))
	}
	w = request(t, a, "enrollments", map[string]string{"revoke": ticket.HostID}, cookie, csrf, ID())
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if status, _ = call(authenticated, "/agent/v1/poll", map[string]string{}); status != 403 {
		t.Fatal("revoked cert", status)
	}
}
func TestIncidentDedupRecoveryAndStale(t *testing.T) {
	a := testApp(t)
	now := time.Now()
	tunnel := protocol.Tunnel{Name: "one", Checks: []protocol.Evidence{{Kind: "gre_peer", Status: "failed", ObservedAt: now, ExpiresAt: now.Add(time.Minute)}}}
	a.recordIncident("host", tunnel)
	a.recordIncident("host", tunnel)
	var n int
	a.Store.DB.QueryRow("SELECT count(*) FROM deliveries").Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
	tunnel.Checks[0].Status = "ok"
	a.recordIncident("host", tunnel)
	a.Store.DB.QueryRow("SELECT count(*) FROM deliveries").Scan(&n)
	if n != 2 {
		t.Fatal(n)
	}
	tunnel.Checks[0].ExpiresAt = now.Add(-time.Second)
	if incidentState(tunnel) != "stale" {
		t.Fatal("stale healthy")
	}
}
