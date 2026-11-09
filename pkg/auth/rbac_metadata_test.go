package auth

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// identityTextReference pins the pre-optimization output, including the
// intentional difference between fitting and truncated malformed UTF-8.
func identityTextReference(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return value
}

var identityTextBenchmarkSink string

func TestIdentityMetadataTruncationMatchesReference(t *testing.T) {
	for _, limit := range []int{0, 1, maxIdentityProviderRunes, maxIdentityDisplayNameRunes, maxIdentityEmailRunes} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			for name, unit := range map[string]string{
				"ascii": "A", "two-byte": "é", "three-byte": "界", "four-byte": "🌍",
				"invalid": "\xff", "mixed": "A界\xff🌍\xe2\x82",
			} {
				for _, length := range []int{0, 1, limit, limit + 1, limit + 17} {
					input := "\u2003 \t" + strings.Repeat(unit, length) + "\u00a0\r\n"
					if got, want := truncateIdentityText(input, limit), identityTextReference(input, limit); got != want {
						t.Fatalf("%s x %d: got %q, want %q", name, length, got, want)
					}
				}
			}
		})
	}
}

func TestIdentityMetadataMalformedBoundary(t *testing.T) {
	for _, test := range []struct {
		name, value, want string
		limit             int
	}{
		{"fitting-invalid", "\xff", "\xff", 1},
		{"exact-invalid", "a\xff", "a\xff", 2},
		{"truncated-invalid", "\xffa", "\ufffd", 1},
		{"invalid-after-prefix", "a\xff", "a", 1},
		{"incomplete-prefix", "\xe2\x82z", "\ufffd\ufffd", 2},
		{"combining", "e\u0301x", "e\u0301", 2},
		{"unicode-space", "\u2003a\u00a0", "a", 1},
		{"empty-limit", "🌍x", "", 0},
		{"only-space", " \u2003\t", "", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := truncateIdentityText(test.value, test.limit); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestIdentityMetadataTruncationAllocationBound(t *testing.T) {
	// Construct outside measurement. Allocating for the full claim (or retaining
	// it via a substring) defeats the bound even if the displayed result is short.
	input := strings.Repeat("界", 1<<20)
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			identityTextBenchmarkSink = truncateIdentityText(input, maxIdentityEmailRunes)
		}
	})
	if bytes := result.AllocedBytesPerOp(); bytes < int64(len(identityTextBenchmarkSink)) || bytes > 16<<10 {
		t.Fatalf("truncated identity must own a bounded prefix: %d B/op, %d output bytes", bytes, len(identityTextBenchmarkSink))
	}
	if got, want := identityTextBenchmarkSink, strings.Repeat("界", maxIdentityEmailRunes); got != want {
		t.Fatalf("truncated identity differs: %q", got)
	}
	t.Logf("%d B/op, %d allocs/op for %d-byte claim", result.AllocedBytesPerOp(), result.AllocsPerOp(), len(input))
}

func TestIdentityMetadataFileManagerPreservesPrincipalsAndRoles(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewFileManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	metadata := UserIdentityMetadata{
		DisplayName:  " \u2003" + strings.Repeat("界🌍é", 300) + "\n",
		Email:        " " + strings.Repeat("a", 400) + "@example.invalid ",
		ProviderType: " " + strings.Repeat("OIDC", 100) + " ",
		ProviderID:   " " + strings.Repeat("provider-🌍", 100) + " ",
		LastLoginAt:  time.Date(2026, time.November, 9, 1, 2, 3, 0, time.UTC),
	}
	want := UserIdentityMetadata{
		DisplayName:  identityTextReference(metadata.DisplayName, maxIdentityDisplayNameRunes),
		Email:        identityTextReference(metadata.Email, maxIdentityEmailRunes),
		ProviderType: strings.ToLower(identityTextReference(metadata.ProviderType, maxIdentityProviderRunes)),
		ProviderID:   identityTextReference(metadata.ProviderID, maxIdentityProviderRunes),
		LastLoginAt:  metadata.LastLoginAt,
	}
	if got := NormalizeUserIdentityMetadata(metadata); !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized metadata = %#v, want %#v", got, want)
	}
	principals := map[string][]string{
		"sso:oidc:provider-a:stable": {RoleViewer},
		"sso:oidc:provider-b:stable": {RoleOperator},
	}
	for principal, roles := range principals {
		if err := manager.UpdateUserRoles(principal, roles); err != nil {
			t.Fatal(err)
		}
		if err := manager.UpsertUserIdentity(principal, metadata); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := NewFileManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	for principal, roles := range principals {
		assignment, ok := reloaded.GetUserAssignment(principal)
		if !ok || assignment.Username != principal || !reflect.DeepEqual(assignment.RoleIDs, roles) {
			t.Fatalf("identity metadata changed principal/roles: %#v, exists=%v", assignment, ok)
		}
		if assignment.DisplayName != want.DisplayName || assignment.Email != want.Email || assignment.ProviderType != want.ProviderType || assignment.ProviderID != want.ProviderID || assignment.LastLoginAt == nil || !assignment.LastLoginAt.Equal(want.LastLoginAt) {
			t.Fatalf("saved metadata differs: %#v", assignment)
		}
	}
}

func FuzzIdentityMetadataTruncation(f *testing.F) {
	for _, value := range []string{"", " a ", "\xffa", "a\xe2\x82", "界🌍e\u0301", strings.Repeat("🌍", 321), "\u2003\u00a0"} {
		for _, limit := range []uint16{0, 1, 128, 256, 320} {
			f.Add(value, limit)
		}
	}
	f.Fuzz(func(t *testing.T, value string, rawLimit uint16) {
		limit := int(rawLimit % 513)
		if got, want := truncateIdentityText(value, limit), identityTextReference(value, limit); got != want {
			t.Fatalf("limit %d, input %q: got %q, want %q", limit, value, got, want)
		}
	})
}

func BenchmarkIdentityMetadataTruncation(b *testing.B) {
	for _, size := range []int{16, 4096, 1 << 20} {
		for name, unit := range map[string]string{"ascii": "a", "unicode": "界🌍", "invalid": "\xffa"} {
			input := strings.Repeat(unit, size)
			for implementation, truncate := range map[string]func(string, int) string{
				"current": truncateIdentityText, "reference": identityTextReference,
			} {
				b.Run(fmt.Sprintf("%s/%d/%s", name, size, implementation), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						identityTextBenchmarkSink = truncate(input, maxIdentityDisplayNameRunes)
					}
				})
			}
		}
	}
}
