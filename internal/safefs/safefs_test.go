package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRejectsTraversal(t *testing.T) {
	r := t.TempDir()
	if _, e := Resolve(r, r, "../evil", "x"); e == nil {
		t.Fatal("traversal accepted")
	}
}
func TestResolveRejectsAbsoluteOutsideRoot(t *testing.T) {
	r := t.TempDir()
	if _, e := Resolve(r, r, "/etc/passwd", "x"); e == nil {
		t.Fatal("outside absolute path accepted")
	}
}
func TestResolveRejectsSymlinkComponent(t *testing.T) {
	r := t.TempDir()
	out := t.TempDir()
	if e := os.Symlink(out, filepath.Join(r, "link")); e != nil {
		t.Skip(e)
	}
	if _, e := Resolve(r, r, "link/x.epub", "x"); e == nil {
		t.Fatal("symlink traversal accepted")
	}
}
func TestUnique(t *testing.T) {
	r := t.TempDir()
	p := filepath.Join(r, "a.epub")
	os.WriteFile(p, []byte("a"), 0644)
	if got := Unique(p); got == p {
		t.Fatal("not uniquified")
	}
}
