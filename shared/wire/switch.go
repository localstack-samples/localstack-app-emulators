package wire

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// Modes a Switch reports.
const (
	ModeDemo       = "demo"       // every vendor is at its app emulator
	ModeProduction = "production" // at least one vendor has been pointed at its own service
)

// Lane is one vendor a Switch can point at its app emulator or at the
// vendor's own service.
type Lane struct {
	Vendor     string
	Name       string // how the UI writes the vendor, such as "Stripe"
	Emulator   Target // where the app emulator is
	ServiceURL string // the vendor's own base URL
}

// LaneState is one lane as the UI shows it.
type LaneState struct {
	Vendor      string `json:"vendor"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	EmulatorURL string `json:"emulator_url"`
	ServiceURL  string `json:"service_url"`
	Service     bool   `json:"service"`         // the URL is not the app emulator's
	Error       string `json:"error,omitempty"` // why the URL cannot be used
}

// State is what a Switch reports.
type State struct {
	Mode  string      `json:"mode"`
	Lanes []LaneState `json:"lanes"`
}

// Switch holds, for each vendor, the base URL the demo has been pointed at.
//
// The demo shows the mechanism of moving from an app emulator to a vendor: only
// the base URL changes. It holds no credential for a vendor's service, so a
// call is never sent to one. A call to a vendor that has been pointed at its
// own service is still answered by the app emulator, and is recorded with where
// it would have gone.
//
// A Switch is safe for concurrent use. The demo's flow asks it for a target on
// every call, so a change takes effect at once, with no restart.
type Switch struct {
	mu    sync.RWMutex
	lanes []Lane
	urls  map[string]string
}

// NewSwitch starts every lane at its app emulator.
func NewSwitch(lanes ...Lane) *Switch {
	s := &Switch{lanes: lanes, urls: map[string]string{}}
	for _, l := range lanes {
		s.urls[l.Vendor] = l.Emulator.BaseURL()
	}
	return s
}

func (s *Switch) lane(vendor string) (Lane, bool) {
	for _, l := range s.lanes {
		if l.Vendor == vendor {
			return l, true
		}
	}
	return Lane{}, false
}

func clean(u string) string { return strings.TrimRight(strings.TrimSpace(u), "/") }

// Check says why a base URL cannot be used, or returns "".
func Check(name, raw string) string {
	raw = clean(raw)
	if raw == "" {
		return fmt.Sprintf("The %s URL is empty. Press Use emulator or Use service to fill it in.", name)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Sprintf("%q is not a URL the demo can use. Start it with http:// or https://, such as https://example.com, or press Use emulator.", raw)
	}
	return ""
}

// Target is where the vendor's calls go now: always the app emulator, with
// WouldSend set when the vendor has been pointed at another URL. A URL that
// cannot be used makes every call fail with the reason.
func (s *Switch) Target(vendor string) Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.lane(vendor)
	if !ok {
		return Target{Vendor: vendor, Invalid: fmt.Sprintf("no vendor named %q", vendor)}
	}
	u := s.urls[vendor]
	if u == l.Emulator.BaseURL() {
		return l.Emulator
	}
	if reason := Check(l.Name, u); reason != "" {
		return Target{Vendor: vendor, Source: l.Emulator.Source, Invalid: reason}
	}
	t := l.Emulator
	t.WouldSend = u
	return t
}

// State reports every lane.
func (s *Switch) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := State{Mode: ModeDemo}
	for _, l := range s.lanes {
		u := s.urls[l.Vendor]
		ls := LaneState{
			Vendor: l.Vendor, Name: l.Name, URL: u,
			EmulatorURL: l.Emulator.BaseURL(), ServiceURL: l.ServiceURL,
			Service: u != l.Emulator.BaseURL(),
		}
		if ls.Service {
			st.Mode = ModeProduction
			ls.Error = Check(l.Name, u)
		}
		st.Lanes = append(st.Lanes, ls)
	}
	return st
}

// Apply points vendors at URLs. It rejects a vendor it does not know, and
// otherwise accepts any URL: one that cannot be used is reported by State and
// fails the call that uses it, so a presenter sees why.
func (s *Switch) Apply(urls map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for v := range urls {
		if _, ok := s.lane(v); !ok {
			return fmt.Errorf("no vendor named %q", v)
		}
	}
	for v, raw := range urls {
		s.urls[v] = clean(raw)
	}
	return nil
}
