#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VCPKG_ROOT="${VCPKG_ROOT:-$ROOT_DIR/tools/vcpkg}"

if [[ ! -d "$VCPKG_ROOT/.git" ]]; then
  echo "Cloning vcpkg into $VCPKG_ROOT..."
  git clone https://github.com/microsoft/vcpkg "$VCPKG_ROOT"
fi

echo "Bootstrapping vcpkg..."
"$VCPKG_ROOT/bootstrap-vcpkg.sh"

UNAME_S="$(uname -s)"
UNAME_M="$(uname -m)"
if [[ "$UNAME_S" == "Darwin" && "$UNAME_M" == "arm64" ]]; then
  export VCPKG_DEFAULT_TRIPLET="arm64-osx"
fi

echo "Installing dependencies via vcpkg (overlay ports enabled)..."
"$VCPKG_ROOT/vcpkg" install \
  --overlay-ports="$ROOT_DIR/server/ports" \
  --x-manifest-root="$ROOT_DIR/server"

cat <<EOF
Done.

Build:
  VCPKG_ROOT="$VCPKG_ROOT" make -C server all
EOF
