#!/usr/bin/env python3
"""Ask Finder to store the UDZO window, then check that it was written.

macOS 27 drops a background whose alias points at a dotfile, and it also
drops an alias that was synthesized without Finder. The picture has to be a
normal file while Finder saves the window. It is marked hidden afterwards so
it does not appear as a third icon.
"""

import argparse
import json
import os
import subprocess
import sys
import time

import ds_store.store as ds_store
from ds_store import DSStore


class _RawBookmark:
    @staticmethod
    def decode(data):
        return data


# Finder's bookmark is not always the subset mac_alias can parse. The check
# only needs the icon-view plist and the icon positions.
ds_store.codecs[b"pBBk"] = _RawBookmark
ds_store.codecs[b"pBB0"] = _RawBookmark


def osa(script):
    result = subprocess.run(
        ["osascript", "-e", script],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        message = (result.stderr or result.stdout).strip()
        raise SystemExit(message or f"osascript failed ({result.returncode})")
    return result.stdout.strip()


def apple_string(value):
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'


def configure(mount, layout):
    disk = os.path.basename(mount)
    width = int(layout["width"])
    height = int(layout["height"])
    icon = int(layout["icon_size"])
    app_x = int(layout["app"]["x"])
    app_y = int(layout["app"]["y"])
    apps_x = int(layout["applications"]["x"])
    apps_y = int(layout["applications"]["y"])
    x, y = 200, 120
    # Finder saves this size. The origin is only a hint; Finder moves the
    # window onto the display.
    right = x + width
    bottom = y + height
    script = f"""
tell application "Finder"
  open disk {apple_string(disk)}
  delay 0.4
  set d to disk {apple_string(disk)}
  set w to container window of d
  set current view of w to icon view
  set toolbar visible of w to false
  set statusbar visible of w to false
  set sidebar width of w to 0
  set bounds of w to {{{x}, {y}, {right}, {bottom}}}
  set vo to icon view options of w
  set arrangement of vo to not arranged
  set icon size of vo to {icon}
  set background picture of vo to file "background.png" of d
  set position of item "Dogubako.app" of w to {{{app_x}, {app_y}}}
  set position of item "Applications" of w to {{{apps_x}, {apps_y}}}
  update d
  delay 1
  close w
  delay 0.8
end tell
"""
    osa(script)
    subprocess.run(["sync"], check=False)
    time.sleep(0.4)


def check(mount, layout):
    store = os.path.join(mount, ".DS_Store")
    want = {
        "Dogubako.app": (int(layout["app"]["x"]), int(layout["app"]["y"])),
        "Applications": (
            int(layout["applications"]["x"]),
            int(layout["applications"]["y"]),
        ),
    }
    with DSStore.open(store, "r") as ds:
        found = {}
        background = None
        icon_size = None
        for entry in ds:
            code = entry.code.decode() if isinstance(entry.code, bytes) else str(entry.code)
            if code == "Iloc":
                found[entry.filename] = tuple(entry.value)
            elif code == "icvp":
                background = entry.value.get("backgroundType")
                icon_size = entry.value.get("iconSize")
        if background != 2:
            raise SystemExit("Finder did not store the window background")
        if icon_size != int(layout["icon_size"]):
            raise SystemExit(f"Finder stored icon size {icon_size}")
        for name, pos in want.items():
            if found.get(name) != pos:
                raise SystemExit(f"Finder stored {name} at {found.get(name)}, want {pos}")


def hide(mount):
    path = os.path.join(mount, "background.png")
    # UF_HIDDEN. A dotfile is rejected as a background on macOS 27.
    flags = os.stat(path).st_flags | 0x8000
    os.chflags(path, flags)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--mount", required=True)
    parser.add_argument("--layout", required=True)
    args = parser.parse_args()
    with open(args.layout, encoding="utf-8") as handle:
        layout = json.load(handle)
    configure(args.mount, layout)
    try:
        check(args.mount, layout)
    except SystemExit:
        # Finder sometimes writes the window on the second close.
        configure(args.mount, layout)
        check(args.mount, layout)
    hide(args.mount)


if __name__ == "__main__":
    try:
        main()
    except BrokenPipeError:
        sys.exit(1)
