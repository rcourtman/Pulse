package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// The Docker host view carries the agent's failure-logs setting, which the
// install snapshot counts.
func TestDockerHostViewCarriesTheFailureLogsSetting(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		resource, _ := resourceFromDockerHost(models.DockerHost{ID: "docker01", Hostname: "docker01", FailureLogsEnabled: enabled})
		if got := NewDockerHostView(&resource).FailureLogsEnabled(); got != enabled {
			t.Fatalf("setting %v reads as %v", enabled, got)
		}
	}
	if (DockerHostView{}).FailureLogsEnabled() {
		t.Fatal("an empty view reports failure logs on")
	}
}
