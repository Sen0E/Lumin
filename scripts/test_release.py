import importlib.util
import io
import json
import subprocess
import sys
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import release

spec = importlib.util.spec_from_file_location('ppocr_build', Path(__file__).with_name('build-ppocr-plugin.py'))
ppocr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ppocr)


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'mygo.json').write_text('{"version":"1.2.3"}', encoding="utf-8")
        (self.root / 'CHANGELOG.md').write_text('# Changes\n\n## 1.2.3 — today\n\n- Fixed a bug.\n\n## 1.2.2\nOld notes.\n', encoding='utf-8')
        for folder, name in [('caiyun', 'lumin.caiyun'), ('ppocrv6-tiny', 'lumin.ppocrv6-tiny')]:
            p = self.root / 'plugins' / folder
            p.mkdir(parents=True)
            (p / 'plugin.json').write_text(json.dumps({'id': name, 'version': '1.0.0', 'name': '离线识别与翻译'}, ensure_ascii=False), encoding='utf-8')

    def test_plugin_refs_use_explicit_utf8_and_cli(self):
        self.assertEqual(release.plugin_ref(self.root, 'caiyun'), 'lumin.caiyun-1.0.0')
        # Disable UTF-8 mode and reject implicit encodings: this fails even on a
        # UTF-8 Linux host if the CLI accidentally relies on locale defaults.
        for name in ['caiyun', 'ppocrv6-tiny']:
            result = subprocess.run(
                [sys.executable, '-X', 'utf8=0', '-X', 'warn_default_encoding=1',
                 '-W', 'error::EncodingWarning', str(Path(release.__file__)),
                 'plugin-ref', '--plugin', name],
                check=True, capture_output=True, text=True, encoding='utf-8')
            self.assertEqual(result.stdout.strip(), release.plugin_ref(release.ROOT, name))

    def test_prerelease_linux_names_match_mygo_package_versions(self):
        names = release.installer_assets('1.2.3-rc.1-test', 'linux/amd64')
        self.assertIn('lumin_1.2.3~rc.1.test_amd64.deb', names)
        self.assertIn('lumin-1.2.3~rc.1_test-1.x86_64.rpm', names)

    def test_tag_must_match_version_and_commit(self):
        with self.assertRaises(ValueError):
            release.metadata(self.root, 'v1.2.4')
        with patch.object(release.subprocess, 'check_output', side_effect=['old\n', 'head\n']):
            with self.assertRaises(ValueError):
                release.metadata(self.root, 'v1.2.3')

    def test_finalize_requires_all_platforms_and_both_plugins(self):
        output = self.root / 'release'
        output.mkdir()
        names = [name for target in release.TARGETS for name in release.installer_assets('1.2.3', target) + release.plugin_assets(self.root, target)]
        self.assertEqual(len(names), 12)
        for name in names[:-1]:
            (output / name).write_bytes(name.encode())
        notes = self.root / 'notes.md'
        with self.assertRaises(ValueError):
            release.finalize(self.root, notes)
        (output / names[-1]).write_bytes(b'plugin')
        release.finalize(self.root, notes)
        self.assertEqual(len((output / 'SHA256SUMS').read_text(encoding='utf-8').splitlines()), 12)
        self.assertIn('Fixed a bug.', notes.read_text(encoding='utf-8'))
        self.assertNotIn('Old notes.', notes.read_text(encoding='utf-8'))
        (output / 'stale-version.exe').write_bytes(b'old')
        with self.assertRaises(ValueError):
            release.finalize(self.root, notes)

    def test_windows_portable_requires_both_bundles(self):
        platform = self.root / 'build/windows-amd64'
        platform.mkdir(parents=True)
        for name in ['Lumin.exe', 'icon.png', 'bundled-ocr.luminext', 'lumin-setup-1.2.3-amd64.exe']:
            (platform / name).write_bytes(b'data')
        with patch.object(release, 'verify_plugin'):
            with self.assertRaises(ValueError):
                release.collect(self.root, 'windows/amd64')

    def test_plugin_rejects_wrong_platform_and_missing_source(self):
        file = self.root / 'test.luminext'
        with zipfile.ZipFile(file, 'w') as archive:
            archive.writestr('plugin.json', json.dumps({'entrypoints': {'linux/amd64': 'bin/plugin'}, 'capabilities': ['translate']}))
            archive.writestr('bin/plugin', b'executable')
        with self.assertRaises(ValueError):
            release.verify_plugin(file, 'linux/amd64', 'lumin.caiyun')
        with self.assertRaises(ValueError):
            release.verify_plugin(file, 'darwin/arm64')
        with self.assertRaises(KeyError):
            release.verify_plugin(file, 'linux/amd64')

    def test_go_licenses_download_modules_before_reading_directories(self):
        stage = self.root / 'stage'
        (stage / 'licenses').mkdir(parents=True)
        modules = ['github.com/ebitengine/purego', 'golang.org/x/image', 'golang.org/x/sys']
        def download(command, **kwargs):
            self.assertEqual(command[:4], ['go', 'mod', 'download', '-json'])
            mod = command[4]
            module_dir = self.root / 'empty-cache' / mod
            self.assertFalse(module_dir.exists())
            module_dir.mkdir(parents=True)
            (module_dir / 'LICENSE').write_text('license for ' + mod, encoding='utf-8')
            return json.dumps({'Path': mod, 'Dir': str(module_dir)})
        with patch.object(ppocr.subprocess, 'check_output', side_effect=download) as fetch:
            ppocr.copy_go_licenses(stage, {})
        self.assertEqual(fetch.call_count, 3)
        for mod in modules:
            self.assertEqual((stage / 'licenses' / (mod.replace('/', '-') + '.txt')).read_text(encoding='utf-8'),
                             'license for ' + mod)

    def test_go_licenses_reject_missing_module_directory_and_license(self):
        stage = self.root / 'stage'
        (stage / 'licenses').mkdir(parents=True)
        for record in [{}, {'Error': 'download failed'}, {'Dir': str(self.root / 'missing')}]:
            with self.subTest(record=record):
                with patch.object(ppocr.subprocess, 'check_output', return_value=json.dumps(record)):
                    with self.assertRaisesRegex(ValueError, 'license'):
                        ppocr.copy_go_licenses(stage, {})
        self.assertEqual(list((stage / 'licenses').iterdir()), [])

    def test_macos_sdk_uses_pinned_regular_files_without_extracting_paths(self):
        stage = self.root / 'stage'
        for name in ['lib', 'licenses', 'abi']:
            (stage / name).mkdir(parents=True)
        source = self.root / 'sdk.tar.gz'
        prefix = 'RapidOCRLite-SDK-macosx_arm64-5.0.0a16/'
        def write_sdk(link=False):
            with tarfile.open(source, 'w:gz') as archive:
                for name in ['lib/librapidocrlite.5.0.0.dylib', 'LICENSE', 'LICENSE-indra-ocr', 'include/rapidocrlite/rapidocrlite.h']:
                    member = tarfile.TarInfo(prefix + name)
                    if link and name.endswith('.dylib'):
                        member.type = tarfile.SYMTYPE
                        member.linkname = '/outside'
                        archive.addfile(member)
                    else:
                        member.size = 4
                        archive.addfile(member, io.BytesIO(b'data'))
        write_sdk()
        ppocr.unpack_sdk(source, 'darwin/arm64', '5.0.0a16', stage)
        self.assertEqual((stage / 'lib/librapidocrlite.dylib').read_bytes(), b'data')
        write_sdk(link=True)
        with self.assertRaises(ValueError):
            ppocr.unpack_sdk(source, 'darwin/arm64', '5.0.0a16', stage)
        self.assertEqual(ppocr.sdk_asset('darwin/arm64'), 'sdk-macos-arm64.tar.gz')


if __name__ == '__main__':
    unittest.main()
