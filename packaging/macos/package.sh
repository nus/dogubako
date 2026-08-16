#!/usr/bin/env bash
# Build Dogubako.app bundles and a Universal .dmg for macOS.
#
# Usage:
#   packaging/macos/package.sh [app|dmg|all]
#
# Environment:
#   VERSION                 bundle version (default: 0.1.0)
#   DIST                    output directory (default: dist)
#   MACOS_SIGN_IDENTITY     codesign identity; "-" for ad-hoc (macOS only)
#
# Per-arch .app bundles are built as intermediates. The distribution package
# is the Universal .dmg only.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
CMD=${1:-all}
VERSION=${VERSION:-0.1.0}
DIST=${DIST:-"$REPO_ROOT/dist"}
APP_NAME=Dogubako
BIN_NAME=dogubako
ARCHES=(arm64 amd64)

log() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need_bin() {
	local arch=$1
	local path="$DIST/${BIN_NAME}-darwin-${arch}"
	[[ -f $path ]] || die "missing $path (run: make build-macos)"
	printf '%s\n' "$path"
}

generate_icon() {
	local dest=$1
	(cd "$REPO_ROOT" && go run ./packaging/cmd/genicon -o "$dest")
}

assemble_app() {
	local arch=$1
	local binary=$2
	local app_dir=$3
	local icns=$4

	rm -rf "$app_dir"
	local contents="$app_dir/Contents"
	mkdir -p "$contents/MacOS" \
		"$contents/Resources/ja.lproj" \
		"$contents/Resources/en.lproj"

	cp "$binary" "$contents/MacOS/$BIN_NAME"
	chmod 755 "$contents/MacOS/$BIN_NAME"
	sed "s/__VERSION__/${VERSION}/g" "$SCRIPT_DIR/Info.plist" >"$contents/Info.plist"
	printf 'APPLdgub' >"$contents/PkgInfo"
	cp "$icns" "$contents/Resources/dogubako.icns"
	cp "$SCRIPT_DIR/ja.lproj/InfoPlist.strings" "$contents/Resources/ja.lproj/"
	cp "$SCRIPT_DIR/en.lproj/InfoPlist.strings" "$contents/Resources/en.lproj/"

	if command -v codesign >/dev/null 2>&1; then
		local identity=${MACOS_SIGN_IDENTITY:--}
		codesign --force --deep --sign "$identity" \
			--entitlements "$SCRIPT_DIR/dogubako.entitlements" \
			"$app_dir" || log "warning: codesign failed; continuing unsigned"
	fi

	log "wrote $app_dir"
}

make_apps() {
	mkdir -p "$DIST/macos"
	local icns="$DIST/macos/dogubako.icns"
	generate_icon "$icns"

	local arch binary
	for arch in "${ARCHES[@]}"; do
		binary=$(need_bin "$arch")
		assemble_app "$arch" "$binary" "$DIST/macos/${arch}/${APP_NAME}.app" "$icns"
	done

	local uni="$DIST/${BIN_NAME}-darwin-universal"
	(cd "$REPO_ROOT" && go run ./packaging/macos/cmd/lipo \
		-o "$uni" \
		"$(need_bin arm64)" \
		"$(need_bin amd64)")
	assemble_app universal "$uni" "$DIST/macos/universal/${APP_NAME}.app" "$icns"
}

stage_dmg_root() {
	local app_dir=$1
	local stage=$2
	rm -rf "$stage"
	mkdir -p "$stage"
	cp -R "$app_dir" "$stage/${APP_NAME}.app"
	ln -s /Applications "$stage/Applications"
}

create_dmg_hdiutil() {
	local stage=$1
	local dest=$2
	local volname=$3
	local tmp="${dest}.rw.dmg"
	rm -f "$tmp" "$dest"
	hdiutil create \
		-volname "$volname" \
		-srcfolder "$stage" \
		-ov -format UDRW \
		-fs HFS+ \
		"$tmp"
	hdiutil convert "$tmp" -format UDZO -imagekey zlib-level=9 -o "$dest"
	rm -f "$tmp"
}

create_dmg_iso() {
	local stage=$1
	local dest=$2
	local volname=$3
	(cd "$REPO_ROOT" && go run ./packaging/macos/cmd/makedmg \
		-src "$stage" -out "$dest" -volname "$volname")
}

make_one_dmg() {
	local arch=$1
	local app_dir="$DIST/macos/${arch}/${APP_NAME}.app"
	[[ -d $app_dir ]] || die "missing $app_dir (run: $0 app)"

	local dest="$DIST/${APP_NAME}-${VERSION}-macos-${arch}.dmg"
	local stage="$DIST/macos/${arch}/dmg-root"
	stage_dmg_root "$app_dir" "$stage"

	if command -v hdiutil >/dev/null 2>&1; then
		create_dmg_hdiutil "$stage" "$dest" "道具箱"
	else
		create_dmg_iso "$stage" "$dest" "道具箱"
		log "note: created ISO 9660 .dmg (use a Mac and hdiutil for a native UDZO image)"
	fi
	log "wrote $dest"
}

make_dmgs() {
	# Distribution package is universal only (covers Apple Silicon and Intel).
	[[ -d $DIST/macos/universal/${APP_NAME}.app ]] || die "missing universal app (run: $0 app)"
	make_one_dmg universal
}

case $CMD in
app)
	make_apps
	;;
dmg)
	make_apps
	make_dmgs
	;;
all)
	make_apps
	make_dmgs
	;;
*)
	die "unknown command: $CMD (expected app, dmg, or all)"
	;;
esac
