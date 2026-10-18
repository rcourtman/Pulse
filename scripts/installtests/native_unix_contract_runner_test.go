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
// propagation, not a claimed reproduction of macOS timing.
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
inventory = "\n".join(names + reset + ["BenchmarkIgnored", "ok package 0.1s"])
groups = runner.partition(inventory)
assert tuple(runner.GROUPS) == ("general-0", "general-1", "reset")
assert [len(group) for group in groups] == [250, 250, 2]
assert sorted(sum(groups, [])) == sorted(names + reset)
assert len(set(sum(groups, []))) == 502
assert groups[2] == sorted(reset)
assert groups == runner.partition("\n".join(reversed(names + reset)))
for unsafe in ["", "TestOnly", "TestOne\nTestTwo", inventory + "\nTestFixture000",
               inventory + "\nTestUnsafe|TestInjected", inventory + "\nFuzzBad.*"]:
    try:
        runner.partition(unsafe)
    except ValueError:
        pass
    else:
        raise AssertionError("unsafe or incomplete inventory admitted")

def execute(failure_at=None, output=inventory):
    calls = []
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
        assert argv[6] == "^(" + "|".join(groups[len(calls) - 2]) + ")$"
        assert "-parallel" not in argv
        return SimpleNamespace(returncode=0)
    with patch.object(runner.subprocess, "run", side_effect=run):
        try:
            runner.main()
        except subprocess.CalledProcessError as error:
            assert failure_at == len(calls) and error.returncode == 23
        except ValueError:
            assert output != inventory and len(calls) == 1
        else:
            assert failure_at is None and output == inventory
    return calls

calls = execute()
assert len(calls) == 4
patterns = [re.compile(call[6]) for call in calls[1:]]
for name in names + reset:
    assert sum(bool(pattern.fullmatch(name)) for pattern in patterns) == 1
for name in ["TestFixture000/child", "TestInjected", "BenchmarkIgnored", "TestFixture000Extra"]:
    assert not any(pattern.fullmatch(name) for pattern in patterns)
for failure_at in range(1, 5):
    assert len(execute(failure_at=failure_at)) == failure_at
assert len(execute(output=inventory + "\nTestFixture000")) == 1
`, repoFile("scripts", "installtests", "run_unix_contracts.py"))
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("native installer runner control deadline: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("native installer runner control: %v\n%s", err, output)
	}
}
