package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHTTPDownloadAndLimit(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("hello")) }))
	defer s.Close()
	d := t.TempDir()
	p := filepath.Join(d, "x")
	if _, e := HTTP(s.URL, p, 10, time.Second); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "hello" {
		t.Fatal(string(b))
	}
	if _, e := HTTP(s.URL, filepath.Join(d, "y"), 3, time.Second); e == nil {
		t.Fatal("limit ignored")
	}
}
func TestHTTPRejectsFileScheme(t *testing.T) {
	if _, e := HTTP("file:///etc/passwd", filepath.Join(t.TempDir(), "x"), 10, time.Second); e == nil {
		t.Fatal("file scheme accepted")
	}
}
func TestRedirectToFileRejected(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "file:///etc/passwd", 302) }))
	defer s.Close()
	if _, e := HTTP(s.URL, filepath.Join(t.TempDir(), "x"), 100, time.Second); e == nil {
		t.Fatal("unsafe redirect accepted")
	}
}
