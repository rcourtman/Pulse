package main

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
)

func TestParseTargetSpec(t *testing.T) {
	target, err := parseTargetSpec("https://pulse.example.com|abc123|true")
	if err != nil {
		t.Fatalf("parseTargetSpec returned error: %v", err)
	}

	if target.URL != "https://pulse.example.com" {
		t.Fatalf("expected URL https://pulse.example.com, got %q", target.URL)
	}
	if target.Token != "abc123" {
		t.Fatalf("expected token abc123, got %q", target.Token)
	}
	if !target.InsecureSkipVerify {
		t.Fatalf("expected insecure flag true")
	}
}

func TestParseTargetSpecDefaults(t *testing.T) {
	target, err := parseTargetSpec(" https://pulse.example.com | token456 ")
	if err != nil {
		t.Fatalf("parseTargetSpec returned error: %v", err)
	}

	if target.URL != "https://pulse.example.com" {
		t.Fatalf("expected URL https://pulse.example.com, got %q", target.URL)
	}
	if target.Token != "token456" {
		t.Fatalf("expected token token456, got %q", target.Token)
	}
	if target.InsecureSkipVerify {
		t.Fatalf("expected insecure flag false")
	}
}

func TestParseTargetSpecInvalid(t *testing.T) {
	if _, err := parseTargetSpec("https://pulse.example.com"); err == nil {
		t.Fatalf("expected error for missing token")
	}
	if _, err := parseTargetSpec("https://pulse.example.com|token|maybe"); err == nil {
		t.Fatalf("expected error for invalid insecure flag")
	}
}

func TestParseTargetSpecsSkipsBlanks(t *testing.T) {
	specs, err := parseTargetSpecs([]string{"https://a|tokenA", "   ", "\n", "https://b|tokenB|true"})
	if err != nil {
		t.Fatalf("parseTargetSpecs returned error: %v", err)
	}

	if len(specs) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(specs))
	}

	expected := []dockeragent.TargetConfig{
		{URL: "https://a", Token: "tokenA", InsecureSkipVerify: false},
		{URL: "https://b", Token: "tokenB", InsecureSkipVerify: true},
	}

	for i, target := range specs {
		if target != expected[i] {
			t.Fatalf("target %d mismatch: expected %+v, got %+v", i, expected[i], target)
		}
	}
}

func TestSplitTargetSpecs(t *testing.T) {
	values := splitTargetSpecs("https://a|tokenA;https://b|tokenB\nhttps://c|tokenC")
	expected := []string{"https://a|tokenA", "https://b|tokenB", "https://c|tokenC"}

	if len(values) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(values))
	}

	for i, v := range values {
		if v != expected[i] {
			t.Fatalf("value %d mismatch: expected %q, got %q", i, expected[i], v)
		}
	}
}
