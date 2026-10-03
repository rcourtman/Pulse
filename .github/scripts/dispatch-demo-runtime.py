#!/usr/bin/env python3
"""The two existing demo CI paths share one retained, independently owned child."""
from __future__ import annotations

import base64
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys

BOOTSTRAP = r'''
import base64, fcntl, hashlib, json, os, pathlib, subprocess, sys, time
root = pathlib.Path('/var/lib/pulse-deploy/demo')
payload = json.loads(sys.stdin.read(512 * 1024))
assert os.geteuid() == 0 and set(payload) == {'request', 'source', 'installer'}
source = base64.b64decode(payload['source'], validate=True)
installer = base64.b64decode(payload['installer'], validate=True)
assert len(source) < 128 * 1024 and len(installer) < 256 * 1024
assert (hashlib.sha256(installer).hexdigest() == payload['request']['installer_sha256']) if installer else (payload['request']['installer_sha256'] == '')
identity = hashlib.sha256(json.dumps(payload, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
root.mkdir(mode=0o700, parents=True, exist_ok=True)
assert not root.is_symlink() and root.stat().st_uid == 0
os.chmod(root, 0o700)
(root / 'attempts').mkdir(mode=0o700, exist_ok=True)
assert not (root / 'attempts').is_symlink()
fd = os.open(root / 'submit.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
fcntl.flock(fd, fcntl.LOCK_EX)
attempt = root / 'attempts' / identity
if not attempt.exists():
    attempt.mkdir(mode=0o700)
    for name, data in [('transaction.py', source), ('installer.sh', installer),
                       ('request.json', json.dumps(payload['request']).encode()),
                       ('intent.json', json.dumps({'schema_version': 1, 'identity': identity, 'state': 'intent'}).encode())]:
        with open(attempt / name, 'xb') as stream:
            os.chmod(attempt / name, 0o600)
            stream.write(data); stream.flush(); os.fsync(stream.fileno())
    # Intent is durable before starting the unit. An uncertain launch is never replayed.
    dfd = os.open(attempt, os.O_DIRECTORY); os.fsync(dfd); os.close(dfd)
    try:
        child = subprocess.run(['systemd-run', '--quiet', '--collect',
                                '--unit=pulse-demo-' + identity[:32],
                                '--property=RuntimeMaxSec=45min',
                                '--property=TimeoutStopSec=20min', '--property=KillMode=mixed',
                                '/usr/bin/python3', str(attempt / 'transaction.py'), str(attempt)],
                               capture_output=True, timeout=30)
        launch = {'schema_version': 1, 'state': 'submitted' if child.returncode == 0 else 'uncertain',
                  'unit': 'pulse-demo-' + identity[:32]}
    except (OSError, subprocess.TimeoutExpired):
        launch = {'schema_version': 1, 'state': 'uncertain'}
    with open(attempt / 'launch.json', 'x') as stream:
        os.chmod(attempt / 'launch.json', 0o600)
        json.dump(launch, stream); stream.flush(); os.fsync(stream.fileno())
os.close(fd)
# Closing or losing this observer never stops the independently owned unit.
deadline = time.monotonic() + 4000
terminal = {'committed', 'healthy_noop', 'rolled_back', 'rollback_failed', 'recovery_required', 'refused'}
while time.monotonic() < deadline:
    try:
        receipt = json.loads((attempt / 'receipt.json').read_text())
    except (FileNotFoundError, ValueError):
        receipt = None
    if receipt and receipt.get('status') in terminal:
        receipt['request_identity'] = identity
        print(json.dumps(receipt))
        sys.exit(0 if receipt['status'] in {'committed', 'healthy_noop'} else 1)
    launch = attempt / 'launch.json'
    if launch.exists() and json.loads(launch.read_text()).get('state') == 'uncertain':
        break
    time.sleep(5)
print(json.dumps({'schema_version': 2, 'status': 'uncertain', 'request_identity': identity,
                  'failure': 'terminal-receipt-unavailable', 'recovery_required': True}))
sys.exit(1)
'''


def payload(mode, environ=os.environ):
    profile = {key: environ['MOCK_' + key.upper()] for key in
               ('nodes', 'vms_per_node', 'lxcs_per_node', 'docker_hosts', 'docker_containers',
                'generic_hosts', 'k8s_clusters', 'k8s_nodes', 'k8s_pods', 'k8s_deployments',
                'seed_duration', 'sample_interval', 'update_interval')}
    installer = Path('/tmp/pulse-install.sh').read_bytes() if mode == 'update' else b''
    request = {'mode': mode, 'hostname': environ['DEMO_EXPECTED_HOSTNAME'],
               'local_url': environ['DEMO_LOCAL_BASE_URL'], 'public_url': environ['DEMO_PUBLIC_HEALTH_URL'],
               'version': environ['EXPECTED_VERSION'], 'profile': profile,
               'control_sha': environ['GITHUB_SHA'], 'run_id': environ['GITHUB_RUN_ID'],
               'run_attempt': environ['GITHUB_RUN_ATTEMPT'],
               'installer_sha256': hashlib.sha256(installer).hexdigest() if installer else ''}
    return {'request': request, 'source': base64.b64encode(Path(__file__).with_name('demo-runtime-transaction.py').read_bytes()).decode(),
            'installer': base64.b64encode(installer).decode()}


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in {'update', 'recover'}:
        return 2
    # Same project CI SSH identity and pinned host verification as before. This
    # is not an installed worker dispatcher or a new project credential.
    command = ['ssh', '-i', str(Path.home() / '.ssh/id_ed25519'), '-o', 'IdentitiesOnly=yes',
               '-o', 'StrictHostKeyChecking=yes', '-o', 'UserKnownHostsFile=' + str(Path.home() / '.ssh/known_hosts'),
               os.environ['DEMO_SERVER_USER'] + '@' + os.environ['DEMO_SERVER_HOST'],
               'sudo python3 -c ' + shlex.quote(BOOTSTRAP)]
    try:
        result = subprocess.run(command, input=json.dumps(payload(sys.argv[1])).encode(), timeout=4100, check=False)
    except (OSError, subprocess.TimeoutExpired):
        print(json.dumps({'schema_version': 2, 'status': 'uncertain', 'failure': 'observer-unavailable', 'recovery_required': True}))
        return 1
    return result.returncode


if __name__ == '__main__':
    sys.exit(main())
