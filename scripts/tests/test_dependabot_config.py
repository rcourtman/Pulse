#!/usr/bin/env python3
"""Guard the low-noise dependency update policy and its covered manifests."""

from fnmatch import fnmatchcase
import json
from pathlib import Path
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
CONFIG = ROOT / ".github" / "dependabot.yml"
SECURITY_SCAN = ROOT / ".github" / "workflows" / "security-scan.yml"
ACTION_MANIFESTS = ROOT / "scripts" / "release_control" / "action_consumer_manifests.json"


def update_directories(entry: dict) -> list[str]:
    return [entry["directory"]] if "directory" in entry else entry["directories"]


def manifest_directories(filename: str) -> list[str]:
    """Return repository-relative Dependabot directories for every manifest."""
    directories = set()
    for manifest in ROOT.rglob(filename):
        relative = manifest.relative_to(ROOT)
        if "node_modules" in relative.parts:
            continue
        parent = relative.parent.as_posix()
        directories.add("/" if parent == "." else f"/{parent}")
    return sorted(directories)


def dockerfile_directories() -> list[str]:
    """Return every directory containing a Docker build manifest."""
    directories = set()
    for manifest in ROOT.rglob("Dockerfile*"):
        relative = manifest.relative_to(ROOT)
        if "node_modules" in relative.parts:
            continue
        parent = relative.parent.as_posix()
        directories.add("/" if parent == "." else f"/{parent}")
    return sorted(directories)


class DependabotConfigTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.config = yaml.safe_load(CONFIG.read_text(encoding="utf-8"))
        cls.npm_updates = [
            entry for entry in cls.config["updates"] if entry["package-ecosystem"] == "npm"
        ]
        cls.updates = {
            entry["package-ecosystem"]: entry
            for entry in cls.config["updates"] if entry["package-ecosystem"] != "npm"
        }

    def test_covers_every_shipped_dependency_ecosystem(self) -> None:
        self.assertEqual(
            set(self.updates),
            {
                "github-actions",
                "gomod",
                "docker",
                "docker-compose",
                "devcontainers",
            },
        )
        self.assertEqual(len(self.config["updates"]), len(self.updates) + len(self.npm_updates))
        self.assertEqual(self.updates["github-actions"]["directory"], "/")
        self.assertEqual(
            sorted(self.updates["gomod"]["directories"]),
            manifest_directories("go.mod"),
        )
        # Unlike a set, this check also rejects two entries for the same lock.
        self.assertCountEqual(
            [directory for entry in self.npm_updates for directory in update_directories(entry)],
            manifest_directories("package-lock.json"),
        )
        self.assertEqual(
            sorted(self.updates["docker"]["directories"]),
            dockerfile_directories(),
        )
        self.assertEqual(
            self.updates["docker-compose"]["directory"],
            "/tests/integration",
        )
        self.assertEqual(self.updates["devcontainers"]["directory"], "/")

    def test_updates_are_weekly_staggered_and_bounded(self) -> None:
        self.assertEqual(self.config["version"], 2)
        times = []
        for entry in self.config["updates"]:
            schedule = entry["schedule"]
            self.assertEqual(schedule["interval"], "weekly")
            self.assertEqual(schedule["day"], "tuesday")
            self.assertEqual(schedule["timezone"], "Europe/London")
            self.assertLessEqual(entry["open-pull-requests-limit"], 3)
            self.assertIn("dependencies", entry["labels"])
            times.append(schedule["time"])
        self.assertEqual(len(times), len(set(times)), "update jobs must stay staggered")

    def test_go_updates_group_reviewable_changes(self) -> None:
        groups = self.updates["gomod"]["groups"]
        self.assertEqual(
            groups["go-minor-patch"],
            {
                "applies-to": "version-updates",
                "patterns": ["*"],
                "exclude-patterns": ["modernc.org/*"],
                "update-types": ["minor", "patch"],
            },
        )
        self.assertEqual(
            groups["go-security"],
            {"applies-to": "security-updates", "patterns": ["*"]},
        )

    def test_npm_proposals_stay_within_each_consumer_scope(self) -> None:
        # The Account lock/embedded proof, the product UI, and the integration
        # harness had different outcomes in the combined update (PR 4970).
        # Keep them in disjoint update blocks, not just differently named groups
        # inside one cross-directory block.
        expected_scopes = {
            "ui": frozenset({"/frontend-modern"}),
            "portal": frozenset({"/internal/cloudcp/portal/frontend"}),
            "tooling": frozenset({"/", "/.github/scripts", "/tests/integration"}),
        }
        by_scope = {frozenset(update_directories(entry)): entry for entry in self.npm_updates}
        self.assertEqual(len(self.npm_updates), len(expected_scopes))
        self.assertEqual(set(by_scope), set(expected_scopes.values()))
        for name, directories in expected_scopes.items():
            with self.subTest(scope=name):
                entry = by_scope[directories]
                self.assertEqual(len({"directory", "directories"} & entry.keys()), 1)
                self.assertNotIn("target-branch", entry)
                self.assertIn("javascript", entry["labels"])
                self.assertEqual(entry["open-pull-requests-limit"], 1)
                self.assertEqual(entry["groups"], {
                    f"npm-{name}-minor-patch": {
                        "applies-to": "version-updates",
                        "patterns": ["*"],
                        "update-types": ["minor", "patch"],
                    },
                    f"npm-{name}-security": {
                        "applies-to": "security-updates",
                        "patterns": ["*"],
                    },
                })

    def test_go_storage_updates_stay_together_without_blocking_other_patches(self) -> None:
        gomod = self.updates["gomod"]
        groups = gomod["groups"]
        self.assertEqual(
            groups["go-storage-runtime"],
            {
                "applies-to": "version-updates",
                "patterns": ["modernc.org/*"],
                "update-types": ["minor", "patch"],
            },
        )
        # Separating review scope must not suppress updates or advisories.
        self.assertEqual(gomod.get("ignore", []), [])
        self.assertEqual(set(groups), {"go-minor-patch", "go-storage-runtime", "go-security"})

        def matching_groups(dependency: str, update_type: str, applies_to: str) -> set[str]:
            return {
                name
                for name, group in groups.items()
                if group["applies-to"] == applies_to
                and update_type in group.get("update-types", ["major", "minor", "patch"])
                and any(fnmatchcase(dependency, pattern) for pattern in group["patterns"])
                and not any(
                    fnmatchcase(dependency, pattern)
                    for pattern in group.get("exclude-patterns", [])
                )
            }

        # PR 3178 rebundled the deferred SQLite/libc pair with a new Prometheus
        # patch immediately after the non-storage refresh landed. Keep the
        # driver's transitives in its own reviewable proposal, in either order.
        for dependency, expected in (
            ("modernc.org/sqlite", "go-storage-runtime"),
            ("modernc.org/libc", "go-storage-runtime"),
            ("modernc.org/memory", "go-storage-runtime"),
            ("modernc.org/mathutil", "go-storage-runtime"),
            ("github.com/prometheus/common", "go-minor-patch"),
            ("github.com/klauspost/compress", "go-minor-patch"),
            ("golang.org/x/net", "go-minor-patch"),
        ):
            for update_type in ("minor", "patch"):
                with self.subTest(dependency=dependency, update_type=update_type):
                    self.assertEqual(
                        matching_groups(dependency, update_type, "version-updates"),
                        {expected},
                    )
            self.assertEqual(matching_groups(dependency, "major", "version-updates"), set())
            self.assertEqual(matching_groups(dependency, "patch", "security-updates"), {"go-security"})

    def test_docker_updates_preserve_governed_tags(self) -> None:
        docker = self.updates["docker"]
        self.assertEqual(
            docker["groups"]["shared-container-images"],
            {"group-by": "dependency-name"},
        )
        ignored = {
            item["dependency-name"]: set(item["update-types"])
            for item in docker["ignore"]
        }
        all_semver = {
            "version-update:semver-major",
            "version-update:semver-minor",
            "version-update:semver-patch",
        }
        self.assertEqual(set(ignored), {"node", "golang", "alpine"})
        self.assertTrue(all(types == all_semver for types in ignored.values()))

        compose_ignored = self.updates["docker-compose"]["ignore"]
        self.assertEqual(
            compose_ignored,
            [
                {
                    "dependency-name": "alpine",
                    "update-types": sorted(all_semver),
                }
            ],
        )

    def test_npm_version_ignores_preserve_browser_and_toolchain_parity(self) -> None:
        all_semver = [
            "version-update:semver-major",
            "version-update:semver-minor",
            "version-update:semver-patch",
        ]
        required_types = {
            **dict.fromkeys(("playwright", "playwright-core", "@playwright/test"), all_semver),
            **dict.fromkeys(
                ("typescript", "eslint", "lucide-solid", "solid-js", "vitest",
                 "@vitest/coverage-v8", "jsdom", "vite"),
                ["version-update:semver-major"],
            ),
        }
        for entry in self.npm_updates:
            directories = update_directories(entry)
            with self.subTest(directories=directories):
                locked = set()
                for directory in directories:
                    lock = ROOT / directory.removeprefix("/") / "package-lock.json"
                    packages = json.loads(lock.read_text(encoding="utf-8"))["packages"]
                    locked.update(
                        path.rsplit("node_modules/", 1)[-1] for path in packages if path
                    )
                # Each scope keeps the old version-update-only rules wherever
                # those packages are locked. No rule may ignore security ranges
                # or exclude ordinary minor/patch upgrades of coupled majors.
                self.assertCountEqual(entry["ignore"], [
                    {"dependency-name": name, "update-types": types}
                    for name, types in required_types.items() if name in locked
                ])

    def test_github_actions_updates_preserve_reviewed_action_pins(self) -> None:
        # action_consumer_manifests.json pins each release-consumed action to an
        # exact SHA plus the upstream action.yml sha256, and
        # release_promotion_policy_test.py requires every workflow step to match
        # it. Refreshing a pin needs the upstream bytes through the reviewed
        # pin-refresh route, so Dependabot must not propose version updates the
        # manifest contract would reject.
        manifests = json.loads(ACTION_MANIFESTS.read_text(encoding="utf-8"))
        ignored = {
            item["dependency-name"]
            for item in self.updates["github-actions"]["ignore"]
        }
        self.assertEqual(ignored, set(manifests))

    def test_weekly_scan_covers_the_same_lockfiles(self) -> None:
        workflow = yaml.safe_load(SECURITY_SCAN.read_text(encoding="utf-8"))
        jobs = workflow["jobs"]
        self.assertEqual(
            set(jobs["govulncheck"]["strategy"]["matrix"]["directory"]),
            {
                "." if path == "/" else path.removeprefix("/")
                for path in manifest_directories("go.mod")
            },
        )
        npm_sets = jobs["npm-audit"]["strategy"]["matrix"]["include"]
        self.assertEqual(
            {item["directory"] for item in npm_sets},
            {
                "." if path == "/" else path.removeprefix("/")
                for path in manifest_directories("package-lock.json")
            },
        )
        scan_steps = jobs["npm-audit"]["steps"]
        audit_commands = [
            step["run"]
            for step in scan_steps
            if step.get("id") in {"audit-complete", "audit-production"}
        ]
        self.assertEqual(
            audit_commands,
            [
                'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" '
                "all --package-lock-only",
                'bash "$GITHUB_WORKSPACE/scripts/npm-audit-retry.sh" '
                "production --package-lock-only",
            ],
        )


if __name__ == "__main__":
    unittest.main()
