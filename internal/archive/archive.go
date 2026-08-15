package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"mailpush-koreader/internal/safefs"
	"os"
	"path/filepath"
	"strings"
)

type Limits struct {
	Bytes int64
	Files int
}

func unpackEntry(root, name string, r io.Reader, mode os.FileMode, lim Limits, total *int64, count *int) (string, error) {
	if *count >= lim.Files {
		return "", fmt.Errorf("archive contains too many files")
	}
	if mode&os.ModeSymlink != 0 {
		return "", fmt.Errorf("archive contains a symbolic link")
	}
	p, e := safefs.Resolve(root, root, name, filepath.Base(name))
	if e != nil {
		return "", e
	}
	if strings.HasSuffix(name, "/") {
		return p, os.MkdirAll(p, 0755)
	}
	os.MkdirAll(filepath.Dir(p), 0755)
	f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if os.IsExist(e) {
		p = safefs.Unique(p)
		f, e = os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	}
	if e != nil {
		return "", e
	}
	defer f.Close()
	n, e := io.Copy(f, io.LimitReader(r, lim.Bytes-*total+1))
	*total += n
	*count++
	if *total > lim.Bytes {
		os.Remove(p)
		return "", fmt.Errorf("archive exceeds unpacked size limit")
	}
	return p, e
}
func Zip(src, dst string, lim Limits) ([]string, error) {
	zr, e := zip.OpenReader(src)
	if e != nil {
		return nil, e
	}
	defer zr.Close()
	var out []string
	var total int64
	count := 0
	for _, z := range zr.File {
		if z.FileInfo().Mode()&os.ModeSymlink != 0 {
			return out, fmt.Errorf("archive contains a symbolic link: %s", z.Name)
		}
		r, e := z.Open()
		if e != nil {
			return out, e
		}
		p, e := unpackEntry(dst, z.Name, r, z.Mode(), lim, &total, &count)
		r.Close()
		if e != nil {
			return out, e
		}
		if !z.FileInfo().IsDir() {
			out = append(out, p)
		}
	}
	return out, nil
}
func Tar(src, dst string, kind string, lim Limits) ([]string, error) {
	f, e := os.Open(src)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	var r io.Reader = f
	if kind == "gz" {
		g, e := gzip.NewReader(f)
		if e != nil {
			return nil, e
		}
		defer g.Close()
		r = g
	} else if kind == "bz2" {
		r = bzip2.NewReader(f)
	}
	tr := tar.NewReader(r)
	var out []string
	var total int64
	count := 0
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return out, e
		}
		if h.Typeflag == tar.TypeSymlink || h.Typeflag == tar.TypeLink {
			return out, fmt.Errorf("archive contains a link: %s", h.Name)
		}
		if h.FileInfo().IsDir() {
			p, e := safefs.Resolve(dst, dst, h.Name, h.Name)
			if e != nil {
				return out, e
			}
			if e = os.MkdirAll(p, 0755); e != nil {
				return out, e
			}
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return out, fmt.Errorf("archive contains unsupported special entry: %s", h.Name)
		}
		p, e := unpackEntry(dst, h.Name, tr, h.FileInfo().Mode(), lim, &total, &count)
		if e != nil {
			return out, e
		}
		out = append(out, p)
	}
	return out, nil
}
func Maybe(src string, lim Limits) ([]string, bool, error) {
	l := strings.ToLower(src)
	var dst, kind string
	switch {
	case strings.HasSuffix(l, ".tar.gz"):
		dst, kind = src[:len(src)-len(".tar.gz")], "gz"
	case strings.HasSuffix(l, ".tgz"):
		dst, kind = src[:len(src)-len(".tgz")], "gz"
	case strings.HasSuffix(l, ".tar.bz2"):
		dst, kind = src[:len(src)-len(".tar.bz2")], "bz2"
	case strings.HasSuffix(l, ".tbz2"):
		dst, kind = src[:len(src)-len(".tbz2")], "bz2"
	case strings.HasSuffix(l, ".zip"):
		dst, kind = src[:len(src)-len(".zip")], "zip"
	case strings.HasSuffix(l, ".tar"):
		dst, kind = src[:len(src)-len(".tar")], "tar"
	default:
		return nil, false, nil
	}
	dst = safefs.Unique(dst)
	var out []string
	var err error
	if kind == "zip" {
		out, err = Zip(src, dst, lim)
	} else {
		if kind == "tar" {
			kind = ""
		}
		out, err = Tar(src, dst, kind, lim)
	}
	if err != nil {
		_ = os.RemoveAll(dst)
		return out, true, err
	}
	return out, true, nil
}
