.PHONY: dev test race vet check build fmt

dev:
	go tool mygo dev

fmt:
	gofmt -w main.go internal

test:
	CGO_ENABLED=0 go test ./...

race:
	go test -race ./internal/...

vet:
	go vet ./...
	go tool mygo vet .

check: test vet

build:
	go tool mygo build -platform linux/amd64
