package console

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"math/big"
	"net/http"
	"net/url"
	"time"
)

type caBundle struct {
	Certificate []byte `json:"certificate"`
	Key         []byte `json:"key"`
}

func (a *App) agentCA() (*x509.Certificate, ed25519.PrivateKey, error) {
	var encrypted []byte
	e := a.Store.DB.QueryRow("SELECT body FROM settings WHERE id='agent-ca'").Scan(&encrypted)
	var b caBundle
	if e != nil {
		pub, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return nil, nil, e
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "GreFlow private agent CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(5, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
		der, e := x509.CreateCertificate(rand.Reader, template, template, pub, key)
		if e != nil {
			return nil, nil, e
		}
		b = caBundle{Certificate: der, Key: key}
		encrypted, e = a.Store.Seal([]byte(marshal(b)))
		if e != nil {
			return nil, nil, e
		}
		if _, e = a.Store.DB.Exec("INSERT INTO settings(id,body) VALUES('agent-ca',?)", encrypted); e != nil {
			return nil, nil, e
		}
	} else {
		plain, e := a.Store.Open(encrypted)
		if e != nil {
			return nil, nil, e
		}
		if e = json.Unmarshal(plain, &b); e != nil {
			return nil, nil, e
		}
	}
	cert, e := x509.ParseCertificate(b.Certificate)
	return cert, ed25519.PrivateKey(b.Key), e
}
func (a *App) AgentTLS() (*tls.Config, error) {
	cert, _, e := a.agentCA()
	if e != nil {
		return nil, e
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.VerifyClientCertIfGiven, ClientCAs: pool}, nil
}
func (a *App) enrollmentAdmin(w http.ResponseWriter, r *http.Request, u User) {
	if u.Role != "admin" {
		apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
		return
	}
	if r.Method == "GET" {
		rows, e := a.Store.DB.Query("SELECT host_id,serial,revoked,heartbeat FROM agents")
		if e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var host, serial string
			var revoked int
			var heartbeat *string
			rows.Scan(&host, &serial, &revoked, &heartbeat)
			out = append(out, map[string]any{"host_id": host, "serial": serial, "revoked": revoked == 1, "heartbeat": heartbeat})
		}
		reply(w, 200, out)
		return
	}
	var v struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Revoke string `json:"revoke"`
	}
	if !decode(w, r, &v) {
		return
	}
	if v.Revoke != "" {
		tx, e := a.Store.DB.Begin()
		if e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		defer tx.Rollback()
		tx.Exec("UPDATE agents SET revoked=1 WHERE host_id=?", v.Revoke)
		tx.Exec("UPDATE enrollments SET used=1 WHERE host_id=?", v.Revoke)
		if e = tx.Commit(); e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		a.Store.Audit(u.ID, "revoke_agent", v.Revoke)
		reply(w, 200, map[string]bool{"revoked": true})
		return
	}
	endpoint, e := url.Parse(a.AgentURL)
	if e != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		apiError(w, 409, "AGENT_HTTPS", "عامل خروجی به listener HTTPS قابل دسترس با گواهی معتبر نیاز دارد")
		return
	}
	if v.Name == "" || (v.Kind != "iran" && v.Kind != "outside") {
		apiError(w, 400, "VALIDATION", "نام و نوع میزبان لازم است")
		return
	}
	id, token := ID(), ID()+ID()
	credential, e := a.Store.Seal(nil)
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
	_, e = tx.Exec("INSERT INTO hosts(id,name,kind,addresses,hostname,port,username,fingerprint,credential) VALUES(?,?,?,'[]','agent',22,'root','agent',?)", id, v.Name, v.Kind, credential)
	if e == nil {
		_, e = tx.Exec("INSERT INTO agents(host_id,serial,revoked) VALUES(?,'pending-'||?,1)", id, id)
	}
	expires := time.Now().Add(10 * time.Minute)
	if e == nil {
		_, e = tx.Exec("INSERT INTO enrollments(id,host_id,token_hash,expires) VALUES(?,?,?,?)", ID(), id, hash(token), expires.Unix())
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	a.Store.Audit(u.ID, "create_enrollment", id)
	reply(w, 201, map[string]any{"host_id": id, "token": token, "expires_at": expires.UTC(), "url": a.AgentURL})
}
func (a *App) AgentHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.TLS == nil || r.Method != "POST" {
			apiError(w, 403, "TLS", "TLS POST required")
			return
		}
		if r.URL.Path == "/agent/v1/enroll" {
			a.enrollAgent(w, r)
			return
		}
		if len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
			apiError(w, 401, "MTLS", "client certificate required")
			return
		}
		cert := r.TLS.PeerCertificates[0]
		var host string
		if e := a.Store.DB.QueryRow("SELECT host_id FROM agents WHERE revoked=0 AND (serial=? OR host_id IN (SELECT host_id FROM agent_certificates WHERE serial=? AND expires>?))", cert.SerialNumber.String(), cert.SerialNumber.String(), time.Now().Unix()).Scan(&host); e != nil || host != cert.Subject.CommonName {
			apiError(w, 403, "REVOKED", "agent revoked or unknown")
			return
		}
		if r.URL.Path == "/agent/v1/renew" {
			a.renewAgent(w, r, host)
			return
		}
		if r.URL.Path == "/agent/v1/poll" {
			a.Store.DB.Exec("UPDATE agents SET heartbeat=? WHERE host_id=?", time.Now().UTC().Format(time.RFC3339Nano), host)
			tx, e := a.Store.DB.Begin()
			if e != nil {
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
			defer tx.Rollback()
			var running int
			tx.QueryRow("SELECT count(*) FROM operations WHERE host_id=? AND state='running'", host).Scan(&running)
			if running > 0 {
				reply(w, 200, map[string]any{"request": nil})
				return
			}
			var id, body string
			e = tx.QueryRow("SELECT id,request FROM operations WHERE host_id=? AND state='queued' ORDER BY created_at LIMIT 1", host).Scan(&id, &body)
			if e != nil {
				reply(w, 200, map[string]any{"request": nil})
				return
			}
			now := time.Now().UTC().Format(time.RFC3339Nano)
			if _, e = tx.Exec("UPDATE operations SET state='running',updated_at=? WHERE id=? AND state='queued'", now, id); e != nil {
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
			tx.Exec("INSERT INTO events(operation_id,at,state) VALUES(?,?,'running')", id, now)
			if e = tx.Commit(); e != nil {
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
			var req protocol.Request
			json.Unmarshal([]byte(body), &req)
			reply(w, 200, map[string]any{"request": req})
			return
		}
		if r.URL.Path == "/agent/v1/result" {
			var v struct {
				ID       string            `json:"id"`
				Response protocol.Response `json:"response"`
			}
			if !decode(w, r, &v) {
				return
			}
			var body, state string
			if e := a.Store.DB.QueryRow("SELECT request,state FROM operations WHERE id=? AND host_id=?", v.ID, host).Scan(&body, &state); e != nil {
				apiError(w, 404, "OPERATION", "unknown operation")
				return
			}
			if state != "running" && state != "uncertain" {
				reply(w, 200, map[string]bool{"acknowledged": true})
				return
			}
			switch v.Response.State {
			case "succeeded", "failed", "uncertain", "rolled_back", "rollback_failed":
			default:
				apiError(w, 400, "STATE", "invalid result state")
				return
			}
			var req protocol.Request
			json.Unmarshal([]byte(body), &req)
			if e := a.complete(v.ID, host, req, v.Response, nil); e != nil {
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
			reply(w, 200, map[string]bool{"acknowledged": true})
			return
		}
		apiError(w, 404, "NOT_FOUND", "unknown endpoint")
	})
}
func (a *App) enrollAgent(w http.ResponseWriter, r *http.Request) {
	if !a.allow(r.RemoteAddr) {
		apiError(w, 429, "RATE_LIMIT", "try later")
		return
	}
	var v struct {
		Token string `json:"token"`
		CSR   []byte `json:"csr"`
	}
	if !decode(w, r, &v) {
		return
	}
	csr, e := x509.ParseCertificateRequest(v.CSR)
	if e != nil || csr.CheckSignature() != nil {
		apiError(w, 400, "CSR", "invalid CSR")
		return
	}
	if _, ok := csr.PublicKey.(ed25519.PublicKey); !ok {
		apiError(w, 400, "KEY", "Ed25519 required")
		return
	}
	ca, key, e := a.agentCA()
	if e != nil {
		apiError(w, 500, "CA", "CA unavailable")
		return
	}
	tx, e := a.Store.DB.Begin()
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	defer tx.Rollback()
	var host string
	if e = tx.QueryRow("SELECT host_id FROM enrollments WHERE token_hash=? AND used=0 AND expires>?", hash(v.Token), time.Now().Unix()).Scan(&host); e != nil {
		apiError(w, 403, "TOKEN", "expired, revoked or consumed token")
		return
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: host}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(0, 1, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, ca, csr.PublicKey, key)
	if e != nil {
		apiError(w, 500, "CERT", "signing failed")
		return
	}
	if _, e = tx.Exec("UPDATE enrollments SET used=1 WHERE token_hash=? AND used=0", hash(v.Token)); e == nil {
		_, e = tx.Exec("UPDATE agents SET serial=?,revoked=0 WHERE host_id=?", serial.String(), host)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		apiError(w, 409, "TOKEN", "enrollment conflict")
		return
	}
	reply(w, 200, map[string]any{"certificate": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), "host_id": host, "expires_at": template.NotAfter})
}

func (a *App) renewAgent(w http.ResponseWriter, r *http.Request, host string) {
	var v struct {
		CSR []byte `json:"csr"`
	}
	if !decode(w, r, &v) {
		return
	}
	csr, e := x509.ParseCertificateRequest(v.CSR)
	if e != nil || csr.CheckSignature() != nil {
		apiError(w, 400, "CSR", "invalid CSR")
		return
	}
	if _, ok := csr.PublicKey.(ed25519.PublicKey); !ok {
		apiError(w, 400, "KEY", "Ed25519 required")
		return
	}
	ca, key, e := a.agentCA()
	if e != nil {
		apiError(w, 500, "CA", "CA unavailable")
		return
	}
	serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		apiError(w, 500, "RANDOM", "entropy unavailable")
		return
	}
	until := time.Now().AddDate(0, 1, 0)
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: host}, NotBefore: time.Now().Add(-time.Minute), NotAfter: until, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, ca, csr.PublicKey, key)
	if e != nil {
		apiError(w, 500, "CERT", "signing failed")
		return
	}
	if _, e = a.Store.DB.Exec("INSERT INTO agent_certificates(serial,host_id,expires) VALUES(?,?,?)", serial.String(), host, until.Unix()); e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	reply(w, 200, map[string]any{"certificate": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), "expires_at": until})
}
