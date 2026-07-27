#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-latest}"

if [ "$VERSION" = "latest" ]; then
  VERSION=$(npm view katex version)
fi

echo "Downloading katex@${VERSION}..."

URL="https://registry.npmjs.org/katex/-/katex-${VERSION}.tgz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -sL "$URL" | tar -xz -C "$tmp"
cp "$tmp/package/dist/katex.min.js" katex.min.js
cp "$tmp/package/dist/katex.min.css" katex.min.css

echo "Updated katex.min.js and katex.min.css to version ${VERSION}."