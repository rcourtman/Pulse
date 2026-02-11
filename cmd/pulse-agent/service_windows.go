//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
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

	// Start Auto-Updater
	updater := agentupdate.New(agentupdate.Config{
		PulseURL:           ws.cfg.PulseURL,
		APIToken:           ws.cfg.APIToken,
		AgentName:          "pulse-agent",
		CurrentVersion:     Version,
		CheckInterval:      1 * time.Hour,
		InsecureSkipVerify: ws.cfg.InsecureSkipVerify,
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
			LogLevel:           ws.cfg.LogLevel,
			Logger:             &ws.logger,
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

		g.Go(func() error {
			ws.logger.Info().Msg("Host agent module started")
			return agent.Run(ctx)
		})
	}

	// Start Docker Agent (if enabled)
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
			DisableAutoUpdate:  true,
			LogLevel:           ws.cfg.LogLevel,
			Logger:             &ws.logger,
			SwarmScope:         "node",
			IncludeContainers:  true,
			IncludeServices:    true,
			IncludeTasks:       true,
			CollectDiskMetrics: true,
		}

		agent, err := dockeragent.New(dockerCfg)
		if err != nil {
			ws.logger.Error().Err(err).Msg("Failed to create docker agent")
			if ws.eventLog != nil {
				ws.eventLog.Error(1, fmt.Sprintf("Failed to create docker agent: %v", err))
			}
			changes <- svc.Status{State: svc.Stopped}
			return true, 1
		}

		g.Go(func() error {
			ws.logger.Info().Msg("Docker agent module started")
			return agent.Run(ctx)
		})
	}

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	ws.logger.Info().
		Str("version", Version).
		Str("pulse_url", ws.cfg.PulseURL).
		Bool("host_agent", ws.cfg.EnableHost).
		Bool("docker_agent", ws.cfg.EnableDocker).
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
			elog.Close()
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
