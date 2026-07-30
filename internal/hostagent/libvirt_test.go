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

func TestCollectLibvirtInventoryUsesReadOnlyBoundedBulkStats(t *testing.T) {
	const virshPath = "/usr/bin/virsh"
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	var calls [][]string
	collector := &mockCollector{
		goos:  "linux",
		nowFn: func() time.Time { return now },
		lookPathFn: func(file string) (string, error) {
			if file == "virsh" {
				return virshPath, nil
			}
			return "", os.ErrNotExist
		},
		commandCombinedOutputFn: func(_ context.Context, name string, args ...string) (string, error) {
			if name != virshPath {
				t.Fatalf("command = %q, want %q", name, virshPath)
			}
			calls = append(calls, append([]string(nil), args...))
			if slices.Equal(args, []string{"--readonly", "list", "--all", "--name"}) {
				return "web vm\r\nalpha\nweb vm\n", nil
			}
			wantPrefix := libvirtStatsArgs
			if len(args) != len(wantPrefix)+2 ||
				!slices.Equal(args[:len(wantPrefix)], wantPrefix) ||
				!slices.Equal(args[len(wantPrefix):], []string{"alpha", "web vm"}) {
				t.Fatalf("unexpected domstats args: %#v", args)
			}
			return `Domain: 'alpha'
  state.state=5
  vcpu.current=2
  balloon.current=1048576
  balloon.maximum=2097152
Domain: 'web vm'
  state.state=1
  cpu.time=3000000000
  vcpu.current=4
  balloon.current=4194304
  balloon.maximum=8388608
  net.0.rx.bytes=100
  net.1.rx.bytes=50
  net.0.tx.bytes=80
  block.0.rd.bytes=4096
  block.1.rd.bytes=1024
  block.0.wr.bytes=2048
`, nil
		},
	}
	agent := &Agent{logger: zerolog.Nop(), collector: collector}

	got := agent.collectLibvirtInventory(context.Background())
	if got == nil || len(got.Domains) != 2 {
		t.Fatalf("inventory = %+v, want two domains", got)
	}
	if !got.CollectedAt.Equal(now) || len(calls) != 2 {
		t.Fatalf("inventory metadata = %+v, calls = %#v", got, calls)
	}
	if got.Domains[0].Name != "alpha" ||
		got.Domains[0].State != "shutoff" ||
		got.Domains[0].VCPUs != 2 ||
		got.Domains[0].MemoryCurrentBytes != 1<<30 ||
		got.Domains[0].MemoryMaximumBytes != 2<<30 {
		t.Fatalf("alpha domain = %+v", got.Domains[0])
	}
	web := got.Domains[1]
	if web.Name != "web vm" ||
		web.State != "running" ||
		web.VCPUs != 4 ||
		web.CPUTimeNanoseconds != 3_000_000_000 ||
		web.NetworkRXBytes != 150 ||
		web.NetworkTXBytes != 80 ||
		web.DiskReadBytes != 5120 ||
		web.DiskWriteBytes != 2048 {
		t.Fatalf("web domain = %+v", web)
	}
	if web.ID == "" || web.ID == got.Domains[0].ID {
		t.Fatalf("domain IDs are not stable and distinct: %+v", got.Domains)
	}
}

func TestParseLibvirtDomainStatsKeepsListedDomainWhenStatsAreMissing(t *testing.T) {
	got, err := parseLibvirtDomainStats(
		"Domain: 'first'\nstate.state=1\nDomain: 'unexpected'\nstate.state=6\n",
		[]string{"first", "second"},
	)
	if err != nil {
		t.Fatalf("parseLibvirtDomainStats: %v", err)
	}
	if len(got) != 2 ||
		got[0].Name != "first" ||
		got[0].State != "running" ||
		got[1].Name != "second" ||
		got[1].State != "unknown" {
		t.Fatalf("domains = %+v", got)
	}
}

func TestLibvirtParsingRejectsUnsafeAndOversizedInput(t *testing.T) {
	names, err := parseLibvirtDomainList("good\n-bad-option\nbad\x00name\n")
	if err != nil {
		t.Fatalf("parseLibvirtDomainList: %v", err)
	}
	if !slices.Equal(names, []string{"good"}) {
		t.Fatalf("names = %#v, want only good", names)
	}
	if _, err := parseLibvirtDomainList(strings.Repeat("x", libvirtMaxListOutputBytes+1)); err == nil {
		t.Fatal("expected oversized domain list to fail")
	}
	if _, err := parseLibvirtDomainStats(
		strings.Repeat("x", libvirtMaxStatsOutputBytes+1),
		[]string{"good"},
	); err == nil {
		t.Fatal("expected oversized domain stats to fail")
	}
}
