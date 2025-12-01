package monitoring

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestGuestMetadataCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		instanceName string
		nodeName     string
		vmid         int
		want         string
	}{
		{
			name:         "simple values",
			instanceName: "pve",
			nodeName:     "node1",
			vmid:         100,
			want:         "pve|node1|100",
		},
		{
			name:         "empty instance name",
			instanceName: "",
			nodeName:     "node1",
			vmid:         100,
			want:         "|node1|100",
		},
		{
			name:         "empty node name",
			instanceName: "pve",
			nodeName:     "",
			vmid:         100,
			want:         "pve||100",
		},
		{
			name:         "zero vmid",
			instanceName: "pve",
			nodeName:     "node1",
			vmid:         0,
			want:         "pve|node1|0",
		},
		{
			name:         "large vmid",
			instanceName: "cluster-01",
			nodeName:     "pve-node-2",
			vmid:         999999,
			want:         "cluster-01|pve-node-2|999999",
		},
		{
			name:         "all empty with zero",
			instanceName: "",
			nodeName:     "",
			vmid:         0,
			want:         "||0",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := guestMetadataCacheKey(tc.instanceName, tc.nodeName, tc.vmid)
			if got != tc.want {
				t.Fatalf("guestMetadataCacheKey(%q, %q, %d) = %q, want %q",
					tc.instanceName, tc.nodeName, tc.vmid, got, tc.want)
			}
		})
	}
}

func TestCloneStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  []string
		want []string
	}{
		{
			name: "nil slice",
			src:  nil,
			want: nil,
		},
		{
			name: "empty slice",
			src:  []string{},
			want: nil,
		},
		{
			name: "single element",
			src:  []string{"a"},
			want: []string{"a"},
		},
		{
			name: "multiple elements",
			src:  []string{"a", "b", "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "with empty strings",
			src:  []string{"", "a", ""},
			want: []string{"", "a", ""},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cloneStringSlice(tc.src)

			// Check equality
			if len(got) != len(tc.want) {
				t.Fatalf("cloneStringSlice returned slice of len %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("cloneStringSlice()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}

			// Verify it's a copy (not same backing array) for non-empty slices
			if len(tc.src) > 0 && got != nil {
				tc.src[0] = "modified"
				if got[0] == "modified" {
					t.Fatal("cloneStringSlice did not create independent copy")
				}
			}
		})
	}
}

func TestCloneGuestNetworkInterfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  []models.GuestNetworkInterface
		want []models.GuestNetworkInterface
	}{
		{
			name: "nil slice",
			src:  nil,
			want: nil,
		},
		{
			name: "empty slice",
			src:  []models.GuestNetworkInterface{},
			want: nil,
		},
		{
			name: "single interface no addresses",
			src: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55"},
			},
			want: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55"},
			},
		},
		{
			name: "single interface with addresses",
			src: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					MAC:       "00:11:22:33:44:55",
					Addresses: []string{"192.168.1.10", "10.0.0.5"},
					RXBytes:   1024,
					TXBytes:   512,
				},
			},
			want: []models.GuestNetworkInterface{
				{
					Name:      "eth0",
					MAC:       "00:11:22:33:44:55",
					Addresses: []string{"192.168.1.10", "10.0.0.5"},
					RXBytes:   1024,
					TXBytes:   512,
				},
			},
		},
		{
			name: "multiple interfaces",
			src: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
				{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", Addresses: []string{"10.0.0.5"}},
			},
			want: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
				{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", Addresses: []string{"10.0.0.5"}},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := cloneGuestNetworkInterfaces(tc.src)

			if len(got) != len(tc.want) {
				t.Fatalf("cloneGuestNetworkInterfaces returned slice of len %d, want %d", len(got), len(tc.want))
			}

			for i := range got {
				if got[i].Name != tc.want[i].Name {
					t.Errorf("interface[%d].Name = %q, want %q", i, got[i].Name, tc.want[i].Name)
				}
				if got[i].MAC != tc.want[i].MAC {
					t.Errorf("interface[%d].MAC = %q, want %q", i, got[i].MAC, tc.want[i].MAC)
				}
				if got[i].RXBytes != tc.want[i].RXBytes {
					t.Errorf("interface[%d].RXBytes = %d, want %d", i, got[i].RXBytes, tc.want[i].RXBytes)
				}
				if got[i].TXBytes != tc.want[i].TXBytes {
					t.Errorf("interface[%d].TXBytes = %d, want %d", i, got[i].TXBytes, tc.want[i].TXBytes)
				}
				if len(got[i].Addresses) != len(tc.want[i].Addresses) {
					t.Errorf("interface[%d].Addresses length = %d, want %d", i, len(got[i].Addresses), len(tc.want[i].Addresses))
				}
				for j := range got[i].Addresses {
					if got[i].Addresses[j] != tc.want[i].Addresses[j] {
						t.Errorf("interface[%d].Addresses[%d] = %q, want %q", i, j, got[i].Addresses[j], tc.want[i].Addresses[j])
					}
				}
			}

			// Verify addresses are deep-copied (not shared)
			if len(tc.src) > 0 && len(tc.src[0].Addresses) > 0 && got != nil {
				original := tc.src[0].Addresses[0]
				tc.src[0].Addresses[0] = "modified"
				if got[0].Addresses[0] == "modified" {
					t.Fatal("cloneGuestNetworkInterfaces did not deep copy addresses")
				}
				tc.src[0].Addresses[0] = original // restore for parallel safety
			}
		})
	}
}

func TestProcessGuestNetworkInterfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        []proxmox.VMNetworkInterface
		wantIPs    []string
		wantIfaces []models.GuestNetworkInterface
	}{
		{
			name:       "nil input",
			raw:        nil,
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name:       "empty input",
			raw:        []proxmox.VMNetworkInterface{},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "single interface with valid IP",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "filter loopback 127.x.x.x",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "lo",
					HardwareAddr: "00:00:00:00:00:00",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "127.0.0.1", Prefix: 8},
						{Address: "127.0.0.2", Prefix: 8},
					},
				},
			},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "filter link-local fe80",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "fe80::1", Prefix: 64},
						{Address: "FE80::abcd", Prefix: 64},
					},
				},
			},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "filter IPv6 loopback ::1",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "lo",
					HardwareAddr: "00:00:00:00:00:00",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "::1", Prefix: 128},
					},
				},
			},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "mixed valid and filtered IPs",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
						{Address: "127.0.0.1", Prefix: 8},
						{Address: "fe80::1", Prefix: 64},
						{Address: "10.0.0.5", Prefix: 8},
					},
				},
			},
			wantIPs: []string{"10.0.0.5", "192.168.1.10"}, // sorted
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5", "192.168.1.10"}},
			},
		},
		{
			name: "deduplicate IPs within interface",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
						{Address: "192.168.1.10", Prefix: 24},
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "deduplicate IPs across interfaces",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
				{
					Name:         "eth1",
					HardwareAddr: "AA:BB:CC:DD:EE:FF",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24}, // same IP on different interface
					},
				},
			},
			wantIPs: []string{"192.168.1.10"}, // deduplicated globally
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
				{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "multiple interfaces sorted by name",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth1",
					HardwareAddr: "AA:BB:CC:DD:EE:FF",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "10.0.0.5", Prefix: 8},
					},
				},
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"10.0.0.5", "192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
				{Name: "eth1", MAC: "AA:BB:CC:DD:EE:FF", Addresses: []string{"10.0.0.5"}},
			},
		},
		{
			name: "interface with only traffic (no IPs) is included",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses:  nil,
					Statistics:   map[string]interface{}{"rx-bytes": float64(1024), "tx-bytes": float64(512)},
				},
			},
			wantIPs: []string{},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: nil, RXBytes: 1024, TXBytes: 512},
			},
		},
		{
			name: "interface with no IPs and no traffic is excluded",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses:  nil,
					Statistics:   nil,
				},
			},
			wantIPs:    []string{},
			wantIfaces: []models.GuestNetworkInterface{},
		},
		{
			name: "whitespace trimmed from name and MAC",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "  eth0  ",
					HardwareAddr: "  00:11:22:33:44:55  ",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "whitespace trimmed from IP addresses",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "  192.168.1.10  ", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
		{
			name: "empty IP addresses are skipped",
			raw: []proxmox.VMNetworkInterface{
				{
					Name:         "eth0",
					HardwareAddr: "00:11:22:33:44:55",
					IPAddresses: []proxmox.VMIpAddress{
						{Address: "", Prefix: 0},
						{Address: "   ", Prefix: 0},
						{Address: "192.168.1.10", Prefix: 24},
					},
				},
			},
			wantIPs: []string{"192.168.1.10"},
			wantIfaces: []models.GuestNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10"}},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotIPs, gotIfaces := processGuestNetworkInterfaces(tc.raw)

			// Check IPs
			if len(gotIPs) != len(tc.wantIPs) {
				t.Errorf("processGuestNetworkInterfaces() IPs length = %d, want %d", len(gotIPs), len(tc.wantIPs))
			}
			for i := range gotIPs {
				if i >= len(tc.wantIPs) {
					break
				}
				if gotIPs[i] != tc.wantIPs[i] {
					t.Errorf("IPs[%d] = %q, want %q", i, gotIPs[i], tc.wantIPs[i])
				}
			}

			// Check interfaces
			if len(gotIfaces) != len(tc.wantIfaces) {
				t.Errorf("processGuestNetworkInterfaces() interfaces length = %d, want %d", len(gotIfaces), len(tc.wantIfaces))
			}
			for i := range gotIfaces {
				if i >= len(tc.wantIfaces) {
					break
				}
				if gotIfaces[i].Name != tc.wantIfaces[i].Name {
					t.Errorf("interface[%d].Name = %q, want %q", i, gotIfaces[i].Name, tc.wantIfaces[i].Name)
				}
				if gotIfaces[i].MAC != tc.wantIfaces[i].MAC {
					t.Errorf("interface[%d].MAC = %q, want %q", i, gotIfaces[i].MAC, tc.wantIfaces[i].MAC)
				}
				if gotIfaces[i].RXBytes != tc.wantIfaces[i].RXBytes {
					t.Errorf("interface[%d].RXBytes = %d, want %d", i, gotIfaces[i].RXBytes, tc.wantIfaces[i].RXBytes)
				}
				if gotIfaces[i].TXBytes != tc.wantIfaces[i].TXBytes {
					t.Errorf("interface[%d].TXBytes = %d, want %d", i, gotIfaces[i].TXBytes, tc.wantIfaces[i].TXBytes)
				}
				if len(gotIfaces[i].Addresses) != len(tc.wantIfaces[i].Addresses) {
					t.Errorf("interface[%d].Addresses length = %d, want %d", i, len(gotIfaces[i].Addresses), len(tc.wantIfaces[i].Addresses))
				}
				for j := range gotIfaces[i].Addresses {
					if j >= len(tc.wantIfaces[i].Addresses) {
						break
					}
					if gotIfaces[i].Addresses[j] != tc.wantIfaces[i].Addresses[j] {
						t.Errorf("interface[%d].Addresses[%d] = %q, want %q", i, j, gotIfaces[i].Addresses[j], tc.wantIfaces[i].Addresses[j])
					}
				}
			}
		})
	}
}

func TestRetryGuestAgentCall(t *testing.T) {
	t.Parallel()

	t.Run("successful call on first attempt", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{}
		callCount := 0
		fn := func(ctx context.Context) (interface{}, error) {
			callCount++
			return "success", nil
		}

		ctx := context.Background()
		result, err := m.retryGuestAgentCall(ctx, 50*time.Millisecond, 2, fn)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result != "success" {
			t.Fatalf("expected result 'success', got %v", result)
		}
		if callCount != 1 {
			t.Fatalf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("timeout error triggers retry and eventually succeeds", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{}
		callCount := 0
		fn := func(ctx context.Context) (interface{}, error) {
			callCount++
			if callCount < 3 {
				return nil, errors.New("context deadline exceeded (timeout)")
			}
			return "success after retries", nil
		}

		ctx := context.Background()
		result, err := m.retryGuestAgentCall(ctx, 50*time.Millisecond, 3, fn)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result != "success after retries" {
			t.Fatalf("expected result 'success after retries', got %v", result)
		}
		if callCount != 3 {
			t.Fatalf("expected 3 calls, got %d", callCount)
		}
	})

	t.Run("non-timeout error does not trigger retry", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{}
		callCount := 0
		nonTimeoutErr := errors.New("connection refused")
		fn := func(ctx context.Context) (interface{}, error) {
			callCount++
			return nil, nonTimeoutErr
		}

		ctx := context.Background()
		result, err := m.retryGuestAgentCall(ctx, 50*time.Millisecond, 3, fn)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != nonTimeoutErr.Error() {
			t.Fatalf("expected error %q, got %q", nonTimeoutErr.Error(), err.Error())
		}
		if result != nil {
			t.Fatalf("expected nil result, got %v", result)
		}
		if callCount != 1 {
			t.Fatalf("expected 1 call (no retry for non-timeout), got %d", callCount)
		}
	})

	t.Run("all retries exhausted returns last error", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{}
		callCount := 0
		fn := func(ctx context.Context) (interface{}, error) {
			callCount++
			return nil, errors.New("persistent timeout error")
		}

		ctx := context.Background()
		result, err := m.retryGuestAgentCall(ctx, 50*time.Millisecond, 2, fn)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "persistent timeout error" {
			t.Fatalf("expected 'persistent timeout error', got %q", err.Error())
		}
		if result != nil {
			t.Fatalf("expected nil result, got %v", result)
		}
		// maxRetries=2 means: attempt 0 (initial) + attempts 1,2 (retries) = 3 calls
		if callCount != 3 {
			t.Fatalf("expected 3 calls (1 initial + 2 retries), got %d", callCount)
		}
	})

	t.Run("context cancellation during retry delay aborts early", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{}
		callCount := 0
		fn := func(ctx context.Context) (interface{}, error) {
			callCount++
			return nil, errors.New("timeout error")
		}

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context after first call completes but during retry delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		result, err := m.retryGuestAgentCall(ctx, 50*time.Millisecond, 5, fn)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got %v", err)
		}
		if result != nil {
			t.Fatalf("expected nil result, got %v", result)
		}
		// Should only have made 1 call before context was canceled during delay
		if callCount != 1 {
			t.Fatalf("expected 1 call before cancellation, got %d", callCount)
		}
	})

	t.Run("zero retries means single attempt", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{}
		callCount := 0
		fn := func(ctx context.Context) (interface{}, error) {
			callCount++
			return nil, errors.New("timeout error")
		}

		ctx := context.Background()
		result, err := m.retryGuestAgentCall(ctx, 50*time.Millisecond, 0, fn)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if result != nil {
			t.Fatalf("expected nil result, got %v", result)
		}
		if callCount != 1 {
			t.Fatalf("expected 1 call with maxRetries=0, got %d", callCount)
		}
	})
}

func TestAcquireGuestMetadataSlot(t *testing.T) {
	t.Parallel()

	t.Run("nil monitor returns true", func(t *testing.T) {
		t.Parallel()

		var m *Monitor
		ctx := context.Background()
		if !m.acquireGuestMetadataSlot(ctx) {
			t.Fatal("nil monitor should return true (permissive default)")
		}
	})

	t.Run("nil slots channel returns true", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{guestMetadataSlots: nil}
		ctx := context.Background()
		if !m.acquireGuestMetadataSlot(ctx) {
			t.Fatal("nil slots channel should return true (permissive default)")
		}
	})

	t.Run("successfully acquires slot when available", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{guestMetadataSlots: make(chan struct{}, 2)}
		ctx := context.Background()
		if !m.acquireGuestMetadataSlot(ctx) {
			t.Fatal("should acquire slot when channel has capacity")
		}
		// Verify slot was actually acquired (channel should have 1 element)
		if len(m.guestMetadataSlots) != 1 {
			t.Fatalf("expected 1 slot acquired, got %d", len(m.guestMetadataSlots))
		}
	})

	t.Run("context cancellation returns false when slot not available", func(t *testing.T) {
		t.Parallel()

		// Create a channel with capacity 1 and fill it
		m := &Monitor{guestMetadataSlots: make(chan struct{}, 1)}
		m.guestMetadataSlots <- struct{}{}

		// Create already-cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if m.acquireGuestMetadataSlot(ctx) {
			t.Fatal("should return false when context is cancelled and slot not available")
		}
	})

	t.Run("multiple acquires fill up the channel", func(t *testing.T) {
		t.Parallel()

		capacity := 3
		m := &Monitor{guestMetadataSlots: make(chan struct{}, capacity)}
		ctx := context.Background()

		// Acquire all available slots
		for i := 0; i < capacity; i++ {
			if !m.acquireGuestMetadataSlot(ctx) {
				t.Fatalf("should acquire slot %d of %d", i+1, capacity)
			}
		}

		// Verify channel is full
		if len(m.guestMetadataSlots) != capacity {
			t.Fatalf("expected %d slots acquired, got %d", capacity, len(m.guestMetadataSlots))
		}

		// Next acquire should block; use cancelled context to test
		ctx2, cancel := context.WithCancel(context.Background())
		cancel()
		if m.acquireGuestMetadataSlot(ctx2) {
			t.Fatal("should return false when channel is full and context cancelled")
		}
	})
}

func TestTryReserveGuestMetadataFetch(t *testing.T) {
	t.Parallel()

	t.Run("nil monitor returns false", func(t *testing.T) {
		t.Parallel()

		var m *Monitor
		now := time.Now()
		if m.tryReserveGuestMetadataFetch("test-key", now) {
			t.Fatal("nil monitor should return false")
		}
	})

	t.Run("key not in map reserves and returns true", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:     make(map[string]time.Time),
			guestMetadataHoldDuration: 15 * time.Second,
		}
		now := time.Now()

		if !m.tryReserveGuestMetadataFetch("new-key", now) {
			t.Fatal("should return true for new key")
		}

		// Verify the key was added with correct expiry
		next, ok := m.guestMetadataLimiter["new-key"]
		if !ok {
			t.Fatal("key should be in limiter map")
		}
		expectedNext := now.Add(15 * time.Second)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v, got %v", expectedNext, next)
		}
	})

	t.Run("key in map with future time returns false", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		m := &Monitor{
			guestMetadataLimiter: map[string]time.Time{
				"existing-key": now.Add(10 * time.Second), // future
			},
			guestMetadataHoldDuration: 15 * time.Second,
		}

		if m.tryReserveGuestMetadataFetch("existing-key", now) {
			t.Fatal("should return false when key has future expiry")
		}

		// Verify the expiry was not changed
		next := m.guestMetadataLimiter["existing-key"]
		if !next.Equal(now.Add(10 * time.Second)) {
			t.Fatal("expiry time should not have been modified")
		}
	})

	t.Run("key in map with past time reserves and returns true", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		m := &Monitor{
			guestMetadataLimiter: map[string]time.Time{
				"expired-key": now.Add(-5 * time.Second), // past
			},
			guestMetadataHoldDuration: 20 * time.Second,
		}

		if !m.tryReserveGuestMetadataFetch("expired-key", now) {
			t.Fatal("should return true when key has past expiry")
		}

		// Verify the key was updated with new expiry
		next := m.guestMetadataLimiter["expired-key"]
		expectedNext := now.Add(20 * time.Second)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v, got %v", expectedNext, next)
		}
	})

	t.Run("default hold duration used when guestMetadataHoldDuration is zero", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:     make(map[string]time.Time),
			guestMetadataHoldDuration: 0, // zero triggers default
		}
		now := time.Now()

		if !m.tryReserveGuestMetadataFetch("key", now) {
			t.Fatal("should return true for new key")
		}

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(defaultGuestMetadataHold) // 15 seconds
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (default hold), got %v", expectedNext, next)
		}
	})

	t.Run("default hold duration used when guestMetadataHoldDuration is negative", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:     make(map[string]time.Time),
			guestMetadataHoldDuration: -5 * time.Second, // negative triggers default
		}
		now := time.Now()

		if !m.tryReserveGuestMetadataFetch("key", now) {
			t.Fatal("should return true for new key")
		}

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(defaultGuestMetadataHold)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (default hold), got %v", expectedNext, next)
		}
	})

	t.Run("key at exact current time reserves and returns true", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		m := &Monitor{
			guestMetadataLimiter: map[string]time.Time{
				"exact-key": now, // exactly now
			},
			guestMetadataHoldDuration: 10 * time.Second,
		}

		// now.Before(now) is false, so this should reserve
		if !m.tryReserveGuestMetadataFetch("exact-key", now) {
			t.Fatal("should return true when key expires exactly at now")
		}
	})
}

func TestScheduleNextGuestMetadataFetch(t *testing.T) {
	t.Parallel()

	t.Run("nil monitor is safe", func(t *testing.T) {
		t.Parallel()

		var m *Monitor
		now := time.Now()
		// Should not panic
		m.scheduleNextGuestMetadataFetch("test-key", now)
	})

	t.Run("schedules with default interval when guestMetadataMinRefresh is zero", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:    make(map[string]time.Time),
			guestMetadataMinRefresh: 0, // zero triggers default
		}
		now := time.Now()

		m.scheduleNextGuestMetadataFetch("key", now)

		next, ok := m.guestMetadataLimiter["key"]
		if !ok {
			t.Fatal("key should be in limiter map")
		}
		// config.DefaultGuestMetadataMinRefresh is 2 minutes
		expectedNext := now.Add(2 * time.Minute)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (default interval), got %v", expectedNext, next)
		}
	})

	t.Run("schedules with default interval when guestMetadataMinRefresh is negative", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:    make(map[string]time.Time),
			guestMetadataMinRefresh: -1 * time.Minute, // negative triggers default
		}
		now := time.Now()

		m.scheduleNextGuestMetadataFetch("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(2 * time.Minute) // DefaultGuestMetadataMinRefresh
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (default interval), got %v", expectedNext, next)
		}
	})

	t.Run("uses configured interval when positive", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:    make(map[string]time.Time),
			guestMetadataMinRefresh: 5 * time.Minute,
		}
		now := time.Now()

		m.scheduleNextGuestMetadataFetch("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(5 * time.Minute)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v, got %v", expectedNext, next)
		}
	})

	t.Run("adds jitter when rng is non-nil and jitter is positive", func(t *testing.T) {
		t.Parallel()

		// Use a seeded rng for deterministic testing
		rng := newDeterministicRng(42)

		m := &Monitor{
			guestMetadataLimiter:      make(map[string]time.Time),
			guestMetadataMinRefresh:   1 * time.Minute,
			guestMetadataRefreshJitter: 30 * time.Second,
			rng:                       rng,
		}
		now := time.Now()

		m.scheduleNextGuestMetadataFetch("key", now)

		next := m.guestMetadataLimiter["key"]
		minExpected := now.Add(1 * time.Minute)
		maxExpected := now.Add(1*time.Minute + 30*time.Second)

		if next.Before(minExpected) || next.After(maxExpected) {
			t.Fatalf("expected next time between %v and %v, got %v", minExpected, maxExpected, next)
		}
	})

	t.Run("no jitter when rng is nil", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:      make(map[string]time.Time),
			guestMetadataMinRefresh:   1 * time.Minute,
			guestMetadataRefreshJitter: 30 * time.Second, // jitter configured but rng is nil
			rng:                       nil,
		}
		now := time.Now()

		m.scheduleNextGuestMetadataFetch("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(1 * time.Minute) // no jitter added
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (no jitter), got %v", expectedNext, next)
		}
	})

	t.Run("no jitter when jitter duration is zero", func(t *testing.T) {
		t.Parallel()

		rng := newDeterministicRng(42)

		m := &Monitor{
			guestMetadataLimiter:      make(map[string]time.Time),
			guestMetadataMinRefresh:   1 * time.Minute,
			guestMetadataRefreshJitter: 0, // zero jitter
			rng:                       rng,
		}
		now := time.Now()

		m.scheduleNextGuestMetadataFetch("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(1 * time.Minute)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (no jitter), got %v", expectedNext, next)
		}
	})

	t.Run("no jitter when jitter duration is negative", func(t *testing.T) {
		t.Parallel()

		rng := newDeterministicRng(42)

		m := &Monitor{
			guestMetadataLimiter:      make(map[string]time.Time),
			guestMetadataMinRefresh:   1 * time.Minute,
			guestMetadataRefreshJitter: -10 * time.Second, // negative jitter
			rng:                       rng,
		}
		now := time.Now()

		m.scheduleNextGuestMetadataFetch("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(1 * time.Minute)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (no jitter), got %v", expectedNext, next)
		}
	})

	t.Run("overwrites existing key", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		m := &Monitor{
			guestMetadataLimiter: map[string]time.Time{
				"key": now.Add(-10 * time.Minute), // old value
			},
			guestMetadataMinRefresh: 3 * time.Minute,
		}

		m.scheduleNextGuestMetadataFetch("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(3 * time.Minute)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v, got %v", expectedNext, next)
		}
	})
}

func TestDeferGuestMetadataRetry(t *testing.T) {
	t.Parallel()

	t.Run("nil monitor is safe", func(t *testing.T) {
		t.Parallel()

		var m *Monitor
		now := time.Now()
		// Should not panic
		m.deferGuestMetadataRetry("test-key", now)
	})

	t.Run("uses default backoff when guestMetadataRetryBackoff is zero", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:       make(map[string]time.Time),
			guestMetadataRetryBackoff: 0, // zero triggers default
		}
		now := time.Now()

		m.deferGuestMetadataRetry("key", now)

		next, ok := m.guestMetadataLimiter["key"]
		if !ok {
			t.Fatal("key should be in limiter map")
		}
		// config.DefaultGuestMetadataRetryBackoff is 30 seconds
		expectedNext := now.Add(30 * time.Second)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (default backoff), got %v", expectedNext, next)
		}
	})

	t.Run("uses default backoff when guestMetadataRetryBackoff is negative", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:       make(map[string]time.Time),
			guestMetadataRetryBackoff: -10 * time.Second, // negative triggers default
		}
		now := time.Now()

		m.deferGuestMetadataRetry("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(30 * time.Second) // DefaultGuestMetadataRetryBackoff
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v (default backoff), got %v", expectedNext, next)
		}
	})

	t.Run("uses configured backoff when positive", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{
			guestMetadataLimiter:       make(map[string]time.Time),
			guestMetadataRetryBackoff: 45 * time.Second,
		}
		now := time.Now()

		m.deferGuestMetadataRetry("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(45 * time.Second)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v, got %v", expectedNext, next)
		}
	})

	t.Run("overwrites existing key", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		m := &Monitor{
			guestMetadataLimiter: map[string]time.Time{
				"key": now.Add(-5 * time.Minute), // old value
			},
			guestMetadataRetryBackoff: 1 * time.Minute,
		}

		m.deferGuestMetadataRetry("key", now)

		next := m.guestMetadataLimiter["key"]
		expectedNext := now.Add(1 * time.Minute)
		if next.Sub(expectedNext) > time.Millisecond || expectedNext.Sub(next) > time.Millisecond {
			t.Fatalf("expected next time ~%v, got %v", expectedNext, next)
		}
	})
}

// newDeterministicRng creates a rand.Rand with a fixed seed for reproducible tests.
func newDeterministicRng(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

func TestReleaseGuestMetadataSlot(t *testing.T) {
	t.Parallel()

	t.Run("nil monitor is safe", func(t *testing.T) {
		t.Parallel()

		var m *Monitor
		// Should not panic
		m.releaseGuestMetadataSlot()
	})

	t.Run("nil slots channel is safe", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{guestMetadataSlots: nil}
		// Should not panic
		m.releaseGuestMetadataSlot()
	})

	t.Run("successfully releases a slot", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{guestMetadataSlots: make(chan struct{}, 2)}
		ctx := context.Background()

		// Acquire a slot first
		if !m.acquireGuestMetadataSlot(ctx) {
			t.Fatal("failed to acquire slot")
		}
		if len(m.guestMetadataSlots) != 1 {
			t.Fatalf("expected 1 slot acquired, got %d", len(m.guestMetadataSlots))
		}

		// Release the slot
		m.releaseGuestMetadataSlot()

		// Verify slot was released
		if len(m.guestMetadataSlots) != 0 {
			t.Fatalf("expected 0 slots after release, got %d", len(m.guestMetadataSlots))
		}
	})

	t.Run("release on empty channel does not block", func(t *testing.T) {
		t.Parallel()

		m := &Monitor{guestMetadataSlots: make(chan struct{}, 2)}

		// Channel is empty - release should not block due to default case in select
		done := make(chan struct{})
		go func() {
			m.releaseGuestMetadataSlot()
			close(done)
		}()

		select {
		case <-done:
			// Success - release completed without blocking
		case <-time.After(100 * time.Millisecond):
			t.Fatal("releaseGuestMetadataSlot blocked on empty channel")
		}
	})
}
