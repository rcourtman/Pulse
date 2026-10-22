//go:build !windows

package installtests

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func setupTunnelSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// A real encrypted SSH transport with one allowed direct-tcpip destination.
// This is a disposable fixture, not an installed server or customer endpoint.
func setupTunnelPeer(t *testing.T, signer ssh.Signer, destination string) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	var lock sync.Mutex
	var peers []net.Conn
	var workers sync.WaitGroup
	accepted := make(chan struct{})
	workers.Add(1)
	go func() {
		defer workers.Done()
		defer close(accepted)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			lock.Lock()
			peers = append(peers, conn)
			lock.Unlock()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer conn.Close()
				peer, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer peer.Close()
				go ssh.DiscardRequests(requests)
				for request := range channels {
					var target struct {
						Host       string
						Port       uint32
						Origin     string
						OriginPort uint32
					}
					if request.ChannelType() != "direct-tcpip" || ssh.Unmarshal(request.ExtraData(), &target) != nil ||
						net.JoinHostPort(target.Host, fmt.Sprint(target.Port)) != destination {
						_ = request.Reject(ssh.Prohibited, "fixture destination only")
						continue
					}
					backend, err := net.DialTimeout("tcp4", destination, time.Second)
					if err != nil {
						_ = request.Reject(ssh.ConnectionFailed, "fixture unavailable")
						continue
					}
					channel, requests, err := request.Accept()
					if err != nil {
						backend.Close()
						continue
					}
					go ssh.DiscardRequests(requests)
					workers.Add(1)
					go func() {
						defer workers.Done()
						defer backend.Close()
						defer channel.Close()
						done := make(chan struct{})
						go func() { _, _ = io.Copy(channel, backend); _ = channel.CloseWrite(); close(done) }()
						_, _ = io.Copy(backend, channel)
						_ = backend.Close()
						<-done
					}()
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-accepted
		// Wait for Accept before closing all connections it registered.
		lock.Lock()
		for _, peer := range peers {
			_ = peer.Close()
		}
		lock.Unlock()
		done := make(chan struct{})
		go func() { workers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("SSH fixture did not stop")
		}
	})
	return listener.Addr().String()
}

func TestBootstrapTransportGuidanceNativeSSH(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Fatal("native OpenSSH client required:", err)
	}
	doc, err := os.ReadFile(repoFile("docs", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile("(?s)```bash\n(ssh -N.*?)```").FindStringSubmatch(string(doc))
	if len(match) != 2 {
		t.Fatal("expected documented setup tunnel")
	}
	for _, name := range []string{"trusted", "unknown-key", "changed-key", "occupied-port", "backend-unavailable"} {
		t.Run(name, func(t *testing.T) {
			var hits atomic.Int32
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path != "/synthetic-setup" || r.Header.Get("X-Fixture-Token") != "synthetic-bootstrap" {
					http.Error(w, "wrong fixture exchange", http.StatusBadRequest)
					return
				}
				_, _ = io.WriteString(w, "synthetic setup reached")
			}))
			defer backend.Close()
			destination := strings.TrimPrefix(backend.URL, "http://")
			if name == "backend-unavailable" {
				backend.Close()
			}
			signer := setupTunnelSigner(t)
			peer := setupTunnelPeer(t, signer, destination)
			_, peerPort, _ := net.SplitHostPort(peer)
			known := filepath.Join(t.TempDir(), "known_hosts")
			key := signer.PublicKey()
			if name == "changed-key" {
				key = setupTunnelSigner(t).PublicKey()
			}
			content := []byte(fmt.Sprintf("[%s]:%s %s", "127.0.0.1", peerPort, ssh.MarshalAuthorizedKey(key)))
			if name == "unknown-key" {
				content = nil
			}
			if err := os.WriteFile(known, content, 0600); err != nil {
				t.Fatal(err)
			}
			local, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer local.Close()
			localAddr := local.Addr().String()
			if name != "occupied-port" {
				_ = local.Close()
			}
			command := strings.ReplaceAll(match[1], "\\\n", "")
			command = strings.ReplaceAll(command, "127.0.0.1:17655:127.0.0.1:7655", localAddr+":"+destination)
			command = strings.ReplaceAll(command, "admin@pulse-host", "fixture@127.0.0.1")
			args := strings.Fields(command)
			// Only fixture account, ports, keys and config are supplied here;
			// all tunnel/host-key safety flags come from the copied recipe.
			args = append(args[:len(args)-1], append([]string{"-F", "/dev/null", "-p", peerPort,
				"-o", "BatchMode=yes", "-o", "ConnectTimeout=2", "-o", "GlobalKnownHostsFile=/dev/null",
				"-o", "UserKnownHostsFile=" + known}, args[len(args)-1])...)
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			var output strings.Builder
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			defer func() {
				cancel()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Error("SSH client did not stop")
				}
			}()
			if name == "unknown-key" || name == "changed-key" || name == "occupied-port" {
				select {
				case err := <-done:
					done <- err
					if err == nil {
						t.Fatal("unsafe tunnel was accepted")
					}
				case <-time.After(4 * time.Second):
					t.Fatal("refused tunnel did not exit")
				}
				if hits.Load() != 0 {
					t.Fatal("refused tunnel reached setup")
				}
				if name != "occupied-port" && !strings.Contains(output.String(), "Host key verification failed") {
					t.Fatal("SSH did not refuse the expected host-key check")
				}
				if name == "occupied-port" && !strings.Contains(output.String(), "Address already in use") {
					t.Fatal("SSH did not refuse the expected local listener collision")
				}
				return
			}
			client := &http.Client{Timeout: 150 * time.Millisecond, Transport: &http.Transport{Proxy: nil}}
			defer client.CloseIdleConnections()
			deadline := time.Now().Add(2 * time.Second)
			var connected bool
			var listening bool
			for time.Now().Before(deadline) {
				if probe, err := net.DialTimeout("tcp4", localAddr, 100*time.Millisecond); err == nil {
					listening = true
					_ = probe.Close()
				}
				request, _ := http.NewRequest(http.MethodGet, "http://"+localAddr+"/synthetic-setup", nil)
				request.Header.Set("X-Fixture-Token", "synthetic-bootstrap")
				response, err := client.Do(request)
				if err == nil {
					body, _ := io.ReadAll(response.Body)
					_ = response.Body.Close()
					if response.StatusCode != http.StatusOK || string(body) != "synthetic setup reached" {
						t.Fatal("bad forwarded response")
					}
					connected = true
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if name == "trusted" && (!connected || hits.Load() != 1) {
				t.Fatal("trusted documented tunnel did not carry setup exchange")
			}
			if name == "backend-unavailable" && (!listening || connected || hits.Load() != 0) {
				t.Fatal("unavailable backend did not retain a live tunnel with failed HTTP")
			}
			select {
			case err := <-done:
				done <- err
				t.Fatal("valid tunnel exited unexpectedly")
			default:
			}
		})
	}
}
