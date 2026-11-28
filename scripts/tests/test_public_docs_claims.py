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
            "`docs/PATROL_WEEKLY_DIGEST.md`",
        ):
            with self.subTest(doc="docs/API.md", phrase=phrase):
                self.assertIn(phrase, api_norm)

    def test_assistant_safety_help_matches_retired_prompts_and_fail_closed_classification(
        self,
    ) -> None:
        tool_call_go = (ROOT / "internal/agentcapabilities/tool_call.go").read_text(
            encoding="utf-8"
        )
        manifest_go = (ROOT / "internal/agentcapabilities/manifest.go").read_text(
            encoding="utf-8"
        )
        errors_go = (ROOT / "internal/agentcapabilities/errors.go").read_text(
            encoding="utf-8"
        )
        ai_handlers_go = (ROOT / "internal/api/ai_handlers.go").read_text(
            encoding="utf-8"
        )

        self.assertIn("return ToolCallKindWrite", tool_call_go)
        self.assertIn("HostCommandAuthorityRequiredMessage", tool_call_go)
        self.assertIn("pulse_question since the web chat window was retired", manifest_go)
        self.assertIn('AgentErrCodeRawCommandRetired          = "raw_command_retired"', errors_go)
        self.assertIn(
            "http.StatusGone, agentcapabilities.AgentErrCodeRawCommandRetired",
            ai_handlers_go,
        )

        doc = (ROOT / "docs/ASSISTANT_SAFETY.md").read_text(encoding="utf-8")
        mirror = (ROOT / "frontend-modern/public/docs/ASSISTANT_SAFETY.md").read_text(
            encoding="utf-8"
        )
        self.assertEqual(doc, mirror)
        norm = " ".join(doc.split())
        self.assertNotIn("Interactive tools that ask you something", norm)
        self.assertNotIn("The model chooses when to read, ask, plan or conclude", norm)
        for phrase in (
            "Retired interactive prompt classification (`pulse_question` is no longer offered by any profile and is refused if called)",
            "unknown tool names default to `write` so newly introduced tools cannot bypass governed-action checks accidentally",
            "Raw command execution is retired (`raw_command_retired`, HTTP `410`) in favor of advertised typed resource actions",
            "callers whose Pulse role cannot run actions receive a host-command authority refusal rather than host-command access",
            "Interactive `pulse_question` prompts are retired alongside the in-app web chat window.",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, norm)

    def test_pulse_pro_and_api_docs_match_external_probe_entitlement_and_vmware_routes(
        self,
    ) -> None:
        features_go = (ROOT / "pkg/licensing/features.go").read_text(encoding="utf-8")
        catalog_go = (ROOT / "pkg/licensing/self_hosted_feature_catalog.go").read_text(
            encoding="utf-8"
        )
        resources_go = (ROOT / "internal/api/resourceapi/resources.go").read_text(
            encoding="utf-8"
        )
        routes_go = (ROOT / "internal/api/router_routes_registration.go").read_text(
            encoding="utf-8"
        )
        vmware_go = (ROOT / "internal/api/vmware_handlers.go").read_text(encoding="utf-8")
        vmware_client_go = (ROOT / "internal/vmware/client.go").read_text(
            encoding="utf-8"
        )

        self.assertIn('FeatureExternalProbe     = "external_probe"', features_go)
        self.assertIn("FeatureExternalProbe,", features_go)
        self.assertIn("TierCloud:      proFeatures,", features_go)
        self.assertIn('DisplayName:           "External Probes"', catalog_go)
        self.assertIn('case "vmware", "vmware-vsphere":', resources_go)
        self.assertIn('case "truenas":', resources_go)
        self.assertIn('case "availability":', resources_go)
        self.assertIn('r.mux.HandleFunc("/api/vmware/connections"', routes_go)
        self.assertIn('r.mux.HandleFunc("/api/vmware/connections/test"', routes_go)
        self.assertIn('r.mux.HandleFunc("/api/vmware/connections/preview"', routes_go)
        self.assertIn('FeatureVMware              = "PULSE_ENABLE_VMWARE"', vmware_client_go)
        self.assertIn('"vmware_disabled"', vmware_go)
        self.assertIn('"vmware_not_found"', vmware_go)
        self.assertIn('"vmware_invalid_config"', vmware_go)
        self.assertIn('"vmware_connection_failed"', vmware_go)

        pro_doc = (ROOT / "docs/PULSE_PRO.md").read_text(encoding="utf-8")
        pro_mirror = (ROOT / "frontend-modern/public/docs/PULSE_PRO.md").read_text(
            encoding="utf-8"
        )
        self.assertEqual(pro_doc, pro_mirror)
        pro_norm = " ".join(pro_doc.split())
        for phrase in (
            "| `FeatureExternalProbe` | `external_probe` | External Probes | N | N | Y | Y |",
            "Remote availability-check observation locations on connected agents. Local server checks remain included in Community.",
            "| `FeatureUnlimited` | `unlimited` | Hosted Capacity Policy | N | N | N | Y* |",
            "External availability-check probes from connected remote agents (`external_probe`).",
            "`external_probe`, and `multi_tenant`",
        ):
            with self.subTest(doc="PULSE_PRO.md", phrase=phrase):
                self.assertIn(phrase, pro_norm)

        api_doc = (ROOT / "docs/API.md").read_text(encoding="utf-8")
        api_mirror = (ROOT / "frontend-modern/public/docs/API.md").read_text(
            encoding="utf-8"
        )
        self.assertEqual(api_doc, api_mirror)
        api_norm = " ".join(api_doc.split())
        self.assertNotIn("PULSE_FEATURE_VMWARE", api_doc)
        for phrase in (
            "`truenas`, `vmware` or `vmware-vsphere`, `availability`",
            "## VMware vSphere (Early Access)",
            "`PULSE_ENABLE_VMWARE=false`, these routes return `404` (`vmware_disabled`)",
            "`GET /api/vmware/connections` (admin, `settings:read`)",
            "`POST /api/vmware/connections` (admin, `settings:write`)",
            "`PUT /api/vmware/connections/{id}` (admin, `settings:write`)",
            "`DELETE /api/vmware/connections/{id}` (admin, `settings:write`)",
            "`POST /api/vmware/connections/test` (admin, `settings:write`)",
            "`POST /api/vmware/connections/{id}/test` (admin, `settings:write`)",
            "`POST /api/vmware/connections/preview` and `POST /api/vmware/connections/{id}/preview` (admin, `settings:write`)",
            "`source=vmware` (or its `source=vmware-vsphere` alias)",
        ):
            with self.subTest(doc="API.md", phrase=phrase):
                self.assertIn(phrase, api_norm)

    def test_api_docs_match_saml_notifications_and_alert_durability_runtime(self) -> None:
        auth_routes = (
            ROOT / "internal/api/router_routes_auth_security.go"
        ).read_text(encoding="utf-8")
        saml_go = (ROOT / "internal/api/saml_handlers.go").read_text(encoding="utf-8")
        mon_routes = (
            ROOT / "internal/api/router_routes_monitoring.go"
        ).read_text(encoding="utf-8")
        queue_go = (
            ROOT / "internal/api/alerting/notification_queue.go"
        ).read_text(encoding="utf-8")
        notif_go = (ROOT / "internal/api/alerting/notifications.go").read_text(
            encoding="utf-8"
        )
        alerts_go = (ROOT / "internal/api/alerting/alerts.go").read_text(
            encoding="utf-8"
        )
        durability_go = (
            ROOT / "internal/api/alerting/history_durability.go"
        ).read_text(encoding="utf-8")

        for marker in (
            '"provider_managed_by_environment"',
            'r.mux.HandleFunc("/api/saml/"',
            'case "login":',
            'case "acs":',
            'case "metadata":',
            'case "logout":',
            'case "slo":',
        ):
            self.assertIn(marker, auth_routes)
        for marker in (
            '"authorizationUrl": redirectURL',
            'if req.Method != http.MethodPost',
            'if req.Method != http.MethodGet',
            'if !requireRequestMethod(w, req, http.MethodPost)',
            'IdP-initiated logout not supported',
        ):
            self.assertIn(marker, saml_go)
        for marker in (
            'r.mux.HandleFunc("/api/notifications/dlq", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead',
            'r.mux.HandleFunc("/api/notifications/dlq/retry", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite',
        ):
            self.assertIn(marker, mon_routes)
        self.assertIn("apihttp.EnsureScope(w, r, config.ScopeMonitoringRead)", queue_go)
        self.assertIn("apihttp.EnsureScope(w, r, config.ScopeSettingsWrite)", queue_go)
        self.assertIn('case path == "/rehearsals" || strings.HasPrefix(path, "/rehearsals/"):', notif_go)
        self.assertIn('strings.HasSuffix(path, "/recover")', notif_go)
        for marker in (
            'case path == "intent-policies" && r.Method == http.MethodGet:',
            'case path == "intent-policies" && r.Method == http.MethodPut:',
            'case path == "intent-policies/preview" && r.Method == http.MethodPost:',
            'case path == "history/durability" && r.Method == http.MethodGet:',
            'case path == "history/durability/acknowledgement" && r.Method == http.MethodPost:',
            "writeHistoryCompleteness(w, manager)",
        ):
            self.assertIn(marker, alerts_go)
        self.assertIn('"X-Pulse-History-Complete"', durability_go)
        self.assertIn('"X-Pulse-History-Gap-Revision"', durability_go)

        api_doc = (ROOT / "docs/API.md").read_text(encoding="utf-8")
        api_mirror = (ROOT / "frontend-modern/public/docs/API.md").read_text(
            encoding="utf-8"
        )
        self.assertEqual(api_doc, api_mirror)
        api_norm = " ".join(api_doc.split())
        for phrase in (
            "`GET /api/notifications/dlq` (admin, `settings:read` and `monitoring:read`)",
            "`POST /api/notifications/dlq/retry` (admin, `settings:write`)",
            "### Delivery Rehearsals",
            "`GET /api/notifications/rehearsals` (org manager or admin, `settings:read` or `settings:write`)",
            "`POST /api/notifications/rehearsals` (org manager or admin, `settings:write`)",
            "`GET /api/notifications/rehearsals/{id}` (org manager or admin, `settings:read` or `settings:write`)",
            "`POST /api/notifications/rehearsals/{id}/recover` (org manager or admin, `settings:write`)",
            "`GET /api/alerts/intent-policies` (`monitoring:read`)",
            "`PUT /api/alerts/intent-policies` (`monitoring:write`)",
            "`POST /api/alerts/intent-policies/preview` (`monitoring:read`)",
            "`X-Pulse-History-Complete` and `X-Pulse-History-Gap-Revision`",
            "`GET /api/alerts/history/durability` (`monitoring:read`)",
            "`POST /api/alerts/history/durability/acknowledgement` (`monitoring:write`)",
            "`GET /api/saml/{providerID}/metadata`",
            "`GET` or `POST /api/saml/{providerID}/login`",
            "`POST /api/saml/{providerID}/acs`",
            "`POST /api/saml/{providerID}/logout`",
            "`GET` or `POST /api/saml/{providerID}/slo`",
            "returns `409` (`provider_managed_by_environment`)",
        ):
            with self.subTest(doc="API.md", phrase=phrase):
                self.assertIn(phrase, api_norm)

    def test_storage_pool_staleness_help_matches_frontend_presentation(self) -> None:
        pool_row = (
            ROOT / "frontend-modern/src/components/Storage/StoragePoolRow.tsx"
        ).read_text(encoding="utf-8")
        row_pres = (
            ROOT / "frontend-modern/src/features/storageBackups/rowPresentation.ts"
        ).read_text(encoding="utf-8")
        model_core = (
            ROOT / "frontend-modern/src/features/storageBackups/storageModelCore.ts"
        ).read_text(encoding="utf-8")
        drawer_pres = (
            ROOT
            / "frontend-modern/src/features/storageBackups/storagePoolDrawerPresentation.ts"
        ).read_text(encoding="utf-8")
        page_state = (
            ROOT / "frontend-modern/src/components/Storage/storagePageState.ts"
        ).read_text(encoding="utf-8")

        self.assertIn("shouldShowStoragePoolInlineStaleCue", pool_row)
        self.assertIn("Stale data", pool_row)
        self.assertIn("return 'Stale';", row_pres)
        self.assertIn("Retained last-known storage values.", row_pres)
        self.assertIn("record.freshness === 'stale' ? 'stale stale data' : ''", model_core)
        self.assertIn("getEffectiveStorageRecordHealth(record)", model_core)
        self.assertIn(
            "const STALE_POOL_MESSAGE = 'Retained last-known storage values, not a current reading.';",
            drawer_pres,
        )
        self.assertIn("{ value: 'attention', label: 'Needs attention' }", page_state)

        for rel in ("docs/FAQ.md", "docs/ZFS_MONITORING.md", "docs/TRUENAS.md"):
            doc = (ROOT / rel).read_text(encoding="utf-8")
            mirror = (ROOT / "frontend-modern/public" / rel).read_text(encoding="utf-8")
            self.assertEqual(doc, mirror)
            norm = " ".join(doc.split())
            for phrase in (
                "Status: Healthy",
                "Status: Needs attention",
                "Status: Warning",
                "Stale data",
                "Worth checking",
                "Retained last-known storage values, not a current reading.",
            ):
                with self.subTest(doc=rel, phrase=phrase):
                    self.assertIn(phrase, norm)

    def test_scheduler_poll_planning_and_recovery_help_matches_monitoring_source(
        self,
    ) -> None:
        progress_go = (
            ROOT / "internal/monitoring/poll_work_progress.go"
        ).read_text(encoding="utf-8")
        workers_go = (
            ROOT / "internal/monitoring/poll_task_workers.go"
        ).read_text(encoding="utf-8")

        self.assertIn('"pulse_scheduler_claimed_poll_overdue_seconds"', progress_go)
        self.assertIn('"pulse_scheduler_poll_planning_age_seconds"', progress_go)
        self.assertIn("while the panic handler is blocked writing to the log sink", workers_go)

        prom_norm = " ".join(
            (ROOT / "docs/monitoring/PROMETHEUS_METRICS.md")
            .read_text(encoding="utf-8")
            .split()
        )
        for phrase in (
            "task preparation, provider calls, rescheduling and recovery logging.",
            "If `pulse_scheduler_queued_poll_overdue_seconds` and both claimed-poll stages stay at zero while expected poll counters stop moving, check `pulse_scheduler_poll_planning_age_seconds` on successive scrapes.",
            "A rising age means an unfinished poll planning pass is still blocked reading provider rosters, building the plan, or queuing tasks before any work reaches the queue.",
        ):
            with self.subTest(doc="PROMETHEUS_METRICS.md", phrase=phrase):
                self.assertIn(phrase, prom_norm)

        sched_norm = " ".join(
            (ROOT / "docs/api/SCHEDULER_HEALTH.md")
            .read_text(encoding="utf-8")
            .split()
        )
        for phrase in (
            "or an unfinished poll planning pass blocked before tasks reach the queue.",
            "`pulse_scheduler_claimed_poll_overdue_seconds` and `pulse_scheduler_poll_planning_age_seconds`",
        ):
            with self.subTest(doc="SCHEDULER_HEALTH.md", phrase=phrase):
                self.assertIn(phrase, sched_norm)

    def test_agentexec_session_policy_and_patrol_authority_help_matches_source(
        self,
    ) -> None:
        token_binding = (
            ROOT / "internal/api/agent_exec_token_binding.go"
        ).read_text(encoding="utf-8")
        agentexec_server = (
            ROOT / "internal/agentexec/server.go"
        ).read_text(encoding="utf-8")
        policy_go = (ROOT / "internal/agentexec/policy.go").read_text(encoding="utf-8")
        policy_args = (ROOT / "internal/agentexec/policy_args.go").read_text(
            encoding="utf-8"
        )
        logread_go = (ROOT / "internal/hostagent/commands_logread.go").read_text(
            encoding="utf-8"
        )
        action_auth = (ROOT / "internal/api/action_authority.go").read_text(
            encoding="utf-8"
        )
        ai_handlers = (ROOT / "internal/api/ai_handlers.go").read_text(
            encoding="utf-8"
        )

        self.assertIn("agentExecIdentityHeldByAnotherToken", token_binding)
        self.assertIn("messageWriteWait      = 10 * time.Second", agentexec_server)
        self.assertIn("func (p *CommandPolicy) EvaluateForTarget(", policy_go)
        self.assertIn("var pveshDeniedSegments = []string{", policy_args)
        self.assertIn("func formatTemplatesSelectFields(", policy_args)
        self.assertIn("const maxLogReadFiles = 256", logread_go)
        self.assertIn("func assistantSessionAccessForRequest(", action_auth)
        self.assertIn(
            'Changing the "+config.AIProviderDisplayName(provider)+" base URL to a different server requires entering its credential again',
            ai_handlers,
        )
        self.assertIn(
            'writeErrorResponse(w, http.StatusForbidden, "action_capability_denied", "Writing a dismissal note requires permission to execute actions", nil)',
            ai_handlers,
        )
        self.assertIn("func patrolToolOutputAllowed(", ai_handlers)

        for rel in (
            "docs/AGENT_SECURITY.md",
            "docs/AI.md",
            "docs/ASSISTANT_SAFETY.md",
        ):
            self.assertEqual(
                (ROOT / rel).read_bytes(),
                (ROOT / "frontend-modern/public" / rel).read_bytes(),
            )

        agent_sec = " ".join(
            (ROOT / "docs/AGENT_SECURITY.md").read_text(encoding="utf-8").split()
        )
        for phrase in (
            "First-use install-token bindings, legacy migrations, and ID backfills are refused when another unexpired token in the same organization already holds the live command session, the live host record, or a durable token binding for that agent ID, leaving the refused token unbound.",
            "enforce a **10-second** write deadline on WebSocket text writes so a stalled agent connection is closed instead of pinning action dispatch.",
            "Host log reads (`cat`, `head`, `tail`, and `grep`) under `/var/log` are opened directly by the host agent beneath `/var/log` (symlink-safe, up to `256` files per command) and passed to the program through `/dev/fd` descriptors rather than raw path names.",
            "container and VM log reads require explicit approval rather than auto-approval.",
            "Auto-approved `pvesh get` commands are limited to listed non-credential API subtrees (`version`, `nodes`, `storage`, `pools`, and selected `cluster/*` read paths) and refuse `file-read`, `file-restore`, `extractconfig`, `cloudinit`, `report`, and `subscription` segments.",
            "Auto-approved `docker` and `podman` `--format` templates allow only direct field selections (such as `{{.State.Status}}` or `{{json .Mounts}}`) and refuse functions, loops, pipelines, or credential-named fields that could strip output masking.",
        ):
            with self.subTest(doc="AGENT_SECURITY.md", phrase=phrase):
                self.assertIn(phrase, agent_sec)

        ai_doc = " ".join((ROOT / "docs/AI.md").read_text(encoding="utf-8").split())
        for phrase in (
            "adding free-text dismissal or resolution notes, or saving a finding note, requires action-execution authority (`HTTP 403`, `action_capability_denied`) because those notes enter Patrol's prompt context on later runs.",
            "raw host tool outputs in Patrol run history (`?include=tool_calls`), live streams, and investigation transcripts are withheld unless the caller has action-execution authority.",
            "Changing an OpenAI, Z.ai, or Ollama provider's base URL to a different server origin requires re-entering its saved API key or password so Pulse never replays a write-only credential to another server.",
        ):
            with self.subTest(doc="AI.md", phrase=phrase):
                self.assertIn(phrase, ai_doc)

        safety_doc = " ".join(
            (ROOT / "docs/ASSISTANT_SAFETY.md").read_text(encoding="utf-8").split()
        )
        for phrase in (
            "Assistant sessions (used by legacy Pulse Mobile chat until 31 March 2027) are scoped to the caller who created them (user identity for interactive sign-ins, or the specific API token ID for token callers) unless the caller holds action-execution authority.",
            "Patrol tool outputs in run history, live streams, and investigation transcripts, along with prompt-steering finding, dismissal, and resolution notes, follow the same action-execution authority boundary.",
        ):
            with self.subTest(doc="ASSISTANT_SAFETY.md", phrase=phrase):
                self.assertIn(phrase, safety_doc)


if __name__ == "__main__":
    unittest.main()
