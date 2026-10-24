#!/usr/bin/env bash
# Build Dogubako.app bundles and a Universal .dmg for macOS.
#
# On macOS the disk image is UDZO. Its Finder window is icon view, with
# Dogubako.app on the left, an Applications alias on the right, and an
# arrow drawn between them at package time. Other hosts get an ISO 9660
# image with the same two items and no arrow.
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

# Python is used to read .DS_Store back and refuse a disk image whose
# background Finder did not actually store.
ensure_style_python() {
	local venv="$DIST/.venv-dmg"
	local bin="$venv/bin/python"
	if [[ -x $bin ]]; then
		printf '%s\n' "$bin"
		return
	fi
	command -v python3 >/dev/null 2>&1 || die "python3 is required to style the macOS DMG"
	rm -rf "$venv"
	python3 -m venv "$venv"
	"$venv/bin/pip" install --disable-pip-version-check -r "$SCRIPT_DIR/stylevolume-requirements.txt" >&2
	[[ -x $bin ]] || die "python was not installed into $venv"
	printf '%s\n' "$bin"
}

# Prints the mount point, then the device, one per line.
attach_dmg() {
	local image=$1
	local plist
	plist=$(hdiutil attach -nobrowse -owners off -plist "$image")
	python3 -c 'import plistlib,sys
p=plistlib.loads(sys.stdin.buffer.read())
for e in p["system-entities"]:
    m=e.get("mount-point")
    if m:
        print(m)
        print(e.get("dev-entry",""))
        break
else:
    sys.exit(1)' <<<"$plist"
}

create_dmg_udzo() {
	local app_dir=$1
	local dest=$2
	local volname=$3
	local work mount py dev
	work=$(mktemp -d "${TMPDIR:-/tmp}/dogubako-dmg.XXXXXX")
	# shellcheck disable=SC2064
	trap "rm -rf '$work'" RETURN
	(cd "$REPO_ROOT" && go run ./packaging/macos/cmd/dmgwindow -o "$work/style")
	stage_dmg_root "$app_dir" "$work/root"
	hdiutil create \
		-volname "$volname" \
		-srcfolder "$work/root" \
		-ov -format UDRW \
		-fs HFS+ \
		"$work/rw.dmg" >/dev/null
	local attached
	attached=$(attach_dmg "$work/rw.dmg")
	mount=$(printf '%s\n' "$attached" | sed -n 1p)
	dev=$(printf '%s\n' "$attached" | sed -n 2p)
	cp "$work/style/background.png" "$mount/background.png"
	py=$(ensure_style_python)
	if ! "$py" "$SCRIPT_DIR/stylevolume.py" \
		--mount "$mount" \
		--layout "$work/style/layout.json"; then
		hdiutil detach "$mount" >/dev/null 2>&1 || hdiutil detach -force "$dev" >/dev/null 2>&1 || true
		exit 1
	fi
	hdiutil detach "$mount" >/dev/null || hdiutil detach -force "$dev" >/dev/null
	rm -f "$dest"
	hdiutil convert "$work/rw.dmg" -format UDZO -imagekey zlib-level=9 -o "$dest" >/dev/null
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
	if command -v hdiutil >/dev/null 2>&1; then
		create_dmg_udzo "$app_dir" "$dest" "道具箱"
	else
		local stage="$DIST/macos/${arch}/dmg-root"
		stage_dmg_root "$app_dir" "$stage"
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

cleanup_intermediates() {
	rm -f "$DIST/${BIN_NAME}-darwin-arm64" \
		"$DIST/${BIN_NAME}-darwin-amd64" \
		"$DIST/${BIN_NAME}-darwin-universal"
	rm -rf "$DIST/macos"
}

case $CMD in
app)
	make_apps
	;;
dmg)
	make_apps
	make_dmgs
	cleanup_intermediates
	;;
all)
	make_apps
	make_dmgs
	cleanup_intermediates
	;;
*)
	die "unknown command: $CMD (expected app, dmg, or all)"
	;;
esac
