"""Exercise the actual sliced Go runner, recorder and required-check verdict."""

import copy
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

import yaml


ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / "scripts/release_control/run_go_package_slice.py"
SHARD = ROOT / "scripts/release_control/run_go_rest_shard.py"
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
spec = importlib.util.spec_from_file_location("package_slices", RUNNER)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


def job(name):
    return re.search(rf"^  {name}:\n(.*?)(?=^  [\w-]+:\n|\Z)",
                     WORKFLOW.read_text(), re.M | re.S)[1]


def ordinary_fixture_orders(root):
    names = [runner.MODULE + f"internal/{i}" for i in range(12)]
    selector = root / ".github/scripts/select-go-package-shard.py"
    weights = root / ".github/scripts/go-package-test-seconds.txt"
    orders = [subprocess.run(
        [sys.executable, str(selector), str(weights), "2", str(index)],
        input="\n".join(names) + "\n", capture_output=True, text=True, check=True,
    ).stdout.splitlines() for index in (0, 1)]
    return names, orders


class SliceTest(unittest.TestCase):
    def test_measured_boundary_balances_estimates_without_changing_source_coverage(self):
        with tempfile.TemporaryDirectory() as temporary:
            weights = Path(temporary) / 'weights'
            names = [f'TestZ{i}' for i in range(7)]
            for costs in ([1]*7, [100, 1, 1, 1, 1, 1, 1], [1, 1, 1, 100, 1, 1, 1],
                          [1, 1, 1, 1, 1, 1, 100], [3, 8, 2, 15, 4, 7, 11]):
                weights.write_text('DEFAULT_WEIGHT 1\nTestDeleted 99999\n' +
                                   '\n'.join(f'{n} {c}' for n, c in zip(names, costs)))
                with patch.dict(os.environ, {'DEFAULT_WEIGHT': '99999'}):
                    pieces = [runner.select(names, i, weights) for i in (0, 1)]
                self.assertTrue(all(pieces))
                self.assertEqual(pieces[0] + pieces[1], names)
                cut = len(pieces[0])
                self.assertEqual(max(sum(costs[:cut]), sum(costs[cut:])),
                                 min(max(sum(costs[:j]), sum(costs[j:]))
                                     for j in range(1, len(names))))
            # Weights can be stale or empty. They cannot define coverage or
            # a different default in one worker's environment.
            weights.write_text('# no measured source tests\nDEFAULT_WEIGHT 1\nTestDeleted 100\n')
            self.assertEqual([runner.select(names, i, weights) for i in (0, 1)],
                             [names[:4], names[4:]])

    def test_unavailable_invalid_or_noncontiguous_selection_cannot_admit_tests(self):
        with tempfile.TemporaryDirectory() as temporary:
            weights = Path(temporary) / 'weights'
            names = ['TestA', 'TestB', 'TestC']
            with self.assertRaises(subprocess.CalledProcessError):
                runner.select(names, 0, weights)
            for bad in ('TestA NaN', 'TestA 0', 'TestA -1', 'broken line extra'):
                weights.write_text(bad+'\n')
                with self.assertRaises(subprocess.CalledProcessError):
                    runner.select(names, 0, weights)
            for index, selected in ((0, ''), (0, 'TestB\n'), (1, 'TestA\n'),
                                    (0, 'TestA\nTestA\n'), (1, 'TestB\nTestForeign\n'),
                                    (0, '\n'.join(names)+'\n')):
                with patch.object(runner.subprocess, 'run', return_value=
                                  subprocess.CompletedProcess([], 0, selected, '')):
                    with self.assertRaises(ValueError):
                        runner.select(names, index, weights)

    def test_slices_are_contiguous_disjoint_exhaustive_and_keep_source_order(self):
        for count in (2, 3, 17, 10000):
            names = [f"TestZ{i}" for i in range(count)]
            pieces = [runner.select(names, i) for i in range(2)]
            self.assertTrue(all(pieces))
            self.assertEqual(pieces[0] + pieces[1], names)
            for selected in pieces:
                flag, pattern = runner.test_filter(names, selected)
                actual = [n for n in names if bool(re.fullmatch(pattern, n)) == (flag == "-run")]
                self.assertEqual(actual, selected)
        for names, index in (([], 0), (["TestOne"], 0), (["TestA", "TestB"], 2)):
            with self.assertRaises(ValueError):
                runner.select(names, index)

    def test_shorter_filter_never_combines_run_and_skip_and_bounds_actual_bytes(self):
        names = ["TestA", "Test" + "Long" * 20]
        self.assertEqual(runner.test_filter(names, [names[1]])[0], "-skip")
        with self.assertRaises(ValueError):
            runner.test_filter(["Test" + "A" * 120000, "Test" + "B" * 120000],
                               ["Test" + "A" * 120000])

    def test_listing_rejects_empty_and_duplicate_names_but_ignores_go_summary(self):
        self.assertEqual(runner.source_tests("TestB\nFuzzA\nExampleZ\nok package 0.1s\n"),
                         ["TestB", "FuzzA", "ExampleZ"])
        for output in ("ok package 0.1s\n", "TestA\nTestA\n"):
            with self.assertRaises(ValueError):
                runner.source_tests(output)


class ExecutedRunnerTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        go = self.root / "go"
        go.write_text('''#!/usr/bin/env python3
import json, os, re, sys
from pathlib import Path
args = sys.argv[1:]
ordinary = args[-1].startswith('github.com/')
json_mode = '-json' in args or '-json=true' in os.environ.get('GOFLAGS','').split()
with Path(os.environ['CALLS']).open('a') as f: f.write(json.dumps(args)+'\\n')
if os.environ.get('ENV_CALLS'):
    with Path(os.environ['ENV_CALLS']).open('a') as f:
        f.write(json.dumps(os.environ.get('GOFLAGS', ''))+'\\n')
if os.environ.get('RACE_ENV_CALLS'):
    with Path(os.environ['RACE_ENV_CALLS']).open('a') as f:
        f.write(json.dumps(os.environ.get('GORACE'))+'\\n')
if args[0] == 'list':
    print('\\n'.join('github.com/rcourtman/pulse-go-rewrite/'+n for n in
          ['internal/api','internal/monitoring','scripts/installtests']+
          ['internal/'+str(i) for i in range(12)]))
    sys.exit(int(os.environ.get('INVENTORY_EXIT','0')))
events = os.environ.get('EVENTS')
if events and '-list' not in args:
    import fcntl, time
    def event(kind):
        with open(events,'a') as f:
            fcntl.flock(f,fcntl.LOCK_EX)
            f.write(json.dumps({'kind':kind,'pid':os.getpid(),'ordinary':ordinary,
                                'goflags':os.environ.get('GOFLAGS',''),
                                'at':time.monotonic()})+'\\n')
    import atexit
    event('start'); atexit.register(event,'end')
    time.sleep(0.4 if ordinary else 0.06)
if '-list' not in args and not json_mode:
    print('ok ordinary packages 0.01s')
    sys.exit(int(os.environ.get('ORDINARY_EXIT','0')))
names = json.loads(os.environ.get('SOURCE_NAMES',
                    '["TestZ", "TestA", "FuzzB", "ExampleC", "TestLast"]'))
if '-list' in args:
    if os.environ.get('LISTING_EXIT'):
        print('original listing failure', file=sys.stderr)
        sys.exit(int(os.environ['LISTING_EXIT']))
    print('\\n'.join(names)); print('ok package 0.01s'); sys.exit(0)
if not ordinary:
    flag = '-run' if '-run' in args else '-skip'
    pattern = args[args.index(flag)+1]
    names = [n for n in names if bool(re.fullmatch(pattern,n)) == (flag == '-run')]
pkg = args[-1] if ordinary else 'github.com/rcourtman/pulse-go-rewrite/'+args[-1][2:]
mode = os.environ.get('MODE','')
if mode == 'wrong-package': pkg = 'example.invalid/other'
if mode == 'extra': names.append('TestUnexpected')
if mode == 'missing': names.pop()
if mode == 'duplicate': names.append(names[0])
def emit(action, **kw): print(json.dumps({'Action':action,'Package':pkg,**kw}))
emit('start')
for name in names:
    emit('run',Test=name)
    if mode != 'unfinished':
        emit('skip' if mode == 'skip' else ('fail' if mode == 'failed-root' else 'pass'),
             Test=name,Elapsed=0.01)
if mode != 'truncated': emit('pass',Elapsed=0.02)
sys.exit(int(os.environ.get('ORDINARY_EXIT' if ordinary else 'TEST_EXIT','0')))
''')
        go.chmod(0o700)

    def execute(self, package="internal/monitoring", index=0, **overrides):
        target = self.root / "result"
        env = {**os.environ, "PATH": str(self.root) + ":" + os.environ["PATH"],
               "CALLS": str(self.root / "calls"), **overrides}
        result = subprocess.run(
            ["python3", str(RUNNER), "--package", package, "--index", str(index),
             "--output", str(target)], env=env, capture_output=True, text=True, timeout=20)
        return result, target

    def test_actual_command_runs_all_fixed_package_slices_through_recorder(self):
        for package in runner.PACKAGES:
            selected = []
            for index in range(2):
                result, target = self.execute(package, index)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                receipt = json.loads((target / "receipt.json").read_text())
                self.assertEqual((receipt['test_exit'], receipt['recorder_exit']), (0, 0))
                self.assertEqual(receipt['package'], runner.MODULE + package)
                selected += (target / "selected.tests").read_text().splitlines()
                actual = json.loads((self.root / "calls").read_text().splitlines()[-1])
                self.assertIn("-race", actual)
                self.assertIn("-count=1", actual)
                self.assertIn("50m", actual)
                self.assertEqual(sum(x in actual for x in ("-run", "-skip")), 1)
            self.assertEqual(selected, ['TestZ', 'TestA', 'FuzzB', 'ExampleC', 'TestLast'])

    def test_complete_explicit_skips_stay_observed_not_missing_execution(self):
        result, target = self.execute(MODE="skip")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        sidecar = json.loads((target / "timings.txt.failures.json").read_text())
        self.assertEqual(sidecar['skipped_top_level_tests']['names'], ['FuzzB', 'TestA', 'TestZ'])

    def test_slices_own_execution_modes_but_preserve_compiler_and_module_flags(self):
        overrides = ['-run=', '-skip=', '-list=', '-exec=', '-fuzz=',
                     '-short=false', '-failfast=false', '-c=false', '-n=false',
                     '-bench=', '-shuffle=off']
        for package in runner.PACKAGES:
            for index in (0, 1):
                for inherited in ('', '-p=2 -mod=readonly -json=true',
                                  "-tags='fixture tag' -run=TestOnly -skip=. -list=. "
                                  '-exec=/bin/true -fuzz=FuzzOnly -short -failfast '
                                  '-c -n -bench=. -shuffle=1 -count=0'):
                    with self.subTest(package=package, index=index, inherited=inherited):
                        calls = self.root / 'calls'
                        environments = self.root / 'environments'
                        calls.write_text('')
                        environments.write_text('')
                        result, target = self.execute(package, index, GOFLAGS=inherited,
                                                      ENV_CALLS=str(environments))
                        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                        actual = [json.loads(line) for line in calls.read_text().splitlines()]
                        observed = [json.loads(line) for line in environments.read_text().splitlines()]
                        expected = inherited + ' ' + ' '.join(overrides)
                        self.assertEqual(observed, [expected, expected])
                        self.assertEqual(actual[0], ['test', '-race', '-json=false',
                                                     '-list', '.', './' + package])
                        self.assertIn('-count=1', actual[1])
                        self.assertIn('-json', actual[1])
                        self.assertEqual(sum(flag in actual[1] for flag in ('-run', '-skip')), 1)
                        self.assertEqual(json.loads((target / 'receipt.json').read_text())
                                         ['test_exit'], 0)

    def test_actual_runner_uses_package_weights_not_their_names_as_an_allowlist(self):
        for package, weights in runner.WEIGHTS.items():
            heavy = max((line.split() for line in weights.read_text().splitlines()
                         if line and not line.startswith(('#', 'DEFAULT_WEIGHT'))),
                        key=lambda fields: float(fields[1]))[0]
            names = [heavy] + [f'TestNewUnmeasured{i}' for i in range(6)]
            pieces = []
            for index in (0, 1):
                result, target = self.execute(package, index,
                                              SOURCE_NAMES=json.dumps(names), DEFAULT_WEIGHT='99999')
                self.assertEqual(result.returncode, 0, result.stdout+result.stderr)
                pieces.append((target / 'selected.tests').read_text().splitlines())
                receipt = json.loads((target / 'receipt.json').read_text())
                self.assertEqual(receipt['listed_roots'], len(names))
                self.assertEqual(receipt['weights'], str(weights.relative_to(ROOT)))
            self.assertEqual(pieces, [names[:1], names[1:]])

    def test_actual_pipeline_cannot_hide_go_recorder_or_incomplete_coverage_failure(self):
        for options in ({"TEST_EXIT": "9"}, {"LISTING_EXIT": "7"},
                        *({"MODE": mode} for mode in ('wrong-package', 'extra', 'missing',
                                                      'duplicate', 'unfinished', 'failed-root', 'truncated'))):
            with self.subTest(options=options):
                result, _ = self.execute(**options)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        result, _ = self.execute(LISTING_EXIT="7")
        self.assertIn('original listing failure', result.stderr)


class WorkflowTest(unittest.TestCase):
    def test_coverage_failure_is_fatal_without_replacing_the_observed_go_exit(self):
        spec = importlib.util.spec_from_file_location('ordinary_verdict', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        name = runner.MODULE + 'internal/ordinary'
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            for coverage_exit in (0, 1, None):
                with self.subTest(coverage_exit=coverage_exit), \
                        patch.object(shard, 'ordinary_packages', return_value=[name]), \
                        patch.object(shard, 'tasks', return_value=[(name, ['unused'])]), \
                        patch.object(shard, 'run_task', return_value={
                            'package': name, 'started': True, 'exit': 0, 'coverage_exit': coverage_exit}), \
                        patch.object(sys, 'stdin', io.StringIO(name + '\n')), \
                        patch.object(sys, 'argv', [str(SHARD), '--index', '0', '--output', str(output),
                                                   '--ordinary-command', shard.ORDINARY_COMMAND]), \
                        contextlib.redirect_stdout(io.StringIO()):
                    self.assertEqual(shard.main(), 0 if coverage_exit == 0 else 1)
                    task = json.loads((output / 'ordinary-packages.json').read_text())['tasks'][0]
                    self.assertEqual(task['exit'], 0)
                    self.assertEqual(task['coverage_exit'], coverage_exit)

    def test_ordinary_zero_exit_cannot_hide_listed_roots_or_invalid_events(self):
        fixture = ExecutedRunnerTest()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        spec = importlib.util.spec_from_file_location('ordinary_coverage', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        command = ['bash', '-euo', 'pipefail', '-c', shard.ORDINARY_COMMAND]
        name = runner.MODULE + 'internal/ordinary'
        for mode in ('', 'skip', 'wrong-package', 'extra', 'missing', 'duplicate',
                     'unfinished', 'failed-root', 'truncated'):
            with self.subTest(mode=mode), patch.dict(os.environ, {
                    'PATH': str(fixture.root) + ':' + os.environ['PATH'],
                    'CALLS': str(fixture.root / 'calls'), 'MODE': mode,
                    'GOFLAGS': '-mod=readonly -json=true'}):
                result = shard.run_task((name, command), fixture.root / 'output' / (mode or 'pass'))
                self.assertTrue(result['started'])
                self.assertEqual(result['exit'], 0, 'retain the actual Go exit, not a coverage verdict')
                self.assertEqual(result['listing_exit'], 0)
                self.assertEqual(result['listed_roots'], 5)
                self.assertEqual(result['coverage_exit'], 0 if mode in ('', 'skip') else 1)
                sidecar = fixture.root / 'output' / (mode or 'pass') / 'ordinary' / name / 'timings.txt.failures.json'
                index = json.loads(sidecar.read_text())
                if mode == 'missing':
                    self.assertEqual(index['missing_selected_tests']['names'], ['TestLast'])
                if mode == 'skip':
                    self.assertEqual(index['skipped_top_level_tests']['observed_count'], 5)

    def test_ordinary_race_exit_delay_is_only_removed_from_test_discovery(self):
        fixture = ExecutedRunnerTest()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        spec = importlib.util.spec_from_file_location('ordinary_race_discovery', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        command = ['bash', '-euo', 'pipefail', '-c', shard.ORDINARY_COMMAND]
        name = runner.MODULE + 'internal/ordinary'
        calls = fixture.root / 'calls'
        race_options = fixture.root / 'race-options'
        for inherited in (None, 'halt_on_error=1 atexit_sleep_ms=500'):
            with self.subTest(inherited=inherited), patch.dict(os.environ, {
                    'PATH': str(fixture.root) + ':' + os.environ['PATH'],
                    'CALLS': str(calls), 'RACE_ENV_CALLS': str(race_options),
                    'GOFLAGS': '-mod=readonly'}), contextlib.redirect_stdout(io.StringIO()):
                if inherited is None:
                    os.environ.pop('GORACE', None)
                else:
                    os.environ['GORACE'] = inherited
                calls.write_text('')
                race_options.write_text('')
                result = shard.run_task((name, command), fixture.root / 'output')
                self.assertEqual((result['listing_exit'], result['exit'], result['coverage_exit']),
                                 (0, 0, 0))
                self.assertEqual(result['listed_roots'], 5)
                self.assertEqual(list(map(json.loads, calls.read_text().splitlines())), [
                    ['test', '-race', '-json=false', '-list', '.', name],
                    ['test', '-race', '-timeout', '50m', name],
                ])
                self.assertEqual(list(map(json.loads, race_options.read_text().splitlines())), [
                    ((inherited or '') + ' atexit_sleep_ms=0').strip(), inherited,
                ])
                self.assertEqual(os.environ.get('GORACE'), inherited)

    def test_only_a_complete_empty_package_stream_matches_a_zero_root_listing(self):
        spec = importlib.util.spec_from_file_location('ordinary_empty', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        package = 'example.invalid/empty'
        def event(action, **fields):
            return json.dumps({'Package': package, 'Action': action, **fields}) + '\n'
        cases = [(event('start') + event(action), 0) for action in ('pass', 'skip')]
        cases += [(stream, 1) for stream in (
            '', 'not-json\n', event('start'), event('start') + event('fail'),
            event('pass') + event('pass'), event('pass') + event('output', Output='late'),
            event('start') + event('pass', Package='example.invalid/other'),
            event('run', Test='TestUnexpected') + event('pass'),
            event('start') + event('output', Output=[]) + event('pass'))]
        with tempfile.TemporaryDirectory() as temporary:
            for stream, expected in cases:
                with self.subTest(stream=stream), contextlib.redirect_stdout(io.StringIO()), \
                        contextlib.redirect_stderr(io.StringIO()):
                    self.assertEqual(shard.empty_package_coverage(io.StringIO(stream), package, Path(temporary)),
                                     expected)
                    self.assertEqual(json.loads((Path(temporary) / 'empty-coverage.json').read_text())
                                     ['coverage_exit'], expected)

    def test_main_workflow_keeps_frozen_classifier_recipe_compatible(self):
        # Patch preparations copy main's workflow/controls, not scripts/tests.
        # v6.6.0's test_ci_browser_proof_only.py uses this older extraction;
        # main's safer YAML reader alone cannot repair that frozen consumer.
        block = job('changes')
        steps = re.split(r'(?m)^      - name: ', block)[1:]
        selected = next(part for part in steps if part.startswith('Classify changed files\n'))
        frozen_command = '\n'.join(line[10:] for line in
                                   selected.split('        run: |\n', 1)[1].splitlines())
        actual_command = next(step['run'] for step in yaml.safe_load(WORKFLOW.read_text())
                              ['jobs']['changes']['steps']
                              if step.get('name') == 'Classify changed files')
        self.assertEqual(frozen_command.strip(), actual_command.strip())

    def test_worker_contracts_gate_changed_area_admission_before_compilation(self):
        jobs = yaml.safe_load(WORKFLOW.read_text())['jobs']
        changes = jobs['changes']
        self.assertNotIn('continue-on-error', changes)
        steps = changes['steps']
        name = 'Validate backend worker contracts before compilation'
        gates = [step for step in steps if step.get('name') == name]
        self.assertEqual(gates, [{
            'name': name,
            'if': "steps.filter.outputs.code == 'true'",
            'timeout-minutes': 1,
            'run': 'python3 scripts/release_control/run_go_package_slice_test.py',
        }])
        self.assertLess(next(i for i, step in enumerate(steps) if step.get('id') == 'filter'),
                        steps.index(gates[0]))
        # The gate holds only application admission. Secret scanning stays
        # independent, while compile and runtime keep their existing checks.
        self.assertNotIn('needs', jobs['secret-scan'])
        self.assertEqual(jobs['test-compile']['needs'], 'changes')
        self.assertEqual(jobs['test-compile']['if'],
                         "needs.changes.outputs.go_compile == 'true'")
        for worker in ('backend', 'backend-api'):
            self.assertEqual(jobs[worker]['needs'], ['changes', 'test-compile'])
            self.assertIn("needs.changes.result == 'success'", jobs[worker]['if'])

    def test_actual_precompile_control_command_stops_on_failure_before_next_work(self):
        changes = yaml.safe_load(WORKFLOW.read_text())['jobs']['changes']
        command = next(step['run'] for step in changes['steps']
                       if step.get('name') == 'Validate backend worker contracts before compilation')
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            control = root / 'scripts/release_control/run_go_package_slice_test.py'
            control.parent.mkdir(parents=True)
            # Exercise the workflow's literal shell command without recursively
            # running this suite. These are exit adapters, not Go runtime proof.
            control.write_text('import os\nfrom pathlib import Path\n'
                               'Path("control-ran").write_text("observed")\n'
                               'raise SystemExit(int(os.environ["CONTROL_EXIT"]))\n')
            for exit_code in (0, 1, 7):
                with self.subTest(exit_code=exit_code):
                    following = root / 'following-work'
                    following.unlink(missing_ok=True)
                    result = subprocess.run(
                        ['bash', '-e', '-o', 'pipefail', '-c',
                         command + '\nprintf "started" > following-work\n'],
                        cwd=root, env={**os.environ, 'CONTROL_EXIT': str(exit_code)},
                        capture_output=True, text=True, timeout=5)
                    self.assertEqual(result.returncode, exit_code, result.stdout + result.stderr)
                    self.assertEqual((root / 'control-ran').read_text(), 'observed')
                    self.assertEqual(following.exists(), exit_code == 0)

    def test_same_two_required_workers_cover_both_sliced_packages(self):
        block = job('backend')
        for expected in (
            'name: Backend tests (${{ matrix.shard }})', 'shard: [rest-0, rest-1]',
            'needs: [changes, test-compile]', "GOMAXPROCS: '2'",
            'python3 scripts/release_control/run_go_rest_shard.py',
            'ordinary_command=\'go test -race -timeout 50m $pkgs\'',
            '--ordinary-command "$ordinary_command"',
            '--index "$index" --output "$RUNNER_TEMP/backend-slices"',
            'name: backend-slices-${{ matrix.shard }}',
        ):
            self.assertIn(expected, block)
        self.assert_backend_validation_remains_fatal(
            yaml.safe_load(WORKFLOW.read_text())['jobs']['backend'])
        self.assertNotIn('  backend-slow:', WORKFLOW.read_text())
        self.assertNotIn('  backend-packages:', WORKFLOW.read_text())

    def assert_backend_validation_remains_fatal(self, job):
        # Object transport may miss or fail without changing validation. Only
        # the complete pinned, read-only restore can be optional: not the job,
        # admission, source-listed task queue or its coverage upload.
        self.assertNotIn('continue-on-error', job)
        prefix = ("backend-race-objects-v1-${{ runner.os }}-${{ runner.arch }}-ubuntu24-"
                  "${{ steps.go-setup.outputs.go-version }}-${{ hashFiles('go.mod', 'go.sum') }}-")
        optional_restore = {
            'name': 'Restore backend race build objects',
            'if': "needs.changes.outputs.code == 'true'",
            'id': 'race-cache',
            'continue-on-error': True,
            'uses': 'actions/cache/restore@55cc8345863c7cc4c66a329aec7e433d2d1c52a9',
            'with': {'path': '${{ steps.go-build-cache.outputs.path }}',
                     'key': prefix + '${{ github.sha }}', 'restore-keys': prefix},
        }
        optional_steps = [step for step in job['steps'] if 'continue-on-error' in step]
        self.assertLessEqual(len(optional_steps), 1)
        for step in optional_steps:
            self.assertEqual(step, optional_restore)

    def test_optional_cache_cannot_mask_admission_tests_or_coverage(self):
        job = yaml.safe_load(WORKFLOW.read_text())['jobs']['backend']
        self.assert_backend_validation_remains_fatal(job)
        no_cache = copy.deepcopy(job)
        no_cache['steps'] = [step for step in no_cache['steps'] if step.get('id') != 'race-cache']
        self.assert_backend_validation_remains_fatal(no_cache)
        for name in ('Require compiled backend tests',
                     'Go unit tests (shard ${{ matrix.shard }})',
                     'Retain source-listed backend coverage'):
            with self.subTest(masked_step=name):
                changed = copy.deepcopy(job)
                next(step for step in changed['steps'] if step['name'] == name)['continue-on-error'] = True
                with self.assertRaises(AssertionError):
                    self.assert_backend_validation_remains_fatal(changed)
        changed = copy.deepcopy(job)
        changed['continue-on-error'] = True
        with self.assertRaises(AssertionError):
            self.assert_backend_validation_remains_fatal(changed)
        for field, value in (
                ('uses', 'actions/cache/restore@main'),
                ('uses', 'actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9'),
                ('run', 'exit 0'), ('if', 'always()'),
                ('with', {'path': '${{ github.workspace }}'})):
            with self.subTest(cache_drift=(field, value)):
                changed = copy.deepcopy(job)
                next(step for step in changed['steps'] if step.get('id') == 'race-cache')[field] = value
                with self.assertRaises(AssertionError):
                    self.assert_backend_validation_remains_fatal(changed)
        changed = copy.deepcopy(job)
        changed['steps'].append(copy.deepcopy(next(
            step for step in job['steps'] if step.get('id') == 'race-cache')))
        with self.assertRaises(AssertionError):
            self.assert_backend_validation_remains_fatal(changed)

    def test_actual_worker_recipe_waits_for_both_slices_and_ordinary_failures(self):
        fixture = ExecutedRunnerTest()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        root = fixture.root
        for name in ('scripts/release_control/run_go_package_slice.py',
                     'scripts/release_control/run_go_rest_shard.py',
                     'scripts/release_control/monitoring-test-seconds.txt',
                     'scripts/release_control/installtests-test-seconds.txt',
                     '.github/scripts/select-internal-api-shard.sh',
                     '.github/scripts/record-internal-api-test-seconds.py',
                     '.github/scripts/select-go-package-shard.py',
                     '.github/scripts/go-package-test-seconds.txt'):
            target = root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, target)
        # Do not recursively execute this test from the extracted recipe.
        # The real test command is separately exercised by the CI/source suite.
        (root / 'scripts/release_control/run_go_package_slice_test.py').write_text(
            'import os; raise SystemExit(int(os.environ.get("CONTROL_EXIT","0")))\n')
        step = job('backend').split('      - name: Go unit tests (shard ${{ matrix.shard }})\n', 1)[1]
        body = step.split('        run: |\n', 1)[1].split('\n      - name:', 1)[0]
        command = '\n'.join(line[10:] for line in body.splitlines()
                            if line.startswith('          ') or not line.strip())
        names, orders = ordinary_fixture_orders(root)
        self.assertTrue(all(orders))
        self.assertCountEqual(orders[0] + orders[1], names)
        cases = ((0, {}), (1, {}), (0, {'ORDINARY_EXIT':'9'}), (0, {'MODE':'failed-root'}),
                 (0, {'LISTING_EXIT':'7'}), (0, {'CONTROL_EXIT':'5'}), (0, {'INVENTORY_EXIT':'8'}))
        for index, (shard, options) in enumerate(cases):
            with self.subTest(shard=shard, options=options):
                calls = root / 'calls'
                calls.write_text('')
                env = {**os.environ, 'PATH': str(root)+':'+os.environ['PATH'],
                       'REST_SHARDS':'2', 'COMPONENT_ONLY':'false',
                       'CALLS':str(calls), 'RUNNER_TEMP':str(root / f'output{index}'), **options}
                result = subprocess.run(['bash', '-c', command.replace('${{ matrix.shard }}', f'rest-{shard}')],
                                        cwd=root, env=env, capture_output=True, text=True, timeout=20)
                self.assertEqual(result.returncode == 0, not options, result.stdout+result.stderr)
                observed = [json.loads(line) for line in calls.read_text().splitlines()]
                if 'CONTROL_EXIT' in options:
                    self.assertEqual(observed, [])
                elif 'INVENTORY_EXIT' in options:
                    # Partial output from a failed discovery admits no task.
                    self.assertEqual(observed, [['list', './...']])
                else:
                    ordinary = orders[shard]
                    sliced = ['internal/monitoring', 'scripts/installtests']
                    # Scheduling changes partition sizes, not the need to
                    # independently list and execute every selected package.
                    self.assertCountEqual([call[-1] for call in observed if '-list' in call],
                                          ['./' + name for name in sliced] + ordinary)
                    self.assertCountEqual([call[-1] for call in observed if call[0] == 'test'
                                           and '-json' not in call and '-list' not in call],
                                          [] if 'LISTING_EXIT' in options else ordinary)
                    receipt = json.loads((root / f'output{index}/backend-slices/ordinary-packages.json').read_text())
                    self.assertEqual(receipt['slots'], 4)
                    self.assertEqual([task['package'] for task in receipt['tasks']], sliced + ordinary)
                    self.assertEqual(all(task['exit']==0 for task in receipt['tasks']), not options)

        # Run the actual three PR worker recipes. The last partition lives in
        # the API matrix, so exercise its shell and propagate a failure there.
        jobs = yaml.safe_load(WORKFLOW.read_text())['jobs']
        extra = next(step for step in jobs['backend-api']['steps']
                     if step['name'] == 'Go unit tests (shard rest-2)')
        api = next(step for step in jobs['backend-api']['steps']
                   if step['name'].startswith('Go unit tests (internal/api'))
        self.assertEqual(extra['if'], "needs.changes.outputs.rest_shards == '3' && matrix.index == 4")
        self.assertEqual(api['if'], "needs.changes.outputs.code == 'true' && !(" + extra['if'] + ")")
        self.assertEqual(jobs['changes']['outputs']['rest_shards'],
                         "${{ github.event_name == 'pull_request' && github.base_ref == 'main' && "
                         "steps.filter.outputs.code == 'true' && steps.component.outputs.only != 'true' "
                         "&& '3' || '2' }}")
        self.assertEqual(jobs['backend-api-verdict']['needs'], ['changes', 'backend-api'])
        self.assertFalse(jobs['backend-api']['strategy']['fail-fast'])
        all_ordinary = []
        selected = {package: [] for package in runner.PACKAGES}
        for part in range(3):
            recipe = command.replace('${{ matrix.shard }}', f'rest-{part}') if part < 2 else extra['run']
            output = root / f'pr-{part}'
            env = {**os.environ, 'PATH': str(root) + ':' + os.environ['PATH'],
                   'CALLS': str(root / 'calls'), 'RUNNER_TEMP': str(output),
                   'REST_SHARDS': '3', 'COMPONENT_ONLY': 'false'}
            result = subprocess.run(['bash', '-eu', '-o', 'pipefail', '-c', recipe],
                                    cwd=root, env=env, capture_output=True, text=True, timeout=20)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            artifacts = output / 'backend-slices'
            receipt = json.loads((artifacts / 'ordinary-packages.json').read_text())
            self.assertTrue(receipt['collection_complete'])
            all_ordinary.extend(name for name in receipt['planned_packages'] if name not in selected)
            for package, roots in selected.items():
                directory = artifacts / package.split('/')[-1]
                observation = json.loads((directory / 'receipt.json').read_text())
                self.assertEqual((observation['index'], observation['count']), (part, 3))
                roots.extend((directory / 'selected.tests').read_text().splitlines())
                if part == 2:
                    self.assertEqual(roots, (directory / 'listed.tests').read_text().splitlines())
        self.assertCountEqual(all_ordinary, names)
        failed = subprocess.run(['bash', '-eu', '-o', 'pipefail', '-c', extra['run']],
                                cwd=root, env={**env, 'ORDINARY_EXIT': '9'},
                                capture_output=True, text=True, timeout=20)
        self.assertNotEqual(failed.returncode, 0, failed.stdout + failed.stderr)

    def test_real_process_queue_reuses_finished_slice_slots_and_drains_all_tasks(self):
        fixture = ExecutedRunnerTest()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        events = fixture.root / 'events'
        output = fixture.root / 'output'
        env = {**os.environ, 'PATH': str(fixture.root)+':'+os.environ['PATH'],
               'CALLS':str(fixture.root / 'calls'), 'EVENTS':str(events), 'GOMAXPROCS':'2'}
        names, orders = ordinary_fixture_orders(ROOT)
        all_ordinary = []
        for index, ordinary in enumerate(orders):
            result = subprocess.run(['python3', str(SHARD), '--index', str(index),
                                     '--ordinary-command','go test -race -timeout 50m $pkgs',
                                     '--output',str(output / str(index))],
                                    input='\n'.join(ordinary)+'\n',
                                    cwd=ROOT, env=env, capture_output=True, text=True, timeout=20)
            self.assertEqual(result.returncode, 0, result.stdout+result.stderr)
            receipt = json.loads((output / str(index) / 'ordinary-packages.json').read_text())
            all_ordinary += [task['package'] for task in receipt['tasks'][2:]]
        self.assertCountEqual(all_ordinary, names)
        calls = [json.loads(line) for line in (fixture.root / 'calls').read_text().splitlines()]
        for call in calls:
            if call[0]=='test' and '-list' not in call and '-json' not in call:
                self.assertEqual(call[:-1], ['test','-race','-timeout','50m'])
        active = {}; peak = ordinary_peak = 0
        for event in map(json.loads,events.read_text().splitlines()):
            if event['ordinary']:
                self.assertEqual(event['goflags'].split()[-12:],
                                 ['-p=1', '-count=1', '-run=', '-skip=', '-list=',
                                  '-exec=', '-fuzz=', '-short=false', '-failfast=false',
                                  '-c=false', '-n=false', '-json=true'])
            if event['kind']=='start': active[event['pid']] = event['ordinary']
            else: del active[event['pid']]
            peak = max(peak,len(active)); ordinary_peak = max(ordinary_peak,sum(active.values()))
        self.assertEqual(active,{})
        self.assertLessEqual(peak,4)
        self.assertGreaterEqual(ordinary_peak,3, 'finished slices did not release their slots')

        for supplied in ('', '\n'.join(names), '\n'.join(reversed(orders[0])), '\n'.join(orders[1])):
            result = subprocess.run(['python3', str(SHARD), '--index','0',
                                     '--ordinary-command','go test -race -timeout 50m $pkgs',
                                     '--output',str(output / 'rejected')],
                                    input=supplied, cwd=ROOT, env=env,
                                    capture_output=True, text=True, timeout=20)
            self.assertNotEqual(result.returncode,0)
            self.assertFalse((output / 'rejected').exists())

    def test_inventory_and_launch_failures_cannot_admit_partial_success(self):
        spec = importlib.util.spec_from_file_location('rest_shard', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        from unittest.mock import patch
        prefix = runner.MODULE
        valid = [prefix+n for n in ('internal/api','internal/monitoring','scripts/installtests','internal/a','internal/b')]
        for inventory in (valid[:-3], valid+valid[:1], valid+['example.invalid/foreign']):
            with patch.object(shard.subprocess, 'run', return_value=subprocess.CompletedProcess([],0,'\n'.join(inventory),'')):
                with self.assertRaises(ValueError): shard.ordinary_packages(0)
        with patch.object(shard.subprocess,'run',side_effect=OSError('start failed')):
            observation = shard.run_task(('internal/a',['not-a-command']))
            self.assertIsNone(observation['exit'])
            self.assertFalse(observation['started'])
            self.assertIn('start_error_errno', observation)

    def test_ordinary_execution_overrides_ambient_result_cache_and_compiler_flags(self):
        spec = importlib.util.spec_from_file_location('fresh_rest_shard', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        command = ['bash', '-euo', 'pipefail', '-c', shard.ORDINARY_COMMAND]
        for inherited in ('', '-mod=readonly -count=0 -p=8',
                          "-mod=readonly '-run=TestOnly' -skip=. -list=. -exec=/bin/true "
                          "-fuzz=FuzzOnly -short -failfast -c -n"):
            with self.subTest(inherited=inherited), patch.dict(os.environ, {'GOFLAGS': inherited}):
                with patch.object(shard, 'ordinary_execution',
                                  return_value={'started': True, 'exit': 0, 'coverage_exit': 0}) as execute:
                    self.assertEqual(shard.run_task((runner.MODULE + 'internal/a', command))['exit'], 0)
                environment = execute.call_args.args[2]
                self.assertEqual(environment['GOFLAGS'].split(),
                                 inherited.split() + ['-p=1', '-count=1', '-run=', '-skip=',
                                                      '-list=', '-exec=', '-fuzz=',
                                                      '-short=false', '-failfast=false',
                                                      '-c=false', '-n=false'])
                self.assertEqual(environment['pkgs'], runner.MODULE + 'internal/a')
                self.assertEqual(execute.call_args.args[1], command)
                self.assertEqual(os.environ['GOFLAGS'], inherited)
                # The separate source-listed executor already fixes -count=1
                # in its argv. Do not change its environment or admission.
                with patch.object(shard.subprocess, 'run',
                                  return_value=subprocess.CompletedProcess(command, 0)) as execute:
                    shard.run_task((shard.SLICED[0], command))
                self.assertEqual(execute.call_args.kwargs['env']['GOFLAGS'], inherited)

    def test_completed_children_are_retained_before_a_slow_first_child_returns(self):
        spec = importlib.util.spec_from_file_location('progress_rest_shard', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            output, started, release = root / 'output', root / 'started', root / 'release'
            names = [runner.MODULE + 'internal/' + name for name in ('held', 'failed', 'passed')]
            held = '''from pathlib import Path
import sys,time
started,release=map(Path,sys.argv[1:])
started.write_text('started')
deadline=time.monotonic()+10
while not release.exists() and time.monotonic()<deadline: time.sleep(.01)
sys.exit(0 if release.exists() else 13)
'''
            admitted = [(names[0], [sys.executable, '-I', '-S', '-c', held, str(started), str(release)]),
                        (names[1], [sys.executable, '-I', '-S', '-c', 'raise SystemExit(9)']),
                        (names[2], [sys.executable, '-I', '-S', '-c', 'raise SystemExit(0)'])]
            terminal, errors = [], []
            def execute():
                try:
                    terminal.append(shard.main())
                except BaseException as error:
                    errors.append(error)
            argv = [str(SHARD), '--index', '0', '--output', str(output),
                    '--ordinary-command', shard.ORDINARY_COMMAND]
            with patch.object(shard, 'ordinary_packages', return_value=names), \
                    patch.object(shard, 'tasks', return_value=admitted), \
                    patch.object(sys, 'stdin', io.StringIO('\n'.join(names) + '\n')), \
                    patch.object(sys, 'argv', argv), contextlib.redirect_stdout(io.StringIO()):
                worker = threading.Thread(target=execute)
                worker.start()
                before = None
                try:
                    deadline = time.monotonic() + 5
                    while time.monotonic() < deadline:
                        receipt = output / 'ordinary-packages.json'
                        if receipt.exists():
                            # A partial JSON write is a failure, not a poll to
                            # retry until a favourable snapshot appears.
                            before = json.loads(receipt.read_text())
                            if len(before['tasks']) == 2 and started.exists():
                                break
                        time.sleep(.01)
                    self.assertIsNotNone(before, 'no result retained while the first child waited')
                    self.assertEqual(before['planned_packages'], names)
                    self.assertFalse(before['collection_complete'])
                    self.assertEqual(before['unobserved_packages'], names[:1])
                    self.assertEqual([task['package'] for task in before['tasks']], names[1:])
                    self.assertEqual([task['exit'] for task in before['tasks']], [9, 0])
                    self.assertTrue(all(task['started'] for task in before['tasks']))
                    self.assertEqual(terminal, [], 'partial results became an aggregate verdict')
                finally:
                    release.write_text('release')
                    worker.join(timeout=12)
                self.assertFalse(worker.is_alive(), 'owned test scheduler did not finish')
            self.assertEqual(errors, [])
            self.assertEqual(terminal, [1], 'later successful work hid a failed child')
            final = json.loads((output / 'ordinary-packages.json').read_text())
            self.assertTrue(final['collection_complete'])
            self.assertEqual(final['unobserved_packages'], [])
            self.assertEqual([task['package'] for task in final['tasks']], names)
            self.assertEqual([task['exit'] for task in final['tasks']], [0, 9, 0])
            self.assertEqual(list(output.glob('.ordinary-packages-*')), [])

    def test_a_failed_snapshot_write_preserves_the_previous_complete_json(self):
        spec = importlib.util.spec_from_file_location('atomic_rest_shard', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            names = [runner.MODULE + 'internal/observed', runner.MODULE + 'internal/unobserved']
            results = {names[0]: {'package': names[0], 'started': True, 'exit': 9, 'seconds': .01}}
            shard.retain_results(output, 0, names, results, time.monotonic())
            receipt = output / 'ordinary-packages.json'
            previous = receipt.read_bytes()
            def broken_write(value, stream, **kwargs):
                stream.write('{"incomplete":')
                raise OSError('test-only incomplete snapshot write')
            with patch.object(shard.json, 'dump', side_effect=broken_write), self.assertRaises(OSError):
                shard.retain_results(output, 0, names, results, time.monotonic())
            self.assertEqual(receipt.read_bytes(), previous)
            self.assertEqual(json.loads(previous)['tasks'][0]['exit'], 9)
            self.assertEqual(list(output.glob('.ordinary-packages-*')), [])

    def test_start_failure_is_incomplete_execution_not_an_observed_shell_exit(self):
        spec = importlib.util.spec_from_file_location('start_rest_shard', SHARD)
        shard = importlib.util.module_from_spec(spec); spec.loader.exec_module(shard)
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            names = [runner.MODULE + 'internal/not-started']
            argv = [str(SHARD), '--index', '0', '--output', str(output),
                    '--ordinary-command', shard.ORDINARY_COMMAND]
            with patch.object(shard, 'ordinary_packages', return_value=names), \
                    patch.object(shard, 'tasks', return_value=[(names[0], [str(output / 'absent-command')])]), \
                    patch.object(sys, 'stdin', io.StringIO(names[0] + '\n')), \
                    patch.object(sys, 'argv', argv), \
                    contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(shard.main(), 1)
            receipt = json.loads((output / 'ordinary-packages.json').read_text())
            self.assertTrue(receipt['collection_complete'])
            self.assertEqual(receipt['unobserved_packages'], [])
            self.assertFalse(receipt['tasks'][0]['started'])
            self.assertIsNone(receipt['tasks'][0]['exit'])
            self.assertEqual(receipt['tasks'][0]['start_error_errno'], 2)

    def test_helpers_are_part_of_the_exact_main_controls_copied_to_patches(self):
        stable_patch = (ROOT / 'scripts/release_control/stable_patch.py').read_text()
        self.assertIn('"scripts/release_control"', stable_patch)
        self.assertTrue(RUNNER.is_relative_to(ROOT / 'scripts/release_control'))
        self.assertTrue(SHARD.is_relative_to(ROOT / 'scripts/release_control'))
        self.assertTrue(Path(__file__).is_relative_to(ROOT / 'scripts/release_control'))
        self.assertTrue(all(path.is_relative_to(ROOT / 'scripts/release_control')
                            for path in runner.WEIGHTS.values()))
        self.assertIn('python3 scripts/release_control/run_go_package_slice_test.py', job('backend'))


if __name__ == "__main__":
    unittest.main()
