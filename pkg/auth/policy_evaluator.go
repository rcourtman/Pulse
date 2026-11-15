package auth

import (
	"context"
	"strings"
	"sync"
)

// PolicyEvaluator implements Authorizer with advanced policy evaluation.
// It supports:
// - Deny precedence (deny rules override allow rules)
// - Role inheritance
// - Attribute-based conditions
// - Resource wildcards
type PolicyEvaluator struct {
	manager Manager
}

// NewPolicyEvaluator creates a new policy evaluator.
func NewPolicyEvaluator(manager Manager) *PolicyEvaluator {
	return &PolicyEvaluator{manager: manager}
}

// Authorize checks if the user in the context can perform the action on the resource.
// Returns true if allowed, false if denied.
func (e *PolicyEvaluator) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	return e.AuthorizeWithAttributes(ctx, action, resource, nil)
}

// AuthorizeWithAttributes checks authorization with additional attributes for ABAC.
func (e *PolicyEvaluator) AuthorizeWithAttributes(ctx context.Context, action string, resource string, attributes map[string]string) (bool, error) {
	username := GetUser(ctx)
	if username == "" {
		return false, nil // No user in context = deny
	}

	// Get all permissions for the user
	permissions, err := e.getUserEffectivePermissions(username)
	if err != nil {
		return false, err
	}
	if len(permissions) == 0 {
		return false, nil // No permissions = deny
	}

	// Filter permissions that match the requested action and resource
	matching := e.filterMatching(permissions, action, resource)
	if len(matching) == 0 {
		return false, nil // No matching permissions = deny
	}

	// Evaluate conditions and apply deny precedence
	return e.evaluateWithDenyPrecedence(matching, username, attributes), nil
}

// getUserEffectivePermissions returns all permissions for a user including inherited ones.
func (e *PolicyEvaluator) getUserEffectivePermissions(username string) ([]Permission, error) {
	// Check if we have an extended manager with inheritance support
	if em, ok := e.manager.(ExtendedManager); ok {
		var roles []Role
		if reader, ok := em.(ErrorAwareInheritedRoleReader); ok {
			var err error
			roles, err = reader.GetRolesWithInheritanceWithError(username)
			if err != nil {
				return nil, err
			}
		} else {
			roles = em.GetRolesWithInheritance(username)
		}
		var allPerms []Permission
		for _, role := range roles {
			allPerms = append(allPerms, role.Permissions...)
		}
		return allPerms, nil
	}

	// Basic managers may also preserve read errors. Never evaluate a partial
	// result or silently downgrade a failed read to an ordinary policy denial.
	if reader, ok := e.manager.(ErrorAwarePermissionReader); ok {
		return reader.GetUserPermissionsWithError(username)
	}
	return e.manager.GetUserPermissions(username), nil
}

// filterMatching returns permissions that match the requested action and resource.
func (e *PolicyEvaluator) filterMatching(permissions []Permission, action, resource string) []Permission {
	var matching []Permission

	for _, perm := range permissions {
		// Check action match
		if !MatchesAction(perm.Action, action) {
			continue
		}

		// Check resource match
		if !MatchesResource(perm.Resource, resource) {
			continue
		}

		matching = append(matching, perm)
	}

	return matching
}

// evaluateWithDenyPrecedence evaluates permissions with deny taking precedence.
// Order: explicit deny > explicit allow > implicit deny
func (e *PolicyEvaluator) evaluateWithDenyPrecedence(permissions []Permission, username string, attributes map[string]string) bool {
	var allowFound bool

	for _, perm := range permissions {
		matches, complete := e.evaluateConditions(perm, username, attributes)
		effect := perm.GetEffect()
		if effect == EffectDeny && (matches || !complete) {
			// Missing evidence cannot dismiss a potentially applicable deny.
			// A known mismatch, however, conclusively excludes this rule.
			return false
		}

		if effect == EffectAllow && matches && complete {
			allowFound = true
		}
	}

	return allowFound // Return true only if we found an allow
}

// evaluateConditions distinguishes a known mismatch from incomplete evidence.
// Inspect every condition: a known mismatch disproves a rule even when another
// condition is unknown, regardless of Go map iteration order.
func (e *PolicyEvaluator) evaluateConditions(perm Permission, username string, attributes map[string]string) (matches, complete bool) {
	complete = true
	for key, expectedValue := range perm.Conditions {
		var resolved bool
		expectedValue, resolved = e.substituteVariables(expectedValue, username, attributes)

		// Get actual value from attributes
		actualValue, exists := attributes[key]
		if !exists || !resolved {
			complete = false
			continue
		}

		if actualValue != expectedValue {
			return false, true // A known mismatch rules this permission out.
		}
	}

	return complete, complete
}

// substituteVariables replaces ${variable} placeholders in condition values.
func (e *PolicyEvaluator) substituteVariables(value, username string, attributes map[string]string) (string, bool) {
	// Replace ${user} with the current username
	value = strings.ReplaceAll(value, "${user}", username)

	// Resolve placeholders in the saved expression once, not recursively from
	// attribute values. Missing references are unknown, never literal matches.
	var result strings.Builder
	for {
		start := strings.Index(value, "${attr.")
		if start < 0 {
			result.WriteString(value)
			return result.String(), true
		}
		result.WriteString(value[:start])
		value = value[start+len("${attr."):]
		end := strings.IndexByte(value, '}')
		if end < 0 {
			return "", false
		}
		attribute, exists := attributes[value[:end]]
		if !exists {
			return "", false
		}
		result.WriteString(attribute)
		value = value[end+1:]
	}
}

// RBACAuthorizer wraps PolicyEvaluator to implement Authorizer for the RBAC system.
type RBACAuthorizer struct {
	evaluator *PolicyEvaluator
	adminMu   sync.RWMutex
	adminUser string
}

// NewRBACAuthorizer creates a new RBAC authorizer.
func NewRBACAuthorizer(manager Manager) *RBACAuthorizer {
	return &RBACAuthorizer{
		evaluator: NewPolicyEvaluator(manager),
	}
}

// Authorize checks if the user can perform the action on the resource.
func (a *RBACAuthorizer) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	username := GetUser(ctx)

	// Admin user bypass
	a.adminMu.RLock()
	isAdmin := a.adminUser != "" && username == a.adminUser
	a.adminMu.RUnlock()
	if isAdmin {
		return true, nil
	}

	return a.evaluator.Authorize(ctx, action, resource)
}

// SetAdminUser sets the admin user who has full access.
func (a *RBACAuthorizer) SetAdminUser(username string) {
	a.adminMu.Lock()
	a.adminUser = username
	a.adminMu.Unlock()
}

// AttributeAuthorizer extends Authorizer with attribute-based authorization.
type AttributeAuthorizer interface {
	Authorizer
	AuthorizeWithAttributes(ctx context.Context, action string, resource string, attributes map[string]string) (bool, error)
}
