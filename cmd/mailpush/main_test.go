package main

import (
	"mailpush-koreader/internal/config"
	"mailpush-koreader/internal/message"
	"os"
	"path/filepath"
	"testing"
)

func testCfg(t *testing.T) config.Config {
	root := t.TempDir()
	c := config.Defaults()
	c.Host = "x"
	c.User = "u"
	c.Password = "p"
	c.Root = root
	c.DownloadDir = filepath.Join(root, "downloads")
	c.AutoUnpack = false
	return c
}
func TestSingleSaveToDirectoryAppliesToMultipleAttachments(t *testing.T) {
	c := testCfg(t)
	pm := message.Parsed{SaveTo: []string{"books/"}, Attachments: []message.Attachment{{Name: "a.epub", Data: []byte("a")}, {Name: "b.pdf", Data: []byte("b")}}}
	files, errs := process(c, pm)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(files) != 2 {
		t.Fatal(files)
	}
	for _, name := range []string{"a.epub", "b.pdf"} {
		p := filepath.Join(c.DownloadDir, "books", name)
		if _, e := os.Stat(p); e != nil {
			t.Fatalf("missing %s: %v", p, e)
		}
	}
}
func TestSaveToCannotEscapeRoot(t *testing.T) {
	c := testCfg(t)
	pm := message.Parsed{SaveTo: []string{"../../evil.epub"}, Attachments: []message.Attachment{{Name: "a.epub", Data: []byte("a")}}}
	files, errs := process(c, pm)
	if len(files) != 0 || len(errs) == 0 {
		t.Fatalf("files=%v errs=%v", files, errs)
	}
}
