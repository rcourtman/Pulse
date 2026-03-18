package api

import (
	"strings"
	"testing"
)

func TestIsValidOrganizationID(t *testing.T) {
	testCases := []struct {
		name  string
		orgID string
		want  bool
	}{
		{name: "default", orgID: "default", want: true},
		{name: "hyphen", orgID: "org-1", want: true},
		{name: "underscore", orgID: "org_1", want: true},
		{name: "dot", orgID: "org.1", want: true},
		{name: "max length", orgID: strings.Repeat("a", 64), want: true},
		{name: "empty", orgID: "", want: false},
		{name: "single dot", orgID: ".", want: false},
		{name: "double dot", orgID: "..", want: false},
		{name: "path traversal", orgID: "../evil", want: false},
		{name: "slash", orgID: "org/one", want: false},
		{name: "space", orgID: "org one", want: false},
		{name: "tab", orgID: "org\tone", want: false},
		{name: "newline", orgID: "org\none", want: false},
		{name: "backslash", orgID: "org\\one", want: false},
		{name: "colon", orgID: "org:one", want: false},
		{name: "too long", orgID: strings.Repeat("b", 65), want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidOrganizationID(tc.orgID)
			if got != tc.want {
				t.Fatalf("isValidOrganizationID(%q) = %v, want %v", tc.orgID, got, tc.want)
			}
		})
	}
}
