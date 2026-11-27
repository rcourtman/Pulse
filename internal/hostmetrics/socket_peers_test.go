package hostmetrics

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestReadActivePeersFrom_FiltersPrivacyAndDetectsDirection(t *testing.T) {
	dir := t.TempDir()
	tcpPath := filepath.Join(dir, "tcp")
	tcp6Path := filepath.Join(dir, "tcp6")

	// Little-endian IPv4 hex helpers:
	// 192.168.0.10 -> 0A00A8C0
	// 192.168.0.20 -> 1400A8C0
	// 100.64.1.5   -> 05014064 (Tailscale / CGNAT)
	// 1.1.1.1      -> 01010101 (public internet - MUST be discarded)
	// 127.0.0.1    -> 0100007F (loopback - MUST be discarded)
	// 169.254.1.2  -> 0201FEA9 (link-local - MUST be discarded)
	// 0.0.0.0      -> 00000000
	tcpContent := strings.Join([]string{
		"  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode",
		// Listening on :5432 (0x1538)
		"   0: 00000000:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 100",
		// Outbound established to 192.168.0.20:8086 (0x1F96), two ephemeral sockets should dedupe
		"   1: 0A00A8C0:C350 1400A8C0:1F96 01 00000000:00000000 00:00000000 00000000     0        0 101",
		"   2: 0A00A8C0:C351 1400A8C0:1F96 01 00000000:00000000 00:00000000 00000000     0        0 102",
		// Outbound established to Tailscale peer 100.64.1.5:445 (0x01BD)
		"   3: 0A00A8C0:C352 05014064:01BD 01 00000000:00000000 00:00000000 00000000     0        0 103",
		// Inbound established from 192.168.0.20 ephemeral port to local listening :5432
		"   4: 0A00A8C0:1538 1400A8C0:D001 01 00000000:00000000 00:00000000 00000000     0        0 104",
		// Public internet 1.1.1.1:443 - MUST be discarded
		"   5: 0A00A8C0:C355 01010101:01BB 01 00000000:00000000 00:00000000 00000000     0        0 105",
		// Loopback - MUST be discarded
		"   6: 0100007F:C356 0100007F:1F90 01 00000000:00000000 00:00000000 00000000     0        0 106",
		// Self IP connection - MUST be discarded
		"   7: 0A00A8C0:C357 0A00A8C0:1F90 01 00000000:00000000 00:00000000 00000000     0        0 107",
		// Link-local - MUST be discarded
		"   8: 0A00A8C0:C358 0201FEA9:0050 01 00000000:00000000 00:00000000 00000000     0        0 108",
		// TIME_WAIT (06) - MUST be discarded
		"   9: 0A00A8C0:C359 1400A8C0:0050 06 00000000:00000000 00:00000000 00000000     0        0 109",
	}, "\n")

	// IPv6 ULA fd00::2 -> words: 000000FD 00000000 00000000 02000000
	// IPv6 ULA fd00::1 -> words: 000000FD 00000000 00000000 01000000
	// IPv4-mapped 192.168.0.25 (::ffff:192.168.0.25) -> 00000000 00000000 FFFF0000 1900A8C0
	// Public IPv6 2606:4700:4700::1111 -> 00470626 00000047 00000000 11110000 (MUST be discarded)
	tcp6Content := strings.Join([]string{
		"  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode",
		// Outbound IPv6 ULA fd00::1 -> fd00::2:6379 (0x18EB)
		"   0: 000000FD000000000000000001000000:C400 000000FD000000000000000002000000:18EB 01 00000000:00000000 00:00000000 00000000 0 0 200",
		// Outbound IPv4-mapped ::ffff:192.168.0.10 -> ::ffff:192.168.0.25:2049 (0x0801)
		"   1: 0000000000000000FFFF00000A00A8C0:C401 0000000000000000FFFF00001900A8C0:0801 01 00000000:00000000 00:00000000 00000000 0 0 201",
		// Public IPv6 - MUST be discarded
		"   2: 000000FD000000000000000001000000:C402 00470626000000470000000011110000:01BB 01 00000000:00000000 00:00000000 00000000 0 0 202",
	}, "\n")

	if err := os.WriteFile(tcpPath, []byte(tcpContent), 0o600); err != nil {
		t.Fatalf("write tcp fixture: %v", err)
	}
	if err := os.WriteFile(tcp6Path, []byte(tcp6Content), 0o600); err != nil {
		t.Fatalf("write tcp6 fixture: %v", err)
	}

	got := readActivePeersFrom(tcpPath, tcp6Path)
	want := []agentshost.NetworkPeer{
		{LocalIP: "192.168.0.10", RemoteIP: "100.64.1.5", RemotePort: 445, Protocol: "tcp"},
		{LocalIP: "192.168.0.10", RemoteIP: "192.168.0.20", RemotePort: 8086, Protocol: "tcp"},
		{LocalIP: "192.168.0.10", RemoteIP: "192.168.0.25", RemotePort: 2049, Protocol: "tcp"},
		{LocalIP: "fd00::1", RemoteIP: "fd00::2", RemotePort: 6379, Protocol: "tcp"},
		{LocalIP: "192.168.0.10", LocalPort: 5432, RemoteIP: "192.168.0.20", Protocol: "tcp", Inbound: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readActivePeersFrom() =\n%#v\nwant:\n%#v", got, want)
	}
}

func TestReadActivePeersFrom_MissingFilesReturnNil(t *testing.T) {
	dir := t.TempDir()
	got := readActivePeersFrom(filepath.Join(dir, "missing-tcp"), filepath.Join(dir, "missing-tcp6"))
	if got != nil {
		t.Fatalf("missing procfs files should return nil, got %+v", got)
	}
}

func TestReadActivePeersFrom_CapsAtMaxActivePeers(t *testing.T) {
	dir := t.TempDir()
	tcpPath := filepath.Join(dir, "tcp")
	var b strings.Builder
	b.WriteString("  sl  local_address rem_address st\n")
	for i := 1; i <= maxActivePeers+20; i++ {
		// 10.0.0.<octet> with varying port so each entry is unique
		fmt.Fprintf(&b, " %3d: 0100000A:C350 0200000A:%04X 01\n", i, 1000+i)
	}
	if err := os.WriteFile(tcpPath, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write tcp fixture: %v", err)
	}
	got := readActivePeersFrom(tcpPath, filepath.Join(dir, "missing-tcp6"))
	if len(got) != maxActivePeers {
		t.Fatalf("len(got) = %d, want bound %d", len(got), maxActivePeers)
	}
}
