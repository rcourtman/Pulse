#!/usr/bin/env python3
"""Portable wiring checks; real file/ACL controls run in Windows PowerShell 5.1."""

from pathlib import Path
import os
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = ROOT / "scripts/install.ps1"


def preparation_contract(script: str) -> None:
    def require(fragment: str, text: str = script) -> int:
        if fragment not in text:
            raise ValueError(f"missing binary preparation boundary: {fragment}")
        return text.index(fragment)

    start = require("function New-PulsePreparedBinary {")
    end = require("function Test-HasPinnedInstallerSignatureKey {")
    if start >= end:
        raise ValueError("missing complete preparation function")
    function = script[start:end]
    for fragment in (
        "[ValidatePattern('^[0-9a-fA-F]{64}$')]",
        "[Guid]::NewGuid().ToString('N')",
        "Join-Path $DestinationDirectory",
        "[System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::Read",
        "[System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None",
        "$sourceStream.CopyTo($preparedStream)",
        "if ($null -ne $preparedStream) { $preparedStream.Dispose() }",
        "if ($null -ne $sourceStream) { $sourceStream.Dispose() }",
        "if ((Get-FileChecksum $preparedPath) -cne $ExpectedChecksum.ToLowerInvariant())",
        "throw 'Prepared agent binary does not match the verified download.'",
        "return $preparedPath",
    ):
        require(fragment, function)
    create = require("$preparedStream = [System.IO.File]::Open(", function)
    own = require("$script:TempFiles += $preparedPath", function)
    copy = require("$sourceStream.CopyTo($preparedStream)", function)
    close = require("$preparedStream.Dispose()", function)
    verify = require("Get-FileChecksum $preparedPath", function)
    publish = require("return $preparedPath", function)
    if not create < own < copy < close < verify < publish:
        raise ValueError("stage ownership, copy, disposal, checksum and return are out of order")
    if function.count("} finally {") != 2:
        raise ValueError("both stream handles need independent finally disposal")
    if any(operation in function for operation in ("Move-Item", "Copy-Item", "Remove-PulseService", "$DestPath")):
        raise ValueError("preparation can replace the destination or mutate a service")

    caller = script[require("# --- Binary Verification ---"):]
    checksum = require("$localChecksum = Get-FileChecksum $TempPath", caller)
    signature = require("Invoke-InstallerSignatureVerification -FilePath $TempPath", caller)
    prepare = require("# --- Prepare Verified Binary ---", caller)
    stop = require("# Confirm existing service stopped and absent", caller)
    swap = require("# Move temp file to final location", caller)
    service = require("# --- Service Installation ---", caller)
    if not checksum < signature < prepare < stop < swap < service:
        raise ValueError("preparation must follow trust checks and precede service disruption")
    boundary = caller[prepare:stop]
    prepared = require("$PreparedBinaryPath = New-PulsePreparedBinary -SourcePath $TempPath -DestinationDirectory $InstallDir -ExpectedChecksum $localChecksum", boundary)
    released = require("Remove-Item -LiteralPath $TempPath -Force -ErrorAction Stop", boundary)
    if released <= prepared:
        raise ValueError("original download released before successful preparation")
    if not re.search(r"\} catch \{\s+Cleanup\s+Show-Error .*?\s+Exit 1\s+\}", boundary):
        raise ValueError("preparation failure can fall through to service removal")
    publication = caller[swap:service]
    require("Move-Item $PreparedBinaryPath $DestPath -Force", publication)
    if "Move-Item $TempPath" in publication:
        raise ValueError("publication can still copy from the download volume")
    require("Write-RuntimeTokenFile", caller[service:])
    require("Save-ConnectionState", caller[service:])


class WindowsBinaryPreparation(unittest.TestCase):
    def setUp(self) -> None:
        self.script = INSTALLER.read_text()

    def test_actual_installer_wiring(self) -> None:
        preparation_contract(self.script)

    def test_rejects_missing_safety_boundaries(self) -> None:
        for fragment in (
            "[ValidatePattern('^[0-9a-fA-F]{64}$')]",
            "[System.IO.FileMode]::CreateNew",
            "$script:TempFiles += $preparedPath",
            "$sourceStream.CopyTo($preparedStream)",
            "$preparedStream.Dispose()",
            "$sourceStream.Dispose()",
            "Get-FileChecksum $preparedPath",
            "-ExpectedChecksum $localChecksum",
            "Remove-Item -LiteralPath $TempPath -Force -ErrorAction Stop",
            "Move-Item $PreparedBinaryPath $DestPath -Force",
        ):
            with self.subTest(boundary=fragment), self.assertRaises(ValueError):
                preparation_contract(self.script.replace(fragment, "", 1))

    def test_rejects_preparation_after_service_removal(self) -> None:
        start = self.script.index("# --- Prepare Verified Binary ---")
        stop = self.script.index("# Confirm existing service stopped and absent")
        swap = self.script.index("# Move temp file to final location")
        changed = self.script[:start] + self.script[stop:swap] + self.script[start:stop] + self.script[swap:]
        with self.assertRaisesRegex(ValueError, "precede service disruption"):
            preparation_contract(changed)

    def test_rejects_failure_fallthrough(self) -> None:
        start = self.script.index("# --- Prepare Verified Binary ---")
        end = self.script.index("# Confirm existing service stopped and absent")
        changed = self.script[:start] + self.script[start:end].replace("Exit 1", "return") + self.script[end:]
        with self.assertRaisesRegex(ValueError, "fall through"):
            preparation_contract(changed)

    def test_native_runtime_fixture_is_on_the_existing_ci_path(self) -> None:
        go = (ROOT / "scripts/installtests/install_ps1_test.go").read_text()
        workflow = (ROOT / ".github/workflows/unified-agent-native.yml").read_text()
        self.assertIn("func TestInstallPS1PreparedBinaryRuntime(t *testing.T)", go)
        self.assertIn("nativeInstallerPowerShell(t)", go)
        self.assertIn("windows_binary_preparation_controls.ps1", go)
        self.assertIn("-run '^Test(InstallPS1|WindowsAgentLifecycle|UninstallProof|PowerShellParser)'", workflow)
        fixture = (ROOT / "scripts/installtests/windows_binary_preparation_controls.ps1").read_text()
        self.assertIn("Invoke-Expression $functions[0].Extent.Text", fixture)
        self.assertIn("Invoke-Expression $replacement", fixture)
        for case in ("installed-success", "fresh-success", "missing-source", "locked-source", "source-cleanup-locked", "changed-source",
                     "invalid-checksum", "destination-denied", "stop-refused", "publication-locked"):
            self.assertIn("'" + case + "'", fixture)
        for observation in ("File-Hash $dest", "service-removed", "$before[$path]", "GetSecurityDescriptorSddlForm('Access')",
                            "$one -cne $two", "WaitForExit(15000)", "$child.Dispose()"):
            self.assertIn(observation, fixture)

    @unittest.skipUnless(os.environ.get("PULSE_WINDOWS_PREPARATION_PARENT"), "optional exact-parent wiring witness")
    def test_exact_parent_has_the_reported_gap(self) -> None:
        parent = Path(os.environ["PULSE_WINDOWS_PREPARATION_PARENT"]).read_text()
        self.assertIn("Move-Item $TempPath $DestPath -Force", parent)
        with self.assertRaises(ValueError):
            preparation_contract(parent)


if __name__ == "__main__":
    unittest.main(verbosity=2)
