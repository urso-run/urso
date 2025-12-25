.PHONY: test
test:
	go test -cpu 24 -race -count=1 -timeout=30s ./...

tidy:
	go mod tidy -v
	go fmt ./...

vet:
	go vet ./...

.PHONY: lint
lint: bin/golangci-lint
	golangci-lint run --fix
