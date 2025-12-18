package kubernetesagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildRESTConfig_ExplicitKubeconfig(t *testing.T) {
	tmp := t.TempDir()
	kubeconfigPath := filepath.Join(tmp, "config")

	kubeconfig := `
apiVersion: v1
kind: Config
clusters:
- name: c1
  cluster:
    server: https://k8s.example.invalid
contexts:
- name: ctx1
  context:
    cluster: c1
    user: u1
- name: ctx2
  context:
    cluster: c1
    user: u1
current-context: ctx1
users:
- name: u1
  user:
    token: test
`
	if err := os.WriteFile(kubeconfigPath, []byte(strings.TrimSpace(kubeconfig)), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	restCfg, ctxName, err := buildRESTConfig(kubeconfigPath, "ctx2")
	if err != nil {
		t.Fatalf("buildRESTConfig: %v", err)
	}
	if restCfg.Host != "https://k8s.example.invalid" {
		t.Fatalf("restCfg.Host = %q", restCfg.Host)
	}
	if ctxName != "ctx2" {
		t.Fatalf("contextName = %q", ctxName)
	}
}

func TestNamespaceAllowed_IncludeExclude(t *testing.T) {
	a := &Agent{
		includeNamespaces: []string{"a", "b"},
		excludeNamespaces: []string{"b"},
	}

	if !a.namespaceAllowed("a") {
		t.Fatalf("expected namespace a allowed")
	}
	if a.namespaceAllowed("b") {
		t.Fatalf("expected namespace b excluded")
	}
	if a.namespaceAllowed("") {
		t.Fatalf("expected empty namespace disallowed")
	}
}

func TestRolesFromNodeLabels(t *testing.T) {
	roles := rolesFromNodeLabels(map[string]string{
		"node-role.kubernetes.io/master": "",
		"kubernetes.io/role":             "worker",
	})
	if len(roles) != 2 || roles[0] != "master" || roles[1] != "worker" {
		t.Fatalf("unexpected roles: %+v", roles)
	}
}

func TestCollectPods_FiltersProblemsAndSorts(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "ok"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "c", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "pending"},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "b", Name: "not-ready"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "c", Ready: false, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "b", Name: "failed"},
			Status:     corev1.PodStatus{Phase: corev1.PodFailed, Reason: "CrashLoopBackOff"},
		},
	)

	a := &Agent{
		cfg: Config{
			MaxPods:        2,
			IncludeAllPods: false,
		},
		kubeClient:        clientset,
		includeNamespaces: nil,
		excludeNamespaces: nil,
	}

	pods, err := a.collectPods(context.Background())
	if err != nil {
		t.Fatalf("collectPods: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("expected MaxPods=2, got %d (%+v)", len(pods), pods)
	}
	if pods[0].Namespace != "a" || pods[0].Name != "pending" {
		t.Fatalf("unexpected first pod: %+v", pods[0])
	}
	if pods[1].Namespace != "b" {
		t.Fatalf("unexpected second pod: %+v", pods[1])
	}
}

func TestCollectDeployments_FiltersProblems(t *testing.T) {
	replicas := int32(3)
	okReplicas := int32(2)

	clientset := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "bad"},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "ok"},
			Spec:       appsv1.DeploymentSpec{Replicas: &okReplicas},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "disabled"},
			Spec:       appsv1.DeploymentSpec{Replicas: nil},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: 0, ReadyReplicas: 0, UpdatedReplicas: 0},
		},
	)

	a := &Agent{
		cfg:               Config{IncludeAllPods: false},
		kubeClient:        clientset,
		includeNamespaces: nil,
		excludeNamespaces: nil,
	}

	deps, err := a.collectDeployments(context.Background())
	if err != nil {
		t.Fatalf("collectDeployments: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 deployment, got %d (%+v)", len(deps), deps)
	}
	if deps[0].Name != "bad" {
		t.Fatalf("unexpected deployment: %+v", deps[0])
	}
}

func TestCollectNodes_MapsReadyRolesAndResources(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "n1",
				UID:  "uid1",
				Labels: map[string]string{
					"node-role.kubernetes.io/master": "",
				},
			},
			Spec: corev1.NodeSpec{Unschedulable: true},
			Status: corev1.NodeStatus{
				NodeInfo: corev1.NodeSystemInfo{
					KubeletVersion:          "v1.30.0",
					ContainerRuntimeVersion: "containerd://1.7.0",
					OSImage:                 "linux",
					KernelVersion:           "6.0",
					Architecture:            "amd64",
				},
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("3"),
					corev1.ResourceMemory: resource.MustParse("7Gi"),
					corev1.ResourcePods:   resource.MustParse("100"),
				},
			},
		},
	)

	a := &Agent{kubeClient: clientset}
	nodes, err := a.collectNodes(context.Background())
	if err != nil {
		t.Fatalf("collectNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if !n.Ready || !n.Unschedulable {
		t.Fatalf("unexpected ready/unschedulable: %+v", n)
	}
	if n.Capacity.CPUCores != 4 || n.Allocatable.CPUCores != 3 {
		t.Fatalf("unexpected cpu: %+v", n)
	}
	if len(n.Roles) != 1 || n.Roles[0] != "master" {
		t.Fatalf("unexpected roles: %+v", n.Roles)
	}
}

func TestSendReport_SetsHeadersAndHandlesStatus(t *testing.T) {
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents/kubernetes/report" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-API-Token") != "token" {
			t.Fatalf("X-API-Token = %q", r.Header.Get("X-API-Token"))
		}
		if r.Header.Get("User-Agent") != reportUserAgent+"1.2.3" {
			t.Fatalf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		sawAuth = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := &Agent{
		cfg:         Config{APIToken: "token"},
		httpClient:  server.Client(),
		pulseURL:    server.URL,
		agentVersion: "1.2.3",
	}

	if err := a.sendReport(context.Background(), agentsk8s.Report{Timestamp: time.Now().UTC()}); err != nil {
		t.Fatalf("sendReport: %v", err)
	}
	if !sawAuth {
		t.Fatalf("expected server to receive request")
	}
}

