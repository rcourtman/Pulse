package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
)

type committerValidationProvider struct {
	digest     string
	validation chan error
}

func (p *committerValidationProvider) Stage(context.Context, agenthelper.UpdateStageRequest) (agenthelper.UpdateStageResult, error) {
	return agenthelper.UpdateStageResult{}, errors.New("unexpected stage operation")
}

func (p *committerValidationProvider) Activate(context.Context, agenthelper.UpdateActivateRequest) (agenthelper.UpdateResult, error) {
	return agenthelper.UpdateResult{}, errors.New("unexpected activate operation")
}

func (p *committerValidationProvider) Commit(ctx context.Context, request agenthelper.UpdateCommitRequest) (agenthelper.UpdateResult, error) {
	err := validatePulseAgentCommitter(ctx, p.digest)
	p.validation <- err
	if err != nil {
		return agenthelper.UpdateResult{}, &agenthelper.ProviderError{
			Code:    agenthelper.ErrorStateConflict,
			Message: err.Error(),
		}
	}
	return agenthelper.UpdateResult{
		Action:       "committed",
		ActivationID: request.ActivationID,
		ActiveSHA256: request.CurrentSHA256,
	}, nil
}

func (p *committerValidationProvider) Rollback(context.Context, agenthelper.UpdateRollbackRequest) (agenthelper.UpdateResult, error) {
	return agenthelper.UpdateResult{}, errors.New("unexpected rollback operation")
}

func exerciseCommitterValidation(t *testing.T, digest string) (error, error) {
	t.Helper()

	socketPath := filepath.Join(t.TempDir(), "helper.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on helper socket: %v", err)
	}
	provider := &committerValidationProvider{
		digest:     digest,
		validation: make(chan error, 1),
	}
	server, err := agenthelper.NewServer(agenthelper.ServerConfig{
		AllowedUID:          uint32(os.Getuid()),
		PeerResolver:        agenthelper.PlatformPeerResolver{},
		Registry:            agenthelper.NewRegistryWithProviders(nil, nil, agenthelper.Providers{Updates: provider}),
		MaxOperationTimeout: 30 * time.Second,
	})
	if err != nil {
		_ = listener.Close()
		t.Fatalf("configure helper server: %v", err)
	}
	serverCtx, cancelServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Serve(serverCtx, listener)
	}()

	client, err := agenthelper.NewClient(agenthelper.ClientConfig{
		SocketPath:   socketPath,
		MaxDeadline:  30 * time.Second,
		NewRequestID: func() (string, error) { return "committer-validation", nil },
	})
	if err != nil {
		cancelServer()
		_ = listener.Close()
		<-serverDone
		t.Fatalf("configure helper client: %v", err)
	}
	var result agenthelper.UpdateResult
	_, callErr := client.Call(t.Context(), agenthelper.OperationAgentUpdateCommit, agenthelper.OperationVersion1, 30*time.Second, agenthelper.UpdateCommitRequest{
		ActivationID:  "committer-validation:0123456789abcdef",
		CurrentSHA256: digest,
	}, &result)
	validationErr := <-provider.validation

	cancelServer()
	_ = listener.Close()
	if err := <-serverDone; err != nil {
		t.Fatalf("serve helper protocol: %v", err)
	}
	return callErr, validationErr
}

func TestValidatePulseAgentCommitterAcceptsCurrentExecutableDigest(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc executable identity is Linux-specific")
	}
	data, err := os.ReadFile("/proc/self/exe")
	if err != nil {
		t.Fatalf("read current executable: %v", err)
	}
	digest := sha256.Sum256(data)
	callErr, validationErr := exerciseCommitterValidation(t, fmt.Sprintf("%x", digest))
	if validationErr != nil || callErr != nil {
		t.Fatalf("current executable was rejected: validation=%v call=%v", validationErr, callErr)
	}
}

func TestValidatePulseAgentCommitterRejectsIncorrectExecutableDigest(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc executable identity is Linux-specific")
	}
	callErr, validationErr := exerciseCommitterValidation(t, strings.Repeat("0", sha256.Size*2))
	if validationErr == nil || !strings.Contains(validationErr.Error(), "not executing the pending agent binary") {
		t.Fatalf("incorrect executable digest validation error = %v", validationErr)
	}
	var remoteErr *agenthelper.RemoteError
	if !errors.As(callErr, &remoteErr) || remoteErr.Code != agenthelper.ErrorStateConflict {
		t.Fatalf("incorrect executable digest helper error = %v", callErr)
	}
}

func TestInspectPulseAgentVersionRejectsWrongGoCommand(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspectPulseAgentVersion(context.Background(), executable); err == nil || !strings.Contains(err.Error(), "not the pulse-agent package") {
		t.Fatalf("helper test command accepted as pulse-agent: %v", err)
	}
}

func TestParseFlagsAcceptsOnlyLocalHelperConfiguration(t *testing.T) {
	config, err := parseFlags([]string{
		"--socket", "/run/pulse-agent/custom-helper.sock",
		"--allowed-uid", "1001",
		"--socket-gid", "1002",
		"--max-deadline", "45s",
	})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if config.socketPath != "/run/pulse-agent/custom-helper.sock" ||
		config.allowedUID != 1001 || config.socketGID != 1002 || config.maxDeadline != 45*time.Second {
		t.Fatalf("config = %#v", config)
	}

	for _, forbidden := range [][]string{
		{"--url", "https://pulse.example"},
		{"--token", "secret"},
		{"--listen", "tcp://127.0.0.1:9999"},
		{"positional"},
	} {
		if _, err := parseFlags(forbidden); err == nil {
			t.Fatalf("network/credential/positional flags accepted: %v", forbidden)
		}
	}
}

func TestParseFlagsRejectsUnsafeValues(t *testing.T) {
	tests := [][]string{
		{"--socket", "relative.sock"},
		{"--socket", "/run/../tmp/helper.sock"},
		{"--max-deadline", "0s"},
		{"--max-deadline", "1ns"},
		{"--max-deadline", "6m"},
		{"--allowed-uid", "-2"},
		{"--socket-gid", "-2"},
		{"--allowed-uid", "4294967296"},
		{"--socket-gid", "4294967296"},
	}
	for _, args := range tests {
		if _, err := parseFlags(args); err == nil {
			t.Fatalf("unsafe flags accepted: %v", args)
		}
	}
}

func TestResolveIdentityUsesExplicitNumericValues(t *testing.T) {
	uid, gid, err := resolveIdentity(1001, 1002)
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if uid != 1001 || gid != 1002 {
		t.Fatalf("uid=%d gid=%d", uid, gid)
	}
}

func TestResolveIdentityRejectsOutOfRangeNumericValues(t *testing.T) {
	if _, _, err := resolveIdentity(int64(^uint32(0))+1, 1002); err == nil {
		t.Fatal("UID above uint32 accepted")
	}
	if strconv.IntSize == 32 {
		if _, _, err := resolveIdentity(1001, int64(math.MaxInt32)+1); err == nil {
			t.Fatal("GID above 32-bit int accepted")
		}
	}
}

func TestInheritedSystemdListenerRejectsMalformedActivation(t *testing.T) {
	t.Setenv("LISTEN_PID", "")
	t.Setenv("LISTEN_FDS", "")
	if listener, ok, err := inheritedSystemdListener(); err != nil || ok || listener != nil {
		t.Fatalf("no activation = listener=%v ok=%t err=%v", listener, ok, err)
	}

	t.Setenv("LISTEN_PID", "not-a-pid")
	t.Setenv("LISTEN_FDS", "1")
	if _, _, err := inheritedSystemdListener(); err == nil {
		t.Fatal("malformed LISTEN_PID accepted")
	}

	t.Setenv("LISTEN_PID", strings.TrimSpace(strings.Repeat("0", 1)))
	t.Setenv("LISTEN_FDS", "2")
	if _, _, err := inheritedSystemdListener(); err == nil {
		t.Fatal("foreign LISTEN_PID accepted")
	}
}

func TestOpenListenerRefusesToReplaceNonSocket(t *testing.T) {
	t.Setenv("LISTEN_PID", "")
	t.Setenv("LISTEN_FDS", "")
	path := filepath.Join(t.TempDir(), "helper.sock")
	if err := os.WriteFile(path, []byte("do-not-replace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := openListener(path, os.Getgid()); err == nil {
		t.Fatal("regular file at socket path was replaced")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "do-not-replace" {
		t.Fatalf("protected file changed: contents=%q err=%v", contents, err)
	}
}

func TestCommandRegistryWiresTypedLocalProviders(t *testing.T) {
	var _ agenthelper.SMARTProvider = localSMARTProvider{}
	var _ agenthelper.ProxmoxProvider = localProxmoxProvider{}
	registry := agenthelper.NewRegistry(localSMARTProvider{}, localProxmoxProvider{})
	// Registry availability and strict empty request schemas are exercised
	// through the public protocol by the internal server suite.
	if registry == nil {
		t.Fatal("registry is nil")
	}
}

func TestFixedPrivilegedEndpointsAreNotCallerConfigurable(t *testing.T) {
	for _, args := range [][]string{
		{"--docker-socket", "/tmp/attacker.sock"},
		{"--staging-dir", "/tmp/staged"},
		{"--target", "/tmp/pulse-agent"},
	} {
		if _, err := parseFlags(args); err == nil {
			t.Fatalf("caller-selected privileged endpoint accepted: %v", args)
		}
	}
	if updateStagingDir != "/var/lib/pulse-agent-helper/update-staging" || agentBinaryPath != "/usr/local/bin/pulse-agent" {
		t.Fatal("update activation paths are not the fixed installer contract")
	}
}

func TestLocalProxmoxProviderAlwaysReturnsValidJSON(t *testing.T) {
	result, err := (localProxmoxProvider{}).LXCFilesystems(t.Context())
	if err != nil {
		t.Fatalf("LXCFilesystems: %v", err)
	}
	if !json.Valid(result) {
		t.Fatalf("invalid provider JSON: %q", result)
	}
}

func TestRunFailsClosedOutsideLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-Linux fail-closed assertion")
	}
	if err := run(nil); err == nil || !strings.Contains(err.Error(), "only on Linux") {
		t.Fatalf("run error = %v", err)
	}
}
