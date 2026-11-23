.PHONY: dev test race vet check build fmt plugins

dev:
	go tool mygo dev

fmt:
	gofmt -w main.go internal pkg cmd examples plugins

test:
	CGO_ENABLED=0 go test ./...

race:
	go test -race ./internal/... ./pkg/... ./plugins/...

vet:
	go vet ./...
	go tool mygo vet .

check: test vet

build:
	go tool mygo build -platform linux/amd64

plugins:
	./scripts/build-plugins.sh
