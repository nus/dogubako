package rtsp

import (
	"fmt"
	"net/url"
	"strings"
)

func parseRTSPURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("rtsp: empty url")
	}
	if !strings.Contains(raw, "://") {
		raw = "rtsp://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("rtsp: %w", err)
	}
	if u.Scheme != "rtsp" {
		return nil, fmt.Errorf("rtsp: unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("rtsp: missing host")
	}
	if u.Port() == "" {
		u.Host = u.Hostname() + ":554"
	}
	return u, nil
}

func requestURI(u *url.URL) string {
	if u == nil {
		return "*"
	}
	out := *u
	out.User = nil
	if out.Path == "" {
		out.Path = "/"
	}
	return out.String()
}

func hostPort(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Host
}

func credentials(u *url.URL) (user, pass string) {
	if u == nil || u.User == nil {
		return "", ""
	}
	user = u.User.Username()
	pass, _ = u.User.Password()
	return user, pass
}
