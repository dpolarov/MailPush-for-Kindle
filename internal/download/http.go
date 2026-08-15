package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

func HTTP(raw, dst string, max int64, timeout time.Duration) (string, error) {
	u, e := url.Parse(raw)
	if e != nil {
		return "", e
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
	cl := &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("redirect to unsupported URL scheme")
		}
		return nil
	}}
	req, _ := http.NewRequestWithContext(context.Background(), "GET", raw, nil)
	resp, e := cl.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP download failed: %s", resp.Status)
	}
	if resp.ContentLength > max {
		return "", fmt.Errorf("remote file exceeds configured size limit")
	}
	f, e := os.CreateTemp(path.Dir(dst), ".mailpush-http-*.part")
	if e != nil {
		return "", e
	}
	tmp := f.Name()
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(tmp)
		}
	}()
	n, e := io.Copy(f, io.LimitReader(resp.Body, max+1))
	if e != nil {
		return "", e
	}
	if n > max {
		return "", fmt.Errorf("download exceeds configured size limit")
	}
	if e = f.Chmod(0644); e != nil {
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	if e = os.Rename(tmp, dst); e != nil {
		return "", e
	}
	ok = true
	return dst, nil
}
func Name(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return "download"
	}
	n := path.Base(u.Path)
	if n == "." || n == "/" || n == "" {
		return "download"
	}
	return n
}
