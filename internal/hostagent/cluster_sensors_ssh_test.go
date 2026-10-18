package hostagent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"golang.org/x/crypto/ssh"
)

// The real OpenSSH client authenticates to a disposable in-process SSH server.
// All key bytes are generated here. Production root paths are translated by the
// collector double; no real root key, pvesh, customer peer or host sshd is used.
func TestClusterPeerSSHNative(t *testing.T) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		t.Fatal("native peer proof needs the OpenSSH client:", err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	signers := map[string]ssh.Signer{}
	for name, key := range map[string]any{"id_rsa": rsaKey, "id_ed25519": edKey} {
		block, err := ssh.MarshalPrivateKey(key, "invented-peer-fixture")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
			t.Fatal(err)
		}
		keys["/root/.ssh/"+name] = path
		signers[name], err = ssh.NewSignerFromKey(key)
		if err != nil {
			t.Fatal(err)
		}
	}
	reports := make(map[string][]agentshost.ClusterNodeSensors)
	for _, trusted := range []string{"id_rsa", "id_ed25519"} {
		t.Run(trusted, func(t *testing.T) {
			port, allow := peerSSHServer(t, signers[trusted], signers["id_ed25519"])
			var logs bytes.Buffer
			a := peerSensorFixtureAgent(&logs, func(ctx context.Context, name string, args ...string) (string, error) {
				if name != "ssh" {
					return "", errors.New("unexpected command")
				}
				if strings.Join(args[len(args)-2:], " ") != "root@192.0.2.2 sensors -j 2>/dev/null" {
					return "", errors.New("unexpected peer or remote command")
				}
				// Check production bounds and unchanged host-key policy before
				// isolating fixture config, key files, port and loopback address.
				for _, option := range []string{"IdentitiesOnly=yes", "BatchMode=yes", "ConnectTimeout=5", "StrictHostKeyChecking=no", "UserKnownHostsFile=/dev/null"} {
					if !containsSSHOption(args, option) {
						return "", fmt.Errorf("missing production option %s", option)
					}
				}
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > clusterSensorsPerPeerTimeout {
					return "", errors.New("missing bounded peer context")
				}
				isolated := []string{"-F", "/dev/null", "-p", port, "-o", "IdentityAgent=none"}
				for i := 0; i < len(args); i++ {
					arg := args[i]
					if arg == "-i" {
						i++
						path, ok := keys[args[i]]
						if !ok {
							return "", errors.New("unexpected identity")
						}
						isolated = append(isolated, "-i", path)
					} else if arg == "root@192.0.2.2" {
						isolated = append(isolated, "root@127.0.0.1")
					} else {
						isolated = append(isolated, arg)
					}
				}
				cmd := exec.CommandContext(ctx, sshPath, isolated...)
				// The fixed fixture ignores ambient agent/config/askpass state.
				cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "LC_ALL=C"}
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Logf("invented loopback SSH result: %s (%v)", strings.TrimSpace(string(out)), err)
				}
				return string(out), err
			})
			result := a.collectClusterSensors(context.Background())
			if len(result) != 1 || result[0].Sensors.TemperatureCelsius["cpu_package"] != 55 {
				t.Fatalf("real SSH did not recover %s: %+v; %s", trusted, result, &logs)
			}
			reports[trusted] = result
			if trusted == "id_rsa" {
				allow(false)
				result = a.collectClusterSensors(context.Background())
				if len(result) != 0 || !strings.Contains(logs.String(), `"level":"warn"`) {
					t.Fatalf("real rejected SSH produced readings or no warning: %+v; %s", result, &logs)
				}
				reports["denied"] = result
				allow(true)
				result = a.collectClusterSensors(context.Background())
				if len(result) != 1 || !strings.Contains(logs.String(), "Cluster peer sensor collection recovered") {
					t.Fatalf("real SSH recovery missing: %+v; %s", result, &logs)
				}
				reports["recovered"] = result
			}
		})
	}
	if len(reports) == 4 {
		// Stable machine-readable producer bytes, safe synthetic values only.
		payload, err := json.Marshal(reports)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PEER_SENSOR_REPORTS=%s", payload)
	}
}

func containsSSHOption(args []string, option string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-o" && args[i+1] == option {
			return true
		}
	}
	return false
}

func peerSSHServer(t *testing.T, trusted, host ssh.Signer) (string, func(bool)) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var allowed atomic.Bool
	allowed.Store(true)
	config := &ssh.ServerConfig{PublicKeyCallback: func(metadata ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if allowed.Load() && metadata.User() == "root" && bytes.Equal(key.Marshal(), trusted.PublicKey().Marshal()) {
			return nil, nil
		}
		return nil, errors.New("fixture identity not authorised")
	}}
	config.AddHostKey(host)
	var connections sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer connections.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
				server, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return // expected for an untrusted offered identity
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						_ = incoming.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, requests, err := incoming.Accept()
					if err != nil {
						return
					}
					for request := range requests {
						var command struct{ Command string }
						if request.Type != "exec" || ssh.Unmarshal(request.Payload, &command) != nil || command.Command != "sensors -j 2>/dev/null" {
							_ = request.Reply(false, nil)
							continue
						}
						_ = request.Reply(true, nil)
						_, _ = channel.Write([]byte(peerTemperatureFixture))
						_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
						_ = channel.Close()
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
		connections.Wait()
	})
	return fmt.Sprint(listener.Addr().(*net.TCPAddr).Port), allowed.Store
}

func TestClusterPeerWarningIntervalAndPruning(t *testing.T) {
	var logs bytes.Buffer
	a := peerSensorFixtureAgent(&logs, nil)
	peer := clusterPeer{Name: "peer", IP: "192.0.2.2", Online: true}
	now := time.Now()
	failure := errors.New("exit status 255")
	a.logClusterSensorResult(peer, failure, now)
	a.logClusterSensorResult(peer, failure, now.Add(clusterSensorWarningInterval-time.Nanosecond))
	a.logClusterSensorResult(peer, failure, now.Add(clusterSensorWarningInterval))
	if count := strings.Count(logs.String(), `"level":"warn"`); count != 2 {
		t.Fatalf("wrong warning interval, count %d: %s", count, &logs)
	}
	a.pruneClusterSensorFailures([]clusterPeer{peer})
	if len(a.clusterSensorFailures) != 1 {
		t.Fatal("active failure was discarded")
	}
	a.pruneClusterSensorFailures(nil)
	if len(a.clusterSensorFailures) != 0 {
		t.Fatal("removed/offline peers must not accumulate warning state")
	}
}

func TestClusterPeerNoKeysAndCancellation(t *testing.T) {
	var logs bytes.Buffer
	a := peerSensorFixtureAgent(&logs, func(context.Context, string, ...string) (string, error) {
		t.Error("must not SSH without a key")
		return "", errors.New("unexpected SSH")
	})
	a.collector.(*mockCollector).statFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	if got := a.collectClusterSensors(context.Background()); len(got) != 0 {
		t.Fatal(got)
	}
	if !strings.Contains(logs.String(), "no supported cluster SSH key found") {
		t.Fatal("missing key warning not visible:", &logs)
	}
	logs.Reset()
	a = peerSensorFixtureAgent(&logs, func(context.Context, string, ...string) (string, error) {
		return "", context.Canceled
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := a.collectClusterSensors(ctx); len(got) != 0 || logs.Len() != 0 {
		t.Fatalf("shutdown manufactured data or a peer warning: %+v %s", got, &logs)
	}
}

func TestClusterPeerUnusableOutputIsNotRecovery(t *testing.T) {
	for _, output := range []string{"", "{}", "not sensor JSON"} {
		t.Run(output, func(t *testing.T) {
			var logs bytes.Buffer
			a := peerSensorFixtureAgent(&logs, func(context.Context, string, ...string) (string, error) { return output, nil })
			if got := a.collectClusterSensors(context.Background()); len(got) != 0 {
				t.Fatalf("invented reading for %q: %+v", output, got)
			}
			if !strings.Contains(logs.String(), `"level":"warn"`) || strings.Contains(logs.String(), "recovered") {
				t.Fatalf("unusable output was silent/recovered: %s", &logs)
			}
		})
	}
}

func TestClusterPeerSSHArgumentsPreserveBoundaries(t *testing.T) {
	var logs bytes.Buffer
	a := peerSensorFixtureAgent(&logs, func(_ context.Context, name string, args ...string) (string, error) {
		want := []string{"-i", "/root/.ssh/id_rsa", "-i", "/root/.ssh/id_ed25519", "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "LogLevel=ERROR", "root@192.0.2.2", "sensors -j 2>/dev/null"}
		if name != "ssh" || !reflect.DeepEqual(args, want) {
			t.Errorf("unexpected SSH command: %s %q", name, args)
		}
		return peerTemperatureFixture, nil
	})
	a.collectClusterSensors(context.Background())
}

func TestClusterPeerPartialSensorOutput(t *testing.T) {
	for _, tc := range []struct {
		name        string
		exit        int
		output      string
		wantReading bool
	}{
		{"partial chip failure", 1, peerTemperatureFixture, true},
		{"invalid partial output", 1, "invalid JSON", false},
		{"missing command cannot admit output", 127, peerTemperatureFixture, false},
		{"SSH failure cannot admit output", 255, peerTemperatureFixture, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Obtain the real os/exec exit type, without a sensor or network call.
			commandErr := exec.Command("sh", "-c", fmt.Sprintf("exit %d", tc.exit)).Run()
			if commandErr == nil {
				t.Fatal("expected fixture command failure")
			}
			var logs bytes.Buffer
			a := peerSensorFixtureAgent(&logs, func(context.Context, string, ...string) (string, error) { return tc.output, commandErr })
			result := a.collectClusterSensors(context.Background())
			if tc.wantReading {
				if len(result) != 1 || result[0].Sensors.TemperatureCelsius["cpu_package"] != 55 {
					t.Fatalf("valid partial readings lost: %+v", result)
				}
			} else if len(result) != 0 {
				t.Fatalf("failed transport/invalid data admitted: %+v", result)
			}
			if !strings.Contains(logs.String(), `"level":"warn"`) || strings.Contains(logs.String(), "recovered") {
				t.Fatalf("partial/failure wrongly labelled: %s", &logs)
			}
		})
	}
}
