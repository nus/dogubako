.PHONY: test vet build build-macos build-ubuntu app-macos dmg-macos package-macos appimage-ubuntu package-ubuntu

GO ?= go
VERSION ?= 0.1.0
export VERSION

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

# Dogubako.app for Apple Silicon, Intel, and Universal (any host).
app-macos: build-macos
	./packaging/macos/package.sh app

# Universal .dmg next to the Universal .app. On macOS this uses hdiutil (UDZO).
# Elsewhere an ISO 9660 image that DiskImageMounter can open is written.
dmg-macos: build-macos
	./packaging/macos/package.sh dmg

package-macos: dmg-macos

# AppImage for amd64 and arm64 (any Linux host with squashfs-tools).
appimage-ubuntu: build-ubuntu
	./packaging/linux/package.sh appimage

package-ubuntu: appimage-ubuntu
