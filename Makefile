SHELL := /bin/bash
.DEFAULT_GOAL := help

GO ?= go
COVERAGE_MIN ?= 95
COVER_DIR := cover
PKGS := ./...
# Unit tests cover both modules. Coverage includes library packages exercised
# by server scenarios; individual specialized targets select their relevant packages.
SERVER_DIR := server
LIB_MOD := github.com/deploymenttheory/go-microsoft-dm
SRV_MOD := github.com/deploymenttheory/go-microsoft-dm/server
ALL_PKGS := $(LIB_MOD)/...,$(SRV_MOD)/...
INTEGRATION_PKGS := ./sqlstore/... ./adminauth/... ./audit/... ./internal/app/...
E2E_PKGS := ./e2e/...
E2E_STORE ?= sqlite
FUZZ_SMOKE_TIME ?= 20s
FUZZ_TIME ?= 10m
DDF_DIR := third_party/ddf
# Prefer the golangci-lint that make tools built with this module's Go version;
# a distribution binary built with an older Go refuses a newer go directive.
GOPATH_BIN := $(shell $(GO) env GOPATH)/bin
GOLANGCI_LINT ?= $(if $(wildcard $(GOPATH_BIN)/golangci-lint),$(GOPATH_BIN)/golangci-lint,golangci-lint)

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | column -t -s ':'

## tools: install developer tools, built with the Go version go.mod declares so they can load this module (jq is also needed: brew install jq)
GO_VERSION := $(shell sed -n 's/^go \(.*\)$$/\1/p' go.mod)
tools:
	GOTOOLCHAIN=go$(GO_VERSION) $(GO) install gotest.tools/gotestsum@latest
	GOTOOLCHAIN=go$(GO_VERSION) $(GO) install mvdan.cc/gofumpt@latest
	GOTOOLCHAIN=go$(GO_VERSION) $(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	GOTOOLCHAIN=go$(GO_VERSION) $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

## generate: regenerate schema/csp, schema/policy and schema/registry from the pinned DDF bundle in third_party/ddf
generate:
	$(GO) run ./cmd/ddfgen -ddf $(DDF_DIR) -out schema generate

## verify: fail if the pinned DDF bundle does not match its manifest, if regeneration changes anything, or if a locked identifier disappeared
verify:
	$(GO) run ./cmd/ddfgen -ddf $(DDF_DIR) -out schema verify

## ddf-diff: list node changes between two drops (make ddf-diff OLD=third_party/ddf/a.zip NEW=third_party/ddf/b.zip)
ddf-diff:
	$(GO) run ./cmd/ddfgen diff "$(OLD)" "$(NEW)"

## lint: run golangci-lint with the repository configuration in both modules
lint:
	$(GOLANGCI_LINT) run --config=.golangci.yml ./...
	cd $(SERVER_DIR) && $(GOLANGCI_LINT) run --config=../.golangci.yml ./...

## test: unit tests with race detector, coverage written to cover/unit
test:
	@rm -rf $(COVER_DIR)/unit && mkdir -p $(COVER_DIR)/unit
	$(GO) test -race -shuffle=on -count=1 -cover -coverpkg=$(LIB_MOD)/... $(PKGS) -args -test.gocoverdir=$(PWD)/$(COVER_DIR)/unit
	cd $(SERVER_DIR) && $(GO) test -race -shuffle=on -count=1 -cover -coverpkg=$(ALL_PKGS) ./... -args -test.gocoverdir=$(PWD)/$(COVER_DIR)/unit

## testdb-up: start local PostgreSQL and MySQL in Docker and print the DSNs to export
testdb-up:
	@docker run -d --rm --name dm-postgres -e POSTGRES_USER=dm -e POSTGRES_PASSWORD=dm -e POSTGRES_DB=dm -p 5432:5432 postgres:16 >/dev/null
	@docker run -d --rm --name dm-mysql -e MYSQL_ROOT_PASSWORD=dm -e MYSQL_DATABASE=dm -p 3306:3306 mysql:8 >/dev/null
	@echo 'export TEST_POSTGRES_DSN="postgres://dm:dm@localhost:5432/dm?sslmode=disable"'
	@echo 'export TEST_MYSQL_DSN="root:dm@tcp(localhost:3306)/dm?parseTime=true&multiStatements=true"'
	@echo '# wait a few seconds for the databases to accept connections, then: make test-storage'

## testdb-down: stop the local test databases
testdb-down:
	@docker rm -f dm-postgres dm-mysql >/dev/null 2>&1 || true

## test-storage: storage contract suites against SQL backends (Phase 6; needs TEST_POSTGRES_DSN / TEST_MYSQL_DSN)
test-storage:
	@rm -rf $(COVER_DIR)/storage && mkdir -p $(COVER_DIR)/storage
	@cd $(SERVER_DIR) && if $(GO) list -tags integration $(INTEGRATION_PKGS) >/dev/null 2>&1 && ls sqlstore/integration_test.go >/dev/null 2>&1; then \
		$(GO) test -race -count=1 -tags integration -cover -coverpkg=$(ALL_PKGS) $(INTEGRATION_PKGS) -args -test.gocoverdir=$(PWD)/$(COVER_DIR)/storage; \
	else echo "no storage suites yet"; fi

## test-conformance: generated schema conformance tests only (Phase 3)
test-conformance:
	@if ls schema/*/*_test.go >/dev/null 2>&1; then \
		$(GO) test -count=1 -run 'Conformance' ./schema/...; \
	else echo "no schema conformance tests yet"; fi

## test-e2e: reference server plus simulator scenarios on E2E_STORE (Phase 6; sqlite, postgres, inmem)
test-e2e: export E2E_STORE := $(E2E_STORE)
test-e2e:
	@rm -rf $(COVER_DIR)/e2e-$(E2E_STORE) && mkdir -p $(COVER_DIR)/e2e-$(E2E_STORE)
	@cd $(SERVER_DIR) && if ls e2e/*_test.go >/dev/null 2>&1; then \
		$(GO) test -race -count=1 -tags e2e -cover -coverpkg=$(ALL_PKGS) $(E2E_PKGS) -args -test.gocoverdir=$(PWD)/$(COVER_DIR)/e2e-$(E2E_STORE); \
	else echo "no e2e scenarios yet"; fi

## test-conformance-guest: enroll a real Windows guest through guestweave (Phase 7; needs WEAVE_URL and WEAVE_TOKEN, skipped otherwise)
test-conformance-guest:
	@if [ -z "$$WEAVE_URL" ] || [ -z "$$WEAVE_TOKEN" ]; then echo "skipped: WEAVE_URL and WEAVE_TOKEN are not set"; \
	elif ! ls $(SERVER_DIR)/e2e/guest/*_test.go >/dev/null 2>&1; then echo "no guest harness yet"; \
	else cd $(SERVER_DIR) && $(GO) test -count=1 -tags guest -v ./e2e/guest/...; fi

## fuzz-smoke: run every fuzz target briefly
fuzz-smoke:
	@scripts/fuzz.sh $(FUZZ_SMOKE_TIME)

## fuzz: run every fuzz target for FUZZ_TIME each
fuzz:
	@scripts/fuzz.sh $(FUZZ_TIME)

## coverage: merge profiles and enforce COVERAGE_MIN per package and overall
coverage:
	@COVERAGE_MIN=$(COVERAGE_MIN) scripts/coverage-gate.sh $(COVER_DIR)

## vuln: govulncheck in both modules
vuln:
	govulncheck ./...
	cd $(SERVER_DIR) && govulncheck ./...

## refs: clone reference implementations read-only into third_party/refs
refs:
	@scripts/refs.sh

## refs-activity: list reference repos pushed in the last 30 days
refs-activity:
	@scripts/refs-activity.sh

## specs: download the pinned specifications into third_party/specs and verify them against MANIFEST.json (SPECS_RECORD=1 records new hashes)
specs:
	@scripts/specs.sh

## ddf: download a DDF drop by URL into third_party/ddf and print its manifest entry (DDF_URL=...)
ddf:
	@scripts/ddf.sh "$(DDF_URL)"

## ci: everything CI runs, in order
ci: lint verify test fuzz-smoke coverage

## clean: remove coverage output
clean:
	rm -rf $(COVER_DIR)

.PHONY: help tools generate verify ddf-diff lint test testdb-up testdb-down test-storage test-conformance test-e2e test-conformance-guest fuzz-smoke fuzz coverage vuln refs refs-activity specs ddf ci clean
