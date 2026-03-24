# Origami — task runner
# Run `just` with no args to see available recipes.
# Flywheel: `just check` runs build → lint → test before every commit.

set dotenv-load := false

bin_dir := "bin"
cmd     := "./cmd/origami"

# ─── Default ──────────────────────────────────────────────

# List available recipes
default:
    @just --list

# ─── Flywheel (micro circuit: build → lint → test) ───────

# Full quality gate — run before every commit
check: build lint test

# ─── Build ────────────────────────────────────────────────

# Build all Go packages
build:
    go build ./...

# Build all CLIs into bin/
build-cli:
    @mkdir -p {{ bin_dir }}
    go build -o {{ bin_dir }}/origami ./cmd/origami
    go build -o {{ bin_dir }}/mediator ./cmd/mediator
    go build -o {{ bin_dir }}/llm-worker ./cmd/llm-worker

# Install origami to $GOPATH/bin
install:
    go install ./cmd/origami
    go install ./cmd/mediator
    go install ./cmd/llm-worker

# ─── Lint ─────────────────────────────────────────────────

# Run golangci-lint (Go code lint)
lint:
    golangci-lint run ./...

# Run go vet only (fast)
vet:
    go vet ./...

# Run origami lint on all testdata YAMLs (circuit YAML lint)
lint-pipelines:
    @for f in testdata/*.yaml testdata/**/*.yaml; do echo "lint: $f"; origami lint --profile strict "$f"; done

# ─── Test ─────────────────────────────────────────────────

# Run all Go tests with race detector
test:
    go test ./... -count=1 -race -timeout 120s

# Run tests without race detector (faster)
test-fast:
    go test ./... -short -count=1 -timeout 60s

# Run tests with verbose output
test-v:
    go test -v ./... -count=1 -timeout 120s

# Run tests with coverage
cover:
    go test ./... -coverprofile=coverage.out -timeout 120s
    go tool cover -func=coverage.out | tail -1
    @rm coverage.out

# ─── Container Images ─────────────────────────────────────

workspace := justfile_directory() / ".."

# Build all OCI images
build-images: build-mediator build-rca

# Build mediator image
build-mediator:
    docker build --no-cache -t origami-mediator -f deploy/Dockerfile.mediator .

# Build RCA engine image (from rh-rca context)
build-rca:
    docker build --no-cache -t origami-rca -f deploy/Dockerfile.rca {{ workspace }}/rh-rca

# ─── Deploy ───────────────────────────────────────────────

# Start compose stack
up:
    cd deploy && docker compose up -d

# Stop compose stack
down:
    cd deploy && docker compose down

# Rebuild images and restart stack
deploy: build-images up

# ─── Clean ────────────────────────────────────────────────

# Remove build artifacts
clean:
    rm -rf {{ bin_dir }}
