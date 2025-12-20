package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// ========================================
// truncateKubernetesMessage tests
// ========================================

func TestTruncateKubernetesMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short message",
			input:    "This is a short message",
			expected: "This is a short message",
		},
		{
			name:     "empty message",
			input:    "",
			expected: "",
		},
		{
			name:     "max length message",
			input:    string(make([]byte, maxKubernetesMessageLength)),
			expected: string(make([]byte, maxKubernetesMessageLength)),
		},
		{
			name:     "over max length",
			input:    string(make([]byte, maxKubernetesMessageLength+50)),
			expected: string(make([]byte, maxKubernetesMessageLength)) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateKubernetesMessage(tt.input)
			if result != tt.expected {
				if len(result) < 100 {
					t.Errorf("truncateKubernetesMessage() = %q, want %q", result, tt.expected)
				} else {
					t.Errorf("truncateKubernetesMessage() length = %d, want %d", len(result), len(tt.expected))
				}
			}
		})
	}
}

// ========================================
// formatKubernetesAge tests
// ========================================

func TestFormatKubernetesAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "0s",
		},
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "5 minutes",
			duration: 5 * time.Minute,
			expected: "5m",
		},
		{
			name:     "59 minutes",
			duration: 59 * time.Minute,
			expected: "59m",
		},
		{
			name:     "1 hour",
			duration: time.Hour,
			expected: "1h",
		},
		{
			name:     "23 hours",
			duration: 23 * time.Hour,
			expected: "23h",
		},
		{
			name:     "1 day",
			duration: 24 * time.Hour,
			expected: "1d",
		},
		{
			name:     "7 days",
			duration: 7 * 24 * time.Hour,
			expected: "7d",
		},
		{
			name:     "negative duration",
			duration: -5 * time.Second,
			expected: "0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatKubernetesAge(tt.duration)
			if result != tt.expected {
				t.Errorf("formatKubernetesAge(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

// ========================================
// isKubernetesPodHealthy tests
// ========================================

func TestIsKubernetesPodHealthy(t *testing.T) {
	tests := []struct {
		name     string
		pod      models.KubernetesPod
		expected bool
	}{
		{
			name:     "empty phase",
			pod:      models.KubernetesPod{Phase: ""},
			expected: false,
		},
		{
			name:     "pending phase",
			pod:      models.KubernetesPod{Phase: "Pending"},
			expected: false,
		},
		{
			name:     "running phase, no containers",
			pod:      models.KubernetesPod{Phase: "Running", Containers: nil},
			expected: true,
		},
		{
			name: "running phase, healthy container",
			pod: models.KubernetesPod{
				Phase: "Running",
				Containers: []models.KubernetesPodContainer{
					{Name: "app", Ready: true, State: "running"},
				},
			},
			expected: true,
		},
		{
			name: "running phase, not ready container",
			pod: models.KubernetesPod{
				Phase: "Running",
				Containers: []models.KubernetesPodContainer{
					{Name: "app", Ready: false, State: "running"},
				},
			},
			expected: false,
		},
		{
			name: "running phase, container not running",
			pod: models.KubernetesPod{
				Phase: "Running",
				Containers: []models.KubernetesPodContainer{
					{Name: "app", Ready: true, State: "waiting"},
				},
			},
			expected: false,
		},
		{
			name:     "failed phase",
			pod:      models.KubernetesPod{Phase: "Failed"},
			expected: false,
		},
		{
			name:     "succeeded phase",
			pod:      models.KubernetesPod{Phase: "Succeeded"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isKubernetesPodHealthy(tt.pod)
			if result != tt.expected {
				t.Errorf("isKubernetesPodHealthy() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ========================================
// kubernetesPodReason tests
// ========================================

func TestKubernetesPodReason(t *testing.T) {
	tests := []struct {
		name     string
		pod      models.KubernetesPod
		contains []string
	}{
		{
			name:     "empty pod",
			pod:      models.KubernetesPod{},
			contains: []string{},
		},
		{
			name:     "pod with phase",
			pod:      models.KubernetesPod{Phase: "Pending"},
			contains: []string{"phase=Pending"},
		},
		{
			name:     "pod with reason",
			pod:      models.KubernetesPod{Phase: "Failed", Reason: "OOMKilled"},
			contains: []string{"phase=Failed", "reason=OOMKilled"},
		},
		{
			name:     "pod with restarts",
			pod:      models.KubernetesPod{Phase: "Running", Restarts: 5},
			contains: []string{"restarts=5"},
		},
		{
			name: "pod with container issue",
			pod: models.KubernetesPod{
				Phase: "Running",
				Containers: []models.KubernetesPodContainer{
					{Name: "app", Ready: false, State: "waiting", Reason: "CrashLoopBackOff"},
				},
			},
			contains: []string{"containers=app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kubernetesPodReason(tt.pod)
			for _, expected := range tt.contains {
				if len(expected) > 0 && !kubeTestContains(result, expected) {
					t.Errorf("kubernetesPodReason() = %q, want to contain %q", result, expected)
				}
			}
		})
	}
}

func kubeTestContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ========================================
// isKubernetesDeploymentHealthy tests
// ========================================

func TestIsKubernetesDeploymentHealthy(t *testing.T) {
	tests := []struct {
		name       string
		deployment models.KubernetesDeployment
		expected   bool
	}{
		{
			name:       "empty deployment",
			deployment: models.KubernetesDeployment{},
			expected:   true, // 0/0 replicas is considered healthy
		},
		{
			name: "all replicas ready",
			deployment: models.KubernetesDeployment{
				DesiredReplicas:   3,
				ReadyReplicas:     3,
				AvailableReplicas: 3,
				UpdatedReplicas:   3,
			},
			expected: true,
		},
		{
			name: "some replicas not ready",
			deployment: models.KubernetesDeployment{
				DesiredReplicas:   3,
				ReadyReplicas:     2,
				AvailableReplicas: 3,
				UpdatedReplicas:   3,
			},
			expected: false,
		},
		{
			name: "no replicas ready",
			deployment: models.KubernetesDeployment{
				DesiredReplicas:   3,
				ReadyReplicas:     0,
				AvailableReplicas: 0,
				UpdatedReplicas:   0,
			},
			expected: false,
		},
		{
			name: "not all available",
			deployment: models.KubernetesDeployment{
				DesiredReplicas:   3,
				ReadyReplicas:     3,
				AvailableReplicas: 2,
				UpdatedReplicas:   3,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isKubernetesDeploymentHealthy(tt.deployment)
			if result != tt.expected {
				t.Errorf("isKubernetesDeploymentHealthy() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ========================================
// formatPodIssueLine tests
// ========================================

func TestFormatPodIssueLine(t *testing.T) {
	tests := []struct {
		name     string
		issue    podIssue
		expected string
	}{
		{
			name: "with namespace and name only",
			issue: podIssue{
				namespace: "default",
				name:      "nginx-pod",
			},
			expected: "default/nginx-pod",
		},
		{
			name: "with reason",
			issue: podIssue{
				namespace: "kube-system",
				name:      "coredns-pod",
				reason:    "CrashLoopBackOff",
			},
			expected: "kube-system/coredns-pod CrashLoopBackOff",
		},
		{
			name: "empty reason",
			issue: podIssue{
				namespace: "production",
				name:      "api-server",
				reason:    "",
			},
			expected: "production/api-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPodIssueLine(tt.issue)
			if result != tt.expected {
				t.Errorf("formatPodIssueLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ========================================
// formatPodIssues tests
// ========================================

func TestFormatPodIssues(t *testing.T) {
	issues := []podIssue{
		{namespace: "ns1", name: "pod1", reason: "reason1"},
		{namespace: "ns2", name: "pod2", reason: "reason2"},
		{namespace: "ns3", name: "pod3", reason: "reason3"},
		{namespace: "ns4", name: "pod4", reason: "reason4"},
	}

	t.Run("within limit", func(t *testing.T) {
		result := formatPodIssues(issues, 10)
		if len(result) != 4 {
			t.Errorf("Expected 4 lines, got %d", len(result))
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		result := formatPodIssues(issues, 2)
		if len(result) != 3 { // 2 issues + "... and X more"
			t.Errorf("Expected 3 lines (2 issues + truncation notice), got %d", len(result))
		}
		if !kubeTestContains(result[2], "and 2 more") {
			t.Errorf("Expected truncation notice, got %q", result[2])
		}
	})

	t.Run("empty issues", func(t *testing.T) {
		result := formatPodIssues([]podIssue{}, 10)
		if len(result) != 0 {
			t.Errorf("Expected 0 lines for empty issues, got %d", len(result))
		}
	})
}

// ========================================
// formatPodRestarts tests
// ========================================

func TestFormatPodRestarts(t *testing.T) {
	t.Run("empty issues", func(t *testing.T) {
		result := formatPodRestarts(nil, 10)
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("sorts by restarts descending", func(t *testing.T) {
		issues := []podIssue{
			{namespace: "ns1", name: "pod1", restarts: 5},
			{namespace: "ns2", name: "pod2", restarts: 10},
			{namespace: "ns3", name: "pod3", restarts: 2},
		}
		result := formatPodRestarts(issues, 10)
		if len(result) != 3 {
			t.Errorf("Expected 3 lines, got %d", len(result))
		}
		// First should be pod2 (10 restarts)
		if !kubeTestContains(result[0], "restarts=10") {
			t.Errorf("Expected first to have 10 restarts, got %q", result[0])
		}
	})

	t.Run("with reason", func(t *testing.T) {
		issues := []podIssue{
			{namespace: "ns1", name: "pod1", restarts: 5, reason: "OOMKilled"},
		}
		result := formatPodRestarts(issues, 10)
		if len(result) != 1 {
			t.Errorf("Expected 1 line, got %d", len(result))
		}
		if !kubeTestContains(result[0], "OOMKilled") {
			t.Errorf("Expected reason in output, got %q", result[0])
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		issues := []podIssue{
			{namespace: "ns1", name: "pod1", restarts: 10},
			{namespace: "ns2", name: "pod2", restarts: 8},
			{namespace: "ns3", name: "pod3", restarts: 6},
			{namespace: "ns4", name: "pod4", restarts: 4},
		}
		result := formatPodRestarts(issues, 2)
		if len(result) != 2 {
			t.Errorf("Expected 2 lines (limit), got %d", len(result))
		}
	})
}


