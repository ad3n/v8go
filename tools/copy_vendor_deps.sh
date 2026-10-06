#!/bin/sh
set -eu
src=$(cd "$(dirname "$0")/.." && pwd)
dest=${1:?usage: copy_vendor_deps.sh VENDORED_V8GO_DIRECTORY}
[ -f "$dest/cgo.go" ] || { echo "v8go must be vendored first" >&2; exit 1; }
for dir in "$src"/deps/*_*/; do
    [ -f "$dir/libmanifest" ] || continue
    target="$dest/deps/$(basename "$dir")"
    mkdir -p "$target"
    cp "$dir/libmanifest" "$target/"
    for archive in "$dir"/*.a "$dir"/*.lib; do
        [ -f "$archive" ] || continue
        cp "$archive" "$target/"
    done
done
