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

### Run the demo

```sh
make kit APP=acme-commerce-lenders
```

Open <http://127.0.0.1:8080>. The UI is not built yet, so this serves a placeholder page, and the demo does not read the `<VENDOR>_EMULATOR_URL` variables yet.

## What you see

Not built yet. The planned flow is two clicks: apply, then accept.

## Reset

Not built yet. The plan is one click that returns every app emulator and the demo to the seeded starting state.

## Working without internet

Not verified yet. The plan is to install every app emulator while online, then run the demo with the network off. LocalStack itself needs to have activated recently, since it falls back to offline activation that has to be renewed every 24 hours.
