package console

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// checkSSH authenticates and reads the installed helper version. It never installs
// software or changes GRE, firewall, users or keys on the remote host.
func (a *App) checkSSH(w http.ResponseWriter, r *http.Request, u User) {
	if r.Method != "POST" || u.Role != "admin" {
		apiError(w, 403, "ROLE", "دسترسی مدیر لازم است")
		return
	}
	var h Host
	if !decode(w, r, &h) {
		return
	}
	credential, e := credentialFromHost(h)
	if e != nil {
		apiError(w, 400, "CREDENTIAL", e.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	if h.Fingerprint == "" {
		h.Fingerprint, e = Fingerprint(ctx, h.Hostname, h.Port)
		if e != nil {
			apiError(w, 502, "SSH", "سرور پاسخ SSH نداد؛ آدرس و پورت را بررسی کنید")
			return
		}
	}
	client, e := connectSSH(ctx, h, credential)
	if e != nil {
		apiError(w, 502, "SSH", e.Error())
		return
	}
	defer client.Close()
	session, e := client.NewSession()
	if e != nil {
		apiError(w, 502, "SSH", e.Error())
		return
	}
	defer session.Close()
	// The version command is read-only and does not need sudo, even for non-root.
	var version bytes.Buffer
	session.Stdout = &limitedBuffer{Writer: &version, Remaining: 4096}
	session.Stderr = io.Discard
	helper := session.Run("/usr/local/bin/greflow-agent version") == nil && strings.TrimSpace(version.String()) == "GreFlow dispatcher protocol 1"
	reply(w, 200, map[string]any{"connected": true, "fingerprint": h.Fingerprint, "helper_ready": helper, "username": h.Username, "trust": "first_connection"})
}

type limitedBuffer struct {
	Writer    io.Writer
	Remaining int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Remaining > 0 {
		part := p
		if len(part) > b.Remaining {
			part = part[:b.Remaining]
		}
		written, e := b.Writer.Write(part)
		b.Remaining -= written
		if e != nil {
			return written, e
		}
	}
	return n, nil
}
