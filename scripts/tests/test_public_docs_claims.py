#!/usr/bin/env python3
"""Guard public capability claims and complete, scan-local link validation."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import re
import sys
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "check_public_docs.py"
SPEC = importlib.util.spec_from_file_location("check_public_docs", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
public_docs = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = public_docs
SPEC.loader.exec_module(public_docs)


class PublicDocsClaimsTest(unittest.TestCase):
    def test_release_notes_are_in_the_public_claim_surface(self) -> None:
        files = public_docs.public_markdown_files()

        self.assertIn(ROOT / "docs/releases/RELEASE_NOTES_v6.4.0-rc.7.md", files)

    def test_rejects_reversed_dead_man_direction(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            note = root / "note.md"
            note.write_text(
                "Dead-man checks can notify when an expected external signal "
                "stops arriving.\n",
                encoding="utf-8",
            )
            with mock.patch.object(public_docs, "ROOT", root):
                errors = public_docs.check_public_claims([note])

        self.assertEqual(len(errors), 1)
        self.assertIn("dead-man direction is reversed", errors[0])

    def test_accepts_outbound_watchdog_description(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            note = root / "note.md"
            note.write_text(
                "Pulse sends its own health signal to a watchdog on another host.\n",
                encoding="utf-8",
            )
            with mock.patch.object(public_docs, "ROOT", root):
                errors = public_docs.check_public_claims([note])

        self.assertEqual(errors, [])

    def test_rejects_removed_truenas_rest_probe(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            note = root / "note.md"
            note.write_text(
                "Test with https://nas.local/api/v2.0/system/info.\n",
                encoding="utf-8",
            )
            with mock.patch.object(public_docs, "ROOT", root):
                errors = public_docs.check_public_claims([note])

        self.assertEqual(len(errors), 1)
        self.assertIn("TrueNAS 26 removed", errors[0])

    def test_contributing_references_live_repository_paths_and_commands(self) -> None:
        text = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
        for required in (
            "`cmd/pulse-agent`",
            "`cmd/pulse-agent-helper`",
            "`cmd/pulse-agent-runner`",
            "`internal/monitoring`",
            "`npm --prefix frontend-modern run build`",
            "`make build-agents`",
        ):
            self.assertIn(required, text)
        for stale in (
            "cmd/pulse-*-agent",
            "`internal/monitor`",
            "docs/internal/SCRIPT_LIBRARY.md",
            "make bundle-scripts",
            "MIGRATION_SCAFFOLDING.md",
        ):
            self.assertNotIn(stale, text)
        self.assertEqual(
            (ROOT / "frontend-modern/public/docs/CONTRIBUTING.md").read_text(encoding="utf-8"),
            text,
        )


class PublicDocsLinksTest(unittest.TestCase):
    def setUp(self) -> None:
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.root_patch = mock.patch.object(public_docs, "ROOT", self.root)
        self.root_patch.start()
        self.addCleanup(self.root_patch.stop)

    def document(self, name: str, text: str) -> Path:
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def test_repeated_links_scan_each_target_once_without_hiding_any_errors(self) -> None:
        target = self.document("guide.md", "# Shared\n# Shared\n")
        first = self.document(
            "first.md", "[one](guide.md#shared) [two](guide.md#shared-1) "
            "[bad](guide.md#missing)\n",
        )
        second = self.document(
            "nested/second.md", "[same](../guide.md#shared) "
            "[bad](../guide.md#missing)\n",
        )
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([first, second])
        self.assertEqual(errors, [
            "first.md: missing anchor #missing in guide.md",
            "nested/second.md: missing anchor #missing in guide.md",
        ])
        scan.assert_called_once_with(target)

    def test_empty_anchor_set_is_cached_and_each_link_still_fails(self) -> None:
        target = self.document("empty.md", "There are no headings.\n")
        source = self.document("source.md", "[one](empty.md#one) [two](empty.md#two)\n")
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([source])
        self.assertEqual(errors, [
            "source.md: missing anchor #one in empty.md",
            "source.md: missing anchor #two in empty.md",
        ])
        scan.assert_called_once_with(target)

    def test_distinct_targets_with_the_same_filename_do_not_share_anchors(self) -> None:
        first = self.document("first/guide.md", "# First\n")
        second = self.document("second/guide.md", "# Second\n")
        source = self.document(
            "source.md", "[one](first/guide.md#first) [two](second/guide.md#second) "
            "[bad](second/guide.md#first)\n",
        )
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([source])
        self.assertEqual(errors, ["source.md: missing anchor #first in second/guide.md"])
        self.assertEqual(scan.call_args_list, [mock.call(first), mock.call(second)])

    def test_next_check_reads_changed_headings_instead_of_reusing_an_old_scan(self) -> None:
        target = self.document("guide.md", "# Before\n")
        source = self.document("source.md", "[one](guide.md#before) [two](guide.md#before)\n")
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            self.assertEqual(public_docs.check_links([source]), [])
            target.write_text("# After\n", encoding="utf-8")
            errors = public_docs.check_links([source])
        self.assertEqual(errors, ["source.md: missing anchor #before in guide.md"] * 2)
        self.assertEqual(scan.call_args_list, [mock.call(target), mock.call(target)])

    def test_self_links_decoding_and_heading_suffixes_keep_their_existing_rules(self) -> None:
        source = self.document(
            "source.md", "# Café\n# Café\n[one](#caf%C3%A9) [two](#café-1) [bad](#café-2)\n",
        )
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([source])
        self.assertEqual(errors, ["source.md: missing anchor #café-2 in source.md"])
        scan.assert_called_once_with(source)

    def test_code_heading_keeps_literal_underscores_but_prose_emphasis_does_not(self) -> None:
        self.document("guide.md", "### `invalid_client` after _saving_\n")
        source = self.document(
            "source.md", "[direct](guide.md#invalid_client-after-saving) "
            "[wrong](guide.md#invalidclient-after-saving)\n",
        )
        self.assertEqual(public_docs.check_links([source]), [
            "source.md: missing anchor #invalidclient-after-saving in guide.md",
        ])

    def test_missing_files_still_fail_without_scanning_external_or_nonmarkdown_targets(self) -> None:
        self.document("image.svg", "<svg/>\n")
        source = self.document(
            "source.md", "[missing](missing.md#one) [external](https://example.invalid/#one) "
            "![image](image.svg#one) [self](#)\n",
        )
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([source])
        self.assertEqual(errors, ["source.md: missing local target missing.md#one"])
        scan.assert_not_called()

    def test_code_comments_do_not_create_recovery_destinations_or_shift_suffixes(self) -> None:
        self.document(
            "guide.md", "# Recovery\n```sh\n# Recovery\n# Reset lockout\n```\n# Recovery\n",
        )
        source = self.document(
            "source.md", "[first](guide.md#recovery) [second](guide.md#recovery-1) "
            "[phantom](guide.md#recovery-2) [comment](guide.md#reset-lockout)\n",
        )

        self.assertEqual(public_docs.check_links([source]), [
            "source.md: missing anchor #recovery-2 in guide.md",
            "source.md: missing anchor #reset-lockout in guide.md",
        ])

    def test_fenced_markdown_and_html_examples_are_not_live_links(self) -> None:
        source = self.document(
            "source.md", "# Present\n```markdown\n[example](absent.md#one)\n"
            '<a href="also-absent.md#two">example</a>\n```\n'
            "~~~html\n<img src='absent.png'>\n~~~\n"
            "[real](#present) [broken](#missing)\n",
        )

        self.assertEqual(public_docs.check_links([source]), [
            "source.md: missing anchor #missing in source.md",
        ])

    def test_closing_fence_needs_matching_character_and_enough_markers(self) -> None:
        for opening, short, wrong, closing in [
            ("````sh", "```", "~~~~", "`````"),
            ("~~~~sh", "~~~", "````", "~~~~~"),
        ]:
            with self.subTest(opening=opening):
                guide = self.document(
                    "guide.md", f"{opening}\n# Hidden\n{short}\n# Still hidden\n"
                    f"{wrong}\n# Also hidden\n{closing}\n# Visible\n",
                )
                self.assertEqual(public_docs.anchors_for(guide), {"visible"})

    def test_closing_fence_cannot_carry_an_info_string(self) -> None:
        guide = self.document(
            "guide.md", "```sh\n# Hidden\n```not-a-close\n# Still hidden\n```\n# Visible\n",
        )
        self.assertEqual(public_docs.anchors_for(guide), {"visible"})

    def test_fences_allow_up_to_three_spaces_and_trailing_closing_whitespace(self) -> None:
        for indent in range(4):
            with self.subTest(indent=indent):
                guide = self.document(
                    "guide.md", f"{' ' * indent}~~~sh\n# Hidden\n"
                    f"{' ' * indent}~~~ \t\n# Visible\n",
                )
                self.assertEqual(public_docs.anchors_for(guide), {"visible"})

    def test_unclosed_fences_hide_examples_through_the_end_of_the_file(self) -> None:
        source = self.document(
            "source.md", "# Present\n```sh\n# Not a heading\n[example](absent.md)\n",
        )
        self.assertEqual(public_docs.anchors_for(source), {"present"})
        self.assertEqual(public_docs.check_links([source]), [])

    def test_backticks_in_backtick_info_do_not_open_a_fence(self) -> None:
        guide = self.document("guide.md", "```not`a-fence\n# Visible\n")
        self.assertEqual(public_docs.anchors_for(guide), {"visible"})

    def test_closed_example_does_not_join_prose_into_an_accidental_link(self) -> None:
        source = self.document(
            "source.md", "[example]\n```markdown\nanything\n```\n(absent.md)\n",
        )
        self.assertEqual(public_docs.check_links([source]), [])




class PublicDocsProsePunctuationTest(unittest.TestCase):
    def test_rejects_em_dash_and_semicolon_in_prose_only(self) -> None:
        text = (
            "Clean line with a period.\n"
            "Line with an em dash \u2014 should fail.\n"
            "Line with a semicolon; should fail.\n"
            "Inline code `foo; bar \u2014 baz` and HTML &amp; entity and "
            "[link](https://example.invalid/a;b) are allowed.\n"
            "```sh\n"
            "echo one; echo two # \u2014 inside code fence\n"
            "```\n"
        )
        errors = public_docs.check_prose_punctuation_in_text("README.md", text)
        self.assertEqual(
            errors,
            [
                "README.md:2: avoid em dashes in public docs prose",
                "README.md:3: avoid semicolons in public docs prose",
            ],
        )

    def test_diff_mode_checks_only_added_lines_and_ignores_existing_history(self) -> None:
        import os
        import subprocess

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            env = dict(
                os.environ,
                GIT_AUTHOR_NAME="Fixture",
                GIT_AUTHOR_EMAIL="fixture@example.invalid",
                GIT_COMMITTER_NAME="Fixture",
                GIT_COMMITTER_EMAIL="fixture@example.invalid",
            )

            def git(*args: str) -> str:
                return subprocess.run(
                    ["git", *args],
                    cwd=root,
                    env=env,
                    text=True,
                    check=True,
                    capture_output=True,
                    timeout=5,
                ).stdout.strip()

            git("init", "-q")
            readme = root / "README.md"
            readme.write_text("Legacy line; untouched.\n", encoding="utf-8")
            git("add", "README.md")
            git("commit", "-qm", "initial")
            base_sha = git("rev-parse", "HEAD")

            readme.write_text(
                "Legacy line; untouched.\nAdded line with an em dash \u2014 here.\n",
                encoding="utf-8",
            )
            git("add", "README.md")

            with mock.patch.object(public_docs, "ROOT", root):
                staged_errors = public_docs.check_changed_prose_punctuation([readme], staged=True)
            self.assertEqual(
                staged_errors,
                ["README.md:2: avoid em dashes in public docs prose"],
            )

            git("commit", "-qm", "add doc line")
            with mock.patch.object(public_docs, "ROOT", root):
                pr_errors = public_docs.check_changed_prose_punctuation(
                    [readme], base_ref=base_sha
                )
            self.assertEqual(
                pr_errors,
                ["README.md:2: avoid em dashes in public docs prose"],
            )


class AIAutonomyHelpDocsTest(unittest.TestCase):
    @staticmethod
    def _extract_prometheus_metrics(source_path: Path) -> set[str]:
        text = source_path.read_text(encoding="utf-8")
        metrics: set[str] = set()
        for match in re.finditer(r"Opts\{([^}]*)\}", text, re.S):
            block = match.group(1)
            namespace = re.search(r'Namespace:\s*"([^"]+)"', block)
            subsystem = re.search(r'Subsystem:\s*"([^"]+)"', block)
            name = re.search(r'Name:\s*"([^"]+)"', block)
            if not name:
                continue
            parts = [part.group(1) for part in (namespace, subsystem, name) if part]
            full_name = "_".join(parts)
            if full_name.startswith("pulse_"):
                metrics.add(full_name)
        return metrics

    def test_prometheus_section_matches_exported_metrics_and_endpoints(self) -> None:
        doc = (ROOT / "docs/AI_AUTONOMY.md").read_text(encoding="utf-8")
        mirror = (ROOT / "frontend-modern/public/docs/AI_AUTONOMY.md").read_text(
            encoding="utf-8"
        )
        self.assertEqual(doc, mirror)
        self.assertNotIn("Prometheus counters (prefix `pulse_patrol_*`) track:", doc)
        self.assertNotIn(
            "Pulse does not export a `pulse_patrol_*` Prometheus metric family.",
            doc,
        )
        patrol_metrics = self._extract_prometheus_metrics(
            ROOT / "internal/ai/patrol_metrics.go"
        )
        self.assertGreaterEqual(len(patrol_metrics), 18)
        ai_metrics = self._extract_prometheus_metrics(
            ROOT / "internal/ai/chat/metrics.go"
        )
        self.assertEqual(len(ai_metrics), 4)
        for metric in sorted(
            patrol_metrics
            | ai_metrics
            | {
                "pulse_operational_trust_action_offers_total",
                "pulse_operational_trust_action_verification_total",
            }
        ):
            with self.subTest(metric=metric):
                self.assertIn(f"`{metric}`", doc)
        for endpoint in (
            "`GET /api/ai/patrol/status`",
            "`GET /api/ai/patrol/runs`",
            "`GET /api/ai/patrol/findings`",
            "`GET /api/ai/circuit/status`",
        ):
            self.assertIn(endpoint, doc)

    def test_prometheus_metrics_reference_covers_live_server_and_agent_families(
        self,
    ) -> None:
        prom_doc = (ROOT / "docs/monitoring/PROMETHEUS_METRICS.md").read_text(
            encoding="utf-8"
        )
        source_files = (
            ROOT / "internal/api/http_metrics.go",
            ROOT / "pkg/db/slowlog.go",
            ROOT / "internal/discovery/service.go",
            ROOT / "internal/api/access_metrics_handlers.go",
            ROOT / "internal/ai/chat/metrics.go",
            ROOT / "internal/ai/patrol_metrics.go",
            ROOT / "cmd/pulse-agent/main.go",
            ROOT / "internal/agenttarget/metrics.go",
            ROOT / "internal/hostagent/commands.go",
        )
        for source_file in source_files:
            exported = self._extract_prometheus_metrics(source_file)
            self.assertTrue(exported, f"no metrics extracted from {source_file}")
            for metric in sorted(exported):
                with self.subTest(source=source_file.name, metric=metric):
                    self.assertIn(f"`{metric}`", prom_doc)
        self.assertIn("`127.0.0.1:9191/metrics`", prom_doc)
        self.assertIn("`PULSE_HEALTH_ADDR=off`", prom_doc)

    def test_inline_enable_remote_commands_dialog_docs_match_ui(self) -> None:
        dialog_src = (
            ROOT
            / "frontend-modern/src/components/shared/EnableRemoteCommandsDialog.tsx"
        ).read_text(encoding="utf-8")
        model_src = (
            ROOT
            / "frontend-modern/src/components/shared/enableRemoteCommandsModel.ts"
        ).read_text(encoding="utf-8")
        for ui_marker in (
            "Enable remote commands on",
            "Generate command for",
            "View command",
            "Copy command",
            "Self-signed HTTPS certificate?",
            "Open Agent Doctor",
        ):
            self.assertIn(ui_marker, dialog_src)
        self.assertIn("typed-helper-summary", model_src)
        self.assertIn("Continue", model_src)

        for rel in ("docs/AGENT_SECURITY.md", "docs/UNIFIED_AGENT.md"):
            doc = (ROOT / rel).read_text(encoding="utf-8")
            mirror = (ROOT / "frontend-modern/public" / rel).read_text(encoding="utf-8")
            self.assertEqual(doc, mirror)
            norm = " ".join(doc.split())
            for phrase in (
                "Enable remote commands",
                "Generate command",
                "View command",
                "Self-signed HTTPS certificate?",
                "Open Agent Doctor",
            ):
                with self.subTest(doc=rel, phrase=phrase):
                    self.assertIn(phrase, norm)
        security_norm = " ".join(
            (ROOT / "docs/AGENT_SECURITY.md").read_text(encoding="utf-8").split()
        )
        self.assertIn("typed-helper-summary", security_norm)
        self.assertIn(
            "Read-only and non-admin sessions stay passive with no token generation triggers.",
            security_norm,
        )

    def test_relay_entitlements_and_investigation_runtime_limits_match_code(self) -> None:
        normalized = " ".join(
            (ROOT / "docs/AI_AUTONOMY.md").read_text(encoding="utf-8").split()
        )
        self.assertNotIn("Community and Relay baseline", normalized)
        self.assertNotIn("Community and Relay can run Patrol with BYOK", normalized)
        self.assertIn(
            "Continuing Relay subscribers receive Pro entitlements",
            normalized,
        )
        self.assertIn("retire on 31 March 2027", normalized)
        self.assertIn(
            "Pro, continuing Relay, legacy Pro+, or Cloud license",
            normalized,
        )
        self.assertIn("`investigation_budget`", normalized)
        self.assertIn("`investigation_timeout_sec`", normalized)
        self.assertIn("up to 64 additional findings can wait in memory", normalized)
        self.assertIn(
            "20 minute cooldown after a failed fix or failed fix verification",
            normalized,
        )
        self.assertIn("10 minute cooldown after a timed-out outcome", normalized)
        self.assertIn("30-day budget (USD)", normalized)

    def test_patrol_plan_approval_ttl_matches_actionplanner_source(self) -> None:
        planner = (ROOT / "internal/actionplanner/planner.go").read_text(encoding="utf-8")
        broker = (ROOT / "internal/api/patrol_action_broker.go").read_text(encoding="utf-8")
        self.assertIn("DefaultPlanTTL       = 5 * time.Minute", planner)
        self.assertIn("DefaultPatrolPlanTTL = 24 * time.Hour", planner)
        self.assertIn("opts.TTL = actionplanner.DefaultPatrolPlanTTL", broker)

        autonomy = (ROOT / "docs/AI_AUTONOMY.md").read_text(encoding="utf-8")
        autonomy_mirror = (ROOT / "frontend-modern/public/docs/AI_AUTONOMY.md").read_text(
            encoding="utf-8"
        )
        self.assertEqual(autonomy, autonomy_mirror)
        autonomy_norm = " ".join(autonomy.split())
        for phrase in (
            "background Patrol plans stay open for approval for 24 hours",
            "Background Patrol action plans, including refreshed Patrol plans, stay open for approval for 24 hours before expiring, matching investigation retention",
            "Direct API and interactive action plans keep the 5-minute default expiry",
            "Live freshness, policy, and emergency-stop checks still run when an approved plan executes",
        ):
            with self.subTest(autonomy_phrase=phrase):
                self.assertIn(phrase, autonomy_norm)

        api_doc = (ROOT / "docs/API.md").read_text(encoding="utf-8")
        api_mirror = (ROOT / "frontend-modern/public/docs/API.md").read_text(encoding="utf-8")
        self.assertEqual(api_doc, api_mirror)
        api_norm = " ".join(api_doc.split())
        self.assertIn(
            "Direct API and interactive plans expire after 5 minutes by default, while background and refreshed Patrol plans stay open for approval for 24 hours.",
            api_norm,
        )

    def test_agent_security_failure_log_disclosure_reflects_retired_web_assistant(self) -> None:
        doc = (ROOT / "docs/AGENT_SECURITY.md").read_text(encoding="utf-8")
        mirror = (ROOT / "frontend-modern/public/docs/AGENT_SECURITY.md").read_text(
            encoding="utf-8"
        )
        self.assertEqual(doc, mirror)
        normalized = " ".join(doc.split())
        self.assertNotIn("when Patrol or the Assistant reads the container", normalized)
        self.assertIn(
            "when Patrol reads the container (or legacy Pulse Mobile chat until 31 March 2027)",
            normalized,
        )

    def test_architecture_reflects_unsupported_claude_oauth_and_self_serve_msp(self) -> None:
        arch = (ROOT / "ARCHITECTURE.md").read_text(encoding="utf-8")
        mirror = (ROOT / "frontend-modern/public/docs/ARCHITECTURE.md").read_text(encoding="utf-8")
        self.assertEqual(arch, mirror)
        normalized = " ".join(arch.split())
        self.assertNotIn("plus Claude OAuth", normalized)
        self.assertIn("Anthropic subscription OAuth is unsupported and fails closed.", normalized)
        self.assertIn(
            "provider-hosted **MSP** (self-serve 60-day evaluation and paid plans)",
            normalized,
        )

    def test_temperature_security_and_rollout_docs_match_runtime(self) -> None:
        mtm_src = (ROOT / "internal/monitoring/multi_tenant_monitor.go").read_text(
            encoding="utf-8"
        )
        self.assertIn("monitor.tempCollector = nil", mtm_src)

        temp_sec = (ROOT / "docs/security/TEMPERATURE_MONITORING.md").read_text(encoding="utf-8")
        temp_norm = " ".join(temp_sec.split())
        self.assertIn('from="<pulse-ip>",command="/usr/local/sbin/pulse-sensors"', temp_sec)
        self.assertNotIn("If you must for dev/test, set `PULSE_DEV_ALLOW_CONTAINER_SSH=true`", temp_norm)
        self.assertIn(
            "an already mounted key can still be used by the legacy collector with a security warning",
            temp_norm,
        )
        self.assertIn(
            "SSH temperature collection runs only in the **default** organization and is disabled on non-default tenants",
            temp_norm,
        )

        sched_src = (ROOT / "internal/monitoring/monitor_polling_schedule.go").read_text(
            encoding="utf-8"
        )
        self.assertIn("m.deadLetterQueue.Remove(task.InstanceType, task.InstanceName)", sched_src)

        rollout = (ROOT / "docs/operations/ADAPTIVE_POLLING_ROLLOUT.md").read_text(
            encoding="utf-8"
        )
        rollout_norm = " ".join(rollout.split())
        self.assertNotIn("A [dead-letter entry can remain after a later poll succeeds]", rollout_norm)
        self.assertIn(
            "On current builds a later scheduled success clears the [dead-letter entry]",
            rollout_norm,
        )
        self.assertIn("while older builds can still retain it after recovery", rollout_norm)

    def test_security_and_api_docs_match_csrf_lockout_body_and_ai_origin_guards(self) -> None:
        sec_go = (ROOT / "internal/api/security.go").read_text(encoding="utf-8")
        body_go = (ROOT / "internal/api/request_body_authority.go").read_text(encoding="utf-8")
        setup_go = (ROOT / "internal/api/security_setup_fix.go").read_text(encoding="utf-8")
        transfer_go = (ROOT / "internal/api/config_transfer_authorization.go").read_text(
            encoding="utf-8"
        )
        ai_go = (ROOT / "internal/api/ai_handlers.go").read_text(encoding="utf-8")

        self.assertIn('csrfToken := r.Header.Get("X-CSRF-Token")', sec_go)
        self.assertIn("if !ValidateSession(sessionID)", sec_go)
        self.assertIn("credential withdrawn before the request body arrived", body_go)
        self.assertIn("invalidateAdminCredentialSessions(previousAuthUser, setupRequest.Username)", setup_go)
        self.assertIn("RecordLoginAttempt(credentialsValid, username, clientIP)", transfer_go)
        self.assertIn("aiProviderWithSecretFollowingAddress(&originalSettings, settings, req)", ai_go)

        sec_doc = (ROOT / "SECURITY.md").read_text(encoding="utf-8")
        sec_mirror = (ROOT / "frontend-modern/public/docs/SECURITY.md").read_text(encoding="utf-8")
        self.assertEqual(sec_doc, sec_mirror)
        sec_norm = " ".join(sec_doc.split())
        for phrase in (
            "require the CSRF token in the `X-CSRF-Token` header (query parameters and form fields are ignored)",
            "replacement `pulse_csrf` cookies are issued only for live sessions",
            "across browser login, HTTP Basic authentication, password changes, and Basic-authenticated configuration transfers",
            "Session invalidation on password change and quick-setup credential rotation",
            "Admitted local sessions and API tokens are rechecked on every request body read",
        ):
            with self.subTest(doc="SECURITY.md", phrase=phrase):
                self.assertIn(phrase, sec_norm)

        api_doc = (ROOT / "docs/API.md").read_text(encoding="utf-8")
        api_mirror = (ROOT / "frontend-modern/public/docs/API.md").read_text(encoding="utf-8")
        self.assertEqual(api_doc, api_mirror)
        api_norm = " ".join(api_doc.split())
        for phrase in (
            "require the CSRF token in the `X-CSRF-Token` header (query parameters and form fields are ignored)",
            "replacement `pulse_csrf` cookies are issued only for live sessions",
            "Admitted local sessions and API tokens are also rechecked while reading request bodies",
            "Rotating administrator credentials invalidates existing administrator sessions immediately.",
            "Changing an OpenAI, Z.ai, or Ollama base URL to a different origin while retaining an existing stored API key or password returns `400`",
            "[Weekly summary and email](AI.md#weekly-summary-and-email)",
        ):
            with self.subTest(doc="docs/API.md", phrase=phrase):
                self.assertIn(phrase, api_norm)
        self.assertNotIn("`docs/PATROL_WEEKLY_DIGEST.md`", api_norm)


if __name__ == "__main__":
    unittest.main()
