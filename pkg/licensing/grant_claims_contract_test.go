package licensing

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestCloudClaimPlanVersionNormalizationContract(t *testing.T) {
	claims := Claims{
		Tier:        TierCloud,
		PlanVersion: "cloud_v1",
		Limits: map[string]int64{
			"max_agents": 999,
		},
	}

	if got := claims.EntitlementPlanVersion(); got != "cloud_starter" {
		t.Fatalf("EntitlementPlanVersion() = %q, want %q", got, "cloud_starter")
	}
	if got := claims.EffectiveLimits()["max_agents"]; got != 10 {
		t.Fatalf("EffectiveLimits()[max_agents] = %d, want %d", got, 10)
	}
}

// grantContractJSONTags lists the JSON field names that MUST exist with
// identical types in both pulse/pkg/licensing.GrantClaims and
// pulse-pro/relay-server.RelayGrantClaims.
//
// The license server issues grants with these fields, the pulse client parses
// them, and the relay server validates them. Any JSON tag drift means the
// receiving side silently drops the field value — a real bug (e.g. max_nodes
// vs max_agents would cause agent limit enforcement to fail on the relay).
//
// We compare by JSON tag (the wire contract) rather than Go field name because
// the two repos may use different Go naming conventions (e.g. JTI vs JWTID)
// while producing identical wire output.
//
// Fields that only exist in one struct (e.g. "sub"/"nbf" in relay-server only)
// are NOT listed here — only fields that BOTH sides must agree on.
var grantContractJSONTags = []string{
	"iss",
	"aud",
	"iat",
	"exp",
	"lid",
	"iid",
	"lv",
	"st",
	"tier",
	"plan",
	"feat",
	"max_agents",
	"max_guests",
	"grace_until",
	"email",
	"jti",
}

type grantField struct {
	GoName  string
	GoType  string
	JSONTag string // full json tag value including options like omitempty
}

// TestGrantClaimsContractMatchesRelayServer validates that the shared JSON
// field tags between GrantClaims (pulse client) and RelayGrantClaims (relay
// server) produce identical wire output. This catches drift like max_nodes
// vs max_agents that would cause silent field loss during JWT deserialization.
func TestGrantClaimsContractMatchesRelayServer(t *testing.T) {
	localPath, referencePath := grantContractPaths(t)

	localSchema := extractGrantSchemaByJSON(t, localPath, "GrantClaims")
	referenceSchema := extractGrantSchemaByJSON(t, referencePath, "RelayGrantClaims")

	for _, jsonTag := range grantContractJSONTags {
		localField, lok := localSchema[jsonTag]
		refField, rok := referenceSchema[jsonTag]

		if !lok && !rok {
			t.Errorf("contract JSON tag %q missing in BOTH local and reference", jsonTag)
			continue
		}
		if !lok {
			t.Errorf("contract JSON tag %q missing in local GrantClaims (%s)", jsonTag, localPath)
			continue
		}
		if !rok {
			t.Errorf("contract JSON tag %q missing in reference RelayGrantClaims (%s)", jsonTag, referencePath)
			continue
		}

		// Go type must match for correct serialization
		if localField.GoType != refField.GoType {
			t.Errorf(
				"JSON tag %q Go type drift: local %s (%s) vs reference %s (%s)",
				jsonTag, localField.GoName, localField.GoType, refField.GoName, refField.GoType,
			)
		}
	}
}

func grantContractPaths(t *testing.T) (string, string) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	localPath := filepath.Join(repoRoot, "pkg", "licensing", "activation_types.go")

	referenceFromEnv := strings.TrimSpace(os.Getenv("PULSE_GRANT_CLAIMS_REFERENCE"))
	if referenceFromEnv != "" {
		if _, err := os.Stat(referenceFromEnv); err != nil {
			t.Fatalf(
				"PULSE_GRANT_CLAIMS_REFERENCE is set but file is unavailable (%s): %v",
				referenceFromEnv, err,
			)
		}
		return localPath, referenceFromEnv
	}

	candidates := []string{
		filepath.Join(repoRoot, "..", "pulse-pro", "relay-server", "v6_grant.go"),
	}
	if reposDir := strings.TrimSpace(os.Getenv("PULSE_REPOS_DIR")); reposDir != "" {
		candidates = append(candidates, filepath.Join(reposDir, "pulse-pro", "relay-server", "v6_grant.go"))
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return localPath, candidate
		}
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_GRANT_CLAIMS_REQUIRED")), "true") {
		t.Fatalf("pulse-pro relay-server v6_grant.go not found and PULSE_GRANT_CLAIMS_REQUIRED=true")
	}
	t.Skip("pulse-pro relay-server v6_grant.go not found; set PULSE_GRANT_CLAIMS_REFERENCE to enforce")
	return "", ""
}

// extractGrantSchemaByJSON parses a Go file and returns a map keyed by JSON
// tag name (e.g. "lid", "max_agents") → grantField for the specified struct type.
func extractGrantSchemaByJSON(t *testing.T, path, typeName string) map[string]grantField {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != typeName {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("%s in %s is not a struct type", typeName, path)
			}

			fields := make(map[string]grantField)
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				goType := grantASTExprString(fset, field.Type)
				jsonTag := grantExtractJSONTag(field.Tag)
				if jsonTag == "" {
					continue
				}
				baseTag := grantBaseJSONTag(jsonTag)
				for _, name := range field.Names {
					fields[baseTag] = grantField{
						GoName:  name.Name,
						GoType:  goType,
						JSONTag: jsonTag,
					}
				}
			}
			return fields
		}
	}

	t.Fatalf("type %s not found in %s", typeName, path)
	return nil
}

// grantBaseJSONTag strips options (omitempty, etc.) from a JSON tag.
// "max_agents,omitempty" → "max_agents"
func grantBaseJSONTag(tag string) string {
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		return tag[:idx]
	}
	return tag
}

func grantASTExprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(buf.String()), "")
}

func grantExtractJSONTag(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		raw = strings.Trim(tag.Value, "`")
	}
	jsonTag := reflect.StructTag(raw).Get("json")
	if jsonTag == "" {
		return ""
	}
	return jsonTag
}

// TestGrantClaimsContractFieldListComplete validates that the contract JSON tag
// list covers all fields in GrantClaims. Any new field added to GrantClaims
// must be explicitly added to grantContractJSONTags or grantLocalOnlyJSONTags.
var grantLocalOnlyJSONTags = map[string]bool{
	// None currently — all GrantClaims fields are shared with relay-server.
	// If a pulse-client-only field is added in the future, list it here.
}

func TestGrantClaimsContractFieldListComplete(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	localPath := filepath.Join(repoRoot, "pkg", "licensing", "activation_types.go")

	schema := extractGrantSchemaByJSON(t, localPath, "GrantClaims")

	contractSet := make(map[string]bool, len(grantContractJSONTags))
	for _, tag := range grantContractJSONTags {
		contractSet[tag] = true
	}

	var uncovered []string
	for jsonTag := range schema {
		if !contractSet[jsonTag] && !grantLocalOnlyJSONTags[jsonTag] {
			uncovered = append(uncovered, jsonTag)
		}
	}
	sort.Strings(uncovered)

	if len(uncovered) > 0 {
		t.Errorf(
			"GrantClaims JSON tags not in contract or local-only list: %v\n"+
				"Add them to grantContractJSONTags or grantLocalOnlyJSONTags in this test",
			uncovered,
		)
	}
}
