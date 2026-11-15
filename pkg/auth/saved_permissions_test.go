package auth

import (
	"context"
	"reflect"
	"testing"
)

func savedPolicyManager(t *testing.T, kind string) Manager {
	t.Helper()
	if kind == "file" {
		m, err := NewFileManager(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	return m
}

func TestSavedPermissionsPreservePolicyMetadata(t *testing.T) {
	for _, kind := range []string{"file", "sqlite"} {
		t.Run(kind, func(t *testing.T) {
			m := savedPolicyManager(t, kind)
			parent := Role{ID: "saved-parent", Name: "Parent", Permissions: []Permission{
				{Action: "read", Resource: "backups", Effect: EffectAllow},
				{Action: "delete", Resource: "nodes", Effect: EffectDeny},
			}}
			child := Role{ID: "saved-child", Name: "Child", ParentID: parent.ID, Permissions: []Permission{
				{Action: "read", Resource: "nodes", Effect: EffectAllow},
				{Action: "read", Resource: "nodes", Effect: EffectDeny, Conditions: map[string]string{"env": "prod"}},
				{Action: "read", Resource: "nodes", Effect: EffectAllow, Conditions: map[string]string{"env": "test"}},
				{Action: "read", Resource: "nodes", Effect: EffectAllow, Conditions: map[string]string{"env": "dev"}},
				{Action: "delete", Resource: "nodes", Effect: EffectAllow},
			}}
			sibling := Role{ID: "saved-sibling", Name: "Sibling", ParentID: parent.ID, Permissions: []Permission{child.Permissions[0], child.Permissions[2]}}
			for _, r := range []Role{parent, child, sibling} {
				if err := m.SaveRole(r); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.UpdateUserRoles("saved-reader", []string{child.ID, sibling.ID}); err != nil {
				t.Fatal(err)
			}
			want := append(append([]Permission{}, parent.Permissions...), child.Permissions...)
			for i := 0; i < 3; i++ {
				if got := m.GetUserPermissions("saved-reader"); !reflect.DeepEqual(got, want) {
					t.Errorf("complete saved rules = %#v, want %#v", got, want)
				}
			}
			if reader, ok := m.(interface {
				GetUserPermissionsWithError(string) ([]Permission, error)
			}); ok {
				got, err := reader.GetUserPermissionsWithError("saved-reader")
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Errorf("error-aware read = %#v, %v", got, err)
				}
			} else {
				t.Error("missing error-aware permission reader")
			}
			// The same saved policy must govern real evaluator decisions, not just its inventory.
			evaluator := NewPolicyEvaluator(m)
			ctx := WithUser(context.Background(), "saved-reader")
			for _, tc := range []struct {
				action, resource string
				attrs            map[string]string
				allowed          bool
			}{
				{"read", "backups", nil, true}, {"delete", "nodes", nil, false},
				{"read", "nodes", map[string]string{"env": "prod"}, false},
				{"read", "nodes", map[string]string{"env": "test"}, true},
				{"read", "nodes", nil, false}, {"read", "unrelated", nil, false},
			} {
				got, err := evaluator.AuthorizeWithAttributes(ctx, tc.action, tc.resource, tc.attrs)
				if err != nil || got != tc.allowed {
					t.Errorf("authorize(%s,%s,%v) = %v,%v; want %v", tc.action, tc.resource, tc.attrs, got, err, tc.allowed)
				}
			}
			// Caller mutation must not change the saved condition map.
			got := m.GetUserPermissions("saved-reader")
			for _, p := range got {
				if p.Conditions["env"] == "prod" {
					p.Conditions["env"] = "caller-mutation"
				}
			}
			if next := m.GetUserPermissions("saved-reader"); !reflect.DeepEqual(next, want) {
				t.Error("returned conditions alias saved policy")
			}
			if got := m.GetUserPermissions("absent-reader"); len(got) != 0 {
				t.Errorf("unknown user has rules: %#v", got)
			}
		})
	}
}

func TestFileSavedPermissionsRejectIncompleteInheritance(t *testing.T) {
	for _, kind := range []string{"missing-parent", "cycle"} {
		t.Run(kind, func(t *testing.T) {
			m := savedPolicyManager(t, "file")
			parent := Role{ID: "parent", Name: "Parent", Permissions: []Permission{{Action: "write", Resource: "settings", Effect: EffectDeny}}}
			child := Role{ID: "child", Name: "Child", ParentID: parent.ID, Permissions: []Permission{{Action: "write", Resource: "settings"}}}
			for _, r := range []Role{parent, child} {
				if err := m.SaveRole(r); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.UpdateUserRoles("saved-reader", []string{child.ID}); err != nil {
				t.Fatal(err)
			}
			if kind == "missing-parent" {
				if err := m.DeleteRole(parent.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				parent.ParentID = child.ID
				if err := m.SaveRole(parent); err != nil {
					t.Fatal(err)
				}
			}
			reader, ok := m.(interface {
				GetUserPermissionsWithError(string) ([]Permission, error)
			})
			if !ok {
				t.Error("missing error-aware permission reader")
			} else {
				if got, err := reader.GetUserPermissionsWithError("saved-reader"); err == nil || len(got) != 0 {
					t.Errorf("incomplete chain returned %#v, %v", got, err)
				}
			}
			if got := m.GetUserPermissions("saved-reader"); len(got) != 0 {
				t.Errorf("legacy read returned partial policy %#v", got)
			}
			allowed, err := NewPolicyEvaluator(m).Authorize(WithUser(context.Background(), "saved-reader"), "write", "settings")
			if allowed || err == nil {
				t.Errorf("partial policy authorized = %v, %v", allowed, err)
			}
			// Repair exactly the saved parent; no user/role recreation is required.
			parent.ParentID = ""
			if err := m.SaveRole(parent); err != nil {
				t.Fatal(err)
			}
			allowed, err = NewPolicyEvaluator(m).Authorize(WithUser(context.Background(), "saved-reader"), "write", "settings")
			if allowed || err != nil {
				t.Errorf("restored inherited deny = %v, %v", allowed, err)
			}
		})
	}
}

func TestSavedPolicyConditionalDenyRequiresEvidence(t *testing.T) {
	for _, tc := range []struct {
		name              string
		conditions, attrs map[string]string
		allowed           bool
	}{
		{"nil attributes", map[string]string{"env": "prod"}, nil, false},
		{"partial attributes", map[string]string{"env": "prod", "owner": "${user}"}, map[string]string{"owner": "saved-reader"}, false},
		{"matching", map[string]string{"env": "prod"}, map[string]string{"env": "prod"}, false},
		{"known mismatch", map[string]string{"env": "prod"}, map[string]string{"env": "test"}, true},
		{"known mismatch plus missing", map[string]string{"env": "prod", "missing": "x"}, map[string]string{"env": "test"}, true},
		{"missing referenced attribute", map[string]string{"owner": "${attr.expected}"}, map[string]string{"owner": "${attr.expected}"}, false},
		{"matching reference", map[string]string{"owner": "${attr.expected}"}, map[string]string{"owner": "alice", "expected": "alice"}, false},
		{"mismatching reference", map[string]string{"owner": "${attr.expected}"}, map[string]string{"owner": "alice", "expected": "bob"}, true},
		{"empty is present", map[string]string{"env": ""}, map[string]string{"env": ""}, false},
		{"empty differs from absent", map[string]string{"env": ""}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, denyFirst := range []bool{false, true} {
				allow := Permission{Action: "read", Resource: "nodes:*"}
				deny := Permission{Action: "read", Resource: "nodes:prod", Effect: EffectDeny, Conditions: tc.conditions}
				rules := []Permission{allow, deny}
				if denyFirst {
					rules = []Permission{deny, allow}
				}
				evaluator := NewPolicyEvaluator(&basicPolicyReader{permissions: rules})
				got, err := evaluator.AuthorizeWithAttributes(WithUser(context.Background(), "saved-reader"), "read", "nodes:prod", tc.attrs)
				if err != nil || got != tc.allowed {
					t.Errorf("denyFirst=%v: allowed=%v,%v; want %v", denyFirst, got, err, tc.allowed)
				}
				// A nonmatching deny never suppresses unrelated grants.
				got, err = evaluator.Authorize(WithUser(context.Background(), "saved-reader"), "read", "nodes:other")
				if err != nil || !got {
					t.Errorf("unrelated grant=%v,%v", got, err)
				}
			}
		})
	}
	// Incomplete allow rules still grant nothing, even with a literal unresolved placeholder.
	evaluator := NewPolicyEvaluator(&basicPolicyReader{permissions: []Permission{{Action: "read", Resource: "nodes", Conditions: map[string]string{"owner": "${attr.expected}"}}}})
	got, err := evaluator.AuthorizeWithAttributes(WithUser(context.Background(), "saved-reader"), "read", "nodes", map[string]string{"owner": "${attr.expected}"})
	if err != nil || got {
		t.Errorf("unresolved allow = %v,%v", got, err)
	}
}

func TestSavedPermissionsDeduplicateOnlyCompleteRules(t *testing.T) {
	rules := []Permission{
		{Action: "read:future", Resource: "nodes"},
		{Action: "read", Resource: "future:nodes"},
		{Action: "read", Resource: "nodes", Conditions: map[string]string{"a": "b:c;d"}},
		{Action: "read", Resource: "nodes", Conditions: map[string]string{"a:b": "c;d"}},
		{Action: "read", Resource: "nodes", Conditions: map[string]string{"z": "last", "a": "first"}},
		{Action: "read", Resource: "nodes", Effect: EffectAllow},
		{Action: "read", Resource: "nodes"},
	}
	duplicate := Permission{Action: "read", Resource: "nodes", Conditions: map[string]string{"a": "first", "z": "last"}}
	got := savedPermissions([]Role{{Permissions: rules}, {Permissions: []Permission{duplicate, rules[0]}}})
	if !reflect.DeepEqual(got, rules) {
		t.Fatalf("rules were conflated: %#v; want %#v", got, rules)
	}
	// Go's JSON encoder replaces invalid UTF-8; identity must not conflate it
	// with another literal string and accidentally remove a deny or restriction.
	invalid := Permission{Action: string([]byte{'r', 0xff}), Resource: "nodes"}
	replacement := Permission{Action: "r\ufffd", Resource: "nodes"}
	if got := savedPermissions([]Role{{Permissions: []Permission{invalid, replacement}}}); len(got) != 2 {
		t.Fatal("nonidentical Go strings conflated")
	}
}
