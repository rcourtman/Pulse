package auth

import "testing"

type dummyManager struct{}

func (d dummyManager) GetRoles() []Role                         { return nil }
func (d dummyManager) GetRole(id string) (Role, bool)           { return Role{}, false }
func (d dummyManager) SaveRole(role Role) error                 { return nil }
func (d dummyManager) DeleteRole(id string) error               { return nil }
func (d dummyManager) GetUserAssignments() []UserRoleAssignment { return nil }
func (d dummyManager) GetUserAssignment(username string) (UserRoleAssignment, bool) {
	return UserRoleAssignment{}, false
}
func (d dummyManager) AssignRole(username string, roleID string) error { return nil }
func (d dummyManager) UpdateUserRoles(username string, roleIDs []string) error {
	return nil
}
func (d dummyManager) RemoveRole(username string, roleID string) error { return nil }
func (d dummyManager) GetUserPermissions(username string) []Permission { return nil }

type dummyExtendedManager struct {
	dummyManager
}

func (d dummyExtendedManager) GetRoleWithInheritance(id string) (Role, []Permission, bool) {
	return Role{}, nil, false
}
func (d dummyExtendedManager) GetRolesWithInheritance(username string) []Role { return nil }
func (d dummyExtendedManager) GetChangeLogs(limit int, offset int) []RBACChangeLog {
	return nil
}
func (d dummyExtendedManager) GetChangeLogsForEntity(entityType, entityID string) []RBACChangeLog {
	return nil
}
func (d dummyExtendedManager) SaveRoleWithContext(role Role, username string) error {
	return nil
}
func (d dummyExtendedManager) DeleteRoleWithContext(id string, username string) error {
	return nil
}
func (d dummyExtendedManager) UpdateUserRolesWithContext(username string, roleIDs []string, byUser string) error {
	return nil
}

func TestGetExtendedManager(t *testing.T) {
	orig := GetManager()
	t.Cleanup(func() { SetManager(orig) })

	base := &dummyManager{}
	SetManager(base)
	if GetManager() != base {
		t.Fatalf("expected GetManager to return the set manager")
	}
	if GetExtendedManager() != nil {
		t.Fatalf("expected nil extended manager")
	}

	extended := dummyExtendedManager{}
	SetManager(extended)
	if GetExtendedManager() == nil {
		t.Fatalf("expected extended manager")
	}
}
