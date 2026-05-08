.PHONY: build install test test-fast test-e2e fmt lint lint-new vet check preflight cover clean

BIN := bin/tako

build:
	go build -o $(BIN) ./cmd/tako/

install:
	go install ./cmd/tako/

test:
	go test ./... -count=1 -race -timeout 120s

test-fast:
	go test ./agent/cerebrum/ ./assemble/ ./tui/ -count=1 -race -timeout 60s

test-e2e:
	TAKO_PROVIDER=$${TAKO_PROVIDER} go test ./tests/e2e/ -v -count=1 -race -timeout 300s

fmt:
	goimports -w .

lint:
	golangci-lint run ./...

lint-new:
	golangci-lint run --new-from-rev=HEAD ./...

vet:
	go vet ./...

check: build vet test-fast

preflight: fmt vet lint test
	@echo "Preflight complete"

cover:
	go test ./... -coverprofile=coverage.out -timeout 120s
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1

clean:
	rm -rf bin/ coverage.out coverage.html

run: build
	./$(BIN) agent $(ARGS)

tui: build
	./$(BIN) tui $(ARGS)
