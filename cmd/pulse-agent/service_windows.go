//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/kubernetesagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

const serviceName = "PulseAgent"

type windowsService struct {
	cfg      Config
	logger   zerolog.Logger
	eventLog *eventlog.Log
}

func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	if ws.eventLog != nil {
		ws.eventLog.Info(1, "Pulse Agent service starting")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	agentInfo.WithLabelValues(
		Version,
		fmt.Sprintf("%t", ws.cfg.EnableHost),
		fmt.Sprintf("%t", ws.cfg.EnableDocker),
		fmt.Sprintf("%t", ws.cfg.EnableKubernetes),
	).Set(1)
	agentUp.Set(1)
	defer agentUp.Set(0)

	var ready atomic.Bool
	runtimeStatus := newRuntimeHealth(&ready, map[string]bool{
		"host":       ws.cfg.EnableHost,
		"docker":     ws.cfg.EnableDocker,
		"kubernetes": ws.cfg.EnableKubernetes,
	})
	if ws.cfg.HealthAddr != "" {
		startHealthServer(ctx, ws.cfg.HealthAddr, &ready, &ws.logger, runtimeStatus)
	}
	remoteConfigAppliers := make([]RemoteConfigApplier, 0, 1)

	// Start Auto-Updater
	updater := newUpdater(agentupdate.Config{
		PulseURL:           ws.cfg.PulseURL,
		APIToken:           ws.cfg.APIToken,
		AgentName:          "pulse-agent",
		CurrentVersion:     Version,
		StateDir:           ws.cfg.StateDir,
		CheckInterval:      1 * time.Hour,
		InsecureSkipVerify: ws.cfg.InsecureSkipVerify,
		CACertPath:         ws.cfg.CACertPath,
		ServerFingerprint:  ws.cfg.ServerFingerprint,
		Logger:             &ws.logger,
		Disabled:           ws.cfg.DisableAutoUpdate,
	})

	g.Go(func() error {
		updater.RunLoop(ctx)
		return nil
	})

	// Start Host Agent (if enabled)
	if ws.cfg.EnableHost {
		hostCfg := hostagent.Config{
			PulseURL:           ws.cfg.PulseURL,
			APIToken:           ws.cfg.APIToken,
			Interval:           ws.cfg.Interval,
			HostnameOverride:   ws.cfg.HostnameOverride,
			AgentID:            ws.cfg.AgentID,
			AgentType:          "unified",
			AgentVersion:       Version,
			Tags:               ws.cfg.Tags,
			InsecureSkipVerify: ws.cfg.InsecureSkipVerify,
			CACertPath:         ws.cfg.CACertPath,
			ServerFingerprint:  ws.cfg.ServerFingerprint,
			DeploySSHUser:      ws.cfg.DeploySSHUser,
			LogLevel:           ws.cfg.LogLevel,
			Logger:             &ws.logger,
			AppliedConfig:      ws.cfg.AppliedConfig,
			UpdateStatus:       updater.Snapshot,
			ModuleStatus:       runtimeStatus.moduleStatuses,
			Observers:          hostObserverTargets(ws.cfg.Observers),
		}
		agent, err := hostagent.New(hostCfg)
		if err != nil {
			ws.logger.Error().Err(err).Msg("Failed to create host agent")
			if ws.eventLog != nil {
				ws.eventLog.Error(1, fmt.Sprintf("Failed to create host agent: %v", err))
			}
			changes <- svc.Status{State: svc.Stopped}
			return true, 1
		}
		remoteConfigAppliers = append(remoteConfigAppliers, agent)
		runtimeStatus.setState("host", moduleStateRunning, nil)

		g.Go(func() error {
			ws.logger.Info().Msg("Host agent module started")
			return agent.Run(ctx)
		})
	}

	// Start Docker / Podman module (if enabled). Match the foreground runtime's
	// retry semantics so a temporarily unavailable Docker Desktop pipe does not
	// terminate the Windows service.
	var dockerAgent RunnableCloser
	defer func() { cleanupDockerAgent(dockerAgent, &ws.logger) }()
	if ws.cfg.EnableDocker {
		dockerCfg := dockeragent.Config{
			PulseURL:           ws.cfg.PulseURL,
			APIToken:           ws.cfg.APIToken,
			Interval:           ws.cfg.Interval,
			HostnameOverride:   ws.cfg.HostnameOverride,
			AgentID:            ws.cfg.AgentID,
			AgentType:          "unified",
			AgentVersion:       Version,
			InsecureSkipVerify: ws.cfg.InsecureSkipVerify,
			CACertPath:         ws.cfg.CACertPath,
			ServerFingerprint:  ws.cfg.ServerFingerprint,
			DisableAutoUpdate:  true,
			LogLevel:           ws.cfg.LogLevel,
			Logger:             &ws.logger,
			SwarmScope:         "node",
			IncludeContainers:  true,
			IncludeServices:    true,
			IncludeTasks:       true,
			CollectDiskMetrics: true,
			Targets:            dockerReportTargets(ws.cfg),
		}

		agent, err := dockeragent.New(dockerCfg)
		if err != nil {
			runtimeStatus.setState("docker", moduleStateRetrying, err)
			ws.logger.Warn().Err(err).Msg("Docker / Podman module unavailable, retrying")
			g.Go(func() error {
				retried := initDockerWithRetry(ctx, dockerCfg, &ws.logger)
				if retried == nil {
					return nil
				}
				dockerAgent = retried
				runtimeStatus.setState("docker", moduleStateRunning, nil)
				return retried.Run(ctx)
			})
		} else {
			dockerAgent = agent
			runtimeStatus.setState("docker", moduleStateRunning, nil)
			g.Go(func() error {
				ws.logger.Info().Msg("Docker / Podman module started")
				return agent.Run(ctx)
			})
		}
	}

	if ws.cfg.EnableKubernetes {
		kubeCfg := kubernetesagent.Config{
			PulseURL:              ws.cfg.PulseURL,
			APIToken:              ws.cfg.APIToken,
			Interval:              ws.cfg.Interval,
			AgentID:               ws.cfg.AgentID,
			AgentType:             "unified",
			AgentVersion:          Version,
			InsecureSkipVerify:    ws.cfg.InsecureSkipVerify,
			CACertPath:            ws.cfg.CACertPath,
			ServerFingerprint:     ws.cfg.ServerFingerprint,
			LogLevel:              ws.cfg.LogLevel,
			Logger:                &ws.logger,
			KubeconfigPath:        ws.cfg.KubeconfigPath,
			KubeContext:           ws.cfg.KubeContext,
			IncludeNamespaces:     ws.cfg.KubeIncludeNamespaces,
			ExcludeNamespaces:     ws.cfg.KubeExcludeNamespaces,
			IncludeAllPods:        ws.cfg.KubeIncludeAllPods,
			IncludeAllDeployments: ws.cfg.KubeIncludeAllDeployments,
			MaxPods:               ws.cfg.KubeMaxPods,
			Targets:               kubernetesReportTargets(ws.cfg),
		}
		agent, err := kubernetesagent.New(kubeCfg)
		if err != nil {
			runtimeStatus.setState("kubernetes", moduleStateRetrying, err)
			ws.logger.Warn().Err(err).Msg("Kubernetes module unavailable, retrying")
			g.Go(func() error {
				retried := initKubernetesWithRetry(ctx, kubeCfg, &ws.logger)
				if retried == nil {
					return nil
				}
				runtimeStatus.setState("kubernetes", moduleStateRunning, nil)
				return retried.Run(ctx)
			})
		} else {
			runtimeStatus.setState("kubernetes", moduleStateRunning, nil)
			g.Go(func() error { return agent.Run(ctx) })
		}
	}

	if ws.cfg.PulseURL != "" && ws.cfg.APIToken != "" && ws.cfg.AgentID != "" && len(remoteConfigAppliers) > 0 {
		client := remoteconfig.New(remoteconfig.Config{
			PulseURL:           ws.cfg.PulseURL,
			APIToken:           ws.cfg.APIToken,
			AgentID:            ws.cfg.AgentID,
			Hostname:           ws.cfg.HostnameOverride,
			InsecureSkipVerify: ws.cfg.InsecureSkipVerify,
			CACertPath:         ws.cfg.CACertPath,
			ServerFingerprint:  ws.cfg.ServerFingerprint,
			Logger:             ws.logger,
		})
		defer client.Close()
		g.Go(func() error {
			runRemoteConfigLoop(ctx, client, remoteConfigAppliers, &ws.logger)
			return nil
		})
	}

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	ws.logger.Info().
		Str("version", Version).
		Str("pulse_url", ws.cfg.PulseURL).
		Bool("host_enabled", ws.cfg.EnableHost).
		Bool("docker_enabled", ws.cfg.EnableDocker).
		Msg("Pulse Agent service is running")
	if ws.eventLog != nil {
		ws.eventLog.Info(1, fmt.Sprintf("Pulse Agent started (URL: %s, Host: %v, Docker: %v)", ws.cfg.PulseURL, ws.cfg.EnableHost, ws.cfg.EnableDocker))
	}

	// Channel to receive errgroup completion
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- g.Wait()
	}()
	doneReceived := false

	// Service control loop
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				ws.logger.Info().Uint32("command", uint32(c.Cmd)).Msg("Received service control command")
				if ws.eventLog != nil {
					ws.eventLog.Info(1, "Pulse Agent received stop command")
				}
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				break loop
			default:
				ws.logger.Warn().Uint32("command", uint32(c.Cmd)).Msg("Unexpected service control command")
			}
		case err := <-doneChan:
			doneReceived = true
			if err != nil && err != context.Canceled {
				ws.logger.Error().Err(err).Msg("Agent error")
				if ws.eventLog != nil {
					ws.eventLog.Error(1, fmt.Sprintf("Pulse Agent error: %v", err))
				}
				changes <- svc.Status{State: svc.Stopped}
				return true, 1
			}
			break loop
		}
	}

	// Wait for agents to stop gracefully (with timeout)
	if doneReceived {
		ws.logger.Info().Msg("Agents stopped gracefully")
		if ws.eventLog != nil {
			ws.eventLog.Info(1, "Pulse Agent stopped gracefully")
		}
	} else {
		shutdownTimeout := time.NewTimer(10 * time.Second)
		defer shutdownTimeout.Stop()

		select {
		case err := <-doneChan:
			if err != nil && err != context.Canceled {
				ws.logger.Error().Err(err).Msg("Agent error during shutdown")
				if ws.eventLog != nil {
					ws.eventLog.Error(1, fmt.Sprintf("Pulse Agent shutdown error: %v", err))
				}
				changes <- svc.Status{State: svc.Stopped}
				return true, 1
			}
			ws.logger.Info().Msg("Agents stopped gracefully")
			if ws.eventLog != nil {
				ws.eventLog.Info(1, "Pulse Agent stopped gracefully")
			}
		case <-shutdownTimeout.C:
			ws.logger.Warn().Msg("Agent shutdown timeout, forcing stop")
			if ws.eventLog != nil {
				ws.eventLog.Warning(1, "Pulse Agent shutdown timeout")
			}
		}
	}

	changes <- svc.Status{State: svc.Stopped}
	return false, 0
}

// runAsWindowsService checks if we're running as a Windows service and handles it.
// Returns a special error to indicate the service ran (and main should exit),
// returns nil if not running as a service (main should continue normally),
// or returns an error if something failed.
func runAsWindowsService(cfg Config, logger zerolog.Logger) (ranAsService bool, err error) {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false, fmt.Errorf("failed to determine if running as service: %w", err)
	}

	if !isService {
		return false, nil
	}

	logger.Info().Msg("Running as Windows service")

	// Open Windows Event Log (best effort)
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		logger.Warn().Err(err).Msg("Could not open Windows Event Log, continuing without it")
		elog = nil
	}
	defer func() {
		if elog != nil {
			if closeErr := elog.Close(); closeErr != nil {
				logger.Warn().Err(closeErr).Msg("Failed to close Windows Event Log handle")
			}
		}
	}()

	ws := &windowsService{
		cfg:      cfg,
		logger:   logger,
		eventLog: elog,
	}

	err = svc.Run(serviceName, ws)
	if err != nil {
		if elog != nil {
			elog.Error(1, fmt.Sprintf("Failed to run service: %v", err))
		}
		return true, fmt.Errorf("failed to run Windows service: %w", err)
	}

	// Service ran successfully and exited
	os.Exit(0)
	return true, nil // unreachable, but required for compilation
}
