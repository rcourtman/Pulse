package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/collectorlifecycle"
)

const (
	collectorReduceAuthorityCommand    = "collector-reduce-authority"
	collectorVerifyRegistrationCommand = "collector-verify-registration"
	collectorReadAgentIDCommand        = "collector-read-agent-id"
)

func isCollectorLifecycleCommand(command string) bool {
	return command == collectorReduceAuthorityCommand || command == collectorVerifyRegistrationCommand || command == collectorReadAgentIDCommand
}

func runCollectorLifecycleCommand(ctx context.Context, command string, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	pulseURL := flags.String("url", "", "Pulse server URL")
	tokenFile := flags.String("token-file", "", "private collector token file")
	agentIDFile := flags.String("agent-id-file", "", "private collector agent identity file")
	tokenOwnerUID := flags.String("token-owner-uid", "", "dedicated collector UID allowed to own the token file")
	agentID := flags.String("agent-id", "", "collector agent identity")
	hostname := flags.String("hostname", "", "collector hostname")
	caFile := flags.String("cacert", "", "custom CA certificate bundle")
	serverFingerprint := flags.String("server-fingerprint", "", "exact Pulse server leaf certificate SHA-256 fingerprint")
	previousLastSeen := flags.String("previous-last-seen", "", "registration timestamp that the replacement must advance")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("collector lifecycle command received unexpected positional arguments")
	}
	var allowedTokenOwnerUID *uint64
	if raw := strings.TrimSpace(*tokenOwnerUID); raw != "" {
		parsed, parseErr := strconv.ParseUint(raw, 10, 32)
		if parseErr != nil {
			return fmt.Errorf("parse --token-owner-uid: %w", parseErr)
		}
		allowedTokenOwnerUID = &parsed
	}
	if command == collectorReadAgentIDCommand {
		if strings.TrimSpace(*agentIDFile) == "" {
			return errors.New("collector-read-agent-id requires --agent-id-file")
		}
		identity, err := collectorlifecycle.ReadAgentIDFile(*agentIDFile, allowedTokenOwnerUID)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, identity)
		return err
	}
	if strings.TrimSpace(*pulseURL) == "" || strings.TrimSpace(*tokenFile) == "" {
		return errors.New("collector lifecycle network command requires --url and --token-file")
	}
	client, err := collectorlifecycle.New(collectorlifecycle.Config{
		PulseURL:          *pulseURL,
		TokenFile:         *tokenFile,
		TokenOwnerUID:     allowedTokenOwnerUID,
		CACertPath:        *caFile,
		ServerFingerprint: *serverFingerprint,
	})
	if err != nil {
		return err
	}
	defer client.Close()

	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	switch command {
	case collectorReduceAuthorityCommand:
		if strings.TrimSpace(*previousLastSeen) != "" {
			return errors.New("--previous-last-seen is valid only for registration verification")
		}
		return client.ReduceAuthority(requestCtx, *agentID, *hostname)
	case collectorVerifyRegistrationCommand:
		var previous time.Time
		if raw := strings.TrimSpace(*previousLastSeen); raw != "" {
			previous, err = time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				return fmt.Errorf("parse --previous-last-seen: %w", err)
			}
		}
		registration, err := client.VerifyRegistration(requestCtx, *agentID, *hostname, previous)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, registration.LastSeen.Format(time.RFC3339Nano))
		return err
	default:
		return fmt.Errorf("unknown collector lifecycle command %q", command)
	}
}

func collectorLifecycleExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, collectorlifecycle.ErrCredentialRejected) {
		return 2
	}
	return 1
}
