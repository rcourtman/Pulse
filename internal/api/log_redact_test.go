package api

import "testing"

func TestSafePrefixForLog(t *testing.T) {
	if got := safePrefixForLog("value", 0); got != "" {
		t.Fatalf("expected empty for n<=0, got %q", got)
	}
	if got := safePrefixForLog("", 3); got != "" {
		t.Fatalf("expected empty for empty value, got %q", got)
	}
	if got := safePrefixForLog("abc", 3); got != "abc" {
		t.Fatalf("expected full value when len<=n, got %q", got)
	}
	if got := safePrefixForLog("abcdef", 3); got != "abc" {
		t.Fatalf("expected prefix, got %q", got)
	}
}
