import unittest

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
        proof.validate_output(passed, 0, strict=True)
        for text, status in [(report + '--- SKIP: TestSLO_WorkloadCharts (1.0s)\n', 0),
                             (passed.replace('90ms', '550ms'), 0), (passed, 1), ('PASS\n', 0),
                             (passed + report, 0), (passed + passed, 0),
                             (passed + '--- SKIP: TestSLO_WorkloadCharts (1.0s)\n', 0)]:
            with self.subTest(text=text, status=status):
                with self.assertRaises(RuntimeError):
                    proof.validate_output(text, status, strict=True)


if __name__ == '__main__':
    unittest.main()
