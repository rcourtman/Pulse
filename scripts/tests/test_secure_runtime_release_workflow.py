"""Exercise both verification steps after the canonical branch moves."""

import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


WORKFLOW = Path(__file__).resolve().parents[2] / ".github/workflows/qualify-secure-runtime-release.yml"
PREAUTH = "Pre-authenticate exact qualification packet"
ATTEST = "Create release-candidate secure-runtime attestation"
TAG = "v6.9.0-rc.3"


def run_step(name):
    step = WORKFLOW.read_text().split(f"      - name: {name}\n", 1)[1]
    return textwrap.dedent(step.split("      - name:", 1)[0].split("        run: |\n", 1)[1])


class QualificationBranchRefreshTest(unittest.TestCase):
    def setUp(self):
        fixture = tempfile.TemporaryDirectory()
        self.addCleanup(fixture.cleanup)
        self.root = Path(fixture.name)
        shims = self.root / "bin"
        shims.mkdir()
        self.origin = self.root / "origin"
        self.checkout = self.root / "rc"
        self.env = {
            "HOME": str(self.root),
            "PATH": f"{shims}:{os.environ['PATH']}",
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_AUTHOR_NAME": "Fixture",
            "GIT_AUTHOR_EMAIL": "fixture@example.test",
            "GIT_COMMITTER_NAME": "Fixture",
            "GIT_COMMITTER_EMAIL": "fixture@example.test",
        }
        (shims / "go").write_text(
            '#!/bin/bash\nset -eu\ntest "$#" = 5\n'
            'test "$1 $2 $3" = "run ./scripts/release_update_key.go fingerprint"\n'
            'printf "%s\\n" fixture-fingerprint\n'
        )
        # Model only the legacy attester's existing ref/commit admission. The
        # actual script and packet verifier belong to the published RC.
        (shims / "python3").write_text(
            """#!/bin/bash
set -euo pipefail
test "$1" = scripts/release_control/secure_runtime_attestation_v7.py
: > "$RUNNER_TEMP/attester-invoked"
shift
mode=attest
if [[ "$1" == --verify-release-packet-only ]]; then
  mode=preauth
  shift
fi
checkout= commit= line= tag=
while (( $# )); do
  case "$1" in
    --checkout) checkout="$2" ;;
    --commit) commit="$2" ;;
    --main-ref) line="$2" ;;
    --release-candidate-tag) tag="$2" ;;
  esac
  shift 2
done
cd "$checkout"
test "$(git rev-parse HEAD)" = "$commit"
test "$(git rev-parse "${tag}^{commit}")" = "$commit"
test -z "$(git branch --show-current)"
ref="refs/heads/${line#origin/}"
local="$(git rev-parse "$line")"
remote="$(git ls-remote origin "$ref")"
if [[ "$remote" != "$local"$'\t'"$ref" ]]; then
  echo "canonical $line does not match the remote branch commit" >&2
  exit 1
fi
git merge-base --is-ancestor "$commit" "$line"
printf '%s %s\\n' "$mode" "$local"
"""
        )
        for shim in shims.iterdir():
            shim.chmod(0o755)

    def git(self, cwd, *args):
        return subprocess.run(
            ["git", "-C", cwd, *args], env=self.env, check=True,
            capture_output=True, text=True, timeout=15,
        ).stdout.strip()

    def prepare(self, branch):
        self.git(self.root, "init", "--initial-branch=main", str(self.origin))
        self.git(self.origin, "commit", "--allow-empty", "-m", "published source")
        self.commit = self.git(self.origin, "rev-parse", "HEAD")
        self.git(self.origin, "tag", "-a", TAG, "-m", f"Release {TAG}")
        if branch != "main":
            self.git(self.origin, "branch", branch)
        self.git(self.origin, "switch", branch)
        self.git(self.origin, "commit", "--allow-empty", "-m", "canonical head at checkout")
        self.initial_tip = self.git(self.origin, "rev-parse", "HEAD")
        self.git(self.root, "clone", "--no-hardlinks", str(self.origin), str(self.checkout))
        self.git(self.checkout, "checkout", "--detach", TAG)
        self.tag_object = self.git(self.checkout, "rev-parse", TAG)
        self.line_ref = f"origin/{branch}"

    def invoke(self, name, *, line=None):
        env = self.env | {
            "GITHUB_WORKSPACE": str(self.checkout),
            "RUNNER_TEMP": str(self.root),
            "PULSE_UPDATE_SIGNING_PUBLIC_KEY": "fixture-key",
            "QUALIFIED_COMMIT": self.commit,
            "LINE_REF": self.line_ref if line is None else line,
            "TAG": TAG,
            "GITHUB_REPOSITORY": "rcourtman/Pulse",
            "RELEASE_ID": "123",
            "WORKFLOW_OUTPUT_1": "1",
        }
        return subprocess.run(
            ["bash", "-c", run_step(name)], cwd=self.checkout, env=env,
            capture_output=True, text=True, timeout=30,
        )

    def verify_step(self, name, mode):
        self.git(self.origin, "commit", "--allow-empty", "-m", f"advance before {mode}")
        tip = self.git(self.origin, "rev-parse", "HEAD")
        run = self.invoke(name)
        self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
        self.assertEqual(run.stdout.strip(), f"{mode} {tip}")
        self.assertEqual(self.git(self.checkout, "rev-parse", self.line_ref), tip)
        self.assertEqual(self.git(self.checkout, "rev-parse", "HEAD"), self.commit)
        self.assertEqual(self.git(self.checkout, "rev-parse", TAG), self.tag_object)
        self.assertEqual(self.git(self.checkout, "tag", "--list"), TAG)

    def test_both_verifiers_see_advancing_main_without_switching_the_tag(self):
        self.prepare("main")
        self.git(self.origin, "tag", "unrelated-new-tag")
        self.verify_step(PREAUTH, "preauth")
        self.verify_step(ATTEST, "attest")

    def test_patch_verifiers_refresh_only_the_mapped_release_branch(self):
        self.prepare("release/v6.9")
        old_main = self.git(self.checkout, "rev-parse", "origin/main")
        self.git(self.checkout, "config", "--add", "remote.origin.fetch",
                 "refs/heads/release/v6.9:refs/remotes/origin/extra")
        self.git(self.origin, "switch", "main")
        self.git(self.origin, "commit", "--allow-empty", "-m", "unrelated main movement")
        self.git(self.origin, "switch", "release/v6.9")
        self.verify_step(PREAUTH, "preauth")
        self.verify_step(ATTEST, "attest")
        self.assertEqual(self.git(self.checkout, "rev-parse", "origin/main"), old_main)
        self.assertEqual(self.git(self.checkout, "branch", "-r", "--list", "origin/extra"), "")

    def test_ref_loss_and_unexpected_line_do_not_reach_the_attester(self):
        self.prepare("release/v6.9")
        self.git(self.origin, "switch", "main")
        self.git(self.origin, "branch", "-D", "release/v6.9")
        for line in (self.line_ref, "origin/other"):
            with self.subTest(line=line):
                run = self.invoke(ATTEST, line=line)
                self.assertNotEqual(run.returncode, 0)
                self.assertFalse((self.root / "attester-invoked").exists(), run.stderr)

    def test_non_fast_forward_branch_change_does_not_reattest(self):
        self.prepare("main")
        self.git(self.origin, "reset", "--hard", self.commit)
        run = self.invoke(ATTEST)
        self.assertNotEqual(run.returncode, 0)
        self.assertFalse((self.root / "attester-invoked").exists(), run.stderr)


if __name__ == "__main__":
    unittest.main()
