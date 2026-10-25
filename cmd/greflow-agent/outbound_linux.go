package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"github.com/TIR3D4/GreFlow/core/manager"
	"github.com/TIR3D4/GreFlow/core/system"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"github.com/TIR3D4/GreFlow/internal/providers/gre"
	"golang.org/x/sys/unix"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const agentConfig = "/etc/greflow/console-agent.json"

type outboundConfig struct {
	URL         string `json:"url"`
	Key         []byte `json:"key"`
	Certificate []byte `json:"certificate"`
}

func post(client *http.Client, url string, v, out any) error {
	b, _ := json.Marshal(v)
	r, e := client.Post(url, "application/json", bytes.NewReader(b))
	if e != nil {
		return fmt.Errorf("HTTPS request failed")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("control plane returned HTTP %d", r.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(out)
}
func enroll(endpoint string) error {
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("public HTTPS agent URL required")
	}
	if _, e = os.Stat(agentConfig); e == nil {
		return fmt.Errorf("agent identity already exists; revoke and archive it before enrolling again")
	}
	fmt.Fprint(os.Stderr, "One-time enrollment token (hidden): ")
	fd := int(os.Stdin.Fd())
	term, e := unix.IoctlGetTermios(fd, unix.TCGETS)
	if e == nil {
		copy := *term
		copy.Lflag &^= unix.ECHO
		unix.IoctlSetTermios(fd, unix.TCSETS, &copy)
		defer unix.IoctlSetTermios(fd, unix.TCSETS, term)
	}
	token, e := bufio.NewReader(io.LimitReader(os.Stdin, 256)).ReadString('\n')
	fmt.Fprintln(os.Stderr)
	if e != nil && e != io.EOF {
		return e
	}
	token = strings.TrimSpace(token)
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	csr, e := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "enrolling"}}, key)
	if e != nil {
		return e
	}
	var result struct {
		Certificate []byte    `json:"certificate"`
		HostID      string    `json:"host_id"`
		ExpiresAt   time.Time `json:"expires_at"`
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirect refused") }}
	if e = post(client, strings.TrimRight(endpoint, "/")+"/agent/v1/enroll", map[string]any{"token": token, "csr": csr}, &result); e != nil {
		return e
	}
	pk, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		return e
	}
	c := outboundConfig{URL: strings.TrimRight(endpoint, "/"), Key: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}), Certificate: result.Certificate}
	if e = config.Save(agentConfig, c); e != nil {
		return e
	}
	fmt.Printf("Enrolled host %s; certificate expires %s. Enable greflow-agent.service to poll.\n", result.HostID, result.ExpiresAt.UTC().Format(time.RFC3339))
	return nil
}
func outbound() error {
	b, e := os.ReadFile(agentConfig)
	if e != nil {
		return e
	}
	var c outboundConfig
	if e = json.Unmarshal(b, &c); e != nil {
		return e
	}
	cert, e := tls.X509KeyPair(c.Certificate, c.Key)
	if e != nil {
		return e
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}}}, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirect refused") }}
	m := manager.Manager{R: system.Exec{}}
	pendingPath := "/var/lib/greflow/console-agent-result.json"
	for {
		leaf, e := x509.ParseCertificate(cert.Certificate[0])
		if e != nil {
			return e
		}
		if time.Until(leaf.NotAfter) < 7*24*time.Hour {
			block, _ := pem.Decode(c.Key)
			if block == nil {
				return fmt.Errorf("invalid local key")
			}
			private, e := x509.ParsePKCS8PrivateKey(block.Bytes)
			if e != nil {
				return e
			}
			csr, e := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: leaf.Subject.CommonName}}, private)
			if e != nil {
				return e
			}
			var renewed struct {
				Certificate []byte `json:"certificate"`
			}
			if e = post(client, c.URL+"/agent/v1/renew", map[string]any{"csr": csr}, &renewed); e == nil {
				next, e := tls.X509KeyPair(renewed.Certificate, c.Key)
				if e != nil {
					return e
				}
				c.Certificate = renewed.Certificate
				if e = config.Save(agentConfig, c); e != nil {
					return e
				}
				cert = next
				client.CloseIdleConnections()
				client.Transport = &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}}}
			}
		}
		if b, e := os.ReadFile(pendingPath); e == nil {
			var result struct {
				ID       string            `json:"id"`
				Response protocol.Response `json:"response"`
			}
			if e = json.Unmarshal(b, &result); e != nil {
				return e
			}
			var ack map[string]bool
			if e = post(client, c.URL+"/agent/v1/result", result, &ack); e == nil && ack["acknowledged"] {
				os.Remove(pendingPath)
			} else {
				time.Sleep(15 * time.Second)
				continue
			}
		}
		var work struct {
			Request *protocol.Request `json:"request"`
		}
		if e = post(client, c.URL+"/agent/v1/poll", map[string]string{}, &work); e != nil {
			fmt.Fprintln(os.Stderr, "Control plane unavailable; retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}
		if work.Request == nil {
			time.Sleep(5 * time.Second)
			continue
		}
		r := *work.Request
		// Save the claimed request before running. A crash before result delivery
		// leaves the console uncertain; no unjournaled mutation is auto-replayed.
		res := gre.Dispatch(m, r)
		if e = config.Save(pendingPath, map[string]any{"id": r.ID, "response": res}); e != nil {
			return e
		}
	}
}
