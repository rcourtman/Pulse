#!/usr/bin/env python3
"""Packaging's normal smoke-suite controls; no Docker daemon is used here."""
import base64
import contextlib
import gzip
import hashlib
import importlib.util
import io
import json
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('packaging', ROOT/'scripts/test-container-packaging.py')
packaging = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packaging)


def archive(files):
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode='w') as tar:
        for name, data, kind in files:
            member = tarfile.TarInfo(name)
            member.uid = member.gid = 1000
            member.mode = 0o755
            if kind == 'file':
                member.size = len(data)
                tar.addfile(member, io.BytesIO(data))
            else:
                member.type = tarfile.SYMTYPE
                member.linkname = data
                tar.addfile(member)
    return stream.getvalue()


class InventoryTests(unittest.TestCase):
    def test_bytes_metadata_and_links_are_retained_without_extraction(self):
        blob = archive([('app/pulse', b'actual-server', 'file'),
                        ('opt/pulse/bin/agent', b'agent', 'file'),
                        ('usr/local/bin/pulse-agent', '/opt/pulse/bin/agent', 'link'),
                        ('etc/hostname', b'ephemeral', 'file')])
        result = packaging.inventory(io.BytesIO(blob))
        self.assertEqual(set(result), {'app/pulse', 'opt/pulse/bin/agent', 'usr/local/bin/pulse-agent'})
        self.assertEqual(result['app/pulse']['sha256'], hashlib.sha256(b'actual-server').hexdigest())
        self.assertEqual(result['app/pulse']['uid'], 1000)
        self.assertEqual(result['app/pulse']['mode'], 0o755)
        self.assertEqual(result['usr/local/bin/pulse-agent']['link'], '/opt/pulse/bin/agent')

    def test_duplicate_exported_paths_are_not_silently_overwritten(self):
        with self.assertRaisesRegex(ValueError, 'duplicate exported path'):
            packaging.inventory(io.BytesIO(archive([('app/pulse', b'a', 'file')]*2)))

    def test_layer_inventory_counts_repeated_payload_and_explicit_reencoding(self):
        layer = archive([('app/pulse', b'server', 'file'), ('opt/pulse/bin/agent', b'agent', 'file')])
        image = archive([('one/layer.tar', layer, 'file'), ('two/layer.tar', layer, 'file'),
                         ('manifest.json', json.dumps([{'Layers':['one/layer.tar','two/layer.tar']}]).encode(), 'file')])
        measured = packaging.layers(io.BytesIO(image))
        self.assertEqual(len(measured), 2)
        self.assertEqual(sum(l['uncompressed_tar_bytes'] for l in measured), len(layer)*2)
        self.assertTrue(all(0 < l['reencoded_gzip_bytes'] < len(layer) for l in measured))
        packaging.no_payload_copyups(measured[:1])
        with self.assertRaisesRegex(ValueError, 'more than one layer'):
            packaging.no_payload_copyups(measured)

    def test_absent_layer_or_payload_cannot_pass(self):
        with self.assertRaisesRegex(ValueError, 'manifest'):
            packaging.layers(io.BytesIO(archive([])))
        with self.assertRaisesRegex(ValueError, 'payload missing'):
            packaging.no_payload_copyups([])

    def test_containerd_blob_paths_and_gzip_measure_uncompressed_bytes(self):
        layer = archive([('app/pulse', b'server', 'file'), ('opt/pulse/bin/agent', b'agent', 'file')])
        blob = gzip.compress(layer)
        path = 'blobs/sha256/'+hashlib.sha256(blob).hexdigest()
        image = archive([(path, blob, 'file'), ('unreferenced/layer.tar', layer, 'file'),
                         ('manifest.json', json.dumps([{'Layers':[path]}]).encode(), 'file')])
        measured = packaging.layers(io.BytesIO(image))
        self.assertEqual(len(measured), 1)
        self.assertEqual(measured[0]['archive_blob_bytes'], len(blob))
        self.assertEqual(measured[0]['uncompressed_tar_bytes'], len(layer))
        packaging.no_payload_copyups(measured)

    def test_manifest_identity_missing_duplicate_and_multiimage_fail_closed(self):
        for manifest in [[{'Layers':['missing']}], [{'Layers':['same','same']}],
                         [{'Layers':[]}], [{'Layers':['a']},{'Layers':['b']}], {}]:
            with self.subTest(manifest=manifest), self.assertRaises(ValueError):
                packaging.layers(io.BytesIO(archive([('manifest.json', json.dumps(manifest).encode(), 'file')])))

    def test_every_compatibility_dimension_is_checked(self):
        baseline = packaging.inventory(io.BytesIO(archive([('app/pulse', b'server', 'file')])))
        for key, value in [('sha256', 'different'), ('mode', 0o644), ('uid', 0),
                           ('gid', 0), ('link', 'wrong'), ('size', 0), ('type', '2')]:
            with self.subTest(key=key), self.assertRaisesRegex(ValueError, 'changed final payload'):
                changed = json.loads(json.dumps(baseline))
                changed['app/pulse'][key] = value
                packaging.compare(baseline, changed)
        with self.assertRaises(ValueError):
            packaging.compare(baseline, {})

    def test_process_failure_cannot_become_a_successful_inventory(self):
        with self.assertRaises(subprocess.CalledProcessError):
            packaging.stream_command(['sh', '-c', 'printf partial; exit 19'], lambda f: f.read())


class DownloadConsumerTests(unittest.TestCase):
    def test_actual_consumer_checks_every_body_and_both_sidecars(self):
        # Execute the exact in-container consumer, substituting only its file
        # root and HTTP transport. This is not a real-image/startup result.
        for fault in ['', 'body', 'checksum', 'ed25519', 'sshsig', 'sidecar-mode']:
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                linux = ['linux-'+a for a in ['amd64','arm64','armv7','armv6','386']]
                files = {}
                for family, arches in [('pulse-agent', linux+['darwin-amd64','darwin-arm64',
                        'freebsd-amd64','freebsd-arm64','windows-amd64','windows-arm64','windows-386']),
                        ('pulse-agent-helper', linux), ('pulse-agent-runner', linux)]:
                    for arch in arches:
                        name = family+'-'+arch+('.exe' if arch.startswith('windows-') else '')
                        files['/download/'+family+'?arch='+arch] = root/'bin'/name
                for name in ['install.sh','install.ps1']:
                    files['/'+name] = root/'scripts'/name
                for path in files.values():
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(path.name.encode())
                    path.chmod(0o755)
                    for suffix in ['.sig','.sshsig']:
                        sidecar = Path(str(path)+suffix)
                        sidecar.write_bytes(('signature-'+path.name+suffix).encode())
                        sidecar.chmod(0o755 if fault == 'sidecar-mode' else 0o644)
                called = []
                def fetch(url, timeout):
                    route = url.removeprefix('http://127.0.0.1:7655')
                    path = files[route]
                    called.append(route)
                    response = io.BytesIO(path.read_bytes() if fault != 'body' else b'wrong')
                    response.headers = {
                        'X-Checksum-Sha256': hashlib.sha256(path.read_bytes()).hexdigest() if fault != 'checksum' else 'wrong',
                        'X-Signature-Ed25519': Path(str(path)+'.sig').read_text() if fault != 'ed25519' else 'wrong',
                        'X-Signature-SSHSIG': base64.b64encode(Path(str(path)+'.sshsig').read_bytes()).decode() if fault != 'sshsig' else 'd3Jvbmc=',
                    }
                    return response
                code = packaging.DOWNLOAD_CHECK.replace("'/opt/pulse'", repr(str(root)))
                capture = io.StringIO()
                with patch('urllib.request.urlopen', side_effect=fetch), contextlib.redirect_stdout(capture):
                    if fault:
                        with self.assertRaises(AssertionError):
                            exec(compile(code, '<packaging-download-consumer>', 'exec'), {})
                    else:
                        exec(compile(code, '<packaging-download-consumer>', 'exec'), {})
                        self.assertEqual(set(called), set(files))
                        self.assertEqual(len(called), 24)
                        self.assertEqual(len(json.loads(capture.getvalue())), 24)


class DockerfileTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        text = (ROOT/'Dockerfile').read_text().replace('\\\n', '')
        cls.stages = {}
        current = None
        for line in text.splitlines():
            if line.startswith('FROM '):
                current = line.split(' AS ')[1]
                cls.stages[current] = []
            elif current and line.strip() and not line.startswith('#'):
                cls.stages[current].append(line)

    def test_final_server_copies_assign_metadata_before_any_runtime_layer(self):
        for stage in ['pulse-runtime-base', 'prebuilt-runtime-base']:
            commands = self.stages[stage]
            pulse = next(c for c in commands if c.startswith('COPY ') and c.endswith(' ./pulse'))
            self.assertIn('--chown=pulse:pulse', pulse)
            self.assertIn('--chmod=0755', pulse)
            self.assertFalse(any(c.startswith('RUN ') for c in commands))

    def test_all_source_payload_copies_have_final_ownership_not_recursive_copyup(self):
        commands = self.stages['runtime']
        for copy in (c for c in commands if c.startswith('COPY ')):
            self.assertIn('--chown=pulse:pulse', copy)
            if copy.endswith(('.sh', '.ps1')):
                self.assertIn('--chmod=0755', copy)
        text = '\n'.join(commands)
        self.assertNotIn('chown -R', text)
        self.assertIn('-type l -exec chown -h pulse:pulse {} +', text)

    def test_final_candidate_copy_is_single_owned_payload_and_links_only(self):
        commands = self.stages['runtime_prebuilt']
        copies = [c for c in commands if c.startswith('COPY ')]
        self.assertEqual(copies, ['COPY --from=prebuilt-downloads --chown=pulse:pulse /payload/ /opt/pulse/'])
        text = '\n'.join(commands)
        self.assertNotIn('chmod', text)
        self.assertNotIn('chown -R', text)
        self.assertIn('-type l -exec chown -h pulse:pulse {} +', text)
        for arch in ['amd64', 'arm64', '386']:
            for suffix in ['', '.sig', '.sshsig']:
                self.assertIn(f'pulse-agent-windows-{arch}.exe{suffix}', text)

    def test_actual_staging_recipe_preserves_every_byte_and_nonexecutable_sidecar(self):
        recipe = next(c[4:] for c in self.stages['prebuilt-downloads'] if c.startswith('RUN '))
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'payload'
            (root/'bin').mkdir(parents=True)
            (root/'scripts').mkdir()
            files = {}
            for family, arches in [('pulse-agent', ['linux-amd64', 'linux-arm64', 'linux-armv7',
                 'linux-armv6', 'linux-386', 'darwin-amd64', 'darwin-arm64', 'windows-amd64.exe',
                 'windows-arm64.exe', 'windows-386.exe', 'freebsd-amd64', 'freebsd-arm64']),
                 ('pulse-agent-helper', ['linux-'+a for a in ['amd64','arm64','armv7','armv6','386']]),
                 ('pulse-agent-runner', ['linux-'+a for a in ['amd64','arm64','armv7','armv6','386']])]:
                for arch in arches:
                    for suffix in ['', '.sig', '.sshsig']:
                        files[root/'bin'/f'{family}-{arch}{suffix}'] = suffix == ''
            for arch in ['amd64', 'arm64']:
                files[root/'bin'/f'pulse-linux-{arch}'] = True
            for script in ['install.sh','install.ps1','install-container-agent.sh','install-docker.sh']:
                files[root/'scripts'/script] = True
                if script in ['install.sh','install.ps1']:
                    for suffix in ['.sig','.sshsig']:
                        files[root/'scripts'/(script+suffix)] = False
            for path, executable in files.items():
                path.write_bytes(path.name.encode())
                path.chmod(0o755 if path.name.startswith('pulse-linux-') else 0o644)
            before = {p: p.read_bytes() for p in files}
            subprocess.run(['sh', '-ec', recipe.replace('/payload', str(root))], check=True)
            for path, executable in files.items():
                with self.subTest(path=path.name):
                    self.assertEqual(path.read_bytes(), before[path])
                    self.assertEqual(path.stat().st_mode & 0o777, 0o755 if executable else 0o644)


if __name__ == '__main__':
    unittest.main()
