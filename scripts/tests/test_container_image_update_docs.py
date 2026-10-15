#!/usr/bin/env python3
"""Exercise the shipped passive image comparison without a runtime or registry.

The synthetic CLI accepts only the two documented inspection operations. This
proves quoting, exact-image selection and failure handling, not a collector fix
or native Docker/Podman recovery.
"""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "Image update disagrees with the runtime"


def section():
    text = (ROOT / "docs/DOCKER.md").read_text()
    return text.split(f"### {HEADING}\n", 1)[1].split("\n### ", 1)[0]


def recipe():
    blocks = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    if len(blocks) != 1:
        raise AssertionError("expected one passive image recipe")
    return blocks[0]


CLI = r'''#!/usr/bin/python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
with Path(os.environ["OPERATIONS"]).open("a") as output:
    output.write(json.dumps(args) + "\n")
if args == ["inspect", "--type", "container", "--format", "{{.Image}}", "affected-container"]:
    if os.environ["MODE"] == "container-error":
        print("container inspection unavailable", file=sys.stderr)
        sys.exit(17)
    print(os.environ["RUNNING_IMAGE_ID"])
elif args == ["image", "inspect", "--format", "{{json .RepoDigests}}", os.environ["RUNNING_IMAGE_ID"]]:
    if os.environ["MODE"] == "image-error":
        print("local image inspection unavailable", file=sys.stderr)
        sys.exit(23)
    print(json.dumps(None if os.environ["MODE"] == "no-digests" else [
        "example.test/app@sha256:first", "example.test/app@sha256:second"]))
else:
    raise AssertionError("undocumented or mutating operation")
'''


class ContainerImageUpdateDocsTest(unittest.TestCase):
    def run_recipe(self, runtime="docker", mode="ok", image_id="sha256:running-image"):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cli = root / runtime
            cli.write_text(CLI)
            cli.chmod(0o700)
            operations = root / "operations.jsonl"
            env = dict(os.environ, PATH=f"{root}:{os.environ['PATH']}",
                       MODE=mode, RUNNING_IMAGE_ID=image_id, OPERATIONS=str(operations),
                       SYNTHETIC_PRIVATE_CONFIGURATION="must-not-be-disclosed")
            code = recipe().replace("docker ", f"{runtime} ")
            result = subprocess.run(["bash", "-c", code], cwd=root, env=env,
                                    capture_output=True, text=True, timeout=5)
            calls = [json.loads(line) for line in operations.read_text().splitlines()]
            self.assertFalse((root / "injected").exists())
            self.assertNotIn(env["SYNTHETIC_PRIVATE_CONFIGURATION"], result.stdout + result.stderr)
            return result, calls

    def test_docker_and_podman_read_the_running_image_not_the_current_tag(self):
        for runtime in ("docker", "podman"):
            with self.subTest(runtime=runtime):
                result, calls = self.run_recipe(runtime)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(json.loads(result.stdout), [
                    "example.test/app@sha256:first", "example.test/app@sha256:second"])
                self.assertEqual(len(calls), 2)
                self.assertEqual(calls[1][-1], "sha256:running-image")

    def test_failed_container_read_stops_before_image_inspection(self):
        result, calls = self.run_recipe(mode="container-error")
        self.assertEqual(result.returncode, 17)
        self.assertEqual(result.stdout, "")
        self.assertIn("container inspection unavailable", result.stderr)
        self.assertEqual(len(calls), 1)

    def test_failed_image_read_remains_visible_and_nonzero(self):
        result, calls = self.run_recipe(mode="image-error")
        self.assertEqual(result.returncode, 23)
        self.assertEqual(result.stdout, "")
        self.assertIn("local image inspection unavailable", result.stderr)
        self.assertEqual(len(calls), 2)

    def test_no_repo_digests_remains_missing_evidence(self):
        result, calls = self.run_recipe(mode="no-digests")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIsNone(json.loads(result.stdout))
        self.assertEqual(len(calls), 2)
        self.assertIn("An empty list is not proof that the image is current", section())

    def test_image_id_is_an_argument_not_shell_source(self):
        value = "sha256:synthetic; touch injected"
        result, calls = self.run_recipe(image_id=value)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls[1][-1], value)

    def test_context_privacy_and_action_uncertainty_are_preserved(self):
        text = section()
        for required in ("same host,", "same account and Docker context",
                         "rootless Podman connection", "not contact a registry",
                         "not just the image currently stored under its tag",
                         "If inspection fails", "Never share registry credentials",
                         "do not substitute a full inspection", "clear alerts or retry an update",
                         "daily\nresolve/reopen cycle", "#check-a-failed-or-pending-workload-update"):
            with self.subTest(required=required):
                self.assertIn(required, text)
        self.assertNotRegex(recipe(), r"(?:pull|restart|sudo|printenv|login|curl|--token|\.Config)")
        for field in ("Current digest", "Target digest"):
            self.assertIn(field, (ROOT / "frontend-modern/src/components/Infrastructure/ResourceDetailSummary.tsx").read_text())

    def test_shipped_mirrors_and_troubleshooting_entry_match(self):
        for name in ("DOCKER.md", "TROUBLESHOOTING.md"):
            with self.subTest(name=name):
                self.assertEqual((ROOT / "docs" / name).read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / name).read_bytes())
        self.assertIn("DOCKER.md#image-update-disagrees-with-the-runtime",
                      (ROOT / "docs/TROUBLESHOOTING.md").read_text())


if __name__ == "__main__":
    unittest.main()
