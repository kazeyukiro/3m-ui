#!/usr/bin/env python3
"""Exercise installation/update/restore entirely in temporary directories.

Network, init system, root identity, binaries and sleep are mocked. No host
service, root directory, release or proxy is modified by these tests.
"""
import hashlib
import io
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parents[1]

PANEL = '''#!/bin/sh
set -eu
case "${1:-}" in
  --version) echo "3m-ui @VERSION@";;
  storage-paths)
    if [ -n "${TEST_EXTERNAL_STORAGE:-}" ]; then echo "$TEST_EXTERNAL_STORAGE"; else echo "$THREE_M_UI_CONFIG"; fi;;
  init)
    mkdir -p "$(dirname "$THREE_M_UI_CONFIG")" "$THREE_M_UI_DATA_DIR"
    if [ ! -f "$THREE_M_UI_CONFIG" ]; then
      echo "port: 8080" > "$THREE_M_UI_CONFIG"
      echo "old-db" > "$THREE_M_UI_DATA_DIR/3m-ui.db"
      echo "Initial password: random-one-time-test-password"
    fi
    if [ "@VERSION@" = v2.0.0-badinit ]; then
      echo corrupted > "$THREE_M_UI_CONFIG"
      echo migrated > "$THREE_M_UI_DATA_DIR/3m-ui.db"
      echo new > "$THREE_M_UI_DATA_DIR/created-by-migration"
      exit 1
    fi;;
  healthcheck) [ "@VERSION@" != v2.0.0-badhealth ];;
  *) exit 2;;
esac
'''

CURL = '''#!/usr/bin/env python3
import os, pathlib, sys, shutil
args=sys.argv[1:]
url=next(a for a in args if a.startswith('https://'))
with open(os.environ['TEST_EVENTS'], 'a') as f: f.write('download '+url+'\\n')
if url.endswith('/releases/latest'):
 print(url[:-len('/releases/latest')]+'/releases/tag/'+os.environ.get('TEST_LATEST','v1.0.0'),end='')
else:
 path=url.split('/releases/download/',1)[1]
 source=pathlib.Path(os.environ['TEST_RELEASES'])/path
 if not source.is_file(): sys.exit(22)
 shutil.copyfile(source,args[args.index('-o')+1])
'''

SYSTEMCTL = '''#!/usr/bin/env python3
import os,pathlib,sys
root=pathlib.Path(os.environ['THREE_M_UI_ROOT'])
a=sys.argv[1]
active=root/'running'
enabled=root/'enabled'
with open(os.environ['TEST_EVENTS'],'a') as f: f.write('service '+a+'\\n')
if a=='is-active': sys.exit(0 if active.exists() else 3)
if a=='is-enabled': sys.exit(0 if enabled.exists() else 1)
if a=='start':
 version=(root/'usr/local/lib/3m-ui/VERSION').read_text().strip()
 if version=='v2.0.0-badstart': sys.exit(1)
 active.touch()
if a=='stop': active.unlink(missing_ok=True)
if a=='enable': enabled.touch()
if a=='disable': enabled.unlink(missing_ok=True)
'''


class LifecycleTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='3m-lifecycle-')
        self.directory = Path(self.tmp.name)
        self.root = self.directory / 'root'
        self.root.mkdir()
        (self.root / 'run/systemd/system').mkdir(parents=True)
        self.release_root = self.directory / 'releases'
        self.release_root.mkdir()
        self.mock = self.directory / 'mock-bin'
        self.mock.mkdir()
        self.events = self.directory / 'events'
        self.events.touch()
        self.env = os.environ.copy()
        self.env.update(THREE_M_UI_ROOT=str(self.root),
                        THREE_M_UI_REPO='example/3m-ui',
                        TEST_EVENTS=str(self.events), TEST_RELEASES=str(self.release_root),
                        PATH=str(self.mock) + os.pathsep + self.env['PATH'])
        for name, source in {
            'curl': CURL, 'systemctl': SYSTEMCTL,
            'id': '#!/bin/sh\necho 0\n',
            'uname': '#!/bin/sh\ncase "$1" in -m) echo x86_64;; -s) echo Linux;; esac\n',
            'sleep': '#!/bin/sh\nexit 0\n',
        }.items():
            self.executable(self.mock / name, source)
        self.make_release('v1.0.0')

    def tearDown(self):
        self.tmp.cleanup()

    @property
    def base(self): return self.root / 'usr/local/lib/3m-ui'
    @property
    def config(self): return self.root / 'etc/3m-ui'
    @property
    def data(self): return self.root / 'var/lib/3m-ui'

    def executable(self, path, source):
        path.write_text(source)
        path.chmod(0o755)

    def make_release(self, version):
        destination = self.release_root / version
        destination.mkdir()
        panel = PANEL.replace('@VERSION@', version).encode()
        members = {
            '3m-ui-bin': panel, 'mihomo': b'#!/bin/sh\necho Mihomo test\n',
            'VERSION': version.encode(), 'REPOSITORY': b'example/3m-ui',
            'MIHOMO_VERSION': b'v1.19.0', 'mihomo.env': b'MIHOMO_VERSION=v1.19.0',
            'LICENSE': b'Panel license', 'MIHOMO_LICENSE': b'Core license',
            'MIHOMO_SOURCE': b'https://github.com/MetaCubeX/mihomo/tree/v1.19.0',
        }
        members.update({name: (SCRIPTS / name).read_bytes() for name in
                        ['install.sh', 'update.sh', 'uninstall.sh', '3m-ui', '3m-ui.sh']})
        archive = destination / '3m-ui-bundle-linux-amd64.tar.gz'
        with tarfile.open(archive, 'w:gz') as tar:
            for name, content in members.items():
                info = tarfile.TarInfo(name)
                info.size = len(content)
                info.mode = 0o755
                tar.addfile(info, io.BytesIO(content))
        for name in ['install.sh', 'update.sh', 'uninstall.sh', '3m-ui', '3m-ui.sh']:
            (destination / name).write_bytes(members[name])
        (destination / '3m-ui-linux-amd64').write_bytes(panel)
        (destination / 'SHA256SUMS').write_text(''.join(
            hashlib.sha256(path.read_bytes()).hexdigest() + '  ' + path.name + '\n'
            for path in sorted(destination.iterdir())))

    def run_script(self, name='install.sh', *args, success=True, **env):
        run_env = self.env | env
        result = subprocess.run(['sh', str(SCRIPTS / name), *args], env=run_env,
                                capture_output=True, text=True)
        if success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def install(self): return self.run_script('install.sh', '--yes')

    def test_default_stable_complete_install(self):
        result = self.install()
        self.assertEqual((self.base / 'VERSION').read_text().strip(), 'v1.0.0')
        self.assertEqual((self.base / 'CHANNEL').read_text().strip(), 'stable')
        self.assertEqual((self.base / 'REPOSITORY').read_text().strip(), 'example/3m-ui')
        self.assertTrue((self.base / 'mihomo').is_file())
        self.assertTrue((self.root / 'running').is_file())
        self.assertIn('Initial password:', result.stdout)
        for directory in (self.base, self.config, self.data):
            for file in directory.rglob('*'):
                if file.is_file() and file.suffix != '.gz':
                    # The fake binary embeds the test fixture, so inspect actual state only.
                    if file.parent == self.config or file.name == '3m-ui.db':
                        self.assertNotIn(b'random-one-time-test-password', file.read_bytes())
        self.assertIn('/releases/latest', self.events.read_text())
        self.assertNotIn('/download/pre/', self.events.read_text())

    def test_explicit_pre_channel_persists(self):
        self.make_release('pre')
        self.run_script('install.sh', 'pre', '--yes')
        self.events.write_text('')
        self.run_script('update.sh', '--yes', THREE_M_UI_REPO='')
        self.assertEqual((self.base / 'CHANNEL').read_text().strip(), 'pre')
        self.assertIn('https://github.com/example/3m-ui/releases/download/pre/', self.events.read_text())
        self.assertNotIn('/releases/latest', self.events.read_text())

    def test_semver_build_metadata_can_be_installed(self):
        self.make_release('v1.0.0+build.1')
        self.run_script('install.sh', 'v1.0.0+build.1', '--yes')
        self.assertEqual((self.base / 'VERSION').read_text().strip(), 'v1.0.0+build.1')

    def test_bad_checksum_keeps_service_and_every_file(self):
        self.install()
        old = (self.base / '3m-ui-bin').read_bytes()
        self.make_release('v2.0.0')
        (self.release_root / 'v2.0.0/3m-ui-bundle-linux-amd64.tar.gz').write_bytes(b'broken')
        self.events.write_text('')
        result = self.run_script('update.sh', 'v2.0.0', '--yes', success=False)
        self.assertIn('Checksum mismatch', result.stderr)
        self.assertNotIn('service stop', self.events.read_text())
        self.assertEqual((self.base / '3m-ui-bin').read_bytes(), old)
        self.assertTrue((self.root / 'running').is_file())

    def add_persistent_state(self):
        (self.config / 'config.yaml').write_text('custom config and encryption keys')
        (self.data / '3m-ui.db-wal').write_text('wal-state')
        (self.data / 'certificates').mkdir(exist_ok=True)
        (self.data / 'certificates/listener.key').write_text('private-key')
        (self.base / 'custom').write_text('preserve-extra')

    def assert_persistent_state(self):
        self.assertEqual((self.config / 'config.yaml').read_text(), 'custom config and encryption keys')
        self.assertEqual((self.data / '3m-ui.db').read_text().strip(), 'old-db')
        self.assertEqual((self.data / '3m-ui.db-wal').read_text(), 'wal-state')
        self.assertEqual((self.data / 'certificates/listener.key').read_text(), 'private-key')
        self.assertEqual((self.base / 'custom').read_text(), 'preserve-extra')
        self.assertFalse((self.data / 'created-by-migration').exists())

    def test_failed_init_restores_complete_state(self):
        self.install()
        self.add_persistent_state()
        old_entry = (self.root / 'usr/local/bin/3m-ui').read_bytes()
        self.make_release('v2.0.0-badinit')
        self.events.write_text('')
        self.run_script('update.sh', 'v2.0.0-badinit', '--yes', success=False)
        self.assert_persistent_state()
        self.assertEqual((self.base / 'VERSION').read_text().strip(), 'v1.0.0')
        self.assertEqual((self.root / 'usr/local/bin/3m-ui').read_bytes(), old_entry)
        self.assertTrue((self.root / 'running').is_file())
        events = self.events.read_text().splitlines()
        self.assertLess(max(i for i, line in enumerate(events) if line.startswith('download ')),
                        next(i for i, line in enumerate(events) if line == 'service stop'))

    def test_failed_start_and_health_restore(self):
        for version in ['v2.0.0-badstart', 'v2.0.0-badhealth']:
            with self.subTest(version=version):
                self.install()
                self.add_persistent_state()
                self.make_release(version)
                self.run_script('update.sh', version, '--yes', success=False)
                self.assert_persistent_state()
                self.assertEqual((self.base / 'VERSION').read_text().strip(), 'v1.0.0')
                self.assertTrue((self.root / 'running').is_file())

    def test_backup_restore_and_retained_archives(self):
        self.install()
        self.add_persistent_state()
        result = self.run_script('install.sh', '--backup')
        snapshot = Path(next(line.removeprefix('Snapshot: ') for line in result.stdout.splitlines() if line.startswith('Snapshot: ')))
        with tarfile.open(snapshot) as tar:
            self.assertFalse(any('/backups/' in name for name in tar.getnames()))
            self.assertIn('./data/3m-ui.db-wal', tar.getnames())
        (self.data / '3m-ui.db').write_text('changed')
        (self.data / 'certificates/listener.key').unlink()
        self.run_script('install.sh', '--restore', str(snapshot), '--yes')
        self.assert_persistent_state()
        self.assertTrue(snapshot.exists())
        self.assertTrue((self.root / 'running').is_file())

    def test_external_storage_rejected_before_stop(self):
        self.install()
        self.make_release('v2.0.0')
        self.events.write_text('')
        result = self.run_script('update.sh', 'v2.0.0', success=False, TEST_EXTERNAL_STORAGE='/srv/shared/cert.pem')
        self.assertIn('Storage outside', result.stderr)
        self.assertNotIn('service stop', self.events.read_text())
        self.assertTrue((self.root / 'running').is_file())

    def test_uninstall_keeps_data_config_external_core(self):
        self.install()
        self.add_persistent_state()
        external = self.root / 'usr/local/bin/mihomo'
        external.write_text('external-core')
        self.run_script('uninstall.sh', '--yes')
        self.assertTrue((self.config / 'config.yaml').is_file())
        self.assertTrue((self.data / 'certificates/listener.key').is_file())
        self.assertEqual(external.read_text(), 'external-core')
        self.assertFalse(self.base.exists())
        self.run_script('uninstall.sh', '--yes', '--purge')
        self.assertFalse(self.config.exists())
        self.assertFalse(self.data.exists())
        self.assertTrue(external.is_file())

    def test_failed_first_install_removes_service_and_enablement(self):
        self.make_release('v2.0.0-badstart')
        self.run_script('install.sh', 'v2.0.0-badstart', '--yes', success=False)
        self.assertFalse(self.base.exists())
        self.assertFalse((self.root / 'etc/systemd/system/3m-ui.service').exists())
        self.assertFalse((self.root / 'enabled').exists())
        self.assertFalse((self.root / 'running').exists())

    def test_failed_snapshot_restarts_old_service(self):
        self.install()
        self.make_release('v2.0.0')
        real_tar = shutil.which('tar')
        self.executable(self.mock / 'tar', '#!/bin/sh\nif [ "$1" = -czf ]; then exit 1; fi\nexec "' + real_tar + '" "$@"\n')
        self.run_script('update.sh', 'v2.0.0', '--yes', success=False)
        self.assertEqual((self.base / 'VERSION').read_text().strip(), 'v1.0.0')
        self.assertTrue((self.root / 'running').exists())

    def test_custom_data_directory_persists_for_updates(self):
        custom = self.root / 'srv/3m-ui-data'
        self.run_script('install.sh', '--yes', THREE_M_UI_DATA_DIR=str(custom))
        self.make_release('v2.0.0')
        self.run_script('update.sh', 'v2.0.0', '--yes')
        self.assertEqual((self.base / 'DATA_DIRECTORY').read_text().strip(), str(custom))
        self.assertTrue((custom / '3m-ui.db').is_file())
        self.assertFalse((self.data / '3m-ui.db').exists())

    def test_latest_never_falls_back_to_pre(self):
        self.make_release('pre')
        result = self.run_script('install.sh', '--yes', success=False, TEST_LATEST='pre')
        self.assertIn('Unable to resolve a stable release', result.stderr)
        self.assertNotIn('/download/pre/', self.events.read_text())
        self.assertFalse(self.base.exists())

    def test_no_mihomo_is_explicit_standalone(self):
        self.run_script('install.sh', '--no-mihomo', '--yes')
        self.assertTrue((self.base / '3m-ui-bin').is_file())
        self.assertFalse((self.base / 'mihomo').exists())


if __name__ == '__main__':
    unittest.main()
