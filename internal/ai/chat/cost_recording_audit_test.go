package chat

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

// TestChatCostRecordingCoverage is the chat-package counterpart to
// the audit in internal/ai/. It walks internal/ai/chat/*.go (excluding
// _test.go and sub-packages) and asserts every function calling
// .Chat() or .ChatStream() on a providers.Provider/StreamingProvider
// either records cost (.Record() on a cost store) within the same
// body, or carries an explicit //cost-recording-exempt: <reason> doc
// comment.
//
// The chat package's orchestrator/loop split means most Chat callers
// here are loop methods, and recording lives in the orchestrator
// (chat.Service.recordChatTurnCost) that owns the loop. Those
// callers carry the exemption marker pointing at the recording site.
// Without this audit, a new ChatStream caller added to the loop or
// helper layer could silently bypass cost recording — the exact bug
// class this guards against in the parent package's audit
// (TestCostRecordingCoverage in internal/ai/).
func TestChatCostRecordingCoverage(t *testing.T) {
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
			if hasChatExemptionComment(fn.Doc, exemptMarker) {
				return true
			}
			if !chatBodyCallsAny(fn.Body, "Chat", "ChatStream") {
				return true
			}
			if chatBodyCallsMethod(fn.Body, "Record") {
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
		t.Fatalf("found %d function(s) calling provider.Chat or provider.ChatStream without recording cost.\n"+
			"Each call site must record a cost.UsageEvent or carry a //%s: <reason> doc comment.\n"+
			"Offenders:\n%s",
			len(offenders), exemptMarker, strings.Join(lines, "\n"))
	}
}

func chatBodyCallsAny(body *ast.BlockStmt, methods ...string) bool {
	for _, m := range methods {
		if chatBodyCallsMethod(body, m) {
			return true
		}
	}
	return false
}

func chatBodyCallsMethod(body *ast.BlockStmt, method string) bool {
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

func hasChatExemptionComment(doc *ast.CommentGroup, marker string) bool {
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
