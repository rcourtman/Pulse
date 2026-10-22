"""Keep missing-report help distinct from authentication and storage recovery.

Documentation/source consistency only. Runtime controls use synthetic usage
calls in hostmetrics; neither suite establishes installed NFS recovery.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "Agent is healthy locally but host reports are stale"
FRAGMENT = "agent-is-healthy-locally-but-host-reports-are-stale"


def section(text, heading):
    return " ".join(text.split("### " + heading + "\n", 1)[1].split("\n### ", 1)[0].split())


class AgentReportingHelpTest(unittest.TestCase):
    def setUp(self):
        self.guide = (ROOT / "docs/UNIFIED_AGENT.md").read_text()

    def test_symptom_and_filter_links_reach_the_same_shipped_help(self):
        index = (ROOT / "docs/TROUBLESHOOTING.md").read_text().split("## ⚡ Quick Fixes", 1)[0]
        self.assertIn("UNIFIED_AGENT.md#" + FRAGMENT, index)
        filters = section(self.guide, "Include an Automatically Filtered Filesystem")
        self.assertIn("](#" + FRAGMENT + ")", filters)
        help_text = section(self.guide, HEADING)
        self.assertIn("](#include-an-automatically-filtered-filesystem)", help_text)
        self.assertIn("](#agent-lookup-returns-http-401-or-403)", help_text)
        for name in ("UNIFIED_AGENT", "TROUBLESHOOTING"):
            self.assertEqual((ROOT / f"docs/{name}.md").read_bytes(),
                             (ROOT / f"frontend-modern/public/docs/{name}.md").read_bytes())

    def test_filter_guidance_explains_the_real_cost_of_opt_in(self):
        text = section(self.guide, "Include an Automatically Filtered Filesystem")
        for distinction in ("NFS and CIFS/SMB", "before asking for their usage",
                            "unless an include pattern matches", "active filesystem calls",
                            "`--disk-exclude` match still wins", "capacity unmonitored",
                            "physical-disk health and wear alerts", "independent coverage",
                            "do not remove a working exclusion", "not the underlying kernel call"):
            self.assertIn(distinction, text)

    def test_local_health_acceptance_and_kernel_recovery_stay_separate(self):
        text = section(self.guide, HEADING)
        for distinction in ("`/healthz` success does **not** prove", "**Last seen**",
                            "missing filesystem usage is not zero", "HTTP 401 or 403",
                            "not a diagnosis from staleness alone", "kernel call can remain blocked",
                            "not proof of a recovered mount or accepted host reports"):
            self.assertIn(distinction, text)

    def test_passive_help_does_not_create_a_dangerous_diagnostic_recipe(self):
        text = section(self.guide, HEADING)
        for boundary in ("Do not rotate tokens", "delete identity state",
                         "Do not run `df`", "force an unmount", "hard/soft semantics",
                         "repeatedly restart or kill", "safe maintenance",
                         "do not use an exclusion to disguise an unexplained outage",
                         "raw process or kernel dumps private", "Unknown evidence is valid"):
            self.assertIn(boundary, text)
        self.assertNotIn("```", text)
        self.assertNotIn("--token", text)

    def test_filter_order_matches_the_documented_collector(self):
        source = (ROOT / "internal/hostmetrics/collector.go").read_text()
        loop = source.split("for _, part := range partitions {", 1)[1]
        order = [loop.index(marker) for marker in (
            "fsfilters.MatchesDiskExclude", "fsfilters.MatchesDiskInclude",
            "fsfilters.ShouldSkipFilesystem", "guardedDiskUsage(ctx, part.Mountpoint)")]
        self.assertEqual(order, sorted(order))
        self.assertIn("if !isZFS && !explicitlyIncluded", loop)
        controls = (ROOT / "internal/hostmetrics/collector_stuck_mount_test.go").read_text()
        for control in ("TestCollectDisks_NetworkMountsNeverReachUsageSyscall",
                        "TestCollectDisks_UnresponsiveMountDoesNotBlockCycle",
                        "TestGuardedDiskUsage_CancelledWaitKeepsProbe"):
            self.assertIn("func " + control + "(", controls)


if __name__ == "__main__":
    unittest.main()
