#!/usr/bin/env python3
"""Guard the low-noise dependency update policy and its covered manifests."""

import json
from fnmatch import fnmatchcase
from pathlib import Path
import re
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
CONFIG = ROOT / ".github" / "dependabot.yml"
SECURITY_SCAN = ROOT / ".github" / "workflows" / "security-scan.yml"
ACTION_MANIFESTS = ROOT / "scripts" / "release_control" / "action_consumer_manifests.json"


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
        cls.updates = {
            entry["package-ecosystem"]: entry for entry in cls.config["updates"]
        }

    def test_covers_every_shipped_dependency_ecosystem(self) -> None:
        self.assertEqual(
            set(self.updates),
            {
                "github-actions",
                "gomod",
                "npm",
                "docker",
                "docker-compose",
                "devcontainers",
            },
        )
        self.assertEqual(self.updates["github-actions"]["directory"], "/")
        self.assertEqual(
            sorted(self.updates["gomod"]["directories"]),
            manifest_directories("go.mod"),
        )
        self.assertEqual(
            sorted(self.updates["npm"]["directories"]),
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

    def test_language_updates_group_reviewable_changes(self) -> None:
        for ecosystem, version_group, security_group in (
            ("gomod", "go-minor-patch", "go-security"),
            ("npm", "npm-minor-patch", "npm-security"),
        ):
            groups = self.updates[ecosystem]["groups"]
            expected_version_group = {
                "applies-to": "version-updates",
                "patterns": ["*"],
                "update-types": ["minor", "patch"],
            }
            if ecosystem == "gomod":
                expected_version_group["exclude-patterns"] = ["modernc.org/*"]
            self.assertEqual(groups[version_group], expected_version_group)
            self.assertEqual(
                groups[security_group],
                {"applies-to": "security-updates", "patterns": ["*"]},
            )

    def test_go_storage_version_updates_have_a_disjoint_review_group(self) -> None:
        go = self.updates["gomod"]
        self.assertEqual(
            go["groups"]["go-storage-minor-patch"],
            {
                "applies-to": "version-updates",
                "patterns": ["modernc.org/*"],
                "update-types": ["minor", "patch"],
            },
        )
        # Grouping is not a deferral: keep every dependency eligible, including
        # security updates. The resolver and containing CI still own transitive
        # graph compatibility; this only separates the named update roots.
        self.assertNotIn("ignore", go)
        self.assertNotIn("allow", go)

        def matching_groups(name, kind, update_type):
            return [
                group_name
                for group_name, group in go["groups"].items()
                if group["applies-to"] == kind
                and update_type in group.get("update-types", ["major", "minor", "patch"])
                and any(fnmatchcase(name, pattern) for pattern in group["patterns"])
                and not any(fnmatchcase(name, pattern) for pattern in group.get("exclude-patterns", []))
            ]

        # These are the eleven update roots in mixed proposal #3056, not an
        # invented happy-path graph. SQLite must no longer share a version
        # proposal with the independent compression/metrics/agent updates.
        proposed = {
            "github.com/klauspost/compress", "github.com/moby/moby/api",
            "github.com/moby/moby/client", "github.com/prometheus/client_golang",
            "github.com/prometheus/common", "github.com/shirou/gopsutil/v4",
            "golang.org/x/crypto", "golang.org/x/sync", "golang.org/x/sys",
            "golang.org/x/term", "modernc.org/sqlite",
        }
        # Include every currently required module (direct and indirect) in
        # both manifests. The whole modernc family needs the same boundary.
        required = {
            match[1]
            for directory in go["directories"]
            for match in re.finditer(
                r"(?m)^\s*([^\s()]+)\s+v[0-9][^\s]*",
                (ROOT / directory.lstrip("/") / "go.mod").read_text(),
            )
        }
        self.assertTrue(proposed <= required)
        self.assertIn("modernc.org/libc", required)
        for name in sorted(required | proposed):
            expected = "go-storage-minor-patch" if fnmatchcase(name, "modernc.org/*") else "go-minor-patch"
            for update_type in ("minor", "patch"):
                with self.subTest(name=name, update_type=update_type):
                    self.assertEqual(matching_groups(name, "version-updates", update_type), [expected])
            with self.subTest(name=name, update_type="major"):
                self.assertEqual(matching_groups(name, "version-updates", "major"), [])
            for update_type in ("major", "minor", "patch"):
                with self.subTest(name=name, security_type=update_type):
                    self.assertEqual(matching_groups(name, "security-updates", update_type), ["go-security"])

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

    def test_npm_updates_preserve_browser_runtime_parity(self) -> None:
        all_semver = {
            "version-update:semver-major",
            "version-update:semver-minor",
            "version-update:semver-patch",
        }
        ignored = {
            item["dependency-name"]: set(item["update-types"])
            for item in self.updates["npm"]["ignore"]
        }
        self.assertEqual(
            set(ignored),
            {
                "playwright",
                "playwright-core",
                "@playwright/test",
                "typescript",
                "eslint",
                "@eslint/js",
                "lucide-solid",
                "vitest",
                "@vitest/coverage-v8",
                "jsdom",
                "vite",
            },
        )
        for browser_runtime in ("playwright", "playwright-core", "@playwright/test"):
            self.assertEqual(ignored[browser_runtime], all_semver)
        # These majors require an explicit toolchain, DOM or import migration.
        # Minor and patch version updates must remain eligible for Dependabot.
        for dependency in (
            "typescript",
            "eslint",
            "@eslint/js",
            "lucide-solid",
            "vitest",
            "@vitest/coverage-v8",
            "jsdom",
            "vite",
        ):
            self.assertEqual(
                ignored[dependency],
                {"version-update:semver-major"},
            )

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
