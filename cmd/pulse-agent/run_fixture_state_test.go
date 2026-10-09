package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Startup secures (and can create) the service's real state directory even
// with mocked modules or a cancelled context. Runtime unit fixtures must own
// their state; otherwise native Windows tests pre-seed ProgramData\Pulse and
// correctly fail the later clean-runner service-lifecycle admission.
func unsafeAgentRunFixtures(source []byte) ([]int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture_test.go", source, 0)
	if err != nil {
		return nil, err
	}
	var unsafe []int
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		// An identifier is owned only when all its assignments derive from a
		// test temporary directory. Reject fixed paths and unknown provenance.
		values := make(map[string][]ast.Expr)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if assign, ok := node.(*ast.AssignStmt); ok && len(assign.Lhs) == len(assign.Rhs) {
				for i, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						values[id.Name] = append(values[id.Name], assign.Rhs[i])
					}
				}
			}
			return true
		})
		var temporary func(ast.Expr, map[string]bool) bool
		temporary = func(expr ast.Expr, visiting map[string]bool) bool {
			switch expr := expr.(type) {
			case *ast.CallExpr:
				if selector, ok := expr.Fun.(*ast.SelectorExpr); ok {
					if receiver, ok := selector.X.(*ast.Ident); ok {
						if receiver.Name == "t" && selector.Sel.Name == "TempDir" && len(expr.Args) == 0 {
							return true
						}
						if receiver.Name == "filepath" && selector.Sel.Name == "Join" && len(expr.Args) > 0 {
							for _, suffix := range expr.Args[1:] {
								text, ok := suffix.(*ast.BasicLit)
								if !ok || text.Kind != token.STRING {
									return false
								}
								path, err := strconv.Unquote(text.Value)
								if err != nil || strings.ContainsAny(path, `/\:`) || path == ".." {
									return false
								}
							}
							return temporary(expr.Args[0], visiting)
						}
					}
				}
			case *ast.Ident:
				if visiting[expr.Name] || len(values[expr.Name]) == 0 {
					return false
				}
				visiting[expr.Name] = true
				defer delete(visiting, expr.Name)
				for _, value := range values[expr.Name] {
					if !temporary(value, visiting) {
						return false
					}
				}
				return true
			}
			return false
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok || name.Name != "run" {
				return true
			}
			owned := false
			stateFlags := 0
			if len(call.Args) == 3 {
				if args, ok := call.Args[1].(*ast.CompositeLit); ok {
					for i, arg := range args.Elts {
						if text, ok := arg.(*ast.BasicLit); ok && text.Kind == token.STRING {
							flag, _ := strconv.Unquote(text.Value)
							if flag == "-state-dir" || flag == "--state-dir" {
								stateFlags++
								owned = i+1 < len(args.Elts) && temporary(args.Elts[i+1], make(map[string]bool))
							}
						}
					}
				}
			}
			if !owned || stateFlags != 1 {
				unsafe = append(unsafe, fset.Position(call.Pos()).Line)
			}
			return true
		})
	}
	return unsafe, nil
}

func TestAgentRunFixturesOwnTheirState(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("discover agent test fixtures: %v", err)
	}
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		unsafe, err := unsafeAgentRunFixtures(source)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		if len(unsafe) != 0 {
			t.Errorf("%s run fixtures need explicit temporary state at lines %v", file, unsafe)
		}
	}
}

func TestAgentRunFixtureStateGuard(t *testing.T) {
	for _, tc := range []struct {
		name, setup, args string
		owned             bool
	}{
		{"default", "", `"-token", "fixture"`, false},
		{"empty", "", `"-state-dir", ""`, false},
		{"service path", "", `"-state-dir", "/var/lib/pulse-agent"`, false},
		{"fixed variable", `stateDir := "/service-state"`, `"-state-dir", stateDir`, false},
		{"unknown variable", "", `"-state-dir", stateDir`, false},
		{"reassigned", `stateDir := t.TempDir(); stateDir = "/service-state"`, `"-state-dir", stateDir`, false},
		{"duplicate flag", "", `"-state-dir", t.TempDir(), "-state-dir", t.TempDir()`, false},
		{"parent traversal", "", `"-state-dir", filepath.Join(t.TempDir(), "..", "service-state")`, false},
		{"absolute suffix", "", `"-state-dir", filepath.Join(t.TempDir(), "/service-state")`, false},
		{"temporary", "", `"-state-dir", t.TempDir()`, true},
		{"long flag", "", `"--state-dir", t.TempDir()`, true},
		{"temporary variable", `stateDir := t.TempDir()`, `"-state-dir", stateDir`, true},
		{"custom temporary path", `root := t.TempDir(); stateDir := filepath.Join(root, "share", ".pulse-agent")`, `"-state-dir", stateDir`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "package main\nfunc fixture() { " + tc.setup + "; run(ctx, []string{" + tc.args + "}, getenv) }"
			unsafe, err := unsafeAgentRunFixtures([]byte(source))
			if err != nil {
				t.Fatal(err)
			}
			if (len(unsafe) == 0) != tc.owned {
				t.Fatalf("unsafe lines = %v, owned = %v", unsafe, tc.owned)
			}
		})
	}
	if path := os.Getenv("PULSE_AGENT_RUN_FIXTURE_PARENT"); path != "" {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		unsafe, err := unsafeAgentRunFixtures(source)
		if err != nil || len(unsafe) != 15 {
			t.Fatalf("original parent: unsafe run lines = %v, error = %v; want 14 default paths and one fixed NAS path", unsafe, err)
		}
		t.Logf("original parent rejected at all 15 unowned run calls: %v", unsafe)
	}
}
