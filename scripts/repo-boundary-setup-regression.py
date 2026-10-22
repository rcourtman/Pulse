#!/usr/bin/env python3
"""Exercise the workflow's real dependency block without network or root access."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[1]


def dependency_block():
    workflow = (ROOT / ".github/workflows/repo-boundary-audit.yml").read_text()
    step = workflow.split("      - name: Install boundary audit dependencies\n", 1)[1]
    block = step.split("        run: |\n", 1)[1].split("\n      - name:", 1)[0]
    return textwrap.dedent(block)


class BoundaryDependencySetupTests(unittest.TestCase):
    def run_setup(self, *, installed=False, update_exit=0, install_exit=0,
                  rg_exit=0, install_provides_rg=True):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            runner_temp = root / "runner-temp"
            runner_temp.mkdir()
            log = root / "calls.jsonl"
            rg_source = (
                f"#!{sys.executable}\n"
                "import json, os, sys\n"
                "with open(os.environ['SETUP_CALL_LOG'], 'a') as log:\n"
                "    log.write(json.dumps(['rg', *sys.argv[1:]]) + '\\n')\n"
                f"sys.exit({rg_exit})\n"
            )
            if installed:
                (bin_dir / "rg").write_text(rg_source)
                (bin_dir / "rg").chmod(0o755)
            # No ambient rg, sudo, apt, or shell startup files can be used.
            for tool in ("mktemp", "chmod"):
                os.symlink(shutil.which(tool), bin_dir / tool)
            sudo = bin_dir / "sudo"
            sudo.write_text(f"#!{sys.executable}\n" + textwrap.dedent(f"""\
                import json, os, pathlib, sys
                args = sys.argv[1:]
                with open(os.environ['SETUP_CALL_LOG'], 'a') as log:
                    log.write(json.dumps(['sudo', *args]) + '\\n')
                prefix = [
                    'apt-get',
                    '-o', 'Dir::Etc::sourcelist=/etc/apt/sources.list.d/ubuntu.sources',
                    '-o', 'Dir::Etc::sourceparts=-',
                ]
                # Model the reported unrelated repository refusal. An unscoped
                # update fails; no package operation may silently ignore it.
                if args[:5] != prefix or len(args) < 9 or args[5] != '-o':
                    print('unrelated runner repository: 403 Forbidden', file=sys.stderr)
                    sys.exit(100)
                lists = args[6].removeprefix('Dir::State::lists=')
                path = pathlib.Path(lists)
                if (not args[6].startswith('Dir::State::lists=')
                        or path.parent != pathlib.Path(os.environ['RUNNER_TEMP'])
                        or not path.is_dir() or any(path.iterdir())):
                    sys.exit(98)
                operation = args[7:]
                if operation == ['update', '--error-on=any']:
                    sys.exit({update_exit})
                if operation != ['install', '--yes', '--no-install-recommends', 'ripgrep']:
                    sys.exit(97)
                if {install_exit} == 0 and {install_provides_rg!r}:
                    rg = pathlib.Path({str(bin_dir / 'rg')!r})
                    rg.write_text({rg_source!r})
                    rg.chmod(0o755)
                sys.exit({install_exit})
                """))
            sudo.chmod(0o755)
            result = subprocess.run(
                [shutil.which("bash"), "--noprofile", "--norc", "-c", dependency_block()],
                env={"PATH": str(bin_dir), "RUNNER_TEMP": str(runner_temp),
                     "SETUP_CALL_LOG": str(log), "LC_ALL": "C"},
                text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                timeout=10, check=False,
            )
            calls = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
            return result, calls

    def test_usable_preinstalled_rg_needs_no_package_repository(self):
        result, calls = self.run_setup(installed=True)
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual(calls, [["rg", "--version"]])

    def test_broken_preinstalled_rg_fails_without_package_fallback(self):
        result, calls = self.run_setup(installed=True, rg_exit=23)
        self.assertEqual(result.returncode, 23, result.stdout)
        self.assertEqual(calls, [["rg", "--version"]])

    def test_missing_rg_installs_only_from_fresh_ubuntu_indexes(self):
        result, calls = self.run_setup()
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual(len(calls), 3, calls)
        self.assertEqual(calls[0][:-2], calls[1][:-4])
        self.assertEqual(calls[0][-2:], ["update", "--error-on=any"])
        self.assertEqual(calls[1][-4:], ["install", "--yes", "--no-install-recommends", "ripgrep"])
        self.assertEqual(calls[2], ["rg", "--version"])

    def test_ubuntu_index_failure_stops_before_install(self):
        result, calls = self.run_setup(update_exit=100)
        self.assertEqual(result.returncode, 100, result.stdout)
        self.assertEqual(len(calls), 1, calls)
        self.assertEqual(calls[0][-2:], ["update", "--error-on=any"])

    def test_package_install_failure_remains_terminal(self):
        result, calls = self.run_setup(install_exit=42)
        self.assertEqual(result.returncode, 42, result.stdout)
        self.assertEqual(len(calls), 2, calls)

    def test_successful_apt_without_rg_does_not_pass_setup(self):
        result, calls = self.run_setup(install_provides_rg=False)
        self.assertEqual(result.returncode, 127, result.stdout)
        self.assertEqual(len(calls), 2, calls)


if __name__ == "__main__":
    unittest.main()
