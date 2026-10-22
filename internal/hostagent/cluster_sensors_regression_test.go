package hostagent

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	"github.com/rs/zerolog"
)

const peerTemperatureFixture = `{"coretemp-isa-0000":{"Adapter":"ISA adapter","Package id 0":{"temp1_input":55}}}`

// Only invented metadata and sensor output are used: no real key is opened and
// no command or peer is contacted by this collector double.
func peerSensorFixtureAgent(log *bytes.Buffer, command func(context.Context, string, ...string) (string, error)) *Agent {
	return &Agent{
		cfg:    Config{EnableProxmox: true},
		logger: zerolog.New(log).Level(zerolog.InfoLevel),
		collector: &mockCollector{
			goos: "linux",
			statFn: func(path string) (os.FileInfo, error) {
				if path == "/root/.ssh/id_rsa" || path == "/root/.ssh/id_ed25519" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
			commandCombinedOutputFn: func(ctx context.Context, name string, args ...string) (string, error) {
				if name == "pvesh" {
					return `[{"type":"node","name":"local","ip":"192.0.2.1","local":1,"online":1},{"type":"node","name":"peer","ip":"192.0.2.2","online":1}]`, nil
				}
				return command(ctx, name, args...)
			},
			sensorsParseFn: sensors.Parse,
		},
	}
}

func TestClusterPeerMixedSSHIdentities(t *testing.T) {
	var logs bytes.Buffer
	a := peerSensorFixtureAgent(&logs, func(_ context.Context, name string, args ...string) (string, error) {
		if name != "ssh" {
			return "", errors.New("unexpected command")
		}
		var identities []string
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-i" {
				identities = append(identities, args[i+1])
			}
		}
		if strings.Join(identities, ",") != "/root/.ssh/id_rsa,/root/.ssh/id_ed25519" {
			return "", errors.New("both existing cluster identities must be offered, RSA first")
		}
		if args[len(args)-1] != "sensors -j 2>/dev/null" {
			return "", errors.New("sensor command errors must not be masked")
		}
		return peerTemperatureFixture, nil
	})
	result := a.collectClusterSensors(context.Background())
	if len(result) != 1 || result[0].NodeName != "peer" || result[0].Sensors.TemperatureCelsius["cpu_package"] != 55 {
		t.Fatalf("working RSA cluster identity lost behind unrelated Ed25519: %+v; logs: %s", result, &logs)
	}
}

func TestClusterPeerFailureVisibleAndRecovery(t *testing.T) {
	var logs bytes.Buffer
	failing := true
	a := peerSensorFixtureAgent(&logs, func(_ context.Context, _ string, _ ...string) (string, error) {
		if failing {
			return "private peer command output", errors.New("exit status 255")
		}
		return peerTemperatureFixture, nil
	})
	for i := 0; i < 3; i++ {
		if result := a.collectClusterSensors(context.Background()); len(result) != 0 {
			t.Fatalf("failed peer produced sensor readings: %+v", result)
		}
	}
	if count := strings.Count(logs.String(), `"level":"warn"`); count != 1 {
		t.Fatalf("want one actionable warning, not silence or interval spam, got %d: %s", count, &logs)
	}
	if !strings.Contains(logs.String(), "--disable-cluster-peer-sensors") || strings.Contains(logs.String(), "private peer command output") {
		t.Fatalf("warning must offer the opt-out without dumping command output: %s", &logs)
	}
	failing = false
	for i := 0; i < 2; i++ {
		if result := a.collectClusterSensors(context.Background()); len(result) != 1 {
			t.Fatalf("recovered peer missing: %+v", result)
		}
	}
	if count := strings.Count(logs.String(), "Cluster peer sensor collection recovered"); count != 1 {
		t.Fatalf("want one observed recovery, got %d: %s", count, &logs)
	}
	failing = true
	a.collectClusterSensors(context.Background())
	if count := strings.Count(logs.String(), `"level":"warn"`); count != 2 {
		t.Fatalf("a new failure after recovery must be visible immediately, got %d", count)
	}
}
