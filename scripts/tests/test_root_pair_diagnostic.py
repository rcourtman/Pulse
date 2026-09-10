#!/usr/bin/env python3
"""No real build or benchmark: verify the diagnostic's bounds and receipts."""
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('diagnostic', ROOT / 'scripts/diagnose-root-pair.py')
diag = importlib.util.module_from_spec(spec)
spec.loader.exec_module(diag)
SOURCE = ('package api\n\nfunc normalizeRoute() {}\n\n'
          'func normalizeSegment() {}\n\nfunc isNumeric() {}\n')
SAMPLE = 'BenchmarkNormalizeRoute/root-4  1000  51.64 ns/op  0 B/op  0 allocs/op\nPASS\n'


class DiagnosticTests(unittest.TestCase):
    def test_ten_balanced_alternating_rounds(self):
        self.assertEqual(diag.ROUNDS, 10)
        for label in diag.LABELS:
            for position in range(2):
                self.assertIn(sum(diag.order(n)[position] == label for n in range(1, 11)), (0, 5))
        self.assertEqual(diag.order(2), tuple(reversed(diag.order(1))))

    def test_execution_is_root_only(self):
        for warmup in (True, False):
            args = diag.bench_args(Path('/tmp/fixture.test'), warmup)
            self.assertIn('-test.run=^$', args)
            self.assertIn('-test.bench=^BenchmarkNormalizeRoute$/^root$', args)
            self.assertIn('-test.cpu=4', args)
            self.assertIn('-test.count=1', args)
            self.assertIn('-test.timeout=2m', args)

    def test_sample_validation_fails_closed(self):
        diag.validate_sample(SAMPLE)
        for bad in ('PASS\n', SAMPLE * 2, SAMPLE.replace('root', 'numeric'),
                    SAMPLE.replace('-4', '-8'), SAMPLE.replace('1000', '0')):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                diag.validate_sample(bad)

    def exercise(self, output, fail_sample=False, bad_tree=False, bad_go=False, bad_source=False):
        captured = []
        hashes = {'http_metrics.go': hashlib.sha256(SOURCE.encode()).hexdigest(),
                  'http_metrics_bench_test.go': hashlib.sha256(b'fixture').hexdigest()}

        def fake_run(args, cwd=None):
            if args == ['go', 'version']:
                return 'go version go1.26.7 linux/amd64' if bad_go else 'go version go1.26.8 linux/amd64'
            if args[:2] == ['git', 'rev-parse']:
                return 'wrong' if bad_tree else diag.TREES[args[2].split('^')[0]]
            if args[:3] == ['go', 'tool', 'nm']:
                return '123 T api.normalizeSegment\n456 T api.BenchmarkNormalizeRoute\n789 T other.symbol'
            raise AssertionError(args)

        def fake_subprocess(args, **kwargs):
            if args[:2] == ['git', 'archive']:
                return
            if args[:2] == ['tar', '-xf']:
                folder = Path(args[-1]) / 'internal/api'
                folder.mkdir(parents=True)
                (folder / 'http_metrics.go').write_text('changed' if bad_source else SOURCE)
                (folder / 'http_metrics_bench_test.go').write_text('fixture')
                return
            raise AssertionError(args)

        def fake_capture(args, path, cwd=None):
            captured.append(args)
            if args[:3] == ['go', 'test', '-c']:
                Path(args[4]).write_bytes(b'fake binary')
                path.write_text('compiled')
            elif args[0].endswith('.test'):
                path.write_text(SAMPLE)
                if fail_sample and '-test.benchtime=1s' in args:
                    raise subprocess.CalledProcessError(1, args)
            else:
                path.write_text('fixture metadata')

        with patch.object(diag, 'run', side_effect=fake_run), \
                patch.object(diag, 'capture', side_effect=fake_capture), \
                patch.object(diag.subprocess, 'run', side_effect=fake_subprocess), \
                patch.object(diag, 'SOURCE_HASHES', hashes), patch.dict(diag.os.environ), \
                patch.object(diag.os, 'sched_getaffinity', return_value={0, 1, 2, 3}), \
                patch.object(diag.os, 'sched_setaffinity'), patch('time.sleep'):
            diag.execute(ROOT, output)
        return captured

    def test_complete_receipt_and_no_uploaded_executables(self):
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / 'evidence'
            calls = self.exercise(output)
            metadata = json.loads((output / 'metadata.json').read_text())
            self.assertTrue(metadata['complete'])
            self.assertEqual(len(metadata['conditions']), 2)
            for label in diag.LABELS:
                self.assertEqual((output / (label + '.txt')).read_text(), SAMPLE * 10)
                self.assertEqual(metadata['conditions'][label]['experimental'], label.endswith('-reordered'))
            orders = [json.loads(line) for line in (output / 'order.jsonl').read_text().splitlines()]
            self.assertEqual([row['condition'] for row in orders],
                             [label for n in range(1, 11) for label in diag.order(n)])
            self.assertEqual(sum(call[:3] == ['go', 'test', '-c'] for call in calls), 2)
            self.assertEqual(sum(call[0].endswith('.test') for call in calls), 22)
            first_benchmark = next(i for i, call in enumerate(calls) if call[0].endswith('.test'))
            self.assertEqual(sum(call[:3] == ['go', 'test', '-c']
                                 for call in calls[:first_benchmark]), 2)
            self.assertEqual(len((output / 'ends.jsonl').read_text().splitlines()), 20)
            self.assertEqual(metadata['affinity'], [0, 1, 2, 3])
            self.assertFalse(list(output.glob('*.test')))
            for line in (output / 'SHA256SUMS').read_text().splitlines():
                digest, name = line.split('  ')
                self.assertEqual(diag.digest(output / name), digest)

    def test_partial_evidence_survives_failure_without_success_claim(self):
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / 'evidence'
            with self.assertRaises(subprocess.CalledProcessError):
                self.exercise(output, fail_sample=True)
            self.assertFalse(json.loads((output / 'metadata.json').read_text())['complete'])
            self.assertTrue((output / 'base-01.txt').exists())
            self.assertFalse((output / 'SHA256SUMS').exists())

    def test_wrong_tree_or_toolchain_rejected(self):
        for option in ('bad_tree', 'bad_go', 'bad_source'):
            with tempfile.TemporaryDirectory() as tmp, self.assertRaises(ValueError):
                self.exercise(Path(tmp) / 'evidence', **{option: True})

    def test_refuses_overwriting_receipt(self):
        with tempfile.TemporaryDirectory() as tmp, self.assertRaises(FileExistsError):
            diag.execute(ROOT, Path(tmp))

    def test_workflow_authority_and_artifact_bounds(self):
        workflow = (ROOT / '.github/workflows/root-pair-diagnostic.yml').read_text()
        self.assertIn('workflow_dispatch:', workflow)
        self.assertNotIn('pull_request:', workflow)
        self.assertNotIn('push:', workflow)
        self.assertIn('contents: read', workflow)
        self.assertNotIn('secrets.', workflow)
        self.assertIn('github.ref == \'refs/heads/main\'', workflow)
        self.assertIn('[[ "$GITHUB_SHA" == "$EXPECTED_SHA" ]]', workflow)
        self.assertIn('[[ "$GITHUB_RUN_ATTEMPT" == 1 ]]', workflow)
        self.assertIn('path: root-pair-evidence/', workflow)
        self.assertIn('if: always()', workflow)
        self.assertIn("go-version: '1.26.8'", workflow)
        self.assertIn('timeout-minutes: 45', workflow)


if __name__ == '__main__':
    unittest.main()
