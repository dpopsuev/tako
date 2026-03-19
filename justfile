# Origami — task runner
# Run `just` with no args to see available recipes.

set dotenv-load := false

bin_dir := "bin"
cmd     := "./cmd/origami"

# ─── Default ──────────────────────────────────────────────

# List available recipes
default:
    @just --list

# ─── Build ────────────────────────────────────────────────

# Build the origami CLI
build:
    @mkdir -p {{ bin_dir }}
    go build -o {{ bin_dir }}/origami {{ cmd }}

# Install origami to ~/.local/bin
install:
    go build -o ~/.local/bin/origami {{ cmd }}

# ─── Test ─────────────────────────────────────────────────

# Run all Go tests
test:
    go test ./...

# Run all Go tests with race detector
test-race:
    go test -race ./...

# Run all Go tests with verbose output
test-v:
    go test -v ./...

# ─── Lint ─────────────────────────────────────────────────

# Run go vet
vet:
    go vet ./...

# Run origami lint on all testdata YAMLs (strict profile)
lint-pipelines:
    @for f in testdata/*.yaml testdata/**/*.yaml; do echo "lint: $f"; origami lint --profile strict "$f"; done

# ─── Container Images ─────────────────────────────────────

workspace := justfile_directory() / ".."

# Build all OCI images (mediator + rca + gnd from workspace root context)
build-images: build-mediator build-rca build-gnd

# Build mediator image (origami-only context)
build-mediator:
    docker build -t origami-mediator -f deploy/Dockerfile.mediator .

# Build RCA engine image (workspace root context for sibling repos)
build-rca:
    docker build -t origami-rca -f {{ workspace }}/rh-rca/Dockerfile {{ workspace }}

# Build GND engine image (workspace root context for sibling repos)
build-gnd:
    docker build -t origami-gnd -f {{ workspace }}/rh-gnd/Dockerfile {{ workspace }}

# ─── Clean ────────────────────────────────────────────────

# Remove build artifacts
clean:
    rm -rf {{ bin_dir }}
