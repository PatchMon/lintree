VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u)
LDFLAGS := -ldflags "-X 'lintree/internal/version.Version=$(VERSION)' -X 'lintree/internal/version.Commit=$(COMMIT)' -X 'lintree/internal/version.Date=$(DATE)'"

.PHONY: build install test clean lint run

build:
	go build $(LDFLAGS) -o lintree .

install:
	go install $(LDFLAGS) .

test:
	go test ./...

clean:
	rm -f lintree
	rm -rf dist/

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install it: https://golangci-lint.run/welcome/install/"; \
		echo "Falling back to go vet..."; \
		go vet ./...; \
	fi

run: build
	./lintree $(ARGS)
