package imapclient

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	conn        net.Conn
	r           *bufio.Reader
	tag         int
	timeout     time.Duration
	UIDValidity uint32
}

func Dial(host string, port int, useTLS bool, caFile, bundle string, connectTimeout, ioTimeout time.Duration) (*Client, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	d := net.Dialer{Timeout: connectTimeout}
	var conn net.Conn
	var err error
	if useTLS {
		roots, _ := x509.SystemCertPool()
		if roots == nil {
			roots = x509.NewCertPool()
		}
		for _, p := range []string{bundle, caFile} {
			if p != "" {
				if b, e := os.ReadFile(p); e == nil {
					roots.AppendCertsFromPEM(b)
				} else if p == caFile {
					return nil, fmt.Errorf("cannot read custom CA file: %w", e)
				}
			}
		}
		conn, err = tls.DialWithDialer(&d, "tcp", addr, &tls.Config{ServerName: host, RootCAs: roots, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot connect to IMAP server: %w", err)
	}
	c := &Client{conn: conn, r: bufio.NewReader(conn), timeout: ioTimeout}
	line, e := c.readLine()
	if e != nil {
		conn.Close()
		return nil, e
	}
	if !strings.HasPrefix(line, "* OK") {
		conn.Close()
		return nil, fmt.Errorf("IMAP server rejected connection: %s", line)
	}
	return c, nil
}
func (c *Client) Close() {
	if c.conn != nil {
		c.command("LOGOUT")
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
func quote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
func (c *Client) deadline() { c.conn.SetDeadline(time.Now().Add(c.timeout)) }
func (c *Client) readLine() (string, error) {
	c.deadline()
	s, e := c.r.ReadString('\n')
	if e != nil {
		return "", fmt.Errorf("IMAP read failed: %w", e)
	}
	return strings.TrimRight(s, "\r\n"), nil
}
func (c *Client) command(cmd string) ([]string, error) {
	c.tag++
	tag := fmt.Sprintf("A%04d", c.tag)
	c.deadline()
	if _, e := fmt.Fprintf(c.conn, "%s %s\r\n", tag, cmd); e != nil {
		return nil, e
	}
	var lines []string
	for {
		l, e := c.readLine()
		if e != nil {
			return lines, e
		}
		lines = append(lines, l)
		if strings.HasPrefix(l, tag+" ") {
			if strings.HasPrefix(l, tag+" OK") {
				return lines, nil
			}
			return lines, errors.New(l)
		}
	}
}
func (c *Client) Login(user, pass string) error {
	_, e := c.command("LOGIN " + quote(user) + " " + quote(pass))
	if e != nil {
		return fmt.Errorf("authentication failed: %w", e)
	}
	return nil
}

var uvRE = regexp.MustCompile(`(?i)UIDVALIDITY\s+(\d+)`)

func (c *Client) Select(box string) error {
	ls, e := c.command("SELECT " + quote(box))
	if e != nil {
		return e
	}
	for _, l := range ls {
		if m := uvRE.FindStringSubmatch(l); m != nil {
			v, _ := strconv.ParseUint(m[1], 10, 32)
			c.UIDValidity = uint32(v)
		}
	}
	if c.UIDValidity == 0 {
		return errors.New("server did not provide UIDVALIDITY")
	}
	return nil
}
func (c *Client) Search(since time.Time, unseen bool) ([]uint32, error) {
	q := "UID SEARCH"
	if unseen {
		q += " UNSEEN"
	}
	if !since.IsZero() {
		q += " SINCE " + since.Format("02-Jan-2006")
	}
	ls, e := c.command(q)
	if e != nil {
		return nil, e
	}
	var out []uint32
	for _, l := range ls {
		if strings.HasPrefix(l, "* SEARCH") {
			for _, x := range strings.Fields(strings.TrimPrefix(l, "* SEARCH")) {
				v, _ := strconv.ParseUint(x, 10, 32)
				if v > 0 {
					out = append(out, uint32(v))
				}
			}
		}
	}
	return out, nil
}

var literalRE = regexp.MustCompile(`\{(\d+)\}$`)

func (c *Client) FetchPeek(uid uint32, maxBytes int64) ([]byte, error) {
	c.tag++
	tag := fmt.Sprintf("A%04d", c.tag)
	c.deadline()
	if _, e := fmt.Fprintf(c.conn, "%s UID FETCH %d (BODY.PEEK[])\r\n", tag, uid); e != nil {
		return nil, e
	}
	var body []byte
	for {
		l, e := c.readLine()
		if e != nil {
			return nil, e
		}
		if m := literalRE.FindStringSubmatch(l); m != nil {
			n64, _ := strconv.ParseInt(m[1], 10, 64)
			if n64 < 0 {
				return nil, errors.New("invalid IMAP literal size")
			}
			c.deadline()
			if maxBytes > 0 && n64 > maxBytes {
				if _, e := io.CopyN(io.Discard, c.r, n64); e != nil {
					return nil, fmt.Errorf("cannot discard oversized message: %w", e)
				}
				body = nil
				for {
					tail, e := c.readLine()
					if e != nil {
						return nil, e
					}
					if strings.HasPrefix(tail, tag+" ") {
						break
					}
				}
				return nil, fmt.Errorf("message exceeds configured size limit (%d bytes)", maxBytes)
			}
			n := int(n64)
			body = make([]byte, n)
			if _, e := io.ReadFull(c.r, body); e != nil {
				return nil, fmt.Errorf("cannot read message body: %w", e)
			}
		}
		if strings.HasPrefix(l, tag+" ") {
			if strings.HasPrefix(l, tag+" OK") {
				if body == nil {
					return nil, errors.New("message body missing in FETCH response")
				}
				return body, nil
			}
			return nil, errors.New(l)
		}
	}
}
func (c *Client) MarkSeen(uid uint32) error {
	_, e := c.command(fmt.Sprintf("UID STORE %d +FLAGS.SILENT (\\Seen)", uid))
	return e
}
