REPO_DIR    := $(shell basename $(CURDIR))
PLUGIN_NAME := $(shell command ls cmd/)
GO_FILES    := $(shell find . -name '*.go' -not -path "./vendor/*" | grep -v _test.go)

.PHONY: default
default: dev

.PHONY: dev
dev:
	CGO_ENABLED=0 go build -o bin/$(PLUGIN_NAME) cmd/$(PLUGIN_NAME)/main.go

.PHONY: build
build:
	go build -o clickhouse-database-plugin -ldflags="-s -w" ./cmd/vault-plugin-database-clickhouse/main.go

.PHONY: test
test: fmtcheck
	CGO_ENABLED=0 go test -v ./... $(TESTARGS) -timeout=20m

.PHONY: testacc
testacc: fmtcheck
	CGO_ENABLED=0 VAULT_ACC=1 go test -v ./... $(TESTARGS) -timeout=20m

.PHONY: fmtcheck
fmtcheck:
	gofmt -d -e -s $(GO_FILES)

.PHONY: fmt
fmt:
	gofumpt -l -w .