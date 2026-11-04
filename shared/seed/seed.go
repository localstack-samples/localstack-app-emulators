// Package seed resets, checks and seeds the app emulators a demo runs against.
//
// Reset never defaults to a target. A caller passes the targets it means to
// wipe, and Reset refuses to run with none, because a reset script that
// defaulted to a port once wiped another demo.
package seed

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/vendorcfg"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// State is what Ready found at one app emulator.
type State string

const (
	Healthy     State = "healthy"
	Unreachable State = "unreachable"  // nothing answered
	NotIncluded State = "not-included" // LocalStack has no app emulator at that host name
	Unhealthy   State = "unhealthy"    // something answered, but not a healthy app emulator
)

// Status is one app emulator's readiness.
type Status struct {
	Vendor string `json:"vendor"`
	Source string `json:"source"` // wire.SourceLocalStack or wire.SourceLocal
	URL    string `json:"url"`
	State  State  `json:"state"`
	Detail string `json:"detail,omitempty"`
	// Hint says what to do about a status that is not OK, in a sentence a
	// presenter can act on.
	Hint string `json:"hint,omitempty"`
}

// OK reports whether the app emulator is ready.
func (s Status) OK() bool { return s.State == Healthy }

// hint says what to do about a status that is not OK.
func (s Status) hint() string {
	switch s.State {
	case NotIncluded:
		return fmt.Sprintf("this LocalStack release does not include %s. Run it yourself and set %s.", s.Vendor, vendorcfg.EmulatorEnvName(s.Vendor))
	case Unreachable:
		if s.Source == wire.SourceLocalStack {
			return "LocalStack is not answering on its gateway. Start it with lstk."
		}
		return fmt.Sprintf("nothing is listening at %s. Start the %s app emulator, or unset %s to use LocalStack.", s.URL, s.Vendor, vendorcfg.EmulatorEnvName(s.Vendor))
	case Unhealthy:
		return "it answered, but not with {\"status\":\"ok\"}."
	}
	return ""
}

// Ready checks every target's /admin/health. The first call to an app emulator
// that LocalStack has not installed yet makes LocalStack download it, so give
// hc a generous timeout.
func Ready(ctx context.Context, hc *http.Client, targets ...wire.Target) []Status {
	out := make([]Status, 0, len(targets))
	for _, t := range targets {
		out = append(out, ready(ctx, hc, t))
	}
	return out
}

func ready(ctx context.Context, hc *http.Client, t wire.Target) Status {
	st := Status{Vendor: t.Vendor, Source: t.Source, URL: t.Upstream}
	var body map[string]any
	err := API{HC: hc, Target: t}.JSON(ctx, http.MethodGet, "/admin/health", nil, nil, &body)
	var apiErr *Error
	switch {
	case err == nil:
		if len(body) == 1 && body["status"] == "ok" {
			st.State = Healthy
		} else {
			st.State, st.Detail = Unhealthy, "unexpected health answer"
		}
	case errors.As(err, &apiErr):
		if apiErr.Status == http.StatusNotFound && t.Source == wire.SourceLocalStack {
			st.State = NotIncluded
		} else {
			st.State = Unhealthy
		}
		st.Detail = fmt.Sprintf("HTTP %d", apiErr.Status)
	default:
		st.State, st.Detail = Unreachable, err.Error()
	}
	st.Hint = st.hint()
	return st
}

// ResetResult is the outcome of resetting one app emulator.
type ResetResult struct {
	Vendor string
	Source string
	URL    string
	Err    error
}

// Reset wipes every target through POST /admin/reset. It goes on past a
// failure, so one unreachable app emulator does not leave the others half
// reset, and returns an error naming every vendor that failed. It refuses to
// run with no targets.
func Reset(ctx context.Context, hc *http.Client, targets ...wire.Target) ([]ResetResult, error) {
	if len(targets) == 0 {
		return nil, errors.New("reset needs the targets to wipe named, and has no default")
	}
	results := make([]ResetResult, 0, len(targets))
	var failed []string
	for _, t := range targets {
		err := API{HC: hc, Target: t}.Do(ctx, http.MethodPost, "/admin/reset", nil, "", nil, nil)
		results = append(results, ResetResult{Vendor: t.Vendor, Source: t.Source, URL: t.Upstream, Err: err})
		if err != nil {
			failed = append(failed, t.Vendor)
		}
	}
	if len(failed) > 0 {
		return results, fmt.Errorf("reset failed for %s", strings.Join(failed, ", "))
	}
	return results, nil
}

// AdvanceClock moves an app emulator's clock forward. Reset puts it back to
// real time.
func AdvanceClock(ctx context.Context, hc *http.Client, t wire.Target, d time.Duration) error {
	return API{HC: hc, Target: t}.JSON(ctx, http.MethodPost, "/admin/time/advance", nil, map[string]string{"duration": d.String()}, nil)
}

// Step is one named stage of seeding.
type Step struct {
	Name string
	Do   func(ctx context.Context) error
}

// StepEvent reports a step finishing.
type StepEvent struct {
	Step    string
	Err     error
	Elapsed time.Duration
}

// Run does the steps in order and stops at the first failure, naming the step.
// on, if not nil, hears about each step as it finishes, which is how a UI shows
// progress.
func Run(ctx context.Context, steps []Step, on func(StepEvent)) error {
	for _, s := range steps {
		start := time.Now()
		err := s.Do(ctx)
		if on != nil {
			on(StepEvent{Step: s.Name, Err: err, Elapsed: time.Since(start)})
		}
		if err != nil {
			return fmt.Errorf("seed step %q: %w", s.Name, err)
		}
	}
	return nil
}
