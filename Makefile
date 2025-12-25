build:
	go build -o ./bin/urso ./cmd/urso

.PHONY: test
test:
	go test -race -count=1 -timeout=30s ./...

tidy:
	go mod tidy -v
	go fmt ./...

vet:
	go vet ./...

.PHONY: lint
lint:
	golangci-lint run --fix
