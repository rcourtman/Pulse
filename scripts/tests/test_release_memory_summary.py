#!/usr/bin/env python3
"""Check launch parity and the release memory summary's observations."""

import importlib.util
import io
import json
import os
import subprocess
from contextlib import redirect_stdout
from pathlib import Path
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location(
    "summarize_release_memory", ROOT / "scripts/memory/summarize_release_memory.py"
)
summary = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(summary)

HEADER = "elapsed_s\trss_kib\tgo_heap_kib\tgo_other_kib\tanon_other_kib\tfile_kib\tcpu_ticks\twrite_bytes\twchar_bytes\n"
STABLE_TAG, BASELINE_SHA, CANDIDATE_SHA = "v6.8.0", "a" * 40, "b" * 40
SOURCE_ARGS = (STABLE_TAG, BASELINE_SHA, CANDIDATE_SHA)
SOURCE_GIT_SHIM = r'''git() {
  [[ "$1" == -C ]] || return 2
  case "${2##*/}" in
    stable) sha="$PULSE_TEST_BASELINE_SHA" ;;
    main) sha="$PULSE_TEST_CANDIDATE_SHA" ;;
    *) return 2 ;;
  esac
  case "$3 $4 $5" in
    "rev-parse --verify HEAD") printf '%s\n' "$sha" ;;
    "diff --quiet HEAD") [[ "$6" == -- && "${2##*/}" != "${PULSE_TEST_DIRTY_SOURCE-}" ]] ;;
    *) return 2 ;;
  esac
}
export -f git
'''


def source_env():
    return {"PULSE_TEST_BASELINE_SHA": BASELINE_SHA, "PULSE_TEST_CANDIDATE_SHA": CANDIDATE_SHA}


def samples(rss_kib, ticks_per_s, write_bytes_per_s, io=True, rss_hwm=None, end=900, interval=15):
    rows = [HEADER if rss_hwm is None else HEADER.rstrip("\n") + "\trss_hwm_kib\n"]
    for elapsed in range(0, end + 1, interval):
        write = str(write_bytes_per_s * elapsed) if io else ""
        wchar = str(2 * write_bytes_per_s * elapsed) if io else ""
        row = f"{elapsed}\t{rss_kib}\t50000\t10000\t20000\t30000\t{ticks_per_s * elapsed}\t{write}\t{wchar}"
        if rss_hwm is not None:
            row += f"\t{rss_hwm(elapsed)}"
        rows.append(row + "\n")
    return "".join(rows)


def select_samples(raw, keep):
    header, *rows = raw.splitlines(keepends=True)
    return header + "".join(row for row in rows if keep(int(row.split("\t", 1)[0])))


def archive_stats(window_s=300):
    return {
        "storage": "nfs-backups",
        "rows_by_content": {"backup": 840, "iso": 24, "vztmpl": 24},
        "body_bytes": 144000,
        "body_sha256": "c" * 64,
        "window_s": window_s,
        "nodes": {"pve1": {"responses": 10, "steady_responses": 6}},
    }


class ReleaseMemorySummaryTests(unittest.TestCase):
    def run_summary(self, baseline, candidate, estate=None, agents=None, env=None, duration=900, interval=15,
                    wait_statuses=(), child_wait_statuses=None, proxmox=None):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            (out / "samples-baseline.tsv").write_text(baseline)
            (out / "samples-candidate.tsv").write_text(candidate)
            (out / "clk_tck").write_text("100\n")
            if estate:
                (out / "estate").write_text(estate + "\n")
            for label, stats in (agents or {}).items():
                (out / f"agents-{label}.json").write_text(json.dumps(stats))
            for label, stats in (proxmox or {}).items():
                (out / f"proxmox-{label}.json").write_text(json.dumps(stats))
            for label, status in (child_wait_statuses or {}).items():
                (out / f"run-{label}-wait.json").write_text(json.dumps(status))
            saved = {key: os.environ.get(key) for key in (env or {})}
            os.environ.update(env or {})
            try:
                with redirect_stdout(io.StringIO()):
                    code = summary.main(["summarize", str(out), str(duration), str(interval), *map(str, wait_statuses)])
            finally:
                for key, value in saved.items():
                    if value is None:
                        os.environ.pop(key, None)
                    else:
                        os.environ[key] = value
            return code, json.loads((out / "result.json").read_text()), (out / "summary.md").read_text()

    def test_agents_estate_reports_traffic_and_can_skip_the_gate(self):
        stats = {
            "agents": 30, "docker_agents": 6, "containers_per_docker_agent": 40, "interval_s": 30,
            "accepted": {"host": 900, "docker": 180}, "failed": {"host": 1, "docker": 0},
            "dashboard": {"connects": 1, "messages": 452, "bytes": 3 * 1024 * 1024,
                          "state_messages": 450, "state_updates": 449},
        }
        code, result, text = self.run_summary(
            samples(130000, 25, 0), samples(170000, 25, 0), estate="agents",
            agents={"baseline": stats, "candidate": stats}, env={"PULSE_MEMORY_GATE": "0"},
        )
        self.assertEqual(code, 0)
        self.assertTrue(result["exceeded"])
        self.assertFalse(result["regressed"])
        self.assertIn("(agents estate)", text)
        self.assertIn("Reported only: this estate does not gate.", text)
        self.assertIn("| Host reports accepted | 900 | 900 |", text)
        self.assertIn("| Dashboard MiB received | 3.0 | 3.0 |", text)
        self.assertIn("| Dashboard application messages | 452 | 452 |", text)
        self.assertIn("| Dashboard state messages | 450 | 450 |", text)
        self.assertIn("| Dashboard state updates | 449 | 449 |", text)
        self.assertIn("| Agents with repeated host report acceptance | n/a | n/a |", text)
        self.assertNotIn("Mock mode keeps", text)

    def test_incomplete_agent_measurement_keeps_window_traffic_without_an_rss_verdict(self):
        stats = {
            "agents": 2, "docker_agents": 1, "containers_per_docker_agent": 40, "interval_s": 30,
            "accepted": {"host": 60, "docker": 30}, "failed": {"host": 0, "docker": 0},
            "dashboard": {"connects": 1, "messages": 90, "bytes": 1024, "state_updates": 89},
        }
        healthy = {
            "window_s": 300, "end_reached": True, "complete": True, "dashboard_state_updates": 41,
            "reports": {
                "host": {"expected": 2, "accepted": 20, "agents_with_repeated_acceptance": 2},
                "docker": {"expected": 1, "accepted": 10, "agents_with_repeated_acceptance": 1},
            },
        }
        thin = {
            **healthy, "complete": False, "dashboard_state_updates": 0,
            "reports": {**healthy["reports"], "host": {
                "expected": 2, "accepted": 10, "agents_with_repeated_acceptance": 1,
            }},
        }
        code, result, text = self.run_summary(
            samples(130000, 25, 0), samples(95000, 25, 0), estate="agents",
            agents={"baseline": dict(stats, steady_traffic=healthy), "candidate": dict(stats, steady_traffic=thin)},
            env={"PULSE_MEMORY_GATE": "0"}, wait_statuses=(0, 1), child_wait_statuses={"candidate": {"driver_wait_status": 1}},
        )
        self.assertEqual(code, 1)
        self.assertEqual(result["status"], "incomplete")
        self.assertNotIn("rss_growth_kib", result)
        self.assertEqual(result["candidate_agents"]["steady_traffic"], thin)
        self.assertIn("Candidate run did not complete successfully", text)
        self.assertIn("| Agents with repeated host report acceptance | 2/2 (20 acceptances) | 1/2 (10 acceptances) |", text)
        self.assertIn("| Dashboard state updates | 41 | 0 |", text)

    def test_legacy_frame_counts_are_not_reported_as_verified_state(self):
        stats = {
            "agents": 30, "docker_agents": 6, "containers_per_docker_agent": 40, "interval_s": 30,
            "accepted": {"host": 900, "docker": 180}, "failed": {"host": 0, "docker": 0},
            "dashboard": {"connects": 1, "messages": 450, "bytes": 3 * 1024 * 1024},
        }
        lines = "\n".join(summary.agent_lines(stats, stats))
        self.assertIn("| Dashboard application messages | 450 | 450 |", lines)
        self.assertIn("| Dashboard state messages | n/a | n/a |", lines)
        self.assertIn("| Dashboard state updates | n/a | n/a |", lines)

    def test_proxmox_reports_served_pbs_groups_without_inventing_legacy_coverage(self):
        legacy = {
            "pve_nodes": 3, "guests": 35, "pbs_instances": 2, "total_pbs_snapshots": 5200,
            "snapshots_per_group": 50, "pve_requests": 914, "pbs_requests": 210,
            "dashboard": {"connects": 1, "messages": 60, "bytes": 1024},
        }
        candidate = dict(legacy, pve_guests_observed=35,
                         pve_guests_recent=34, pve_guest_freshness_window_s=60, pbs_snapshot_group_coverage={
            "pbs1": {"store-fast": {"served_groups": 35, "expected_groups": 35},
                     "store-archive": {"served_groups": 17, "expected_groups": 17}},
            "pbs2": {"store-fast": {"served_groups": 35, "expected_groups": 35},
                     "store-archive": {"served_groups": 5, "expected_groups": 17}},
        })
        lines = "\n".join(summary.proxmox_lines(legacy, candidate, 300))
        self.assertIn("| PVE guests observed in dashboard state | n/a | 35 |", lines)
        self.assertIn("| PVE guests with recent poll sightings at measurement end | n/a | 34/35 (last 60 s) |", lines)
        self.assertIn("| PVE guest poll sightings during steady RSS sampling | n/a | n/a |", lines)
        self.assertIn("| PVE archive entries per response | n/a | n/a |", lines)
        self.assertIn("| PVE archive responses by node (all / steady) | n/a | n/a |", lines)
        self.assertIn("| PBS API requests | 210 | 210 |", lines)
        self.assertIn("| PBS snapshot groups served/configured (whole run) | n/a | 92/104 |", lines)
        self.assertIn("No same-window snapshot-listing evidence is available.", lines)
        self.assertIn("| PBS datastore status responses during steady RSS sampling | n/a | n/a |", lines)
        self.assertIn("not snapshots Pulse retained or displayed", lines)

    def test_steady_proxmox_sighting_gap_is_reported_separately_from_the_rss_result(self):
        baseline = {
            "pve_nodes": 3, "guests": 35, "pbs_instances": 2, "total_pbs_snapshots": 5200,
            "snapshots_per_group": 50, "pve_requests": 914, "pbs_requests": 210,
            "pve_guests_recent": 35, "pve_guest_freshness_window_s": 60,
            "dashboard": {"connects": 1, "messages": 60, "bytes": 1024},
            "pve_steady_poll_sightings": {
                "guests_with_sightings": 35, "guests_over_limit": [], "limit_s": 60,
                "longest_gap_ms": 10000, "window_s": 300,
            },
        }
        candidate = dict(baseline, pve_steady_poll_sightings={
            **baseline["pve_steady_poll_sightings"],
            "guests_over_limit": ["lxc/ct-101"], "longest_gap_ms": 150000,
        })
        code, result, text = self.run_summary(
            samples(131072, 25, 0), samples(98304, 25, 0), estate="proxmox",
            proxmox={"baseline": baseline, "candidate": candidate},
            env={"PULSE_MEMORY_GATE": "0"}, wait_statuses=(0, 0),
        )
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "measured")
        self.assertEqual(result["rss_growth_kib"], -32768)
        self.assertIn("resident memory changed -32.0 MiB (-25.0%)", text)
        self.assertIn("Candidate: PVE guest sightings were missing or exceeded the 60 s gap limit.", text)
        self.assertIn("This RSS change does not establish memory use under equivalent polling.", text)
        self.assertNotIn("No steady-state RSS regression", text)
        self.assertIn("| PVE guests with recent poll sightings at measurement end | 35/35 (last 60 s) |"
                      " 35/35 (last 60 s) |", text)
        self.assertIn("| PVE guest poll sightings during steady RSS sampling |"
                      " 35/35 sighted; 0 with a gap over 60 s; longest 10 s in 300 s |"
                      " 35/35 sighted; 1 with a gap over 60 s; longest 150 s in 300 s |", text)
        self.assertIn("a gap does not identify where observation stopped", text)

    def test_steady_pbs_datastore_gap_is_visible_even_when_snapshot_coverage_matches(self):
        primary = {"store-fast": {"responses": 10, "longest_gap_ms": 30000},
                   "store-archive": {"responses": 10, "longest_gap_ms": 31000}}
        baseline = {
            "pve_nodes": 3, "guests": 35, "pbs_instances": 2, "total_pbs_snapshots": 5200,
            "snapshots_per_group": 50, "pve_requests": 914, "pbs_requests": 210,
            "dashboard": {"connects": 1, "messages": 60, "bytes": 1024},
            "pve_steady_poll_sightings": {
                "guests_with_sightings": 35, "guests_over_limit": [], "limit_s": 60,
                "longest_gap_ms": 10000, "window_s": 300,
            },
            "pbs_snapshot_group_coverage": {
                name: {"store-fast": {"served_groups": 35, "expected_groups": 35},
                       "store-archive": {"served_groups": 17, "expected_groups": 17}}
                for name in ("pbs1", "pbs2")
            },
            "pbs_steady_datastore_status": {
                "window_s": 300, "datastores": {"pbs1": primary, "pbs2": primary},
            },
            "pve_archive_inventory": archive_stats(),
        }
        candidate = dict(baseline, pbs_steady_datastore_status={
            "window_s": 300,
            "datastores": {
                "pbs1": primary,
                "pbs2": {**primary, "store-archive": {"responses": 0, "longest_gap_ms": 300000}},
            },
        })
        full = samples(131072, 25, 0)
        code, result, text = self.run_summary(
            full, full, estate="proxmox", proxmox={"baseline": baseline, "candidate": candidate},
            env={"PULSE_MEMORY_GATE": "0"}, wait_statuses=(0, 0),
        )
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "measured")
        self.assertIn("Candidate: 1 of 4 PBS datastores had no status response in the RSS window.", text)
        self.assertNotIn("- Baseline:", text)
        self.assertNotIn("No steady-state RSS regression", text)
        self.assertIn("| PBS snapshot groups served/configured (whole run) | 104/104 | 104/104 |", text)
        self.assertIn("| PBS datastore status responses during steady RSS sampling |"
                      " 4/4 with a response; longest gap 31 s in 300 s |"
                      " 3/4 with a response; longest gap 300 s in 300 s |", text)
        self.assertIn("not proof of backup reads or of Pulse accepting the response", text)

        _, _, intact = self.run_summary(
            full, full, estate="proxmox", proxmox={"baseline": baseline, "candidate": baseline},
            env={"PULSE_MEMORY_GATE": "0"}, wait_statuses=(0, 0),
        )
        self.assertNotIn("Polling evidence limits the comparison", intact)
        self.assertIn("Steady-state process RSS observation", intact)
        self.assertNotIn("No steady-state RSS regression", intact)

        older_window = dict(baseline, pbs_steady_datastore_status={
            **candidate["pbs_steady_datastore_status"], "window_s": 299,
        })
        _, _, mismatched = self.run_summary(
            full, full, estate="proxmox", proxmox={"baseline": older_window, "candidate": baseline},
            env={"PULSE_MEMORY_GATE": "0"}, wait_statuses=(0, 0),
        )
        self.assertIn("Baseline: PBS datastore status evidence is unavailable for the RSS window.", mismatched)
        self.assertNotIn("- Candidate:", mismatched)
        self.assertNotIn("PBS datastores had no status response in the RSS window.", mismatched)

    def test_archive_polling_limits_keep_rss_visible_and_do_not_grant_coverage_for_warmup(self):
        baseline = {
            "pve_nodes": 1, "guests": 35, "pbs_instances": 0, "total_pbs_snapshots": 0,
            "snapshots_per_group": 50, "pve_requests": 914, "pbs_requests": 0,
            "dashboard": {"connects": 1, "messages": 60, "bytes": 1024},
            "pve_steady_poll_sightings": {
                "guests_with_sightings": 35, "guests_over_limit": [], "limit_s": 60,
                "longest_gap_ms": 10000, "window_s": 300,
            },
            "pve_archive_inventory": archive_stats(),
        }

        def compare(candidate):
            return self.run_summary(
                samples(131072, 25, 0), samples(98304, 25, 0), estate="proxmox",
                proxmox={"baseline": baseline, "candidate": candidate},
                env={"PULSE_MEMORY_GATE": "0"}, wait_statuses=(0, 0),
            )

        warmup_only = {
            **archive_stats(), "nodes": {"pve1": {"responses": 10, "steady_responses": 0}},
        }
        code, result, text = compare(dict(baseline, pve_archive_inventory=warmup_only))
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "measured")
        self.assertEqual(result["rss_growth_kib"], -32768)
        self.assertEqual(result["candidate_proxmox"]["pve_archive_inventory"], warmup_only)
        self.assertIn("Candidate: PVE archive inventory had no completed full response in the RSS window on pve1.", text)
        self.assertIn("This RSS change does not establish memory use under equivalent polling.", text)
        self.assertIn("| PVE archive entries per response | 840 backups, 24 ISOs, 24 templates (144000 bytes) |", text)
        self.assertIn("| PVE archive responses by node (all / steady) | pve1 10/6 | pve1 10/0 |", text)
        self.assertIn("not proof that Pulse accepted or retained those archive rows", text)

        for unavailable in (None, archive_stats(None), archive_stats(299)):
            with self.subTest(archive=unavailable):
                _, _, text = compare(dict(baseline, pve_archive_inventory=unavailable))
                self.assertIn("Candidate: PVE archive response evidence is unavailable for the RSS window.", text)

        _, _, text = compare(dict(baseline, pve_archive_inventory={**archive_stats(), "body_sha256": "d" * 64}))
        self.assertIn("The two PVE archive fixtures were configured with different response bodies.", text)
        _, _, text = compare(baseline)
        self.assertNotIn("Polling evidence limits the comparison", text)

    def test_pbs_warmup_coverage_and_steady_status_do_not_hide_missing_snapshot_listings(self):
        whole_run = {"pbs1": {"store-fast": {"served_groups": 2, "expected_groups": 2},
                              "store-archive": {"served_groups": 1, "expected_groups": 1}}}
        baseline = {
            "pve_nodes": 1, "guests": 2, "pbs_instances": 1, "total_pbs_snapshots": 150,
            "snapshots_per_group": 50, "pve_requests": 90, "pbs_requests": 50,
            "dashboard": {"connects": 1, "messages": 60, "bytes": 1024},
            "pve_steady_poll_sightings": {
                "guests_with_sightings": 2, "guests_over_limit": [], "limit_s": 60,
                "longest_gap_ms": 10000, "window_s": 300,
            },
            "pve_archive_inventory": archive_stats(),
            "pbs_snapshot_group_coverage": whole_run,
            "pbs_steady_snapshot_group_coverage": {"window_s": 300, "datastores": whole_run},
            "pbs_steady_datastore_status": {
                "window_s": 300, "datastores": {"pbs1": {
                    "store-fast": {"responses": 10, "longest_gap_ms": 30000},
                    "store-archive": {"responses": 10, "longest_gap_ms": 30000},
                }},
            },
        }

        def compare(snapshot_window):
            candidate = dict(baseline, pbs_steady_snapshot_group_coverage=snapshot_window)
            return self.run_summary(
                samples(131072, 25, 0), samples(98304, 25, 0), estate="proxmox",
                proxmox={"baseline": baseline, "candidate": candidate},
                env={"PULSE_MEMORY_GATE": "0"}, wait_statuses=(0, 0),
            )

        steady = {"window_s": 300, "datastores": {"pbs1": {
            **whole_run["pbs1"], "store-archive": {"served_groups": 0, "expected_groups": 1},
        }}}
        code, result, text = compare(steady)
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "measured")
        self.assertEqual(result["rss_growth_kib"], -32768)
        self.assertEqual(result["candidate_proxmox"]["pbs_steady_snapshot_group_coverage"], steady)
        self.assertIn("| pbs1/store-archive | 1/1 | 0/1 |", text)
        self.assertIn("| PBS snapshot groups served/configured (whole run) | 3/3 | 3/3 |", text)
        self.assertIn("| PBS datastore status responses during steady RSS sampling |"
                      " 2/2 with a response; longest gap 30 s in 300 s |"
                      " 2/2 with a response; longest gap 30 s in 300 s |", text)
        self.assertIn("| pbs1/store-fast | 2/2 | 2/2 |", text)
        self.assertIn("PBS snapshot-listing evidence differs", text)
        self.assertIn("not a backup failure", text)

        partial = {"window_s": 300, "datastores": {"pbs1": {"store-fast": whole_run["pbs1"]["store-fast"]}}}
        for unavailable in (None, {**steady, "window_s": 299}, partial):
            with self.subTest(unavailable=unavailable):
                _, _, text = compare(unavailable)
                self.assertIn("| pbs1/store-archive | 1/1 | n/a |", text)
                self.assertIn("PBS snapshot-listing evidence differs", text)
        _, _, text = compare(baseline["pbs_steady_snapshot_group_coverage"])
        self.assertNotIn("PBS snapshot-listing evidence differs", text)

    def test_proxmox_unavailable_traffic_does_not_erase_adverse_rss(self):
        code, result, text = self.run_summary(
            samples(131072, 25, 0), samples(174080, 25, 0), estate="proxmox",
            env={"PULSE_MEMORY_GATE": "0"}, wait_statuses=(0, 0),
        )
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "measured")
        self.assertTrue(result["exceeded"])
        self.assertFalse(result["regressed"])
        self.assertIn("resident memory changed +42.0 MiB (+32.8%), past both 15% and 16 MiB", text)
        self.assertIn("Baseline: PVE/PBS polling evidence is unavailable.", text)
        self.assertIn("Candidate: PVE/PBS polling evidence is unavailable.", text)
        self.assertIn("Reported only: this estate does not gate.", text)

    def test_mock_estate_is_the_default(self):
        _, result, text = self.run_summary(samples(130000, 25, 0), samples(130000, 25, 0))
        self.assertEqual(result["estate"], "mock")
        self.assertIn("(mock estate)", text)
        self.assertNotIn("Agent traffic", text)

    def test_refuses_rss_verdict_without_a_covered_window_on_both_builds(self):
        full = samples(131072, 25, 0)
        cases = (
            ("candidate", lambda t: t <= 30, "only 0 RSS samples"),
            ("candidate", lambda t: t >= 870, "no RSS sample across 600–870 s"),
            ("candidate", lambda t: not 660 <= t <= 750, "no RSS sample across 645–765 s"),
            ("baseline", lambda t: t <= 855, "no RSS sample across 855–900 s"),
        )
        for label, keep, diagnostic in cases:
            with self.subTest(label=label, diagnostic=diagnostic):
                partial = select_samples(full, keep)
                code, result, text = self.run_summary(
                    partial if label == "baseline" else full,
                    partial if label == "candidate" else full,
                    env={"PULSE_MEMORY_GATE": "0"},
                )
                self.assertEqual(code, 1)
                self.assertEqual(result["status"], "incomplete")
                self.assertEqual(result["sampling"]["window_start_s"], 600)
                self.assertIn(diagnostic, result["sampling"][label]["error"])
                self.assertNotIn("regressed", result)
                self.assertNotIn("rss_growth_kib", result)
                self.assertIn("**Incomplete measurement:**", text)
                self.assertIn(f"{label.capitalize()}: {diagnostic}", text)
                self.assertNotIn("No steady-state RSS regression", text)

    def test_uses_one_planned_window_and_allows_one_missed_nominal_sample(self):
        baseline = samples(131072, 25, 0)
        candidate = select_samples(baseline, lambda t: t not in (600, 750, 900))
        code, result, text = self.run_summary(baseline, candidate, wait_statuses=(0, 0))
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "measured")
        self.assertEqual(result["sampling"], {
            "duration_s": 900, "interval_s": 15, "window_start_s": 600, "max_gap_s": 30,
            "baseline": {"first_s": 600, "last_s": 900, "count": 21},
            "candidate": {"first_s": 615, "last_s": 885, "count": 18},
        })
        self.assertEqual(result["candidate_activity"]["cpu_pct"], 25.0)
        self.assertIn("planned 600–900 s window", text)
        self.assertNotIn("failed_runs", result)

    def test_complete_sample_windows_do_not_clear_a_failed_load_driver(self):
        full = samples(131072, 25, 0)
        code, result, text = self.run_summary(
            full, full, estate="proxmox", wait_statuses=(0, 1), env={"PULSE_MEMORY_GATE": "0"},
            child_wait_statuses={"candidate": {"pulse_wait_status_before_stop": None, "driver_wait_status": 42}},
        )
        self.assertEqual(code, 1)
        self.assertEqual(result["status"], "incomplete")
        self.assertEqual(result["sampling"]["candidate"], {"first_s": 600, "last_s": 900, "count": 21})
        self.assertEqual(result["failed_runs"], {"candidate": {"worker_wait_status": 1, "driver_wait_status": 42}})
        self.assertNotIn("regressed", result)
        self.assertIn("Candidate run did not complete successfully (worker wait status 1; load driver wait status 42)", text)
        self.assertNotIn("No steady-state RSS regression", text)

    def test_worker_failure_without_child_observation_remains_incomplete(self):
        full = samples(131072, 25, 0)
        code, result, text = self.run_summary(
            full, full, wait_statuses=(143, 1),
            child_wait_statuses={"candidate": {"pulse_wait_status_before_stop": True, "driver_wait_status": 256}},
        )
        self.assertEqual(code, 1)
        self.assertEqual(result["failed_runs"], {
            "baseline": {"worker_wait_status": 143}, "candidate": {"worker_wait_status": 1},
        })
        self.assertIn("worker wait status 143; no child wait status observed", text)

    def test_short_run_uses_its_declared_duration(self):
        short = samples(131072, 25, 0, end=38, interval=2)
        code, result, _ = self.run_summary(short, short, duration=40, interval=2)
        self.assertEqual(code, 0)
        self.assertEqual(result["sampling"]["window_start_s"], 26)
        self.assertEqual(result["sampling"]["candidate"], {"first_s": 26, "last_s": 38, "count": 7})

    def test_duplicate_timestamp_cannot_fill_a_sampling_gap(self):
        baseline = samples(131072, 25, 0)
        head, duplicate, rest = baseline.partition("\n675\t")
        edited = head + duplicate.replace("675", "660") + rest
        code, result, _ = self.run_summary(baseline, edited)
        self.assertEqual(code, 1)
        self.assertEqual(result["status"], "incomplete")
        self.assertIn("timestamps must increase", result["sampling"]["candidate"]["error"])

    def test_missing_mapping_sample_does_not_disable_the_rss_gate(self):
        baseline = samples(130000, 25, 0).replace("\t50000\t", "\t0\t")
        candidate = samples(170000, 25, 0, rss_hwm=lambda _: 180000).replace(
            "600\t170000\t50000\t10000\t20000\t30000", "600\t170000\t\t\t\t", 1
        )
        code, result, text = self.run_summary(baseline, candidate)
        self.assertEqual(code, 1)
        self.assertEqual(result["status"], "measured")
        self.assertTrue(result["regressed"])
        self.assertEqual(result["rss_growth_kib"], 40000)
        self.assertEqual(result["candidate_rss_high_water_kib"], 180000)
        self.assertEqual(result["candidate_activity"]["cpu_pct"], 25.0)
        self.assertEqual(result["baseline"]["go_heap_kib"], 0)
        for key in ("go_heap_kib", "go_other_kib", "anon_other_kib", "file_kib"):
            self.assertIsNone(result["candidate"][key])
        self.assertIn("| Go heap | 0.0 | n/a | n/a |", text)
        self.assertIn("Mapping breakdown is unavailable", text)

    def test_shifted_sample_cannot_establish_a_comparison(self):
        full = samples(130000, 25, 0, rss_hwm=lambda _: 160000)
        shifted = full.replace("600\t130000\t50000\t10000\t20000\t30000", "600\t130000\t", 1)
        code, result, _ = self.run_summary(full, shifted)
        self.assertEqual(code, 1)
        self.assertEqual(result["status"], "incomplete")
        self.assertIn("misaligned", result["sampling"]["candidate"]["error"])
        self.assertNotIn("regressed", result)

    def test_invalid_resident_memory_cannot_establish_a_comparison(self):
        full = samples(130000, 25, 0)
        invalid = full.replace("600\t130000\t", "600\tunknown\t", 1)
        code, result, _ = self.run_summary(full, invalid)
        self.assertEqual(code, 1)
        self.assertEqual(result["status"], "incomplete")
        self.assertIn("invalid resident memory sample", result["sampling"]["candidate"]["error"])
        self.assertNotIn("regressed", result)

    def test_reports_cpu_and_write_rates_over_the_final_third(self):
        # 25 ticks/s at 100 Hz is a quarter of a core; 1 MiB per 64 s is 1350 MiB/day.
        code, result, text = self.run_summary(
            samples(130000, 25, 16384), samples(130000, 50, 32768)
        )
        self.assertEqual(code, 0)
        self.assertAlmostEqual(result["baseline_activity"]["cpu_pct"], 25.0)
        self.assertAlmostEqual(result["candidate_activity"]["cpu_pct"], 50.0)
        self.assertAlmostEqual(result["baseline_activity"]["write_mib_per_day"], 1350.0)
        self.assertAlmostEqual(result["candidate_activity"]["wchar_mib_per_day"], 5400.0)
        self.assertIn("| CPU | % of one core | 25.0 | 50.0 | +25.0 |", text)
        self.assertIn("Reported, not gated.", text)

    def test_cpu_and_write_growth_do_not_fail_the_run(self):
        code, result, _ = self.run_summary(samples(130000, 10, 100), samples(130000, 90, 10**7))
        self.assertEqual(code, 0)
        self.assertFalse(result["regressed"])

    def test_tiny_negative_change_prints_as_zero(self):
        self.assertEqual(summary.format_change(0.62, 0.6), "+0.0")

    def test_missing_io_accounting_reports_not_available(self):
        code, result, text = self.run_summary(
            samples(130000, 25, 0, io=False), samples(130000, 25, 0, io=False)
        )
        self.assertEqual(code, 0)
        self.assertIsNone(result["candidate_activity"]["write_mib_per_day"])
        self.assertIn("| Disk writes | MiB/day | n/a | n/a | n/a |", text)

    def test_process_peak_separate_from_unchanged_sampled_rss_and_gate(self):
        code, result, text = self.run_summary(
            samples(131072, 25, 0, rss_hwm=lambda _: 163840),
            samples(131072, 25, 0, rss_hwm=lambda t: 131072 if t == 0 else 262144),
        )
        self.assertEqual(code, 0)
        self.assertEqual(result["rss_growth_kib"], 0)
        self.assertFalse(result["regressed"])
        self.assertEqual(result["baseline_rss_high_water_kib"], 163840)
        self.assertEqual(result["candidate_rss_high_water_kib"], 262144)
        self.assertIn("No steady-state RSS regression", text)
        self.assertIn("| Process RSS high-water mark | 160.0 | 256.0 | +96.0 |", text)
        self.assertIn("does not measure cgroup usage", text)

    def test_legacy_samples_do_not_substitute_current_rss_for_high_water(self):
        code, result, text = self.run_summary(
            samples(131072, 25, 0), samples(131072, 25, 0, rss_hwm=lambda _: 163840)
        )
        self.assertEqual(code, 0)
        self.assertIsNone(result["baseline_rss_high_water_kib"])
        self.assertIn("| Process RSS high-water mark | n/a | 160.0 | n/a |", text)

    def test_missing_or_invalid_last_high_water_keeps_comparison_unknown(self):
        for invalid in ("", "0", "-1", "unavailable"):
            with self.subTest(invalid=invalid):
                code, result, text = self.run_summary(
                    samples(131072, 25, 0, rss_hwm=lambda t: 163840 if t < 900 else invalid),
                    samples(131072, 25, 0, rss_hwm=lambda _: 163840),
                )
                self.assertEqual(code, 0)
                self.assertIsNone(result["baseline_rss_high_water_kib"])
                self.assertIn("| Process RSS high-water mark | n/a | 160.0 | n/a |", text)

    def test_memory_regression_still_fails(self):
        code, result, _ = self.run_summary(samples(130000, 25, 0), samples(170000, 25, 0))
        self.assertEqual(code, 1)
        self.assertTrue(result["regressed"])


class ReleaseMemoryLauncherTests(unittest.TestCase):
    def test_invalid_mismatched_or_dirty_source_never_creates_build_inputs(self):
        for args, dirty_source, message in (
            (SOURCE_ARGS[:1], "", "Usage:"),
            (("v6.8.0-rc.1", BASELINE_SHA, CANDIDATE_SHA), "", "Invalid stable tag"),
            ((STABLE_TAG, "c" * 40, CANDIDATE_SHA), "", "baseline checkout"),
            ((STABLE_TAG, BASELINE_SHA, "c" * 40), "", "candidate checkout"),
            (SOURCE_ARGS, "stable", "baseline checkout is changed"),
            (SOURCE_ARGS, "main", "candidate checkout is changed"),
        ):
            with self.subTest(args=args, dirty_source=dirty_source), tempfile.TemporaryDirectory() as tmp:
                stable, main, output = [Path(tmp) / name for name in ("stable", "main", "out")]
                stable.mkdir()
                main.mkdir()
                result = subprocess.run(
                    ["bash", "-c", SOURCE_GIT_SHIM + 'exec bash "$@"', "memory-fixture",
                     str(ROOT / "scripts/memory/compare_release_memory.sh"), str(stable), str(main),
                     str(output), *args],
                    env={**os.environ, **source_env(), "PULSE_TEST_DIRTY_SOURCE": dirty_source},
                    text=True, capture_output=True, timeout=5,
                )
                self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
                self.assertIn(message, result.stderr)
                self.assertFalse(output.exists())
                self.assertEqual(list(stable.iterdir()), [])
                self.assertEqual(list(main.iterdir()), [])

    def test_launch_parity_and_incomplete_report_after_early_pulse_exit(self):
        # Exercise the actual Bash launch, not a copy of its env arguments.
        # The fixture records its environment and deliberately exits, so this
        # exercises launcher reporting, not a real server, bind or RSS verdict.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            baseline, candidate, output = [root / name for name in ("stable", "main", "out")]
            tool_bin = root / "bin"
            for directory in (baseline, candidate, tool_bin):
                directory.mkdir()
            fixture = root / "pulse-fixture"
            fixture.write_text(
                '#!/bin/bash\nset -eu\n'
                'printf "%s:%s\\n" "${PULSE_METRICS_BIND_ADDRESS-unset}" '
                '"${PULSE_METRICS_PORT-unset}" > "$PULSE_DATA_DIR/metrics-bind"\n'
                'case "$PULSE_DATA_DIR" in */data-candidate) exit 42 ;; *) exit 0 ;; esac\n'
            )
            fixture.chmod(0o700)
            for name, body in (
                ("go", 'set -eu\n[[ "$1" == build && "$2" == -o ]]\ncp "$PULSE_TEST_BINARY" "$3"\n'),
                ("curl", "exit 1\n"),
            ):
                fake = tool_bin / name
                fake.write_text("#!/bin/bash\n" + body)
                fake.chmod(0o700)
            result = subprocess.run(
                ["bash", str(ROOT / "scripts/memory/compare_release_memory.sh"),
                 str(baseline), str(candidate), str(output)],
                env={**os.environ, "PATH": f"{tool_bin}:{os.environ['PATH']}",
                     "PULSE_TEST_BINARY": str(fixture), "PULSE_FOOTPRINT_ESTATE": "mock",
                     "PULSE_MEMORY_DURATION": "8", "PULSE_MEMORY_INTERVAL": "1"},
                text=True, capture_output=True, timeout=30,
            )
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
            for label in ("baseline", "candidate"):
                with self.subTest(label=label):
                    self.assertEqual((output / f"data-{label}" / "metrics-bind").read_text(),
                                     "127.0.0.1:0\n")
            observation = json.loads((output / "result.json").read_text())
            self.assertEqual(observation["status"], "incomplete")
            self.assertNotIn("sources", observation)  # Legacy local invocation is not source-bound.
            self.assertEqual(observation["failed_runs"], {
                "baseline": {"worker_wait_status": 1, "pulse_wait_status_before_stop": 0},
                "candidate": {"worker_wait_status": 1, "pulse_wait_status_before_stop": 42},
            })
            summary_text = (output / "summary.md").read_text()
            for label, status in (("Baseline", 0), ("Candidate", 42)):
                self.assertIn(f"{label.lower()} exited before the watch stopped it (wait status {status})", result.stderr)
                self.assertIn(f"{label} run did not complete successfully", summary_text)
                self.assertIn(f"Pulse wait status before the watch's stop {status}", summary_text)
            self.assertNotIn("regressed", observation)
            self.assertNotIn("No steady-state RSS regression", summary_text)

    def test_full_window_requires_normal_stop_but_not_optional_mapping_data(self):
        for mode, expected, failure_field in (
            ("exit-during-profile", 1, "pulse_wait_status_before_stop"),
            ("unexpected-stop-status", 1, "pulse_wait_status_after_stop_request"),
            ("normal-term", 0, None),
            ("unreadable-smaps", 0, None),
            ("empty-smaps", 0, None),
        ):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                baseline, candidate, output = [root / name for name in ("stable", "main", "out")]
                for directory in (baseline, candidate):
                    directory.mkdir()
                (candidate / "mode").write_text(mode)
                smaps = root / "fixture-smaps"
                smaps.write_text(
                    "1000-2000 rw-p 00000000 00:00 0                 [anon: Go: heap]\n"
                    "Rss:                  50000 kB\n"
                    "2000-3000 rw-p 00000000 00:00 0                 [anon: Go: runtime]\n"
                    "Rss:                  10000 kB\n"
                    "3000-4000 rw-p 00000000 00:00 0\n"
                    "Rss:                  20000 kB\n"
                    "4000-5000 r-xp 00000000 00:00 1                 /fixture/file\n"
                    "Rss:                  30000 kB\n"
                )
                fixture = root / "pulse-fixture"
                fixture.write_text(
                    '''#!/usr/bin/env python3
import os
from pathlib import Path
import signal

data = Path(os.environ["PULSE_DATA_DIR"])
mode = (data.parent.parent / "main" / "mode").read_text() if data.name == "data-candidate" else "graceful"
signal.alarm(15)  # Do not leave a fixture process behind if the test fails.
signal.signal(signal.SIGUSR1, lambda *_: os._exit(42))
if mode != "normal-term":
    signal.signal(signal.SIGTERM, lambda *_: os._exit(42 if mode == "unexpected-stop-status" else 0))
(data / "ready").touch()
while True:
    signal.pause()
'''
                )
                fixture.chmod(0o700)
                # Control the clock and counters so both sides have a complete
                # window, but exercise the real launch, signals, wait and summary.
                shims = r'''go() {
  [[ "$1" == build && "$2" == -o ]] && cp "$PULSE_TEST_BINARY" "$3"
}
date() {
  # The archive pair fixes one timestamp before either worker starts.
  if [[ -z "${data-}" ]]; then printf '%s\n' 90; return; fi
  local tick=100 i
  for ((i=0; i<100; i++)); do
    [[ -f "$data/ready" ]] && break
    command sleep 0.02
  done
  [[ -f "$data/ready" ]] || return 1
  [[ ! -f "$data/clock" ]] || read -r tick < "$data/clock"
  printf '%s\n' "$((tick + 1))" > "$data/clock"
  printf '%s\n' "$tick"
}
awk() {
  case "${2-}" in
    /proc/*/status) printf '131072\t163840\n' ;;
    /proc/*/smaps)
      if [[ "$label" == candidate ]]; then
        case "$PULSE_TEST_MODE" in
          unreadable-smaps) printf '123\t456'; return 2 ;;
          empty-smaps) command awk "$1" /dev/null; return $? ;;
        esac
      fi
      command awk "$1" "$PULSE_TEST_SMAPS"
      ;;
    /proc/*/io) printf '1024\t2048' ;;
    *) command awk "$@" ;;
  esac
}
curl() {
  printf '%s\n' "$@" > "$PULSE_TEST_OUT/profile-$label"
  local i
  while (( $# > 1 )); do
    if [[ "$1" == -o ]]; then
      printf 'partial' > "$2"
      break
    fi
    shift
  done
  if [[ "$label" == candidate && "$PULSE_TEST_MODE" == exit-during-profile ]]; then
    kill -USR1 "$pid"
    for ((i=0; i<100; i++)); do
      kill -0 "$pid" 2>/dev/null || break
      command sleep 0.02
    done
  fi
  return 28
}
python3() {
  if [[ "$1" == */proxmox_load.py ]]; then
    # The poll decoder is tested separately. Here capture the actual
    # launcher's requested observation window under the controlled clock.
    printf '%s\n' "$@" > "$PULSE_TEST_OUT/proxmox-args-$label"
    return 0
  fi
  command python3 "$@"
}
sleep() { :; }
export -f go date awk curl python3 sleep
exec bash "$@"
'''
                result = subprocess.run(
                    ["bash", "-c", SOURCE_GIT_SHIM + shims, "memory-fixture",
                     str(ROOT / "scripts/memory/compare_release_memory.sh"),
                     str(baseline), str(candidate), str(output), *SOURCE_ARGS],
                    env={**os.environ, **source_env(), "PULSE_TEST_BINARY": str(fixture),
                         "PULSE_TEST_SMAPS": str(smaps),
                         "PULSE_TEST_OUT": str(output), "PULSE_TEST_MODE": mode,
                         "PULSE_FOOTPRINT_ESTATE": "proxmox" if mode == "normal-term" else "mock",
                         "PULSE_MEMORY_GATE": "0", "PULSE_MEMORY_DURATION": "8", "PULSE_MEMORY_INTERVAL": "1"},
                    text=True, capture_output=True, timeout=25,
                )
                self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
                observation = json.loads((output / "result.json").read_text())
                self.assertEqual(observation["sources"], {
                    "baseline": {"tag": STABLE_TAG, "sha": BASELINE_SHA},
                    "candidate": {"sha": CANDIDATE_SHA},
                })
                self.assertIn(f"Baseline: `{STABLE_TAG}` at `{BASELINE_SHA}`. Candidate: `{CANDIDATE_SHA}`.",
                              (output / "summary.md").read_text())
                for side in ("baseline", "candidate"):
                    self.assertEqual(observation["sampling"][side], {"first_s": 5, "last_s": 7, "count": 3})
                if failure_field is not None:
                    self.assertEqual(observation["status"], "incomplete")
                    self.assertEqual(observation["failed_runs"], {
                        "candidate": {"worker_wait_status": 1, failure_field: 42},
                    })
                    self.assertNotIn("regressed", observation)
                    self.assertIn("wait status 42", result.stderr)
                else:
                    self.assertEqual(observation["status"], "measured")
                    self.assertNotIn("failed_runs", observation)
                    if mode.endswith("-smaps"):
                        self.assertEqual(observation["candidate"]["rss_kib"], 131072)
                        self.assertIsNone(observation["candidate"]["go_heap_kib"])
                        self.assertEqual(observation["baseline"]["go_heap_kib"], 50000)
                        self.assertEqual(observation["candidate_rss_high_water_kib"], 163840)
                        self.assertEqual(observation["candidate_activity"]["write_mib_per_day"], 0.0)
                        row = (output / "samples-candidate.tsv").read_text().splitlines()[-1].split("\t")
                        self.assertEqual(row[2:6], [""] * 4)
                        self.assertEqual(row[7:], ["1024", "2048", "163840"])
                for side in ("baseline", "candidate"):
                    if mode == "normal-term":
                        args = (output / f"proxmox-args-{side}").read_text().splitlines()
                        self.assertEqual(args[args.index("--steady-since") + 1], "105")
                        self.assertEqual(args[args.index("--sample-until") + 1], "108")
                        self.assertEqual(args[args.index("--archive-as-of") + 1], "90")
                    args = (output / f"profile-{side}").read_text().splitlines()
                    self.assertIn("--max-time", args)
                    self.assertEqual(args[args.index("--max-time") + 1], "5")
                    self.assertEqual((output / f"heap-{side}.pb.gz.partial").read_text(), "partial")
                    self.assertFalse((output / f"heap-{side}.pb.gz").exists())


if __name__ == "__main__":
    unittest.main()
