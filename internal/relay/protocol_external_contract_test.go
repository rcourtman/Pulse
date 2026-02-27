package relay

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

var protocolContractTypes = []string{
	"RegisterPayload",
	"RegisterAckPayload",
	"ConnectPayload",
	"ConnectAckPayload",
	"ChannelOpenPayload",
	"ChannelClosePayload",
	"ErrorPayload",
	"DrainPayload",
	"PushNotificationPayload",
}

type protocolField struct {
	TypeName string
	JSONName string
}

func TestProtocolPayloadSchemaMatchesRelayServer(t *testing.T) {
	localPath, referencePath := protocolPathsForComparison(t)

	localSchema := extractProtocolSchema(t, localPath)
	referenceSchema := extractProtocolSchema(t, referencePath)

	for _, typeName := range protocolContractTypes {
		localFields, ok := localSchema[typeName]
		if !ok {
			t.Fatalf("local protocol missing type %q in %s", typeName, localPath)
		}
		referenceFields, ok := referenceSchema[typeName]
		if !ok {
			t.Fatalf("reference protocol missing type %q in %s", typeName, referencePath)
		}

		for _, fieldName := range sortedFieldNames(localFields) {
			localField := localFields[fieldName]
			referenceField, ok := referenceFields[fieldName]
			if !ok {
				t.Errorf("%s.%s missing in reference protocol", typeName, fieldName)
				continue
			}
			if localField.TypeName != referenceField.TypeName {
				t.Errorf(
					"%s.%s type mismatch: local=%q reference=%q",
					typeName, fieldName, localField.TypeName, referenceField.TypeName,
				)
			}
			if localField.JSONName != referenceField.JSONName {
				t.Errorf(
					"%s.%s json tag mismatch: local=%q reference=%q",
					typeName, fieldName, localField.JSONName, referenceField.JSONName,
				)
			}
		}

		for _, fieldName := range sortedFieldNames(referenceFields) {
			if _, ok := localFields[fieldName]; !ok {
				t.Errorf("%s.%s exists in reference protocol but not local", typeName, fieldName)
			}
		}
	}
}

func protocolPathsForComparison(t *testing.T) (string, string) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	localPath := filepath.Join(repoRoot, "internal", "relay", "protocol.go")

	referenceFromEnv := strings.TrimSpace(os.Getenv("PULSE_RELAY_PROTOCOL_REFERENCE"))
	if referenceFromEnv != "" {
		if _, err := os.Stat(referenceFromEnv); err != nil {
			t.Fatalf(
				"PULSE_RELAY_PROTOCOL_REFERENCE is set but file is unavailable (%s): %v",
				referenceFromEnv, err,
			)
		}
		return localPath, referenceFromEnv
	}

	candidates := []string{
		filepath.Join(repoRoot, "..", "pulse-pro", "relay-server", "protocol.go"),
	}
	if reposDir := strings.TrimSpace(os.Getenv("PULSE_REPOS_DIR")); reposDir != "" {
		candidates = append(candidates, filepath.Join(reposDir, "pulse-pro", "relay-server", "protocol.go"))
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return localPath, candidate
		}
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_RELAY_PROTOCOL_REQUIRED")), "true") {
		t.Fatalf("pulse-pro relay protocol file not found and PULSE_RELAY_PROTOCOL_REQUIRED=true")
	}
	t.Skip("pulse-pro relay protocol file not found; set PULSE_RELAY_PROTOCOL_REFERENCE to enforce this check")
	return "", ""
}

func extractProtocolSchema(t *testing.T, path string) map[string]map[string]protocolField {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse protocol file %s: %v", path, err)
	}

	wantTypes := make(map[string]struct{}, len(protocolContractTypes))
	for _, typeName := range protocolContractTypes {
		wantTypes[typeName] = struct{}{}
	}

	out := make(map[string]map[string]protocolField, len(protocolContractTypes))

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			typeName := typeSpec.Name.Name
			if _, keep := wantTypes[typeName]; !keep {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("%s in %s is not a struct type", typeName, path)
			}

			fields := make(map[string]protocolField)
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				typeName := normalizeType(astExprString(fset, field.Type))
				jsonName := jsonTagName(field.Tag)
				for _, name := range field.Names {
					fields[name.Name] = protocolField{
						TypeName: typeName,
						JSONName: jsonName,
					}
				}
			}
			out[typeSpec.Name.Name] = fields
		}
	}

	return out
}

func astExprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return ""
	}
	return buf.String()
}

func normalizeType(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func jsonTagName(tag *ast.BasicLit) string {
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
	return strings.Split(jsonTag, ",")[0]
}

func sortedFieldNames(fields map[string]protocolField) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// protocolContractConstants lists constants whose values must be identical
// across pulse/internal/relay/protocol.go and pulse-pro/relay-server/protocol.go.
// Frame type bytes and error code strings are the wire contract — any drift
// breaks cross-repo communication.
var protocolContractConstants = []string{
	// Protocol parameters
	"ProtocolVersion",
	"MaxPayloadSize",
	"HeaderSize",
	// Frame type bytes
	"FrameRegister",
	"FrameRegisterAck",
	"FrameConnect",
	"FrameConnectAck",
	"FrameChannelOpen",
	"FrameChannelClose",
	"FrameData",
	"FramePing",
	"FramePong",
	"FrameError",
	"FrameDrain",
	"FrameKeyExchange",
	"FramePushNotification",
	// Error code strings
	"ErrCodeInternal",
	"ErrCodeNotFound",
	"ErrCodeAuthFailed",
	"ErrCodeLicenseInvalid",
	"ErrCodeLicenseExpired",
	"ErrCodeRateLimited",
	"ErrCodeDuplicate",
	"ErrCodeChannelFull",
	"ErrCodeDraining",
}

// TestProtocolConstantsMatchRelayServer validates that frame type bytes,
// error code strings, and protocol parameters have identical literal values
// in both the local relay client and the relay-server reference copy.
// This catches wire-level drift that the struct schema test cannot detect.
//
// Values are compared as source-text expressions. Since protocol.go is kept
// as a verbatim copy across repos, identical text implies identical semantics.
// If the files ever diverge in expression style (e.g. 64*1024 vs 65536),
// this test intentionally fails to force manual review.
func TestProtocolConstantsMatchRelayServer(t *testing.T) {
	localPath, referencePath := protocolPathsForComparison(t)

	localConsts := extractConstants(t, localPath)
	referenceConsts := extractConstants(t, referencePath)

	for _, name := range protocolContractConstants {
		localVal, lok := localConsts[name]
		refVal, rok := referenceConsts[name]

		if !lok {
			t.Errorf("constant %s missing in local protocol (%s)", name, localPath)
			continue
		}
		if !rok {
			t.Errorf("constant %s missing in reference protocol (%s)", name, referencePath)
			continue
		}
		if localVal != refVal {
			t.Errorf("constant %s drift: local=%s reference=%s", name, localVal, refVal)
		}
	}
}

// extractConstants parses a Go source file and returns a map of constant
// name → literal value (as source text) for all top-level const declarations.
//
// Each const spec is expected to have exactly one name and one value (the
// standard pattern in protocol.go). Multi-name specs (e.g. "A, B = 1, 2")
// and iota blocks are not used in the protocol contract and are skipped to
// avoid incorrect inheritance logic.
func extractConstants(t *testing.T, path string) map[string]string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse protocol file %s: %v", path, err)
	}

	out := make(map[string]string)

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}

		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			// Only handle simple single-name, single-value const declarations.
			// Skip multi-name or value-less (iota-inherited) specs since the
			// protocol contract constants don't use those patterns.
			if len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}

			out[vs.Names[0].Name] = astExprString(fset, vs.Values[0])
		}
	}

	return out
}
