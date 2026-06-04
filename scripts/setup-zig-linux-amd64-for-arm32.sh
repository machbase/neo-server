#!/usr/bin/env bash
set -euo pipefail

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

install_dir="${RUNNER_TEMP:-/tmp}/zig"
rm -rf "$install_dir"
mkdir -p "$install_dir"

index_json="$workdir/index.json"

curl -fsSL https://ziglang.org/download/index.json -o "$index_json"

zig_url="$(python3 - "$index_json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding='utf-8') as fh:
    versions = json.load(fh)

release_versions = [version for version in versions if version != 'master' and version.count('.') == 2 and '-dev' not in version]
if not release_versions:
    raise SystemExit('No Zig release versions found in index.json')

latest = max(release_versions, key=lambda version: tuple(int(part) for part in version.split('.')))
assets = versions[latest]

for key in ('x86_64-linux', 'x86_64-linux-musl'):
    if key in assets:
        print(assets[key]['tarball'])
        break
else:
    raise SystemExit(f'No Linux x86_64 tarball found for Zig {latest}')
PY
)"

curl -fsSL "$zig_url" -o "$workdir/zig.tar.xz"
tar -xJf "$workdir/zig.tar.xz" -C "$workdir"

zig_dir="$(find "$workdir" -maxdepth 1 -type d -name 'zig-*' | head -n 1)"
if [[ -z "$zig_dir" ]]; then
  echo "Failed to locate extracted Zig directory" >&2
  exit 1
fi

mv "$zig_dir" "$install_dir/zig"

echo "$install_dir/zig" >> "$GITHUB_PATH"
