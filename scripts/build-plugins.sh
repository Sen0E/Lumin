#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
python_cmd="${PYTHON:-python3}"
if [[ $# -eq 1 && "$1" == ppocrv6-tiny ]]; then
  exec "$python_cmd" scripts/build-ppocr-plugin.py --target "${GOOS:-$(go env GOOS)}/${GOARCH:-$(go env GOARCH)}"
fi
plugin_os="${GOOS:-$(go env GOOS)}"
plugin_arch="${GOARCH:-$(go env GOARCH)}"
plugin_stage="$(mktemp -d)"
trap 'rm -rf "$plugin_stage"' EXIT
mkdir -p build/plugins
# The packer executes on the build host even when plugins target another OS.
env -u GOOS -u GOARCH CGO_ENABLED=0 go build -o "$plugin_stage/pack" ./cmd/lumin-plugin-pack
sources=(examples/plugins/libretranslate plugins/caiyun)
if [[ $# -gt 1 ]]; then echo "Usage: $0 [libretranslate|caiyun|ppocrv6-tiny]" >&2; exit 2; fi
matched=false
for source in "${sources[@]}"; do
  name="${source##*/}"
  if [[ $# -eq 1 && "$1" != "$name" ]]; then continue; fi
  matched=true
  mkdir -p "$plugin_stage/$name/bin"
  entry="bin/plugin"
  if [[ "$plugin_os" == windows ]]; then entry="bin/plugin.exe"; fi
  CGO_ENABLED=0 GOOS="$plugin_os" GOARCH="$plugin_arch" go build -buildvcs=false -trimpath -ldflags='-s -w' -o "$plugin_stage/$name/$entry" "./$source"
  # Generate platform metadata from the checked-in manifest.
  metadata="$("$python_cmd" - "$source/plugin.json" "$plugin_stage/$name/plugin.json" "$plugin_os/$plugin_arch" "$entry" <<'PY'
import json,sys
with open(sys.argv[1],encoding='utf-8') as f: manifest=json.load(f)
manifest['entrypoints']={sys.argv[3]:sys.argv[4]}
with open(sys.argv[2],'w',encoding='utf-8') as f: json.dump(manifest,f,ensure_ascii=False,indent=2)
print(manifest['id'],manifest['version'])
PY
)"
  read -r plugin_id plugin_version <<< "$metadata"
  if [[ "$name" == caiyun ]]; then
    # Ship GPL notices and complete standalone source with every binary package.
    cp "$source/LICENSE" "$source/NOTICE" "$source/README.md" "$plugin_stage/$name/"
    mkdir -p "$plugin_stage/$name/src/plugins/caiyun" "$plugin_stage/$name/src/pkg/pluginapi"
    cp "$source/"*.go "$plugin_stage/$name/src/plugins/caiyun/"
    cp pkg/pluginapi/*.go "$plugin_stage/$name/src/pkg/pluginapi/"
    printf 'module lumin\n\ngo %s\n' "$(go list -m -f '{{.GoVersion}}')" > "$plugin_stage/$name/src/go.mod"
  fi
  package="$plugin_id-$plugin_version-$plugin_os-$plugin_arch.luminext"
  "$plugin_stage/pack" -dir "$plugin_stage/$name" -out "$plugin_stage/$package"
  mv -f "$plugin_stage/$package" "build/plugins/$package"
  echo "Built build/plugins/$package"
done
if [[ "$matched" == false ]]; then echo "Unknown plugin: $1" >&2; exit 2; fi
