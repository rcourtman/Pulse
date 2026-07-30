package hostagent

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestCollectXCPNGInventoryUsesBoundedReadOnlyLists(t *testing.T) {
	const xePath = "/opt/xensource/bin/xe"
	now := time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC)
	var calls [][]string
	collector := &mockCollector{
		goos:  "linux",
		nowFn: func() time.Time { return now },
		lookPathFn: func(file string) (string, error) {
			if file == "xe" {
				return xePath, nil
			}
			return "", os.ErrNotExist
		},
		commandCombinedOutputFn: func(_ context.Context, name string, args ...string) (string, error) {
			if name != xePath {
				t.Fatalf("command = %q, want %q", name, xePath)
			}
			calls = append(calls, append([]string(nil), args...))
			switch {
			case slices.Equal(args, xcpngPoolListArgs):
				return `uuid ( RO) : 11111111-1111-1111-1111-111111111111
          name-label ( RW): Lab Pool
              master ( RO): aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
`, nil
			case slices.Equal(args, xcpngHostListArgs):
				return `uuid ( RO) : aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
          name-label ( RW): xcp-one
            hostname ( RO): xcp-one.local

uuid ( RO) : bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb
          name-label ( RW): xcp-two
            hostname ( RO): xcp-two.local
`, nil
			case slices.Equal(args, xcpngVMListArgs):
				return `uuid ( RO) : cccccccc-cccc-cccc-cccc-cccccccccccc
          name-label ( RW): Database
         power-state ( RO): running
        VCPUs-number ( RO): 4
        memory-actual ( RO): 4294967296
    memory-static-max ( RW): 8589934592
          resident-on ( RO): aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa

uuid ( RO) : dddddddd-dddd-dddd-dddd-dddddddddddd
          name-label ( RW): Worker
         power-state ( RO): halted
        VCPUs-number ( RO): 2
        memory-actual ( RO): 2147483648
    memory-static-max ( RW): 4294967296
          resident-on ( RO): bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb
`, nil
			default:
				t.Fatalf("unexpected xe args: %#v", args)
				return "", nil
			}
		},
	}
	agent := &Agent{
		logger:    zerolog.Nop(),
		collector: collector,
		hostname:  "xcp-one.local",
	}

	got := agent.collectXCPNGInventory(context.Background())
	if got == nil || len(got.VMs) != 2 || len(calls) != 3 {
		t.Fatalf("inventory = %+v, calls = %#v", got, calls)
	}
	if got.PoolName != "Lab Pool" ||
		got.LocalHostUUID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" ||
		!got.CollectedAt.Equal(now) {
		t.Fatalf("inventory metadata = %+v", got)
	}
	if got.VMs[0].Name != "Database" ||
		got.VMs[0].PowerState != "running" ||
		got.VMs[0].VCPUs != 4 ||
		got.VMs[0].MemoryActual != 4<<30 ||
		got.VMs[0].MemoryStaticMax != 8<<30 {
		t.Fatalf("database VM = %+v", got.VMs[0])
	}
}

func TestXCPNGParsingRejectsUnsafeAndOversizedInput(t *testing.T) {
	records, err := parseXERecords(
		"uuid ( RO): ok\nignored line\nname-label ( RW): safe\n\n",
		1024,
		4,
	)
	if err != nil || len(records) != 1 || records[0]["name-label"] != "safe" {
		t.Fatalf("records = %#v, err = %v", records, err)
	}
	if _, err := parseXERecords(strings.Repeat("x", 1025), 1024, 1); err == nil {
		t.Fatal("expected oversized xe output to fail")
	}
	if canonicalXCPNGUUID("not-a-uuid") != "" ||
		safeXCPNGName("bad\x00name") != "" ||
		normalizeXCPNGPowerState("migrating") != "unknown" {
		t.Fatal("unsafe XCP-ng values were accepted")
	}
}
