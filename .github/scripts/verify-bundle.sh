#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

archive="${1:-tool/data/otelc-bundle.tgz}"

if [[ ! -f "$archive" ]]; then
    echo "archive not found: $archive"
    exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

tar -xzf "$archive" -C "$tmpdir"

# Mapping of source directories to their names inside the archive.
declare -A dirs=(
    [pkg]=pkg_temp
    [instrumentation]=instrumentation_temp
)

# Verify the archive contains only the expected top-level directories.
while IFS= read -r -d '' path; do
    name="${path#$tmpdir/}"

    case "$name" in
        pkg_temp|instrumentation_temp)
            ;;
        *)
            echo "unexpected top-level entry in archive: $name"
            status=1
            ;;
    esac
done < <(find "$tmpdir" -mindepth 1 -maxdepth 1 -print0)

status=0

check_tree() {
    local root="$1"

    # Reject any filesystem objects other than regular files and directories.
    while IFS= read -r -d '' path; do
        if [[ -L "$path" ]]; then
            echo "symlinks are not allowed: $path"
            status=1
            continue
        fi

        if [[ -f "$path" || -d "$path" ]]; then
            continue
        fi

        echo "unsupported file type: $path"
        status=1
    done < <(find "$root" -print0)
}

compare_tree() {
    local src="$1"
    local dst="$2"

    # Verify directory structure and file contents match.
    if ! diff --recursive --brief "$src" "$dst"; then
        status=1
    fi

    # Git only preserves the executable bit for regular files.
    while IFS= read -r -d '' file; do
        rel="${file#$src/}"
        other="$dst/$rel"

        if [[ -x "$file" && ! -x "$other" ]] || [[ ! -x "$file" && -x "$other" ]]; then
            echo "executable bit mismatch: $rel"
            status=1
        fi
    done < <(find "$src" -type f -print0)
}

for src in "${!dirs[@]}"; do
    dst="${dirs[$src]}"

    if [[ ! -d "$src" ]]; then
        echo "missing source directory: $src"
        exit 1
    fi

    if [[ ! -d "$tmpdir/$dst" ]]; then
        echo "missing archive directory: $dst"
        exit 1
    fi

    # Ensure both trees only contain supported filesystem objects.
    check_tree "$src"
    check_tree "$tmpdir/$dst"

    # Verify the directory trees are identical.
    compare_tree "$src" "$tmpdir/$dst"
done

if [[ "$status" -eq 0 ]]; then
    echo "archive verified successfully"
else
    echo "archive verification failed"
fi

exit "$status"
