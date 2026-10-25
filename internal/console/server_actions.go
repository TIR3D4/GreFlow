package console

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/TIR3D4/GreFlow/internal/protocol"
)

type ServerTransport interface {
	ServerAction(context.Context, Host, []byte, protocol.Request) (protocol.Response, error)
}

func consoleAction(action string) bool {
	return action == "prepare" || action == "server_info" || action == "server_logs" || action == "server_reboot"
}
func serverCommand(user, action, id string) (string, error) {
	command := ""
	switch action {
	case "server_info":
		return `/bin/sh -c 'set -e; printf "=== System ===\n"; uname -srmo; printf "\n=== Uptime / load ===\n"; uptime; printf "\n=== Memory ===\n"; free -m; printf "\n=== Root disk ===\n"; df -h /; printf "\n=== Failed services ===\n"; systemctl --failed --no-pager --plain'`, nil
	case "server_logs":
		command = "journalctl --no-pager -o short-iso -n 100 -u ssh.service -u sshd.service"
	case "server_reboot":
		if !protocol.IDPattern.MatchString(id) {
			return "", fmt.Errorf("invalid operation ID")
		}
		command = "systemd-run --quiet --unit=greflow-console-reboot-" + id + " --on-active=5s /usr/bin/systemctl reboot"
	default:
		return "", fmt.Errorf("unsupported server action")
	}
	if user != "root" {
		command = "sudo -n " + command
	}
	return command, nil
}

func safeServerLog(value string, key []byte) string {
	if c, e := readCredential(key); e == nil {
		for _, secret := range []string{c.Password, c.PrivateKey, c.Passphrase} {
			if secret != "" {
				value = strings.ReplaceAll(value, secret, "[REDACTED]")
			}
		}
	}
	value = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`).ReplaceAllString(value, "[REDACTED KEY]")
	value = regexp.MustCompile(`(?i)(password|passwd|token|secret|authorization)(\s*[:=]\s*)\S+`).ReplaceAllString(value, "$1$2[REDACTED]")
	value = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]").ReplaceAllString(value, "")
	return strings.Map(func(r rune) rune {
		if (r < 32 && r != '\n' && r != '\t') || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return -1
		}
		return r
	}, value)
}

func (t SSHTransport) ServerAction(ctx context.Context, h Host, key []byte, r protocol.Request) (res protocol.Response, err error) {
	res = protocol.Response{Version: 1, State: "failed"}
	record := func(stage, state, code, message string, e error) {
		res.Diagnostics = append(res.Diagnostics, protocol.Diagnostic{At: time.Now().UTC(), Stage: stage, State: state, Code: code, Message: message, ExitCode: exitCode(e)})
	}
	client, e := connectSSH(ctx, h, key)
	if e != nil {
		res.Error = e.Error()
		record("ssh", "failed", "SSH_CONNECT", e.Error(), e)
		return res, nil
	}
	defer client.Close()
	record("ssh", "succeeded", "", "ورود و هویت SSH تأیید شد", nil)
	command, e := serverCommand(h.Username, r.Action, r.ID)
	if e != nil {
		return res, e
	}
	out, e := runSSH(client, command, nil)
	res.Logs = safeServerLog(out, key)
	if e != nil {
		res.Error = "اجرای فرمان سرور موفق نبود؛ دسترسی کاربر، وجود systemd و کد خروج را بررسی کنید"
		if r.Action == "server_reboot" && exitCode(e) == nil {
			res.State = "uncertain"
			res.Error = "ارتباط هنگام درخواست ری‌استارت قطع شد؛ نتیجه مشخص نیست. وضعیت سرور را بررسی کنید و درخواست را خودکار تکرار نکنید"
		}
		record(r.Action, "failed", "SERVER_COMMAND", res.Error, e)
		return res, nil
	}
	res.State = "succeeded"
	message := "اطلاعات واقعی سرور دریافت شد"
	if r.Action == "server_reboot" {
		message = "درخواست ری‌استارت سرور برای ۵ ثانیهٔ دیگر ثبت شد؛ بالا آمدن دوبارهٔ سرور هنوز تأیید نشده است"
		res.Logs = message
	}
	record(r.Action, "succeeded", "", message, nil)
	return res, nil
}
