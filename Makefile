BINARY := muninndb-lite

.PHONY: build test vet clean

## build: compile the muninndb-lite binary.
build:
	go build -o $(BINARY) ./cmd/muninn/

## test: run all tests.
test:
	go test -timeout 300s ./cmd/muninn/... ./internal/...

## vet: run go vet on all packages.
vet:
	go vet ./cmd/muninn/... ./internal/...

## clean: remove build artifacts.
clean:
	rm -f $(BINARY)
