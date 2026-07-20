package monitoring

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func udpTestTarget(address string, port int) config.AvailabilityTarget {
	return config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID: "udp-test", Name: "UDP test", TargetKind: config.AvailabilityTargetService,
		Address: address, Protocol: config.AvailabilityProbeUDP, Port: port, Enabled: true,
		TimeoutMillis: 250, UDPMode: config.AvailabilityUDPResponseRequired, UDPRequest: "PING",
	})
}

func TestProbeAvailabilityTargetUDPResponseRequired(t *testing.T) {
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		buffer := make([]byte, 32)
		_, remote, readErr := listener.ReadFromUDP(buffer)
		if readErr == nil {
			_, _ = listener.WriteToUDP([]byte("PONG"), remote)
		}
	}()

	port := listener.LocalAddr().(*net.UDPAddr).Port
	target := udpTestTarget("127.0.0.1", port)
	target.UDPExpected = "PONG"
	outcome, err := ProbeAvailabilityTargetResult(context.Background(), target)
	if err != nil || outcome != AvailabilityProbeReachable {
		t.Fatalf("UDP outcome = %q, error = %v", outcome, err)
	}
}

func TestProbeAvailabilityTargetUDPOpenOrFilteredIsIndeterminate(t *testing.T) {
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		buffer := make([]byte, 32)
		_, _, _ = listener.ReadFromUDP(buffer)
	}()

	port := listener.LocalAddr().(*net.UDPAddr).Port
	target := udpTestTarget("127.0.0.1", port)
	target.UDPMode = config.AvailabilityUDPOpenOrFiltered
	target.UDPRequest = ""
	started := time.Now()
	outcome, err := ProbeAvailabilityTargetResult(context.Background(), target)
	if err != nil || outcome != AvailabilityProbeIndeterminate {
		t.Fatalf("UDP outcome = %q, error = %v", outcome, err)
	}
	if time.Since(started) < 200*time.Millisecond {
		t.Fatalf("indeterminate result returned before the response deadline")
	}
}

func TestAvailabilityUDPValidationRequiresPayloadInResponseMode(t *testing.T) {
	target := udpTestTarget("127.0.0.1", 53)
	target.UDPRequest = ""
	if err := target.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want response-required payload error")
	}
	target.UDPMode = config.AvailabilityUDPOpenOrFiltered
	if err := target.Validate(); err != nil {
		t.Fatalf("open-or-filtered Validate() error = %v", err)
	}
}

func TestAvailabilityUDPIndeterminateDoesNotClaimReachabilityOrAccumulateFailures(t *testing.T) {
	target := udpTestTarget("127.0.0.1", 53)
	target.UDPMode = config.AvailabilityUDPOpenOrFiltered
	target.UDPRequest = ""
	monitor := &Monitor{availabilityStatuses: map[string]AvailabilityProbeStatus{
		target.ID: {TargetID: target.ID, ConsecutiveFailures: 2},
	}}

	checkedAt := time.Now().UTC()
	monitor.setAvailabilityStatus(target, checkedAt, 250*time.Millisecond, AvailabilityProbeIndeterminate, nil)
	status := monitor.AvailabilityStatusSnapshot()[target.ID]
	if status.Available {
		t.Fatal("indeterminate UDP result must not be reported as reachable")
	}
	if status.Outcome != string(AvailabilityProbeIndeterminate) {
		t.Fatalf("outcome = %q, want indeterminate", status.Outcome)
	}
	if status.ConsecutiveFailures != 0 {
		t.Fatalf("consecutive failures = %d, want 0", status.ConsecutiveFailures)
	}
	if got := availabilityResourceStatus(target, status); got != "warning" {
		t.Fatalf("resource status = %q, want warning", got)
	}
	if incident := availabilityIncident(target, status, checkedAt); incident != nil {
		t.Fatalf("indeterminate UDP result created an incident: %#v", incident)
	}
}
