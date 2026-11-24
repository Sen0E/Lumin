#!/usr/bin/env python3
"""Build pinned, self-contained .luminext packages. Python is build-time only."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / 'plugins/ppocrv6-tiny'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def protobuf_fields(data):
    pos = 0

    def varint():
        nonlocal pos
        result = shift = 0
        while pos < len(data) and shift < 64:
            value = data[pos]
            pos += 1
            result |= (value & 127) << shift
            if value < 128:
                return result
            shift += 7
        raise ValueError('invalid protobuf varint')

    while pos < len(data):
        tag = varint()
        field, wire = tag >> 3, tag & 7
        if wire == 0:
            value = varint()
        elif wire in (1, 2, 5):
            size = varint() if wire == 2 else (8 if wire == 1 else 4)
            if size > len(data) - pos:
                raise ValueError('truncated protobuf')
            value = data[pos:pos + size]
            pos += size
        else:
            raise ValueError('unsupported protobuf wire type')
        yield field, value


def dictionary(model):
    for field, value in protobuf_fields(model):
        if field == 14:  # onnx.ModelProto.metadata_props
            item = dict(protobuf_fields(value))
            if item.get(1) == b'character':
                return item[2]
    raise ValueError('recognition model does not contain its character dictionary')


def fetch(cache, name, record):
    path = cache / name
    if path.is_file() and sha(path.read_bytes()) == record['sha256']:
        return path
    part = path.with_suffix(path.suffix + '.part')
    try:
        subprocess.run(['curl', '-L', '--fail', '--silent', '--show-error', '--retry', '2',
                        '--max-time', '180', record['url'], '-o', str(part)], check=True)
        if sha(part.read_bytes()) != record['sha256']:
            raise ValueError(f'SHA256 mismatch: {name}')
        part.replace(path)
    finally:
        part.unlink(missing_ok=True)
    return path



def sdk_asset(target):
    return {
        'linux/amd64': 'sdk-linux.tar.gz',
        'windows/amd64': 'sdk-windows.zip',
        'darwin/arm64': 'sdk-macos-arm64.tar.gz',
    }[target]


def unpack_sdk(archive, target, version, stage):
    """Read only named files; never extract paths or follow archive links."""
    target_os, arch = target.split('/')
    platforms = {'linux': 'linux_x86_64', 'windows': 'win_amd64',
                 'darwin': 'macosx_' + ('arm64' if arch == 'arm64' else 'x86_64')}
    prefix = f'RapidOCRLite-SDK-{platforms[target_os]}-{version}/'
    if target_os == 'windows':
        sdk = zipfile.ZipFile(archive)
        def read(name):
            return sdk.read(prefix + name)
        native_name, output = 'bin/rapidocrlite.dll', 'rapidocrlite.dll'
    else:
        sdk = tarfile.open(archive, 'r:gz')
        def read(name):
            member = sdk.getmember(prefix + name)
            if not member.isfile():
                raise ValueError(f'expected regular SDK file: {name}')
            return sdk.extractfile(member).read()
        native_name, output = ('lib/librapidocrlite.so.5.0.0', 'librapidocrlite.so')
        if target_os == 'darwin':
            native_name, output = 'lib/librapidocrlite.5.0.0.dylib', 'librapidocrlite.dylib'
    try:
        (stage / 'lib' / output).write_bytes(read(native_name))
        for name in ['LICENSE', 'LICENSE-indra-ocr']:
            (stage / 'licenses' / ('runtime-' + name)).write_bytes(read(name))
        (stage / 'abi/rapidocrlite.h').write_bytes(read('include/rapidocrlite/rapidocrlite.h'))
    finally:
        sdk.close()

def copy_go_licenses(stage, host_env):
    # A target build only downloads modules used on that OS. Explicitly fetch
    # each pinned module before reading its license; `go list -m` may have no Dir.
    for mod in ['github.com/ebitengine/purego', 'golang.org/x/image', 'golang.org/x/sys']:
        record = json.loads(subprocess.check_output(
            ['go', 'mod', 'download', '-json', mod], cwd=ROOT, env=host_env, text=True, encoding='utf-8'))
        if record.get('Error') or not record.get('Dir'):
            raise ValueError(f'cannot locate license for {mod}: {record.get("Error", "missing module directory")}')
        source = Path(record['Dir']) / 'LICENSE'
        if not source.is_file():
            raise ValueError(f'missing dependency license for {mod}: {source}')
        shutil.copyfile(source, stage / 'licenses' / (mod.replace('/', '-') + '.txt'))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', choices=['linux/amd64', 'windows/amd64', 'darwin/arm64'], default='linux/amd64')
    parser.add_argument('--cache', type=Path, default=ROOT / 'build/ppocr-cache')
    args = parser.parse_args()
    args.cache.mkdir(parents=True, exist_ok=True)
    lock = json.loads((SOURCE / 'assets.lock.json').read_text(encoding='utf-8'))
    target_os, arch = args.target.split('/')
    sdk_name = sdk_asset(args.target)
    assets = {name: fetch(args.cache, name, lock['assets'][name])
              for name in ['detection.onnx', 'recognition.onnx', sdk_name]}
    chars = dictionary(assets['recognition.onnx'].read_bytes())
    if sha(chars) != lock['dictionary_sha256']:
        raise ValueError('dictionary checksum mismatch')
    with tempfile.TemporaryDirectory(prefix='lumin-ppocr-') as tmp:
        stage = Path(tmp) / 'plugin'
        for directory in ['bin', 'lib', 'models', 'licenses', 'abi']:
            (stage / directory).mkdir(parents=True)
        for name in ['detection.onnx', 'recognition.onnx']:
            shutil.copyfile(assets[name], stage / 'models' / name)
        (stage / 'models/dictionary.txt').write_bytes(chars)
        version = lock['runtime_version']
        unpack_sdk(assets[sdk_name], args.target, version, stage)
        for name in ['README.md', 'NOTICE', 'MODEL_LICENSES.md', 'LICENSE-APACHE-2.0', 'assets.lock.json']:
            shutil.copyfile(SOURCE / name, stage / name)
        env = dict(os.environ, CGO_ENABLED='0', GOOS=target_os, GOARCH=arch)
        exe = 'bin/plugin.exe' if target_os == 'windows' else 'bin/plugin'
        subprocess.run(['go', 'build', '-buildvcs=false', '-trimpath', '-ldflags=-s -w',
                        '-o', str(stage / exe), './plugins/ppocrv6-tiny'], cwd=ROOT, env=env, check=True)
        # macOS runners sign the extracted executable and library before the
        # archive is made; MyGo cannot sign code nested inside a plugin ZIP.
        if target_os == 'darwin' and sys.platform == 'darwin':
            for code in [stage / exe, stage / 'lib/librapidocrlite.dylib']:
                subprocess.run(['codesign', '--force', '--sign', '-', str(code)], check=True)
        host_env = dict(os.environ, CGO_ENABLED='0')
        host_env.pop('GOOS', None)
        host_env.pop('GOARCH', None)
        # Preserve licenses of the Go libraries linked into the entrypoint.
        copy_go_licenses(stage, host_env)
        goroot = subprocess.check_output(['go', 'env', 'GOROOT'], env=host_env, text=True, encoding='utf-8').strip()
        go_license = Path(goroot) / 'LICENSE'
        if not go_license.is_file():
            go_license = SOURCE / 'LICENSE-GO'
        shutil.copyfile(go_license, stage / 'licenses/Go-LICENSE.txt')
        manifest = json.loads((SOURCE / 'plugin.json').read_text(encoding='utf-8'))
        manifest['entrypoints'] = {args.target: exe}
        (stage / 'plugin.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding='utf-8')
        output = ROOT / 'build/plugins' / f"{manifest['id']}-{manifest['version']}-{target_os}-{arch}.luminext"
        output.parent.mkdir(parents=True, exist_ok=True)
        subprocess.run(['go', 'run', './cmd/lumin-plugin-pack', '-dir', str(stage), '-out', str(output)],
                       cwd=ROOT, env=host_env, check=True)
        print(f'Built {output} ({output.stat().st_size:,} bytes)')


if __name__ == '__main__':
    main()
