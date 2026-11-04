package lender

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/world"
)

// XeroSignals is what the lender learns from the merchant's books.
type XeroSignals struct {
	Invoices         int   `json:"invoices"`
	BilledCents      int64 `json:"billed_cents"`
	OutstandingCents int64 `json:"outstanding_cents"`
	OverdueCents     int64 `json:"overdue_cents"`
}

const xeroBase = "/api.xro/2.0"

func xeroHeaders(f world.Facts) map[string]string {
	return map[string]string{"Authorization": "Bearer demo", "Xero-Tenant-Id": f.Xero.TenantID}
}

func (v Vendors) connectXero(ctx context.Context, f world.Facts) (Connection, error) {
	var out struct {
		Organisations []struct {
			Name string `json:"Name"`
		} `json:"Organisations"`
	}
	if err := v.Xero.JSON(ctx, http.MethodGet, xeroBase+"/Organisation", xeroHeaders(f), nil, &out); err != nil {
		return Connection{}, fmt.Errorf("reach the merchant's Xero books: %w", err)
	}
	if len(out.Organisations) == 0 {
		return Connection{}, fmt.Errorf("Xero returned no organisation for tenant %s", f.Xero.TenantID)
	}
	return Connection{Vendor: "xero", Detail: "organisation " + out.Organisations[0].Name}, nil
}

// readXero adds up what customers owe the merchant. An invoice is overdue when
// it is unpaid and past its due date as of the moment the world's history ends.
func (v Vendors) readXero(ctx context.Context, f world.Facts) (XeroSignals, error) {
	var s XeroSignals
	var out struct {
		Invoices []struct {
			Type      string  `json:"Type"`
			Status    string  `json:"Status"`
			Total     float64 `json:"Total"`
			AmountDue float64 `json:"AmountDue"`
			DueDate   string  `json:"DueDate"`
		} `json:"Invoices"`
	}
	if err := v.Xero.JSON(ctx, http.MethodGet, xeroBase+"/Invoices", xeroHeaders(f), nil, &out); err != nil {
		return s, fmt.Errorf("list the merchant's invoices: %w", err)
	}
	now := time.Unix(f.AsOfUnix, 0)
	for _, in := range out.Invoices {
		if in.Type != "ACCREC" || in.Status == "VOIDED" || in.Status == "DELETED" {
			continue
		}
		s.Invoices++
		s.BilledCents += cents(in.Total)
		due := cents(in.AmountDue)
		if due <= 0 {
			continue
		}
		s.OutstandingCents += due
		if d, err := parseXeroDate(in.DueDate); err == nil && d.Before(now) {
			s.OverdueCents += due
		}
	}
	return s, nil
}

func cents(dollars float64) int64 { return int64(dollars*100 + 0.5) }

var xeroDate = regexp.MustCompile(`^/Date\((-?\d+)(?:[+-]\d{4})?\)/$`)

// parseXeroDate reads Xero's /Date(milliseconds+0000)/ form, which is what the
// app emulator sends, and falls back to a plain ISO date or timestamp.
func parseXeroDate(s string) (time.Time, error) {
	if m := xeroDate.FindStringSubmatch(s); m != nil {
		ms, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.UnixMilli(ms).UTC(), nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised Xero date %q", s)
}
