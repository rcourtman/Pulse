package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
)

const defaultSocketPath = "/run/pulse-agent/helper.sock"

type commandConfig struct {
	socketPath  string
	allowedUID  int64
	socketGID   int64
	maxDeadline time.Duration
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("pulse-agent-helper: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if runtime.GOOS != "linux" {
		return errors.New("helper is supported only on Linux")
	}
	if os.Geteuid() != 0 {
		return errors.New("helper must run as root")
	}

	config, err := parseFlags(args)
	if err != nil {
		return err
	}
	uid, gid, err := resolveIdentity(config.allowedUID, config.socketGID)
	if err != nil {
		return err
	}

	listener, cleanup, err := openListener(config.socketPath, gid)
	if err != nil {
		return err
	}
	defer cleanup()

	registry := agenthelper.NewRegistry(localSMARTProvider{}, localProxmoxProvider{})
	server, err := agenthelper.NewServer(agenthelper.ServerConfig{
		AllowedUID:          uid,
		PeerResolver:        agenthelper.PlatformPeerResolver{},
		Registry:            registry,
		MaxOperationTimeout: config.maxDeadline,
		Audit: func(event agenthelper.AuditEvent) {
			log.Printf(
				"request_id=%q operation=%q operation_version=%d peer_uid=%d peer_pid=%d success=%t error_code=%q duration_ms=%d request_bytes=%d response_bytes=%d",
				event.RequestID,
				event.Operation,
				event.OperationVersion,
				event.Peer.UID,
				event.Peer.PID,
				event.Success,
				event.ErrorCode,
				event.Duration.Milliseconds(),
				event.RequestBytes,
				event.ResponseBytes,
			)
		},
	})
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	return server.Serve(ctx, listener)
}

func parseFlags(args []string) (commandConfig, error) {
	set := flag.NewFlagSet("pulse-agent-helper", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	config := commandConfig{}
	set.StringVar(&config.socketPath, "socket", defaultSocketPath, "fixed local Unix socket path used when systemd socket activation is absent")
	set.Int64Var(&config.allowedUID, "allowed-uid", -1, "authorized collector UID (defaults to the pulse-agent account)")
	set.Int64Var(&config.socketGID, "socket-gid", -1, "Unix socket group ID (defaults to the pulse-agent account group)")
	set.DurationVar(&config.maxDeadline, "max-deadline", 30*time.Second, "maximum accepted operation deadline")
	if err := set.Parse(args); err != nil {
		return commandConfig{}, err
	}
	if set.NArg() != 0 {
		return commandConfig{}, errors.New("positional arguments are not supported")
	}
	if !filepath.IsAbs(config.socketPath) || filepath.Clean(config.socketPath) != config.socketPath {
		return commandConfig{}, errors.New("socket must be a clean absolute path")
	}
	if config.maxDeadline < time.Millisecond || config.maxDeadline > 5*time.Minute {
		return commandConfig{}, errors.New("max-deadline must be between 1ms and 5m")
	}
	if config.allowedUID < -1 || config.socketGID < -1 {
		return commandConfig{}, errors.New("UID and GID must be -1 or non-negative")
	}
	if config.allowedUID > int64(^uint32(0)) || config.socketGID > int64(^uint32(0)) {
		return commandConfig{}, errors.New("UID and GID must fit in uint32")
	}
	return config, nil
}

func resolveIdentity(uidValue, gidValue int64) (uint32, int, error) {
	var account *user.User
	if uidValue < 0 || gidValue < 0 {
		resolved, err := user.Lookup("pulse-agent")
		if err != nil {
			return 0, 0, fmt.Errorf("look up pulse-agent account: %w", err)
		}
		account = resolved
	}
	if uidValue < 0 {
		parsed, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("parse pulse-agent UID: %w", err)
		}
		uidValue = int64(parsed)
	}
	if gidValue < 0 {
		parsed, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("parse pulse-agent GID: %w", err)
		}
		gidValue = int64(parsed)
	}
	if uidValue < 0 || gidValue < 0 || gidValue > int64(^uint(0)>>1) {
		return 0, 0, errors.New("resolved UID or GID is invalid")
	}
	return uint32(uidValue), int(gidValue), nil
}

func openListener(socketPath string, socketGID int) (net.Listener, func(), error) {
	if listener, ok, err := inheritedSystemdListener(); err != nil {
		return nil, func() {}, err
	} else if ok {
		return listener, func() { _ = listener.Close() }, nil
	}

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return nil, func() {}, fmt.Errorf("create socket directory: %w", err)
	}
	if info, err := os.Lstat(socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, func() {}, fmt.Errorf("refusing to replace non-socket path %s", socketPath)
		}
		if err := os.Remove(socketPath); err != nil {
			return nil, func() {}, fmt.Errorf("remove stale socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, func() {}, fmt.Errorf("inspect socket path: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("listen on helper socket: %w", err)
	}
	cleanup := func() {
		_ = listener.Close()
		if info, err := os.Lstat(socketPath); err == nil && info.Mode()&os.ModeSocket != 0 {
			_ = os.Remove(socketPath)
		}
	}
	if err := os.Chown(socketPath, 0, socketGID); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("set helper socket ownership: %w", err)
	}
	if err := os.Chmod(socketPath, 0o660); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("set helper socket mode: %w", err)
	}
	return listener, cleanup, nil
}

func inheritedSystemdListener() (net.Listener, bool, error) {
	pidText := strings.TrimSpace(os.Getenv("LISTEN_PID"))
	fdsText := strings.TrimSpace(os.Getenv("LISTEN_FDS"))
	if pidText == "" && fdsText == "" {
		return nil, false, nil
	}
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid != os.Getpid() {
		return nil, false, errors.New("LISTEN_PID does not identify this helper process")
	}
	fds, err := strconv.Atoi(fdsText)
	if err != nil || fds != 1 {
		return nil, false, errors.New("helper requires exactly one inherited systemd socket")
	}

	file := os.NewFile(uintptr(3), "systemd-helper-socket")
	if file == nil {
		return nil, false, errors.New("systemd socket file descriptor is unavailable")
	}
	defer file.Close()
	listener, err := net.FileListener(file)
	if err != nil {
		return nil, false, fmt.Errorf("open inherited systemd socket: %w", err)
	}
	if _, ok := listener.(*net.UnixListener); !ok {
		_ = listener.Close()
		return nil, false, errors.New("inherited systemd listener is not a Unix socket")
	}
	return listener, true, nil
}
