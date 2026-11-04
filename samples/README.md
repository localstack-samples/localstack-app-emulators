# Samples

A sample is an open source app, written by someone else for real users, that is run against the app emulators. It has the use cases and edge cases real customers hit, and its own tests, which we did not invent. The demos in `apps/` are different: they are staged to show well, and they are ours.

This directory records which app was run and how. It never holds the app's code. The code stays in its own repo under its own license, and a run fetches the commit named here.

## What a sample holds

```
samples/<name>/
  sample.json   which app, at which commit, and how it is run
  README.md     optional notes
  results/      the output of runs (JSON)
```

Nothing else. `make check-samples` fails on any other file.

## sample.json

```json
{
  "name": "example-shop",
  "repo": "https://github.com/example/shop",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "license": "MIT",
  "vendors": ["stripe"],
  "overrides": { "stripe": "STRIPE_API_BASE" },
  "run": "npm test"
}
```

This is an illustration, not a real sample.

| Field | Meaning |
|---|---|
| `name` | The directory name. Lower-case words joined by hyphens. |
| `repo` | The https URL of the app's repo. |
| `commit` | The full 40-character SHA that was run. A branch or tag is rejected, because a sample must be reproducible. |
| `license` | The app's SPDX identifier, such as `MIT` or `Apache-2.0`. |
| `vendors` | The vendors whose app emulators it is run against. |
| `overrides` | For each vendor, the environment variable or setting that holds its base URL. |
| `run` | The command that runs the app or its tests, from the app's checkout. |

## Checking

```sh
make check-samples
```

## Recording a run

Two commands in `shared/` record what an app asks of the app emulators and report what they do not cover.

**1. Record.** `wire-proxy` listens on one port per vendor and forwards to that vendor's app emulator, the one at `<VENDOR>_EMULATOR_URL` if set, otherwise the one LocalStack serves. Point the app's vendor base URL at the listening address, using the `overrides` in `sample.json`:

```sh
cd shared
STRIPE_EMULATOR_VERSION=<release id> go run ./cmd/wire-proxy \
  --proxy stripe=127.0.0.1:5301 --proxy plaid=127.0.0.1:5302 \
  --events 127.0.0.1:5399 --out ../samples/<name>/results/<date>-calls.json
```

Run the app or its tests, then stop the proxy with Ctrl-C. It writes every call to `--out`, with credentials in bodies masked. While it runs, `curl http://127.0.0.1:5399/events` shows the calls so far. Each call records its `origin` and, if you set `<VENDOR>_EMULATOR_VERSION`, the app emulator's release id. An app emulator does not announce its release, so the id is the one in the release's tag, `twin-<name>-v<id>`.

**2. Report.** `gap-report` checks each call against the app emulators' manifests, which are `twin-<name>/twin-manifest.json` in `WonderTwin-AI/wondertwin`:

```sh
cd shared
go run ./cmd/gap-report --events ../samples/<name>/results/<date>-calls.json \
  --manifest stripe=<path>/twin-stripe/twin-manifest.json \
  --manifest plaid=<path>/twin-plaid/twin-manifest.json
```

It lists each distinct call, with identifiers in the path replaced by `{}`, as `covered`, `gap` or `unchecked`, and which app emulator release it ran against. `--json` prints the same as JSON. `--fail-on-gap` exits 1 if any call is a gap.

### What the report can and cannot tell you

- **It reads the manifest, not the response.** Stripe's app emulator answers a route it lacks with a 404 that copies the real Stripe, so a status cannot say whether a route is missing.
- **The manifests do not share a format, so precision differs.** Xero lists routes (`PUT /api.xro/2.0/Accounts`), so a covered call is exact. Plaid lists paths (`item/get`), so it cannot tell methods apart. Stripe lists resource names (`payment_intents`), so `covered` means only that some of that resource is implemented. The `BASIS` column says which applies.
- **Identifiers are recognised by their shape.** A segment with a digit or a capital that looks like an id (`acct_3Bs...`, a GUID, a long token) becomes `{}`. An unusual identifier could be reported as its own call.
- **A vendor with no manifest is `unchecked`, never a gap.**
