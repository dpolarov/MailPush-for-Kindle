package message

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"path/filepath"
	"regexp"
	"strings"
)

type Attachment struct {
	Name string
	Data []byte
}
type Parsed struct {
	Subject     string
	Text        string
	Attachments []Attachment
	SaveTo      []string
	URLs        []string
}

var urlRE = regexp.MustCompile(`https?://[^\s<>"'|]+`)

func DecodeHeader(s string) string {
	v, e := new(mime.WordDecoder).DecodeHeader(s)
	if e == nil {
		return v
	}
	return s
}
func parsePart(h textproto.MIMEHeader, r io.Reader, out *Parsed) error {
	ct, params, _ := mime.ParseMediaType(h.Get("Content-Type"))
	disp, cdparams, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	name := params["name"]
	if disp != "" {
		if v := cdparams["filename"]; v != "" {
			name = v
		}
	}
	name = DecodeHeader(name)
	data, e := io.ReadAll(io.LimitReader(r, 110*1024*1024))
	if e != nil {
		return e
	}
	cte := strings.ToLower(h.Get("Content-Transfer-Encoding"))
	if cte == "base64" {
		clean := bytes.NewBuffer(nil)
		for _, b := range data {
			if b != '\r' && b != '\n' {
				clean.WriteByte(b)
			}
		}
		d := make([]byte, base64.StdEncoding.DecodedLen(clean.Len()))
		n, e := base64.StdEncoding.Decode(d, clean.Bytes())
		if e == nil {
			data = d[:n]
		}
	}
	if name != "" || strings.HasPrefix(strings.ToLower(h.Get("Content-Disposition")), "attachment") {
		if name == "" {
			name = "attachment"
		}
		out.Attachments = append(out.Attachments, Attachment{Name: filepath.Base(name), Data: data})
		return nil
	}
	if strings.HasPrefix(ct, "text/plain") || strings.HasPrefix(ct, "text/html") || ct == "" {
		out.Text += "\n" + html.UnescapeString(string(data))
	}
	return nil
}
func walk(entity *mail.Message, out *Parsed) error {
	ct, p, _ := mime.ParseMediaType(entity.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/") {
		mr := multipart.NewReader(entity.Body, p["boundary"])
		for {
			part, e := mr.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				return e
			}
			pct, pp, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			if strings.HasPrefix(pct, "multipart/") {
				b, _ := io.ReadAll(part)
				fake := &mail.Message{Header: mail.Header(part.Header), Body: bytes.NewReader(b)}
				_ = pp
				if e := walk(fake, out); e != nil {
					return e
				}
			} else if e := parsePart(textproto.MIMEHeader(part.Header), part, out); e != nil {
				return e
			}
		}
		return nil
	}
	return parsePart(textproto.MIMEHeader(entity.Header), entity.Body, out)
}
func Parse(raw []byte) (Parsed, error) {
	m, e := mail.ReadMessage(bytes.NewReader(raw))
	if e != nil {
		return Parsed{}, e
	}
	o := Parsed{Subject: DecodeHeader(m.Header.Get("Subject"))}
	if e := walk(m, &o); e != nil {
		return o, e
	}
	all := o.Subject + "\n" + o.Text
	for _, line := range strings.Split(all, "\n") {
		s := strings.TrimSpace(line)
		low := strings.ToLower(s)
		prefix := ""
		switch {
		case strings.HasPrefix(low, "save to "):
			prefix = s[len("save to "):]
		case strings.HasPrefix(low, "saveto "):
			prefix = s[len("saveto "):]
		case strings.HasPrefix(low, "сохранить в "):
			prefix = s[len("сохранить в "):]
		}
		if prefix != "" {
			for _, v := range regexp.MustCompile(`[|<>]`).Split(prefix, -1) {
				v = strings.TrimSpace(v)
				if v != "" {
					o.SaveTo = append(o.SaveTo, v)
				}
			}
		}
		for _, u := range urlRE.FindAllString(s, -1) {
			o.URLs = append(o.URLs, strings.TrimRight(u, ".,;)]}"))
		}
	}
	if len(o.URLs) > 1 {
		seen := make(map[string]bool, len(o.URLs))
		dedup := o.URLs[:0]
		for _, u := range o.URLs {
			if !seen[u] {
				seen[u] = true
				dedup = append(dedup, u)
			}
		}
		o.URLs = dedup
	}
	return o, nil
}
func (p Parsed) String() string {
	return fmt.Sprintf("subject=%q attachments=%d urls=%d", p.Subject, len(p.Attachments), len(p.URLs))
}
