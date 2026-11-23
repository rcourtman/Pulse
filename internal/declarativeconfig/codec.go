package declarativeconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// MarshalYAML encodes doc as deterministic, block-style YAML whose field names
// match the canonical JSON schema identically across all nested Pulse types.
func MarshalYAML(doc Document) ([]byte, error) {
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal declarative config to json: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(jsonBytes, &node); err != nil {
		return nil, fmt.Errorf("convert declarative config to yaml node: %w", err)
	}
	normalizeYAMLNodeStyle(&node)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("encode declarative config yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close declarative config yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func normalizeYAMLNodeStyle(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			normalizeYAMLNodeStyle(child)
		}
	case yaml.MappingNode, yaml.SequenceNode:
		if len(node.Content) > 0 {
			node.Style = 0
		}
		for _, child := range node.Content {
			normalizeYAMLNodeStyle(child)
		}
	case yaml.ScalarNode:
		node.Style = 0
	}
}

// UnmarshalDocument parses either YAML or JSON into a Document using a YAML to
// JSON round-trip so embedded types that define only JSON tags or custom
// UnmarshalJSON methods decode identically in both formats.
func UnmarshalDocument(data []byte) (Document, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Document{}, fmt.Errorf("%w: document is empty", ErrInvalidDocument)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(trimmed))
	var raw any
	if err := decoder.Decode(&raw); err != nil {
		return Document{}, fmt.Errorf("%w: parse document: %v", ErrInvalidDocument, err)
	}
	if raw == nil {
		return Document{}, fmt.Errorf("%w: document is empty", ErrInvalidDocument)
	}

	var extra any
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return Document{}, fmt.Errorf("%w: parse trailing document: %v", ErrInvalidDocument, err)
	} else if err == nil && extra != nil {
		return Document{}, fmt.Errorf("%w: multiple YAML documents are not supported", ErrInvalidDocument)
	}

	normalized, err := normalizeYAMLValue(raw)
	if err != nil {
		return Document{}, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}

	if _, ok := normalized.(map[string]any); !ok {
		return Document{}, fmt.Errorf("%w: document root must be a mapping object", ErrInvalidDocument)
	}

	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		return Document{}, fmt.Errorf("%w: encode normalized document: %v", ErrInvalidDocument, err)
	}

	var doc Document
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return Document{}, fmt.Errorf("%w: decode document: %v", ErrInvalidDocument, err)
	}
	doc.APIVersion = strings.TrimSpace(doc.APIVersion)
	doc.Kind = strings.TrimSpace(doc.Kind)
	return doc, nil
}

func normalizeYAMLValue(v any) (any, error) {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, child := range typed {
			normChild, err := normalizeYAMLValue(child)
			if err != nil {
				return nil, err
			}
			out[k] = normChild
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(typed))
		for k, child := range typed {
			strKey, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("mapping keys must be strings")
			}
			normChild, err := normalizeYAMLValue(child)
			if err != nil {
				return nil, err
			}
			out[strKey] = normChild
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			normItem, err := normalizeYAMLValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = normItem
		}
		return out, nil
	default:
		return typed, nil
	}
}
