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
	docker compose -f tests/integration/docker-compose.yml up -d --wait
	go test -tags integration ./tests/integration/ -v
	docker compose -f tests/integration/docker-compose.yml down

clean:
	rm -f $(BINARY) coverage.out coverage.html