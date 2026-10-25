package console

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TIR3D4/GreFlow/internal/protocol"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Host struct {
	PreparationID string              `json:"preparation_id,omitempty"`
	AuthMethod    string              `json:"auth_method,omitempty"`
	Password      string              `json:"password,omitempty"`
	KeyPassphrase string              `json:"key_passphrase,omitempty"`
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Kind          string              `json:"kind"`
	Addresses     []string            `json:"addresses"`
	Hostname      string              `json:"hostname"`
	Port          int                 `json:"port"`
	Username      string              `json:"username"`
	Fingerprint   string              `json:"fingerprint"`
	PrivateKey    string              `json:"private_key,omitempty"`
	Inventory     *protocol.Inventory `json:"inventory,omitempty"`
	LastError     string              `json:"last_error,omitempty"`
	ObservedAt    string              `json:"observed_at,omitempty"`
}

func (h Host) Validate() error {
	if h.Name == "" || len(h.Name) > 100 || (h.Kind != "iran" && h.Kind != "outside") || h.Port < 1 || h.Port > 65535 {
		return fmt.Errorf("invalid host metadata")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9._:\-]+$`).MatchString(h.Hostname) || len(h.Hostname) > 253 {
		return fmt.Errorf("invalid hostname")
	}
	if !regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`).MatchString(h.Username) {
		return fmt.Errorf("invalid SSH username")
	}
	if !regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}$`).MatchString(h.Fingerprint) {
		return fmt.Errorf("pinned SHA256 host fingerprint required")
	}
	return nil
}

type Transport interface {
	Call(context.Context, Host, []byte, protocol.Request) (protocol.Response, error)
}
type SSHTransport struct{ HelperDir string }

type SSHFailure struct{ Stage, Code, Message string }

func (e *SSHFailure) Error() string                { return e.Message }
func sshFailure(stage, code, message string) error { return &SSHFailure{stage, code, message} }

func connectSSH(ctx context.Context, h Host, key []byte) (*ssh.Client, error) {
	if e := h.Validate(); e != nil {
		return nil, sshFailure("validation", "SSH_METADATA", "اطلاعات آدرس، پورت، کاربر یا هویت ذخیره‌شدهٔ SSH معتبر نیست")
	}
	auth, e := credentialAuth(key)
	if e != nil {
		return nil, sshFailure("authentication", "SSH_CREDENTIAL", "کلید یا اطلاعات ورود ذخیره‌شده قابل خواندن نیست؛ دسترسی SSH را دوباره ثبت کنید")
	}
	keyChanged := false
	cfg := &ssh.ClientConfig{User: h.Username, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
		if ssh.FingerprintSHA256(k) != h.Fingerprint {
			keyChanged = true
			return fmt.Errorf("HOST_KEY_CHANGED: fingerprint mismatch")
		}
		return nil
	}, Timeout: 15 * time.Second}
	address := net.JoinHostPort(h.Hostname, strconv.Itoa(h.Port))
	conn, e := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", address)
	if e != nil {
		var dns *net.DNSError
		if errors.As(e, &dns) {
			return nil, sshFailure("network", "SSH_DNS", "نام دامنه به IP تبدیل نشد؛ دامنه و DNS کنسول را بررسی کنید")
		}
		if ne, ok := e.(net.Error); ok && ne.Timeout() {
			return nil, sshFailure("network", "SSH_TIMEOUT", "مهلت اتصال TCP به پورت SSH تمام شد؛ مسیر شبکه، فایروال و پورت را بررسی کنید")
		}
		return nil, sshFailure("network", "SSH_TCP", "اتصال TCP به پورت SSH برقرار نشد؛ آدرس، پورت، سرویس SSH و فایروال را بررسی کنید")
	}
	deadline := time.Now().Add(150 * time.Second)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	conn.SetDeadline(deadline)
	cc, ch, req, e := ssh.NewClientConn(conn, address, cfg)
	if e != nil {
		conn.Close()
		if keyChanged {
			return nil, sshFailure("identity", "HOST_KEY_CHANGED", "هویت SSH سرور با مقدار ذخیره‌شده تفاوت دارد؛ اتصال متوقف شد. تغییر هویت را از مسیر معتبر بررسی کنید")
		}
		if strings.Contains(e.Error(), "unable to authenticate") {
			return nil, sshFailure("authentication", "SSH_AUTH", "سرور ورود SSH را رد کرد؛ نام کاربر، پسورد یا کلید و مجوز ورود این کاربر را بررسی کنید")
		}
		return nil, sshFailure("handshake", "SSH_HANDSHAKE", "پورت پاسخ داد اما مذاکرهٔ SSH کامل نشد؛ سرویس SSH، الگوریتم‌های مجاز و مهلت اتصال را بررسی کنید")
	}
	return ssh.NewClient(cc, ch, req), nil
}
func dispatcherCommand(username string) string {
	if username == "root" {
		return "/usr/local/bin/greflow-agent dispatch"
	}
	return "sudo -n /usr/local/bin/greflow-agent dispatch"
}
func (t SSHTransport) Call(ctx context.Context, h Host, key []byte, r protocol.Request) (out protocol.Response, err error) {
	steps := []protocol.Diagnostic{{At: time.Now().UTC(), Stage: "ssh", State: "running", Message: "برقراری اتصال SSH"}}
	defer func() {
		if err != nil {
			stage, code := "dispatch", "SSH_DISPATCH"
			var failure *SSHFailure
			if errors.As(err, &failure) {
				stage, code = failure.Stage, failure.Code
			}
			steps = append(steps, protocol.Diagnostic{At: time.Now().UTC(), Stage: stage, State: "failed", Code: code, Message: err.Error(), ExitCode: exitCode(err)})
		}
		out.Diagnostics = steps
	}()
	client, e := connectSSH(ctx, h, key)
	if e != nil {
		out.State = "failed"
		return out, e
	}
	defer client.Close()
	steps = append(steps, protocol.Diagnostic{At: time.Now().UTC(), Stage: "ssh", State: "succeeded", Message: "ورود و هویت SSH تأیید شد"})
	out, err = t.callConnected(client, h, r)
	if err == nil {
		steps = append(steps, protocol.Diagnostic{At: time.Now().UTC(), Stage: "dispatch", State: out.State, Message: "پاسخ ابزار مدیریت دریافت شد"})
	}
	return out, err
}

func (SSHTransport) callConnected(client *ssh.Client, h Host, r protocol.Request) (protocol.Response, error) {
	var out protocol.Response
	session, e := client.NewSession()
	if e != nil {
		return out, e
	}
	defer session.Close()
	body, _ := json.Marshal(r)
	session.Stdin = bytes.NewReader(body)
	stdout, e := session.StdoutPipe()
	if e != nil {
		return out, e
	}
	session.Stderr = io.Discard
	// This is a constant command; no field from a browser enters the SSH shell.
	if e = session.Start(dispatcherCommand(h.Username)); e != nil {
		return out, fmt.Errorf("dispatcher unavailable")
	}
	b, e := io.ReadAll(io.LimitReader(stdout, (4<<20)+1))
	if e != nil {
		return out, e
	}
	if len(b) > 4<<20 {
		return out, sshFailure("dispatch", "RESPONSE_TOO_LARGE", "پاسخ ابزار بیش از حد مجاز بود؛ نتیجهٔ عملیات نیاز به بررسی دارد")
	}
	if e = session.Wait(); e != nil {
		return out, diagnoseDispatcherFailure(client, h.Username, e)
	}
	if e = json.Unmarshal(b, &out); e != nil {
		return out, fmt.Errorf("invalid dispatcher response")
	}
	if out.Version != 1 {
		return out, fmt.Errorf("unsupported dispatcher protocol")
	}
	return out, nil
}

// A failed command does not prove the helper is absent. Inspect only fixed paths
// and the read-only version command; never install, retry dispatch, or echo remote
// stderr (which can contain credentials or other unrelated shell output).
const helperProbeCommand = "if [ ! -e /usr/local/bin/greflow-agent ]; then printf 'GREFLOW_HELPER_MISSING'; elif [ ! -x /usr/local/bin/greflow-agent ]; then printf 'GREFLOW_HELPER_NOT_EXECUTABLE'; else /usr/local/bin/greflow-agent version; fi"

type dispatcherFailure struct{ message, cause error }

func (e *dispatcherFailure) Error() string   { return e.message.Error() }
func (e *dispatcherFailure) Unwrap() []error { return []error{e.message, e.cause} }

func diagnoseDispatcherFailure(client *ssh.Client, username string, cause error) (result error) {
	defer func() { result = &dispatcherFailure{message: result, cause: cause} }()
	var exit *ssh.ExitError
	if !errors.As(cause, &exit) {
		return fmt.Errorf("ارتباط هنگام اجرای ابزار مدیریت قطع شد؛ نتیجهٔ عملیات مشخص نیست")
	}
	probe, e := client.NewSession()
	if e == nil {
		defer probe.Close()
		var version bytes.Buffer
		probe.Stdout = &limitedBuffer{Writer: &version, Remaining: 4096}
		probe.Stderr = io.Discard
		if probe.Run(helperProbeCommand) == nil {
			switch strings.TrimSpace(version.String()) {
			case "GREFLOW_HELPER_MISSING":
				return sshFailure("helper", "HELPER_MISSING", "اتصال SSH برقرار است؛ ابزار مدیریت نصب نیست. آماده‌سازی خودکار را اجرا کنید")
			case "GREFLOW_HELPER_NOT_EXECUTABLE":
				return sshFailure("helper", "HELPER_PERMISSION", "اتصال SSH برقرار است؛ فایل /usr/local/bin/greflow-agent اجازهٔ اجرا ندارد")
			case "GreFlow dispatcher protocol 1":
				if username != "root" {
					return fmt.Errorf("اتصال SSH و نسخهٔ ابزار مدیریت تأیید شد؛ اجرای dispatch با کد %d شکست خورد. مجوز sudo بدون رمز برای این فرمان و لاگ سرور را بررسی کنید", exit.ExitStatus())
				}
				return fmt.Errorf("اتصال SSH و نسخهٔ ابزار مدیریت تأیید شد؛ اجرای dispatch با کد %d شکست خورد. لاگ ابزار مدیریت را بررسی کنید", exit.ExitStatus())
			default:
				return fmt.Errorf("اتصال SSH برقرار است؛ نسخهٔ ابزار مدیریت با پروتکل ۱ کنسول سازگار نیست")
			}
		}
	}
	return fmt.Errorf("اتصال SSH برقرار شد؛ ابزار مدیریت با کد %d شکست خورد و بررسی نسخهٔ آن موفق نبود. نصب و سازگاری فایل اجرایی با معماری سرور را بررسی کنید", exit.ExitStatus())
}

// Fingerprint performs a handshake only; it never authenticates or trusts a key.
func Fingerprint(ctx context.Context, hostname string, port int) (string, error) {
	if !regexp.MustCompile(`^[a-zA-Z0-9._:\-]+$`).MatchString(hostname) || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid SSH address")
	}
	addr := net.JoinHostPort(hostname, strconv.Itoa(port))
	conn, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", addr)
	if e != nil {
		return "", e
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	fp := ""
	_, _, _, _ = ssh.NewClientConn(conn, addr, &ssh.ClientConfig{User: "greflow-key-inspection", HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
		fp = ssh.FingerprintSHA256(k)
		return fmt.Errorf("inspection only")
	}})
	if fp == "" {
		return "", fmt.Errorf("SSH host key unavailable")
	}
	return fp, nil
}
