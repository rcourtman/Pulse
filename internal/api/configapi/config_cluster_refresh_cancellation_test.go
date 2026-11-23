package configapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
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

func TestClusterRefreshOrdersSameConfigSavesAndApplies(t *testing.T) {
	for _, next := range []string{"refresh", "edit", "cancelled refresh"} {
		t.Run(next, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cfg := refreshTestConfig(t, "https://one.example.test:8006", "")
				cfg.PVEInstances = append(cfg.PVEInstances, config.PVEInstance{
					Name: "other-connection", Host: "https://two.example.test:8006", IsCluster: true, ClusterName: "other-old",
					ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "delta", NativeNodeID: 1, Host: "https://delta.example.test:8006"}},
				})
				persistence := config.NewConfigPersistence(cfg.DataPath)
				if err := persistence.SaveNodesConfig(cfg.PVEInstances, nil, nil); err != nil {
					t.Fatal(err)
				}
				initial, err := persistence.LoadNodesConfig()
				if err != nil {
					t.Fatal(err)
				}
				cfg.PVEInstances = initial.PVEInstances
				initialOther := cfg.PVEInstances[1].DeepCopy()

				restore := ConfigureAutoRegisterNetworkDependencies(func(_ proxmox.ClientConfig, name string, endpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
					return true, name + "-new", endpoints
				}, nil)
				defer restore()

				firstRead, applyFirst := make(chan struct{}), make(chan struct{})
				var reloads atomic.Int32
				h := &ConfigHandlers{defaultConfig: cfg, defaultPersistence: persistence, reloadFunc: func() error {
					// Model the reload boundary: read the saved config, then
					// publish it after the previous monitor finishes stopping.
					first := reloads.Add(1) == 1
					saved, err := persistence.LoadNodesConfig()
					if first {
						close(firstRead)
						<-applyFirst
					}
					if err != nil {
						return err
					}
					config.Mu.Lock()
					cfg.PVEInstances = saved.PVEInstances
					config.Mu.Unlock()
					return nil
				}}
				run := func(req *http.Request, handle http.HandlerFunc) <-chan *httptest.ResponseRecorder {
					done := make(chan *httptest.ResponseRecorder, 1)
					go func() {
						rec := httptest.NewRecorder()
						handle(rec, req)
						done <- rec
					}()
					return done
				}
				first := run(httptest.NewRequest(http.MethodPost, refreshPath, nil), h.HandleRefreshClusterNodes)
				<-firstRead

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var second <-chan *httptest.ResponseRecorder
				const otherPath = "/api/config/nodes/pve:other-connection"
				if next == "edit" {
					second = run(httptest.NewRequest(http.MethodPut, otherPath,
						strings.NewReader(`{"guestURL":"https://console.example.test"}`)), h.HandleUpdateNode)
				} else {
					second = run(httptest.NewRequestWithContext(ctx, http.MethodPost, otherPath+"/refresh-cluster", nil), h.HandleRefreshClusterNodes)
				}
				// Both requests have run to completion or reached a blocking
				// boundary while the first apply still holds its old read.
				synctest.Wait()
				waiting := reloads.Load() == 1
				if next == "cancelled refresh" {
					cancel()
					synctest.Wait()
					select {
					case <-second:
					default:
						t.Error("cancelled request did not return while the older apply was still held")
					}
				}
				close(applyFirst)
				if rec := <-first; rec.Code != http.StatusOK {
					t.Fatalf("first refresh: %d %q", rec.Code, rec.Body)
				}
				if next != "cancelled refresh" {
					if rec := <-second; rec.Code != http.StatusOK {
						t.Fatalf("second %s: %d %q", next, rec.Code, rec.Body)
					}
				}

				wantOtherName, wantOtherURL, wantReloads := initialOther.ClusterName, initialOther.GuestURL, int32(2)
				switch next {
				case "refresh":
					wantOtherName = "other-connection-new"
				case "edit":
					wantOtherURL = "https://console.example.test"
				case "cancelled refresh":
					wantReloads = 1
				}
				saved, err := persistence.LoadNodesConfig()
				if err != nil || len(saved.PVEInstances) != 2 {
					t.Fatalf("load two saved connections: %v", err)
				}
				if saved.PVEInstances[0].ClusterName != "fixture-pve-new" ||
					saved.PVEInstances[1].ClusterName != wantOtherName || saved.PVEInstances[1].GuestURL != wantOtherURL {
					t.Errorf("saved inventory did not preserve both outcomes: first cluster=%q, second cluster=%q, second URL=%q",
						saved.PVEInstances[0].ClusterName, saved.PVEInstances[1].ClusterName, saved.PVEInstances[1].GuestURL)
				}
				rec := httptest.NewRecorder()
				h.HandleGetNodes(rec, httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil))
				var live []NodeResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &live); rec.Code != http.StatusOK || err != nil || len(live) != 2 {
					t.Fatalf("read back two live connections: status=%d, decode=%v", rec.Code, err)
				}
				if live[0].ClusterName != "fixture-pve-new" || live[1].ClusterName != wantOtherName || live[1].GuestURL != wantOtherURL {
					t.Errorf("an older apply erased the live outcome: first cluster=%q, second cluster=%q, second URL=%q",
						live[0].ClusterName, live[1].ClusterName, live[1].GuestURL)
				}
				if !waiting || reloads.Load() != wantReloads {
					t.Errorf("second request must wait until the first apply returns: waited=%v, reloads=%d, want %d", waiting, reloads.Load(), wantReloads)
				}
			})
		})
	}
}
