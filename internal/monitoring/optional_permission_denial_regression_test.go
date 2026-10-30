package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Synthetic, completed HTTP refusals only; no real guest or token is used.
type optionalDenialFixture struct {
	status      atomic.Int32
	aptCalls    atomic.Int32
	memoryCalls atomic.Int32
}

func optionalDenialClient(t *testing.T, kind string, status int) (*optionalDenialFixture, PVEClientInterface) {
	t.Helper()
	f := &optionalDenialFixture{}
	f.status.Store(int32(status))
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
			t.Error("configured authentication changed")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			fmt.Fprint(w, `{"data":[]}`)
		case strings.HasSuffix(r.URL.Path, "/config"):
			// Honour the production guest-operation lock check.
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":8388608,"mem":8388608}}`)
		case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
			fmt.Fprint(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":1048576000,"used-bytes":314572800}]}}`)
		case strings.HasSuffix(r.URL.Path, "/agent/network-get-interfaces"):
			fmt.Fprint(w, `{"data":{"result":[]}}`)
		case strings.HasSuffix(r.URL.Path, "/agent/get-osinfo"):
			fmt.Fprint(w, `{"data":{"name":"Debian GNU/Linux","version":"fixture"}}`)
		case strings.HasSuffix(r.URL.Path, "/agent/info"):
			fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
		case strings.HasSuffix(r.URL.Path, "/apt/update"):
			f.aptCalls.Add(1)
			if code := int(f.status.Load()); code != http.StatusOK {
				w.WriteHeader(code)
				fmt.Fprint(w, "upstream API error 403: permission denied")
				return
			}
			fmt.Fprint(w, `{"data":[]}`)
		case strings.HasSuffix(r.URL.Path, "/agent/file-read"):
			f.memoryCalls.Add(1)
			if r.URL.Query().Get("file") != "/proc/meminfo" {
				t.Error("unexpected guest file")
			}
			if code := int(f.status.Load()); code != http.StatusOK {
				w.WriteHeader(code)
				fmt.Fprint(w, "upstream API error 403: permission denied")
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": "MemTotal: 8192 kB\nMemAvailable: 6144 kB\nMemFree: 2048 kB\n", "truncated": false}})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return f, windowsBootstrapClient(t, s.URL, kind)
}

func TestOptionalAPTDenialKeepsBackoffAndEvidence(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, prior := range []string{"none", "zero", "seven"} {
			t.Run(kind+"/"+prior, func(t *testing.T) {
				f, client := optionalDenialClient(t, kind, http.StatusForbidden)
				m := &Monitor{}
				original := time.Time{}
				count, state := 0, "unavailable"
				if prior != "none" {
					original = time.Now().Add(-pendingUpdatesCacheTTL - time.Minute)
					if prior == "seven" {
						count = 7
					}
					state = "stale"
					m.nodePendingUpdatesCache = map[string]pendingUpdatesCache{"lab-node": {count: count, checkedAt: original}}
				}
				for poll := 0; poll < 3; poll++ {
					node := &models.Node{}
					m.applyNodePendingUpdates(context.Background(), "lab", client, proxmox.Node{Node: "node"}, "lab-node", "online", node)
					if node.PendingUpdatesStatus != state || node.PendingUpdatesReason != "permission_denied" || node.PendingUpdates != count || !node.PendingUpdatesCheckedAt.Equal(original) {
						t.Errorf("poll %d: denial fabricated/renewed evidence: %+v", poll, node)
					}
				}
				if got := f.aptCalls.Load(); got != 1 {
					t.Errorf("three polls sent %d APT requests, want 1", got)
				}
			})
		}
	}
}

func TestOptionalMemoryDenialUsesLongBackoff(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		t.Run(kind, func(t *testing.T) {
			f, client := optionalDenialClient(t, kind, http.StatusForbidden)
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{guestMetadataCacheKey("lab", "node", 105): {osName: "Debian GNU/Linux"}}}
			key := guestMemoryCacheKey("lab", "node", 105)
			if info, err := m.getVMAgentMemoryAvailability(context.Background(), client, "lab", "node", 105); err == nil || info != (proxmox.LinuxMemoryAvailability{}) {
				t.Fatal("denied file-read became measured memory")
			}
			entry := m.vmAgentMemCache[key]
			entry.fetchedAt = time.Now().Add(-time.Minute)
			m.vmAgentMemCache[key] = entry // simulate elapsed polling time, not a new observation
			for i := 0; i < 2; i++ {
				if info, err := m.getVMAgentMemoryAvailability(context.Background(), client, "lab", "node", 105); err == nil || info != (proxmox.LinuxMemoryAvailability{}) {
					t.Error("cached denial became measured memory")
				}
			}
			if got := f.memoryCalls.Load(); got != 1 {
				t.Errorf("denial retried after 60 seconds: calls=%d, want 1", got)
			}
			if after := m.vmAgentMemCache[key]; !after.negative || !after.fetchedAt.Equal(entry.fetchedAt) {
				t.Error("backoff poll renewed the failed observation clock")
			}
		})
	}
}

func TestOptionalDenialCadenceControls(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		t.Run(kind+"/APT502quoting403", func(t *testing.T) {
			f, client := optionalDenialClient(t, kind, http.StatusBadGateway)
			m := &Monitor{}
			for i := 0; i < 3; i++ {
				m.applyNodePendingUpdates(context.Background(), "lab", client, proxmox.Node{Node: "node"}, "lab-node", "online", &models.Node{})
			}
			want := int32(3)
			if kind == "cluster-client" {
				want = 9 // The existing cluster transport retries each 502 three times.
			}
			if got := f.aptCalls.Load(); got != want {
				t.Errorf("non-403 backoff changed: calls=%d, want %d", got, want)
			}
		})
		t.Run(kind+"/memory400quoting403", func(t *testing.T) {
			f, client := optionalDenialClient(t, kind, http.StatusBadRequest)
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{guestMetadataCacheKey("lab", "node", 105): {osName: "Debian GNU/Linux"}}}
			key := guestMemoryCacheKey("lab", "node", 105)
			_, _ = m.getVMAgentMemoryAvailability(context.Background(), client, "lab", "node", 105)
			entry := m.vmAgentMemCache[key]
			entry.fetchedAt = time.Now().Add(-time.Minute)
			m.vmAgentMemCache[key] = entry
			f.status.Store(http.StatusOK)
			info, err := m.getVMAgentMemoryAvailability(context.Background(), client, "lab", "node", 105)
			if err != nil || info.EffectiveAvailable != 6144*1024 || f.memoryCalls.Load() != 2 {
				t.Errorf("non-403 recovery changed: %+v / %v / calls %d", info, err, f.memoryCalls.Load())
			}
		})
	}
}
