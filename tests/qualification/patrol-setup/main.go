// This browser fixture runs the production public router, settings persistence,
// provider transport and Watch-only Patrol loop. Only the inventory is synthetic.
// It has no configured collectors, agent executor, licence, notification channel
// or external provider. The browser harness owns its loopback provider and data.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

func run() error {
	port := flag.Int("port", 5301, "loopback preview port")
	flag.Parse()
	if *port < 1024 || *port > 65535 {
		return fmt.Errorf("preview port must be unprivileged")
	}
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	data, err := os.MkdirTemp("", "pulse-patrol-setup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(data)
	fmt.Println("patrol-setup-fixture-data " + data)
	password, err := bcrypt.GenerateFromPassword([]byte("synthetic-browser-password"), bcrypt.MinCost)
	if err != nil {
		return err
	}
	cfg := &config.Config{
		DataPath: data, ConfigPath: data, AppRoot: filepath.Dir(data),
		BindAddress: "127.0.0.1", FrontendPort: *port,
		AuthUser: "synthetic-browser-admin", AuthPass: string(password),
		PublicURL: "http://127.0.0.1:5300", TelemetryEnabled: false,
		ConnectionTimeout: time.Second, PVEPollingInterval: time.Hour,
		AdaptivePollingBaseInterval: time.Hour, AdaptivePollingMaxInterval: time.Hour,
		Discovery: config.DefaultDiscoveryConfig(),
	}
	persistence := config.NewConfigPersistence(data)
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = false
	aiCfg.Model = ""
	aiCfg.PatrolModel = ""
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		return err
	}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		return err
	}
	defer monitor.Stop()
	// There is deliberately no Monitor.Start: no guest, SSH, disk, backup or
	// network collector can run. An ordinary admitted host report supplies the
	// real Resources projection and the Patrol prompt, without mock/demo mode.
	_, err = monitor.ApplyHostReport(agentshost.Report{
		Agent:     agentshost.AgentInfo{ID: "setup-fixture-agent", IntervalSeconds: 3600},
		Host:      agentshost.HostInfo{ID: "setup-fixture-machine", Hostname: "setup-fixture-host", Platform: "linux", OSName: "debian"},
		Metrics:   agentshost.Metrics{CPUUsagePercent: 12, Memory: agentshost.MemoryMetric{TotalBytes: 8 << 30, UsedBytes: 2 << 30, FreeBytes: 6 << 30, Usage: 25}},
		Timestamp: time.Now().UTC(),
	}, nil)
	if err != nil {
		return err
	}
	hub := websocket.NewHub(func(string) interface{} { return monitor.GetState() })
	go hub.Run()
	defer hub.Stop()
	router := api.NewRouter(cfg, monitor, nil, hub, nil, "dev-patrol-setup-proof")
	defer router.ShutdownRBAC()
	defer router.ShutdownResourceStores()
	defer router.ShutdownAIIntelligence()
	defer router.StopAIChat(context.Background())
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	router.StartPatrol(ctx)
	router.StartAIChat(ctx)
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		return err
	}
	server := &http.Server{Handler: router.Handler(), ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Println("patrol-setup-fixture-ready")
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	fmt.Println("patrol-setup-fixture-http-closed")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("patrol-setup-fixture-clean-shutdown")
}
