"""Exercise the actual sliced Go runner, recorder and required-check verdict."""

import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


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


class SliceTest(unittest.TestCase):
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
with Path(os.environ['CALLS']).open('a') as f: f.write(json.dumps(args)+'\\n')
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
            f.write(json.dumps({'kind':kind,'pid':os.getpid(),'ordinary':'-json' not in args,
                                'goflags':os.environ.get('GOFLAGS',''),
                                'at':time.monotonic()})+'\\n')
    import atexit
    event('start'); atexit.register(event,'end')
    time.sleep(0.06 if '-json' in args else 0.4)
if '-list' not in args and '-json' not in args:
    print('ok ordinary packages 0.01s')
    sys.exit(int(os.environ.get('ORDINARY_EXIT','0')))
names = ['TestZ', 'TestA', 'FuzzB', 'ExampleC', 'TestLast']
if '-list' in args:
    if os.environ.get('LISTING_EXIT'):
        print('original listing failure', file=sys.stderr)
        sys.exit(int(os.environ['LISTING_EXIT']))
    print('\\n'.join(names)); print('ok package 0.01s'); sys.exit(0)
flag = '-run' if '-run' in args else '-skip'
pattern = args[args.index(flag)+1]
names = [n for n in names if bool(re.fullmatch(pattern,n)) == (flag == '-run')]
pkg = 'github.com/rcourtman/pulse-go-rewrite/'+args[-1][2:]
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
sys.exit(int(os.environ.get('TEST_EXIT','0')))
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
        self.assertEqual(sidecar['skipped_top_level_tests']['names'], ['TestA', 'TestZ'])

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
        self.assertNotIn('continue-on-error', block)
        self.assertNotIn('  backend-slow:', WORKFLOW.read_text())
        self.assertNotIn('  backend-packages:', WORKFLOW.read_text())

    def test_actual_worker_recipe_waits_for_both_slices_and_ordinary_failures(self):
        fixture = ExecutedRunnerTest()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        root = fixture.root
        for name in ('scripts/release_control/run_go_package_slice.py',
                     'scripts/release_control/run_go_rest_shard.py',
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
        for index, options in enumerate(({}, {'ORDINARY_EXIT':'9'}, {'MODE':'failed-root'},
                                         {'LISTING_EXIT':'7'}, {'CONTROL_EXIT':'5'},
                                         {'INVENTORY_EXIT':'8'})):
            with self.subTest(options=options):
                calls = root / 'calls'
                calls.write_text('')
                env = {**os.environ, 'PATH': str(root)+':'+os.environ['PATH'],
                       'CALLS':str(calls), 'RUNNER_TEMP':str(root / f'output{index}'), **options}
                result = subprocess.run(['bash', '-c', command.replace('${{ matrix.shard }}', 'rest-0')],
                                        cwd=root, env=env, capture_output=True, text=True, timeout=20)
                self.assertEqual(result.returncode == 0, not options, result.stdout+result.stderr)
                observed = [json.loads(line) for line in calls.read_text().splitlines()]
                if 'CONTROL_EXIT' in options:
                    self.assertEqual(observed, [])
                elif 'INVENTORY_EXIT' in options:
                    # Partial output from a failed discovery admits no task.
                    self.assertEqual(observed, [['list', './...']])
                else:
                    self.assertEqual(sum('-list' in call for call in observed), 2)
                    self.assertEqual(sum('-json' not in call and '-list' not in call
                                         and call[0]=='test' for call in observed), 6)
                    receipt = json.loads((root / f'output{index}/backend-slices/ordinary-packages.json').read_text())
                    self.assertEqual(receipt['slots'], 4)
                    self.assertEqual(len(receipt['tasks']), 8)
                    self.assertEqual(len({task['package'] for task in receipt['tasks']}), 8)
                    self.assertEqual(all(task['exit']==0 for task in receipt['tasks']), not options)

    def test_real_process_queue_reuses_finished_slice_slots_and_drains_all_tasks(self):
        fixture = ExecutedRunnerTest()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        events = fixture.root / 'events'
        output = fixture.root / 'output'
        env = {**os.environ, 'PATH': str(fixture.root)+':'+os.environ['PATH'],
               'CALLS':str(fixture.root / 'calls'), 'EVENTS':str(events), 'GOMAXPROCS':'2'}
        all_ordinary = []
        for index in (0, 1):
            names = sorted(runner.MODULE+'internal/'+str(i) for i in range(12))
            result = subprocess.run(['python3', str(SHARD), '--index', str(index),
                                     '--ordinary-command','go test -race -timeout 50m $pkgs',
                                     '--output',str(output / str(index))],
                                    input='\n'.join(names[index::2])+'\n',
                                    cwd=ROOT, env=env, capture_output=True, text=True, timeout=20)
            self.assertEqual(result.returncode, 0, result.stdout+result.stderr)
            receipt = json.loads((output / str(index) / 'ordinary-packages.json').read_text())
            all_ordinary += [task['package'] for task in receipt['tasks'][2:]]
        self.assertEqual(sorted(all_ordinary), sorted(runner.MODULE+'internal/'+str(i) for i in range(12)))
        calls = [json.loads(line) for line in (fixture.root / 'calls').read_text().splitlines()]
        for call in calls:
            if call[0]=='test' and '-list' not in call and '-json' not in call:
                self.assertEqual(call[:-1], ['test','-race','-timeout','50m'])
        active = {}; peak = ordinary_peak = 0
        for event in map(json.loads,events.read_text().splitlines()):
            if event['ordinary']: self.assertEqual(event['goflags'].split()[-1],'-p=1')
            if event['kind']=='start': active[event['pid']] = event['ordinary']
            else: del active[event['pid']]
            peak = max(peak,len(active)); ordinary_peak = max(ordinary_peak,sum(active.values()))
        self.assertEqual(active,{})
        self.assertLessEqual(peak,4)
        self.assertGreaterEqual(ordinary_peak,3, 'finished slices did not release their slots')

        for supplied in ('', '\n'.join(names), '\n'.join(reversed(names[::2]))):
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
            self.assertEqual(shard.run_task(('internal/a',['not-a-command']))['exit'],127)

    def test_helpers_are_part_of_the_exact_main_controls_copied_to_patches(self):
        stable_patch = (ROOT / 'scripts/release_control/stable_patch.py').read_text()
        self.assertIn('"scripts/release_control"', stable_patch)
        self.assertTrue(RUNNER.is_relative_to(ROOT / 'scripts/release_control'))
        self.assertTrue(SHARD.is_relative_to(ROOT / 'scripts/release_control'))
        self.assertTrue(Path(__file__).is_relative_to(ROOT / 'scripts/release_control'))
        self.assertIn('python3 scripts/release_control/run_go_package_slice_test.py', job('backend'))


if __name__ == "__main__":
    unittest.main()
