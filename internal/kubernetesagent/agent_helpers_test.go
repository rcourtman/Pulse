package kubernetesagent

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsProblemPod(t *testing.T) {
	if !isProblemPod(corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}) {
		t.Fatal("expected pending pod to be a problem")
	}
	if !isProblemPod(corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}}) {
		t.Fatal("expected failed pod to be a problem")
	}
	if !isProblemPod(corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodUnknown}}) {
		t.Fatal("expected unknown pod to be a problem")
	}

	okPod := corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready: true,
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
		},
	}
	if isProblemPod(okPod) {
		t.Fatal("expected healthy running pod to be non-problem")
	}

	waitingPod := corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
					},
				},
			},
		},
	}
	if !isProblemPod(waitingPod) {
		t.Fatal("expected waiting container to be a problem")
	}

	initFailedPod := corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
					},
				},
			},
		},
	}
	if !isProblemPod(initFailedPod) {
		t.Fatal("expected failed init container to be a problem")
	}
}

func TestSummarizeContainerState(t *testing.T) {
	status, reason, message := summarizeContainerState(corev1.ContainerStatus{
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	})
	if status != "running" || reason != "" || message != "" {
		t.Fatalf("unexpected running summary: %s %s %s", status, reason, message)
	}

	status, reason, message = summarizeContainerState(corev1.ContainerStatus{
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: " waiting "},
		},
	})
	if status != "waiting" || reason != "CrashLoopBackOff" || message != "waiting" {
		t.Fatalf("unexpected waiting summary: %s %s %s", status, reason, message)
	}

	status, reason, message = summarizeContainerState(corev1.ContainerStatus{
		State: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{Reason: "Error", Message: " boom "},
		},
	})
	if status != "terminated" || reason != "Error" || message != "boom" {
		t.Fatalf("unexpected terminated summary: %s %s %s", status, reason, message)
	}

	status, reason, message = summarizeContainerState(corev1.ContainerStatus{})
	if status != "unknown" || reason != "" || message != "" {
		t.Fatalf("unexpected unknown summary: %s %s %s", status, reason, message)
	}
}

func TestOwnerRef(t *testing.T) {
	controller := true
	refs := []metav1.OwnerReference{
		{Kind: "ReplicaSet", Name: "rs-1"},
		{Kind: "Deployment", Name: "deploy-1", Controller: &controller},
	}
	kind, name := ownerRef(refs)
	if kind != "Deployment" || name != "deploy-1" {
		t.Fatalf("expected controller ref, got %s %s", kind, name)
	}

	kind, name = ownerRef([]metav1.OwnerReference{
		{Kind: "Job", Name: "job-1"},
	})
	if kind != "Job" || name != "job-1" {
		t.Fatalf("expected first ref, got %s %s", kind, name)
	}

	kind, name = ownerRef(nil)
	if kind != "" || name != "" {
		t.Fatalf("expected empty ref, got %s %s", kind, name)
	}
}
