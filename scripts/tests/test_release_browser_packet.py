"""Exact release identity and fail-closed hosted browser control tests, offline."""
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('browser_packet', ROOT / 'scripts/release_browser_packet.py')
packet = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packet)


def release(tag, commit):
    return {'id': 123, 'tag_name': tag, 'target_commitish': commit, 'draft': False,
            'prerelease': '-rc.' in tag, 'immutable': True, 'published_at': '2026-10-02T19:21:28Z',
            'assets': [{'id': i + 1, 'name': name, 'size': 5, 'digest': 'sha256:' + 'b' * 64,
                        'browser_download_url': f'https://github.com/rcourtman/Pulse/releases/download/{tag}/{name}'}
                       for i, name in enumerate((f'pulse-{tag}-linux-amd64.tar.gz', f'pulse-{tag}-linux-amd64.tar.gz.sshsig'))]}


class BrowserPacketTest(unittest.TestCase):
    def setUp(self):
        self.baseline = release('v6.4.5', 'a' * 40)
        self.candidate = release('v6.4.6-rc.1', 'c' * 40)

    def bind(self, archives=None):
        return packet.packet(self.baseline, self.candidate, 'a' * 40, 'c' * 40, 'v6.4.6-rc.1', archives)

    def test_binds_two_immutable_exact_commits_not_control_head(self):
        result = self.bind()
        self.assertEqual(['a' * 40, 'c' * 40], [r['commit'] for r in result['releases']])
        self.assertEqual('rcourtman/Pulse', result['repository'])
        self.assertNotIn('assets', result)  # Only bounded constituent identities are retained.

    def test_rejects_unpublished_mutable_wrong_maturity_or_ref(self):
        for field, value in [('draft', True), ('immutable', False), ('prerelease', False),
                             ('target_commitish', 'main'), ('tag_name', 'v6.4.6-rc.2'),
                             ('published_at', None), ('id', True)]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                candidate = self.candidate | {field: value}
                packet.packet(self.baseline, candidate, 'a' * 40, 'c' * 40, 'v6.4.6-rc.1')
        with self.assertRaises(ValueError):
            packet.packet(self.baseline, release('v6.4.5-rc.9', 'c' * 40), 'a' * 40, 'c' * 40, 'v6.4.5-rc.9')

    def test_current_published_archives_fit_bounded_admission(self):
        # Actual GitHub metadata readback: stable276780330 and RC277779060
        # bytes both exceed the original256MiB limit. No asset bytes executed.
        self.baseline['assets'][0]['size'] = 276780330
        self.candidate['assets'][0]['size'] = 277779060
        self.assertEqual(2, len(self.bind()['releases']))
        self.candidate['assets'][1]['size'] = packet.MAX_SIGNATURE + 1
        with self.assertRaises(ValueError):
            self.bind()

    def test_rejects_redirect_asset_duplicate_oversize_or_unbound_digest(self):
        for field, value in [('browser_download_url', 'https://example.com/archive'), ('size', 0),
                             ('size', 513 * 1024 * 1024), ('digest', None), ('id', True)]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                candidate = copy.deepcopy(self.candidate)
                candidate['assets'][0][field] = value
                packet.packet(self.baseline, candidate, 'a' * 40, 'c' * 40, 'v6.4.6-rc.1')
        self.candidate['assets'].append(self.candidate['assets'][0])
        with self.assertRaises(ValueError):
            self.bind()

    def make_assets(self, root):
        for r in (self.baseline, self.candidate):
            folder = root / r['tag_name']; folder.mkdir()
            for asset in r['assets']:
                path = folder / asset['name']
                if asset['name'].endswith('.sshsig'):
                    path.write_bytes(b'fixture signature - cryptographic verification belongs to SSH workflow')
                else:
                    data = ('fixture-' + r['tag_name']).encode()
                    with tarfile.open(path, 'w:gz') as archive:
                        member = tarfile.TarInfo('./bin/pulse'); member.size = len(data)
                        archive.addfile(member, io.BytesIO(data))
                asset['size'] = path.stat().st_size
                asset['digest'] = 'sha256:' + hashlib.sha256(path.read_bytes()).hexdigest()

    def test_asset_digest_and_binary_digest_both_checked_without_extraction(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); self.make_assets(root)
            result = self.bind(root)
            self.assertEqual(hashlib.sha256(b'fixture-v6.4.6-rc.1').hexdigest(), result['releases'][1]['binary_sha256'])
            target = root / self.candidate['tag_name'] / self.candidate['assets'][1]['name']
            target.write_bytes(b'X' * target.stat().st_size)
            with self.assertRaisesRegex(ValueError, 'digest'):
                self.bind(root)

    def test_symlink_and_duplicate_binary_are_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); self.make_assets(root)
            target = root / self.candidate['tag_name'] / self.candidate['assets'][1]['name']
            real = target.with_suffix('.real'); target.rename(real); target.symlink_to(real)
            with self.assertRaisesRegex(ValueError, 'size/type'):
                self.bind(root)
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'bad.tar.gz'
            for names in (['./bin/pulse', 'bin/pulse'], ['./bin/pulse']):
                with self.subTest(names=names):
                    with tarfile.open(path, 'w:gz') as archive:
                        for name in names:
                            member = tarfile.TarInfo(name); member.type = tarfile.SYMTYPE; member.linkname = '/etc/passwd'
                            archive.addfile(member)
                    with self.assertRaises(ValueError):
                        packet.archive_identity(path)

    def test_native_mode_refuses_worker_before_install_or_network(self):
        result = subprocess.run(['bash', ROOT / 'scripts/release_lifecycle_rehearsal.sh', '--hosted-browser',
                                 '--from', 'v6.4.5', '--to', 'v6.4.6-rc.1'],
                                env={'PATH': '/usr/bin:/bin'}, capture_output=True, text=True)
        self.assertEqual(2, result.returncode)
        self.assertIn('fresh disposable public hosted runner', result.stdout)

    def test_cleanup_failure_sets_process_exit_and_scrubs_only_ephemeral_auth(self):
        source = (ROOT / 'scripts/release_lifecycle_rehearsal.sh').read_text()
        cleanup = source.split('cleanup() {', 1)[1].split('\ntrap cleanup EXIT', 1)[0]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); (root / 'state').mkdir(); (root / 'state/browser-auth.json').write_text('throwaway')
            # Execute the real cleanup function with fixture systemctl: not a system/installation proof.
            script = 'set -u\nOVERALL_STATUS=0; CONTAINER_STARTED=true; HOSTED_BROWSER=true\n'
            script += f'WORK_DIR={json.dumps(tmp)}\nprint_diagnostics() {{ :; }}\nsystemctl() {{ return 1; }}\n'
            script += 'cleanup() {' + cleanup + '\ntrap cleanup EXIT\nexit 0\n'
            result = subprocess.run(['bash', '-c', script], capture_output=True, text=True)
            self.assertEqual(1, result.returncode, result.stderr)
            self.assertEqual('', (root / 'state/browser-auth.json').read_text())
            self.assertEqual(1, json.loads((root / 'state/browser-cleanup.json').read_text())['cleanup_exit'])

    def test_cexec_native_environment_drops_provider_and_registration_secrets(self):
        source = (ROOT / 'scripts/release_lifecycle_rehearsal.sh').read_text()
        function = source.split('cexec() {', 1)[1].split('\nprint_diagnostics()', 1)[0]
        script = 'HOSTED_BROWSER=true\ncexec() {' + function + '\ncexec env EXTRA=fixture\n'
        result = subprocess.run(['bash', '-c', script], env={'PATH': '/usr/bin:/bin',
                                 'GH_TOKEN': 'synthetic-secret', 'TS_OAUTH_SECRET': 'synthetic-secret',
                                 'TS_CI_DEPLOY_OAUTH_CLIENT_ID': 'synthetic-narrow-id',
                                 'TS_CI_DEPLOY_OAUTH_SECRET': 'synthetic-narrow-secret'},
                                capture_output=True, text=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertNotIn('synthetic-secret', result.stdout)
        self.assertNotIn('synthetic-narrow', result.stdout)
        self.assertNotIn('PULSE_INSTALL_ALLOW_DOCKER', result.stdout)
        self.assertIn('EXTRA=fixture', result.stdout)

    def test_workflow_has_no_arbitrary_target_build_publish_or_credential_in_acceptance(self):
        source = (ROOT / '.github/workflows/qualify-browser-update-release.yml').read_text()
        self.assertNotIn('pull_request:', source)
        self.assertNotIn('contents: write', source)
        self.assertNotIn('ignoreHTTPSErrors', (ROOT / 'tests/integration/scripts/release-browser-journey.cjs').read_text())
        self.assertIn('test "$GITHUB_WORKFLOW_SHA" = "$EXPECTED_CONTROL"', source)
        self.assertIn('runs-on: ubuntu-24.04', source)
        self.assertIn('tailscale status --json | python3 scripts/release_browser_tailnet.py', source)
        self.assertIn('ssh-keygen -Y verify', source)
        acceptance = source.split('- name: Exercise native browser update', 1)[1].split('- name: Close', 1)[0]
        self.assertIn('sudo env -i', acceptance)
        for name in ('GH_TOKEN', 'TS_OAUTH_SECRET', 'TS_CI_DEPLOY_OAUTH_CLIENT_ID',
                     'TS_CI_DEPLOY_OAUTH_SECRET', 'PULSE_ALLOW_DOCKER_UPDATES', 'ignoreHTTPSErrors'):
            self.assertNotIn(name, acceptance)
        artifact = source.split('- name: Retain bounded', 1)[1]
        for private in ('browser-auth.json', 'private-install.log', 'private-updater.log', 'datadir.'):
            self.assertNotIn(private, artifact)


if __name__ == '__main__':
    unittest.main()
