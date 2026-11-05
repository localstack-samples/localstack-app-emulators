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
	ModeProduction = "production" // at least one vendor is at a URL that is not its app emulator
)

// Run styles, for a vendor that is not at its app emulator.
const (
	StyleDryRun = "dry"  // build and record the request, send nothing
	StyleLive   = "live" // send it, with no credential
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
	Error       string `json:"error,omitempty"` // why the URL cannot be called
}

// State is what a Switch reports.
type State struct {
	Mode  string      `json:"mode"`
	Style string      `json:"style"`
	Lanes []LaneState `json:"lanes"`
}

// Update changes a Switch. A nil field is left alone.
type Update struct {
	Style *string           `json:"style,omitempty"`
	URLs  map[string]string `json:"urls,omitempty"` // vendor to base URL
}

// Switch holds, for each vendor, the base URL the demo is calling now, and
// whether a call to a vendor's own service is sent. It is safe for concurrent
// use. The demo's flow asks it for a target on every call, so a change takes
// effect at once, with no restart.
//
// A Switch never sends a credential: a call to a vendor's service is either
// dry-run, so it is built and not sent, or live with the Authorization header
// removed.
type Switch struct {
	mu    sync.RWMutex
	lanes []Lane
	urls  map[string]string
	style string
}

// NewSwitch starts every lane at its app emulator, in dry-run style.
func NewSwitch(lanes ...Lane) *Switch {
	s := &Switch{lanes: lanes, urls: map[string]string{}, style: StyleDryRun}
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

// Check says why a base URL cannot be called, or returns "".
func Check(name, raw string) string {
	raw = clean(raw)
	if raw == "" {
		return fmt.Sprintf("The %s URL is empty. Press Use emulator or Use service to fill it in.", name)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Sprintf("%q is not a URL the demo can call. Start it with http:// or https://, such as https://example.com, or press Use emulator.", raw)
	}
	return ""
}

// Target is where the vendor's calls go now.
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
	t := Target{Vendor: vendor, Source: SourceService, Upstream: u, Origin: OriginVendor, DryRun: s.style == StyleDryRun}
	t.Invalid = Check(l.Name, u)
	return t
}

// State reports every lane.
func (s *Switch) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := State{Mode: ModeDemo, Style: s.style}
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

// Apply makes the change. It rejects a style or a vendor it does not know, and
// otherwise accepts any URL: one that cannot be called is reported by State and
// fails the call that uses it, so a presenter sees why.
func (s *Switch) Apply(u Update) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.Style != nil && *u.Style != StyleDryRun && *u.Style != StyleLive {
		return fmt.Errorf("style must be %q or %q", StyleDryRun, StyleLive)
	}
	for v := range u.URLs {
		if _, ok := s.lane(v); !ok {
			return fmt.Errorf("no vendor named %q", v)
		}
	}
	if u.Style != nil {
		s.style = *u.Style
	}
	for v, raw := range u.URLs {
		s.urls[v] = clean(raw)
	}
	return nil
}
