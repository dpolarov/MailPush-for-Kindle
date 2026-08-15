package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Host                  string `json:"host"`
	Port                  int    `json:"port"`
	User                  string `json:"user"`
	Password              string `json:"password"`
	Mailbox               string `json:"mailbox"`
	TLS                   bool   `json:"tls"`
	CAFile                string `json:"ca_file"`
	DownloadDir           string `json:"download_dir"`
	Root                  string `json:"root"`
	MaxAgeDays            int    `json:"max_age_days"`
	MaxMessages           int    `json:"max_messages"`
	FetchUnreadOnly       bool   `json:"fetch_unread_only"`
	MarkSeen              bool   `json:"mark_seen"`
	FetchOnStart          bool   `json:"fetch_on_start"`
	AutoUnpack            bool   `json:"auto_unpack"`
	MaxFileBytes          int64  `json:"max_file_bytes"`
	MaxMessageBytes       int64  `json:"max_message_bytes"`
	MaxArchiveBytes       int64  `json:"max_archive_bytes"`
	MaxArchiveFiles       int    `json:"max_archive_files"`
	ConnectTimeoutSeconds int    `json:"connect_timeout_seconds"`
	IOTimeoutSeconds      int    `json:"io_timeout_seconds"`
	HTTPTimeoutSeconds    int    `json:"http_timeout_seconds"`
}

func Defaults() Config {
	return Config{
		Port: 993, Mailbox: "INBOX", TLS: true,
		DownloadDir: "/mnt/us/documents/downloads", Root: "/mnt/us",
		MaxAgeDays: 7, MaxMessages: 20, FetchUnreadOnly: true, MarkSeen: true,
		AutoUnpack: true, MaxFileBytes: 100 * 1024 * 1024, MaxMessageBytes: 150 * 1024 * 1024,
		MaxArchiveBytes: 300 * 1024 * 1024, MaxArchiveFiles: 500,
		ConnectTimeoutSeconds: 15, IOTimeoutSeconds: 45, HTTPTimeoutSeconds: 60,
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("cannot read configuration: %w", err)
	}
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("configuration is not valid JSON: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("IMAP host is empty")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("IMAP port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.User) == "" {
		return errors.New("IMAP username is empty")
	}
	if c.Password == "" {
		return errors.New("IMAP password is empty")
	}
	if c.Mailbox == "" {
		return errors.New("mailbox is empty")
	}
	if !filepath.IsAbs(c.Root) || !filepath.IsAbs(c.DownloadDir) {
		return errors.New("root and download_dir must be absolute paths")
	}
	if c.MaxAgeDays < 0 || c.MaxMessages < 1 || c.MaxFileBytes < 1 || c.MaxMessageBytes < 1 || c.MaxArchiveBytes < 1 || c.MaxArchiveFiles < 1 {
		return errors.New("limits must be positive")
	}
	if c.ConnectTimeoutSeconds < 1 || c.IOTimeoutSeconds < 1 || c.HTTPTimeoutSeconds < 1 {
		return errors.New("timeouts must be positive")
	}
	return nil
}
func (c Config) ConnectTimeout() time.Duration {
	return time.Duration(c.ConnectTimeoutSeconds) * time.Second
}
func (c Config) IOTimeout() time.Duration   { return time.Duration(c.IOTimeoutSeconds) * time.Second }
func (c Config) HTTPTimeout() time.Duration { return time.Duration(c.HTTPTimeoutSeconds) * time.Second }

func SaveAtomic(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
