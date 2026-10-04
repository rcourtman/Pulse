#!/usr/bin/env python3
"""Execute the published installer recipes with controlled transport failures."""

import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


class SignedInstallerDocsTest(unittest.TestCase):
    def test_download_and_signature_failures_never_execute_installer(self):
        blocks = [
            block for block in re.findall(r"```bash\n(.*?)\n```", (ROOT / "docs/INSTALL.md").read_text(), re.S)
            if "ssh-keygen -Y verify" in block
        ]
        self.assertEqual(len(blocks), 2, "Exercise both LXC and systemd recipes")
        for recipe, block in enumerate(blocks):
            for failure in ("installer-download", "signature-download", "signature", "none"):
                with self.subTest(recipe=recipe, failure=failure), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    commands = root / "bin"
                    commands.mkdir()
                    scripts = {
                        "curl": '''#!/bin/bash
set -e
url="${!#}"
case "$url" in
  */install.sh)
    [[ "$FAILURE" != installer-download ]] || exit 22
    printf '%s\\n' '#!/bin/bash' 'printf "%s\\n" "$@" > "$INSTALL_RECEIPT"' > install.sh
    ;;
  */install.sh.sshsig)
    [[ "$FAILURE" != signature-download ]] || exit 22
    printf '%s\\n' fixture-signature > install.sh.sshsig
    ;;
  *) exit 23 ;;
esac
''',
                        "ssh-keygen": '#!/bin/bash\n[[ "$FAILURE" != signature ]]\n',
                        "sudo": '#!/bin/bash\nexec "$@"\n',
                    }
                    for name, source in scripts.items():
                        executable = commands / name
                        executable.write_text(source)
                        executable.chmod(0o755)
                    receipt = root / "executed"
                    env = {**os.environ, "PATH": f"{commands}:{os.environ['PATH']}",
                           "FAILURE": failure, "INSTALL_RECEIPT": str(receipt)}
                    result = subprocess.run(["bash", "-c", block.replace("vX.Y.Z", "v6.4.5-rc.2")],
                                            cwd=root, env=env, capture_output=True, text=True)
                    if failure == "none":
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertEqual(receipt.read_text(), "--version\nv6.4.5-rc.2\n")
                    else:
                        self.assertNotEqual(result.returncode, 0)
                        self.assertFalse(receipt.exists(), "Unverified installer was executed")


if __name__ == "__main__":
    unittest.main()
