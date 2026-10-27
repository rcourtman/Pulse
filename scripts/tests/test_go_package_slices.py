"""Exercise the actual sliced Go runner, recorder and required-check verdict."""

import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / ".github/scripts/run-go-package-slice.py"
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
    def test_fixed_matrix_and_whole_package_exclusion_match_the_runner(self):
        slow = job('backend-slow')
        self.assertIn('suite: [monitoring, installtests]', slow)
        self.assertIn('index: [0, 1]', slow)
        self.assertEqual(re.findall(r'            package: (\S+)', slow), list(runner.PACKAGES))
        for required in ('fail-fast: false', 'needs: [changes, test-compile]',
                         'needs.changes.outputs.code == \'true\'', 'persist-credentials: false',
                         'if: always()', 'run-go-package-slice.py', '--package "$TEST_PACKAGE"',
                         '--index "$TEST_INDEX"', 'backend-slice-${{ matrix.suite }}-${{ matrix.index }}'):
            self.assertIn(required, slow)
        self.assertNotIn('continue-on-error', slow)
        self.assertIn("grep -vE '/(internal/api|internal/monitoring|scripts/installtests)$'",
                      job('backend-packages'))
        self.assertIn('shard: [rest-0, rest-1]', job('backend'))
        self.assertIn('name: Backend tests (${{ matrix.shard }})', job('backend'))
        self.assertIn('needs: [changes, backend-packages, backend-slow]', job('backend'))
        self.assertIn('if: always()', job('backend'))

    def test_actual_required_verdict_rejects_every_unavailable_or_incomplete_matrix(self):
        verdict = job('backend').split('        run: |\n', 1)[1]
        script = '\n'.join(line[10:] for line in verdict.splitlines()
                           if line.startswith('          ') or not line.strip())
        for code, slices in (('true', 'success'), ('false', 'skipped')):
            env = {**os.environ, 'CHANGE_RESULT': 'success', 'CODE_REQUIRED': code,
                   'PACKAGES_RESULT': 'success', 'SLICES_RESULT': slices}
            result = subprocess.run(['bash', '-c', script], env=env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            for field in ('CHANGE_RESULT', 'PACKAGES_RESULT', 'SLICES_RESULT', 'CODE_REQUIRED'):
                for value in ('', 'failure', 'cancelled', 'unknown', 'true' if field != 'CODE_REQUIRED' else 'other'):
                    with self.subTest(code=code, field=field, value=value):
                        result = subprocess.run(['bash', '-c', script], env={**env, field: value},
                                                capture_output=True)
                        self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
