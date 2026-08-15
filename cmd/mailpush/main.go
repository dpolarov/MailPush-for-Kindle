package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"mailpush-koreader/internal/archive"
	"mailpush-koreader/internal/config"
	"mailpush-koreader/internal/download"
	"mailpush-koreader/internal/imapclient"
	"mailpush-koreader/internal/message"
	"mailpush-koreader/internal/safefs"
	"mailpush-koreader/internal/state"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Result struct {
	OK         bool     `json:"ok"`
	Message    string   `json:"message"`
	Downloaded []string `json:"downloaded,omitempty"`
	Errors     []string `json:"errors,omitempty"`
	Messages   int      `json:"messages"`
	Skipped    int      `json:"skipped"`
}

func emit(r Result) { json.NewEncoder(os.Stdout).Encode(r) }
func locateBundle(cfgPath, explicit string) string {
	if explicit != "" {
		return explicit
	}
	p := filepath.Join(filepath.Dir(cfgPath), "cacert.pem")
	if _, e := os.Stat(p); e == nil {
		return p
	}
	return ""
}
func connect(cfg config.Config, cfgPath, bundle string) (*imapclient.Client, error) {
	c, e := imapclient.Dial(cfg.Host, cfg.Port, cfg.TLS, cfg.CAFile, locateBundle(cfgPath, bundle), cfg.ConnectTimeout(), cfg.IOTimeout())
	if e != nil {
		return nil, e
	}
	if e = c.Login(cfg.User, cfg.Password); e != nil {
		c.Close()
		return nil, e
	}
	if e = c.Select(cfg.Mailbox); e != nil {
		c.Close()
		return nil, e
	}
	return c, nil
}
func requestedPath(requested, fallback string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return fallback
	}
	if strings.HasSuffix(requested, "/") || strings.HasSuffix(requested, "\\") {
		return filepath.Join(requested, fallback)
	}
	return requested
}

func saveOne(cfg config.Config, requested, fallback string, data []byte) (string, error) {
	requested = requestedPath(requested, fallback)
	p, e := safefs.Resolve(cfg.Root, cfg.DownloadDir, requested, fallback)
	if e != nil {
		return "", e
	}
	p = safefs.Unique(p)
	if e := safefs.AtomicWrite(p, data, cfg.MaxFileBytes); e != nil {
		return "", e
	}
	return p, nil
}
func maybeUnpack(cfg config.Config, p string) ([]string, error) {
	if !cfg.AutoUnpack {
		return []string{p}, nil
	}
	o, ok, e := archive.Maybe(p, archive.Limits{Bytes: cfg.MaxArchiveBytes, Files: cfg.MaxArchiveFiles})
	if e != nil {
		return nil, e
	}
	if ok {
		os.Remove(p)
		return o, nil
	}
	return []string{p}, nil
}
func process(cfg config.Config, pm message.Parsed) ([]string, []string) {
	var files, errs []string
	idx := 0
	next := func(fallback string) string {
		if len(pm.SaveTo) == 1 && (strings.HasSuffix(pm.SaveTo[0], "/") || strings.HasSuffix(pm.SaveTo[0], "\\")) {
			idx++
			return requestedPath(pm.SaveTo[0], fallback)
		}
		if idx < len(pm.SaveTo) {
			s := pm.SaveTo[idx]
			idx++
			return requestedPath(s, fallback)
		}
		idx++
		return fallback
	}
	for _, u := range pm.URLs {
		req := next(download.Name(u))
		p, e := safefs.Resolve(cfg.Root, cfg.DownloadDir, req, download.Name(u))
		if e == nil {
			os.MkdirAll(filepath.Dir(p), 0755)
			p = safefs.Unique(p)
			_, e = download.HTTP(u, p, cfg.MaxFileBytes, cfg.HTTPTimeout())
		}
		if e != nil {
			errs = append(errs, "URL "+u+": "+e.Error())
			continue
		}
		o, e := maybeUnpack(cfg, p)
		if e != nil {
			errs = append(errs, p+": "+e.Error())
		} else {
			files = append(files, o...)
		}
	}
	for _, a := range pm.Attachments {
		req := next(a.Name)
		p, e := saveOne(cfg, req, a.Name, a.Data)
		if e != nil {
			errs = append(errs, a.Name+": "+e.Error())
			continue
		}
		o, e := maybeUnpack(cfg, p)
		if e != nil {
			errs = append(errs, p+": "+e.Error())
		} else {
			files = append(files, o...)
		}
	}
	return files, errs
}
func main() {
	cfgPath := flag.String("config", "config.json", "config path")
	statePath := flag.String("state", "state.json", "state path")
	bundlePath := flag.String("ca-bundle", "", "bundled CA certificate file")
	flag.Parse()
	cmd := "fetch"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}
	cfg, e := config.Load(*cfgPath)
	if e != nil {
		emit(Result{Message: e.Error()})
		os.Exit(2)
	}
	if cmd == "test" {
		c, e := connect(cfg, *cfgPath, *bundlePath)
		if e != nil {
			emit(Result{Message: e.Error()})
			os.Exit(1)
		}
		c.Close()
		emit(Result{OK: true, Message: "Connection and authentication succeeded."})
		return
	}
	if cmd != "fetch" {
		emit(Result{Message: "Unknown command."})
		os.Exit(2)
	}
	c, e := connect(cfg, *cfgPath, *bundlePath)
	if e != nil {
		emit(Result{Message: e.Error()})
		os.Exit(1)
	}
	defer c.Close()
	st, e := state.Load(*statePath, c.UIDValidity)
	if e != nil {
		emit(Result{Message: "Cannot read state: " + e.Error()})
		os.Exit(1)
	}
	since := time.Now().AddDate(0, 0, -cfg.MaxAgeDays)
	uids, e := c.Search(since, cfg.FetchUnreadOnly)
	if e != nil {
		emit(Result{Message: "Cannot search mailbox: " + e.Error()})
		os.Exit(1)
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })
	pending := make([]uint32, 0, len(uids))
	res := Result{OK: true}
	for _, uid := range uids {
		if st.Has(uid) {
			res.Skipped++
			continue
		}
		pending = append(pending, uid)
	}
	if len(pending) > cfg.MaxMessages {
		pending = pending[:cfg.MaxMessages]
	}
	for _, uid := range pending {
		raw, e := c.FetchPeek(uid, cfg.MaxMessageBytes)
		if e != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("UID %d: %v", uid, e))
			continue
		}
		pm, e := message.Parse(raw)
		if e != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("UID %d: cannot parse message: %v", uid, e))
			continue
		}
		files, errs := process(cfg, pm)
		res.Errors = append(res.Errors, errs...)
		if len(errs) == 0 {
			st.Add(uid)
			if e := state.Save(*statePath, st); e != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("UID %d downloaded, but processed state could not be saved: %v", uid, e))
			} else if cfg.MarkSeen {
				if e := c.MarkSeen(uid); e != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("UID %d downloaded, but could not mark as read: %v", uid, e))
				}
			}
		}
		res.Downloaded = append(res.Downloaded, files...)
		res.Messages++
	}
	if e := state.Save(*statePath, st); e != nil {
		res.OK = false
		res.Errors = append(res.Errors, "Cannot save state: "+e.Error())
	}
	if len(res.Errors) > 0 {
		res.OK = false
	}
	if len(res.Downloaded) == 0 && len(res.Errors) == 0 {
		res.Message = "No new files found."
	} else {
		res.Message = fmt.Sprintf("Processed %d message(s), downloaded %d file(s).", res.Messages, len(res.Downloaded))
	}
	if len(res.Errors) > 0 {
		res.Message += " Some items failed. Check details."
	}
	res.Message = strings.TrimSpace(res.Message)
	emit(res)
	if !res.OK {
		os.Exit(1)
	}
}
