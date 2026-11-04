.PHONY: build test vet check-kits check-samples kit web

# Every module in the Go workspace is built, vetted and tested. In a workspace,
# ./... from the root matches only the root module, so each module is listed.

# Builds every entry point under apps/<demo>/cmd/<name> to bin/<demo>-<name>.
build:
	mkdir -p bin
	set -e; for d in apps/*/cmd/*/; do \
		cmd=$$(basename "$$d"); app=$$(basename "$$(dirname "$$(dirname "$$d")")"); \
		go build -o "bin/$$app-$$cmd" "./$$d"; \
	done

vet:
	set -e; for m in $$(go list -m); do go vet "$$m/..."; done

test:
	set -e; for m in $$(go list -m); do go test -count=1 "$$m/..."; done

# Every demo ships an offline kit.
check-kits:
	set -e; for a in apps/*/; do [ -d "$${a}cmd/kit" ] || { echo "$$a has no cmd/kit (every demo needs an offline kit)"; exit 1; }; done

# Every sample app's descriptor is well formed, and no sample holds app code.
check-samples:
	cd shared && go run ./cmd/check-samples ../samples

# Builds a demo's web UI and copies it into the Go binary's embedded assets.
# Run it before `make build` to get the real UI instead of the placeholder.
# make web APP=acme-commerce-lenders
web:
	pnpm --dir web install --frozen-lockfile
	pnpm --dir web --filter @localstack-app-emulators/$(APP) build
	find apps/$(APP)/internal/ui/assets -mindepth 1 ! -name placeholder.html -delete
	cp -R web/apps/$(APP)/dist/. apps/$(APP)/internal/ui/assets/

# make kit APP=acme-commerce-lenders ARGS=seed
kit: build
	./bin/$(APP)-kit $(ARGS)
