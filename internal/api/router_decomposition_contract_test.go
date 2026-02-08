package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestRouterDecompositionSetupRoutes(t *testing.T) {
	fset, fileAST := parseAPISourceFile(t, "router.go")
	setup := findMethodDecl(t, fileAST, "Router", "setupRoutes")

	directRegs := collectRouteRegistrations(setup.Body)
	const maxDirectRegistrations = 3
	if len(directRegs) > maxDirectRegistrations {
		t.Fatalf(
			"setupRoutes appears re-centralized: found %d direct route registrations (max %d). registrations: %s",
			len(directRegs),
			maxDirectRegistrations,
			strings.Join(directRegs, ", "),
		)
	}

	delegates := collectRouteDelegationCalls(setup.Body)
	const minRouteDelegations = 4
	if len(delegates) < minRouteDelegations {
		t.Fatalf(
			"setupRoutes should delegate route wiring to register*Routes helpers: found %d delegations (min %d). found: %s",
			len(delegates),
			minRouteDelegations,
			strings.Join(delegates, ", "),
		)
	}

	_ = fset
}

func TestRouterDecompositionRouteRegistrationDistribution(t *testing.T) {
	minByFile := map[string]int{
		"router_routes_auth_security.go": 3,
		"router_routes_monitoring.go":    3,
		"router_routes_ai_relay.go":      3,
		"router_routes_org_license.go":   3,
	}

	for fileName, minCount := range minByFile {
		_, fileAST := parseAPISourceFile(t, fileName)
		count := countRouteRegistrations(fileAST)
		if count < minCount {
			t.Fatalf(
				"%s has too few route registrations (%d < %d). this suggests route ownership drift or re-centralization",
				fileName,
				count,
				minCount,
			)
		}
	}
}

func TestConfigHandlersDecompositionDelegationBoundaries(t *testing.T) {
	fset, fileAST := parseAPISourceFile(t, "config_handlers.go")

	methods := exportedHandleMethods(fileAST, "ConfigHandlers")
	if len(methods) == 0 {
		t.Fatal("no exported ConfigHandlers Handle* methods found in config_handlers.go")
	}

	const maxMethodLines = 22
	const maxTopLevelStatements = 6

	for _, method := range methods {
		lines := nodeLineSpan(fset, method)
		if lines > maxMethodLines {
			t.Errorf(
				"%s is too large (%d lines > %d). exported Handle* methods in config_handlers.go should remain delegation-focused",
				method.Name.Name,
				lines,
				maxMethodLines,
			)
		}

		if method.Body == nil {
			t.Errorf("%s has no body", method.Name.Name)
			continue
		}

		if len(method.Body.List) > maxTopLevelStatements {
			t.Errorf(
				"%s has too many top-level statements (%d > %d). keep exported Handle* methods thin",
				method.Name.Name,
				len(method.Body.List),
				maxTopLevelStatements,
			)
		}

		if hasHeavyControlFlow(method.Body) {
			t.Errorf(
				"%s contains control-flow-heavy logic. move implementation into internal helpers and keep Handle* as delegators",
				method.Name.Name,
			)
		}
	}
}

func TestRouteInventoryContractCoversAllRouteModules(t *testing.T) {
	apiDir := apiDirPath(t)
	routeModules := discoverRouteModulesWithRegistrations(t, apiDir)
	if len(routeModules) == 0 {
		t.Fatal("no route module files with registrations discovered")
	}

	_, inventoryAST := parseAPISourceFile(t, "route_inventory_test.go")
	mentioned := collectStringLiterals(inventoryAST)

	var missing []string
	for _, module := range routeModules {
		if _, ok := mentioned[module]; !ok {
			missing = append(missing, module)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf(
			"route_inventory_test.go does not reference route module files: %s. update parseRouterRoutes file coverage and allowlists together",
			strings.Join(missing, ", "),
		)
	}
}

func parseAPISourceFile(t *testing.T, name string) (*token.FileSet, *ast.File) {
	t.Helper()
	filePath := filepath.Join(apiDirPath(t), name)
	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return fset, fileAST
}

func apiDirPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	return filepath.Dir(thisFile)
}

func findMethodDecl(t *testing.T, fileAST *ast.File, recvType, methodName string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range fileAST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil || fn.Name.Name != methodName || len(fn.Recv.List) != 1 {
			continue
		}
		if receiverName(fn.Recv.List[0].Type) == recvType {
			return fn
		}
	}
	t.Fatalf("method %s.%s not found", recvType, methodName)
	return nil
}

func receiverName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return receiverName(v.X)
	default:
		return ""
	}
}

func collectRouteRegistrations(node ast.Node) []string {
	seen := map[string]struct{}{}
	var out []string
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isMuxRouteRegistrationCall(call) {
			return true
		}
		site := registrationSite(call)
		if _, exists := seen[site]; exists {
			return true
		}
		seen[site] = struct{}{}
		out = append(out, site)
		return true
	})
	sort.Strings(out)
	return out
}

func registrationSite(call *ast.CallExpr) string {
	if len(call.Args) > 0 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil {
				return s
			}
		}
	}
	return "dynamic-route"
}

func collectRouteDelegationCalls(body *ast.BlockStmt) []string {
	if body == nil {
		return nil
	}
	delegations := map[string]struct{}{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "r" {
			return true
		}
		name := sel.Sel.Name
		if strings.HasPrefix(name, "register") && strings.Contains(name, "Routes") {
			delegations[name] = struct{}{}
		}
		return true
	})

	out := make([]string, 0, len(delegations))
	for name := range delegations {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func countRouteRegistrations(fileAST *ast.File) int {
	total := 0
	ast.Inspect(fileAST, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && isMuxRouteRegistrationCall(call) {
			total++
		}
		return true
	})
	return total
}

func isMuxRouteRegistrationCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil {
		return false
	}
	if sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc" {
		return false
	}
	if len(call.Args) < 2 {
		return false
	}

	receiver := selectorChain(sel.X)
	return receiver == "mux" || strings.HasSuffix(receiver, ".mux")
}

func selectorChain(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		base := selectorChain(v.X)
		if base == "" {
			return v.Sel.Name
		}
		return base + "." + v.Sel.Name
	default:
		return ""
	}
}

func exportedHandleMethods(fileAST *ast.File, recvType string) []*ast.FuncDecl {
	var methods []*ast.FuncDecl
	for _, decl := range fileAST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		if !ast.IsExported(fn.Name.Name) || !strings.HasPrefix(fn.Name.Name, "Handle") {
			continue
		}
		if receiverName(fn.Recv.List[0].Type) != recvType {
			continue
		}
		methods = append(methods, fn)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name.Name < methods[j].Name.Name
	})
	return methods
}

func nodeLineSpan(fset *token.FileSet, node ast.Node) int {
	start := fset.Position(node.Pos()).Line
	end := fset.Position(node.End()).Line
	if end < start {
		return 0
	}
	return end - start + 1
}

func hasHeavyControlFlow(node ast.Node) bool {
	heavy := false
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.GoStmt:
			heavy = true
			return false
		}
		return true
	})
	return heavy
}

func discoverRouteModulesWithRegistrations(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read api directory: %v", err)
	}

	var modules []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "router_routes") || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		_, fileAST := parseAPISourceFile(t, name)
		if countRouteRegistrations(fileAST) > 0 {
			modules = append(modules, name)
		}
	}

	sort.Strings(modules)
	return modules
}

func collectStringLiterals(fileAST *ast.File) map[string]struct{} {
	values := map[string]struct{}{}
	ast.Inspect(fileAST, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		values[value] = struct{}{}
		return true
	})
	return values
}
