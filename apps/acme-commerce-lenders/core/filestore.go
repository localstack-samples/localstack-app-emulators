package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore is a Store kept in one JSON file, so the demo's command line and
// its server, which are separate processes, see the same state. Every call
// reads the file, so one process sees what another wrote.
//
// It suits one presenter on one machine. It is not safe for two processes
// writing at the same moment.
type FileStore struct {
	mu   sync.Mutex
	path string
}

// NewFileStore keeps its state in path, creating the directory if needed.
func NewFileStore(path string) (*FileStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	return &FileStore{path: path}, nil
}

func (s *FileStore) load() (map[string][]byte, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string][]byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string][]byte{}
	if len(raw) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("read %s: %w", s.path, err)
	}
	return m, nil
}

// save writes through a temporary file, so a crash never leaves half a state.
func (s *FileStore) save(m map[string][]byte) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *FileStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return nil, err
	}
	v, ok := m[key]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (s *FileStore) Put(_ context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return err
	}
	m[key] = append([]byte(nil), value...)
	return s.save(m)
}

func (s *FileStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save(map[string][]byte{})
}
