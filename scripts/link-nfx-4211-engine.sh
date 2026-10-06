#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
nfx_dir="${1:-${MACHBASEDEV_HOME:-$HOME/work/nfx}}"
branch="$(git -C "$nfx_dir" branch --show-current)"
if [[ "$branch" != "4211-vector-column" ]]; then
    printf 'Expected nfx branch 4211-vector-column, got %s\n' "$branch" >&2
    exit 1
fi
header="$nfx_dir/mm/src/include/machEngine.h"
archive="$nfx_dir/machbase_home/lib/libmachengine.a"
if [[ ! -f "$header" || ! -f "$archive" ]]; then
    printf 'Build the Standard #4211 nfx engine first: %s\n' "$nfx_dir" >&2
    exit 1
fi
cp "$header" "$repo_dir/spi/mach/native/machEngine.h"
ln -sfn "$archive" "$repo_dir/spi/mach/native/libmachengine_standard_linux_amd64.a"
printf 'nfx_commit=%s\n' "$(git -C "$nfx_dir" rev-parse HEAD)"
sha256sum "$archive" "$repo_dir/spi/mach/native/machEngine.h"
