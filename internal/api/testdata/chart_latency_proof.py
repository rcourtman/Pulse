"""Run the workload chart SLO on observed idle capacity and retain raw evidence."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time

# A capacity-invalid proof is neither a pass nor a failure. The workflow maps
# this exit status to a neutral check conclusion.
INCONCLUSIVE_EXIT = 78


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


def process_counters(root=Path('/proc')):
    counters = {}
    for path in root.iterdir():
        if not path.name.isdigit():
            continue
        try:
            name, tail = (path / 'stat').read_text().rsplit(')', 1)
            fields = tail.split()
            counters[int(path.name)] = dict(name=name.split('(', 1)[1],
                                            ticks=int(fields[11]) + int(fields[12]),
                                            start=fields[19])
        except (OSError, ValueError, IndexError):
            continue
    return counters


def background_processes(before, after, measurement_pid, total_ticks):
    rows = []
    if total_ticks <= 0:
        return rows
    for pid, current in after.items():
        if pid == measurement_pid:
            continue
        previous = before.get(pid, {})
        previous_ticks = previous.get('ticks', 0) if previous.get('start') == current['start'] else 0
        delta = current['ticks'] - previous_ticks
        if delta > 0:
            rows.append(dict(pid=pid, name=current['name'], cpu_pct=100 * delta / total_ticks))
    return sorted(rows, key=lambda row: row['cpu_pct'], reverse=True)[:5]


def process_io(pid):
    try:
        fields = dict(line.split(': ') for line in Path(f'/proc/{pid}/io').read_text().splitlines())
        return {key: int(fields[key]) for key in ('read_bytes', 'write_bytes')}
    except (OSError, ValueError, KeyError):
        return None


def competing_capacity(row):
    # The measured SQLite process performs I/O. Keep I/O wait in the raw
    # receipt and latency, rather than mistaking its own reads for contention.
    # Admission still requires quiet I/O before the process starts.
    return (row['background_busy_pct'] > 10 or row['guest_pct'] > 2
            or row['steal_pct'] > 0.1 or row['swap_pages'] != 0)


def digest(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def slo_report(text):
    reports = re.findall(r'charts/workloads p50=\S+ p95=\S+ p99=\S+ SLO=(\S+)', text)
    if len(reports) != 1 or reports[0] != '90ms':
        raise RuntimeError(f'Expected one explicit SLO report at the selected budget, got {reports}')
    return re.search(r'charts/workloads p50=\S+ p95=\S+ p99=\S+ SLO=\S+', text).group(0)


def validate_output(text, returncode):
    if returncode != 0:
        raise RuntimeError(f'SLO process exited {returncode}')
    passes = re.findall(r'^--- PASS: TestSLO_WorkloadCharts \(', text, re.MULTILINE)
    if len(passes) != 1 or re.search(r'^--- SKIP: TestSLO_WorkloadCharts ', text, re.MULTILINE):
        raise RuntimeError('SLO did not pass, including a skipped or missing test')
    slo_report(text)


def validate_contended_output(text, returncode):
    # Contention only voids the latency verdict. The test must still have
    # executed once and reported, so a crash, skip or broken fixture fails.
    report = slo_report(text)
    outcome = re.findall(r'^--- (PASS|FAIL|SKIP): TestSLO_WorkloadCharts \(', text, re.MULTILINE)
    diagnostics = re.findall(r'^\s+\S+\.go:\d+: (.*)$', text, re.MULTILINE)
    violations = [line for line in diagnostics if line.startswith('SLO VIOLATION: p95=')]
    others = [line for line in diagnostics
              if line not in violations and not line.startswith('charts/workloads p50=')]
    budget_only = (outcome == ['FAIL'] and returncode == 1
                   and len(violations) == 1 and not others)
    if not (outcome == ['PASS'] and returncode == 0) and not budget_only:
        raise RuntimeError(f'SLO process exited {returncode} with outcome {outcome}')
    return report


def report_inconclusive(report, capacity_errors, summary_path):
    names = sorted({row['name'] for error in capacity_errors
                    for row in error['background_processes'] if row['cpu_pct'] > 2})
    peak = max(error['background_busy_pct'] for error in capacity_errors)
    lines = ['Workload chart 90ms SLO: inconclusive',
             f'{report} (not a verdict)',
             f'{len(capacity_errors)} measurement intervals had competing capacity '
             f'(peak {peak:.1f}% background CPU; processes: {", ".join(names) or "none named"}).',
             'The hosted runner was not idle, so this result neither passes nor fails the budget.']
    print('\n'.join(lines), flush=True)
    if summary_path:
        with open(summary_path, 'a') as summary:
            summary.write('\n\n'.join(lines) + '\n')


def measure(binary, prefix):
    identity = digest(binary)
    wait_idle(prefix)
    # Preserve the real Actions identity, without forwarding runner credentials.
    env = {key: os.environ[key] for key in (
        'PATH', 'HOME', 'TMPDIR', 'TZ', 'GOMAXPROCS', 'CI',
        'GITHUB_ACTIONS', 'GITHUB_RUN_ID', 'GITHUB_RUN_ATTEMPT') if key in os.environ}
    env['PULSE_STRICT_LOCAL_API_SLOS'] = 'true'
    command = [str(binary), '-test.run=^TestSLO_WorkloadCharts$', '-test.v',
               '-test.count=1', '-test.cpu=2', '-test.timeout=5m']
    capacity_errors = []
    with prefix.with_suffix('.log').open('w') as log, prefix.with_suffix('.cpu.jsonl').open('w') as receipt:
        before = snapshot()
        processes_before = process_counters()
        process = subprocess.Popen(command, env=env, stdout=log, stderr=subprocess.STDOUT)
        own_before = 0
        while process.poll() is None:
            time.sleep(1)
            after = snapshot()
            processes_after = process_counters()
            own_after = ticks(process.pid)
            row = interval(before, after)
            row['measurement_io_bytes'] = process_io(process.pid)
            row['background_processes'] = background_processes(
                processes_before, processes_after, process.pid, row['total_ticks'])
            if own_before is not None and own_after is not None and row['total_ticks'] > 0:
                row['measurement_cpu_pct'] = 100 * (own_after - own_before) / row['total_ticks']
                row['background_busy_pct'] = max(0, row['busy_pct'] - row['measurement_cpu_pct'])
                if competing_capacity(row):
                    capacity_errors.append(row)
            receipt.write(json.dumps(row) + '\n')
            receipt.flush()
            before, own_before = after, own_after
            processes_before = processes_after
        returncode = process.wait()
    with prefix.with_suffix('.after.jsonl').open('w') as receipt:
        before = snapshot()
        for _ in range(3):
            time.sleep(5)
            after = snapshot()
            receipt.write(json.dumps(interval(before, after)) + '\n')
            before = after
    verdict = 'inconclusive' if capacity_errors else 'valid'
    result = dict(command=command, binary_sha256=identity, environment=env,
                  pid=process.pid, returncode=returncode, verdict=verdict,
                  capacity_errors=capacity_errors)
    prefix.with_suffix('.result.json').write_text(json.dumps(result, indent=2) + '\n')
    text = prefix.with_suffix('.log').read_text()
    print(text, flush=True)
    if digest(binary) != identity:
        raise RuntimeError('Measured executable changed')
    if capacity_errors:
        report = validate_contended_output(text, returncode)
        report_inconclusive(report, capacity_errors, os.environ.get('GITHUB_STEP_SUMMARY'))
        return verdict
    validate_output(text, returncode)
    return verdict


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--candidate', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=False)
    # Absolute qualification executes the candidate directly. The separate
    # paired benchmark job owns relative comparisons and baseline context.
    if measure(args.candidate.resolve(), args.output / 'candidate') == 'inconclusive':
        sys.exit(INCONCLUSIVE_EXIT)


if __name__ == '__main__':
    main()
