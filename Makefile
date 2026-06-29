BINARY := guardrail
PKG := ./cmd/guardrail

.PHONY: fmt test build run ci

fmt:
	gofmt -w .

test:
	go test -race -shuffle=on -count=1 ./...

build:
	go build -trimpath -ldflags="-s -w" -o bin/$(BINARY) $(PKG)

run: build
	./bin/$(BINARY) -config configs/guardrail.yaml

ci: fmt test build
