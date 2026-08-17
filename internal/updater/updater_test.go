package updater

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct{ current, latest string; want bool }{
		{"v1.0.0", "v1.0.1", true},
		{"1.9.9", "v2.0.0", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.2.1", "v1.2.0", false},
		{"dev", "v9.0.0", false},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.current, tc.latest); got != tc.want {
			t.Fatalf("IsNewer(%q,%q)=%v want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestSafeZipPath(t *testing.T) {
	for _, bad := range []string{"../evil", "/tmp/evil", "mailpush.koplugin/../../evil", "other/file"} {
		if _, err := safeZipPath(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
	if _, err := safeZipPath("mailpush.koplugin/bin/mailpush"); err != nil {
		t.Fatal(err)
	}
}

func TestExtractRequiresPluginLayout(t *testing.T) {
	d := t.TempDir()
	zp := filepath.Join(d, "bad.zip")
	f, err := os.Create(zp)
	if err != nil { t.Fatal(err) }
	zw := zip.NewWriter(f)
	w, err := zw.Create("mailpush.koplugin/main.lua")
	if err != nil { t.Fatal(err) }
	_, _ = w.Write([]byte("return {}"))
	if err := zw.Close(); err != nil { t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }
	if _, err := extract(zp, filepath.Join(d, "out")); err == nil {
		t.Fatal("expected incomplete update archive to be rejected")
	}
}
