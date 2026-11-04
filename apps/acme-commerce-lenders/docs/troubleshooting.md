# Troubleshooting Guide

Status: Ready to rehearse

Check first: `make kit APP=acme-commerce-lenders ARGS=ready`. It lists each app emulator, who serves it and whether it is healthy, and says what to do about one that is not. Each row below is a failure seen while building or rehearsing the demo.

| Symptom | Cause | Fix |
|---|---|---|
| `plaid  localstack  not-included` (or `xero`), and "this LocalStack release does not include plaid" | The LocalStack release does not include that app emulator, so its gateway answers 404. | Run the app emulator yourself and set `PLAID_EMULATOR_URL` or `XERO_EMULATOR_URL`. See [Get the app emulators running](usage.md#get-the-app-emulators-running). |
| An app emulator served by LocalStack shows `unreachable`: "LocalStack is not answering on its gateway" | LocalStack is not running. | Start it with `lstk`. |
| An app emulator you run shows `unreachable`: "nothing is listening at http://127.0.0.1:5101" | The process stopped, or the port is not the one in the variable. | Start it again on that port, or unset the variable to use LocalStack. |
| **Apply for working capital** is disabled | An app emulator is not ready, or the world is not seeded. The page says which. | Fix the app emulator, then press **Reset**. |
| `the world is not seeded; run seed or reset first` | The demo started before the world was built, or a reset failed partway. | Press **Reset**, or run `make kit APP=acme-commerce-lenders ARGS=seed`. |
| `the world is already seeded; reset it first (run: reset)` | `seed` was run twice. It never adds to a world, so the second run stops. | Run `make kit APP=acme-commerce-lenders ARGS=reset`. |
| Settling fails with `xero PUT /api.xro/2.0/Accounts: HTTP 405: Method Not Allowed` | The Xero app emulator is older than the change that lets it create accounts and bank transactions. The earlier steps work, so funding succeeds and only the booking fails. | Rebuild `twin-xero` from `WonderTwin-AI/wondertwin` `main`, restart it and press **Reset**. |
| A step failed halfway, with an error and a **Retry** button | A vendor call failed, for example an app emulator stopped during funding. | Fix the cause and press **Retry**. It carries on from that step. Funding and settling are safe to run again, and cannot pay or book twice. |
| `listen tcp 127.0.0.1:8080: bind: address already in use` | Something else is on port 8080, often an earlier run of the demo. | Stop it, or serve elsewhere: `make kit APP=acme-commerce-lenders ARGS="serve --addr 127.0.0.1:8081"`. |
| The page says "Scaffold only. The web UI has not been built yet." | The binary was built without the UI. | `make web APP=acme-commerce-lenders`, then `make kit APP=acme-commerce-lenders`. |
| The inspector says "Cannot read the calls" | The demo binary stopped. | Start it again. The inspector reconnects by itself. |
| The inspector is empty after the demo has run | A reset clears it, or the demo was restarted. | Run the demo again. Seed calls are never shown. |
| Dates in Stripe, Xero or Plaid are 90 days ahead of today | Stripe cannot back-date a sale, so seeding moves the app emulators' clocks forward a day at a time to build the history. | **Reset** puts the clocks back to real time. |
| Plaid shows a $110.00 balance and two transactions that match nothing in Stripe | The Plaid app emulator gives every bank connection the same small fixture. | Expected. The balance is shown with the offer but does not change it. |
| Reset wiped data you wanted in an app emulator served by LocalStack | LocalStack's app emulators are shared with anything else using that LocalStack. | Run that app emulator yourself for the demo, or keep other work off the same LocalStack. |
