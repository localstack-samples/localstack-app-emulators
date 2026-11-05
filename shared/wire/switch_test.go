package wire

import (
	"strings"
	"testing"
)

func lanes() []Lane {
	return []Lane{
		{Vendor: "stripe", Name: "Stripe", Emulator: LocalStack("stripe"), ServiceURL: "https://api.stripe.com"},
		{Vendor: "plaid", Name: "Plaid", Emulator: Direct("plaid", "http://127.0.0.1:5101"), ServiceURL: "https://production.plaid.com"},
	}
}

func TestSwitchStartsInDemoMode(t *testing.T) {
	s := NewSwitch(lanes()...)
	if st := s.State(); st.Mode != ModeDemo {
		t.Fatalf("state = %+v", st)
	}
	if got := s.Target("stripe"); got.Source != SourceLocalStack || got.Host == "" || got.WouldSend != "" {
		t.Fatalf("stripe target = %+v", got)
	}
	if got := s.Target("plaid"); got.Source != SourceLocal || got.Upstream != "http://127.0.0.1:5101" {
		t.Fatalf("plaid target = %+v", got)
	}
}

func TestSwitchPointedAtAServiceStaysOnTheEmulatorAndRemembersTheService(t *testing.T) {
	s := NewSwitch(lanes()...)
	if err := s.Apply(map[string]string{"stripe": "https://api.stripe.com/"}); err != nil {
		t.Fatal(err)
	}
	if st := s.State(); st.Mode != ModeProduction || !st.Lanes[0].Service || st.Lanes[1].Service {
		t.Fatalf("state = %+v", st)
	}
	got := s.Target("stripe")
	if got.Source != SourceLocalStack || got.Host == "" || got.WouldSend != "https://api.stripe.com" || got.Invalid != "" {
		t.Fatalf("target = %+v, want the emulator with WouldSend set", got)
	}
	if s.Target("plaid").WouldSend != "" {
		t.Fatal("an untouched vendor has a WouldSend")
	}
}

func TestSwitchBackToEmulatorIsDemoMode(t *testing.T) {
	s := NewSwitch(lanes()...)
	_ = s.Apply(map[string]string{"stripe": "https://api.stripe.com"})
	_ = s.Apply(map[string]string{"stripe": s.State().Lanes[0].EmulatorURL})
	if st := s.State(); st.Mode != ModeDemo {
		t.Fatalf("state = %+v", st)
	}
	if got := s.Target("stripe"); got.WouldSend != "" {
		t.Fatalf("target = %+v", got)
	}
}

func TestSwitchReportsAnInvalidURLAndFailsTheCall(t *testing.T) {
	for _, raw := range []string{"", "   ", "stripe", "ftp://x", "http://"} {
		s := NewSwitch(lanes()...)
		_ = s.Apply(map[string]string{"stripe": raw})
		lane := s.State().Lanes[0]
		if lane.Error == "" || !strings.Contains(lane.Error, "Use") {
			t.Fatalf("%q: no helpful error: %+v", raw, lane)
		}
		if s.Target("stripe").Invalid == "" {
			t.Fatalf("%q: target is usable", raw)
		}
	}
}

func TestSwitchRejectsAnUnknownVendorAndChangesNothing(t *testing.T) {
	s := NewSwitch(lanes()...)
	err := s.Apply(map[string]string{"stripe": "https://api.stripe.com", "nope": "http://x"})
	if err == nil {
		t.Fatal("accepted an unknown vendor")
	}
	if st := s.State(); st.Mode != ModeDemo {
		t.Fatalf("a rejected update changed the state: %+v", st)
	}
}
