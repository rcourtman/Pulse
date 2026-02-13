package kubernetesagent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		PulseURL:       "https://pulse.local",
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

func TestNormalizePulseURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		want    string
		wantErr string
	}{
		{
			name:   "https canonicalized",
			rawURL: "HTTPS://pulse.example.com///",
			want:   "https://pulse.example.com",
		},
		{
			name:   "loopback http allowed",
			rawURL: "http://127.0.0.1:7655/",
			want:   "http://127.0.0.1:7655",
		},
		{
			name:    "non-loopback http rejected",
			rawURL:  "http://pulse.example.com",
			wantErr: "must use https unless host is loopback",
		},
		{
			name:    "credentials rejected",
			rawURL:  "https://user:pass@pulse.example.com",
			wantErr: "must not include user credentials",
		},
		{
			name:    "query rejected",
			rawURL:  "https://pulse.example.com?token=abc",
			wantErr: "must not include query or fragment",
		},
		{
			name:    "unsupported scheme rejected",
			rawURL:  "ws://pulse.example.com",
			wantErr: "unsupported scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePulseURL(tt.rawURL)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("normalizePulseURL(%q) expected error containing %q", tt.rawURL, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("normalizePulseURL(%q) error = %v, want contains %q", tt.rawURL, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizePulseURL(%q) returned error: %v", tt.rawURL, err)
			}
			if got != tt.want {
				t.Fatalf("normalizePulseURL(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestNew_RejectsUnsafePulseURL(t *testing.T) {
	_, err := New(Config{
		PulseURL: "http://pulse.example.com",
		APIToken: "token",
		LogLevel: zerolog.Disabled,
	})
	if err == nil {
		t.Fatal("expected New to reject non-loopback http Pulse URL")
	}
	if !strings.Contains(err.Error(), "must use https unless host is loopback") {
		t.Fatalf("unexpected error: %v", err)
	}
}
