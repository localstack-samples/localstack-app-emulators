package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreSharesStateAcrossInstances(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	a, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewFileStore(path) // a second process, in effect

	if _, err := b.Get(ctx, "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing key: want ErrNotFound, got %v", err)
	}
	if err := a.Put(ctx, "k", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if got, err := b.Get(ctx, "k"); err != nil || string(got) != "v1" {
		t.Fatalf("the other instance should see the write: %q, %v", got, err)
	}
	if err := b.Put(ctx, "k2", []byte{0, 1, 2, 255}); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Get(ctx, "k2"); string(got) != string([]byte{0, 1, 2, 255}) {
		t.Fatalf("binary values must survive: %v", got)
	}
	if err := a.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Get(ctx, "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after Clear: want ErrNotFound, got %v", err)
	}
}

func TestFileStoreKeepsTheFilePrivateAndLeavesNoTempFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, _ := NewFileStore(path)
	if err := s.Put(context.Background(), "k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file left behind: %v", err)
	}
}

func TestFileStoreReportsACorruptFileInsteadOfWipingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := NewFileStore(path)
	if err := s.Put(context.Background(), "k", []byte("v")); err == nil {
		t.Fatal("Put must fail on a corrupt file and not overwrite it")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "{not json" {
		t.Fatalf("the corrupt file was overwritten: %q", raw)
	}
}
