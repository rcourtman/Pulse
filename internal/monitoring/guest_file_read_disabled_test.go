package monitoring

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestGuestFileReadDisabledEvidenceBoundary(t *testing.T) {
	t.Parallel()
	now := time.Now()
	key := guestMetadataCacheKey("pve", "node", 105)
	for _, tc := range []struct {
		name  string
		entry guestMetadataCacheEntry
		want  bool
	}{
		{"current", guestMetadataCacheEntry{fileReadDisabled: true, agentInfoObservedAt: now}, true},
		{"just-inside", guestMetadataCacheEntry{fileReadDisabled: true, agentInfoObservedAt: now.Add(-vmAgentMemCleanupMaxAge + time.Nanosecond)}, true},
		{"expired", guestMetadataCacheEntry{fileReadDisabled: true, agentInfoObservedAt: now.Add(-vmAgentMemCleanupMaxAge)}, false},
		{"future", guestMetadataCacheEntry{fileReadDisabled: true, agentInfoObservedAt: now.Add(time.Nanosecond)}, false},
		{"missing-origin", guestMetadataCacheEntry{fileReadDisabled: true, fetchedAt: now}, false},
		{"enabled", guestMetadataCacheEntry{fileReadDisabled: false, agentInfoObservedAt: now}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{key: tc.entry}}
			if got := m.guestFileReadDisabled("pve", "node", 105, now); got != tc.want {
				t.Fatalf("guestFileReadDisabled = %t, want %t", got, tc.want)
			}
			for _, other := range []struct {
				instance, node string
				vmid           int
			}{{"other", "node", 105}, {"pve", "other", 105}, {"pve", "node", 106}} {
				if m.guestFileReadDisabled(other.instance, other.node, other.vmid, now) {
					t.Fatal("file-read disabled evidence crossed guest identity")
				}
			}
		})
	}
}

func TestGuestFileReadDisabledSkipsMeminfoAndPreventsCooldownLoop(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		t.Run(kind, func(t *testing.T) {
			var fileReadEnabled atomic.Bool
			var fileReadCalls, fsInfoCalls, infoCalls atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/nodes"):
					fmt.Fprint(w, `{"data":[]}`)
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{}}`)
				case strings.HasSuffix(r.URL.Path, "/agent/file-read"):
					fileReadCalls.Add(1)
					if !fileReadEnabled.Load() {
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, `{"data":null,"message":"VM 105 qmp command 'guest-file-open' failed - The command guest-file-open has been disabled for this instance\n"}`)
						return
					}
					fmt.Fprint(w, `{"data":{"content":"MemTotal: 8388608 kB\nMemAvailable: 5242880 kB\n","truncated":false}}`)
				case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
					fsInfoCalls.Add(1)
					fmt.Fprint(w, `{"data":{"result":[{"name":"vda1","mountpoint":"/","type":"ext4","total-bytes":1000000000,"used-bytes":300000000}]}}`)
				case strings.HasSuffix(r.URL.Path, "/agent/network-get-interfaces"):
					fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","hardware-address":"02:00:00:00:00:10","ip-addresses":[{"ip-address":"192.0.2.10","ip-address-type":"ipv4","prefix":24}]}]}}`)
				case strings.HasSuffix(r.URL.Path, "/agent/get-osinfo"):
					fmt.Fprint(w, `{"data":{"result":{"id":"debian","name":"Debian GNU/Linux","version-id":"12"}}}`)
				case strings.HasSuffix(r.URL.Path, "/agent/info"):
					infoCalls.Add(1)
					enabled := fileReadEnabled.Load()
					fmt.Fprintf(w, `{"data":{"result":{"version":"10.1.0","supported_commands":[{"name":"guest-ping","enabled":true},{"name":"guest-get-fsinfo","enabled":true},{"name":"guest-file-open","enabled":%t},{"name":"guest-file-read","enabled":%t}]}}}`, enabled, enabled)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			client := windowsBootstrapClient(t, server.URL, kind)
			m := windowsBootstrapMonitor(t)
			m.guestMetadataLimiter = make(map[string]time.Time)
			status := &proxmox.VMStatus{
				MaxMem: 8 * 1024 * 1024 * 1024,
				Mem:    4 * 1024 * 1024 * 1024,
				Agent:  proxmox.VMAgentField{Value: 1},
			}

			// Step 1: If meminfo runs before /agent/info, disabled guest-file-open must
			// neither return ErrGuestAgentDeferred nor trigger cooldown for get-fsinfo.
			_, err := m.getVMAgentMemoryAvailability(context.Background(), client, "pve", "node", 105)
			if err == nil {
				t.Fatal("expected error when guest-file-open is disabled")
			}
			if errors.Is(err, proxmox.ErrGuestAgentDeferred) {
				t.Fatalf("disabled guest-file-open returned ErrGuestAgentDeferred: %v", err)
			}
			if !proxmox.IsGuestAgentCommandDisabled(err) {
				t.Fatalf("expected IsGuestAgentCommandDisabled(err)=true: %v", err)
			}
			if got := fileReadCalls.Load(); got != 1 {
				t.Fatalf("file-read calls = %d, want 1", got)
			}

			// get-fsinfo succeeds immediately with no agent-cooldown fence.
			fs, err := client.GetVMFSInfo(context.Background(), "node", 105)
			if err != nil || len(fs) != 1 || fsInfoCalls.Load() != 1 {
				t.Fatalf("GetVMFSInfo after disabled file-read = %+v, %v (calls=%d)", fs, err, fsInfoCalls.Load())
			}

			// Step 2: Even after known-Linux negative TTL (30s) expires, commandDisabled
			// keeps long 30m backoff instead of probing every 30s.
			memKey := guestMemoryCacheKey("pve", "node", 105)
			m.rrdCacheMu.Lock()
			entry := m.vmAgentMemCache[memKey]
			entry.fetchedAt = time.Now().Add(-vmAgentMemNegativeKnownGuestTTL - time.Second)
			m.vmAgentMemCache[memKey] = entry
			m.rrdCacheMu.Unlock()

			// Populate Linux metadata (with guest-file-open disabled in /agent/info).
			_, _, osName, _, agentVersion, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "pve", "node", "linux-vm", 105, status, false)
			if deferred || osName != "Debian GNU/Linux" || agentVersion != "10.1.0" || infoCalls.Load() != 1 {
				t.Fatalf("fetchGuestAgentMetadata = os=%q ver=%q deferred=%t infoCalls=%d", osName, agentVersion, deferred, infoCalls.Load())
			}

			// Step 3: With /agent/info reporting guest-file-* disabled, resolveGuestStatusMemory
			// skips /agent/file-read completely (even if vmAgentMemCache is empty) and does not defer.
			m.rrdCacheMu.Lock()
			delete(m.vmAgentMemCache, memKey)
			m.rrdCacheMu.Unlock()

			total, used, source, memDeferred := m.resolveGuestStatusMemory(
				context.Background(), client, "pve", "linux-vm", "node", 105, "pve:node:105",
				status, nil, status.MaxMem, "", &VMMemoryRaw{},
			)
			if memDeferred || total != status.MaxMem || used != status.Mem || source != "status-mem" {
				t.Fatalf("resolveGuestStatusMemory = total=%d used=%d source=%q deferred=%t", total, used, source, memDeferred)
			}
			if got := fileReadCalls.Load(); got != 1 {
				t.Fatalf("file-read should be skipped after /agent/info disabled report, calls = %d", got)
			}

			// Step 4: If the operator re-enables guest-file-* and /agent/info refreshes,
			// meminfo reads resume cleanly.
			fileReadEnabled.Store(true)
			metaKey := guestMetadataCacheKey("pve", "node", 105)
			m.guestMetadataMu.Lock()
			meta := m.guestMetadataCache[metaKey]
			meta.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
			m.guestMetadataCache[metaKey] = meta
			m.guestMetadataMu.Unlock()
			m.guestMetadataLimiterMu.Lock()
			m.guestMetadataLimiter[metaKey] = time.Now().Add(-time.Second)
			m.guestMetadataLimiterMu.Unlock()

			_, _, _, _, _, deferred = m.fetchGuestAgentMetadata(context.Background(), client, "pve", "node", "linux-vm", 105, status, false)
			if deferred || infoCalls.Load() != 2 {
				t.Fatalf("refreshed fetchGuestAgentMetadata deferred=%t infoCalls=%d", deferred, infoCalls.Load())
			}

			info, err := m.getVMAgentMemoryAvailability(context.Background(), client, "pve", "node", 105)
			if err != nil || info.Source != "meminfo-available" || fileReadCalls.Load() != 2 {
				t.Fatalf("recovered getVMAgentMemoryAvailability = %+v, %v (fileReadCalls=%d)", info, err, fileReadCalls.Load())
			}
		})
	}
}
