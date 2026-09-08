import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import os
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / 'scripts/release-go-test-events.py'
spec = importlib.util.spec_from_file_location('events', SCRIPT)
events = importlib.util.module_from_spec(spec)
spec.loader.exec_module(events)


class EventsTest(unittest.TestCase):
    def test_output_and_bounded_evidence(self):
        data = [dict(Time='2026-09-08T19:00:00Z', Action=action,
                     Package=events.PACKAGE, Test=events.TARGET, Elapsed=2.0)
                for action in ('run', 'pause', 'cont', 'pass', 'fail', 'skip')]
        data += [dict(Action='output', Output='FAIL\tpackage\t2s\n'),
                 dict(Action='run', Package='other', Test=events.TARGET),
                 dict(Action='run', Package=events.PACKAGE, Test='TestOther')]
        output = io.StringIO()
        with patch.object(events.resources, 'snapshot', return_value={'unix_time_ns': 456}) as sample:
            self.assertEqual(events.render(io.StringIO(''.join(json.dumps(x)+'\n' for x in data)), output), 0)
        self.assertEqual(sample.call_count, 6)
        lines = output.getvalue().splitlines()
        self.assertEqual(lines[-1], 'FAIL\tpackage\t2s')
        for line in lines[:-1]:
            evidence = json.loads(line.removeprefix('RELEASE_GO_TEST_EVENT '))
            self.assertEqual(evidence['Time'], data[0]['Time'])
            self.assertGreater(evidence['received_unix_time_ns'], 456)
            self.assertEqual(evidence['resources'], {'unix_time_ns': 456})

    def test_unavailable_snapshot_does_not_change_verdict(self):
        event = dict(Action='fail', Package=events.PACKAGE, Test=events.TARGET)
        output = io.StringIO()
        with patch.object(events.resources, 'snapshot', side_effect=OSError):
            self.assertEqual(events.render(io.StringIO(json.dumps(event)+'\n'), output), 0)
        self.assertIn('snapshot collection failed', output.getvalue())

    def test_invalid_input_is_retained_and_drained(self):
        output = io.StringIO()
        self.assertEqual(events.render(io.StringIO('not-json\n[]\n{"Output":"later\\n"}\n'), output), 1)
        self.assertTrue(output.getvalue().startswith('not-json\n[]\nlater\n'))

    def test_real_go_pass_skip_and_fail(self):
        # A disposable standard-library-only package, never the product suite.
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            (root / 'go.mod').write_text('module ' + events.PACKAGE + '\n\ngo 1.26.0\n')
            (root / 'event_test.go').write_text('''package api
import ("os"; "testing")
func TestMultiTenant_ConcurrentAPIStress(t *testing.T) {
 t.Log("synthetic log")
 if os.Getenv("FIXTURE_FAIL") == "1" { t.Fatal("synthetic failure") }
}
func TestSkipped(t *testing.T) { t.Skip("synthetic skip") }
''')
            for fail in ('0', '1'):
                env = {**os.environ, 'GOWORK': 'off', 'FIXTURE_FAIL': fail}
                result = subprocess.run(['bash', '-o', 'pipefail', '-c',
                    'go test -json -p 1 -count=1 . | "$1" "$2"',
                    'fixture', sys.executable, str(SCRIPT)], cwd=root, env=env,
                    capture_output=True, text=True, timeout=60)
                self.assertEqual(result.returncode, int(fail), result.stderr)
                records = [json.loads(line.removeprefix('RELEASE_GO_TEST_EVENT '))
                           for line in result.stdout.splitlines()
                           if line.startswith('RELEASE_GO_TEST_EVENT ')]
                self.assertEqual([r['Action'] for r in records],
                                 ['run', 'fail' if fail == '1' else 'pass'])
                self.assertTrue(all(r.get('Time') for r in records))
                self.assertIn('synthetic log', result.stdout)
                self.assertIn('--- SKIP: TestSkipped', result.stdout)
                self.assertIn('FAIL' if fail == '1' else 'PASS', result.stdout)

    def test_pipeline_preserves_producer_exit(self):
        for code in (0, 17):
            result = subprocess.run(['bash', '-o', 'pipefail', '-c',
                '(printf \'%s\\n\' \'{"Action":"output","Output":"FAIL\\n"}\'; exit "$1") | "$2" "$3"',
                'fixture', str(code), sys.executable, str(SCRIPT)], capture_output=True, text=True)
            self.assertEqual(result.returncode, code, result.stderr)
            self.assertEqual(result.stdout, 'FAIL\n')


if __name__ == '__main__':
    unittest.main()
