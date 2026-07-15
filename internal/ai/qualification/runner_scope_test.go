package qualification

import (
	"reflect"
	"testing"
)

func TestPatrolScopeResourceIDsKeepsOracleResourcesOutsideTriggerAnchor(t *testing.T) {
	collected := map[string]Resource{
		"client":     {ID: "app-container:client"},
		"dependency": {ID: "app-container:dependency"},
	}
	manifest := validTestManifest()
	manifest.Patrol.Scoped = true
	manifest.Patrol.ScopeResources = []string{"client"}

	if got, want := patrolScopeResourceIDs(manifest, collected), []string{"app-container:client"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("scoped trigger IDs = %v, want %v", got, want)
	}
	if _, ok := collected["dependency"]; !ok {
		t.Fatal("ground-truth dependency must remain collected even when outside the Patrol trigger scope")
	}

	manifest.Patrol.ScopeResources = nil
	if got, want := patrolScopeResourceIDs(manifest, collected), []string{"app-container:client", "app-container:dependency"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("default scoped trigger IDs = %v, want %v", got, want)
	}
}
