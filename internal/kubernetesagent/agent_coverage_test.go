package kubernetesagent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestNew_RejectsMissingToken(t *testing.T) {
	agent, err := New(Config{})
	if err == nil {
		t.Fatalf("expected error, got agent %+v", agent)
	}
	if !strings.Contains(err.Error(), "api token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_AppliesDefaultsAndInsecureTLS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"gitVersion":"v1.28.4"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmp := t.TempDir()
	kubeconfigPath := filepath.Join(tmp, "config")
	writeTestKubeconfig(t, kubeconfigPath, server.URL, "ctx-defaults")

	agent, err := New(Config{
		APIToken:           "token",
		KubeconfigPath:     kubeconfigPath,
		InsecureSkipVerify: true,
		LogLevel:           zerolog.Disabled,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if agent.interval != defaultInterval {
		t.Fatalf("interval = %v, want %v", agent.interval, defaultInterval)
	}
	if agent.cfg.MaxPods != defaultMaxPods {
		t.Fatalf("MaxPods = %d, want %d", agent.cfg.MaxPods, defaultMaxPods)
	}
	if agent.pulseURL != "http://localhost:7655" {
		t.Fatalf("pulseURL = %q, want default", agent.pulseURL)
	}
	if agent.agentVersion != Version {
		t.Fatalf("agentVersion = %q, want %q", agent.agentVersion, Version)
	}
	if agent.agentID != agent.clusterID {
		t.Fatalf("agentID = %q, want clusterID %q", agent.agentID, agent.clusterID)
	}

	transport, ok := agent.httpClient.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected insecure TLS config, got %#v", agent.httpClient.Transport)
	}

	redirectURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse redirect URL: %v", err)
	}
	redirectErr := agent.httpClient.CheckRedirect(&http.Request{URL: redirectURL}, nil)
	if redirectErr == nil || !strings.Contains(redirectErr.Error(), "server returned redirect") {
		t.Fatalf("unexpected redirect error: %v", redirectErr)
	}
}

func TestBuildRESTConfig_InvalidExplicitPath(t *testing.T) {
	_, _, err := buildRESTConfig(filepath.Join(t.TempDir(), "missing-config"), "")
	if err == nil {
		t.Fatal("expected error for missing kubeconfig path")
	}
	if !strings.Contains(err.Error(), "load kubeconfig") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunOnce_CollectReportErrorSkipsBuffering(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("nodes unavailable")
	})

	agent := &Agent{
		logger:       zerolog.New(io.Discard),
		kubeClient:   clientset,
		reportBuffer: buffer.New[agentsk8s.Report](3),
	}

	agent.runOnce(context.Background())
	if _, ok := agent.reportBuffer.Peek(); ok {
		t.Fatal("expected buffer to stay empty when report collection fails")
	}
}

func TestCollectPods_IncludeAllPodsAndWildcardNamespaces(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod-app", Name: "ok"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod-secret", Name: "skip"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "dev-app", Name: "skip2"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
	)

	agent := &Agent{
		cfg: Config{
			IncludeAllPods: true,
			MaxPods:        10,
		},
		kubeClient:        clientset,
		includeNamespaces: []string{"prod-*"},
		excludeNamespaces: []string{"prod-secret*"},
	}

	pods, err := agent.collectPods(context.Background())
	if err != nil {
		t.Fatalf("collectPods: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod after wildcard filters, got %d", len(pods))
	}
	if pods[0].Namespace != "prod-app" || pods[0].Name != "ok" {
		t.Fatalf("unexpected pod: %+v", pods[0])
	}
}

func TestSendReport_Non2xxWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	agent := &Agent{
		cfg:          Config{APIToken: "token"},
		httpClient:   server.Client(),
		pulseURL:     server.URL,
		agentVersion: "v1",
	}

	err := agent.sendReport(context.Background(), agentsk8s.Report{Timestamp: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pulse responded with status 502 Bad Gateway") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscoverClusterMetadata_Error(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	discovery := clientset.Discovery().(*fakediscovery.FakeDiscovery)
	discovery.PrependReactor("get", "version", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("version unavailable")
	})

	agent := &Agent{kubeClient: clientset, clusterVersion: "v-existing"}
	err := agent.discoverClusterMetadata(context.Background())
	if err == nil {
		t.Fatal("expected metadata discovery to fail")
	}
	if !strings.Contains(err.Error(), "version unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsNodeReadyFalseWithoutTrueCondition(t *testing.T) {
	node := corev1.Node{
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}},
		},
	}
	if isNodeReady(node) {
		t.Fatal("expected node without Ready=True condition to be not ready")
	}
}
