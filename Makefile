.PHONY: test vet build build-macos build-ubuntu

GO ?= go

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build -o dist/dogubako ./cmd/dogubako

# Builds without CGO so binaries can be cross-compiled.
build-macos:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -o dist/dogubako-darwin-arm64 ./cmd/dogubako
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o dist/dogubako-darwin-amd64 ./cmd/dogubako

build-ubuntu:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o dist/dogubako-linux-amd64 ./cmd/dogubako
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -o dist/dogubako-linux-arm64 ./cmd/dogubako
