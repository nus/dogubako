.PHONY: test vet build build-macos build-ubuntu app-macos dmg-macos package-macos appimage-ubuntu package-ubuntu memstat memstat-nocjk

GO ?= go
VERSION ?= 0.1.0
export VERSION

test:
	$(GO) test ./...

# Measure heap / RSS after init and after a few Guigui frames.
# Needs a display (or Xvfb). Compare CJK fonts with: make memstat-nocjk
MEMSTATFLAGS ?=
memstat:
	$(GO) run ./cmd/memstat $(MEMSTATFLAGS)

memstat-nocjk:
	$(GO) run -tags nocjk ./cmd/memstat $(MEMSTATFLAGS)

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

# Dogubako.app for Apple Silicon, Intel, and Universal (any host).
app-macos: build-macos
	./packaging/macos/package.sh app

# Universal .dmg (intermediates cleaned after packaging).
# On macOS this uses hdiutil (UDZO); elsewhere an ISO 9660 image is written.
dmg-macos: build-macos
	./packaging/macos/package.sh dmg

package-macos: dmg-macos

# AppImage for amd64 and arm64 (intermediates cleaned after packaging).
appimage-ubuntu: build-ubuntu
	./packaging/linux/package.sh appimage

package-ubuntu: appimage-ubuntu
