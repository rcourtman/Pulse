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

BASE = '9168d16a32e948665eba80e2899604933292cdc0'
CANDIDATE = 'dd388decf5896123b6587b1b16f9aee7c3a747f3'
EVIDENCE_CANDIDATE = '4cdf250f474ad9cde6ffb121527f021b7387603f7'
TREES = {BASE: '439848bb1d62a8ccf743b923eef89f8b47369edf',
         CANDIDATE: '1120cce4d631bca5525cafc5b5620251998773f7'}
SOURCE_HASHES = {
    'http_metrics.go': '916c27ad5ff07e00170dd94df0b75eb509dd329e34288647f5358a0759353d37',
    'http_metrics_bench_test.go': '1562ae806264d8b22aece08fdadb7284760ae2374864479c7225b5051b5ba6b2',
}
LABELS = ('base', 'candidate')
SYMBOLS = r'api\.(normalizeRoute|BenchmarkNormalizeRoute)'
BENCH = '^BenchmarkNormalizeRoute$/^root$'
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


def order(round_number):
    return LABELS if round_number % 2 else tuple(reversed(LABELS))


def bench_args(binary, warmup=False):
    return [str(binary), '-test.run=^$', '-test.bench=' + BENCH,
            '-test.cpu=4', '-test.benchmem', '-test.count=1',
            '-test.benchtime=' + ('100ms' if warmup else '1s'), '-test.timeout=2m']


def validate_sample(sample):
    rows = [line for line in sample.splitlines() if line.startswith('Benchmark')]
    if len(rows) != 1 or not re.fullmatch(
            r'BenchmarkNormalizeRoute/root-4\s+[1-9][0-9]*\s+[0-9.]+ ns/op\s+'
            r'[0-9]+ B/op\s+[0-9]+ allocs/op', rows[0]):
        raise ValueError('missing, duplicate, or out-of-scope benchmark sample')


def telemetry():
    result = {'affinity': sorted(os.sched_getaffinity(0))}
    for name in ('/proc/pressure/cpu', '/sys/fs/cgroup/cpu.stat', '/proc/self/cgroup'):
        try:
            result[name] = Path(name).read_text()
        except OSError as exc:
            result[name] = {'unavailable': type(exc).__name__}
    return result


def execute(repo, output):
    output.mkdir(parents=True, exist_ok=False)  # never overwrite a previous attempt
    os.environ.update(GOTOOLCHAIN='local', GOMAXPROCS='4', GOFLAGS='-mod=readonly', GOENV='off', GOAMD64='v1', CGO_ENABLED='1')
    if run(['go', 'version']).strip() != 'go version go1.26.8 linux/amd64':
        raise ValueError('requires exact hosted experiment toolchain go1.26.8 linux/amd64')
    metadata = {
        'purpose': 'diagnostic-only; no gate disposition or release candidate',
        'rounds': ROUNDS, 'gomaxprocs': 4, 'benchtime': '1s',
        'original_evidence_candidate': EVIDENCE_CANDIDATE,
        'workflow_sha': os.environ.get('GITHUB_SHA'),
        'run_id': os.environ.get('GITHUB_RUN_ID'),
        'run_attempt': os.environ.get('GITHUB_RUN_ATTEMPT'),
        'conditions': {}, 'complete': False,
    }

    def save():
        (output / 'metadata.json').write_text(json.dumps(metadata, indent=2) + '\n')

    affinity = sorted(os.sched_getaffinity(0))[:4]
    if len(affinity) != 4:
        raise ValueError('requires four available CPUs')
    os.sched_setaffinity(0, affinity)
    metadata['affinity'] = affinity
    save()
    capture(['go', 'version'], output / 'go-version.txt')
    capture(['go', 'env', 'GOOS', 'GOARCH', 'GOVERSION', 'GOTOOLCHAIN', 'GOAMD64',
             'CGO_ENABLED', 'GOFLAGS'], output / 'go-env.txt')
    capture(['lscpu'], output / 'cpu.txt')
    capture(['uname', '-smr'], output / 'kernel.txt')
    # Both builds complete before warm-up/measurement; no compilation drift.
    with tempfile.TemporaryDirectory(prefix='root-pair-') as temporary:
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
            # Exact unmodified source; no layout intervention.
            (output / (label + '-http_metrics.go')).write_text(metrics.read_text())
            stub = source / 'internal/api/frontend-modern/dist/index.html'
            stub.parent.mkdir(parents=True, exist_ok=True)
            stub.write_text('<!doctype html><title>ci embed stub</title>\n')
            binary = work / (label + '.test')
            capture(['go', 'test', '-c', '-o', str(binary), './internal/api'],
                    output / (label + '-build.txt'), source)
            capture(['go', 'version', '-m', str(binary)],
                    output / (label + '-build-info.txt'))
            binaries[label] = binary
            metadata['conditions'][label] = {
                'commit': revision, 'tree': tree, 'binary_sha256': digest(binary),
                'experimental': False,
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
        import time
        time.sleep(10)
        for round_number in range(1, ROUNDS + 1):
            for label in order(round_number):
                with (output / 'order.jsonl').open('a') as log:
                    log.write(json.dumps({'round': round_number, 'condition': label,
                                          'at': datetime.now(timezone.utc).isoformat(),
                                          'load': os.getloadavg(),
                                          'telemetry': telemetry()}) + '\n')
                sample_path = output / f'{label}-{round_number:02}.txt'
                capture(bench_args(binaries[label]), sample_path)
                sample = sample_path.read_text()
                validate_sample(sample)
                with (output / 'ends.jsonl').open('a') as log:
                    log.write(json.dumps({'round': round_number, 'condition': label,
                                          'at': datetime.now(timezone.utc).isoformat(),
                                          'telemetry': telemetry()}) + '\n')
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
