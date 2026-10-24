import tempfile
import unittest
from pathlib import Path

import chart_latency_proof as proof


class ChartLatencyProofTests(unittest.TestCase):
    def test_guest_time_is_not_counted_twice(self):
        before = dict(time=0, cpu=[0] * 10, swaps={'pswpin': 0, 'pswpout': 0})
        after = dict(time=1, cpu=[20, 0, 0, 80, 0, 0, 0, 0, 10, 0],
                     swaps=before['swaps'], load=[0.5, 0.5, 0.5], pressure={})
        row = proof.interval(before, after)
        self.assertEqual(row['total_ticks'], 100)
        self.assertEqual(row['busy_pct'], 20)
        self.assertEqual(row['guest_pct'], 10)

    def test_idle_requires_cpu_io_and_swap_quiet(self):
        row = dict(total_ticks=100, load=[1, 1, 1], idle_pct=95, iowait_pct=0,
                   steal_pct=0, guest_pct=0, swap_pages=0)
        self.assertTrue(proof.quiet(row))
        for field, value in [('total_ticks', 0), ('load', [2, 1, 1]), ('idle_pct', 89),
                             ('iowait_pct', 2), ('steal_pct', 1), ('guest_pct', 3), ('swap_pages', 1)]:
            with self.subTest(field=field):
                self.assertFalse(proof.quiet(dict(row, **{field: value})))

    def test_skip_or_hosted_pass_cannot_satisfy_local_slo(self):
        report = 'charts/workloads p50=40ms p95=50ms p99=60ms SLO=90ms\n'
        passed = report + '--- PASS: TestSLO_WorkloadCharts (1.0s)\n'
        proof.validate_output(passed, 0)
        for text, status in [(report + '--- SKIP: TestSLO_WorkloadCharts (1.0s)\n', 0),
                             (passed.replace('90ms', '550ms'), 0), (passed, 1), ('PASS\n', 0),
                             (passed + report, 0), (passed + passed, 0),
                             (passed + '--- SKIP: TestSLO_WorkloadCharts (1.0s)\n', 0)]:
            with self.subTest(text=text, status=status):
                with self.assertRaises(RuntimeError):
                    proof.validate_output(text, status)

    def test_contention_voids_only_the_latency_verdict(self):
        report = '    slo_bench_test.go:637: charts/workloads p50=75ms p95=107ms p99=112ms SLO=90ms\n'
        violation = '    slo_bench_test.go:637: SLO VIOLATION: p95=107ms exceeds target 90ms\n'
        passed = report + '--- PASS: TestSLO_WorkloadCharts (1.0s)\n'
        failed = report + violation + '--- FAIL: TestSLO_WorkloadCharts (1.0s)\nFAIL\n'
        for text, status in [(passed, 0), (failed, 1)]:
            with self.subTest(text=text):
                self.assertEqual(proof.validate_contended_output(text, status),
                                 report.split(': ', 1)[1].strip())
        cleanup = '    testing.go:1369: TempDir RemoveAll cleanup: directory not empty\n'
        for text, status in [(report + '--- SKIP: TestSLO_WorkloadCharts (1.0s)\n', 0),
                             (failed.replace(violation, ''), 1), (failed, 2), (passed, 1),
                             (failed.replace(violation, violation + cleanup), 1),
                             (failed.replace(violation, violation + violation), 1),
                             ('--- FAIL: TestSLO_WorkloadCharts (1.0s)\n', 1),
                             (passed.replace('90ms', '550ms'), 0), (passed + passed, 0)]:
            with self.subTest(text=text, status=status):
                with self.assertRaises(RuntimeError):
                    proof.validate_contended_output(text, status)

    def test_inconclusive_summary_names_competing_processes(self):
        errors = [dict(background_busy_pct=25.2, background_processes=[
                      dict(pid=1, name='provjobd312033', cpu_pct=25.2),
                      dict(pid=2, name='python3', cpu_pct=0.2)]),
                  dict(background_busy_pct=14.4, background_processes=[
                      dict(pid=1, name='provjobd312033', cpu_pct=14.2)])]
        with tempfile.TemporaryDirectory() as directory:
            summary = Path(directory) / 'summary.md'
            proof.report_inconclusive('charts/workloads p50=1ms p95=2ms p99=3ms SLO=90ms',
                                      errors, summary)
            text = summary.read_text()
        self.assertIn('inconclusive', text)
        self.assertIn('2 measurement intervals', text)
        self.assertIn('peak 25.2%', text)
        self.assertIn('provjobd312033', text)
        self.assertNotIn('python3', text)

    def test_measurement_io_stays_in_latency_and_background_cpu_invalidates(self):
        row = dict(background_busy_pct=0, guest_pct=0, steal_pct=0, swap_pages=0,
                   iowait_pct=20)
        self.assertFalse(proof.competing_capacity(row))
        for field, value in [('background_busy_pct', 25), ('guest_pct', 3),
                             ('steal_pct', 1), ('swap_pages', 1)]:
            with self.subTest(field=field):
                self.assertTrue(proof.competing_capacity(dict(row, **{field: value})))

    def test_bounded_hosted_agent_is_attributed_without_changing_raw_evidence(self):
        row = dict(background_busy_pct=26.799007444168744, guest_pct=0, steal_pct=0,
                   swap_pages=0, background_processes=[
                       dict(pid=2106, name='provjobd1892260', cpu_pct=25.31017369727047),
                       dict(pid=45, name='kworker/u16:1-flush-8:0', cpu_pct=1.2406947890818858),
                       dict(pid=3197, name='python3', cpu_pct=0.49627791563275436)])
        original = repr(row)
        capacity = proof.background_capacity(row, hosted=True, cpu_count=4)
        self.assertEqual(capacity['runner_agent_cpu_pct'], row['background_processes'][0]['cpu_pct'])
        self.assertAlmostEqual(capacity['competing_busy_pct'], 1.7369727047146402)
        self.assertFalse(proof.competing_capacity(dict(row, **capacity)))
        self.assertEqual(repr(row), original)

    def test_agent_exception_requires_hosted_context_exact_name_and_one_cpu_bound(self):
        for hosted, cpus, name, cpu in [
                (False, 4, 'provjobd1892260', 25),
                (True, None, 'provjobd1892260', 25),
                (True, 0, 'provjobd1892260', 25),
                (True, 8, 'provjobd1892260', 25),
                (True, 4, 'provjobd1892260', 26.01),
                (True, 4, 'provjobd', 25),
                (True, 4, 'provjobd1892260-extra', 25),
                (True, 4, 'provjobd-1', 25),
                (True, 4, 'Runner.Worker', 25),
                (True, 4, 'node', 25)]:
            with self.subTest(hosted=hosted, cpus=cpus, name=name, cpu=cpu):
                row = dict(background_busy_pct=cpu, background_processes=[
                    dict(pid=1, name=name, cpu_pct=cpu)])
                capacity = proof.background_capacity(row, hosted=hosted, cpu_count=cpus)
                self.assertEqual(capacity['runner_agent_cpu_pct'], 0)
                self.assertEqual(capacity['competing_busy_pct'], cpu)

    def test_other_unattributed_or_multiple_agent_cpu_stays_competing(self):
        agent = dict(pid=1, name='provjobd1892260', cpu_pct=25)
        other = dict(pid=2, name='node', cpu_pct=11)
        for background, processes in [
                (36, [agent, other]), (5, [agent, other]), (36, [agent]),
                (25, []), (25, [agent, dict(pid=2, name='provjobd2772537', cpu_pct=0.25)])]:
            with self.subTest(background=background, processes=processes):
                row = dict(background_busy_pct=background, guest_pct=0, steal_pct=0,
                           swap_pages=0, background_processes=processes)
                capacity = proof.background_capacity(row, hosted=True, cpu_count=4)
                self.assertTrue(proof.competing_capacity(dict(row, **capacity)))

    def test_runner_agent_cannot_clear_guest_steal_or_swap(self):
        row = dict(background_busy_pct=25, guest_pct=0, steal_pct=0, swap_pages=0,
                   background_processes=[dict(pid=1, name='provjobd1892260', cpu_pct=25)])
        row.update(proof.background_capacity(row, hosted=True, cpu_count=4))
        self.assertFalse(proof.competing_capacity(row))
        for field, value in [('guest_pct', 2.01), ('steal_pct', 0.11), ('swap_pages', 1)]:
            with self.subTest(field=field):
                self.assertTrue(proof.competing_capacity(dict(row, **{field: value})))

    def test_inconclusive_summary_separates_agent_from_actual_contention(self):
        row = dict(background_busy_pct=36, background_processes=[
            dict(pid=1, name='provjobd1892260', cpu_pct=25),
            dict(pid=2, name='node', cpu_pct=11)])
        row.update(proof.background_capacity(row, hosted=True, cpu_count=4))
        with tempfile.TemporaryDirectory() as directory:
            summary = Path(directory) / 'summary.md'
            proof.report_inconclusive('charts/workloads p50=1ms p95=2ms p99=3ms SLO=90ms',
                                      [row], summary)
            text = summary.read_text()
        self.assertIn('peak 11.0% competing CPU; processes: node', text)
        self.assertNotIn('provjobd', text)

    def test_process_receipt_excludes_measurement_and_handles_new_or_reused_pids(self):
        before = {10: dict(name='test', ticks=20, start='1'),
                  20: dict(name='other', ticks=30, start='2'),
                  30: dict(name='old', ticks=100, start='3')}
        after = {10: dict(name='test', ticks=200, start='1'),
                 20: dict(name='other', ticks=55, start='2'),
                 30: dict(name='new', ticks=10, start='4'),
                 40: dict(name='new process', ticks=5, start='5')}
        self.assertEqual(proof.background_processes(before, after, 10, 100), [
            dict(pid=20, name='other', cpu_pct=25),
            dict(pid=30, name='new', cpu_pct=10),
            dict(pid=40, name='new process', cpu_pct=5)])


if __name__ == '__main__':
    unittest.main()
