package dockeragent

import (
	"reflect"
	"testing"

	swarmtypes "github.com/docker/docker/api/types/swarm"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestServiceMode(t *testing.T) {
	tests := []struct {
		name string
		mode swarmtypes.ServiceMode
		want string
	}{
		{
			name: "global mode",
			mode: swarmtypes.ServiceMode{
				Global: &swarmtypes.GlobalService{},
			},
			want: "global",
		},
		{
			name: "replicated mode",
			mode: swarmtypes.ServiceMode{
				Replicated: &swarmtypes.ReplicatedService{},
			},
			want: "replicated",
		},
		{
			name: "replicated-job mode",
			mode: swarmtypes.ServiceMode{
				ReplicatedJob: &swarmtypes.ReplicatedJob{},
			},
			want: "replicated-job",
		},
		{
			name: "global-job mode",
			mode: swarmtypes.ServiceMode{
				GlobalJob: &swarmtypes.GlobalJob{},
			},
			want: "global-job",
		},
		{
			name: "empty mode returns empty string",
			mode: swarmtypes.ServiceMode{},
			want: "",
		},
		{
			name: "multiple modes set returns first match (global takes precedence)",
			mode: swarmtypes.ServiceMode{
				Global:     &swarmtypes.GlobalService{},
				Replicated: &swarmtypes.ReplicatedService{},
			},
			want: "global",
		},
		{
			name: "replicated-job takes precedence over global-job",
			mode: swarmtypes.ServiceMode{
				ReplicatedJob: &swarmtypes.ReplicatedJob{},
				GlobalJob:     &swarmtypes.GlobalJob{},
			},
			want: "replicated-job",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serviceMode(tt.mode)
			if got != tt.want {
				t.Errorf("serviceMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildContainerIndex(t *testing.T) {
	tests := []struct {
		name       string
		containers []agentsdocker.Container
		wantKeys   []string
	}{
		{
			name:       "nil containers returns nil",
			containers: nil,
			wantKeys:   nil,
		},
		{
			name:       "empty containers returns nil",
			containers: []agentsdocker.Container{},
			wantKeys:   nil,
		},
		{
			name: "single container with long ID",
			containers: []agentsdocker.Container{
				{ID: "abcdef1234567890"},
			},
			wantKeys: []string{"abcdef1234567890", "abcdef123456"},
		},
		{
			name: "container with exactly 12 char ID",
			containers: []agentsdocker.Container{
				{ID: "abcdef123456"},
			},
			wantKeys: []string{"abcdef123456"},
		},
		{
			name: "container with short ID (less than 12)",
			containers: []agentsdocker.Container{
				{ID: "abc123"},
			},
			wantKeys: []string{"abc123"},
		},
		{
			name: "multiple containers",
			containers: []agentsdocker.Container{
				{ID: "container1234567890"},
				{ID: "another1234567890"},
			},
			wantKeys: []string{
				"container1234567890", "container123",
				"another1234567890", "another12345",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildContainerIndex(tt.containers)

			if tt.wantKeys == nil {
				if got != nil {
					t.Errorf("buildContainerIndex() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("buildContainerIndex() returned nil, want non-nil")
			}

			for _, key := range tt.wantKeys {
				if _, exists := got[key]; !exists {
					t.Errorf("buildContainerIndex() missing key %q", key)
				}
			}
		})
	}
}

func TestBuildContainerIndexPreservesData(t *testing.T) {
	containers := []agentsdocker.Container{
		{
			ID:    "abcdef1234567890",
			Name:  "mycontainer",
			Image: "nginx:latest",
			State: "running",
		},
	}

	index := buildContainerIndex(containers)

	// Both full ID and short ID should return the same container
	fullContainer, ok := index["abcdef1234567890"]
	if !ok {
		t.Fatal("full ID not found in index")
	}

	shortContainer, ok := index["abcdef123456"]
	if !ok {
		t.Fatal("short ID not found in index")
	}

	if fullContainer.Name != "mycontainer" {
		t.Errorf("full ID container name = %q, want %q", fullContainer.Name, "mycontainer")
	}

	if shortContainer.Name != "mycontainer" {
		t.Errorf("short ID container name = %q, want %q", shortContainer.Name, "mycontainer")
	}

	if fullContainer.Image != shortContainer.Image {
		t.Error("full ID and short ID containers should be identical")
	}
}

func TestLookupContainer(t *testing.T) {
	containers := []agentsdocker.Container{
		{ID: "abcdef1234567890abcd", Name: "container1"},
		{ID: "xyz1234567890xyz12", Name: "container2"},
	}
	index := buildContainerIndex(containers)

	tests := []struct {
		name      string
		index     map[string]agentsdocker.Container
		id        string
		wantName  string
		wantFound bool
	}{
		{
			name:      "nil index returns not found",
			index:     nil,
			id:        "abcdef1234567890",
			wantFound: false,
		},
		{
			name:      "empty index returns not found",
			index:     map[string]agentsdocker.Container{},
			id:        "abcdef1234567890",
			wantFound: false,
		},
		{
			name:      "full ID lookup",
			index:     index,
			id:        "abcdef1234567890abcd",
			wantName:  "container1",
			wantFound: true,
		},
		{
			name:      "short ID lookup (12 chars)",
			index:     index,
			id:        "abcdef123456",
			wantName:  "container1",
			wantFound: true,
		},
		{
			name:      "longer ID truncated to 12 for fallback",
			index:     index,
			id:        "abcdef12345600000000", // different suffix but same first 12
			wantName:  "container1",
			wantFound: true,
		},
		{
			name:      "ID not in index",
			index:     index,
			id:        "notfound1234567890",
			wantFound: false,
		},
		{
			name:      "very short ID not found",
			index:     index,
			id:        "abc",
			wantFound: false,
		},
		{
			name:      "second container by full ID",
			index:     index,
			id:        "xyz1234567890xyz12",
			wantName:  "container2",
			wantFound: true,
		},
		{
			name:      "second container by short ID",
			index:     index,
			id:        "xyz123456789",
			wantName:  "container2",
			wantFound: true,
		},
		{
			name:      "empty ID returns not found",
			index:     index,
			id:        "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := lookupContainer(tt.index, tt.id)

			if found != tt.wantFound {
				t.Errorf("lookupContainer() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound && got.Name != tt.wantName {
				t.Errorf("lookupContainer() name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

func TestCopyStringMap(t *testing.T) {
	tests := []struct {
		name   string
		source map[string]string
		want   map[string]string
	}{
		{
			name:   "nil map returns nil",
			source: nil,
			want:   nil,
		},
		{
			name:   "empty map returns nil",
			source: map[string]string{},
			want:   nil,
		},
		{
			name: "single entry",
			source: map[string]string{
				"key": "value",
			},
			want: map[string]string{
				"key": "value",
			},
		},
		{
			name: "multiple entries",
			source: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name: "empty values",
			source: map[string]string{
				"key": "",
			},
			want: map[string]string{
				"key": "",
			},
		},
		{
			name: "special characters",
			source: map[string]string{
				"key/with/slashes": "value=with=equals",
				"com.docker.label": "test",
			},
			want: map[string]string{
				"key/with/slashes": "value=with=equals",
				"com.docker.label": "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copyStringMap(tt.source)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("copyStringMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCopyStringMapIsDeepCopy(t *testing.T) {
	source := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	copied := copyStringMap(source)

	// Modify source
	source["key1"] = "modified"
	source["key3"] = "new"

	// Copied map should be unchanged
	if copied["key1"] != "value1" {
		t.Error("modifying source changed the copy")
	}

	if _, exists := copied["key3"]; exists {
		t.Error("adding to source affected the copy")
	}

	// Modify copy
	copied["key2"] = "also modified"

	// Source should be unchanged (well, it was already modified, but check consistency)
	if source["key2"] != "value2" {
		t.Error("modifying copy changed the source")
	}
}

func TestIsTaskCompletedState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  bool
	}{
		// Completed states
		{name: "completed lowercase", state: "completed", want: true},
		{name: "completed uppercase", state: "COMPLETED", want: true},
		{name: "completed mixed case", state: "Completed", want: true},
		{name: "complete lowercase", state: "complete", want: true},
		{name: "complete uppercase", state: "COMPLETE", want: true},
		{name: "shutdown lowercase", state: "shutdown", want: true},
		{name: "shutdown uppercase", state: "SHUTDOWN", want: true},
		{name: "failed lowercase", state: "failed", want: true},
		{name: "failed uppercase", state: "FAILED", want: true},
		{name: "rejected lowercase", state: "rejected", want: true},
		{name: "rejected uppercase", state: "REJECTED", want: true},

		// Non-completed states
		{name: "running", state: "running", want: false},
		{name: "pending", state: "pending", want: false},
		{name: "starting", state: "starting", want: false},
		{name: "preparing", state: "preparing", want: false},
		{name: "ready", state: "ready", want: false},
		{name: "assigned", state: "assigned", want: false},
		{name: "accepted", state: "accepted", want: false},
		{name: "new", state: "new", want: false},
		{name: "allocated", state: "allocated", want: false},
		{name: "orphaned", state: "orphaned", want: false},
		{name: "remove", state: "remove", want: false},

		// Edge cases
		{name: "empty string", state: "", want: false},
		{name: "whitespace", state: "  ", want: false},
		{name: "unknown state", state: "unknown", want: false},
		{name: "partial match - fail", state: "fail", want: false},
		{name: "partial match - complete (as prefix)", state: "completing", want: false},
		{name: "with leading space", state: " completed", want: false},
		{name: "with trailing space", state: "completed ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTaskCompletedState(tt.state)
			if got != tt.want {
				t.Errorf("isTaskCompletedState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}
