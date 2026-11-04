// Package world defines the fictional business ACME Commerce Lenders makes
// offers to, and seeds it into the Stripe, Plaid and Xero app emulators.
//
// The world is the same every run. Its sales come from a fixed pattern, so an
// offer built on it can be traced and compared between runs.
package world

// MerchantName is the ecommerce company that applies for working capital.
const MerchantName = "Harbor & Pine Outfitters"

// Days is how much sales history the world has.
const Days = 90

// Order is one card sale.
type Order struct {
	Day   int   // 0 is the first day of history
	Cents int64 // amount in US cents
}

// weekday is the demand on each day of a repeating week, as a multiple of a
// base day. It is indexed by day number, not by calendar date, so the world
// does not depend on which day it is seeded.
var weekday = [7]float64{1.00, 1.00, 1.00, 1.05, 1.15, 1.35, 1.25}

const (
	baseOrdersPerDay = 8.0
	dailyGrowth      = 0.004 // sales grow 0.4% a day, so the history has a trend
)

// splitmix is a small fixed random sequence. It is written out here, not taken
// from math/rand, so the world cannot change with a Go release.
type splitmix struct{ s uint64 }

func (r *splitmix) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *splitmix) intn(n int) int { return int(r.next() % uint64(n)) }

// Orders returns the merchant's sales, in order. It returns the same orders on
// every call.
func Orders() []Order {
	r := &splitmix{s: 20260804}
	cents := [4]int64{0, 49, 95, 99}
	var out []Order
	for day := 0; day < Days; day++ {
		trend := 1 + dailyGrowth*float64(day)
		n := int(baseOrdersPerDay*weekday[day%7]*trend+0.5) + r.intn(3) - 1
		for i := 0; i < n; i++ {
			dollars := int64(28 + r.intn(118))
			out = append(out, Order{Day: day, Cents: dollars*100 + cents[r.intn(4)]})
		}
	}
	return out
}

// Gross is the total of the orders, in cents.
func Gross(orders []Order) int64 {
	var sum int64
	for _, o := range orders {
		sum += o.Cents
	}
	return sum
}

// Facts is what the lender learns about the merchant when it connects the
// merchant's accounts. Seeding writes it, and the lender reads it.
type Facts struct {
	Merchant string      `json:"merchant"`
	AsOf     string      `json:"as_of"`      // the date the history ends, YYYY-MM-DD
	AsOfUnix int64       `json:"as_of_unix"` // the same moment in Unix seconds, for "the last 30 days"
	Stripe   StripeFacts `json:"stripe"`
	Plaid    PlaidFacts  `json:"plaid"`
	Xero     XeroFacts   `json:"xero"`
}

// StoreKey is where Facts live in the lender's Store.
const StoreKey = "world/facts"
