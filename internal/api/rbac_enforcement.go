package api

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

// rbacEnforcementEnvVar is the break-glass override for RBAC enforcement. It is
// read once at startup. Setting it to a falsey value (e.g. "false") and
// restarting force-disables enforcement regardless of the stored setting, so an
// operator who locks themselves out of an SSO-only deployment can always
// recover. A truthy value force-enables it (still subject to the Pro license).
const rbacEnforcementEnvVar = "PULSE_RBAC_ENFORCEMENT"

// gatedRBACAuthorizer wraps the real RBAC authorizer with the gating policy that
// keeps Pulse's historical behaviour intact unless enforcement is explicitly
// turned on. When enforcement is off it allows every action (the pre-existing
// DefaultAuthorizer behaviour), so upgrading never silently flips a running
// deployment to deny-by-default.
//
// Enforcement is active only when ALL of the following hold:
//   - the operator opted in (config setting or the env override), and
//   - the active license grants the RBAC feature (Pro and above).
//
// The wrapped RBACAuthorizer still grants the configured admin user full access,
// so a local admin can never lock themselves out of the toggle.
type gatedRBACAuthorizer struct {
	cfg         *config.Config
	inner       auth.Authorizer
	envOverride *bool // nil = env var unset; non-nil = forced value from the env var
}

// InstallRBACEnforcement registers the gated RBAC authorizer as the global
// authorizer. It must be called before the API router is constructed, because
// the router captures auth.GetAuthorizer() once at build time.
func InstallRBACEnforcement(cfg *config.Config, manager auth.Manager) {
	if cfg == nil || manager == nil {
		return
	}

	var envOverride *bool
	if raw := os.Getenv(rbacEnforcementEnvVar); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			envOverride = &v
			log.Info().Bool("enforce", v).Msgf("RBAC enforcement overridden by %s", rbacEnforcementEnvVar)
		} else {
			log.Warn().Str("value", raw).Msgf("Ignoring invalid %s (expected true/false)", rbacEnforcementEnvVar)
		}
	}

	auth.SetAuthorizer(&gatedRBACAuthorizer{
		cfg:         cfg,
		inner:       auth.NewRBACAuthorizer(manager),
		envOverride: envOverride,
	})
	log.Info().Msg("RBAC authorizer installed (enforcement gated by license + setting)")
}

// enforcing reports whether role enforcement should be applied for this request.
func (g *gatedRBACAuthorizer) enforcing(ctx context.Context) bool {
	enabled := g.cfg.RBACEnforcementEnabled
	if g.envOverride != nil {
		enabled = *g.envOverride
	}
	if !enabled {
		return false
	}

	// License gate: never enforce without the Pro RBAC feature, so unlicensed
	// (community) deployments are never subject to deny-by-default.
	svc := getLicenseServiceForContext(ctx)
	if svc == nil || !svc.HasFeature(license.FeatureRBAC) {
		return false
	}
	return true
}

// Authorize implements auth.Authorizer.
func (g *gatedRBACAuthorizer) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	if !g.enforcing(ctx) {
		return true, nil
	}
	return g.inner.Authorize(ctx, action, resource)
}

// SetAdminUser implements auth.AdminConfigurable by delegating to the wrapped
// authorizer so the configured admin retains full access when enforcement is on.
func (g *gatedRBACAuthorizer) SetAdminUser(username string) {
	if configurable, ok := g.inner.(auth.AdminConfigurable); ok {
		configurable.SetAdminUser(username)
	}
}

func rbacPermissionForScope(scope string) (string, string) {
	switch strings.TrimSpace(scope) {
	case config.ScopeMonitoringRead, config.ScopeHostConfigRead:
		return auth.ActionRead, auth.ResourceNodes
	case config.ScopeMonitoringWrite,
		config.ScopeDockerReport,
		config.ScopeDockerManage,
		config.ScopeKubernetesReport,
		config.ScopeKubernetesManage,
		config.ScopeHostReport,
		config.ScopeHostManage:
		return auth.ActionWrite, auth.ResourceNodes
	case config.ScopeSettingsRead:
		return auth.ActionRead, auth.ResourceSettings
	case config.ScopeSettingsWrite:
		return auth.ActionWrite, auth.ResourceSettings
	case config.ScopeAIChat:
		return auth.ActionRead, auth.ResourceAI
	case config.ScopeAIExecute, config.ScopeAgentExec:
		return auth.ActionWrite, auth.ResourceAI
	case config.ScopeWildcard:
		return auth.ActionAdmin, "*"
	default:
		return auth.ActionAdmin, "*"
	}
}
