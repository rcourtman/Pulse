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
		name      string
		raw       string
		want      string
		wantError string
	}{
		{
			name: "normalizes scheme host and trailing slash",
			raw:  "HTTPS://Pulse.Example.com:7655/base/",
			want: "https://pulse.example.com:7655/base",
		},
		{
			name: "keeps localhost",
			raw:  "http://localhost:7655/",
			want: "http://localhost:7655",
		},
		{
			name:      "missing scheme",
			raw:       "pulse.example.com",
			wantError: "must include http:// or https://",
		},
		{
			name:      "unsupported scheme",
			raw:       "ftp://pulse.example.com",
			wantError: "unsupported scheme",
		},
		{
			name:      "invalid port range",
			raw:       "https://pulse.example.com:70000",
			wantError: "invalid port",
		},
		{
			name:      "userinfo disallowed",
			raw:       "https://user:pass@pulse.example.com",
			wantError: "userinfo is not supported",
		},
		{
			name:      "query disallowed",
			raw:       "https://pulse.example.com?x=1",
			wantError: "query parameters are not supported",
		},
		{
			name:      "fragment disallowed",
			raw:       "https://pulse.example.com#frag",
			wantError: "fragments are not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePulseURL(tt.raw)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("normalizePulseURL(%q) expected error", tt.raw)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("normalizePulseURL(%q) error = %q, want substring %q", tt.raw, err.Error(), tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizePulseURL(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("normalizePulseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNew_InvalidPulseURLFailsFast(t *testing.T) {
	_, err := New(Config{
		PulseURL: "ftp://pulse.local",
		APIToken: "token",
		LogLevel: zerolog.Disabled,
	})
	if err == nil {
		t.Fatal("expected New to fail for invalid pulse URL")
	}
	if !strings.Contains(err.Error(), "invalid pulse URL") {
		t.Fatalf("error = %q, want invalid pulse URL context", err.Error())
	}
	if !strings.Contains(err.Error(), "unsupported scheme") {
		t.Fatalf("error = %q, want unsupported scheme detail", err.Error())
	}
}
