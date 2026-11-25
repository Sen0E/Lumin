#!/usr/bin/env python3
"""Validate release tags, collect exact assets and generate SHA256SUMS."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parent.parent
TARGETS = ('linux/amd64', 'windows/amd64')
BUNDLE_IDS = {'bundled-ocr.luminext': 'lumin.ppocrv6-tiny', 'bundled-caiyun.luminext': 'lumin.caiyun'}
BUNDLES = tuple(BUNDLE_IDS)


def version(root):
    value = json.loads((root / 'mygo.json').read_text(encoding='utf-8'))['version']
    if not re.fullmatch(r'\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?', value):
        raise ValueError('mygo.json version must be major.minor.patch[-prerelease]')
    return value


def plugin_ref(root, name):
    manifest = json.loads((root / 'plugins' / name / 'plugin.json').read_text(encoding='utf-8'))
    return f'{manifest["id"]}-{manifest["version"]}'


def plugin_assets(root, target):
    return [f'{plugin_ref(root, name)}-{target.replace("/", "-")}.luminext'
            for name in ('ppocrv6-tiny', 'caiyun')]


def installer_assets(v, target):
    base, separator, pre = v.partition('-')
    deb_version = base + ('~' + pre.replace('-', '.') if separator else '')
    rpm_version = base + ('~' + pre.replace('-', '_') if separator else '')
    return {
        'linux/amd64': [f'lumin-{v}-linux-amd64.tar.gz', f'lumin_{deb_version}_amd64.deb', f'lumin-{rpm_version}-1.x86_64.rpm'],
        'windows/amd64': [f'lumin-setup-{v}-amd64.exe', f'lumin-{v}-windows-amd64.zip'],
    }[target]


def copy_required(source, destination):
    if not source.is_file() or source.stat().st_size == 0:
        raise ValueError(f'missing/empty release asset: {source}')
    shutil.copyfile(source, destination)


def verify_plugin(path, target, expected_id=None):
    with zipfile.ZipFile(path) as archive:
        if archive.testzip() is not None:
            raise ValueError(f'corrupt plugin: {path}')
        manifest = json.loads(archive.read('plugin.json'))
        if expected_id and manifest.get('id') != expected_id:
            raise ValueError(f'incorrect bundled plugin: {path}')
        entry = manifest['entrypoints'].get(target)
        if not entry or entry not in archive.namelist():
            raise ValueError(f'plugin lacks an entrypoint for {target}: {path}')
        if 'recognize' in manifest['capabilities']:
            native = {'linux/amd64': 'librapidocrlite.so', 'windows/amd64': 'rapidocrlite.dll', 'darwin/arm64': 'librapidocrlite.dylib'}[target]
            for name in [f'lib/{native}', 'models/detection.onnx', 'models/recognition.onnx']:
                if not archive.getinfo(name).file_size:
                    raise ValueError(f'plugin is missing {name}')
        else:
            for name in ['LICENSE', 'NOTICE', 'src/go.mod', 'src/plugins/caiyun/caiyun.go']:
                if not archive.getinfo(name).file_size:
                    raise ValueError(f'plugin lacks corresponding source/license: {name}')


def collect(root, target):
    v = version(root)
    output = root / 'release'
    output.mkdir(exist_ok=True)
    platform = root / 'build' / target.replace('/', '-')
    # Ensure both bundles were prepared for this exact platform.
    for name in BUNDLES:
        verify_plugin(root / 'resources' / target.replace('/', '-') / name, target, BUNDLE_IDS[name])
    if target == 'windows/amd64':
        portable = output / f'lumin-{v}-windows-amd64.zip'
        with zipfile.ZipFile(portable, 'w', zipfile.ZIP_DEFLATED) as archive:
            for name in ['Lumin.exe', 'icon.png', *BUNDLES]:
                source = platform / name
                if not source.is_file() or source.stat().st_size == 0:
                    raise ValueError(f'missing portable file: {source}')
                archive.write(source, 'Lumin/' + name)
        copy_required(platform / f'lumin-setup-{v}-amd64.exe', output / f'lumin-setup-{v}-amd64.exe')
    else:
        import tarfile
        with tarfile.open(platform / f'lumin-{v}-linux-amd64.tar.gz') as archive:
            names = {Path(name).name for name in archive.getnames()}
            if not set(BUNDLES).issubset(names):
                raise ValueError('Linux archive must contain both bundled plugins')
        for name in installer_assets(v, target):
            copy_required(platform / name, output / name)
    for name in plugin_assets(root, target):
        source = root / 'build/plugins' / name
        verify_plugin(source, target)
        copy_required(source, output / name)


def finalize(root, notes):
    v = version(root)
    output = root / 'release'
    expected = {name for target in TARGETS for name in installer_assets(v, target) + plugin_assets(root, target)}
    actual = {p.name for p in output.iterdir() if p.is_file() and p.name != 'SHA256SUMS'}
    if actual != expected:
        raise ValueError(f'asset set mismatch; missing={sorted(expected-actual)}, unexpected={sorted(actual-expected)}')
    rows = []
    for name in sorted(expected):
        data = (output / name).read_bytes()
        if not data:
            raise ValueError(f'empty release asset: {name}')
        rows.append(f'{hashlib.sha256(data).hexdigest()}  {name}\n')
    (output / 'SHA256SUMS').write_text(''.join(rows), encoding='utf-8')
    changelog = (root / 'CHANGELOG.md').read_text(encoding='utf-8')
    match = re.search(rf'^## {re.escape(v)}(?:\s|$).*?\n(.*?)(?=^## |\Z)', changelog, re.M | re.S)
    body = match.group(1).strip() if match else f'Lumin {v}'
    body += '''\n\n## 安装包与插件\n\n- Linux x64：DEB、RPM、tar.gz。\n- Windows x64：安装程序与便携 ZIP。\n- 各平台安装包均附带彩云小译与 PP-OCRv6 tiny 插件，同时提供独立 .luminext 下载。离线 OCR 首次启动自动配置；彩云需主动导入/配置并启用。\n- SHA256SUMS 用于校验下载文件。\n\nWindows 安装程序未做代码签名，系统可能提示验证发布者。\n'''
    notes.write_text(body + '\n', encoding='utf-8')


def metadata(root, tag):
    v = version(root)
    if tag != 'v' + v:
        raise ValueError(f'tag {tag!r} does not match mygo.json: v{v}')
    commit = subprocess.check_output(['git', 'rev-parse', '--verify', f'refs/tags/{tag}^{{commit}}'], cwd=root, text=True).strip()
    head = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
    if commit != head:
        raise ValueError('checkout does not match the requested release tag')
    result = f'version={v}\ntag={tag}\nprerelease={str("-" in v).lower()}\n'
    print(result, end='')
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as stream:
            stream.write(result)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    sub.add_parser('metadata').add_argument('--tag', required=True)
    sub.add_parser('plugin-ref').add_argument('--plugin', choices=['ppocrv6-tiny', 'caiyun'], required=True)
    sub.add_parser('collect').add_argument('--target', choices=TARGETS, required=True)
    sub.add_parser('finalize').add_argument('--notes', type=Path, required=True)
    args = parser.parse_args()
    if args.command == 'metadata':
        metadata(ROOT, args.tag)
    elif args.command == 'plugin-ref':
        print(plugin_ref(ROOT, args.plugin))
    elif args.command == 'collect':
        collect(ROOT, args.target)
    else:
        finalize(ROOT, args.notes)


if __name__ == '__main__':
    main()
