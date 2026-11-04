# Usage Guide

Status: In Development

## Getting started

The demo runs beside LocalStack. LocalStack hosts the app emulators it includes and serves them through its gateway. You run any others yourself, and the demo calls all of them the same way. No AWS services are used.

### Before you start

- A Docker-compatible container runtime, such as Docker, OrbStack, Colima or Rancher Desktop.
- `lstk`, installed with `brew install localstack/tap/lstk` or `npm install -g @localstack/lstk`.
- A LocalStack account. `lstk` guides you through signing in the first time.
- LocalStack hosts the Stripe app emulator. For any app emulator your LocalStack release does not include yet, such as Plaid or Xero, you run it yourself. An older release answers 404 for an app emulator it does not include.
- Go, to build and run the demo from this repo.

### Start LocalStack

```sh
lstk
```

`lstk` handles authentication, configuration and the container, then starts LocalStack.

### Get the app emulators running

Each app emulator is either hosted by LocalStack or run by you, and you can mix them. Stripe can come from LocalStack while Plaid and Xero run on your machine.

**Hosted by LocalStack.** An app emulator installs the first time it is called, so call each one once while you are online. A healthy one answers `{"status":"ok"}`:

```sh
for app in stripe plaid xero; do
  curl -s -H "Host: $app.localhost.localstack.cloud:4566" http://127.0.0.1:4566/admin/health
done
```

The `Host` header sends the request to the right app emulator without a DNS lookup. `*.localhost.localstack.cloud` is a public DNS name that points at `127.0.0.1`, so without the header the same call needs the internet.

**Run by you.** Use this for an app emulator your LocalStack release does not include yet. Build it from [`WonderTwin-AI/wondertwin`](https://github.com/WonderTwin-AI/wondertwin) and start it on a free port:

```sh
git clone https://github.com/WonderTwin-AI/wondertwin && cd wondertwin
go build -o bin/twin-plaid ./twin-plaid/cmd/twin-plaid
go build -o bin/twin-xero ./twin-xero/cmd/twin-xero
./bin/twin-plaid --port=5101 &
./bin/twin-xero --port=5102 &
```

Then tell the demo where each one is:

```sh
export PLAID_EMULATOR_URL=http://127.0.0.1:5101
export XERO_EMULATOR_URL=http://127.0.0.1:5102
```

A vendor with no `<VENDOR>_EMULATOR_URL` set uses LocalStack. When your LocalStack includes Plaid and Xero, unset those two variables and the demo uses LocalStack for them, with no other change.

### Check the app emulators

```sh
make kit APP=acme-commerce-lenders ARGS=ready
```

This lists each app emulator, who serves it (`localstack` or `local`) and whether it is healthy. If one is not, it says what to do: start LocalStack, run that app emulator yourself, or set the variable that points at it.

### Seed the world

```sh
make kit APP=acme-commerce-lenders ARGS=seed
```

This builds the fictional merchant the lender makes an offer to, Harbor & Pine Outfitters, in all three app emulators. It takes a second or two.

| App emulator | What the merchant has |
|---|---|
| Stripe | A connected account with 90 days of card sales (941 orders, $81,083.84) and 12 weekly payouts |
| Xero | Six wholesale invoices: three paid, two inside their terms, and one overdue. $16,150.00 is outstanding and $5,400.00 of it is overdue. |
| Plaid | A bank connection with a checking account |

The world is the same every run. If an app emulator is down, or LocalStack does not include it, nothing is seeded. If the world is already seeded, the command says so and asks for a reset.

Three things to know:

- **The app emulators' clocks move 90 days ahead.** Stripe cannot back-date a sale, so seeding builds the history by moving the clock forward a day at a time. The history ends on the "as of" date the command prints. A reset puts the clocks back to real time.
- **Plaid's data is thin.** The Plaid app emulator gives every bank connection the same small fixture: a few accounts, two transactions, no deposits, and a checking balance of $110.00. It does not match the Stripe payouts. Until the Plaid app emulator can be given a merchant's own history, the demo's bank data is that fixture.
- **Funded payouts.** The Stripe app emulator does not add a connected account's sales to its balance, so seeding funds the account with each week's sales just before paying them out.

### Run the demo

```sh
make kit APP=acme-commerce-lenders
```

Open <http://127.0.0.1:8080>. The UI is not built yet, so this serves a placeholder page, and the demo does not read the `<VENDOR>_EMULATOR_URL` variables yet.

## What you see

The UI is not built yet. The flow runs behind an HTTP API that the UI will call, and you can drive it yourself once the server is up (`make kit APP=acme-commerce-lenders`, on port 8080):

```sh
curl -s -X POST localhost:8080/api/reset                          # seed the world
curl -s -X POST localhost:8080/api/applications                   # the merchant applies
curl -s -X POST localhost:8080/api/applications/app-0001/connect  # reach Stripe, Plaid and Xero
curl -s -X POST localhost:8080/api/applications/app-0001/review   # read what they hold
curl -s -X POST localhost:8080/api/applications/app-0001/offer    # price the offer
```

For the seeded merchant the offer is an advance of **$30,000** for a flat 8% fee ($2,400), so $32,400 to repay. The advance is the last 30 days of card sales ($30,423.72) rounded down to $500. Each number in the offer names the vendor call it came from.

| Route | What it does |
|---|---|
| `GET /api/status` | Whether each app emulator is healthy and the world is seeded |
| `POST /api/reset` | Wipes the app emulators and seeds the world again |
| `POST /api/applications` | Starts an application |
| `POST /api/applications/{id}/connect`, `review`, `offer` | The steps, which must happen in that order |
| `GET /api/applications/{id}` | The application so far |
| `GET /api/events?since=N` | Every vendor call made so far, newest after `N`, with request and response bodies. Credentials are masked. |

Plaid's balance is shown with the offer but does not change it, because the Plaid app emulator's data is too thin to price against.

## Reset

```sh
make kit APP=acme-commerce-lenders ARGS=reset
```

This wipes the app emulators and the lender's own state, then seeds the world again, so the demo is back at its starting point. It first prints each app emulator it is about to wipe and where it is served from.

An app emulator served by LocalStack is shared with anything else using that LocalStack, so a reset wipes everything in it, not only this demo's data.

## Working without internet

Not verified yet. The plan is to install every app emulator while online, then run the demo with the network off. LocalStack itself needs to have activated recently, since it falls back to offline activation that has to be renewed every 24 hours.
