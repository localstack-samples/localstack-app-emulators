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
