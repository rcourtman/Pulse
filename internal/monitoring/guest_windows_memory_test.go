package monitoring

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestGuestOSWindowsNameBoundary(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Microsoft Windows", "Microsoft Windows Server 2022", "Windows", " Windows 11 ", "MSWINDOWS"} {
		if !guestOSIsWindows(name) {
			t.Errorf("known Windows name %q not recognised", name)
		}
	}
	for _, name := range []string{"", "Linux", "Ubuntu", "FreeBSD", "Android", "Windowsill Linux", "Linux with Windows tools", "my-mswindows", "Microsoft Windowsish"} {
		if guestOSIsWindows(name) {
			t.Errorf("unrelated OS name %q suppressed meminfo", name)
		}
	}
}

func TestGuestWindowsMeminfoEvidenceBoundary(t *testing.T) {
	t.Parallel()
	now := time.Now()
	key := guestMetadataCacheKey("pve", "node", 105)
	for _, tc := range []struct {
		name  string
		entry guestMetadataCacheEntry
		want  bool
	}{
		{"current", guestMetadataCacheEntry{windowsGuest: true, osInfoObservedAt: now}, true},
		{"just-inside", guestMetadataCacheEntry{windowsGuest: true, osInfoObservedAt: now.Add(-vmAgentMemCleanupMaxAge + time.Nanosecond)}, true},
		{"expired", guestMetadataCacheEntry{windowsGuest: true, osInfoObservedAt: now.Add(-vmAgentMemCleanupMaxAge)}, false},
		{"future", guestMetadataCacheEntry{windowsGuest: true, osInfoObservedAt: now.Add(time.Nanosecond)}, false},
		{"missing-origin", guestMetadataCacheEntry{windowsGuest: true, fetchedAt: now}, false},
		{"display-only", guestMetadataCacheEntry{osName: "Microsoft Windows", fetchedAt: now}, false},
		{"renewed-network", guestMetadataCacheEntry{windowsGuest: true, osInfoObservedAt: now.Add(-2 * vmAgentMemCleanupMaxAge), fetchedAt: now}, false},
		{"linux", guestMetadataCacheEntry{osName: "Linux", osInfoObservedAt: now}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{key: tc.entry}}
			if got := m.guestWindowsMeminfoUnsupported("pve", "node", 105, now); got != tc.want {
				t.Fatalf("OS eligibility = %t, want %t", got, tc.want)
			}
			for _, identity := range []struct {
				instance, node string
				id             int
			}{{"other", "node", 105}, {"pve", "other", 105}, {"pve", "node", 106}} {
				if m.guestWindowsMeminfoUnsupported(identity.instance, identity.node, identity.id, now) {
					t.Fatal("Windows OS evidence crossed guest identity")
				}
			}
			if !reflect.DeepEqual(m.guestMetadataCache[key], tc.entry) {
				t.Fatal("eligibility read mutated OS evidence")
			}
		})
	}
}

func TestGuestWindowsMeminfoSkipsWithoutChangingMemoryCache(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"missing", "negative", "positive"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Now()
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{
				guestMetadataCacheKey("pve", "node", 105): {windowsGuest: true, osInfoObservedAt: now},
			}}
			key := guestMemoryCacheKey("pve", "node", 105)
			entry := agentMemCacheEntry{negative: true, fetchedAt: now.Add(-2 * vmAgentMemNegativeTTL)}
			if kind == "positive" {
				entry = agentMemCacheEntry{info: proxmox.LinuxMemoryAvailability{Source: "meminfo-available", EffectiveAvailable: 4096}, fetchedAt: now}
			}
			if kind != "missing" {
				m.vmAgentMemCache = map[string]agentMemCacheEntry{key: entry}
			}
			client := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memAvailable: 4096}
			for i := 0; i < 3; i++ {
				info, err := m.getVMAgentMemoryAvailability(context.Background(), client, "pve", "node", 105)
				if err == nil || errors.Is(err, proxmox.ErrGuestAgentDeferred) || info != (proxmox.LinuxMemoryAvailability{}) || client.memCalls != 0 {
					t.Fatalf("known Windows queued or served Linux memory: %+v, %v; calls=%d", info, err, client.memCalls)
				}
			}
			if got, ok := m.vmAgentMemCache[key]; (kind == "missing" && ok) || (kind != "missing" && (!ok || !reflect.DeepEqual(got, entry))) {
				t.Fatal("unsupported OS invented a failure or renewed old memory")
			}
		})
	}
}

func TestGuestWindowsMeminfoDoesNotSuppressOtherMemorySources(t *testing.T) {
	t.Parallel()
	const mib = uint64(1024 * 1024)
	for _, direct := range []bool{false, true} {
		m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{
			guestMetadataCacheKey("pve", "node", 105): {windowsGuest: true, osInfoObservedAt: time.Now()},
		}}
		client := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memAvailable: 4096}
		status := &proxmox.VMStatus{MaxMem: 8 * mib, Mem: 3 * mib, Agent: proxmox.VMAgentField{Value: 1}}
		wantSource, wantUsed := "status-mem", 3*mib
		if direct {
			status.MemInfo = &proxmox.VMMemInfo{Total: 8 * mib, Available: 6 * mib}
			wantSource, wantUsed = "available-field", 2*mib
		}
		total, used, source, deferred := m.resolveGuestStatusMemory(context.Background(), client, "pve", "windows", "node", 105, "pve:node:105", status, nil, 8*mib, "", &VMMemoryRaw{})
		if total != 8*mib || used != wantUsed || source != wantSource || deferred || client.memCalls != 0 {
			t.Fatalf("independent memory changed: total=%d used=%d source=%s deferred=%t calls=%d", total, used, source, deferred, client.memCalls)
		}
	}
}

func TestGuestWindowsOSEvidenceKeepsActualOrigin(t *testing.T) {
	t.Parallel()
	for _, versionDeferred := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "later-deferral"}[versionDeferred], func(t *testing.T) {
			m := &Monitor{guestMetadataLimiter: make(map[string]time.Time)}
			key := guestMetadataCacheKey("pve", "node", 105)
			client := &metadataSuppressionClient{os: map[string]interface{}{"result": map[string]interface{}{"id": "mswindows"}}}
			if versionDeferred {
				client.versionErr = proxmox.ErrGuestAgentDeferred
			}
			status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
			fetch := func() {
				m.guestMetadataLimiter[key] = time.Now().Add(-time.Second)
				m.fetchGuestAgentMetadata(context.Background(), client, "pve", "node", "guest", 105, status, false)
			}
			before := time.Now()
			fetch()
			entry := m.guestMetadataCache[key]
			if !entry.windowsGuest || entry.osInfoObservedAt.Before(before) || entry.osInfoObservedAt.After(time.Now()) {
				t.Fatal("accepted OS reply lost its actual observation")
			}
			if versionDeferred && (!entry.fetchedAt.IsZero() || entry.osName != "" || m.hasRecentGuestMetadataEvidence("pve", "node", 105, time.Now())) {
				t.Fatal("later deferral published partial display/availability evidence")
			}
			// Other useful metadata can renew display eligibility, not this OS
			// origin. Force the actual refresh boundary without sleeping.
			entry.fetchedAt = time.Now().Add(-2 * guestMetadataCacheTTL)
			m.guestMetadataCache[key] = entry
			client.osErr, client.versionErr = errors.New("OS unavailable"), nil
			fetch()
			refreshed := m.guestMetadataCache[key]
			if !refreshed.windowsGuest || !refreshed.osInfoObservedAt.Equal(entry.osInfoObservedAt) || !refreshed.fetchedAt.After(entry.fetchedAt) {
				t.Fatal("version refresh changed the independent OS origin")
			}
			// A complete empty OS reply invalidates the classification even
			// when its old name is retained for display.
			refreshed.fetchedAt = entry.fetchedAt
			m.guestMetadataCache[key] = refreshed
			client.os, client.osErr = nil, nil
			fetch()
			if got := m.guestMetadataCache[key]; got.windowsGuest || !got.osInfoObservedAt.IsZero() {
				t.Fatal("empty OS response renewed retained Windows classification")
			}
			// A normal subsequent Linux observation restores the existing
			// fallback, including a successful measured zero available value.
			current := m.guestMetadataCache[key]
			current.fetchedAt = entry.fetchedAt
			m.guestMetadataCache[key] = current
			client.os = map[string]interface{}{"name": "Linux"}
			fetch()
			if got := m.guestMetadataCache[key]; got.windowsGuest || got.osInfoObservedAt.IsZero() {
				t.Fatal("Linux recovery retained Windows classification")
			}
			memClient := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memInfo: &proxmox.LinuxMemoryAvailability{Total: 8192, Source: "meminfo-available"}}
			if info, err := m.getVMAgentMemoryAvailability(context.Background(), memClient, "pve", "node", 105); err != nil || info.Source != "meminfo-available" || info.EffectiveAvailable != 0 || memClient.memCalls != 1 {
				t.Fatalf("Linux zero-available fallback did not recover: %+v, %v", info, err)
			}
		})
	}
}
