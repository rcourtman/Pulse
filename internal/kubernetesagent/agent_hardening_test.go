package kubernetesagent

import (
	"context"
	"errors"
	"strings"
	"testing"

	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCollectPods_RetriesOnTransientKubernetesErrors(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "pending-1"},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
	)

	attempts := 0
	clientset.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		if attempts == 1 {
			return true, nil, apierrors.NewTooManyRequests("rate limited", 1)
		}
		return false, nil, nil
	})

	a := &Agent{cfg: Config{MaxPods: 10}, kubeClient: clientset}
	pods, err := a.collectPods(context.Background())
	if err != nil {
		t.Fatalf("collectPods: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", attempts)
	}
	if len(pods) != 1 || pods[0].Name != "pending-1" {
		t.Fatalf("unexpected pods: %+v", pods)
	}
}

func TestCollectPods_ClassifiesRBACErrors(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("forbidden"))
	})

	a := &Agent{cfg: Config{MaxPods: 5}, kubeClient: clientset}
	_, err := a.collectPods(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "RBAC") {
		t.Fatalf("expected RBAC guidance in error, got: %v", err)
	}
}

func TestCollectPods_ListsExplicitNamespacesOnly(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "p1"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "team-b", Name: "p2"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "team-c", Name: "p3"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
	)

	seenNamespaces := map[string]int{}
	clientset.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		listAction, ok := action.(k8stesting.ListAction)
		if ok {
			seenNamespaces[listAction.GetNamespace()]++
		}
		return false, nil, nil
	})

	a := &Agent{
		cfg:               Config{MaxPods: 20, IncludeAllPods: true},
		kubeClient:        clientset,
		includeNamespaces: []string{"team-a", "team-b"},
	}

	pods, err := a.collectPods(context.Background())
	if err != nil {
		t.Fatalf("collectPods: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods from explicit include namespaces, got %d (%+v)", len(pods), pods)
	}
	if seenNamespaces[metav1.NamespaceAll] > 0 {
		t.Fatalf("expected explicit namespace listing, saw cluster-wide list action: %+v", seenNamespaces)
	}
	if seenNamespaces["team-a"] == 0 || seenNamespaces["team-b"] == 0 {
		t.Fatalf("expected list calls for team-a and team-b, saw %+v", seenNamespaces)
	}
}

func TestInsertPodSortedBounded_KeepsLexicographicallySmallest(t *testing.T) {
	inputs := []agentsk8s.Pod{
		{Namespace: "ops", Name: "zeta"},
		{Namespace: "default", Name: "mid"},
		{Namespace: "default", Name: "alpha"},
		{Namespace: "zzz", Name: "last"},
	}

	items := make([]agentsk8s.Pod, 0, 2)
	for _, pod := range inputs {
		items = insertPodSortedBounded(items, pod, 2)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(items))
	}
	if items[0].Namespace != "default" || items[0].Name != "alpha" {
		t.Fatalf("unexpected first pod: %+v", items[0])
	}
	if items[1].Namespace != "default" || items[1].Name != "mid" {
		t.Fatalf("unexpected second pod: %+v", items[1])
	}
}
