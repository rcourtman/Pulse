#!/usr/bin/env python3
"""Keep External watchdog Help aligned with AlertDeadManDestinationSection.tsx.

Static source/document checks only: no watchdog URL, network request, or live
alert operation is performed.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/AVAILABILITY.md"
MIRROR = ROOT / "frontend-modern/public/docs/AVAILABILITY.md"
COMPONENT = ROOT / "frontend-modern/src/features/alerts/AlertDeadManDestinationSection.tsx"


def plain(text: str) -> str:
    return " ".join(text.replace("**", "").replace("`", "").split())


class WatchdogHelpDocsTest(unittest.TestCase):
    def setUp(self) -> None:
        self.doc = DOC.read_text(encoding="utf-8")
        self.guide = plain(self.doc)
        self.component = COMPONENT.read_text(encoding="utf-8")

    def test_shipped_guide_matches(self) -> None:
        self.assertEqual(DOC.read_bytes(), MIRROR.read_bytes())

    def test_watchdog_status_badges_match_runtime_presentation(self) -> None:
        for label in (
            "Heartbeat healthy",
            "No recent heartbeat",
            "Status read overdue",
            "Status unavailable",
            "Delivery failing",
            "Monitoring stalled",
            "Starting",
            "Not configured",
            "Configuration invalid",
            "Configuration unavailable",
        ):
            with self.subTest(label=label):
                self.assertIn(f"'{label}'", self.component)
                self.assertIn(label, self.guide)

    def test_watchdog_polling_and_interruption_details_match_component(self) -> None:
        self.assertIn("DEAD_MAN_STATUS_POLL_MS = 30_000", self.component)
        self.assertIn("DEAD_MAN_STATUS_READ_OVERDUE_MS = 2 * DEAD_MAN_STATUS_POLL_MS", self.component)
        self.assertIn("recommendedGraceSeconds", self.component)
        self.assertIn("Last interruption", self.component)
        self.assertIn("clean restart", self.component)
        self.assertIn("unexpected stop", self.component)

        for phrase in (
            "Alerts → Notifications → External watchdog",
            "3-minute grace period",
            "A live status read does not mean a heartbeat was sent",
            "two 30-second poll intervals (60 seconds)",
            "Use Refresh to retry",
            "last known snapshot",
            "Last interruption",
            "clean restart",
            "unexpected stop",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, self.guide)

    def test_availability_lifecycle_truthfulness_matches_poller_source(self) -> None:
        poller = (ROOT / "internal/monitoring/availability_poller.go").read_text(encoding="utf-8")
        probe_agent = (ROOT / "internal/monitoring/availability_probe_agent.go").read_text(
            encoding="utf-8"
        )
        staleness = (ROOT / "internal/monitoring/staleness_tracker.go").read_text(encoding="utf-8")

        for token in (
            "availabilityRefreshMu",
            "currentLocalAvailabilityTarget",
            "pausedIDs",
            "context.Canceled",
        ):
            with self.subTest(poller_token=token):
                self.assertIn(token, poller)
        self.assertIn("Rejecting probe availability result for disabled target", probe_agent)
        self.assertIn("func (t *StalenessTracker) Remove(", staleness)

        for phrase in (
            "clears its scheduler health, retry, failure, dead-letter and staleness entries",
            "Local probes cancelled during a server reload or shutdown are discarded",
            "in-flight local or remote results for a check that was paused, deleted, edited or reassigned to remote-only locations are dropped",
            "clears its pre-pause observations and external-probe grace tracking without raising stale-probe errors while paused",
            "starts a fresh first-report grace window instead of immediately reporting an unavailable probe",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, self.guide)

    def test_watchdog_recovery_and_replacement_truthfulness_matches_deadman_source(self) -> None:
        deadman = (ROOT / "internal/monitoring/deadman.go").read_text(encoding="utf-8")
        for token in (
            "func (d *deadManRuntime) recordHealthyProgress(",
            "func (d *deadManRuntime) recordSignalFailure(",
            "func (d *deadManRuntime) notifyConfigChangedFor(",
            "d.status.LastSuccessAt = nil",
            "d.status.LastAttemptAt = nil",
            "if d.stopping || ctx.Err() != nil {",
        ):
            with self.subTest(deadman_token=token):
                self.assertIn(token, deadman)

        self.assertIn("Last success", self.component)
        self.assertNotIn("Last attempt", self.component)
        self.assertIn('json:"lastAttemptAt,omitempty"', deadman)
        for phrase in (
            "Monitoring stalled clears immediately and moves to Starting or Delivery failing if the recovery heartbeat fails to deliver",
            "Replacing, removing, or saving an invalid watchdog URL clears prior Last success and API lastAttemptAt timestamps",
            "clears prior delivery alerts on replacement",
            "re-saving the same configured URL preserves its Last success",
            "Concurrent shutdown or configuration replacement cancels superseded watchdog cycles",
            "cannot overwrite a clean shutdown record or the new destination's status",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, self.guide)

    def test_poll_planning_retirement_and_panic_logging_help_matches_runtime(self) -> None:
        alerting_path = ROOT / "docs/ALERTING.md"
        alerting_mirror = ROOT / "frontend-modern/public/docs/ALERTING.md"
        self.assertEqual(alerting_path.read_bytes(), alerting_mirror.read_bytes())

        schedule_src = (ROOT / "internal/monitoring/monitor_polling_schedule.go").read_text(
            encoding="utf-8"
        )
        progress_src = (ROOT / "internal/monitoring/poll_work_progress.go").read_text(
            encoding="utf-8"
        )
        workers_src = (ROOT / "internal/monitoring/poll_task_workers.go").read_text(
            encoding="utf-8"
        )

        for token in (
            "func (m *Monitor) planScheduledPolls(ctx context.Context, now time.Time)",
            "id := progress.beginPlanning(time.Now())",
            "defer progress.finishPlanning(id)",
            "descriptors := m.describeInstancesForScheduler()",
            "if ctx.Err() != nil {",
            "m.taskQueue.upsertPlan(task, revision)",
            "m.state.UpdatePollStats(",
            "m.syncAlertsToState()",
        ):
            with self.subTest(schedule_token=token):
                self.assertIn(token, schedule_src)

        for token in (
            '"pulse_scheduler_poll_planning_age_seconds"',
            '"pulse_scheduler_claimed_poll_overdue_seconds"',
            '"waiting_for_slot", "running"',
        ):
            with self.subTest(progress_token=token):
                self.assertIn(token, progress_src)

        for token in (
            "defer progress.finish(work)",
            'Msg("Recovered from panic in scheduled poll task")',
            "defer func() { m.taskQueue.completeClaim(task, next, reschedule) }()",
        ):
            with self.subTest(workers_token=token):
                self.assertIn(token, workers_src)
        self.assertLess(
            workers_src.index("defer progress.finish(work)"),
            workers_src.index('Msg("Recovered from panic in scheduled poll task")'),
        )
        self.assertLess(
            workers_src.index('Msg("Recovered from panic in scheduled poll task")'),
            workers_src.index(
                "defer func() { m.taskQueue.completeClaim(task, next, reschedule) }()"
            ),
        )

        docs_checks = (
            (
                "docs/ALERTING.md",
                (
                    'stage="running"',
                    "panic recovery logging after releasing its queue claim",
                    "Unfinished poll planning:",
                    "pulse_scheduler_poll_planning_age_seconds",
                    "reading provider rosters, building the plan, or queuing tasks",
                    "When a monitor is retired during a planning pass, Pulse abandons the returned plan once cancellation is observed instead of queuing tasks or running post-plan statistics and alert reconciliation",
                    "keeping any still-running roster read visible in pulse_scheduler_poll_planning_age_seconds until it returns",
                ),
            ),
            (
                "docs/monitoring/PROMETHEUS_METRICS.md",
                (
                    "stays visible in this stage while occupied in panic recovery logging even after releasing its queue claim",
                    "When a monitor is retired while a planning pass is reading rosters, Pulse abandons the returned plan once cancellation is observed instead of queuing tasks or running post-plan statistics and alert reconciliation",
                    "keeping the still-running pass visible in pulse_scheduler_poll_planning_age_seconds until it returns",
                ),
            ),
            (
                "docs/api/SCHEDULER_HEALTH.md",
                (
                    'releases its queue claim during panic recovery logging stays visible under stage="running" until logging finishes',
                    "When a monitor is retired during poll planning, Pulse abandons the returned plan once cancellation is observed instead of queuing tasks or running post-plan statistics and alert reconciliation",
                    "keeping any still-running roster read visible in pulse_scheduler_poll_planning_age_seconds until it returns",
                ),
            ),
        )
        for rel_path, phrases in docs_checks:
            text = plain((ROOT / rel_path).read_text(encoding="utf-8"))
            for phrase in phrases:
                with self.subTest(doc=rel_path, phrase=phrase):
                    self.assertIn(phrase, text)


if __name__ == "__main__":
    unittest.main()
