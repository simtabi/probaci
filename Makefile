# Thin wrapper over Taskfile.yml — the real task definitions live there
# (one source of truth). When go-task is installed, every target just delegates
# to `task <name>`; otherwise it falls back to the Go toolchain directly, so
# `make build` works with nothing but Go installed.

TASK := $(shell command -v task 2>/dev/null)
BIN  := bin/probaci
PKG  := ./cmd/probaci

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/simtabi/probaci/internal/version.Version=$(VERSION)

.PHONY: build install test cover vet lint fmt tidy bundle release-check snapshot clean help

ifeq ($(TASK),)
# --- fallback: invoke the Go toolchain directly ---
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)
install:
	go install $(PKG)
test:
	go test -race ./...
cover:
	go test -coverprofile=coverage.txt ./...
vet:
	go vet ./...
fmt:
	gofmt -w .
tidy:
	go mod tidy
lint:
	golangci-lint run
release-check:
	goreleaser check
bundle:
	goreleaser release --snapshot --clean --skip=publish,sign,docker,sbom
snapshot: bundle
clean:
	rm -rf bin dist coverage.txt
else
# --- delegate to Taskfile.yml (single source of truth) ---
build install test cover vet lint fmt tidy bundle release-check snapshot clean:
	@$(TASK) $@
endif

help:
	@echo "probaci make targets (delegate to Taskfile.yml when go-task is present):"
	@echo "  build install test cover vet lint fmt tidy bundle release-check snapshot clean"
