package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.zip")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	for n, v := range entries {
		w, e := z.Create(n)
		if e != nil {
			t.Fatal(e)
		}
		w.Write([]byte(v))
	}
	z.Close()
	f.Close()
	return p
}
func TestZipSafe(t *testing.T) {
	p := makeZip(t, map[string]string{"book/a.txt": "ok"})
	dst := filepath.Join(t.TempDir(), "out")
	o, e := Zip(p, dst, Limits{Bytes: 100, Files: 10})
	if e != nil {
		t.Fatal(e)
	}
	if len(o) != 1 {
		t.Fatal(o)
	}
}
func TestZipSlipRejected(t *testing.T) {
	p := makeZip(t, map[string]string{"../evil.txt": "bad"})
	dst := filepath.Join(t.TempDir(), "out")
	if _, e := Zip(p, dst, Limits{Bytes: 100, Files: 10}); e == nil {
		t.Fatal("zip slip accepted")
	}
}
func TestZipSizeLimit(t *testing.T) {
	p := makeZip(t, map[string]string{"large.txt": "0123456789"})
	dst := filepath.Join(t.TempDir(), "out")
	if _, e := Zip(p, dst, Limits{Bytes: 5, Files: 10}); e == nil {
		t.Fatal("limit ignored")
	}
}
