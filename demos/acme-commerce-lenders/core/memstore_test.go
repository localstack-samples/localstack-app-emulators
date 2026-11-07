package core

import (
	"context"
	"errors"
	"testing"
)

func TestMemStore(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()

	if _, err := s.Get(ctx, "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing key: want ErrNotFound, got %v", err)
	}
	if err := s.Put(ctx, "a", []byte("one")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "a")
	if err != nil || string(got) != "one" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	// Callers cannot change stored values through the returned slice.
	got[0] = 'X'
	again, _ := s.Get(ctx, "a")
	if string(again) != "one" {
		t.Fatalf("stored value was mutated: %q", again)
	}

	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after Clear: want ErrNotFound, got %v", err)
	}
}
