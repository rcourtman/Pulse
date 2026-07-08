package edition

import "testing"

func TestDefaultsToCommunity(t *testing.T) {
	// A fresh process (no SetEdition call) must read as community so the
	// public community binary is never treated as Pro.
	if got := Current(); got != Community {
		t.Fatalf("Current() default = %q, want %q", got, Community)
	}
	if IsPro() {
		t.Fatal("IsPro() default = true, want false")
	}
}

func TestSetEditionPro(t *testing.T) {
	t.Cleanup(func() { SetEdition(Community) })

	SetEdition(Pro)
	if got := Current(); got != Pro {
		t.Fatalf("Current() after SetEdition(Pro) = %q, want %q", got, Pro)
	}
	if !IsPro() {
		t.Fatal("IsPro() after SetEdition(Pro) = false, want true")
	}
}

func TestSetEditionNormalizesUnknownToCommunity(t *testing.T) {
	t.Cleanup(func() { SetEdition(Community) })

	for _, name := range []string{"", "enterprise", "PRO ", "Community", "bogus"} {
		SetEdition(name)
		got := Current()
		want := Community
		if name == "PRO " {
			want = Pro // trimmed + case-insensitive match
		}
		if got != want {
			t.Fatalf("Current() after SetEdition(%q) = %q, want %q", name, got, want)
		}
	}
}
