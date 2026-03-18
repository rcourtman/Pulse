package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// --- statusFromString ---

func TestStatusFromString_Online(t *testing.T) {
	for _, s := range []string{"online", "running", "ok", "ONLINE", " Running "} {
		got := statusFromString(s)
		if got != StatusOnline {
			t.Errorf("statusFromString(%q) = %q, want %q", s, got, StatusOnline)
		}
	}
}

func TestStatusFromString_Offline(t *testing.T) {
	for _, s := range []string{"offline", "down", "stopped"} {
		got := statusFromString(s)
		if got != StatusOffline {
			t.Errorf("statusFromString(%q) = %q, want %q", s, got, StatusOffline)
		}
	}
}

func TestStatusFromString_Warning(t *testing.T) {
	for _, s := range []string{"warning", "degraded"} {
		if statusFromString(s) != StatusWarning {
			t.Errorf("statusFromString(%q) should be warning", s)
		}
	}
}

func TestStatusFromString_Unknown(t *testing.T) {
	for _, s := range []string{"", "bogus", "  "} {
		if statusFromString(s) != StatusUnknown {
			t.Errorf("statusFromString(%q) should be unknown", s)
		}
	}
}

// --- statusFromGuest ---

func TestStatusFromGuest_Online(t *testing.T) {
	for _, s := range []string{"running", "online"} {
		if statusFromGuest(s) != StatusOnline {
			t.Errorf("statusFromGuest(%q) should be online", s)
		}
	}
}

func TestStatusFromGuest_Offline(t *testing.T) {
	for _, s := range []string{"stopped", "offline", "paused"} {
		if statusFromGuest(s) != StatusOffline {
			t.Errorf("statusFromGuest(%q) should be offline", s)
		}
	}
}

// --- statusFromStorage ---

func TestStatusFromStorage_Online(t *testing.T) {
	for _, s := range []string{"online", "running", "available", "active", "ok"} {
		got := statusFromStorage(models.Storage{Status: s, Active: true, Enabled: true})
		if got != StatusOnline {
			t.Errorf("statusFromStorage(status=%q) = %q, want online", s, got)
		}
	}
}

func TestStatusFromStorage_Offline(t *testing.T) {
	for _, s := range []string{"offline", "down", "unavailable", "error"} {
		got := statusFromStorage(models.Storage{Status: s})
		if got != StatusOffline {
			t.Errorf("statusFromStorage(status=%q) = %q, want offline", s, got)
		}
	}
}

func TestStatusFromStorage_InactiveDefault(t *testing.T) {
	got := statusFromStorage(models.Storage{Status: "", Active: false, Enabled: true})
	if got != StatusOffline {
		t.Errorf("inactive storage without explicit status should be offline, got %q", got)
	}
}

func TestStatusFromStorage_DisabledButActive(t *testing.T) {
	got := statusFromStorage(models.Storage{Status: "", Active: true, Enabled: false})
	if got != StatusWarning {
		t.Errorf("active but disabled storage should be warning, got %q", got)
	}
}

func TestStatusFromStorage_ActiveEnabled(t *testing.T) {
	got := statusFromStorage(models.Storage{Status: "", Active: true, Enabled: true})
	if got != StatusOnline {
		t.Errorf("active+enabled storage with unknown status should be online, got %q", got)
	}
}

// --- statusFromPhysicalDisk ---

func TestStatusFromPhysicalDisk(t *testing.T) {
	tests := []struct {
		health   string
		expected ResourceStatus
	}{
		{"PASSED", StatusOnline},
		{"OK", StatusOnline},
		{"passed", StatusOnline},
		{"FAILED", StatusOffline},
		{"UNKNOWN", StatusUnknown},
		{"", StatusUnknown},
	}
	for _, tt := range tests {
		got := statusFromPhysicalDisk(tt.health)
		if got != tt.expected {
			t.Errorf("statusFromPhysicalDisk(%q) = %q, want %q", tt.health, got, tt.expected)
		}
	}
}

// --- statusFromCephHealth ---

func TestStatusFromCephHealth(t *testing.T) {
	tests := []struct {
		health   string
		expected ResourceStatus
	}{
		{"HEALTH_OK", StatusOnline},
		{"HEALTH_WARN", StatusWarning},
		{"HEALTH_ERR", StatusOffline},
		{"", StatusUnknown},
	}
	for _, tt := range tests {
		got := statusFromCephHealth(tt.health)
		if got != tt.expected {
			t.Errorf("statusFromCephHealth(%q) = %q, want %q", tt.health, got, tt.expected)
		}
	}
}

// --- statusFromDockerState ---

func TestStatusFromDockerState(t *testing.T) {
	tests := []struct {
		state    string
		expected ResourceStatus
	}{
		{"running", StatusOnline},
		{"created", StatusOffline},
		{"exited", StatusOffline},
		{"dead", StatusOffline},
		{"paused", StatusOffline},
		{"restarting", StatusWarning},
		{"", StatusUnknown},
	}
	for _, tt := range tests {
		got := statusFromDockerState(tt.state)
		if got != tt.expected {
			t.Errorf("statusFromDockerState(%q) = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

// --- statusFromDockerService ---

func TestStatusFromDockerService_AllRunning(t *testing.T) {
	got := statusFromDockerService(models.DockerService{DesiredTasks: 3, RunningTasks: 3})
	if got != StatusOnline {
		t.Errorf("all running should be online, got %q", got)
	}
}

func TestStatusFromDockerService_NoneRunning(t *testing.T) {
	got := statusFromDockerService(models.DockerService{DesiredTasks: 3, RunningTasks: 0})
	if got != StatusOffline {
		t.Errorf("none running should be offline, got %q", got)
	}
}

func TestStatusFromDockerService_Partial(t *testing.T) {
	got := statusFromDockerService(models.DockerService{DesiredTasks: 3, RunningTasks: 1})
	if got != StatusWarning {
		t.Errorf("partial should be warning, got %q", got)
	}
}

func TestStatusFromDockerService_ZeroDesiredZeroRunning(t *testing.T) {
	got := statusFromDockerService(models.DockerService{DesiredTasks: 0, RunningTasks: 0})
	if got != StatusUnknown {
		t.Errorf("zero/zero should be unknown, got %q", got)
	}
}

func TestStatusFromDockerService_ZeroDesiredButRunning(t *testing.T) {
	got := statusFromDockerService(models.DockerService{DesiredTasks: 0, RunningTasks: 2})
	if got != StatusOnline {
		t.Errorf("zero desired but running should be online, got %q", got)
	}
}

// --- statusFromPBSInstance ---

func TestStatusFromPBSInstance_Primary(t *testing.T) {
	for _, s := range []string{"online", "running", "ok", "healthy"} {
		got := statusFromPBSInstance(models.PBSInstance{Status: s})
		if got != StatusOnline {
			t.Errorf("statusFromPBSInstance(status=%q) = %q, want online", s, got)
		}
	}
}

func TestStatusFromPBSInstance_FallbackToHealth(t *testing.T) {
	got := statusFromPBSInstance(models.PBSInstance{Status: "bogus", ConnectionHealth: "connected"})
	if got != StatusOnline {
		t.Errorf("should fall back to connection health, got %q", got)
	}
}

// --- statusFromKubernetesCluster ---

func TestStatusFromKubernetesCluster_AllReady(t *testing.T) {
	got := statusFromKubernetesCluster(models.KubernetesCluster{
		Nodes: []models.KubernetesNode{
			{Ready: true}, {Ready: true},
		},
	})
	if got != StatusOnline {
		t.Errorf("all ready nodes should be online, got %q", got)
	}
}

func TestStatusFromKubernetesCluster_NoneReady(t *testing.T) {
	got := statusFromKubernetesCluster(models.KubernetesCluster{
		Nodes: []models.KubernetesNode{
			{Ready: false}, {Ready: false},
		},
	})
	if got != StatusOffline {
		t.Errorf("no ready nodes should be offline, got %q", got)
	}
}

func TestStatusFromKubernetesCluster_PartialReady(t *testing.T) {
	got := statusFromKubernetesCluster(models.KubernetesCluster{
		Nodes: []models.KubernetesNode{
			{Ready: true}, {Ready: false},
		},
	})
	if got != StatusWarning {
		t.Errorf("partial ready should be warning, got %q", got)
	}
}

func TestStatusFromKubernetesCluster_NoNodes(t *testing.T) {
	got := statusFromKubernetesCluster(models.KubernetesCluster{})
	if got != StatusUnknown {
		t.Errorf("no nodes should be unknown, got %q", got)
	}
}

func TestStatusFromKubernetesCluster_ExplicitStatus(t *testing.T) {
	got := statusFromKubernetesCluster(models.KubernetesCluster{
		Status: "healthy",
		Nodes:  []models.KubernetesNode{{Ready: false}},
	})
	if got != StatusOnline {
		t.Errorf("explicit healthy status should override node calculation, got %q", got)
	}
}

// --- statusFromKubernetesNode ---

func TestStatusFromKubernetesNode(t *testing.T) {
	tests := []struct {
		name     string
		node     models.KubernetesNode
		expected ResourceStatus
	}{
		{"ready", models.KubernetesNode{Ready: true}, StatusOnline},
		{"not ready", models.KubernetesNode{Ready: false}, StatusOffline},
		{"unschedulable", models.KubernetesNode{Ready: true, Unschedulable: true}, StatusWarning},
	}
	for _, tt := range tests {
		got := statusFromKubernetesNode(tt.node)
		if got != tt.expected {
			t.Errorf("statusFromKubernetesNode(%s) = %q, want %q", tt.name, got, tt.expected)
		}
	}
}

// --- statusFromKubernetesPod ---

func TestStatusFromKubernetesPod_Running(t *testing.T) {
	got := statusFromKubernetesPod(models.KubernetesPod{Phase: "running"})
	if got != StatusOnline {
		t.Errorf("running pod should be online, got %q", got)
	}
}

func TestStatusFromKubernetesPod_RunningUnreadyContainer(t *testing.T) {
	got := statusFromKubernetesPod(models.KubernetesPod{
		Phase: "running",
		Containers: []models.KubernetesPodContainer{
			{Ready: true, State: "running"},
			{Ready: false, State: "waiting"},
		},
	})
	if got != StatusWarning {
		t.Errorf("running pod with unready container should be warning, got %q", got)
	}
}

func TestStatusFromKubernetesPod_FailedPhase(t *testing.T) {
	got := statusFromKubernetesPod(models.KubernetesPod{Phase: "failed"})
	if got != StatusOffline {
		t.Errorf("failed pod should be offline, got %q", got)
	}
}

func TestStatusFromKubernetesPod_Pending(t *testing.T) {
	got := statusFromKubernetesPod(models.KubernetesPod{Phase: "pending"})
	if got != StatusWarning {
		t.Errorf("pending pod should be warning, got %q", got)
	}
}

// --- statusFromKubernetesDeployment ---

func TestStatusFromKubernetesDeployment_AllAvailable(t *testing.T) {
	got := statusFromKubernetesDeployment(models.KubernetesDeployment{
		DesiredReplicas:   3,
		AvailableReplicas: 3,
	})
	if got != StatusOnline {
		t.Errorf("all available should be online, got %q", got)
	}
}

func TestStatusFromKubernetesDeployment_NoneAvailable(t *testing.T) {
	got := statusFromKubernetesDeployment(models.KubernetesDeployment{
		DesiredReplicas:   3,
		AvailableReplicas: 0,
	})
	if got != StatusOffline {
		t.Errorf("none available should be offline, got %q", got)
	}
}

func TestStatusFromKubernetesDeployment_Partial(t *testing.T) {
	got := statusFromKubernetesDeployment(models.KubernetesDeployment{
		DesiredReplicas:   3,
		AvailableReplicas: 1,
	})
	if got != StatusWarning {
		t.Errorf("partial should be warning, got %q", got)
	}
}

func TestStatusFromKubernetesDeployment_ZeroDesired(t *testing.T) {
	got := statusFromKubernetesDeployment(models.KubernetesDeployment{
		DesiredReplicas:   0,
		AvailableReplicas: 0,
	})
	if got != StatusUnknown {
		t.Errorf("zero/zero should be unknown, got %q", got)
	}
}
