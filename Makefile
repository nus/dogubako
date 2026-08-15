.PHONY: test vet build build-macos

GO ?= go

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build -o dist/dogubako ./cmd/dogubako

# First supported target. Builds without CGO so the binary can be
# cross-compiled from Linux or macOS.
build-macos:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -o dist/dogubako-darwin-arm64 ./cmd/dogubako
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o dist/dogubako-darwin-amd64 ./cmd/dogubako
