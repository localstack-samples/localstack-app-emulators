// Package wire is the seam between a demo and the vendors it calls.
//
// A demo points each vendor SDK at a local Proxy instead of at the vendor. The
// proxy forwards the call to its upstream (LocalStack's gateway, which routes
// to the app emulator) and records it, bodies included, in a Recorder. The
// inspector reads the Recorder.
package wire

import (
	"sync"
	"time"
)

// Body is a recorded request or response body.
type Body struct {
	Content   string `json:"content"`
	Size      int    `json:"size"`      // bytes on the wire, before any truncation
	Truncated bool   `json:"truncated"` // Content holds only the first part
}

// Event is one call a demo made to a vendor.
type Event struct {
	ID     uint64    `json:"id"`
	Time   time.Time `json:"time"`
	Vendor string    `json:"vendor"`
	Source string    `json:"source"` // "localstack", "local" or "service": who answers the call
	// Origin says who produced the response, and EmulatorVersion and
	// ArtifactVersion say which build of it and which SDK or API version the
	// call was written against. The last two are empty when not known. Together
	// they make a recorded call comparable with the same call made elsewhere.
	Origin          string        `json:"origin"`
	Upstream        string        `json:"upstream,omitempty"` // the base URL the call was addressed to
	EmulatorVersion string        `json:"emulator_version,omitempty"`
	ArtifactVersion string        `json:"artifact_version,omitempty"`
	Method          string        `json:"method"`
	Path            string        `json:"path"` // including the query string
	Status          int           `json:"status"`
	Duration        time.Duration `json:"duration_ns"`
	Request         Body          `json:"request"`
	Response        Body          `json:"response"`
	Error           string        `json:"error,omitempty"` // set when the upstream could not be reached
}

// Recorder keeps the most recent events and tells subscribers about new ones.
// It is safe for concurrent use.
type Recorder struct {
	mu     sync.RWMutex
	max    int
	next   uint64
	clears uint64
	events []Event
	subs   map[chan Event]struct{}
}

// NewRecorder keeps at most max events, dropping the oldest first.
func NewRecorder(max int) *Recorder {
	return &Recorder{max: max, next: 1, subs: map[chan Event]struct{}{}}
}

// Record stores e, assigns it the next ID and returns it.
func (r *Recorder) Record(e Event) Event {
	r.mu.Lock()
	e.ID = r.next
	r.next++
	if len(r.events) >= r.max {
		r.events = r.events[1:]
	}
	r.events = append(r.events, e)
	subs := make([]chan Event, 0, len(r.subs))
	for ch := range r.subs {
		subs = append(subs, ch)
	}
	r.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- e:
		default: // a slow subscriber misses events and can catch up with Since
		}
	}
	return e
}

// Events returns a copy of the stored events, oldest first.
func (r *Recorder) Events() []Event { return r.Since(0) }

// Since returns the stored events with an ID greater than id, oldest first.
func (r *Recorder) Since(id uint64) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Event, 0, len(r.events))
	for _, e := range r.events {
		if e.ID > id {
			out = append(out, e)
		}
	}
	return out
}

// Clear drops every stored event. IDs keep counting, so a client that polls
// with Since never sees an ID repeat. Generation changes, which is how that
// client learns the events it holds are gone.
func (r *Recorder) Clear() {
	r.mu.Lock()
	r.events = nil
	r.clears++
	r.mu.Unlock()
}

// Generation counts the times Clear has been called.
func (r *Recorder) Generation() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clears
}

// Subscribe returns a channel of new events and a function that stops it.
func (r *Recorder) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	r.mu.Lock()
	r.subs[ch] = struct{}{}
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subs, ch)
		r.mu.Unlock()
	}
}
