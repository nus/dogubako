package rtsp

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type authMode int

const (
	authNone authMode = iota
	authBasic
	authDigest
)

type authState struct {
	mode      authMode
	user      string
	pass      string
	realm     string
	nonce     string
	opaque    string
	qop       string
	algorithm string
	nc        int
}

func (a *authState) header(method, uri string) string {
	if a == nil || a.user == "" {
		return ""
	}
	switch a.mode {
	case authBasic:
		tok := base64.StdEncoding.EncodeToString([]byte(a.user + ":" + a.pass))
		return "Basic " + tok
	case authDigest:
		a.nc++
		nc := fmt.Sprintf("%08x", a.nc)
		cnonce := fmt.Sprintf("%08x", a.nc*0x9e3779b1)
		ha1 := md5hex(a.user + ":" + a.realm + ":" + a.pass)
		ha2 := md5hex(method + ":" + uri)
		var resp string
		if a.qop == "auth" || strings.Contains(a.qop, "auth") {
			a.qop = "auth"
			resp = md5hex(ha1 + ":" + a.nonce + ":" + nc + ":" + cnonce + ":auth:" + ha2)
			h := fmt.Sprintf(`Digest username=%s, realm=%s, nonce=%s, uri=%s, response=%s, algorithm=MD5, qop=auth, nc=%s, cnonce=%s`,
				quote(a.user), quote(a.realm), quote(a.nonce), quote(uri), quote(resp), nc, quote(cnonce))
			if a.opaque != "" {
				h += ", opaque=" + quote(a.opaque)
			}
			return h
		}
		resp = md5hex(ha1 + ":" + a.nonce + ":" + ha2)
		h := fmt.Sprintf(`Digest username=%s, realm=%s, nonce=%s, uri=%s, response=%s`,
			quote(a.user), quote(a.realm), quote(a.nonce), quote(uri), quote(resp))
		if a.algorithm != "" {
			h += ", algorithm=" + a.algorithm
		}
		if a.opaque != "" {
			h += ", opaque=" + quote(a.opaque)
		}
		return h
	default:
		return ""
	}
}

func parseWWWAuth(h, user, pass string) *authState {
	h = strings.TrimSpace(h)
	a := &authState{user: user, pass: pass}
	if strings.HasPrefix(strings.ToLower(h), "basic") {
		a.mode = authBasic
		return a
	}
	if !strings.HasPrefix(strings.ToLower(h), "digest") {
		return nil
	}
	a.mode = authDigest
	params := parseAuthParams(strings.TrimSpace(h[len("digest"):]))
	a.realm = params["realm"]
	a.nonce = params["nonce"]
	a.opaque = params["opaque"]
	a.qop = params["qop"]
	a.algorithm = params["algorithm"]
	return a
}

func parseAuthParams(s string) map[string]string {
	out := make(map[string]string)
	for len(s) > 0 {
		s = strings.TrimLeft(s, " ,")
		if s == "" {
			break
		}
		eq := strings.IndexByte(s, '=')
		if eq < 0 {
			break
		}
		key := strings.ToLower(strings.TrimSpace(s[:eq]))
		s = s[eq+1:]
		var val string
		if strings.HasPrefix(s, `"`) {
			s = s[1:]
			end := strings.IndexByte(s, '"')
			if end < 0 {
				val = s
				s = ""
			} else {
				val = s[:end]
				s = s[end+1:]
			}
		} else {
			end := strings.IndexByte(s, ',')
			if end < 0 {
				val = strings.TrimSpace(s)
				s = ""
			} else {
				val = strings.TrimSpace(s[:end])
				s = s[end:]
			}
		}
		out[key] = val
	}
	return out
}

func quote(s string) string {
	return strconv.Quote(s)
}

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
