package state

import (
	"path/filepath"
	"testing"
)

func TestUIDValidityResetsState(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	s, _ := Load(p, 10)
	s.Add(42)
	if e := Save(p, s); e != nil {
		t.Fatal(e)
	}
	s2, _ := Load(p, 10)
	if !s2.Has(42) {
		t.Fatal("uid lost")
	}
	s3, _ := Load(p, 11)
	if s3.Has(42) {
		t.Fatal("state did not reset on UIDVALIDITY change")
	}
}
