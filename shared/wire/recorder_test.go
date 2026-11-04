package wire

import (
	"testing"
	"time"
)

func TestRecorderDropsOldestAndKeepsIDsIncreasing(t *testing.T) {
	r := NewRecorder(3)
	for i := 0; i < 5; i++ {
		r.Record(Event{Path: "/p"})
	}
	evs := r.Events()
	if len(evs) != 3 || evs[0].ID != 3 || evs[2].ID != 5 {
		t.Fatalf("want events 3..5, got %+v", evs)
	}
}

func TestRecorderSinceAndClear(t *testing.T) {
	r := NewRecorder(10)
	r.Record(Event{})
	r.Record(Event{})
	if got := r.Since(1); len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("Since(1) = %+v", got)
	}
	r.Clear()
	if len(r.Events()) != 0 {
		t.Fatal("Clear left events behind")
	}
	if e := r.Record(Event{}); e.ID != 3 {
		t.Fatalf("IDs must keep counting after Clear, got %d", e.ID)
	}
}

func TestRecorderSubscribe(t *testing.T) {
	r := NewRecorder(10)
	ch, cancel := r.Subscribe()
	r.Record(Event{Path: "/a"})
	select {
	case e := <-ch:
		if e.Path != "/a" || e.ID != 1 {
			t.Fatalf("unexpected event %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber was not told about the event")
	}

	cancel()
	r.Record(Event{Path: "/b"})
	select {
	case e := <-ch:
		t.Fatalf("cancelled subscriber still received %+v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

// A subscriber that never reads must not block the demo's own calls.
func TestRecorderDoesNotBlockOnSlowSubscriber(t *testing.T) {
	r := NewRecorder(1000)
	_, cancel := r.Subscribe()
	defer cancel()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 500; i++ {
			r.Record(Event{})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Record blocked on a subscriber that is not reading")
	}
}

func TestRecorderGenerationCountsClears(t *testing.T) {
	r := NewRecorder(10)
	if r.Generation() != 0 {
		t.Fatalf("a new recorder is generation 0, got %d", r.Generation())
	}
	r.Record(Event{})
	if r.Generation() != 0 {
		t.Fatal("recording must not change the generation")
	}
	r.Clear()
	r.Clear()
	if r.Generation() != 2 {
		t.Fatalf("want generation 2 after two clears, got %d", r.Generation())
	}
}
