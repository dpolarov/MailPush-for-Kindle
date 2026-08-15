package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

type State struct {
	UIDValidity uint32          `json:"uid_validity"`
	Processed   map[string]bool `json:"processed"`
}

func Load(path string, uv uint32) (State, error) {
	s := State{UIDValidity: uv, Processed: map[string]bool{}}
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	if json.Unmarshal(b, &s) != nil || s.UIDValidity != uv {
		s = State{UIDValidity: uv, Processed: map[string]bool{}}
	}
	if s.Processed == nil {
		s.Processed = map[string]bool{}
	}
	return s, nil
}
func (s *State) Has(uid uint32) bool { return s.Processed[strconv.FormatUint(uint64(uid), 10)] }
func (s *State) Add(uid uint32)      { s.Processed[strconv.FormatUint(uint64(uid), 10)] = true }
func Save(path string, s State) error {
	os.MkdirAll(filepath.Dir(path), 0700)
	b, _ := json.MarshalIndent(s, "", "  ")
	b = append(b, '\n')
	tmp := path + ".tmp"
	if e := os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
