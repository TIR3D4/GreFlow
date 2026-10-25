package console

import (
	"golang.org/x/sys/windows/registry"
	"net/http"
	"net/url"
	"strings"
)

// Respect the current Windows user's configured proxy without changing it.
func systemHTTPProxy(r *http.Request) (*url.URL, error) {
	key, e := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if e != nil {
		return nil, nil
	}
	defer key.Close()
	enabled, _, _ := key.GetIntegerValue("ProxyEnable")
	if enabled == 0 {
		return nil, nil
	}
	value, _, _ := key.GetStringValue("ProxyServer")
	if strings.Contains(value, "=") {
		for _, part := range strings.Split(value, ";") {
			if strings.HasPrefix(part, r.URL.Scheme+"=") {
				value = strings.TrimPrefix(part, r.URL.Scheme+"=")
				break
			}
		}
	}
	if value == "" || strings.Contains(value, "=") {
		return nil, nil
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	return url.Parse(value)
}
