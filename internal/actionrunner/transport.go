package actionrunner

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
)

// TransportConfig carries only the separate runner's connection credential,
// state, and TLS inputs. There is deliberately no monitoring/report config.
type TransportConfig = hostagent.ActionRunnerClientConfig
type CredentialLifecycleConfig = hostagent.ActionRunnerCredentialLifecycleConfig

func ValidatePulseURL(raw string) error {
	return hostagent.ValidateActionRunnerPulseURL(raw)
}

func CancelPendingCredential(ctx context.Context, config CredentialLifecycleConfig) error {
	return hostagent.CancelPendingActionRunnerCredential(ctx, config)
}

func RevokeCredential(ctx context.Context, config CredentialLifecycleConfig, hostID, hostname string) error {
	return hostagent.RevokeActionRunnerCredential(ctx, config, hostID, hostname)
}

func RunTypedActionLauncher(args []string) int {
	return hostagent.RunTypedActionLauncher(args)
}

func ReconcileTypedActionUnits(ctx context.Context) error {
	return hostagent.ReconcileTypedActionUnits(ctx)
}

// Client is the action-runner-owned facade over the existing typed action
// codecs and executors. The hostagent implementation remains an internal
// compatibility detail while the collector continues its legacy migration.
type Client struct {
	inner *hostagent.CommandClient
}

func NewClient(config TransportConfig, hostID, hostname, version string) *Client {
	return &Client{inner: hostagent.NewActionRunnerClient(config, hostID, hostname, version)}
}

func (client *Client) Run(ctx context.Context) error {
	return client.inner.Run(ctx)
}

func (client *Client) Close() error {
	if client == nil || client.inner == nil {
		return nil
	}
	return client.inner.Close()
}
