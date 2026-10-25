package console

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TIR3D4/GreFlow/internal/protocol"
	"golang.org/x/crypto/ssh"
)

type ProvisioningTransport interface {
	Prepare(context.Context, Host, []byte, protocol.Request, func([]protocol.Diagnostic) error) (protocol.Response, error)
}

const platformProbeCommand = "uname -s; uname -m"

func runSSH(client *ssh.Client, command string, input io.Reader) (string, error) {
	s, e := client.NewSession()
	if e != nil {
		return "", e
	}
	defer s.Close()
	var b bytes.Buffer
	s.Stdin = input
	s.Stdout = &limitedBuffer{Writer: &b, Remaining: 8192}
	s.Stderr = io.Discard
	e = s.Run(command)
	return strings.TrimSpace(b.String()), e
}

func (t SSHTransport) helperAsset(arch string) ([]byte, string, error) {
	if arch != "amd64" && arch != "arm64" {
		return nil, "", fmt.Errorf("unsupported architecture")
	}
	dir := t.HelperDir
	if dir == "" {
		exe, e := os.Executable()
		if e != nil {
			return nil, "", e
		}
		dir = filepath.Join(filepath.Dir(exe), "helpers")
	}
	b, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return nil, "", e
	}
	var manifest map[string]string
	if e = json.Unmarshal(b, &manifest); e != nil {
		return nil, "", e
	}
	name := "greflow-agent-linux-" + arch
	b, e = os.ReadFile(filepath.Join(dir, name))
	if e != nil {
		return nil, "", e
	}
	h := sha256.Sum256(b)
	digest := hex.EncodeToString(h[:])
	if len(b) < 4 || string(b[:4]) != "\x7fELF" || manifest[name] != digest {
		return nil, "", fmt.Errorf("helper checksum or executable format mismatch")
	}
	return b, digest, nil
}

// Only a locally computed hex digest enters this script. Binary bytes travel on
// stdin over pinned SSH. Publish with link(2): an existing helper is never replaced.
func installHelperCommand(username, digest string) string {
	script := `set -eu
umask 077
if [ "$(id -u)" != 0 ]; then exit 40; fi
for tool in mktemp sha256sum cat chmod ln rm rmdir; do command -v "$tool" >/dev/null || exit 41; done
if [ -e /usr/local/bin/greflow-agent ] || [ -L /usr/local/bin/greflow-agent ]; then exit 42; fi
tmp=$(mktemp -d /usr/local/bin/.greflow-install.XXXXXXXX) || exit 43
trap 'rm -f "$tmp/agent"; rmdir "$tmp"' 0
cat > "$tmp/agent" || exit 44
actual=$(sha256sum "$tmp/agent") || exit 45
[ "${actual%% *}" = "` + digest + `" ] || exit 45
chmod 0755 "$tmp/agent" || exit 46
[ "$("$tmp/agent" version)" = "GreFlow dispatcher protocol 1" ] || exit 47
ln "$tmp/agent" /usr/local/bin/greflow-agent || exit 48
printf GREFLOW_INSTALLED
`
	command := "/bin/sh -c '" + strings.ReplaceAll(script, "'", "'\"'\"'") + "'"
	if username != "root" {
		command = "sudo -n " + command
	}
	return command
}

func exitCode(e error) *int {
	var status *ssh.ExitError
	if errors.As(e, &status) {
		n := status.ExitStatus()
		return &n
	}
	return nil
}

func installFailure(e error) (string, string) {
	code := exitCode(e)
	if code == nil {
		return "INSTALL_INTERRUPTED", "ارتباط هنگام نصب قطع شد؛ وضعیت فایل نصب مشخص نیست. آماده‌سازی را دوباره اجرا کنید تا ابتدا نسخهٔ موجود بررسی شود"
	}
	switch *code {
	case 40:
		return "ROOT_REQUIRED", "نصب ابزار به دسترسی root نیاز دارد"
	case 41:
		return "DEPENDENCY_MISSING", "یکی از ابزارهای استاندارد نصب موجود نیست: mktemp، sha256sum، cat، chmod، ln، rm یا rmdir"
	case 42:
		return "HELPER_EXISTS", "فایل ابزار هم‌زمان ایجاد شده یا یک پیوند موجود است؛ بازنویسی انجام نشد. آماده‌سازی را دوباره اجرا کنید"
	case 43:
		return "INSTALL_DIRECTORY", "ساخت فایل موقت در /usr/local/bin ممکن نشد؛ وجود پوشه، مجوز نوشتن، فضای دیسک و فقط‌خواندنی نبودن فایل‌سیستم را بررسی کنید"
	case 44:
		return "UPLOAD_WRITE", "نوشتن فایل کامل نشد؛ فضای دیسک و ارتباط SSH را بررسی کنید"
	case 45:
		return "CHECKSUM_MISMATCH", "صحت فایل منتقل‌شده تأیید نشد؛ فایل نصب نشد"
	case 46:
		return "EXEC_PERMISSION", "دادن مجوز اجرا به فایل موقت ممکن نشد"
	case 47:
		return "EXEC_VERSION", "اجرای نسخهٔ فایل موقت موفق نبود؛ معماری یا محدودیت اجرای فایل‌سیستم را بررسی کنید. فایل نصب نشد"
	case 48:
		return "ATOMIC_INSTALL", "ثبت اتمیک ابزار ممکن نشد؛ فایل موجود بازنویسی نشد"
	default:
		return "INSTALL_COMMAND", "فرمان نصب اجرا نشد؛ برای کاربر غیر root مجوز sudo بدون رمز لازم است. کد خروج را بررسی کنید"
	}
}

func (t SSHTransport) Prepare(ctx context.Context, h Host, key []byte, request protocol.Request, persist func([]protocol.Diagnostic) error) (res protocol.Response, err error) {
	res = protocol.Response{Version: 1, State: "failed"}
	steps := []protocol.Diagnostic{}
	defer func() { res.Diagnostics = steps }()
	record := func(stage, state, code, message string, cause error) error {
		steps = append(steps, protocol.Diagnostic{At: time.Now().UTC(), Stage: stage, State: state, Code: code, Message: message, ExitCode: exitCode(cause)})
		if persist != nil {
			return persist(steps)
		}
		return nil
	}
	fail := func(stage, code, message string, cause error) (protocol.Response, error) {
		res.Version = 1
		res.Error = message
		if e := record(stage, "failed", code, message, cause); e != nil {
			return res, e
		}
		return res, nil
	}
	if e := record("ssh", "running", "", "برقراری اتصال و بررسی هویت SSH", nil); e != nil {
		return res, e
	}
	client, e := connectSSH(ctx, h, key)
	if e != nil {
		var failure *SSHFailure
		if errors.As(e, &failure) {
			return fail(failure.Stage, failure.Code, failure.Message, e)
		}
		return fail("ssh", "SSH_CONNECT", e.Error(), e)
	}
	defer client.Close()
	if e = record("ssh", "succeeded", "", "ورود SSH و هویت ذخیره‌شدهٔ سرور تأیید شد", nil); e != nil {
		return res, e
	}
	v, e := runSSH(client, helperProbeCommand, nil)
	if e != nil {
		return fail("helper", "HELPER_CHECK", "بررسی ابزار موجود ناموفق بود؛ فایل موجود تغییر نکرد", e)
	}
	if v != "GreFlow dispatcher protocol 1" {
		if v != "GREFLOW_HELPER_MISSING" {
			return fail("helper", "HELPER_INCOMPATIBLE", "ابزار موجود قابل اجرا یا سازگار نیست؛ برای حفظ نصب قبلی بازنویسی نشد", nil)
		}
		platform, e := runSSH(client, platformProbeCommand, nil)
		if e != nil {
			return fail("platform", "PLATFORM_CHECK", "تشخیص سیستم‌عامل و معماری ناموفق بود", e)
		}
		parts := strings.Fields(platform)
		if len(parts) != 2 || parts[0] != "Linux" {
			return fail("platform", "UNSUPPORTED_OS", "ابزار سرور فقط روی لینوکس پشتیبانی می‌شود", nil)
		}
		arch := map[string]string{"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}[parts[1]]
		if arch == "" {
			return fail("platform", "UNSUPPORTED_ARCH", "معماری سرور پشتیبانی نمی‌شود؛ فقط Linux amd64 و arm64", nil)
		}
		if e = record("platform", "succeeded", "", "معماری تأیید شد: Linux / "+arch, nil); e != nil {
			return res, e
		}
		binary, digest, e := t.helperAsset(arch)
		if e != nil {
			return fail("package", "PACKAGE_INVALID", "فایل نصب معتبر در پوشهٔ helpers کنسول موجود نیست؛ بستهٔ کامل GreFlow را استخراج کنید", nil)
		}
		if e = record("package", "succeeded", "", "فایل بسته و SHA256 تأیید شد: "+digest, nil); e != nil {
			return res, e
		}
		if e = record("install", "running", "", "انتقال ابزار از کنسول، بررسی صحت و نصب در /usr/local/bin/greflow-agent", nil); e != nil {
			return res, e
		}
		out, e := runSSH(client, installHelperCommand(h.Username, digest), bytes.NewReader(binary))
		if e != nil {
			code, message := installFailure(e)
			if exitCode(e) == nil {
				res.State = "uncertain"
			}
			return fail("install", code, message, e)
		}
		if out != "GREFLOW_INSTALLED" {
			res.State = "uncertain"
			return fail("install", "INSTALL_RESPONSE", "پاسخ نصب معتبر نبود؛ برای بررسی نسخهٔ موجود دوباره آماده‌سازی کنید", nil)
		}
		res.State = "uncertain" // Installation happened; a lost durable acknowledgement is not a definite failure.
		if e = record("install", "succeeded", "", "ابزار نصب شد؛ CLI، تنظیمات و سرویس‌های تانل تغییری نکردند", nil); e != nil {
			return res, e
		}
		res.State = "failed"
	}
	v, e = runSSH(client, "/usr/local/bin/greflow-agent version", nil)
	if e != nil || v != "GreFlow dispatcher protocol 1" {
		return fail("verify", "VERSION_CHECK", "تأیید نسخهٔ ابزار پس از آماده‌سازی ناموفق بود", e)
	}
	if e = record("verify", "succeeded", "", "ابزار مدیریت آماده است؛ پروتکل ۱ تأیید شد", nil); e != nil {
		return res, e
	}
	if e = record("discover", "running", "", "شناسایی فقط‌خواندنی تانل‌های موجود", nil); e != nil {
		return res, e
	}
	request.Action = "discover"
	res, e = t.callConnected(client, h, request)
	if e != nil {
		res.State = "failed"
		return fail("discover", "DISCOVERY_FAILED", e.Error(), e)
	}
	if res.State != "succeeded" {
		return fail("discover", "DISCOVERY_FAILED", res.Error, nil)
	}
	if e = record("discover", "succeeded", "", "شناسایی سرور کامل شد", nil); e != nil {
		return res, e
	}
	return res, nil
}
