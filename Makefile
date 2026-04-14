# Origami — Makefile (thin wrapper around justfile)
# Prefer `just` for full recipes. This exists so `make lint` works everywhere.

.PHONY: build install test test-fast test-accept fmt lint lint-new vet circuit check preflight cover clean install-hooks serve-image serve

build:
	go build ./...

install:
	go install ./cmd/origami ./cmd/mediator ./cmd/agent-worker

test:
	go test ./... -count=1 -race -timeout 120s

test-fast:
	go test ./... -short -count=1 -timeout 60s

test-accept:
	go test ./testkit/acceptance/ -race -v -count=1 -timeout 120s

fmt:
	goimports -w .

lint:
	golangci-lint run ./...

lint-new:
	golangci-lint run --new-from-rev=HEAD ./...

vet:
	go vet ./...

circuit: fmt vet build lint test test-accept
	@echo "Circuit complete — all gates passed"

check: build lint test-fast

preflight: fmt vet lint test install
	@echo "Preflight complete"

cover:
	go test ./... -coverprofile=coverage.out -timeout 120s
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1

install-hooks:
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make lint-new' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "pre-commit hook installed (runs make lint-new)"

serve-image:
	podman build -f Dockerfile.serve -t origami-serve:latest .

serve:
	podman run --rm -p 9100:9100 \
		-e SDLC_REPO_PATH=/workspace \
		-e SDLC_MODE=real \
		-e SDLC_PROVIDER=$${SDLC_PROVIDER:-anthropic} \
		-e SDLC_MODEL=$${SDLC_MODEL:-claude-sonnet-4-6} \
		-v $$(pwd):/workspace:Z \
		origami-serve:latest

clean:
	rm -rf bin/ coverage.out coverage.html
