//go:build !windows

package installtests

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// Exercise the actual Python runner without recursively running the Go suite.
// The hosted failure had 500 checks under one package alarm; this control uses
// that inventory size and verifies admission, exact-once execution and failure
// propagation across both workflow phases, not a macOS timing reproduction.
func TestNativeUnixContractRunnerPartitionsWithoutDroppingChecks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-c", `
import importlib.util
import re
import subprocess
import sys
from types import SimpleNamespace
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("native_runner", sys.argv[1])
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
names = [f"TestFixture{i:03d}" for i in range(497)]
names += ["ExampleInstaller", "FuzzInstaller", "TestFutureFixture"]
reset = ["TestRootInstallResetSafety", "TestRootInstallResetPersistence"]
preflight = ["TestInstallSHNASBootOwnersQuiesceBeforeRecoveryErasure"]
inventory = "\n".join(names + reset + preflight + ["BenchmarkIgnored", "ok package 0.1s"])
groups = runner.partition(inventory)
assert tuple(runner.GROUPS) == ("nas-preflight", "general-0", "general-1", "reset")
assert [len(group) for group in groups] == [1, 250, 250, 2]
assert sorted(sum(groups, [])) == sorted(names + reset + preflight)
assert len(set(sum(groups, []))) == 503
assert groups[0] == preflight and groups[3] == sorted(reset)
assert groups == runner.partition("\n".join(reversed(names + reset + preflight)))
for unsafe in ["", "TestOnly", "TestOne\nTestTwo", inventory + "\nTestFixture000",
               inventory + "\nTestUnsafe|TestInjected", inventory + "\nFuzzBad.*",
               "\n".join(names + reset)]:
    try:
        runner.partition(unsafe)
    except ValueError:
        pass
    else:
        raise AssertionError("unsafe or incomplete inventory admitted")

def execute(failure_at=None, output=inventory, phase="all"):
    calls = []
    selected = groups if phase == "all" else groups[:1] if phase == "preflight" else groups[1:]
    def run(argv, **kwargs):
        calls.append(argv)
        assert kwargs["cwd"] == runner.ROOT and kwargs["check"] is True
        if len(calls) == failure_at:
            raise subprocess.CalledProcessError(23, argv)
        if len(calls) == 1:
            assert argv == ["go", "test", "-list", ".", runner.PACKAGE]
            assert kwargs["capture_output"] is True and kwargs["text"] is True
            return SimpleNamespace(stdout=output)
        assert argv[:6] == ["go", "test", "-count=1", "-timeout", "10m", "-run"]
        assert argv[-1] == runner.PACKAGE
        assert argv[6] == "^(" + "|".join(selected[len(calls) - 2]) + ")$"
        assert "-parallel" not in argv
        return SimpleNamespace(returncode=0)
    with patch.object(runner.subprocess, "run", side_effect=run):
        try:
            runner.main(phase)
        except subprocess.CalledProcessError as error:
            assert failure_at == len(calls) and error.returncode == 23
        except ValueError:
            assert output != inventory and len(calls) == 1
        else:
            assert failure_at is None and output == inventory
    return calls

calls = execute()
assert len(calls) == 5
patterns = [re.compile(call[6]) for call in calls[1:]]
for name in names + reset + preflight:
    assert sum(bool(pattern.fullmatch(name)) for pattern in patterns) == 1
for name in ["TestFixture000/child", "TestInjected", "BenchmarkIgnored", "TestFixture000Extra"]:
    assert not any(pattern.fullmatch(name) for pattern in patterns)
for failure_at in range(1, 6):
    assert len(execute(failure_at=failure_at)) == failure_at
assert len(execute(output=inventory + "\nTestFixture000")) == 1
early = execute(phase="preflight")
remaining = execute(phase="remaining")
assert early[1:] + remaining[1:] == calls[1:]
for phase, count in [("preflight", 2), ("remaining", 4)]:
    for failure_at in range(1, count + 1):
        assert len(execute(failure_at=failure_at, phase=phase)) == failure_at
`, repoFile("scripts", "installtests", "run_unix_contracts.py"))
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("native installer runner control deadline: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("native installer runner control: %v\n%s", err, output)
	}
}
