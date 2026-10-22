#!/usr/bin/env python3
"""Exercise the unchanged signed download -> collector/helper publication slice.

Real curl/TLS, SSH signatures, file copies, modes and rename operations run in
private files. Service stop/kill and root ownership are explicit doubles; this
is not native service, appliance or installed-customer recovery proof.
"""
import base64
import contextlib
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import platform
import shutil
import ssl
import subprocess
import tempfile
import threading
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get('PULSE_AGENT_PUBLICATION_SOURCE', ROOT / 'scripts/install.sh'))
OBSERVATIONS = []


def definition(source, name):
    marker = name + '() {'
    if source.count(marker) != 1:
        raise ValueError('missing or ambiguous producer function: ' + name)
    return marker + source.split(marker, 1)[1].split('\n}\n', 1)[0] + '\n}\n'


def checked(argv):
    return subprocess.run(argv, check=True, capture_output=True, text=True, timeout=15)


class AgentBinaryPublication(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if platform.system() != 'Linux':
            raise RuntimeError('real cross-filesystem fault proof requires the Linux source VM')
        cls.resources = contextlib.ExitStack()
        cls.addClassCleanup(cls.resources.close)
        cls.root = Path(cls.resources.enter_context(tempfile.TemporaryDirectory()))
        cls.source = SOURCE.read_text()
        names = ['cleanup', 'log_info', 'log_warn', 'log_error', 'redact_token', 'json_event',
                 'fail', 'nearest_existing_dir', 'get_filesystem_device_for_path',
                 'get_available_bytes_for_path', 'bytes_to_human', 'ensure_agent_disk_headroom',
                 'final_response_header_value', 'installer_file_sha256',
                 'has_pinned_installer_signature_key', 'decode_base64_to_file',
                 'verify_download_signature']
        if 'stage_verified_agent_executable() {' in cls.source:
            names += ['stage_verified_agent_executable']
        cls.functions = '\n'.join(definition(cls.source, name) for name in names)
        begin = '# --- Download ---\n'
        end = '# --- Service Installation ---\n'
        if cls.source.count(begin) != 1 or cls.source.count(end) != 1:
            raise RuntimeError('ambiguous complete download/publication boundary')
        cls.producer = cls.source.split(begin, 1)[1].split(end, 1)[0]
        cls.traps = '\n'.join(line for line in cls.source.split('# --- Configuration ---', 1)[0].splitlines()
                              if line.startswith('trap '))
        cls.tools = {}
        for name in ['curl', 'cp', 'mv', 'chmod', 'mktemp', 'openssl', 'ssh-keygen']:
            path = shutil.which(name)
            if not path:
                raise RuntimeError(name + ' is required; never skip this proof')
            cls.tools[name] = path
        cls.previous = Path('/bin/false').read_bytes()
        cls.candidate = Path('/bin/true').read_bytes() + b'\0' * 131072
        (cls.root / 'payload').write_bytes(cls.candidate)
        checked(['ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', str(cls.root / 'signer')])
        checked(['ssh-keygen', '-q', '-Y', 'sign', '-f', str(cls.root / 'signer'),
                 '-n', 'pulse-install', str(cls.root / 'payload')])
        cls.key = (cls.root / 'signer.pub').read_text().strip()
        cls.signature = base64.b64encode((cls.root / 'payload.sig').read_bytes()).decode()
        (cls.root / 'tls.conf').write_text('[req]\ndistinguished_name=dn\nx509_extensions=san\n'
                                           '[dn]\n[san]\nsubjectAltName=IP:127.0.0.1\n')
        checked(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                 '-subj', '/CN=localhost', '-config', str(cls.root / 'tls.conf'),
                 '-keyout', str(cls.root / 'tls.key'), '-out', str(cls.root / 'tls.crt')])
        print('real curl:', checked(['curl', '--version']).stdout.splitlines()[0], flush=True)

    def exercise(self, fault='', *, helper=False, fresh=False, target_kind='file'):
        with tempfile.TemporaryDirectory(prefix='agent-publication-') as temporary:
            root = Path(temporary)
            # /dev/shm is a different, real filesystem. Do not emulate EXDEV or
            # mv's copy fallback: the parent actually loses its old target here.
            with tempfile.TemporaryDirectory(prefix='agent-download-', dir='/dev/shm') as download:
                download = Path(download)
                self.assertNotEqual(root.stat().st_dev, download.stat().st_dev)
                installed = root / 'install with spaces'; installed.mkdir()
                helper_dir = root / 'helper'; helper_dir.mkdir()
                state = root / 'state'; state.mkdir(); (state / 'token').write_text('inert preserved fixture\n')
                state_before = (state / 'token').read_bytes()
                target = installed / 'pulse-agent'
                helper_target = helper_dir / 'pulse-agent-helper'
                victim = root / 'unrelated'
                if target_kind == 'directory':
                    target.mkdir()
                elif target_kind.startswith('symlink'):
                    if target_kind == 'symlink-directory': victim.mkdir()
                    else: victim.write_bytes(self.previous)
                    target.symlink_to(victim)
                elif not fresh:
                    target.write_bytes(self.previous); target.chmod(0o755)
                if helper:
                    helper_target.write_bytes(self.previous); helper_target.chmod(0o755)
                requests = []
                candidate = self.candidate
                signature = self.signature
                digest = hashlib.sha256(candidate).hexdigest()

                class Handler(BaseHTTPRequestHandler):
                    def log_message(self, *_args): pass
                    def do_GET(self):
                        requests.append([self.command, self.path])
                        if self.path == '/api/version':
                            body = b'{"version":"fixture"}'
                            self.send_response(200)
                        elif self.path.startswith('/download/'):
                            body = candidate
                            if fault == 'changed-bytes': body = b'\x7fELF' + b'changed' * 1024
                            self.send_response(200)
                            if fault != 'checksum-missing': self.send_header('X-Checksum-Sha256', digest)
                            if fault != 'signature-missing':
                                self.send_header('X-Signature-SSHSIG', signature if fault != 'bad-signature' else base64.b64encode(b'invalid').decode())
                        else:
                            self.send_response(404); body = b'not found'
                        self.send_header('Content-Length', str(len(body))); self.end_headers(); self.wfile.write(body)

                server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
                tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
                tls.minimum_version = ssl.TLSVersion.TLSv1_2
                tls.load_cert_chain(self.root / 'tls.crt', self.root / 'tls.key')
                server.socket = tls.wrap_socket(server.socket, server_side=True)
                thread = threading.Thread(target=server.serve_forever, daemon=True); thread.start()
                try:
                    tools = root / 'tools'; tools.mkdir()
                    # All file changes are real except the named failure point.
                    wrapper = '''#!/usr/bin/env python3
import hashlib,json,os,resource,signal,subprocess,sys
from pathlib import Path
name=Path(sys.argv[0]).name;args=sys.argv[1:];fault=os.environ['FAULT']
target=Path(os.environ['TARGET']);helper=Path(os.environ['HELPER_TARGET'])
def digest(path):
 return hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None
entry={'tool':name,'args':args}
if name in ('cp','mv'):
 source=Path(args[-2]);dest=Path(args[-1]);destdir=dest if dest.is_dir() else dest.parent
 entry.update(source_device=source.stat().st_dev,destination_device=destdir.stat().st_dev,
              source_sha256=digest(source),live_sha256=digest(target),helper_sha256=digest(helper),
              stage_mode=source.parent.stat().st_mode&0o777)
with open(os.environ['CALLS'],'a') as out:out.write(json.dumps(entry)+'\\n')
is_copy=name=='cp' and Path(args[-2]).name.startswith('tmp.')
is_old_move=name=='mv' and Path(args[-2]).parent==Path(os.environ['TMPDIR'])
is_new_move=name=='mv' and Path(args[-2]).parent.name.startswith('.pulse-agent-stage.')
is_helper=(name in ('cp','mv') and ('helper' in args[-1] or 'helper' in args[-2]))
if fault=='copy-limit' and (is_copy or is_old_move):
 resource.setrlimit(resource.RLIMIT_CORE,(0,0));resource.setrlimit(resource.RLIMIT_FSIZE,(1024,1024))
if fault=='helper-copy-limit' and is_copy and is_helper:
 resource.setrlimit(resource.RLIMIT_CORE,(0,0));resource.setrlimit(resource.RLIMIT_FSIZE,(1024,1024))
if fault=='rename-refused' and (is_old_move or is_new_move):sys.exit(74)
if fault=='helper-rename-refused' and is_new_move and is_helper:sys.exit(74)
if fault=='mode-refused' and name=='chmod' and '.pulse-agent-stage.' in args[-1]:sys.exit(75)
if fault=='stage-allocation-refused' and name=='mktemp' and '-d' in args:sys.exit(73)
real=json.loads(os.environ['REAL_TOOLS'])[name]
if fault=='term-after-copy' and is_copy:
 p=subprocess.run([real]+args)
 if p.returncode:sys.exit(p.returncode)
 os.kill(os.getppid(),signal.SIGTERM);sys.exit(0)
if fault=='directory-before-rename' and is_new_move and not is_helper:
 target.rename(target.parent/'saved-previous');target.mkdir()
os.execv(real,[real]+args)
'''
                    for name in ('cp', 'mv', 'chmod', 'mktemp'):
                        (tools / name).write_text(wrapper); (tools / name).chmod(0o700)
                    env = dict(os.environ, PATH=str(tools)+os.pathsep+os.environ['PATH'],
                               TMPDIR=str(download), FAULT=fault, TARGET=str(target), HELPER_TARGET=str(helper_target),
                               CALLS=str(root / 'calls'), REAL_TOOLS=json.dumps(self.tools),
                               FIXTURE=str(root), FIXTURE_INSTALL=str(installed), FIXTURE_HELPER=str(helper_dir),
                               FIXTURE_URL='https://127.0.0.1:'+str(server.server_port),
                               FIXTURE_KEY=self.key, FIXTURE_CA=str(self.root / 'tls.crt'))
                    setup = r'''
set -euo pipefail
TMP_FILES=(); AGENT_STAGING_DIRS=(); AGENT_STAGED_EXECUTABLE=""
SAFE_PROFILE_TRANSACTION_ACTIVE=false
SAFE_PROFILE_ACTION=""; UPDATE_ONLY=false; PRIVILEGED_HELPER_ENABLED="$FIXTURE_HELPER_ENABLED"; ACTION_RUNNER_ENABLED=false
AGENT_NAME=pulse-agent; BINARY_NAME=pulse-agent; INSTALL_DIR="$FIXTURE_INSTALL"
PRIVILEGE_HELPER_DIR="$FIXTURE_HELPER"; PRIVILEGED_HELPER_BINARY_NAME=pulse-agent-helper
PRIVILEGED_HELPER_BINARY_PATH="$PRIVILEGE_HELPER_DIR/$PRIVILEGED_HELPER_BINARY_NAME"
INSECURE=false; CURL_CA_BUNDLE="$FIXTURE_CA"; PULSE_URL="$FIXTURE_URL"
[[ "$FAULT" != untrusted-tls ]] || CURL_CA_BUNDLE=""
NON_INTERACTIVE=true; OUTPUT_FORMAT=text; PULSE_TOKEN=""; TOKEN_FILE_PATH=""
INSTALL_SIGNATURE_IDENTITY=pulse-installer; INSTALL_SIGNATURE_NAMESPACE=pulse-install
PINNED_INSTALLER_SSH_PUBLIC_KEY="$FIXTURE_KEY"
AGENT_MIN_TEMP_FREE_BYTES=$((48*1024*1024)); AGENT_MIN_INSTALL_FREE_BYTES=$((48*1024*1024))
EXIT_GENERAL=1; EXIT_DOWNLOAD_FAILED=11; EXIT_CHECKSUM_FAILED=12; EXIT_PREFLIGHT_FAILED=14; EXIT_SIGNATURE_FAILED=17; EXIT_MISSING_ARGS=16
stop_existing_agent_service() { printf 'stop\n' >> "$FIXTURE/events"; }
pkill() { printf 'kill\n' >> "$FIXTURE/events"; }
sleep() { :; }
chown() { printf 'ownership:%s\n' "$*" >> "$FIXTURE/events"; }
systemctl() { return 1; }
'''
                    env['FIXTURE_HELPER_ENABLED'] = 'true' if helper else 'false'
                    script = setup + self.functions + '\n' + self.traps + '\n' + self.producer + '\nprintf "published\\n" >> "$FIXTURE/events"\n'
                    scriptfile = root / 'producer.sh'; scriptfile.write_text(script)
                    handle = target.open('rb') if target.is_file() else contextlib.nullcontext(None)
                    with handle as old:
                        result = subprocess.run(['bash', str(scriptfile)], env=env, capture_output=True, text=True, timeout=25)
                        old_bytes = old.read() if old else None
                    calls = [json.loads(line) for line in (root / 'calls').read_text().splitlines()] if (root / 'calls').exists() else []
                    events = (root / 'events').read_text().splitlines() if (root / 'events').exists() else []
                    observation = dict(fault=fault, helper=helper, fresh=fresh, target_kind=target_kind,
                        exit=result.returncode, output=result.stdout+result.stderr, requests=requests, calls=calls,
                        events=events, remaining_downloads=sorted(p.name for p in download.iterdir()),
                        remaining_stages=sorted(str(p.relative_to(root)) for p in root.rglob('.pulse-agent-stage.*')),
                        installed_bytes=target.stat().st_size if target.is_file() else None,
                        installed_mode=target.stat().st_mode & 0o777 if target.is_file() else None,
                        target_is_symlink=target.is_symlink(),
                        target_children=sorted(p.name for p in target.iterdir()) if target.is_dir() else [],
                        installed_sha256=hashlib.sha256(target.read_bytes()).hexdigest() if target.is_file() else None,
                        helper_sha256=hashlib.sha256(helper_target.read_bytes()).hexdigest() if helper_target.is_file() else None)
                    OBSERVATIONS.append(observation)
                    self.assertEqual((state / 'token').read_bytes(), state_before)
                    if old_bytes is not None: self.assertEqual(old_bytes, self.previous, 'existing reader changed')
                    if victim.is_file(): self.assertEqual(victim.read_bytes(), self.previous, 'symlink referent changed')
                    return observation, target.read_bytes() if target.is_file() else None, helper_target.read_bytes() if helper_target.is_file() else None
                finally:
                    server.shutdown(); server.server_close(); thread.join(timeout=5)
                    self.assertFalse(thread.is_alive(), 'fixture listener did not stop')

    def assert_clean(self, result):
        self.assertEqual(result['remaining_stages'], [])
        self.assertEqual(result['remaining_downloads'], [])

    def test_cross_filesystem_copy_failure_preserves_old_file(self):
        for fresh in (False, True):
            with self.subTest(fresh=fresh):
                result, live, _ = self.exercise('copy-limit', fresh=fresh)
                self.assertNotEqual(result['exit'], 0, result)
                self.assertEqual(live, None if fresh else self.previous, result)
                self.assertNotIn('stop', result['events'], result)
                self.assert_clean(result)

    def test_success_publishes_one_complete_same_filesystem_file(self):
        for helper, fresh, kind in ((False, False, 'file'), (False, True, 'file'),
                                    (True, False, 'file'), (False, False, 'symlink-file')):
            with self.subTest(helper=helper, fresh=fresh, target=kind):
                result, live, installed_helper = self.exercise(helper=helper, fresh=fresh, target_kind=kind)
                self.assertEqual(result['exit'], 0, result)
                self.assertEqual(live, self.candidate)
                self.assertEqual(result['installed_mode'], 0o755)
                self.assertFalse(result['target_is_symlink'])
                if helper: self.assertEqual(installed_helper, self.candidate)
                moves = [c for c in result['calls'] if c['tool'] == 'mv']
                self.assertEqual(len(moves), 2 if helper else 1)
                for move in moves:
                    self.assertEqual(move['source_device'], move['destination_device'])
                    self.assertEqual(move['source_sha256'], hashlib.sha256(self.candidate).hexdigest())
                    self.assertEqual(move['stage_mode'], 0o700)
                self.assert_clean(result)

    def test_preparation_failures_do_not_stop_or_replace_the_collector(self):
        for fault, helper in (('stage-allocation-refused', False), ('mode-refused', False),
                              ('term-after-copy', False), ('helper-copy-limit', True)):
            with self.subTest(fault=fault):
                result, live, installed_helper = self.exercise(fault, helper=helper)
                self.assertNotEqual(result['exit'], 0, result)
                self.assertEqual(live, self.previous, result)
                if helper: self.assertEqual(installed_helper, self.previous, result)
                self.assertNotIn('stop', result['events'], result)
                self.assertEqual(result['target_children'], [])
                self.assertNotIn('published', result['events'], result)
                self.assert_clean(result)

    def test_publication_refusal_retains_complete_previous_binary(self):
        for fault in ('rename-refused', 'helper-rename-refused', 'directory-before-rename'):
            with self.subTest(fault=fault):
                result, live, installed_helper = self.exercise(fault, helper=fault=='helper-rename-refused')
                self.assertNotEqual(result['exit'], 0, result)
                self.assertNotIn('published', result['events'], result)
                if fault == 'rename-refused': self.assertEqual(live, self.previous, result)
                if fault == 'helper-rename-refused':
                    self.assertEqual(live, self.candidate); self.assertEqual(installed_helper, self.previous, result)
                self.assert_clean(result)

    def test_unsafe_destination_does_not_absorb_the_agent(self):
        for kind in ('directory', 'symlink-directory'):
            with self.subTest(target=kind):
                result, _, _ = self.exercise(target_kind=kind)
                self.assertNotEqual(result['exit'], 0, result)
                self.assertNotIn('published', result['events'], result)
                self.assertNotIn('stop', result['events'], result)
                self.assertEqual(result['target_children'], [])
                self.assert_clean(result)

    def test_signature_checksum_and_tls_refusals_precede_host_changes(self):
        for fault in ('untrusted-tls', 'changed-bytes', 'checksum-missing', 'signature-missing', 'bad-signature'):
            with self.subTest(fault=fault):
                result, live, _ = self.exercise(fault)
                self.assertNotEqual(result['exit'], 0, result)
                self.assertEqual(live, self.previous, result)
                self.assertEqual(result['events'], [], result)
                self.assertFalse(any(c['tool'] in ('cp', 'mv') for c in result['calls']))
                self.assert_clean(result)


if __name__ == '__main__':
    try:
        unittest.main(verbosity=2)
    finally:
        print('PUBLICATION_OBSERVATIONS=' + json.dumps(OBSERVATIONS, sort_keys=True), flush=True)
