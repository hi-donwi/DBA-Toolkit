.PHONY: build test vet fmt lint cover integration clean install

BINARY := dbakit
VERSION ?= 0.1.0
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X github.com/hi-donwi/DBA-Toolkit/internal/cli.Version=$(VERSION) \
           -X github.com/hi-donwi/DBA-Toolkit/internal/cli.GitCommit=$(GIT_COMMIT) \
           -X github.com/hi-donwi/DBA-Toolkit/internal/cli.BuildDate=$(BUILD_DATE)

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/dbakit

install:
	go install ./cmd/dbakit

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Integration tests need a running PostgreSQL; see tests/integration/README.md.
integration:
	@set -eu; \
	trap 'docker compose -p dbakit-integration-replica -f tests/integration/docker-compose.replica.yml down; docker compose -p dbakit-integration -f tests/integration/docker-compose.yml down' EXIT; \
	docker compose -p dbakit-integration -f tests/integration/docker-compose.yml up -d --wait; \
	docker compose -p dbakit-integration-replica -f tests/integration/docker-compose.replica.yml up -d --wait; \
	DBAKIT_RUN_REPLICA=1 go test -tags integration -count=1 ./tests/integration/ -v

clean:
	rm -f $(BINARY) coverage.out coverage.html
