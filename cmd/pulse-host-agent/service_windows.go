//go:build windows

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

type windowsService struct {
	cfg      Config
	logger   zerolog.Logger
	eventLog *eventlog.Log
}

func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	// Log to Windows Event Log
	if ws.eventLog != nil {
		ws.eventLog.Info(1, "Pulse Host Agent service starting")
	}

	hostCfg := ws.cfg.HostConfig
	hostCfg.Logger = &ws.logger

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Start Auto-Updater
	updater := agentupdate.New(agentupdate.Config{
		PulseURL:           hostCfg.PulseURL,
		APIToken:           hostCfg.APIToken,
		AgentName:          "pulse-host-agent",
		CurrentVersion:     Version,
		CheckInterval:      1 * time.Hour,
		InsecureSkipVerify: hostCfg.InsecureSkipVerify,
		Logger:             &ws.logger,
		Disabled:           ws.cfg.DisableAutoUpdate,
	})

	g.Go(func() error {
		updater.RunLoop(ctx)
		return nil
	})

	// Start the host agent
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
		ws.logger.Info().
			Str("version", Version).
			Str("pulse_url", hostCfg.PulseURL).
			Str("agent_id", hostCfg.AgentID).
			Dur("interval", hostCfg.Interval).
			Bool("auto_update", !ws.cfg.DisableAutoUpdate).
			Msg("Starting Pulse host agent as Windows service")
		return agent.Run(ctx)
	})

	// Channel to receive errgroup completion
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- g.Wait()
	}()
	doneReceived := false

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	ws.logger.Info().Msg("Host agent service is running")
	if ws.eventLog != nil {
		ws.eventLog.Info(1, fmt.Sprintf("Pulse Host Agent started successfully (URL: %s, Interval: %s)", hostCfg.PulseURL, hostCfg.Interval))
	}

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
					ws.eventLog.Info(1, "Pulse Host Agent received stop command")
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
					ws.eventLog.Error(1, fmt.Sprintf("Pulse Host Agent error: %v", err))
				}
				changes <- svc.Status{State: svc.Stopped}
				return true, 1
			}
			break loop
		}
	}

	// Wait for agent to stop gracefully (with timeout)
	if doneReceived {
		ws.logger.Info().Msg("Agent stopped gracefully")
		if ws.eventLog != nil {
			ws.eventLog.Info(1, "Pulse Host Agent stopped gracefully")
		}
	} else {
		shutdownTimeout := time.NewTimer(10 * time.Second)
		defer shutdownTimeout.Stop()

		select {
		case err := <-doneChan:
			if err != nil && err != context.Canceled {
				ws.logger.Error().Err(err).Msg("Agent error during shutdown")
				if ws.eventLog != nil {
					ws.eventLog.Error(1, fmt.Sprintf("Pulse Host Agent shutdown error: %v", err))
				}
				changes <- svc.Status{State: svc.Stopped}
				return true, 1
			}
			ws.logger.Info().Msg("Agent stopped gracefully")
			if ws.eventLog != nil {
				ws.eventLog.Info(1, "Pulse Host Agent stopped gracefully")
			}
		case <-shutdownTimeout.C:
			ws.logger.Warn().Msg("Agent shutdown timeout, forcing stop")
			if ws.eventLog != nil {
				ws.eventLog.Warning(1, "Pulse Host Agent shutdown timeout")
			}
		}
	}

	changes <- svc.Status{State: svc.Stopped}
	return false, 0
}

func runAsWindowsService(cfg Config, logger zerolog.Logger) error {
	// Check if we're running as a Windows service
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("failed to determine if running as service: %w", err)
	}

	if !isService {
		// Not running as a service, run normally
		return nil
	}

	logger.Info().Msg("Running as Windows service")

	// Open Windows Event Log (best effort - don't fail if it doesn't work)
	elog, err := eventlog.Open("PulseHostAgent")
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

	// Run as a Windows service
	err = svc.Run("PulseHostAgent", ws)
	if err != nil {
		if elog != nil {
			elog.Error(1, fmt.Sprintf("Failed to run service: %v", err))
		}
		return fmt.Errorf("failed to run Windows service: %w", err)
	}

	return nil
}
