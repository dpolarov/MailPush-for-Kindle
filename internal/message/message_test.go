package message

import (
	"strings"
	"testing"
)

func TestParseSaveToURLsAndAttachment(t *testing.T) {
	raw := strings.Join([]string{
		"From: a@example.com", "To: b@example.com", "Subject: =?UTF-8?B?0KLQtdGB0YI=?=", "MIME-Version: 1.0", "Content-Type: multipart/mixed; boundary=abc", "", "--abc", "Content-Type: text/plain; charset=utf-8", "", "save to books/one.epub|books/two.pdf", "сохранить в books/three.zip", "https://example.com/a.epub file:///etc/passwd ftp://example.com/x", "--abc", "Content-Type: application/epub+zip; name*=utf-8''book%20name.epub", "Content-Disposition: attachment; filename*=utf-8''book%20name.epub", "Content-Transfer-Encoding: base64", "", "aGVsbG8=", "--abc--", ""}, "\r\n")
	p, e := Parse([]byte(raw))
	if e != nil {
		t.Fatal(e)
	}
	if p.Subject != "Тест" {
		t.Fatalf("subject=%q", p.Subject)
	}
	if len(p.URLs) != 1 || !strings.HasPrefix(p.URLs[0], "https://") {
		t.Fatalf("urls=%v", p.URLs)
	}
	if len(p.SaveTo) != 3 {
		t.Fatalf("saveto=%v", p.SaveTo)
	}
	if len(p.Attachments) != 1 || p.Attachments[0].Name != "book name.epub" || string(p.Attachments[0].Data) != "hello" {
		t.Fatalf("attachment=%+v", p.Attachments)
	}
}
func TestNoFileOrFTPSchemes(t *testing.T) {
	raw := "Subject: file:///etc/passwd ftp://x/a http://ok/a\r\n\r\n"
	p, e := Parse([]byte(raw))
	if e != nil {
		t.Fatal(e)
	}
	if len(p.URLs) != 1 || p.URLs[0] != "http://ok/a" {
		t.Fatalf("urls=%v", p.URLs)
	}
}
