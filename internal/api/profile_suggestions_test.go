package api

import (
	"strings"
	"testing"
)

func TestParseAISuggestion(t *testing.T) {
	payload := "Here is your suggestion:\n```json\n" +
		`{"name":"Media Server","config":{"enable_docker":true},"rationale":["Uses Docker"]}` +
		"\n```\nThanks!"

	suggestion, err := parseAISuggestion(payload)
	if err != nil {
		t.Fatalf("parseAISuggestion error: %v", err)
	}
	if suggestion.Name != "Media Server" {
		t.Fatalf("unexpected name: %s", suggestion.Name)
	}
	if suggestion.Description == "" {
		t.Fatal("expected default description")
	}
	if suggestion.Config["enable_docker"] != true {
		t.Fatalf("unexpected config: %+v", suggestion.Config)
	}
	if len(suggestion.Rationale) != 1 {
		t.Fatalf("unexpected rationale: %+v", suggestion.Rationale)
	}

	payload = `{"name":"Test","description":"Has braces { in text"}`
	suggestion, err = parseAISuggestion(payload)
	if err != nil {
		t.Fatalf("parseAISuggestion error: %v", err)
	}
	if suggestion.Name != "Test" || suggestion.Description != "Has braces { in text" {
		t.Fatalf("unexpected suggestion: %+v", suggestion)
	}

	if _, err := parseAISuggestion("no json here"); err == nil {
		t.Fatal("expected error without JSON")
	}
	if _, err := parseAISuggestion("{\"name\":"); err == nil {
		t.Fatal("expected error for incomplete JSON")
	}
}

func TestBuildConfigSchemaDoc(t *testing.T) {
	doc := buildConfigSchemaDoc()
	if doc == "" {
		t.Fatal("expected schema doc")
	}
	if !strings.Contains(doc, "- interval (duration string") {
		t.Fatalf("expected interval key in doc:\n%s", doc)
	}
	if !strings.Contains(doc, "enable_docker") {
		t.Fatalf("expected enable_docker key in doc:\n%s", doc)
	}
}
