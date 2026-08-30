#!/usr/bin/env python3
"""Guard the low-noise dependency update policy and its covered manifests."""

from pathlib import Path
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
CONFIG = ROOT / ".github" / "dependabot.yml"
SECURITY_SCAN = ROOT / ".github" / "workflows" / "security-scan.yml"


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
            self.assertEqual(
                groups[version_group],
                {
                    "applies-to": "version-updates",
                    "patterns": ["*"],
                    "update-types": ["minor", "patch"],
                },
            )
            self.assertEqual(
                groups[security_group],
                {"applies-to": "security-updates", "patterns": ["*"]},
            )

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
        audit_commands = [step["run"] for step in scan_steps if "run" in step]
        self.assertEqual(
            audit_commands,
            [
                "npm audit --package-lock-only",
                "npm audit --package-lock-only --omit=dev",
            ],
        )


if __name__ == "__main__":
    unittest.main()
