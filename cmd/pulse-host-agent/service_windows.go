//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rs/zerolog"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

type windowsService struct {
	cfg      hostagent.Config
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

	agent, err := hostagent.New(ws.cfg)
	if err != nil {
		ws.logger.Error().Err(err).Msg("Failed to create host agent")
		changes <- svc.Status{State: svc.Stopped}
		return true, 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the agent in a goroutine
	errChan := make(chan error, 1)
	go func() {
		ws.logger.Info().
			Str("pulse_url", ws.cfg.PulseURL).
			Str("agent_id", ws.cfg.AgentID).
			Dur("interval", ws.cfg.Interval).
			Msg("Starting Pulse host agent as Windows service")

		if err := agent.Run(ctx); err != nil && err != context.Canceled {
			errChan <- err
		}
		close(errChan)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	ws.logger.Info().Msg("Host agent service is running")
	if ws.eventLog != nil {
		ws.eventLog.Info(1, fmt.Sprintf("Pulse Host Agent started successfully (URL: %s, Interval: %s)", ws.cfg.PulseURL, ws.cfg.Interval))
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
		case err := <-errChan:
			if err != nil {
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
	shutdownTimeout := time.NewTimer(10 * time.Second)
	defer shutdownTimeout.Stop()

	select {
	case <-errChan:
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

	changes <- svc.Status{State: svc.Stopped}
	return false, 0
}

func runAsWindowsService(cfg hostagent.Config, logger zerolog.Logger) error {
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
			elog.Close()
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

func runServiceDebug(cfg hostagent.Config, logger zerolog.Logger) error {
	ws := &windowsService{
		cfg:    cfg,
		logger: logger,
	}
	return debug.Run("PulseHostAgent", ws)
}
