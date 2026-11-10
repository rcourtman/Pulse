package installtests

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestInstallPS1NativePowerShellAdmission(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "freebsd", "windows", "unknown"} {
		t.Run(goos, func(t *testing.T) {
			calls := 0
			lookupError := errors.New("controlled missing Windows engine")
			for _, available := range []bool{true, false} {
				path, err := findNativeInstallerPowerShell(goos, func(name string) (string, error) {
					calls++
					if name != "powershell.exe" {
						t.Fatalf("selected a substitute engine: %q", name)
					}
					if !available {
						return "", lookupError
					}
					return "controlled-windows-engine", nil
				})
				if goos != "windows" {
					if calls != 0 || path != "" || !errors.Is(err, errInstallerPowerShellPlatform) {
						t.Fatalf("non-Windows contract tried an ambient engine: path=%q err=%v calls=%d", path, err, calls)
					}
				} else if available {
					if calls != 1 || path != "controlled-windows-engine" || err != nil {
						t.Fatalf("Windows lost its native engine: path=%q err=%v calls=%d", path, err, calls)
					}
				} else if calls != 2 || path != "" || !errors.Is(err, lookupError) || errors.Is(err, errInstallerPowerShellPlatform) {
					t.Fatalf("missing Windows engine was skipped or retried: path=%q err=%v calls=%d", path, err, calls)
				}
			}
		})
	}
}

const nativeWindowsInstallerCommand = "go test -count=1 -v ./scripts/installtests ./scripts/installtests/windowslifecycleserver -run '^Test(InstallPS1|WindowsAgentLifecycle|UninstallProof|PowerShellParser)'"

func nativeWindowsInstallerProof(steps []nativeWindowsExitStep) error {
	count := 0
	for _, step := range steps {
		if step.Name != "Test native Windows installer contracts" {
			continue
		}
		count++
		if step.If != "${{ !matrix.unix }}" || step.ContinueOnError || strings.TrimSpace(step.Run) != nativeWindowsInstallerCommand {
			return fmt.Errorf("Windows installer proof must run every required family uncached, verbose and fail-closed")
		}
	}
	if count != 1 {
		return fmt.Errorf("expected one required Windows installer proof, got %d", count)
	}
	return nil
}

func TestInstallPS1NativeRuntimeCallersHaveWindowsCoverage(t *testing.T) {
	steps := nativeWindowsExitSteps(t)
	if err := nativeWindowsInstallerProof(steps); err != nil {
		t.Fatal(err)
	}
	// Discover actual callers instead of keeping a manual list that could omit
	// a new runtime control from the only platform on which it now executes.
	files, err := filepath.Glob(repoFile("scripts", "installtests", "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	selected := regexp.MustCompile(`^Test(InstallPS1|WindowsAgentLifecycle|UninstallProof|PowerShellParser)`)
	callers := 0
	for _, path := range files {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := call.Fun.(*ast.Ident)
				if ok && name.Name == "nativeInstallerPowerShell" {
					callers++
					if !selected.MatchString(function.Name.Name) {
						t.Errorf("runtime caller %s is absent from native Windows proof", function.Name.Name)
					}
				}
				return true
			})
		}
	}
	if callers < 7 {
		t.Fatalf("lost installer/lifecycle runtime controls: found %d callers, want at least seven", callers)
	}
	t.Logf("all %d installer/lifecycle runtime callers have required native Windows coverage", callers)
	for _, mutation := range []string{"condition", "tolerate", "cache", "quiet", "omit-parsers", "omit-peers", "missing", "duplicate"} {
		t.Run(mutation, func(t *testing.T) {
			changed := append([]nativeWindowsExitStep(nil), steps...)
			for i := range changed {
				if changed[i].Name != "Test native Windows installer contracts" {
					continue
				}
				switch mutation {
				case "condition":
					changed[i].If = "false"
				case "tolerate":
					changed[i].ContinueOnError = true
				case "cache":
					changed[i].Run = strings.Replace(changed[i].Run, "-count=1 ", "", 1)
				case "quiet":
					changed[i].Run = strings.Replace(changed[i].Run, "-v ", "", 1)
				case "omit-parsers":
					changed[i].Run = strings.Replace(changed[i].Run, "|PowerShellParser", "", 1)
				case "omit-peers":
					changed[i].Run = strings.Replace(changed[i].Run, "|UninstallProof", "", 1)
				case "missing":
					changed = append(changed[:i], changed[i+1:]...)
				case "duplicate":
					changed = append(changed, changed[i])
				}
				break
			}
			if err := nativeWindowsInstallerProof(changed); err == nil {
				t.Fatal("accepted missing or weakened required Windows execution")
			}
		})
	}
}
