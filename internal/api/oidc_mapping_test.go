package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type mockAuthManager struct {
	auth.Manager
	updatedUser  string
	updatedRoles []string
}

func TestOIDCRoleMappingLogic(t *testing.T) {
	tests := []struct {
		name          string
		groupsClaim   string
		mappings      map[string]string
		claims        map[string]any
		expectedRoles []string
	}{
		{
			name:        "simple mapping",
			groupsClaim: "groups",
			mappings: map[string]string{
				"oidc-admin": "admin",
			},
			claims: map[string]any{
				"groups": []string{"oidc-admin", "other"},
			},
			expectedRoles: []string{"admin"},
		},
		{
			name:        "multiple mappings",
			groupsClaim: "groups",
			mappings: map[string]string{
				"oidc-admin": "admin",
				"oidc-dev":   "operator",
			},
			claims: map[string]any{
				"groups": []string{"oidc-admin", "oidc-dev"},
			},
			expectedRoles: []string{"admin", "operator"},
		},
		{
			name:        "comma separated string groups",
			groupsClaim: "groups",
			mappings: map[string]string{
				"admin": "admin",
				"user":  "viewer",
			},
			claims: map[string]any{
				"groups": "admin, user",
			},
			expectedRoles: []string{"admin", "viewer"},
		},
		{
			name:        "no matches",
			groupsClaim: "groups",
			mappings: map[string]string{
				"admin": "admin",
			},
			claims: map[string]any{
				"groups": []string{"guest"},
			},
			expectedRoles: nil,
		},
		// Regression cases for #1535: the IdP's group casing/whitespace must
		// not have to match the configured mapping exactly. AllowedGroups
		// already match case-insensitively (intersects), so role mapping has to
		// as well or a user passes the group gate but silently lands with no
		// role.
		{
			name:        "case-insensitive group match",
			groupsClaim: "groups",
			mappings: map[string]string{
				"admins": "admin",
			},
			claims: map[string]any{
				"groups": []string{"Admins"},
			},
			expectedRoles: []string{"admin"},
		},
		{
			name:        "whitespace-padded mapping key match",
			groupsClaim: "groups",
			mappings: map[string]string{
				" admin ": "admin",
			},
			claims: map[string]any{
				"groups": []string{"admin"},
			},
			expectedRoles: []string{"admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.OIDCConfig{
				GroupsClaim:       tt.groupsClaim,
				GroupRoleMappings: tt.mappings,
			}

			groups := extractStringSliceClaim(tt.claims, cfg.GroupsClaim)
			rolesToAssign := resolveGroupRoles(groups, cfg.GroupRoleMappings)

			if len(rolesToAssign) != len(tt.expectedRoles) {
				t.Errorf("expected %d roles, got %d", len(tt.expectedRoles), len(rolesToAssign))
				return
			}

			for i, r := range rolesToAssign {
				if r != tt.expectedRoles[i] {
					t.Errorf("expected role %s at index %d, got %s", tt.expectedRoles[i], i, r)
				}
			}
		})
	}
}
