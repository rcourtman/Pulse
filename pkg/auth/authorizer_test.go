package auth

import (
	"context"
	"testing"
)

type testAuthorizer struct {
	allowed bool
	err     error
	seen    struct {
		action   string
		resource string
	}
}

func (t *testAuthorizer) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	t.seen.action = action
	t.seen.resource = resource
	return t.allowed, t.err
}

type testToken struct {
	scopes map[string]bool
}

func (t testToken) HasScope(scope string) bool {
	return t.scopes[scope]
}

type adminAuthorizer struct {
	admin string
}

func (a *adminAuthorizer) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	return false, nil
}

func (a *adminAuthorizer) SetAdminUser(username string) {
	a.admin = username
}

func TestContextUserHelpers(t *testing.T) {
	ctx := WithUser(context.Background(), "alice")
	if got := GetUser(ctx); got != "alice" {
		t.Fatalf("expected user alice, got %q", got)
	}

	if got := GetUser(context.Background()); got != "" {
		t.Fatalf("expected empty user, got %q", got)
	}
}

func TestContextTokenHelpers(t *testing.T) {
	token := testToken{scopes: map[string]bool{"read": true}}
	ctx := WithAPIToken(context.Background(), token)

	got := GetAPIToken(ctx)
	if got == nil || !got.HasScope("read") {
		t.Fatalf("expected token with read scope")
	}

	if GetAPIToken(context.Background()) != nil {
		t.Fatalf("expected nil token")
	}
}

func TestSetAuthorizerAndHasPermission(t *testing.T) {
	orig := GetAuthorizer()
	defer SetAuthorizer(orig)

	custom := &testAuthorizer{allowed: true}
	SetAuthorizer(custom)

	if !HasPermission(context.Background(), "read", "nodes") {
		t.Fatalf("expected permission to be allowed")
	}
	if custom.seen.action != "read" || custom.seen.resource != "nodes" {
		t.Fatalf("expected authorizer to see read/nodes, got %q/%q", custom.seen.action, custom.seen.resource)
	}
}

func TestSetAdminUser(t *testing.T) {
	orig := GetAuthorizer()
	defer SetAuthorizer(orig)

	admin := &adminAuthorizer{}
	SetAuthorizer(admin)

	SetAdminUser("")
	if admin.admin != "" {
		t.Fatalf("expected empty admin, got %q", admin.admin)
	}

	SetAdminUser("root")
	if admin.admin != "root" {
		t.Fatalf("expected admin root, got %q", admin.admin)
	}
}

func TestSetAdminUserNonConfigurable(t *testing.T) {
	orig := GetAuthorizer()
	defer SetAuthorizer(orig)

	SetAuthorizer(&DefaultAuthorizer{})
	SetAdminUser("root")
}
