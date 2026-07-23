package api

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func stableSSOPrincipal(providerType config.SSOProviderType, providerID string, subject string) (string, error) {
	typ := strings.TrimSpace(strings.ToLower(string(providerType)))
	id := strings.TrimSpace(providerID)
	sub := strings.TrimSpace(subject)
	if typ == "" {
		return "", fmt.Errorf("sso provider type is required")
	}
	if id == "" {
		return "", fmt.Errorf("sso provider id is required")
	}
	if sub == "" {
		return "", fmt.Errorf("sso subject is required")
	}

	sum := sha256.Sum256([]byte(typ + "\x00" + id + "\x00" + sub))
	return "sso:" + typ + ":" + id + ":" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func ssoLegacyPrincipalCandidates(values ...string) []string {
	candidates := make([]string, 0, len(values)+2)
	seen := make(map[string]struct{}, len(values)+2)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; !ok {
			candidates = append(candidates, trimmed)
			seen[trimmed] = struct{}{}
		}

		if strings.Contains(trimmed, "@") {
			lower := strings.ToLower(trimmed)
			if _, ok := seen[lower]; !ok {
				candidates = append(candidates, lower)
				seen[lower] = struct{}{}
			}
		}
	}
	return candidates
}

func applySSORoleAssignments(manager internalauth.Manager, principal string, legacyCandidates []string, mappedRoles []string, mappingAuthoritative bool, ensureListed bool) error {
	principal = strings.TrimSpace(principal)
	if manager == nil || principal == "" {
		return nil
	}

	if mappingAuthoritative || len(mappedRoles) > 0 {
		return manager.UpdateUserRoles(principal, mappedRoles)
	}

	principalAssignment, principalExists := manager.GetUserAssignment(principal)
	principalHasRoles := principalExists && len(principalAssignment.RoleIDs) > 0

	for _, candidate := range legacyCandidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == principal {
			continue
		}
		assignment, ok := manager.GetUserAssignment(candidate)
		if !ok || len(assignment.RoleIDs) == 0 {
			continue
		}
		if migrator, ok := manager.(internalauth.AssignmentMigrator); ok {
			return migrator.MigrateUserAssignment(candidate, principal)
		}
		if principalHasRoles {
			return nil
		}
		return manager.UpdateUserRoles(principal, assignment.RoleIDs)
	}

	if principalHasRoles {
		return nil
	}
	if ensureListed {
		return manager.UpdateUserRoles(principal, nil)
	}
	return nil
}
