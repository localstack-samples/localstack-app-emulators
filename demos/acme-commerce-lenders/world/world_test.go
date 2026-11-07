package world

import (
	"reflect"
	"testing"
)

func TestOrdersAreTheSameEveryTime(t *testing.T) {
	a, b := Orders(), Orders()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("Orders must return the same orders on every call")
	}
}

// These numbers pin the world. If a change moves them, the demo's story moves,
// and that should be a decision and not an accident.
func TestTheWorldIsPinned(t *testing.T) {
	orders := Orders()
	if len(orders) != 941 || Gross(orders) != 8108384 {
		t.Fatalf("the world changed: %d orders, gross %d cents", len(orders), Gross(orders))
	}
}

func TestOrdersLookLikeABusiness(t *testing.T) {
	orders := Orders()
	byDay := map[int]int64{}
	for _, o := range orders {
		if o.Day < 0 || o.Day >= Days {
			t.Fatalf("order on day %d is outside the %d days of history", o.Day, Days)
		}
		if o.Cents < 2800 || o.Cents > 14699 {
			t.Fatalf("order of %d cents is outside the expected range", o.Cents)
		}
		byDay[o.Day] += o.Cents
	}
	if len(byDay) != Days {
		t.Fatalf("every day should have sales, got %d of %d days", len(byDay), Days)
	}
	var first, last int64
	for d := 0; d < 28; d++ {
		first += byDay[d]
		last += byDay[Days-28+d]
	}
	if last <= first {
		t.Fatalf("sales should grow over the history: first 28 days %d, last 28 days %d", first, last)
	}
}

func TestOrdersAreInDayOrder(t *testing.T) {
	prev := 0
	for _, o := range Orders() {
		if o.Day < prev {
			t.Fatal("SeedStripe walks the days forward, so orders must be in day order")
		}
		prev = o.Day
	}
}
