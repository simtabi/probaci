# probaci -- Makefile. Run `make help`. This is the one definition of every
# task; Taskfile.yml forwards to it for anyone who prefers `task <name>`.
#
# The Go module lives in src/, so every go command names it with -C src. A bare
# `go test ./...` at the root finds no packages and passes.
#
# build/ holds one folder per OS and nothing else (build/macos, build/linux,
# build/windows, plus targets.txt, checksums.txt and, after `make dist`,
# archives/). The host build is build/<host-os>/probaci, macOS written as macos
# and .exe on Windows. `make check-build` (TestBuildLayout) fails on anything
# else.
#
# Nothing here publishes. A release builds on a pushed vX.Y.Z tag or on request (docs/release.md).

SHELL  := /usr/bin/env bash
.DEFAULT_GOAL := help

BINARY  := probaci
PKG     := github.com/simtabi/probaci/src/internal/version
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

HOST_GOOS := $(shell go env GOOS)
HOST_OS   := $(if $(filter darwin,$(HOST_GOOS)),macos,$(HOST_GOOS))
EXE       := $(if $(filter windows,$(HOST_GOOS)),.exe,)
BIN       := build/$(HOST_OS)/$(BINARY)$(EXE)

.PHONY: help build build-local install run test cover vet fmt lint tidy cross dist check-build \
        sync-docs release-check bundle snapshot clean

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: build-local ## Build $(BIN) (the same as build-local)

build-local: ## Build $(BIN) with the host's Go toolchain
	CGO_ENABLED=0 go -C src build -trimpath -ldflags "$(LDFLAGS)" -o ../$(BIN) ./cmd/$(BINARY)
	@$(MAKE) --no-print-directory check-build

install: ## Install probaci into GOBIN from this checkout (go -C src install)
	go -C src install -ldflags "$(LDFLAGS)" ./cmd/$(BINARY)

run: build-local ## Build, then run the binary (pass ARGS="...")
	./$(BIN) $(ARGS)

test: ## Run the tests with the race detector (includes the release, build-layout and no-container guards)
	go -C src test -race ./... -count=1

cover: ## Run the tests with coverage
	go -C src test -coverprofile=../coverage.txt ./...
	go -C src tool cover -func=../coverage.txt | tail -1

vet: ## go vet
	go -C src vet ./...

fmt: ## gofmt
	gofmt -w src

lint: ## golangci-lint (must be installed)
	cd src && golangci-lint run ./...

tidy: ## go mod tidy
	go -C src mod tidy

cross: ## Cross-compile every target in build/targets.txt into build/<os>/
	ARCHIVES=0 VERSION=$(VERSION) COMMIT=$(COMMIT) DATE=$(DATE) ./src/scripts/build-all.sh
	@$(MAKE) --no-print-directory check-build

dist: ## cross, plus .tar.gz/.zip archives in build/archives/ (publishes nothing)
	ARCHIVES=1 VERSION=$(VERSION) COMMIT=$(COMMIT) DATE=$(DATE) ./src/scripts/build-all.sh
	@$(MAKE) --no-print-directory check-build

check-build: ## Fail when build/ holds anything but the OS folders, inputs, checksums.txt and archives/
	go -C src test ./internal/release -run 'TestBuildLayout$$' -count=1

# The module in src/ cannot embed files above it, so `probaci docs` reads a copy
# of README.md and docs/ in src/embedded/. TestEmbeddedDocsMatchTheRepository
# fails when the copy drifts; this refreshes it.
sync-docs: ## Copy README.md and docs/ into src/embedded/ for `probaci docs`
	rm -rf src/embedded
	mkdir -p src/embedded/docs/tools
	cp README.md src/embedded/
	cp docs/*.md src/embedded/docs/
	cp docs/tools/*.md src/embedded/docs/tools/

release-check: ## Validate the GoReleaser configuration (a check only; builds nothing)
	goreleaser check --config src/.goreleaser.yaml

# A local GoReleaser snapshot, kept for checking the release config end to end.
# It writes into GoReleaser's scratch dist (.cache/goreleaser), never build/,
# and skips every publishing step.
bundle: ## Local GoReleaser snapshot into .cache/goreleaser (publishes nothing)
	goreleaser release --config src/.goreleaser.yaml --snapshot --clean --skip=publish,sign,sbom

snapshot: bundle ## Alias for bundle

clean: ## Remove generated output (build/ inputs are kept) and GoReleaser's scratch dist
	rm -rf bin .cache coverage.txt \
		build/macos build/linux build/windows build/freebsd build/openbsd build/netbsd \
		build/archives build/checksums.txt
