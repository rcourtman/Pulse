package ai

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCostRecordingCoverage is an AST-level audit: every function in
// internal/ai that calls .Chat() on a providers.Provider value must
// also reference cost recording (.Record() on a cost store) within
// the same function body, or be explicitly exempted with a doc
// comment marker. This guards against the bug class we shipped twice
// in the report-narrative path: a Chat call that consumes provider
// tokens but doesn't surface in the operator's cost ledger.
//
// Exemption marker: place "//cost-recording-exempt: <reason>" in the
// function's doc comment. Use sparingly — the only legitimate case so
// far is patrol preflight (a connectivity test, not user workload).
//
// The scan is intentionally local (function-scoped, not interprocedural).
// A passthrough wrapper that calls a recording function instead of
// calling Record itself would need an exemption with the wrapped
// callee named in the reason. Keeping it local makes false positives
// loud rather than letting silent gaps creep in via indirection.
func TestCostRecordingCoverage(t *testing.T) {
	const exemptMarker = "cost-recording-exempt"

	type offender struct {
		file     string
		funcName string
		line     int
	}
	var offenders []offender

	fset := token.NewFileSet()

	walkErr := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Stay in the canonical package directory; the providers
			// package implements Chat itself and is out of scope, and
			// every other subdirectory is a separate package with its
			// own coverage test if needed.
			if path == "." {
				return nil
			}
			return fs.SkipDir
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		file, parseErr := parser.ParseFile(fset, path, src, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}

		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			if hasExemptionComment(fn.Doc, exemptMarker) {
				return true
			}
			if !bodyCallsMethod(fn.Body, "Chat") {
				return true
			}
			if bodyCallsMethod(fn.Body, "Record") {
				return true
			}
			pos := fset.Position(fn.Pos())
			offenders = append(offenders, offender{
				file:     path,
				funcName: fn.Name.Name,
				line:     pos.Line,
			})
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}

	if len(offenders) > 0 {
		var lines []string
		for _, o := range offenders {
			lines = append(lines, "  "+o.file+":"+itoa(o.line)+" "+o.funcName)
		}
		t.Fatalf("found %d function(s) calling provider.Chat without recording cost.\n"+
			"Each call site must record a cost.UsageEvent or carry a //%s: <reason> doc comment.\n"+
			"Offenders:\n%s",
			len(offenders), exemptMarker, strings.Join(lines, "\n"))
	}
}

func bodyCallsMethod(body *ast.BlockStmt, method string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel != nil && sel.Sel.Name == method {
			found = true
			return false
		}
		return true
	})
	return found
}

func hasExemptionComment(doc *ast.CommentGroup, marker string) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		if strings.Contains(c.Text, marker) {
			return true
		}
	}
	return false
}

// itoa avoids strconv just to keep this audit self-contained.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
