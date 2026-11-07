package world

import (
	"context"
	"strings"
	"testing"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

func newFakePlaid(t *testing.T, accounts string) *fake {
	return newFake(t, map[string]string{
		"POST /sandbox/public_token/create": `{"public_token":"public-sandbox-1"}`,
		"POST /item/public_token/exchange":  `{"access_token":"access-sandbox-1","item_id":"item-1"}`,
		"POST /accounts/get":                accounts,
		"POST /transactions/get":            `{"total_transactions":2}`,
	})
}

const plaidAccounts = `{"accounts":[
 {"account_id":"acc-401k","subtype":"401k","mask":"6666","balances":{"current":23631.98}},
 {"account_id":"acc-chk","subtype":"checking","mask":"0000","balances":{"current":110}}]}`

func TestSeedPlaidConnectsTheCheckingAccount(t *testing.T) {
	f := newFakePlaid(t, plaidAccounts)
	facts, err := SeedPlaid(context.Background(), nil, wire.Direct("plaid", f.srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if facts.AccountID != "acc-chk" || facts.Mask != "0000" || facts.AccessToken != "access-sandbox-1" || facts.ItemID != "item-1" {
		t.Fatalf("unexpected facts: %+v", facts)
	}
	if facts.BalanceCents != 11000 || facts.Transactions != 2 {
		t.Fatalf("the thin fixture should show up in the facts: %+v", facts)
	}
	// The clock moves before the Item exists, so the fixture's dates land at the end of the history.
	order := strings.Join(f.paths(), ",")
	clock, item := strings.Index(order, "POST /admin/time/advance"), strings.Index(order, "POST /sandbox/public_token/create")
	if clock < 0 || item < 0 || clock > item {
		t.Fatalf("the clock must move before the Item is created: %s", order)
	}
}

func TestSeedPlaidReportsAnItemWithoutACheckingAccount(t *testing.T) {
	f := newFakePlaid(t, `{"accounts":[{"account_id":"a","subtype":"401k","mask":"1","balances":{"current":1}}]}`)
	_, err := SeedPlaid(context.Background(), nil, wire.Direct("plaid", f.srv.URL))
	if err == nil || !strings.Contains(err.Error(), "no checking account") {
		t.Fatalf("want a clear error: %v", err)
	}
}
