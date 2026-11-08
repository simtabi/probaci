# Thin wrapper over Taskfile.yml — the real task definitions live there
# (one source of truth). When go-task is installed, every target just delegates
# to `task <name>`; otherwise it falls back to the Go toolchain directly, so
# `make build` works with nothing but Go installed.

TASK := $(shell command -v task 2>/dev/null)
BIN  := bin/probaci
PKG  := ./cmd/probaci
# The module lives in src/: every go command names it with -C src, and a bare
# `go test ./...` at the root would find no packages and pass.

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/simtabi/probaci/src/internal/version.Version=$(VERSION)

.PHONY: build install test cover vet lint fmt tidy bundle release-check snapshot clean help sync-docs

ifeq ($(TASK),)
# --- fallback: invoke the Go toolchain directly ---
build:
	go -C src build -ldflags "$(LDFLAGS)" -o ../$(BIN) $(PKG)
install:
	go -C src install $(PKG)
test:
	go -C src test -race ./...
cover:
	go -C src test -coverprofile=../coverage.txt ./...
vet:
	go -C src vet ./...
fmt:
	gofmt -w src
tidy:
	go -C src mod tidy
lint:
	cd src && golangci-lint run ./...
release-check:
	goreleaser check --config src/.goreleaser.yaml
bundle:
	goreleaser release --config src/.goreleaser.yaml --snapshot --clean --skip=publish,sign,docker,sbom
snapshot: bundle
clean:
	rm -rf bin dist coverage.txt
else
# --- delegate to Taskfile.yml (single source of truth) ---
build install test cover vet lint fmt tidy bundle release-check snapshot clean:
	@$(TASK) $@
endif

# The module in src/ cannot embed files above it, so `probaci docs` reads a copy
# of README.md and docs/ in src/embedded/. TestEmbeddedDocsMatchTheRepository
# fails when the copy drifts; this refreshes it.
sync-docs:
	rm -rf src/embedded
	mkdir -p src/embedded/docs/tools
	cp README.md src/embedded/
	cp docs/*.md src/embedded/docs/
	cp docs/tools/*.md src/embedded/docs/tools/

help:
	@echo "probaci make targets (delegate to Taskfile.yml when go-task is present):"
	@echo "  build install test cover vet lint fmt tidy bundle release-check snapshot clean"
