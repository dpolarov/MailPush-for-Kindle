#!/bin/sh
set -eu

EXPECTED_VERSION="${1:-${VERSION:-}}"
if [ -z "$EXPECTED_VERSION" ]; then
  echo "Expected version is required" >&2
  exit 2
fi

ZIP="dist/mailpush.koplugin.zip"
TAR="dist/mailpush.koplugin-update.tar"
HOST="dist/mailpush-linux-amd64"

for f in "$ZIP" "$TAR" "$HOST"; do
  test -s "$f" || { echo "Missing build artifact: $f" >&2; exit 1; }
done

ACTUAL_VERSION="$($HOST version | python3 -c 'import json,sys; print(json.load(sys.stdin)["message"])')"
[ "$ACTUAL_VERSION" = "$EXPECTED_VERSION" ] || {
  echo "Embedded host version mismatch: expected $EXPECTED_VERSION, got $ACTUAL_VERSION" >&2
  exit 1
}

unzip -t "$ZIP" >/dev/null

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

tar -tf "$TAR" > "$tmp/list.txt"
tar -tvf "$TAR" > "$tmp/list-verbose.txt"

# Every member must stay under one plugin root. Reject traversal/absolute paths.
awk '
  /^\// { exit 1 }
  /(^|\/)\.\.($|\/)/ { exit 1 }
  !(/^mailpush\.koplugin\/?$/ || /^mailpush\.koplugin\//) { exit 1 }
' "$tmp/list.txt" || { echo "Unsafe or unexpected path in update archive" >&2; exit 1; }

# Reject links and special files. Regular files and directories only.
awk '{ k=substr($0,1,1); if (k != "-" && k != "d") exit 1 }' "$tmp/list-verbose.txt" || {
  echo "Unsupported file type in update archive" >&2
  exit 1
}

required='_meta.lua main.lua updater_download.lua config.default.json cacert.pem VERSION bin/mailpush'
for rel in $required; do
  grep -Fxq "mailpush.koplugin/$rel" "$tmp/list.txt" || {
    echo "Update archive is missing required file: $rel" >&2
    exit 1
  }
  unzip -Z1 "$ZIP" | grep -Fxq "mailpush.koplugin/$rel" || {
    echo "Install ZIP is missing required file: $rel" >&2
    exit 1
  }
done

mkdir "$tmp/unpacked"
tar -xf "$TAR" -C "$tmp/unpacked"
PLUGIN="$tmp/unpacked/mailpush.koplugin"

FILE_VERSION="$(tr -d '\r\n[:space:]' < "$PLUGIN/VERSION")"
[ "$FILE_VERSION" = "$EXPECTED_VERSION" ] || {
  echo "VERSION file mismatch: expected $EXPECTED_VERSION, got $FILE_VERSION" >&2
  exit 1
}

test -x "$PLUGIN/bin/mailpush" || { echo "Kindle backend is not executable" >&2; exit 1; }

# Simulate the updater's copy-over + rollback behavior without touching a real install.
mkdir -p "$tmp/live/bin" "$tmp/backup"
printf '%s\n' 'old-install-marker' > "$tmp/live/OLD_MARKER"
printf '%s\n' 'old-binary' > "$tmp/live/bin/mailpush"
cp -pfR "$tmp/live/." "$tmp/backup/"
cp -pfR "$PLUGIN/." "$tmp/live/"
[ "$(tr -d '\r\n[:space:]' < "$tmp/live/VERSION")" = "$EXPECTED_VERSION" ] || {
  echo "Simulated update did not install expected VERSION" >&2
  exit 1
}
test -f "$tmp/live/main.lua"

# Simulate rollback and verify the old install can be restored.
rm -rf "$tmp/live"
mkdir "$tmp/live"
cp -pfR "$tmp/backup/." "$tmp/live/"
test -f "$tmp/live/OLD_MARKER" || { echo "Rollback simulation failed" >&2; exit 1; }

printf 'Package verification passed for %s\n' "$EXPECTED_VERSION"
