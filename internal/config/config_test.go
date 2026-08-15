package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func valid() Config {
	c := Defaults()
	c.Host = "imap.example.com"
	c.User = "u"
	c.Password = "p"
	return c
}
func TestLoadUTF8BOMAndRoundTripWithoutBOM(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "config.json")
	b := []byte("\xef\xbb\xbf{\"host\":\"imap.example.com\",\"port\":993,\"user\":\"u\",\"password\":\"p\",\"mailbox\":\"INBOX\",\"tls\":true,\"ca_file\":\"\",\"download_dir\":\"/tmp/d\",\"root\":\"/tmp\",\"max_age_days\":7,\"max_messages\":20,\"fetch_unread_only\":true,\"mark_seen\":true,\"fetch_on_start\":false,\"auto_unpack\":true,\"max_file_bytes\":1024,\"max_archive_bytes\":2048,\"max_archive_files\":10,\"connect_timeout_seconds\":1,\"io_timeout_seconds\":1,\"http_timeout_seconds\":1}")
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	c, e := Load(p)
	if e != nil {
		t.Fatal(e)
	}
	if c.Host != "imap.example.com" {
		t.Fatal(c.Host)
	}
	if e = SaveAtomic(p, c); e != nil {
		t.Fatal(e)
	}
	out, _ := os.ReadFile(p)
	if bytes.HasPrefix(out, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("SaveAtomic wrote BOM")
	}
	if _, e = Load(p); e != nil {
		t.Fatal(e)
	}
}
func TestRejectUnknownField(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "c.json")
	os.WriteFile(p, []byte(`{"host":"x","unknown":1}`), 0600)
	if _, e := Load(p); e == nil {
		t.Fatal("expected error")
	}
}
func TestValidate(t *testing.T) {
	c := valid()
	c.Port = 70000
	if c.Validate() == nil {
		t.Fatal("expected invalid port")
	}
}
