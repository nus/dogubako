#!/usr/bin/env bash
# Build Ubuntu / Linux AppImages for amd64 and arm64.
#
# Usage:
#   packaging/linux/package.sh [appdir|appimage|all]
#
# Environment:
#   VERSION                    package version (default: 0.1.0)
#   DIST                       output directory (default: dist)
#   APPIMAGE_RUNTIME_x86_64    local type-2 runtime ELF (skip download)
#   APPIMAGE_RUNTIME_aarch64   local type-2 runtime ELF (skip download)
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
CMD=${1:-all}
VERSION=${VERSION:-0.1.0}
DIST=${DIST:-"$REPO_ROOT/dist"}
APP_NAME=Dogubako
BIN_NAME=dogubako
ARCHES=(amd64 arm64)
RUNTIME_RELEASE_URL=${APPIMAGE_RUNTIME_BASE_URL:-https://github.com/AppImage/type2-runtime/releases/download/continuous}

log() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

runtime_arch() {
	case $1 in
	amd64) printf 'x86_64\n' ;;
	arm64) printf 'aarch64\n' ;;
	*) die "unsupported arch: $1" ;;
	esac
}

need_bin() {
	local arch=$1
	local path="$DIST/${BIN_NAME}-linux-${arch}"
	[[ -f $path ]] || die "missing $path (run: make build-ubuntu)"
	printf '%s\n' "$path"
}

generate_icons() {
	mkdir -p "$DIST/linux"
	local size dest
	for size in 256 512; do
		dest="$DIST/linux/dogubako-${size}.png"
		(cd "$REPO_ROOT" && go run ./packaging/cmd/genicon -o "$dest" -size "$size")
	done
}

write_desktop() {
	local dest=$1
	sed -e 's/^Icon=.*/Icon=dogubako/' "$REPO_ROOT/packaging/dogubako.desktop" >"$dest"
	printf 'X-AppImage-Version=%s\n' "$VERSION" >>"$dest"
}

assemble_appdir() {
	local arch=$1
	local binary=$2
	local appdir=$3

	rm -rf "$appdir"
	mkdir -p "$appdir/usr/bin" \
		"$appdir/usr/share/applications" \
		"$appdir/usr/share/icons/hicolor/256x256/apps" \
		"$appdir/usr/share/icons/hicolor/512x512/apps"

	cp "$binary" "$appdir/usr/bin/$BIN_NAME"
	chmod 755 "$appdir/usr/bin/$BIN_NAME"
	cp "$SCRIPT_DIR/AppRun" "$appdir/AppRun"
	chmod 755 "$appdir/AppRun"

	write_desktop "$appdir/${BIN_NAME}.desktop"
	cp "$appdir/${BIN_NAME}.desktop" "$appdir/usr/share/applications/${BIN_NAME}.desktop"

	cp "$DIST/linux/dogubako-256.png" "$appdir/${BIN_NAME}.png"
	cp "$DIST/linux/dogubako-256.png" "$appdir/usr/share/icons/hicolor/256x256/apps/${BIN_NAME}.png"
	cp "$DIST/linux/dogubako-512.png" "$appdir/usr/share/icons/hicolor/512x512/apps/${BIN_NAME}.png"
	ln -s "${BIN_NAME}.png" "$appdir/.DirIcon"

	log "wrote $appdir"
}

make_appdirs() {
	generate_icons
	local arch binary
	for arch in "${ARCHES[@]}"; do
		binary=$(need_bin "$arch")
		assemble_appdir "$arch" "$binary" "$DIST/linux/${arch}/${APP_NAME}.AppDir"
	done
}

fetch_runtime() {
	local rarch=$1
	local dest="$DIST/linux/runtime-${rarch}"
	local override=""
	case $rarch in
	x86_64) override=${APPIMAGE_RUNTIME_x86_64:-} ;;
	aarch64) override=${APPIMAGE_RUNTIME_aarch64:-} ;;
	esac
	if [[ -n $override ]]; then
		[[ -f $override ]] || die "runtime override not found: $override"
		printf '%s\n' "$override"
		return
	fi
	if [[ ! -s $dest ]]; then
		mkdir -p "$(dirname "$dest")"
		local url="${RUNTIME_RELEASE_URL}/runtime-${rarch}"
		log "downloading AppImage runtime ${rarch}"
		curl -fL --retry 4 --retry-all-errors --retry-delay 2 -o "$dest" "$url" \
			|| die "failed to download $url"
	fi
	printf '%s\n' "$dest"
}

make_one_appimage() {
	local arch=$1
	local appdir="$DIST/linux/${arch}/${APP_NAME}.AppDir"
	[[ -d $appdir ]] || die "missing $appdir (run: $0 appdir)"

	local rarch dest runtime
	rarch=$(runtime_arch "$arch")
	dest="$DIST/${APP_NAME}-${VERSION}-linux-${arch}.AppImage"
	runtime=$(fetch_runtime "$rarch")

	(cd "$REPO_ROOT" && go run ./packaging/linux/cmd/makeappimage \
		-appdir "$appdir" \
		-runtime "$runtime" \
		-out "$dest")
	chmod 755 "$dest"
	log "wrote $dest"
}

make_appimages() {
	command -v mksquashfs >/dev/null 2>&1 || die "mksquashfs not found (install squashfs-tools)"
	command -v curl >/dev/null 2>&1 || die "curl not found"
	local arch
	for arch in "${ARCHES[@]}"; do
		make_one_appimage "$arch"
	done
}

case $CMD in
appdir)
	make_appdirs
	;;
appimage)
	make_appdirs
	make_appimages
	;;
all)
	make_appdirs
	make_appimages
	;;
*)
	die "unknown command: $CMD (expected appdir, appimage, or all)"
	;;
esac
