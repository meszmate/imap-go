.PHONY: all build test test-race vet lint clean fmt

all: build test vet

build:
	go build ./...

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

fmt:
	gofmt -s -w .

lint:
	golangci-lint run ./...

fuzz:
	go test -fuzz=Fuzz -fuzztime=30s ./wire/

clean:
	rm -f coverage.out coverage.html
