package rtsp

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	rtspUserAgent = "dogubako"
	rtspVersion   = "RTSP/1.0"
)

type rtspResponse struct {
	Status int
	Reason string
	Header map[string]string
	Body   []byte
	Raw    string
}

type rtspConn struct {
	conn  net.Conn
	br    *bufio.Reader
	cseq  int
	auth  *authState
	sess  string
	bytes *atomic.Uint64
}

func (c *rtspConn) close() {
	if c != nil && c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *rtspConn) setDeadline(d time.Duration) {
	if c == nil || c.conn == nil {
		return
	}
	if d <= 0 {
		_ = c.conn.SetDeadline(time.Time{})
		return
	}
	_ = c.conn.SetDeadline(time.Now().Add(d))
}

func (c *rtspConn) request(method, uri string, extra map[string]string) (*rtspResponse, string, error) {
	c.cseq++
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s\r\n", method, uri, rtspVersion)
	fmt.Fprintf(&b, "CSeq: %d\r\n", c.cseq)
	fmt.Fprintf(&b, "User-Agent: %s\r\n", rtspUserAgent)
	if c.sess != "" {
		fmt.Fprintf(&b, "Session: %s\r\n", c.sess)
	}
	if h := c.auth.header(method, uri); h != "" {
		fmt.Fprintf(&b, "Authorization: %s\r\n", h)
	}
	for k, v := range extra {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	b.WriteString("\r\n")
	req := b.String()
	c.setDeadline(15 * time.Second)
	n, err := io.WriteString(c.conn, req)
	c.addBytes(n)
	if err != nil {
		return nil, req, err
	}
	resp, err := c.readResponse()
	return resp, req, err
}

func (c *rtspConn) readResponse() (*rtspResponse, error) {
	var raw strings.Builder
	statusLine, err := c.readLine()
	if err != nil {
		return nil, err
	}
	raw.WriteString(statusLine)
	raw.WriteString("\r\n")
	code, reason, err := parseStatus(statusLine)
	if err != nil {
		return nil, err
	}
	hdr := make(map[string]string)
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		raw.WriteString(line)
		raw.WriteString("\r\n")
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key := canonicalHeader(k)
		val := strings.TrimSpace(v)
		if prev, exists := hdr[key]; exists {
			hdr[key] = prev + ", " + val
		} else {
			hdr[key] = val
		}
	}
	var body []byte
	if cl := hdr["Content-Length"]; cl != "" {
		n, err := strconv.Atoi(strings.TrimSpace(cl))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("rtsp: bad Content-Length")
		}
		body = make([]byte, n)
		if n > 0 {
			if _, err := io.ReadFull(c.br, body); err != nil {
				return nil, err
			}
			c.addBytes(n)
			raw.Write(body)
		}
	}
	if sid := sessionID(hdr["Session"]); sid != "" {
		c.sess = sid
	}
	return &rtspResponse{Status: code, Reason: reason, Header: hdr, Body: body, Raw: raw.String()}, nil
}

func (c *rtspConn) readLine() (string, error) {
	line, err := c.br.ReadString('\n')
	c.addBytes(len(line))
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (c *rtspConn) addBytes(n int) {
	if c.bytes != nil && n > 0 {
		c.bytes.Add(uint64(n))
	}
}

func (c *rtspConn) readInterleaved() (channel byte, payload []byte, resp *rtspResponse, err error) {
	for {
		b, err := c.br.ReadByte()
		if err != nil {
			return 0, nil, nil, err
		}
		c.addBytes(1)
		if b == '$' {
			hdr := make([]byte, 3)
			if _, err := io.ReadFull(c.br, hdr); err != nil {
				return 0, nil, nil, err
			}
			c.addBytes(3)
			n := int(binary.BigEndian.Uint16(hdr[1:]))
			payload := make([]byte, n)
			if n > 0 {
				if _, err := io.ReadFull(c.br, payload); err != nil {
					return 0, nil, nil, err
				}
				c.addBytes(n)
			}
			return hdr[0], payload, nil, nil
		}
		if err := c.br.UnreadByte(); err != nil {
			return 0, nil, nil, err
		}
		resp, err := c.readResponse()
		return 0, nil, resp, err
	}
}

func parseStatus(line string) (int, string, error) {
	parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "RTSP/") {
		return 0, "", fmt.Errorf("rtsp: bad status %q", line)
	}
	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, "", fmt.Errorf("rtsp: bad status code %q", line)
	}
	reason := ""
	if len(parts) > 2 {
		reason = parts[2]
	}
	return code, reason, nil
}

func canonicalHeader(k string) string {
	k = strings.TrimSpace(k)
	parts := strings.Split(k, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, "-")
}

func sessionID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	id, _, _ := strings.Cut(v, ";")
	return strings.TrimSpace(id)
}

func sessionTimeout(v string) time.Duration {
	for _, part := range strings.Split(v, ";") {
		k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(k), "timeout") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 60 * time.Second
}

func contentBase(resp *rtspResponse, fallback string) string {
	if resp == nil {
		return fallback
	}
	if v := resp.Header["Content-Base"]; v != "" {
		return strings.TrimSpace(v)
	}
	if v := resp.Header["Content-Location"]; v != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func headerAuth(resp *rtspResponse) string {
	if resp == nil {
		return ""
	}
	if v := resp.Header["Www-Authenticate"]; v != "" {
		return v
	}
	return resp.Header["WWW-Authenticate"]
}
