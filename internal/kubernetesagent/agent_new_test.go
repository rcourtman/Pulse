package kubernetesagent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func writeTestKubeconfig(t *testing.T, path, serverURL, contextName string) {
	t.Helper()
	kubeconfig := fmt.Sprintf(`
apiVersion: v1
kind: Config
clusters:
- name: c1
  cluster:
    server: %s
contexts:
- name: %s
  context:
    cluster: c1
    user: u1
current-context: %s
users:
- name: u1
  user:
    token: test
`, serverURL, contextName, contextName)

	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
}

func TestBuildRESTConfig_DefaultKubeconfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"gitVersion":"v1.25.0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmp := t.TempDir()
	kubeconfigPath := filepath.Join(tmp, "config")
	writeTestKubeconfig(t, kubeconfigPath, server.URL, "ctx-default")

	t.Setenv("KUBECONFIG", kubeconfigPath)

	restCfg, ctxName, err := buildRESTConfig("", "")
	if err != nil {
		t.Fatalf("buildRESTConfig: %v", err)
	}
	if restCfg.Host != server.URL {
		t.Fatalf("restCfg.Host = %q, want %q", restCfg.Host, server.URL)
	}
	if ctxName != "ctx-default" {
		t.Fatalf("contextName = %q, want ctx-default", ctxName)
	}
}

func TestNew_WithKubeconfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"gitVersion":"v1.26.1"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmp := t.TempDir()
	kubeconfigPath := filepath.Join(tmp, "config")
	writeTestKubeconfig(t, kubeconfigPath, server.URL, "ctx-test")

	agent, err := New(Config{
		PulseURL:       "http://pulse.local",
		APIToken:       "token",
		KubeconfigPath: kubeconfigPath,
		KubeContext:    "ctx-test",
		LogLevel:       zerolog.Disabled,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if agent.clusterServer != server.URL {
		t.Fatalf("clusterServer = %q, want %q", agent.clusterServer, server.URL)
	}
	if agent.clusterContext != "ctx-test" {
		t.Fatalf("clusterContext = %q, want ctx-test", agent.clusterContext)
	}
	if agent.clusterVersion != "v1.26.1" {
		t.Fatalf("clusterVersion = %q, want v1.26.1", agent.clusterVersion)
	}
	if agent.agentID == "" || agent.clusterID == "" {
		t.Fatalf("expected non-empty agent and cluster IDs")
	}
}
