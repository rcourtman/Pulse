#!/usr/bin/env python3
"""Keep the existing contribution entrypoint safe before a public report.

These are guidance checks, not diagnostic-runtime or incident-recovery proof.
"""

from pathlib import Path
import os
import re
import shlex
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


def report_guidance() -> str:
    text = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
    return text.split("## How To Make An Issue Useful\n", 1)[1].split("\n---\n", 1)[0]


class ContributionReportSafetyTest(unittest.TestCase):
    def development_recipe(self):
        guide = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
        section = guide.split("### Hot Reload Dev Loop\n", 1)[1].split("\n---\n", 1)[0]
        recipe = re.search(r"```bash\n(.*?)```", section, re.S).group(1)
        return section, recipe

    def test_root_readme_routes_development_to_the_isolated_workflow(self):
        text = (ROOT / "README.md").read_text(encoding="utf-8")
        section = text.split("## Development\n", 1)[1].split("\n## ", 1)[0]
        self.assertIn("(CONTRIBUTING.md#hot-reload-dev-loop)", section)
        self.assertIn("disposable", section)
        self.assertIn("no production configuration", " ".join(section.split()))
        self.assertIn("`--takeover`", section)
        self.assertIn("stop existing listeners", " ".join(section.split()))
        for block in re.findall(r"```(?:bash|sh)?\n(.*?)```", section, re.S):
            self.assertNotRegex(block, r"npm(?: --prefix frontend-modern)? run dev(?:\s|$)")
            self.assertNotIn("hot-dev-bg.sh start --takeover", block)

    def test_shipped_index_reaches_the_same_isolated_workflow(self):
        for name in ("docs/README.md", "frontend-modern/public/docs/README.md"):
            with self.subTest(surface=name):
                text = (ROOT / name).read_text(encoding="utf-8")
                section = text.split("## Development and reference\n", 1)[1].split("\n## ", 1)[0]
                self.assertIn("(../CONTRIBUTING.md#hot-reload-dev-loop)", section)
                for statement in ("disposable", "production", "authentication bypass",
                                  "existing listeners", "fresh clone alone is not isolation"):
                    self.assertIn(statement, " ".join(section.split()))

    def test_development_recipe_names_the_real_isolation_boundary(self):
        guide = " ".join((ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8").split())
        for statement in (
            "disposable development VM or machine", "no production configuration",
            "fresh clone alone is not isolation", "does not load installed configuration",
            "bypass must never face an untrusted network",
            "do not sandbox local environment files or establish host isolation",
            "they can execute commands and override shell settings",
            "Review any such local files before starting",
            "Neither is a workaround for a startup refusal",
            "choose a separate empty development directory",
            "not required for Community reproduction",
        ):
            self.assertIn(statement, guide)
        section, recipe = self.development_recipe()
        commands = [shlex.split(line) for line in recipe.splitlines() if line.startswith("./")]
        self.assertEqual(commands, [
            ["./scripts/hot-dev-bg.sh", "start"],
            ["./scripts/hot-dev-bg.sh", "status"],
        ])
        self.assertNotIn("--takeover", recipe)
        self.assertNotIn("mock:on", recipe)
        self.assertIn("Do not follow", section)
        self.assertIn("choose unused", section)
        self.assertIn("same clone and shell", section)

    def recipe_environment(self, directory):
        # Do not inherit credentials, installed configuration or a dev session.
        return {
            "PATH": os.environ["PATH"], "HOME": directory, "LANG": "C.UTF-8",
            "HOT_DEV_BG_PID_FILE": f"{directory}/runtime.pid",
            "HOT_DEV_BG_LOG_FILE": f"{directory}/runtime.log",
            "HOT_DEV_LAB_MODE_FILE": f"{directory}/lab-mode",
            "LAN_IP": "127.0.0.1", "ALL_IPS": "127.0.0.1",
        }

    def test_recipe_exports_reach_the_actual_network_configuration_consumer(self):
        _, recipe = self.development_recipe()
        exports = "\n".join(line for line in recipe.splitlines() if line.startswith("export "))
        with tempfile.TemporaryDirectory() as directory:
            env = self.recipe_environment(directory)
            # An ambient lab setting must not defeat the explicit safe recipe.
            env.update(PULSE_DEV_LAN="true", PULSE_DEV_LAB_AGENTS="true", BIND_ADDRESS="0.0.0.0",
                       HOT_DEV_USE_PROD_DATA="true", HOT_DEV_IMPORT_PROD_CONFIG="true")
            result = subprocess.run(["bash", "-c", "set -eu\n" + exports + '''
source scripts/lib/hot-dev-runtime.sh
hot_dev_configure_network_defaults
test "$HOT_DEV_USE_PRO" = false && test "$HOT_DEV_USE_PROD_DATA" = false
test "$HOT_DEV_IMPORT_PROD_CONFIG" = false
test "$PULSE_DATA_DIR" = "$PWD/tmp/contribution-data"
test "$PULSE_DEV_LAN" = false && test "$PULSE_DEV_LAB_AGENTS" = false
test "$FRONTEND_DEV_HOST" = 127.0.0.1 && test "$PULSE_DEV_API_HOST" = 127.0.0.1
test "$BIND_ADDRESS" = 127.0.0.1
'''], cwd=ROOT, env=env, capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_recipe_selects_no_installed_configuration_through_real_helpers(self):
        _, recipe = self.development_recipe()
        exports = "\n".join(line for line in recipe.splitlines() if line.startswith("export "))
        with tempfile.TemporaryDirectory() as directory:
            installed = Path(directory) / "installed"
            installed.mkdir()
            sentinel = Path(directory) / "unexpected-read-or-copy"
            (installed / ".env").write_text("synthetic installed fixture; never source\n")
            env = self.recipe_environment(directory)
            env.update(HOT_DEV_USE_PROD_DATA="true", HOT_DEV_IMPORT_PROD_CONFIG="true",
                       FIXTURE_INSTALLED=str(installed), FIXTURE_SENTINEL=str(sentinel))
            # Consume the exact documented exports through the production selector,
            # load and copy functions. A synthetic peer records any attempted import;
            # no real installed path, listener, application or host is accessed.
            result = subprocess.run(["bash", "-c", "set -euo pipefail\n" + exports + r'''
source scripts/lib/hot-dev-config.sh
log_error() { printf '%s\n' "$*" >&2; }
log_warn() { printf '%s\n' "$*" >&2; }
load_env_file() { printf 'unexpected load\n' >> "$FIXTURE_SENTINEL"; return 91; }
ROOT_DIR="$PWD"
hot_dev_prepare_config_source "$FIXTURE_INSTALLED"
hot_dev_load_installed_env "$FIXTURE_INSTALLED"
hot_dev_import_installed_config
[[ "$HOT_DEV_USE_PROD_DATA" == false && "$HOT_DEV_IMPORT_PROD_CONFIG" == false ]]
[[ "$PULSE_DATA_DIR" == "$PWD/tmp/contribution-data" ]]
'''], cwd=ROOT, env=env, capture_output=True, text=True, timeout=10)
            self.assertFalse(sentinel.exists(), "recipe selected installed configuration")
            self.assertEqual((installed / ".env").read_text(), "synthetic installed fixture; never source\n")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_documented_start_refuses_occupied_ports_before_stop_or_spawn(self):
        _, recipe = self.development_recipe()
        start = next(shlex.split(line)[1:] for line in recipe.splitlines() if line.endswith(" start"))
        with tempfile.TemporaryDirectory() as directory:
            sentinel = Path(directory) / "unexpected-operation"
            env = self.recipe_environment(directory)
            env["RECIPE_SENTINEL"] = str(sentinel)
            result = subprocess.run(["bash", "-c", '''
source scripts/hot-dev-bg.sh
# Deterministic occupied-port discovery; no real process is signalled or started.
is_running() { return 1; }
has_unmanaged_listeners() { return 0; }
describe_listener() { printf 'fixture occupied port %s\\n' "$1"; }
unexpected_operation() { printf '%s\\n' "$*" >> "$RECIPE_SENTINEL"; return 1; }
stop_launchd_hot_dev_job() { unexpected_operation launchd; }
stop_hot_dev_sessions() { unexpected_operation sessions; }
stop_takeover_targets() { unexpected_operation targets; }
spawn_detached_supervisor() { unexpected_operation spawn; }
main "$@"
''', "recipe", *start], cwd=ROOT, env=env, capture_output=True, text=True, timeout=10)
            self.assertFalse(sentinel.exists(), "recipe attempted takeover or spawn")
            self.assertFalse((Path(directory) / "runtime.pid").exists())
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("Refusing to take over unmanaged listeners", result.stdout + result.stderr)

    def test_root_build_command_resolves_to_the_frontend_package(self):
        guide = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
        self.assertIn("`npm --prefix frontend-modern run build`", guide)
        self.assertNotIn("`gofmt -w ./cmd ./internal ./pkg`", guide)
        self.assertTrue((ROOT / "internal/monitoring").is_dir())

    def test_help_entrypoints_accept_original_evidence_before_public_sharing(self):
        surfaces = (
            ("README.md", "## Community and support\n", "\n## ", "docs/"),
            ("docs/README.md", "# Pulse documentation\n", "\n## Start here", ""),
        )
        for name, start, end, prefix in surfaces:
            with self.subTest(surface=name):
                text = (ROOT / name).read_text(encoding="utf-8")
                entry = " ".join(text.split(start, 1)[1].split(end, 1)[0].split())
                self.assertIn(f"({prefix}TROUBLESHOOTING.md#-getting-help)", entry)
                self.assertIn("A second reproduction or diagnostics export is not required", entry)
                self.assertIn("original evidence", entry)
                self.assertIn("interrupt workloads or send duplicate notifications", entry)
                self.assertIn("reports and attachments are public", entry)
                self.assertIn("review them locally", entry)
                self.assertIn("credentials and private details", entry)
                self.assertNotIn("Report reproducible bugs", entry)
        self.assertEqual(
            (ROOT / "docs/README.md").read_bytes(),
            (ROOT / "frontend-modern/public/docs/README.md").read_bytes(),
        )

    def contribution_surfaces(self):
        return (
            " ".join((ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8").split()),
            " ".join((ROOT / ".github/PULL_REQUEST_TEMPLATE.md").read_text(encoding="utf-8").split()),
        )

    def test_no_new_external_pr_submission_instructions(self):
        guide, template = self.contribution_surfaces()
        for text in (guide, template):
            with self.subTest(surface=text[:40]):
                self.assertIn("not accepting unsolicited external pull requests", text)
                self.assertNotRegex(text, r"(?i)(?:^|[.!?]\s+|\d+\.\s+)Open a PR\b")
        self.assertNotIn("Submitting Requested Changes", guide)
        self.assertNotIn("Every requested PR", guide)

    def test_patch_is_optional_evidence_in_existing_issue(self):
        guide, template = self.contribution_surfaces()
        self.assertIn("## Sharing a tested patch", guide)
        self.assertIn("**existing issue**", guide)
        self.assertIn("do not open a new pull request", guide)
        self.assertIn("not a condition of reporting", guide)
        self.assertIn("not a promise that the change will be used", guide)
        self.assertIn("https://github.com/rcourtman/Pulse/blob/main/CONTRIBUTING.md#sharing-a-tested-patch", template)
        self.assertIn("Do not duplicate an existing report", template)

    def test_old_requested_contributions_keep_their_disposition(self):
        guide, template = self.contribution_surfaces()
        self.assertIn("one requested in an earlier conversation", guide)
        self.assertIn("reply and a disposition in its own thread", guide)
        self.assertIn("equivalent maintainer fix with credit and a commit link", guide)
        self.assertIn("already requested in an earlier conversation", template)
        self.assertIn("reply and disposition in this thread", template)
        for text in (guide, template):
            with self.subTest(surface=text[:40]):
                self.assertNotIn("please close it and open an issue instead", text)
                self.assertIn("refile", text)
        self.assertIn("Do not recreate it or refile evidence", guide)
        self.assertIn("do not recreate the PR or refile its evidence", template)

    def test_code_evidence_preserves_safety_and_publication_limits(self):
        guide, template = self.contribution_surfaces()
        for statement in (
            "source version", "tests actually run and known limits",
            "do not repeat an unsafe failure", "production installation",
            "public branch also exposes its commit history",
            "[SECURITY.md](SECURITY.md)", "fix reply credits its author",
            "release availability and reporter confirmation remain separate facts",
        ):
            self.assertIn(statement, guide)
        self.assertIn("test does not establish release availability", template)

    def test_collection_is_optional_and_distinct_from_downloading_a_result(self):
        text = " ".join(report_guidance().split())
        for statement in (
            "Diagnostics are optional, not a condition of reporting",
            "download buttons reuse that result without running checks again",
            "Run Diagnostics** can make live API and guest-agent requests",
            "Do not run it during a backup, freeze/thaw or an unresponsive-host incident",
            "Keep the original evidence instead",
            "not prove that normal monitoring has recovered or a guest has thawed",
            "docs/TROUBLESHOOTING.md#collect-diagnostics-safely",
        ):
            self.assertIn(statement, text)

    def test_review_is_required_before_public_attachment_even_for_sanitized_exports(self):
        text = " ".join(report_guidance().split())
        for statement in (
            "local file, not an upload", "GitHub (review first)",
            "Export for GitHub (sanitized)** in older versions",
            "review files and screenshots locally before posting",
            "a sanitized export is not a guarantee", "free-text errors",
            "credentials", "session cookies", "secret URLs", "private host",
            "network or personal details", "echoed in errors",
            "Keep **Full (private)** exports private", "configuration", ".env",
            "private keys", "Copy as cURL", "full network exports",
            "Never put credentials in a command line, URL or thread",
        ):
            self.assertIn(statement, text)

    def test_performance_evidence_does_not_require_risky_or_private_collection(self):
        text = " ".join(report_guidance().split())
        for statement in (
            "existing readings or safe passive observations", "Where known",
            "Pulse process", "container or the whole host", "units",
            "measurement window", "uptime", "Unavailable readings are valid evidence",
            "Do not restart, create load or change polling or retention just to measure",
            "do not attach raw profiles, heap dumps, databases or full process command lines",
        ):
            self.assertIn(statement, text)

    def test_shipped_contribution_guide_matches_the_repository_entrypoint(self):
        self.assertEqual(
            (ROOT / "CONTRIBUTING.md").read_bytes(),
            (ROOT / "frontend-modern/public/docs/CONTRIBUTING.md").read_bytes(),
        )


if __name__ == "__main__":
    unittest.main()
