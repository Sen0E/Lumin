#!/usr/bin/env bash
# Build the application with both self-contained plugins for its exact target.
set -euo pipefail
cd "$(dirname "$0")/.."
target="${1:-linux/amd64}"
python_cmd="${PYTHON:-python3}"
case "$target" in
  linux/amd64|windows/amd64|darwin/arm64) ;;
  *) echo "Unsupported release target: $target" >&2; exit 2 ;;
esac
"$python_cmd" scripts/build-ppocr-plugin.py --target "$target"
target_os="${target%/*}"
target_arch="${target#*/}"
GOOS="$target_os" GOARCH="$target_arch" PYTHON="$python_cmd" ./scripts/build-plugins.sh caiyun
resource_platform="${target/\//-}"
mkdir -p "resources/$resource_platform"
# Bundle both maintained plugins, reading versions from their manifests.
for plugin in ppocrv6-tiny caiyun; do
  ref="$("$python_cmd" scripts/release.py plugin-ref --plugin "$plugin")"
  bundle="bundled-$plugin.luminext"
  if [[ "$plugin" == ppocrv6-tiny ]]; then bundle="bundled-ocr.luminext"; fi
  cp "build/plugins/$ref-$resource_platform.luminext" "resources/$resource_platform/$bundle"
done
# Forward optional MyGo flags, e.g. -skip-installer for local cross builds.
# Native Windows CI builds the NSIS installer; native macOS CI builds the DMG.
shift "$(( $# > 0 ? 1 : 0 ))"
go tool mygo build -platform "$target" "$@"
