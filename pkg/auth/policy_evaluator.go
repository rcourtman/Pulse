package auth

import (
	"context"
	"strings"
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
	permissions := e.getUserEffectivePermissions(username)
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
func (e *PolicyEvaluator) getUserEffectivePermissions(username string) []Permission {
	// Check if we have an extended manager with inheritance support
	if em, ok := e.manager.(ExtendedManager); ok {
		roles := em.GetRolesWithInheritance(username)
		var allPerms []Permission
		for _, role := range roles {
			allPerms = append(allPerms, role.Permissions...)
		}
		return allPerms
	}

	// Fall back to basic manager
	return e.manager.GetUserPermissions(username)
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
		// Check conditions
		if !e.evaluateConditions(perm, username, attributes) {
			continue // Condition not met, skip this permission
		}

		// Check effect
		effect := perm.GetEffect()
		if effect == EffectDeny {
			return false // Explicit deny wins immediately
		}

		if effect == EffectAllow {
			allowFound = true
		}
	}

	return allowFound // Return true only if we found an allow
}

// evaluateConditions checks if all conditions in a permission are satisfied.
func (e *PolicyEvaluator) evaluateConditions(perm Permission, username string, attributes map[string]string) bool {
	if len(perm.Conditions) == 0 {
		return true // No conditions = always matches
	}

	for key, expectedValue := range perm.Conditions {
		// Handle variable substitution
		expectedValue = e.substituteVariables(expectedValue, username, attributes)

		// Get actual value from attributes
		actualValue, exists := attributes[key]
		if !exists {
			return false // Required attribute missing
		}

		if actualValue != expectedValue {
			return false // Value doesn't match
		}
	}

	return true
}

// substituteVariables replaces ${variable} placeholders in condition values.
func (e *PolicyEvaluator) substituteVariables(value, username string, attributes map[string]string) string {
	// Replace ${user} with the current username
	value = strings.ReplaceAll(value, "${user}", username)

	// Replace ${attr.key} with attribute values
	for key, val := range attributes {
		value = strings.ReplaceAll(value, "${attr."+key+"}", val)
	}

	return value
}

// SetAdminUser implements AdminConfigurable.
// The admin user always has full access regardless of roles.
func (e *PolicyEvaluator) SetAdminUser(username string) {
	// Store admin user for bypass - not implemented in this basic version
	// The FileManager handles this separately
}

// RBACAuthorizer wraps PolicyEvaluator to implement Authorizer for the RBAC system.
type RBACAuthorizer struct {
	evaluator *PolicyEvaluator
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
	if a.adminUser != "" && username == a.adminUser {
		return true, nil
	}

	return a.evaluator.Authorize(ctx, action, resource)
}

// AuthorizeWithAttributes checks authorization with ABAC attributes.
func (a *RBACAuthorizer) AuthorizeWithAttributes(ctx context.Context, action string, resource string, attributes map[string]string) (bool, error) {
	username := GetUser(ctx)

	// Admin user bypass
	if a.adminUser != "" && username == a.adminUser {
		return true, nil
	}

	return a.evaluator.AuthorizeWithAttributes(ctx, action, resource, attributes)
}

// SetAdminUser sets the admin user who has full access.
func (a *RBACAuthorizer) SetAdminUser(username string) {
	a.adminUser = username
}

// AttributeAuthorizer extends Authorizer with attribute-based authorization.
type AttributeAuthorizer interface {
	Authorizer
	AuthorizeWithAttributes(ctx context.Context, action string, resource string, attributes map[string]string) (bool, error)
}
