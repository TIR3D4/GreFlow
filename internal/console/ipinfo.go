package console

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type IPInfo struct {
	IP              string    `json:"ip"`
	Status          string    `json:"status"`
	Country         string    `json:"country,omitempty"`
	CountryCode     string    `json:"country_code,omitempty"`
	Region          string    `json:"region,omitempty"`
	City            string    `json:"city,omitempty"`
	Organization    string    `json:"organization,omitempty"`
	ASN             string    `json:"asn,omitempty"`
	Source          string    `json:"source"`
	URL             string    `json:"url"`
	ObservedAt      time.Time `json:"observed_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	Error           string    `json:"error,omitempty"`
	CountryConflict bool      `json:"country_conflict,omitempty"`
}

type IPLookup struct {
	Store   *Store
	Client  *http.Client
	mu      sync.Mutex
	pending map[string]chan struct{}
	slots   chan struct{}
}

func newIPLookup(s *Store) *IPLookup {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(r *http.Request) (*url.URL, error) {
		p, e := http.ProxyFromEnvironment(r)
		if p != nil || e != nil {
			return p, e
		}
		return systemHTTPProxy(r)
	}
	return &IPLookup{Store: s, Client: &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("redirect refused") }}, pending: map[string]chan struct{}{}, slots: make(chan struct{}, 2)}
}

func publicIP(value string) bool {
	ip, e := netip.ParseAddr(value)
	if e != nil || !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"} {
		if netip.MustParsePrefix(cidr).Contains(ip) {
			return false
		}
	}
	return true
}

func (a *App) ipInfo(w http.ResponseWriter, r *http.Request, u User) {
	if r.Method != "GET" {
		apiError(w, 405, "METHOD", "GET لازم است")
		return
	}
	host := r.URL.Query().Get("host_id")
	if !u.Can(host) {
		apiError(w, 403, "SCOPE", "سرور در محدودهٔ دسترسی شما نیست")
		return
	}
	h, _, e := a.host(host)
	if e != nil || h.Inventory == nil {
		apiError(w, 404, "HOST", "ابتدا سرور را شناسایی کنید")
		return
	}
	ip := ""
	for _, t := range h.Inventory.Tunnels {
		if t.Name == r.URL.Query().Get("tunnel") {
			ip = t.Desired.Remote
			if t.Desired.Role == "exit" {
				ip = t.Desired.Local
			}
			break
		}
	}
	if ip == "" {
		apiError(w, 404, "TUNNEL", "تانل پیدا نشد")
		return
	}
	if !publicIP(ip) {
		reply(w, 200, IPInfo{IP: ip, Status: "unavailable", Source: "Check-Host", ExpiresAt: time.Now().UTC().Add(24 * time.Hour), Error: "این IP عمومی نیست؛ به سرویس بیرونی ارسال نشد"})
		return
	}
	info, e := a.Geo.lookup(r.Context(), ip)
	if e != nil {
		apiError(w, 502, "IP_INFO", "دریافت اطلاعات Check-Host کامل نشد؛ دوباره تلاش کنید")
		return
	}
	reply(w, 200, info)
}

func (g *IPLookup) lookup(ctx context.Context, ip string) (IPInfo, error) {
	var old IPInfo
	var body []byte
	if g.Store.DB.QueryRow("SELECT body FROM settings WHERE id=?", "ipinfo:"+ip).Scan(&body) == nil && json.Unmarshal(body, &old) == nil && time.Now().Before(old.ExpiresAt) {
		return old, nil
	}
	g.mu.Lock()
	if pending := g.pending[ip]; pending != nil {
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return old, ctx.Err()
		case <-pending:
			return g.lookup(ctx, ip)
		}
	}
	done := make(chan struct{})
	g.pending[ip] = done
	g.mu.Unlock()
	defer func() { g.mu.Lock(); delete(g.pending, ip); close(done); g.mu.Unlock() }()
	select {
	case g.slots <- struct{}{}:
		defer func() { <-g.slots }()
	case <-ctx.Done():
		return old, ctx.Err()
	}
	endpoint := "https://check-host.net/ip-info?host=" + url.QueryEscape(ip) + "&lang=en"
	info := IPInfo{IP: ip, Status: "unavailable", Source: "Check-Host", URL: endpoint, ObservedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(5 * time.Minute)}
	req, e := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if e != nil {
		return info, e
	}
	req.Header.Set("User-Agent", "GreFlow-Console/1.0 (+https://github.com/TIR3D4/GreFlow)")
	req.Header.Set("Accept-Language", "en")
	response, e := g.Client.Do(req)
	if e != nil {
		info.Error = "ارتباط با Check-Host برقرار نشد؛ اتصال اینترنت یا پراکسی کنسول را بررسی کنید"
	} else {
		defer response.Body.Close()
		if response.StatusCode != 200 {
			info.Error = fmt.Sprintf("Check-Host پاسخ HTTP %d داد؛ اطلاعات موقعیت تأیید نشد", response.StatusCode)
		} else {
			b, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
			if err != nil || len(b) > 1<<20 {
				info.Error = "پاسخ Check-Host کامل یا معتبر نیست"
			} else if err = parseIPInfo(b, ip, &info); err != nil {
				info.Error = "اطلاعات IP در پاسخ Check-Host پیدا نشد یا قالب پاسخ تغییر کرده است"
			} else {
				info.Status = "ok"
				info.ExpiresAt = info.ObservedAt.Add(24 * time.Hour)
			}
		}
	}
	if info.Status != "ok" && old.CountryCode != "" {
		message := info.Error
		retry := info.ExpiresAt
		info = old
		info.Status = "stale"
		info.Error = message
		info.ExpiresAt = retry
	}
	b, _ := json.Marshal(info)
	if _, e = g.Store.DB.Exec("INSERT INTO settings(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body", "ipinfo:"+ip, b); e != nil {
		return info, e
	}
	return info, nil
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	s := strings.Join(strings.Fields(b.String()), " ")
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}
func parseIPInfo(b []byte, ip string, out *IPInfo) error {
	doc, e := html.Parse(strings.NewReader(string(b)))
	if e != nil {
		return e
	}
	found := false
	countries := map[string]bool{}
	for table := range doc.Descendants() {
		if table.Type != html.ElementNode || table.Data != "table" {
			continue
		}
		fields := map[string]string{}
		for row := range table.Descendants() {
			if row.Type != html.ElementNode || row.Data != "tr" {
				continue
			}
			cells := []*html.Node{}
			for c := row.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "td" {
					cells = append(cells, c)
				}
			}
			if len(cells) == 2 {
				fields[nodeText(cells[0])] = nodeText(cells[1])
			}
		}
		if fields["IP address"] != ip {
			continue
		}
		country := regexp.MustCompile(`^(.+?)\s*\(([A-Z]{2})\)$`).FindStringSubmatch(fields["Country"])
		if len(country) != 3 {
			continue
		}
		countries[country[2]] = true
		if found {
			continue
		}
		found = true
		out.Country = strings.TrimSpace(country[1])
		out.CountryCode = strings.ToLower(country[2])
		out.Region = fields["Region"]
		out.City = fields["City"]
		out.Organization = fields["ISP / Org"]
		out.ASN = fields["ASN"]
		if out.ASN != "" && !strings.HasPrefix(out.ASN, "AS") {
			out.ASN = "AS" + out.ASN
		}
		for parent := table.Parent; parent != nil; parent = parent.Parent {
			for _, attr := range parent.Attr {
				if attr.Key == "id" && strings.HasPrefix(attr.Val, "ip_info-") {
					out.Source = "Check-Host / " + strings.TrimPrefix(attr.Val, "ip_info-")
				}
			}
		}
	}
	if !found {
		return fmt.Errorf("matching IP info absent")
	}
	out.CountryConflict = len(countries) > 1
	return nil
}
