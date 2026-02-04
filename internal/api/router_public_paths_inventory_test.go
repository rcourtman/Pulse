package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestRouterPublicPathsInventory(t *testing.T) {
	literalPaths, dynamicPaths := parsePublicPaths(t)

	expectedLiterals := sliceToSet(t, publicPathsAllowlist, "public paths allowlist")
	expectedDynamics := sliceToSet(t, publicPathsDynamicAllowlist, "public dynamic allowlist")
	expectedPublic := sliceToSet(t, publicRouteAllowlist, "public route allowlist")

	actualLiterals := sliceToSet(t, literalPaths, "router public paths")
	actualDynamics := sliceToSet(t, dynamicPaths, "router public dynamic paths")

	if missing := setDifference(actualLiterals, expectedLiterals); len(missing) > 0 {
		t.Fatalf("public paths missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedLiterals, actualLiterals); len(stale) > 0 {
		t.Fatalf("public allowlist contains paths not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
	if missing := setDifference(actualDynamics, expectedDynamics); len(missing) > 0 {
		t.Fatalf("public dynamic paths missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedDynamics, actualDynamics); len(stale) > 0 {
		t.Fatalf("public dynamic allowlist contains paths not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}
	if missing := setDifference(expectedLiterals, expectedPublic); len(missing) > 0 {
		t.Fatalf("publicPaths entries missing from public route allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	supportedPublic := sliceToSet(t, append(publicPathsAllowlist, authBypassAllowlist...), "supported public routes")
	if stale := setDifference(expectedPublic, supportedPublic); len(stale) > 0 {
		t.Fatalf("public route allowlist contains paths not backed by bypass logic: %s", strings.Join(sortedKeys(stale), ", "))
	}
}

func parsePublicPaths(t *testing.T) ([]string, []string) {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to locate test file path")
	}
	routerPath := filepath.Join(filepath.Dir(file), "router.go")

	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, routerPath, nil, 0)
	if err != nil {
		t.Fatalf("parse router.go: %v", err)
	}

	var literalPaths []string
	var dynamicPaths []string
	var found bool

	ast.Inspect(fileAST, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || ident.Name != "publicPaths" {
			return true
		}
		comp, ok := assign.Rhs[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		found = true
		for _, elt := range comp.Elts {
			switch v := elt.(type) {
			case *ast.BasicLit:
				if v.Kind != token.STRING {
					continue
				}
				unquoted, err := strconv.Unquote(v.Value)
				if err != nil {
					unquoted = strings.Trim(v.Value, "`\"'")
				}
				literalPaths = append(literalPaths, unquoted)
			case *ast.Ident:
				dynamicPaths = append(dynamicPaths, v.Name)
			case *ast.SelectorExpr:
				dynamicPaths = append(dynamicPaths, selectorName(v))
			}
		}
		return false
	})

	if !found {
		t.Fatalf("publicPaths slice not found in router.go")
	}

	return literalPaths, dynamicPaths
}

var publicPathsDynamicAllowlist = []string{
	"config.DefaultOIDCCallbackPath",
}

var publicPathsAllowlist = []string{
	"/api/health",
	"/api/security/status",
	"/api/security/validate-bootstrap-token",
	"/api/security/quick-setup",
	"/api/version",
	"/api/login",
	"/api/oidc/login",
	"/install-docker-agent.sh",
	"/install-container-agent.sh",
	"/download/pulse-docker-agent",
	"/install-host-agent.sh",
	"/install-host-agent.ps1",
	"/uninstall-host-agent.sh",
	"/uninstall-host-agent.ps1",
	"/download/pulse-host-agent",
	"/install.sh",
	"/install.ps1",
	"/download/pulse-agent",
	"/api/agent/version",
	"/api/agent/ws",
	"/api/server/info",
	"/api/install/install-docker.sh",
	"/api/ai/oauth/callback",
}
