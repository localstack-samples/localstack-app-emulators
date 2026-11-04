package lender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/core"
	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/world"
)

// Status is how far an application has got.
type Status string

const (
	Applied   Status = "applied"   // the merchant has applied
	Connected Status = "connected" // the lender reached the merchant's three accounts
	Reviewed  Status = "reviewed"  // the lender has read what they hold
	Offered   Status = "offered"   // the lender has priced an offer
	Accepted  Status = "accepted"  // the merchant accepted an approved offer
	Funded    Status = "funded"    // the advance has been paid out to the merchant's bank
)

// Application is one merchant's request for working capital.
type Application struct {
	ID          string       `json:"id"`
	Merchant    string       `json:"merchant"`
	Status      Status       `json:"status"`
	Connections []Connection `json:"connections,omitempty"`
	Signals     *Signals     `json:"signals,omitempty"`
	Offer       *Offer       `json:"offer,omitempty"`
	Funding     *Funding     `json:"funding,omitempty"`
}

var (
	// ErrNotSeeded means there is no merchant yet. Seed or reset the world.
	ErrNotSeeded = errors.New("the world is not seeded; run seed or reset first")
	// ErrNotFound means there is no such application.
	ErrNotFound = errors.New("no such application")
	// ErrDeclined means the lender declined, so there is nothing to accept.
	ErrDeclined = errors.New("the offer was declined, so there is nothing to accept")
)

// WrongStateError means a step was tried out of order.
type WrongStateError struct {
	Step string
	Have Status
	Want Status
}

func (e *WrongStateError) Error() string {
	return fmt.Sprintf("cannot %s an application that is %s; it must be %s", e.Step, e.Have, e.Want)
}

// Service runs applications. Its state is in the Store, so the command line and
// the server see the same applications.
type Service struct {
	Vendors Vendors
	Store   core.Store
	mu      sync.Mutex
}

func (s *Service) facts(ctx context.Context) (world.Facts, error) {
	f, err := world.LoadFacts(ctx, s.Store)
	if errors.Is(err, core.ErrNotFound) {
		return f, ErrNotSeeded
	}
	return f, err
}

func key(id string) string { return "applications/" + id }

func (s *Service) load(ctx context.Context, id string) (Application, error) {
	var a Application
	raw, err := s.Store.Get(ctx, key(id))
	if errors.Is(err, core.ErrNotFound) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	return a, json.Unmarshal(raw, &a)
}

func (s *Service) save(ctx context.Context, a Application) error {
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return s.Store.Put(ctx, key(a.ID), raw)
}

// Get returns an application.
func (s *Service) Get(ctx context.Context, id string) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(ctx, id)
}

// Apply starts an application for the seeded merchant.
func (s *Service) Apply(ctx context.Context) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.facts(ctx)
	if err != nil {
		return Application{}, err
	}
	n := 0
	if raw, err := s.Store.Get(ctx, "applications/counter"); err == nil {
		n, _ = strconv.Atoi(string(raw))
	}
	n++
	if err := s.Store.Put(ctx, "applications/counter", []byte(strconv.Itoa(n))); err != nil {
		return Application{}, err
	}
	a := Application{ID: fmt.Sprintf("app-%04d", n), Merchant: f.Merchant, Status: Applied}
	return a, s.save(ctx, a)
}

// step loads an application, checks that it is at the right stage, runs fn, and
// saves the result. If fn fails nothing is saved, so the step can be tried again.
func (s *Service) step(ctx context.Context, id, name string, want, next Status, fn func(context.Context, world.Facts, *Application) error) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.load(ctx, id)
	if err != nil {
		return a, err
	}
	if a.Status != want {
		return a, &WrongStateError{Step: name, Have: a.Status, Want: want}
	}
	f, err := s.facts(ctx)
	if err != nil {
		return a, err
	}
	if err := fn(ctx, f, &a); err != nil {
		return a, err
	}
	a.Status = next
	return a, s.save(ctx, a)
}

// Connect reaches the merchant's Stripe, Plaid and Xero accounts.
func (s *Service) Connect(ctx context.Context, id string) (Application, error) {
	return s.step(ctx, id, "connect", Applied, Connected, func(ctx context.Context, f world.Facts, a *Application) error {
		a.Connections = nil
		for _, connect := range []func(context.Context, world.Facts) (Connection, error){
			s.Vendors.connectStripe, s.Vendors.connectPlaid, s.Vendors.connectXero,
		} {
			c, err := connect(ctx, f)
			if err != nil {
				return err
			}
			a.Connections = append(a.Connections, c)
		}
		return nil
	})
}

// Review reads what the merchant's three accounts hold.
func (s *Service) Review(ctx context.Context, id string) (Application, error) {
	return s.step(ctx, id, "review", Connected, Reviewed, func(ctx context.Context, f world.Facts, a *Application) error {
		var sig Signals
		var err error
		if sig.Stripe, err = s.Vendors.readStripe(ctx, f); err != nil {
			return err
		}
		if sig.Plaid, err = s.Vendors.readPlaid(ctx, f); err != nil {
			return err
		}
		if sig.Xero, err = s.Vendors.readXero(ctx, f); err != nil {
			return err
		}
		a.Signals = &sig
		return nil
	})
}

// Offer prices working capital from what the review found.
func (s *Service) Offer(ctx context.Context, id string) (Application, error) {
	return s.step(ctx, id, "offer", Reviewed, Offered, func(_ context.Context, _ world.Facts, a *Application) error {
		o := MakeOffer(*a.Signals)
		a.Offer = &o
		return nil
	})
}

// Accept records the merchant accepting an approved offer.
func (s *Service) Accept(ctx context.Context, id string) (Application, error) {
	return s.step(ctx, id, "accept", Offered, Accepted, func(_ context.Context, _ world.Facts, a *Application) error {
		if a.Offer == nil || !a.Offer.Approved {
			return ErrDeclined
		}
		return nil
	})
}
