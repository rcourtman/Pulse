package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
)

const hostedRelayBootstrapOrgID = "default"

func (r *Router) loadRelayConfigForRuntime(ctx context.Context) (*relay.Config, error) {
	if r == nil || r.persistence == nil {
		return relay.DefaultConfig(), nil
	}

	cfg, err := r.persistence.LoadRelayConfig()
	if err != nil {
		return nil, err
	}

	cfg, err = r.ensureHostedRelayConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return r.ensureEnabledRelayIdentityConfig(cfg)
}

func (r *Router) ensureHostedRelayConfig(ctx context.Context, cfg *relay.Config) (*relay.Config, error) {
	if cfg == nil {
		cfg = relay.DefaultConfig()
	}
	if !r.shouldAutoBootstrapHostedRelayConfig(cfg) {
		return cfg, nil
	}

	state, instanceHost, err := r.hostedRelayEntitlementState(ctx)
	if err != nil {
		return nil, err
	}
	if state == nil || strings.TrimSpace(state.EntitlementJWT) == "" || strings.TrimSpace(instanceHost) == "" {
		return cfg, nil
	}

	effective := *cfg
	changed := false

	if !effective.Enabled {
		effective.Enabled = true
		changed = true
	}
	if strings.TrimSpace(effective.ServerURL) == "" {
		effective.ServerURL = relay.DefaultServerURL
		changed = true
	}
	if strings.TrimSpace(effective.InstanceSecret) == "" {
		// Hosted relay identity must be machine-owned and stable per tenant.
		effective.InstanceSecret = instanceHost
		changed = true
	}
	identityGenerated, err := ensureRelayIdentityKeyPair(&effective)
	if err != nil {
		return nil, fmt.Errorf("generate hosted relay identity: %w", err)
	}
	if identityGenerated {
		changed = true
	}

	if changed {
		if err := r.persistence.SaveRelayConfig(effective); err != nil {
			return nil, fmt.Errorf("save hosted relay config: %w", err)
		}
	}

	return &effective, nil
}

func (r *Router) shouldAutoBootstrapHostedRelayConfig(cfg *relay.Config) bool {
	if r == nil || r.licenseHandlers == nil || !r.licenseHandlers.hostedMode {
		return false
	}
	if cfg == nil {
		return true
	}
	if cfg.Enabled {
		return false
	}
	return strings.TrimSpace(cfg.InstanceSecret) == "" &&
		strings.TrimSpace(cfg.IdentityPrivateKey) == "" &&
		strings.TrimSpace(cfg.IdentityPublicKey) == "" &&
		strings.TrimSpace(cfg.IdentityFingerprint) == ""
}

func (r *Router) ensureEnabledRelayIdentityConfig(cfg *relay.Config) (*relay.Config, error) {
	if cfg == nil {
		cfg = relay.DefaultConfig()
	}
	if !cfg.Enabled {
		return cfg, nil
	}

	effective := *cfg
	identityGenerated, err := ensureRelayIdentityKeyPair(&effective)
	if err != nil {
		return nil, fmt.Errorf("generate relay identity: %w", err)
	}
	if !identityGenerated {
		return cfg, nil
	}

	if r != nil && r.persistence != nil {
		if err := r.persistence.SaveRelayConfig(effective); err != nil {
			return nil, fmt.Errorf("save relay identity config: %w", err)
		}
	}

	return &effective, nil
}

func ensureRelayIdentityKeyPair(cfg *relay.Config) (bool, error) {
	if cfg == nil {
		return false, nil
	}
	if strings.TrimSpace(cfg.IdentityPrivateKey) != "" &&
		strings.TrimSpace(cfg.IdentityPublicKey) != "" &&
		strings.TrimSpace(cfg.IdentityFingerprint) != "" {
		return false, nil
	}

	privKey, pubKey, fingerprint, err := relay.GenerateIdentityKeyPair()
	if err != nil {
		return false, err
	}
	cfg.IdentityPrivateKey = privKey
	cfg.IdentityPublicKey = pubKey
	cfg.IdentityFingerprint = fingerprint
	return true, nil
}

func (r *Router) relayRegistrationToken(ctx context.Context) string {
	if r == nil || r.licenseHandlers == nil {
		return ""
	}

	svc := r.licenseHandlers.Service(backgroundContext(ctx))
	if svc == nil {
		return ""
	}
	if lic := svc.Current(); lic != nil {
		return strings.TrimSpace(lic.Raw)
	}

	state, _, err := r.hostedRelayEntitlementState(ctx)
	if err != nil || state == nil {
		return ""
	}
	return strings.TrimSpace(state.EntitlementJWT)
}

func (r *Router) hostedRelayEntitlementState(ctx context.Context) (*billingState, string, error) {
	if r == nil || r.licenseHandlers == nil || !r.licenseHandlers.hostedMode || r.licenseHandlers.mtPersistence == nil {
		return nil, "", nil
	}

	svc := r.licenseHandlers.Service(backgroundContext(ctx))
	if svc == nil || !svc.HasFeature(featureRelayKey) {
		return nil, "", nil
	}

	billingStore := config.NewFileBillingStore(r.licenseHandlers.mtPersistence.BaseDataDir())
	state, err := billingStore.GetBillingState(hostedRelayBootstrapOrgID)
	if err != nil {
		return nil, "", fmt.Errorf("load hosted relay billing state: %w", err)
	}
	if state == nil {
		return nil, "", nil
	}

	instanceHost := r.licenseHandlers.hostedEntitlementInstanceHost(state)
	if strings.TrimSpace(state.EntitlementJWT) == "" || strings.TrimSpace(instanceHost) == "" {
		return nil, "", nil
	}

	return state, instanceHost, nil
}

func backgroundContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
