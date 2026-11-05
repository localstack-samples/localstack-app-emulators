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
	st := s.State()
	if st.Mode != ModeDemo || st.Style != StyleDryRun {
		t.Fatalf("state = %+v", st)
	}
	if got := s.Target("stripe"); got.Source != SourceLocalStack || got.Host == "" {
		t.Fatalf("stripe target = %+v", got)
	}
	if got := s.Target("plaid"); got.Source != SourceLocal || got.Upstream != "http://127.0.0.1:5101" {
		t.Fatalf("plaid target = %+v", got)
	}
}

func TestSwitchServiceURLIsProductionAndDryRunByDefault(t *testing.T) {
	s := NewSwitch(lanes()...)
	if err := s.Apply(Update{URLs: map[string]string{"stripe": "https://api.stripe.com/"}}); err != nil {
		t.Fatal(err)
	}
	if st := s.State(); st.Mode != ModeProduction || !st.Lanes[0].Service || st.Lanes[1].Service {
		t.Fatalf("state = %+v", st)
	}
	got := s.Target("stripe")
	if !got.DryRun || got.Source != SourceService || got.Upstream != "https://api.stripe.com" || got.Invalid != "" {
		t.Fatalf("target = %+v", got)
	}
	live := StyleLive
	if err := s.Apply(Update{Style: &live}); err != nil {
		t.Fatal(err)
	}
	if got := s.Target("stripe"); got.DryRun {
		t.Fatalf("live target is still a dry run: %+v", got)
	}
}

func TestSwitchBackToEmulatorIsDemoMode(t *testing.T) {
	s := NewSwitch(lanes()...)
	_ = s.Apply(Update{URLs: map[string]string{"stripe": "https://api.stripe.com"}})
	_ = s.Apply(Update{URLs: map[string]string{"stripe": s.State().Lanes[0].EmulatorURL}})
	if st := s.State(); st.Mode != ModeDemo {
		t.Fatalf("state = %+v", st)
	}
	if got := s.Target("stripe"); got.Source != SourceLocalStack {
		t.Fatalf("target = %+v", got)
	}
}

func TestSwitchReportsAnInvalidURLAndFailsTheCall(t *testing.T) {
	for _, raw := range []string{"", "   ", "stripe", "ftp://x", "http://"} {
		s := NewSwitch(lanes()...)
		_ = s.Apply(Update{URLs: map[string]string{"stripe": raw}})
		lane := s.State().Lanes[0]
		if lane.Error == "" || !strings.Contains(lane.Error, "emulator") && !strings.Contains(lane.Error, "Use") {
			t.Fatalf("%q: no helpful error: %+v", raw, lane)
		}
		if s.Target("stripe").Invalid == "" {
			t.Fatalf("%q: target is callable", raw)
		}
	}
}

func TestSwitchRejectsUnknownStyleAndVendor(t *testing.T) {
	s := NewSwitch(lanes()...)
	bad := "yolo"
	if err := s.Apply(Update{Style: &bad}); err == nil {
		t.Fatal("accepted an unknown style")
	}
	if err := s.Apply(Update{URLs: map[string]string{"nope": "http://x"}}); err == nil {
		t.Fatal("accepted an unknown vendor")
	}
	if st := s.State(); st.Style != StyleDryRun {
		t.Fatalf("a rejected update changed the state: %+v", st)
	}
}
