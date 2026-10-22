#!/usr/bin/env python3
"""Execute copied server recipes with argv recorders, never Helm or a cluster.

These checks prove command selection, flags and failure propagation. They do
not render a chart, model Helm's value merging, or establish rollout recovery.
"""

import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
VERSION = "6.5.0"


def section(start, end):
    return (ROOT / "docs/KUBERNETES.md").read_text().split(start + "\n", 1)[1].split(end, 1)[0]


def blocks(text):
    return re.findall(r"```bash\n(.*?)```", text, re.S)


RECORDER = r'''
import json, os, sys
tool, args = os.path.basename(sys.argv[0]), sys.argv[1:]
phase = tool + ':' + (':'.join(args[:2]) if args[:1] == ['repo'] else args[0])
with open(os.environ['CALLS'], 'a') as stream:
    stream.write(json.dumps({'tool': tool, 'args': args, 'phase': phase}) + '\n')
if os.environ.get('FAIL_PHASE') == phase:
    print('synthetic command failure', file=sys.stderr)
    sys.exit(37)
if phase == 'helm:template':
    print('# synthetic manifest: this is not Helm rendering')
'''


class KubernetesServerDocsTest(unittest.TestCase):
    def server(self):
        return section("## 1. Deploying the Pulse Server", "\n## 2.")

    def openshift(self):
        return blocks(section("### OpenShift profile (Helm)", "\n### "))

    def exercise(self, command, *, failure="", existing_output=False):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tools = root / "tools"
            tools.mkdir()
            for name in ("helm", "kubectl"):
                tool = tools / name
                tool.write_text(f"#!{sys.executable}\n" + RECORDER)
                tool.chmod(0o700)
            if existing_output:
                (root / "pulse-server.yaml").write_text("original GitOps deployment\n")
            calls = root / "calls.jsonl"
            env = dict(os.environ, PATH=f"{tools}:{os.environ['PATH']}",
                       CALLS=str(calls), FAIL_PHASE=failure)
            command = command.replace("X.Y.Z", VERSION)
            subprocess.run(["bash", "-n", "-c", command], check=True, capture_output=True)
            result = subprocess.run(["bash", "-c", command], cwd=root, env=env,
                                    capture_output=True, text=True, timeout=10)
            recorded = [json.loads(line) for line in calls.read_text().splitlines()] if calls.exists() else []
            output = root / "pulse-server.yaml"
            return result, recorded, output.read_text() if output.exists() else None

    def assert_server_flags(self, args):
        self.assertEqual(args[args.index("--version") + 1], VERSION)
        self.assertEqual(args[args.index("--namespace") + 1], "pulse")
        for setting in ("replicaCount=1", "strategy.type=Recreate", "persistence.enabled=true"):
            self.assertIn(setting, args)
        for unsafe in ("--install", "--devel", "--force", "--replace", "--take-ownership"):
            self.assertNotIn(unsafe, args)

    def test_new_install_never_selects_upgrade(self):
        command = blocks(self.server().split("### Option B:", 1)[0])
        self.assertEqual(len(command), 1, "one fail-stopping repository/install block")
        result, calls, _ = self.exercise(command[0])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([call["phase"] for call in calls],
                         ["helm:repo:add", "helm:repo:update", "helm:install"])
        self.assertEqual(calls[1]["args"], ["repo", "update", "pulse"])
        self.assertEqual(calls[-1]["args"][:3], ["install", "pulse", "pulse/pulse"])
        self.assert_server_flags(calls[-1]["args"])
        self.assertIn("--create-namespace", calls[-1]["args"])

    def test_repository_errors_stop_before_install(self):
        command = "\n".join(blocks(self.server().split("### Option B:", 1)[0]))
        for phase, count in (("helm:repo:add", 1), ("helm:repo:update", 2)):
            with self.subTest(phase=phase):
                result, calls, _ = self.exercise(command, failure=phase)
                self.assertEqual(result.returncode, 37, result.stderr)
                self.assertEqual(len(calls), count)
                self.assertNotIn("helm:install", [call["phase"] for call in calls])

    def test_install_failure_is_returned_without_retry(self):
        command = "\n".join(blocks(self.server().split("### Option B:", 1)[0]))
        result, calls, _ = self.exercise(command, failure="helm:install")
        self.assertEqual(result.returncode, 37, result.stderr)
        self.assertEqual([call["phase"] for call in calls].count("helm:install"), 1)

    def test_static_render_is_pinned_single_writer_and_not_an_apply(self):
        command = blocks(self.server().split("### Option B:", 1)[1])[0]
        result, calls, output = self.exercise(command)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([call["phase"] for call in calls],
                         ["helm:repo:add", "helm:repo:update", "helm:template"])
        self.assert_server_flags(calls[-1]["args"])
        self.assertIn("synthetic manifest", output)

    def test_static_render_preserves_existing_file(self):
        command = blocks(self.server().split("### Option B:", 1)[1])[0]
        result, calls, output = self.exercise(command, existing_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(output, "original GitOps deployment\n")
        self.assertNotIn("helm:template", [call["phase"] for call in calls])

    def test_static_failure_never_applies_anything(self):
        command = blocks(self.server().split("### Option B:", 1)[1])[0]
        for phase in ("helm:repo:add", "helm:repo:update", "helm:template"):
            with self.subTest(phase=phase):
                result, calls, _ = self.exercise(command, failure=phase)
                self.assertEqual(result.returncode, 37, result.stderr)
                self.assertFalse(any(call["tool"] == "kubectl" for call in calls))
                self.assertEqual(calls[-1]["phase"], phase)

    def test_static_apply_names_the_namespace_explicitly(self):
        command = blocks(self.server().split("### Option B:", 1)[1])[1]
        result, calls, _ = self.exercise(command)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls[0]["tool"], "kubectl")
        self.assertEqual(calls[0]["args"], ["--namespace", "pulse", "apply", "-f", "pulse-server.yaml"])

    def test_openshift_install_and_settings_change_stay_separate(self):
        commands = self.openshift()
        self.assertEqual(len(commands), 2)
        result, calls, _ = self.exercise(commands[0])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls[0]["args"][:3], ["install", "pulse", "pulse/pulse"])
        self.assert_server_flags(calls[0]["args"])
        self.assertIn("openShift.enabled=true", calls[0]["args"])
        self.assertNotIn("openShift.kubernetesAgent.enabled=true", calls[0]["args"])
        result, calls, _ = self.exercise(commands[1])
        self.assertEqual(result.returncode, 0, result.stderr)
        args = calls[0]["args"]
        self.assertEqual(args[:3], ["upgrade", "pulse", "pulse/pulse"])
        self.assertNotIn("--install", args)
        self.assertIn("--reuse-values", args)
        self.assertEqual(args[args.index("--version") + 1], VERSION)
        self.assertIn("agent.secretEnv.name=pulse-agent-env", args)
        self.assertIn("agent.secretEnv.keys[0]=PULSE_TOKEN", args)
        self.assertNotIn("secretEnv.data", " ".join(args))

    def test_related_entrypoints_keep_install_and_update_distinct(self):
        install = (ROOT / "docs/INSTALL.md").read_text().split("### 1. Kubernetes (Helm)\n")[1].split("\n### 2.")[0]
        self.assertNotIn("helm upgrade", install)
        self.assertIn("KUBERNETES.md#option-a-using-helm-recommended", install)
        guide = (ROOT / "docs/KUBERNETES.md").read_text()
        for phrase in ("single Pulse server", "brief monitoring interruption", "not high availability",
                       "without a leading `v`", "same installed chart version", "not a version-upgrade recipe",
                       "DEPLOYMENT_MODELS.md#kubernetes-helm"):
            self.assertIn(phrase, guide)
        for name in ("KUBERNETES", "INSTALL", "DEPLOYMENT_MODELS"):
            self.assertEqual((ROOT / f"docs/{name}.md").read_bytes(),
                             (ROOT / f"frontend-modern/public/docs/{name}.md").read_bytes())

    def test_documented_values_reach_the_existing_chart_fields(self):
        deployment = (ROOT / "deploy/helm/pulse/templates/deployment.yaml").read_text()
        for field in (".Values.replicaCount", ".Values.strategy.type", ".Values.persistence.existingClaim",
                      ".Values.image.repository", ".Chart.AppVersion", ".Values.image.tag"):
            self.assertIn(field, deployment)
        # Bind the reason for explicit recipe settings to the actual defaults;
        # this test does not claim that storage class selection serialises pods.
        values = (ROOT / "deploy/helm/pulse/values.yaml").read_text()
        self.assertIn("replicaCount: 1", values)
        self.assertIn("type: RollingUpdate", values)
        self.assertIn("- ReadWriteOnce", values)


if __name__ == "__main__":
    unittest.main()
