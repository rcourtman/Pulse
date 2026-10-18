"""Run the workload chart SLO on observed idle capacity and retain raw evidence."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time


def snapshot():
    cpu = list(map(int, Path('/proc/stat').read_text().splitlines()[0].split()[1:]))
    swaps = {}
    for line in Path('/proc/vmstat').read_text().splitlines():
        key, value = line.split()
        if key in ('pswpin', 'pswpout'):
            swaps[key] = int(value)
    pressure = {}
    for kind in ('cpu', 'memory', 'io'):
        path = Path('/proc/pressure') / kind
        if path.exists():
            pressure[kind] = path.read_text().strip()
    return dict(time=time.time(), cpu=cpu, swaps=swaps, load=os.getloadavg(), pressure=pressure)


def interval(before, after):
    delta = [b - a for a, b in zip(before['cpu'], after['cpu'])]
    total = sum(delta[:8])  # Guest time is already included in user/nice time.
    scale = 100 / total if total > 0 else 0
    return dict(time=after['time'], seconds=after['time'] - before['time'],
                total_ticks=total, idle_pct=delta[3] * scale, iowait_pct=delta[4] * scale,
                steal_pct=delta[7] * scale, guest_pct=sum(delta[8:10]) * scale,
                busy_pct=(total - delta[3] - delta[4] - delta[7]) * scale,
                swap_pages=sum(after['swaps'][k] - before['swaps'][k] for k in after['swaps']),
                load=after['load'], pressure=after['pressure'])


def quiet(row):
    return (row['total_ticks'] > 0 and row['load'][0] < 2 and row['idle_pct'] >= 90
            and row['iowait_pct'] <= 1 and row['steal_pct'] <= 0.1
            and row['guest_pct'] <= 2 and row['swap_pages'] == 0)


def wait_idle(prefix):
    deadline = time.monotonic() + 300
    streak = 0
    before = snapshot()
    with prefix.with_suffix('.idle.jsonl').open('w') as receipt:
        while streak < 30:
            time.sleep(5)
            after = snapshot()
            row = interval(before, after)
            streak = streak + row['seconds'] if quiet(row) else 0
            row['quiet_streak_seconds'] = streak
            receipt.write(json.dumps(row) + '\n')
            receipt.flush()
            before = after
            if time.monotonic() > deadline:
                raise RuntimeError('No qualifying idle window. No measurement executed.')


def ticks(pid):
    try:
        fields = Path(f'/proc/{pid}/stat').read_text().rsplit(')', 1)[1].split()
        return int(fields[11]) + int(fields[12])
    except (FileNotFoundError, ProcessLookupError):
        return None


def digest(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def validate_output(text, returncode, strict):
    if returncode != 0:
        raise RuntimeError(f'SLO process exited {returncode}')
    passes = re.findall(r'^--- PASS: TestSLO_WorkloadCharts \(', text, re.MULTILINE)
    if len(passes) != 1 or re.search(r'^--- SKIP: TestSLO_WorkloadCharts ', text, re.MULTILINE):
        raise RuntimeError('SLO did not pass, including a skipped or missing test')
    reports = re.findall(r'charts/workloads p50=\S+ p95=\S+ p99=\S+ SLO=(\S+)', text)
    if len(reports) != 1 or (strict and reports[0] != '90ms'):
        raise RuntimeError(f'Expected one explicit SLO report at the selected budget, got {reports}')


def measure(binary, prefix, strict):
    identity = digest(binary)
    wait_idle(prefix)
    # Preserve the real Actions identity, without forwarding runner credentials.
    env = {key: os.environ[key] for key in (
        'PATH', 'HOME', 'TMPDIR', 'TZ', 'GOMAXPROCS', 'CI',
        'GITHUB_ACTIONS', 'GITHUB_RUN_ID', 'GITHUB_RUN_ATTEMPT') if key in os.environ}
    if strict:
        env['PULSE_STRICT_LOCAL_API_SLOS'] = 'true'
    command = [str(binary), '-test.run=^TestSLO_WorkloadCharts$', '-test.v',
               '-test.count=1', '-test.cpu=2', '-test.timeout=5m']
    capacity_errors = []
    with prefix.with_suffix('.log').open('w') as log, prefix.with_suffix('.cpu.jsonl').open('w') as receipt:
        before = snapshot()
        process = subprocess.Popen(command, env=env, stdout=log, stderr=subprocess.STDOUT)
        own_before = 0
        while process.poll() is None:
            time.sleep(1)
            after = snapshot()
            own_after = ticks(process.pid)
            row = interval(before, after)
            if own_before is not None and own_after is not None and row['total_ticks'] > 0:
                row['measurement_cpu_pct'] = 100 * (own_after - own_before) / row['total_ticks']
                row['background_busy_pct'] = max(0, row['busy_pct'] - row['measurement_cpu_pct'])
                if (row['background_busy_pct'] > 10 or row['guest_pct'] > 2
                        or row['steal_pct'] > 0.1 or row['iowait_pct'] > 1 or row['swap_pages']):
                    capacity_errors.append(row)
            receipt.write(json.dumps(row) + '\n')
            receipt.flush()
            before, own_before = after, own_after
        returncode = process.wait()
    with prefix.with_suffix('.after.jsonl').open('w') as receipt:
        before = snapshot()
        for _ in range(3):
            time.sleep(5)
            after = snapshot()
            receipt.write(json.dumps(interval(before, after)) + '\n')
            before = after
    result = dict(command=command, binary_sha256=identity, environment=env,
                  pid=process.pid, returncode=returncode, capacity_errors=capacity_errors)
    prefix.with_suffix('.result.json').write_text(json.dumps(result, indent=2) + '\n')
    text = prefix.with_suffix('.log').read_text()
    print(text, flush=True)
    if digest(binary) != identity:
        raise RuntimeError('Measured executable changed')
    validate_output(text, returncode, strict)
    if capacity_errors:
        raise RuntimeError('Background CPU or guest/steal activity invalidated the idle measurement')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--baseline', type=Path, required=True)
    parser.add_argument('--candidate', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=False)
    # Baseline uses its ordinary hosted envelope only as timing context. The
    # candidate must pass the existing local 90 ms budget without any skip.
    measure(args.baseline.resolve(), args.output / 'baseline', strict=False)
    measure(args.candidate.resolve(), args.output / 'candidate', strict=True)


if __name__ == '__main__':
    main()
