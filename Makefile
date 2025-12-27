VERSION ?= $(shell cat VERSION 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo "unknown")

LDFLAGS = -ldflags="\
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT)' \
	-X 'main.date=$(DATE)'"

build:
	go build $(LDFLAGS) -o ./bin/urso ./cmd/urso

.PHONY: test
test:
	go test -v -race -count=1 -timeout=30s ./...

tidy:
	go mod tidy -v
	go fmt ./...

vet:
	go vet ./...

.PHONY: lint
lint:
	golangci-lint run --fix
