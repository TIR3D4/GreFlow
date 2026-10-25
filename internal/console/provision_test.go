package console

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TIR3D4/GreFlow/internal/protocol"
	"golang.org/x/crypto/ssh"
)

func TestAutomaticPreparationOverSSH(t *testing.T) {
	for _, tc := range []struct {
		name, initial, platform, want       string
		status                              int
		corrupt, disconnect, storageFailure bool
	}{
		{name: "install", want: "succeeded"},
		{name: "existing", initial: "GreFlow dispatcher protocol 1", want: "succeeded"},
		{name: "incompatible", initial: "GreFlow dispatcher protocol 0", want: "HELPER_INCOMPATIBLE"},
		{name: "arm64", platform: "Linux\naarch64", want: "succeeded"},
		{name: "unsupported", platform: "Linux\nmips", want: "UNSUPPORTED_ARCH"},
		{name: "nonlinux", platform: "Darwin\nx86_64", want: "UNSUPPORTED_OS"},
		{name: "corrupt", corrupt: true, want: "PACKAGE_INVALID"},
		{name: "permission", status: 40, want: "ROOT_REQUIRED"},
		{name: "disk", status: 44, want: "UPLOAD_WRITE"},
		{name: "checksum", status: 45, want: "CHECKSUM_MISMATCH"},
		{name: "race", status: 48, want: "ATOMIC_INSTALL"},
		{name: "disconnect", disconnect: true, want: "INSTALL_INTERRUPTED"},
		{name: "audit-failure", storageFailure: true, want: "storage failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asset := []byte("\x7fELFisolated-test-payload")
			digestBytes := sha256.Sum256(asset)
			digest := hex.EncodeToString(digestBytes[:])
			dir := t.TempDir()
			manifest := map[string]string{}
			for _, arch := range []string{"amd64", "arm64"} {
				name := "greflow-agent-linux-" + arch
				b := asset
				if tc.corrupt {
					b = []byte("corrupt")
				}
				if e := os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
					t.Fatal(e)
				}
				manifest[name] = digest
			}
			b, _ := json.Marshal(manifest)
			os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600)
			_, private, _ := ed25519.GenerateKey(rand.Reader)
			signer, _ := ssh.NewSignerFromKey(private)
			block, _ := ssh.MarshalPrivateKey(private, "test")
			credential := pem.EncodeToMemory(block)
			cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
				if !bytes.Equal(k.Marshal(), signer.PublicKey().Marshal()) {
					return nil, fmt.Errorf("wrong key")
				}
				return nil, nil
			}}
			cfg.AddHostKey(signer)
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer listener.Close()
			var installs, discovers atomic.Int32
			done := make(chan struct{})
			go func() {
				defer close(done)
				c, e := listener.Accept()
				if e != nil {
					return
				}
				defer c.Close()
				conn, channels, requests, e := ssh.NewServerConn(c, cfg)
				if e != nil {
					return
				}
				defer conn.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					channel, requests, e := incoming.Accept()
					if e != nil {
						return
					}
					for req := range requests {
						if req.Type != "exec" {
							req.Reply(false, nil)
							continue
						}
						var command struct{ Command string }
						ssh.Unmarshal(req.Payload, &command)
						req.Reply(true, nil)
						status := 0
						out := ""
						switch command.Command {
						case helperProbeCommand:
							out = tc.initial
							if out == "" {
								out = "GREFLOW_HELPER_MISSING"
							}
						case platformProbeCommand:
							out = tc.platform
							if out == "" {
								out = "Linux\nx86_64"
							}
						case "/usr/local/bin/greflow-agent version":
							out = "GreFlow dispatcher protocol 1"
						case installHelperCommand("root", digest):
							installs.Add(1)
							received, e := io.ReadAll(channel)
							if e != nil || !bytes.Equal(received, asset) {
								t.Errorf("incorrect upload")
							}
							if tc.disconnect {
								conn.Close()
								return
							}
							status = tc.status
							out = "GREFLOW_INSTALLED"
							if status != 0 {
								io.WriteString(channel.Stderr(), "PRIVATE_ERROR_OUTPUT")
							}
						case dispatcherCommand("root"):
							discovers.Add(1)
							var r protocol.Request
							json.NewDecoder(channel).Decode(&r)
							if r.Action != "discover" {
								t.Errorf("unexpected mutation: %s", r.Action)
							}
							b, _ := json.Marshal(protocol.Response{Version: 1, State: "succeeded", Inventory: &protocol.Inventory{Version: 1, HostID: "isolated-machine", ObservedAt: time.Now().UTC()}})
							out = string(b)
						default:
							t.Errorf("unapproved command")
							status = 99
						}
						io.WriteString(channel, out)
						channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(status)}))
						channel.Close()
					}
				}
			}()
			h := Host{Name: "test", Kind: "iran", Hostname: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Username: "root", Fingerprint: ssh.FingerprintSHA256(signer.PublicKey())}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			progress := 0
			result, e := (SSHTransport{HelperDir: dir}).Prepare(ctx, h, credential, protocol.Request{Version: 1, ID: ID(), Action: "prepare"}, func(steps []protocol.Diagnostic) error {
				progress++
				if tc.storageFailure && steps[len(steps)-1].Stage == "install" {
					return fmt.Errorf("storage failure")
				}
				return nil
			})
			<-done
			if progress == 0 || len(result.Diagnostics) == 0 {
				t.Fatal("missing progress")
			}
			if tc.storageFailure {
				if e == nil || installs.Load() != 0 {
					t.Fatal("install despite unavailable audit", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if strings.Contains(marshal(result), "PRIVATE_ERROR_OUTPUT") {
				t.Fatal("stderr leaked")
			}
			if tc.want == "succeeded" {
				if result.State != "succeeded" || discovers.Load() != 1 {
					t.Fatal(result)
				}
				want := int32(1)
				if tc.initial != "" {
					want = 0
				}
				if installs.Load() != want {
					t.Fatal("existing helper replaced")
				}
			} else {
				last := result.Diagnostics[len(result.Diagnostics)-1]
				if last.Code != tc.want || discovers.Load() != 0 {
					t.Fatal(result)
				}
				if tc.status != 0 && (last.ExitCode == nil || *last.ExitCode != tc.status) {
					t.Fatal("missing exit code")
				}
				if tc.disconnect && result.State != "uncertain" {
					t.Fatal("disconnect misreported")
				}
			}
		})
	}
}

func TestPreparationRequiresAdmin(t *testing.T) {
	a := testApp(t)
	cookie, csrf := loginTest(t, a)
	host := addTestHost(t, a)
	w := request(t, a, "users", map[string]any{"name": "operator", "password": "operator-test-password", "role": "operator", "scope": []string{host}}, cookie, csrf, ID())
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = request(t, a, "auth/login", map[string]string{"name": "operator", "password": "operator-test-password"}, "", "", "")
	var session map[string]string
	json.Unmarshal(w.Body.Bytes(), &session)
	operatorCookie := w.Result().Cookies()[0].String()
	for _, action := range []string{"prepare", "server_reboot"} {
		w = request(t, a, "operations", map[string]any{"host_id": host, "request": protocol.Request{Action: action}}, operatorCookie, session["csrf"], ID())
		if w.Code != 403 {
			t.Fatal("operator can mutate host", action, w.Code)
		}
	}
}
