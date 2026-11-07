// Package core is the lender logic: apply, ingest, offer, accept, fund and
// settle. It depends only on the interfaces below, so the storage and the event
// delivery can change without touching it: the kit uses a file or in-memory
// Store and in-process events.
//
// Keep HTTP-server types out of this package.
package core

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Store.Get when the key does not exist.
var ErrNotFound = errors.New("not found")

// Store holds application state as opaque values by key.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, value []byte) error
	// Clear removes everything the lender has stored. It is what the Reset
	// button calls, together with each app emulator's own reset.
	Clear(ctx context.Context) error
}

// Publisher emits an event about the application, such as an offer being made
// or a settlement landing, for the UI timeline.
type Publisher interface {
	Publish(ctx context.Context, topic string, body []byte) error
}
