// Generates TypeScript types for frontend consumption from Go types.
//
// Currently this covers Pulse Assistant chat SSE event payloads and the
// Pulse Intelligence agent capabilities discovery manifest.
// Sources of truth: internal/ai/chat event data structs and
// internal/agentcapabilities manifest structs.
//
// Usage:
//
//	go run ./scripts/generate-types.go
//
// Output:
//
//	frontend-modern/src/api/generated/aiChatEvents.ts
//	frontend-modern/src/api/generated/agentCapabilities.ts
//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
)

type tsTypeDef struct {
	Name string
	Body string
}

func main() {
	chatDefs := []reflect.Type{
		reflect.TypeOf(chat.ContentData{}),
		reflect.TypeOf(chat.ThinkingData{}),
		reflect.TypeOf(chat.WorkflowStateData{}),
		reflect.TypeOf(chat.SessionData{}),
		reflect.TypeOf(chat.ToolStartData{}),
		reflect.TypeOf(chat.ToolProgressData{}),
		reflect.TypeOf(chat.ToolCancelData{}),
		reflect.TypeOf(chat.ToolEndData{}),
		reflect.TypeOf(chat.ApprovalPlanData{}),
		reflect.TypeOf(chat.ApprovalContextConfidenceData{}),
		reflect.TypeOf(chat.ApprovalPreflightData{}),
		reflect.TypeOf(chat.ApprovalNeededData{}),
		reflect.TypeOf(chat.QuestionData{}),
		reflect.TypeOf(chat.Question{}),
		reflect.TypeOf(chat.QuestionOption{}),
		reflect.TypeOf(chat.SteerAppliedData{}),
		reflect.TypeOf(chat.DoneData{}),
		reflect.TypeOf(chat.ErrorData{}),
	}

	if err := writeTypeScriptDefinitions(
		filepath.Join("frontend-modern", "src", "api", "generated", "aiChatEvents.ts"),
		"internal/ai/chat event payload structs.",
		chatDefs,
		"",
		chatStreamEventUnion(),
	); err != nil {
		fatal(err)
	}

	agentCapabilityDefs := []reflect.Type{
		reflect.TypeOf(agentcapabilities.Capability{}),
		reflect.TypeOf(agentcapabilities.CapabilityCategory{}),
		reflect.TypeOf(agentcapabilities.Manifest{}),
		reflect.TypeOf(agentcapabilities.MCPAdapterConfigFamily{}),
		reflect.TypeOf(agentcapabilities.MCPAdapterContract{}),
		reflect.TypeOf(agentcapabilities.OperatorSurfaceContract{}),
		reflect.TypeOf(agentcapabilities.PulseWorkflowPrompt{}),
		reflect.TypeOf(agentcapabilities.PulseWorkflowPromptArgument{}),
		reflect.TypeOf(agentcapabilities.SurfaceAffordanceContract{}),
		reflect.TypeOf(agentcapabilities.SurfaceContract{}),
		reflect.TypeOf(agentcapabilities.SurfaceContractComponent{}),
		reflect.TypeOf(agentcapabilities.SurfaceToolContract{}),
	}

	if err := writeTypeScriptDefinitions(
		filepath.Join("frontend-modern", "src", "api", "generated", "agentCapabilities.ts"),
		"internal/agentcapabilities manifest structs.",
		agentCapabilityDefs,
		agentCapabilitiesAliases(),
		"",
	); err != nil {
		fatal(err)
	}
}

func writeTypeScriptDefinitions(outPath, source string, defs []reflect.Type, extraHeader, extraFooter string) error {
	tsDefs := make([]tsTypeDef, 0, len(defs))
	for _, t := range defs {
		tsDefs = append(tsDefs, tsTypeDef{Name: t.Name(), Body: renderInterface(t)})
	}
	// Sort for stable output.
	sort.Slice(tsDefs, func(i, j int) bool { return tsDefs[i].Name < tsDefs[j].Name })

	var buf bytes.Buffer
	buf.WriteString("// This file is generated from scripts/generate-types.go; DO NOT EDIT.\n")
	buf.WriteString("// Source: ")
	buf.WriteString(source)
	buf.WriteString("\n\n")
	buf.WriteString("/* eslint-disable */\n\n")
	if extraHeader != "" {
		buf.WriteString(extraHeader)
		if !strings.HasSuffix(extraHeader, "\n\n") {
			buf.WriteString("\n\n")
		}
	}

	for _, d := range tsDefs {
		buf.WriteString(d.Body)
		buf.WriteString("\n\n")
	}

	if extraFooter != "" {
		buf.WriteString(extraFooter)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	content := bytes.TrimRight(buf.Bytes(), "\n")
	content = append(content, '\n')
	if err := os.WriteFile(outPath, content, 0o644); err != nil {
		return err
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func chatStreamEventUnion() string {
	var buf strings.Builder
	// Stream event union matches internal/api/contract_test.go snapshots.
	buf.WriteString("export type AIChatStreamEvent =\n")
	buf.WriteString("  | { type: 'session'; data: SessionData }\n")
	buf.WriteString("  | { type: 'content'; data: ContentData }\n")
	buf.WriteString("  | { type: 'thinking'; data: ThinkingData }\n")
	buf.WriteString("  | { type: 'workflow_state'; data: WorkflowStateData }\n")
	buf.WriteString("  | { type: 'tool_start'; data: ToolStartData }\n")
	buf.WriteString("  | { type: 'tool_progress'; data: ToolProgressData }\n")
	buf.WriteString("  | { type: 'tool_cancel'; data: ToolCancelData }\n")
	buf.WriteString("  | { type: 'tool_end'; data: ToolEndData }\n")
	buf.WriteString("  | { type: 'approval_needed'; data: ApprovalNeededData }\n")
	// QuestionData is wrapped by the backend as {question_id, questions} plus session_id in some callers.
	// The contract test covers {question_id, questions}; the UI currently expects session_id too.
	// Keep session_id optional for backward compatibility.
	buf.WriteString("  | { type: 'question'; data: QuestionData & { session_id?: string } }\n")
	buf.WriteString("  | { type: 'steer_applied'; data: SteerAppliedData }\n")
	buf.WriteString("  | { type: 'done'; data?: DoneData }\n")
	buf.WriteString("  | { type: 'error'; data: ErrorData };\n")
	return buf.String()
}

func agentCapabilitiesAliases() string {
	return strings.Join([]string{
		"export type AgentCapabilityActionMode = 'read' | 'mixed' | 'write';",
		"export type AgentCapabilityApprovalPolicy = 'scope_only' | 'action_plan';",
	}, "\n") + "\n\n"
}

func renderInterface(t reflect.Type) string {
	if t.Kind() != reflect.Struct {
		return ""
	}

	var b strings.Builder
	b.WriteString("export interface ")
	b.WriteString(t.Name())
	b.WriteString(" {\n")

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}

		jsonName, optional := jsonFieldName(f)
		if jsonName == "-" || jsonName == "" {
			continue
		}

		b.WriteString("  ")
		b.WriteString(jsonName)
		if optional {
			b.WriteString("?: ")
		} else {
			b.WriteString(": ")
		}
		b.WriteString(goTypeToTS(f.Type))
		b.WriteString(";\n")
	}

	b.WriteString("}")
	return b.String()
}

func jsonFieldName(f reflect.StructField) (name string, optional bool) {
	tag := f.Tag.Get("json")
	if tag == "" {
		// Default to lower_snake? The backend always uses json tags for these structs.
		return "", false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, p := range parts[1:] {
		if p == "omitempty" {
			optional = true
			break
		}
	}
	// Pointers are optional in practice.
	if f.Type.Kind() == reflect.Pointer {
		optional = true
	}
	return name, optional
}

func goTypeToTS(t reflect.Type) string {
	if t.PkgPath() == "github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities" {
		switch t.Name() {
		case "ActionMode":
			return "AgentCapabilityActionMode"
		case "ApprovalPolicy":
			return "AgentCapabilityApprovalPolicy"
		}
	}
	if t.PkgPath() == "encoding/json" && t.Name() == "RawMessage" {
		return "Record<string, unknown>"
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "number"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "number"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return goTypeToTS(t.Elem()) + "[]"
	case reflect.Pointer:
		return goTypeToTS(t.Elem())
	case reflect.Struct:
		// Named structs become references.
		if t.Name() != "" {
			return t.Name()
		}
		// Fallback anonymous struct shape.
		return "Record<string, unknown>"
	case reflect.Map:
		// We only need map for generic payloads; keep unknown to avoid lying.
		return "Record<string, unknown>"
	case reflect.Interface:
		return "unknown"
	default:
		return "unknown"
	}
}
