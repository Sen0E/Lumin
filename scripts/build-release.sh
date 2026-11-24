#!/usr/bin/env bash
# Release packages include the platform's self-contained default OCR plugin.
set -euo pipefail
cd "$(dirname "$0")/.."
target="${1:-linux/amd64}"
case "$target" in
  linux/amd64|windows/amd64) ;;
  *) echo "Bundled OCR is currently available for linux/amd64 and windows/amd64" >&2; exit 2 ;;
esac
python3 scripts/build-ppocr-plugin.py --target "$target"
resource_platform="${target/\//-}"
mkdir -p "resources/$resource_platform"
cp "build/plugins/lumin.ppocrv6-tiny-1.0.0-$resource_platform.luminext" "resources/$resource_platform/bundled-ocr.luminext"
if [[ "$target" == windows/amd64 ]]; then
  go tool mygo build -platform "$target" -skip-installer
else
  go tool mygo build -platform "$target"
fi
