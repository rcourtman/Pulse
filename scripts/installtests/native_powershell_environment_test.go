package installtests

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestInstallPS1NativePowerShellEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, []string{}},
		{"native inputs", []string{"SystemRoot=C:\\Windows", "PATH=native-tools", "PULSE_FIXTURE=a=b"}, []string{"SystemRoot=C:\\Windows", "PATH=native-tools", "PULSE_FIXTURE=a=b"}},
		{"foreign modules", []string{"PSModulePath=PowerShell7", "PULSE_FIXTURE=kept"}, []string{"PULSE_FIXTURE=kept"}},
		{"case-insensitive duplicates", []string{"psmodulepath=one", "PSMODULEPATH=two", "PsModulePath="}, []string{}},
		{"similar names and values", []string{"PSModulePathExtra=kept", "X=PSModulePath=kept", "=C:=C:\\work"}, []string{"PSModulePathExtra=kept", "X=PSModulePath=kept", "=C:=C:\\work"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]string(nil), tc.in...)
			got := nativeInstallerEnvironment(tc.in)
			if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(tc.in, before) {
				t.Fatal("native environment dropped another input, retained a foreign module path or mutated its caller")
			}
			if len(got) > 0 {
				got[0] = "changed copy"
				if !reflect.DeepEqual(tc.in, before) {
					t.Fatal("native environment aliases its caller's storage")
				}
			}
		})
	}
}

// This portable subprocess checks the command builder, not a Windows engine.
// Actual module loading and real-file/ACL acceptance stay in the native suite.
func TestInstallPS1NativePowerShellCommandEnvironment(t *testing.T) {
	const control = "PULSE_TEST_NATIVE_COMMAND_ENV"
	const fixture = "literal ' $() = fixture input"
	if os.Getenv(control) == fixture {
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if strings.EqualFold(key, "PSModulePath") {
				t.Fatal("foreign module path reached the child")
			}
		}
		return
	}
	t.Setenv("PSModulePath", "parent-engine-only")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-test.run=^TestInstallPS1NativePowerShellCommandEnvironment$", "-test.count=1", "-test.v"}
	cmd := nativeInstallerCommand(context.Background(), executable, args...)
	cmd.Env = append(cmd.Environ(), control+"="+fixture)
	if !reflect.DeepEqual(cmd.Args[1:], args) {
		t.Fatal("native launcher changed its arguments")
	}
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "--- PASS: TestInstallPS1NativePowerShellCommandEnvironment") {
		t.Fatalf("filtered command did not preserve its literal fixture input: %v\n%s", err, output)
	}
	if os.Getenv("PSModulePath") != "parent-engine-only" {
		t.Fatal("child environment preparation changed the parent")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := nativeInstallerCommand(ctx, executable, args...).Run(); !errors.Is(err, context.Canceled) {
		t.Fatalf("native launcher lost command cancellation: %v", err)
	}
}
