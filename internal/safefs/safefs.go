package safefs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Resolve(root, base, requested, fallback string) (string, error) {
	root, _ = filepath.Abs(filepath.Clean(root))
	if requested == "" {
		requested = fallback
	}
	requested = strings.ReplaceAll(requested, "\\", "/")
	var p string
	if filepath.IsAbs(requested) {
		p = filepath.Clean(requested)
	} else {
		p = filepath.Join(base, requested)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("target path is outside configured root: %s", requested)
	}
	// Reject any existing symlink component between root and parent.
	cur := root
	relParent, _ := filepath.Rel(root, filepath.Dir(abs))
	for _, part := range strings.Split(relParent, string(os.PathSeparator)) {
		if part == "." || part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		if fi, e := os.Lstat(cur); e == nil && fi.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("target path crosses a symbolic link")
		}
	}
	return abs, nil
}

func Unique(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		p := fmt.Sprintf("%s(%d)%s", base, i, ext)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
}

func AtomicWrite(path string, data []byte, max int64) error {
	if int64(len(data)) > max {
		return fmt.Errorf("file exceeds configured size limit (%d bytes)", max)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mailpush-*.part")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0644); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}
