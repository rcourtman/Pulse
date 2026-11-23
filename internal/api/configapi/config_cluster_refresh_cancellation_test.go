package configapi

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const refreshPath = "/api/config/nodes/pve-0/refresh-cluster"

func refreshTestConfig(t *testing.T, host, fingerprint string) *config.Config {
	t.Helper()
	cfg := &config.Config{DataPath: t.TempDir(), PVEInstances: []config.PVEInstance{{
		Name: "fixture-pve", Host: host, TokenName: "fixture@pve!readonly", TokenValue: "synthetic",
		VerifySSL: true, VerifySSLExplicit: true, Fingerprint: fingerprint,
		IsCluster: true, ClusterName: "old-cluster",
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "alpha", NativeNodeID: 1, Host: "https://alpha:8006", GuestURL: "https://alpha-console.invalid"},
			{NodeName: "beta", NativeNodeID: 2, Host: "https://beta:8006", IPOverride: "127.0.0.1"},
			{NodeName: "gamma", NativeNodeID: 3, Host: "https://gamma:8006"},
		},
	}}}
	config.EnsurePVEClusterNodeIdentities(cfg.PVEInstances)
	return cfg
}

func awaitRefreshFixture(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("waiting for %s", what)
	}
}

func TestClusterRefreshDisconnectStopsDiscovery(t *testing.T) {
	for _, holdPath := range []string{"/api2/json/cluster/status", "/api2/json/nodes"} {
		t.Run(holdPath, func(t *testing.T) {
			entered, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var held atomic.Bool
			pve := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == holdPath && held.CompareAndSwap(false, true) {
					close(entered)
					select {
					case <-r.Context().Done():
						close(stopped)
					case <-release:
					}
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api2/json/cluster/status":
					_, _ = io.WriteString(w, `{"data":[{"type":"cluster","name":"renamed-cluster"},
					  {"type":"node","name":"alpha","id":"node/alpha","ip":"127.0.0.1","nodeid":1,"online":1},
					  {"type":"node","name":"beta","id":"node/beta","ip":"127.0.0.1","nodeid":2,"online":1},
					  {"type":"node","name":"gamma","id":"node/gamma","ip":"127.0.0.1","nodeid":3,"online":1}]}`)
				case "/api2/json/nodes":
					_, _ = io.WriteString(w, `{"data":[{"node":"alpha","status":"online"}]}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer pve.Close()
			defer close(release)
			cfg := refreshTestConfig(t, pve.URL, fmt.Sprintf("%x", sha256.Sum256(pve.Certificate().Raw)))
			persistence := config.NewConfigPersistence(cfg.DataPath)
			if err := persistence.SaveNodesConfig(cfg.PVEInstances, nil, nil); err != nil {
				t.Fatal(err)
			}
			nodesFile := filepath.Join(cfg.DataPath, "nodes.enc")
			saved, err := os.ReadFile(nodesFile)
			if err != nil {
				t.Fatal(err)
			}
			original := cfg.PVEInstances[0].DeepCopy()
			reloads := 0
			h := &ConfigHandlers{defaultConfig: cfg, defaultPersistence: persistence, reloadFunc: func() error {
				reloads++
				return nil
			}}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := httptest.NewRequest(http.MethodPost, refreshPath, nil).WithContext(ctx)
			done := make(chan struct{})
			go func() {
				defer close(done)
				h.HandleRefreshClusterNodes(httptest.NewRecorder(), req)
			}()
			t.Cleanup(func() { awaitRefreshFixture(t, done, "refresh shutdown") })
			awaitRefreshFixture(t, entered, "upstream request")
			cancel()
			awaitRefreshFixture(t, stopped, "upstream cancellation")
			awaitRefreshFixture(t, done, "cancelled refresh")

			after, err := os.ReadFile(nodesFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(saved) || !reflect.DeepEqual(cfg.PVEInstances, []config.PVEInstance{original}) || reloads != 0 {
				t.Fatal("cancelled discovery changed saved or live membership, or reloaded monitoring")
			}
		})
	}
}

func TestClusterRefreshSaveBoundary(t *testing.T) {
	for _, mode := range []string{"cancel before save", "failed save", "cancel after save"} {
		t.Run(mode, func(t *testing.T) {
			cfg := refreshTestConfig(t, "https://pve-fixture.invalid", "saved-pin")
			persistence := config.NewConfigPersistence(cfg.DataPath)
			var originalSaved []config.PVEInstance
			if mode == "failed save" {
				if err := os.Mkdir(filepath.Join(cfg.DataPath, "nodes.enc"), 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := persistence.SaveNodesConfig(cfg.PVEInstances, nil, nil); err != nil {
					t.Fatal(err)
				}
				nodes, err := persistence.LoadNodesConfig()
				if err != nil || nodes == nil || len(nodes.PVEInstances) != 1 {
					t.Fatalf("load initial saved connection: %v", err)
				}
				originalSaved = nodes.PVEInstances
			}
			original := cfg.PVEInstances[0].DeepCopy()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reloads := 0
			h := &ConfigHandlers{defaultConfig: cfg, defaultPersistence: persistence, reloadFunc: func() error {
				reloads++
				nodes, err := persistence.LoadNodesConfig()
				expected := originalSaved[0].DeepCopy()
				expected.ClusterName = "renamed-cluster"
				if err != nil || nodes == nil || !reflect.DeepEqual(nodes.PVEInstances, []config.PVEInstance{expected}) {
					t.Fatalf("reload did not see the complete saved rename: %v", err)
				}
				cancel() // A disconnect here must not undo the admitted save.
				return nil
			}}
			// Even a result that completed just as the client disconnected
			// must not be promoted to a save.
			restore := ConfigureAutoRegisterNetworkDependencies(func(_ proxmox.ClientConfig, _ string, endpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
				if mode == "cancel before save" {
					cancel()
				}
				return true, "renamed-cluster", endpoints
			}, nil)
			t.Cleanup(restore)

			rec := httptest.NewRecorder()
			h.HandleRefreshClusterNodes(rec, httptest.NewRequest(http.MethodPost, refreshPath, nil).WithContext(ctx))
			switch mode {
			case "cancel after save":
				expected := original
				expected.ClusterName = "renamed-cluster"
				if rec.Code != http.StatusOK || reloads != 1 || !reflect.DeepEqual(cfg.PVEInstances, []config.PVEInstance{expected}) {
					t.Fatal("an admitted save did not publish the preserved members and rename")
				}
			case "failed save":
				if rec.Code != http.StatusInternalServerError || reloads != 0 || !reflect.DeepEqual(cfg.PVEInstances, []config.PVEInstance{original}) {
					t.Fatal("a failed save changed the live connection or attempted reload")
				}
			case "cancel before save":
				nodes, err := persistence.LoadNodesConfig()
				if err != nil || nodes == nil || reloads != 0 || !reflect.DeepEqual(nodes.PVEInstances, originalSaved) ||
					!reflect.DeepEqual(cfg.PVEInstances, []config.PVEInstance{original}) {
					t.Fatalf("a cancelled result changed the connection or triggered a reload: %v", err)
				}
			}
		})
	}
}
