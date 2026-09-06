#!/usr/bin/env python3
"""One fixed diagnostic experiment; never a release qualification verdict."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
from datetime import datetime, timezone

BASE = '5845692daf302a1524c2ca8fe6d06339ae7f8a24'
CANDIDATE = '57f3b6401a6553efaa7646d5f1db446a2725b9dc'
TREES = {BASE: 'dedef52efe4fc28d1c05c9d957a0cc439f54db22',
         CANDIDATE: '7c2765cb06f424ce49647b33a5ebf3ade375e828'}
SOURCE_HASHES = {
    'http_metrics.go': '81fc091698cf7c1b011af70776dad53db3aa735367e51f4cda8a9bbf4b2b8527',
    'http_metrics_bench_test.go': '1562ae806264d8b22aece08fdadb7284760ae2374864479c7225b5051b5ba6b2',
}
LABELS = ('base', 'candidate', 'base-reordered', 'candidate-reordered')
SYMBOLS = r'api\.(normalizeSegment|BenchmarkNormalizeSegment)'
BENCH = '^BenchmarkNormalizeSegment$/^uuid$'
ROUNDS = 10


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def run(args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True, stderr=subprocess.STDOUT)


def capture(args, output, cwd=None):
    # Preserve partial diagnostics on failure as well as successful output.
    with output.open('w') as stream:
        subprocess.run(args, cwd=cwd, stdout=stream, stderr=subprocess.STDOUT, check=True)


def reorder(source):
    """Same declaration-only intervention as Core's retained controlled run."""
    for marker in ('func normalizeSegment(', 'func isNumeric(', 'func normalizeRoute('):
        if source.count(marker) != 1:
            raise ValueError('unexpected declaration layout')
    start = source.index('func normalizeSegment(')
    end = source.index('func isNumeric(', start)
    block = source[start:end]
    remainder = source[:start] + source[end:]
    index = remainder.index('func normalizeRoute(')
    return remainder[:index] + block + remainder[index:]


def order(round_number):
    return LABELS if round_number % 2 else tuple(reversed(LABELS))


def bench_args(binary, warmup=False):
    return [str(binary), '-test.run=^$', '-test.bench=' + BENCH,
            '-test.cpu=4', '-test.benchmem', '-test.count=1',
            '-test.benchtime=' + ('1x' if warmup else '300ms'), '-test.timeout=2m']


def validate_sample(sample):
    rows = [line for line in sample.splitlines() if line.startswith('Benchmark')]
    if len(rows) != 1 or not re.fullmatch(
            r'BenchmarkNormalizeSegment/uuid-4\s+[1-9][0-9]*\s+[0-9.]+ ns/op\s+'
            r'[0-9]+ B/op\s+[0-9]+ allocs/op', rows[0]):
        raise ValueError('missing, duplicate, or out-of-scope benchmark sample')


def execute(repo, output):
    output.mkdir(parents=True, exist_ok=False)  # never overwrite a previous attempt
    os.environ.update(GOTOOLCHAIN='local', GOMAXPROCS='4', GOFLAGS='', GOENV='off')
    if run(['go', 'version']).strip() != 'go version go1.26.7 linux/amd64':
        raise ValueError('requires exact hosted experiment toolchain go1.26.7 linux/amd64')
    metadata = {
        'purpose': 'diagnostic-only; no gate disposition or release candidate',
        'rounds': ROUNDS, 'gomaxprocs': 4, 'benchtime': '300ms',
        'workflow_sha': os.environ.get('GITHUB_SHA'),
        'run_id': os.environ.get('GITHUB_RUN_ID'),
        'run_attempt': os.environ.get('GITHUB_RUN_ATTEMPT'),
        'conditions': {}, 'complete': False,
    }

    def save():
        (output / 'metadata.json').write_text(json.dumps(metadata, indent=2) + '\n')

    save()
    capture(['go', 'version'], output / 'go-version.txt')
    capture(['go', 'env', 'GOOS', 'GOARCH', 'GOVERSION', 'GOTOOLCHAIN', 'GOAMD64',
             'CGO_ENABLED', 'GOFLAGS'], output / 'go-env.txt')
    capture(['lscpu'], output / 'cpu.txt')
    capture(['uname', '-smr'], output / 'kernel.txt')
    # All four builds complete before warm-up/measurement; no compilation drift.
    with tempfile.TemporaryDirectory(prefix='uuid-layout-') as temporary:
        work = Path(temporary)
        binaries = {}
        for label in LABELS:
            revision = CANDIDATE if label.startswith('candidate') else BASE
            tree = run(['git', 'rev-parse', revision + '^{tree}'], repo).strip()
            if tree != TREES[revision]:
                raise ValueError('unexpected source tree')
            source = work / label
            source.mkdir()
            archive = work / (label + '.tar')
            subprocess.run(['git', 'archive', '-o', str(archive), revision], cwd=repo, check=True)
            subprocess.run(['tar', '-xf', str(archive), '-C', str(source)], check=True)
            for filename, expected in SOURCE_HASHES.items():
                if digest(source / 'internal/api' / filename) != expected:
                    raise ValueError('unexpected HTTP source content')
            metrics = source / 'internal/api/http_metrics.go'
            original = metrics.read_text()
            if label.endswith('-reordered'):
                metrics.write_text(reorder(original))
            # Retain the exact intervention, not the experimental executable.
            (output / (label + '-http_metrics.go')).write_text(metrics.read_text())
            stub = source / 'internal/api/frontend-modern/dist/index.html'
            stub.parent.mkdir(parents=True, exist_ok=True)
            stub.write_text('<!doctype html><title>ci embed stub</title>\n')
            binary = work / (label + '.test')
            capture(['go', 'test', '-c', '-o', str(binary), './internal/api'],
                    output / (label + '-build.txt'), source)
            binaries[label] = binary
            metadata['conditions'][label] = {
                'commit': revision, 'tree': tree, 'binary_sha256': digest(binary),
                'experimental': label.endswith('-reordered'),
                'http_metrics_sha256': digest(metrics), 'embed_stub_sha256': digest(stub),
            }
            save()
            capture(['go', 'tool', 'objdump', '-s', SYMBOLS, str(binary)],
                    output / (label + '-http.asm'))
            # Restrict retained symbols to the HTTP normaliser experiment.
            symbols = run(['go', 'tool', 'nm', str(binary)])
            (output / (label + '-symbols.txt')).write_text('\n'.join(
                line for line in symbols.splitlines() if re.search(SYMBOLS, line)) + '\n')
        for label in LABELS:
            capture(bench_args(binaries[label], warmup=True), output / (label + '-warmup.txt'))
        for round_number in range(1, ROUNDS + 1):
            for label in order(round_number):
                with (output / 'order.jsonl').open('a') as log:
                    log.write(json.dumps({'round': round_number, 'condition': label,
                                          'at': datetime.now(timezone.utc).isoformat(),
                                          'load': os.getloadavg()}) + '\n')
                sample_path = output / f'{label}-{round_number:02}.txt'
                capture(bench_args(binaries[label]), sample_path)
                sample = sample_path.read_text()
                validate_sample(sample)
                with (output / (label + '.txt')).open('a') as aggregate:
                    aggregate.write(sample)
        metadata['complete'] = True
        save()
    files = sorted(path for path in output.iterdir() if path.is_file())
    (output / 'SHA256SUMS').write_text(''.join(f'{digest(path)}  {path.name}\n' for path in files))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    execute(Path(__file__).resolve().parents[1], args.output.resolve())
