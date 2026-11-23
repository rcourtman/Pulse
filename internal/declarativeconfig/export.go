package declarativeconfig

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Export converts live Pulse monitoring state into a deterministic,
// secret-redacted declarative configuration Document. No plaintext password,
// token, auth header value, webhook URL, custom field secret, signing secret,
// Apprise credential or target, HTTP probe credential/body/header, or dead-man
// ping URL is ever emitted.
func Export(snapshot Snapshot) Document {
	doc := Document{
		APIVersion: APIVersion,
		Kind:       Kind,
	}

	if snapshot.AlertConfig != nil || snapshot.AlertIntentPolicies != nil {
		section := &AlertsSection{}
		if snapshot.AlertConfig != nil {
			cloned := normalizeAlertConfigForDeclarative(*snapshot.AlertConfig)
			section.Config = &cloned
		}
		if snapshot.AlertIntentPolicies != nil {
			normalized := alerts.NormalizeAlertIntentPolicyDocument(*snapshot.AlertIntentPolicies)
			normalized.Revision = 0
			normalized.UpdatedAt = nil
			section.IntentPolicies = &normalized
		}
		doc.Alerts = section
	}

	targets := exportAvailabilityTargets(snapshot.AvailabilityTargets)
	doc.AvailabilityTargets = &targets

	states := exportOperatorStates(snapshot.OperatorStates)
	doc.OperatorStates = &states

	notif := &NotificationsSection{}
	if snapshot.Email != nil {
		emailSpec := exportEmailSpec(*snapshot.Email)
		notif.Email = &emailSpec
	}
	webhooks := exportWebhookSpecs(snapshot.Webhooks)
	notif.Webhooks = &webhooks
	if snapshot.Apprise != nil {
		appriseSpec := exportAppriseSpec(*snapshot.Apprise)
		notif.Apprise = &appriseSpec
	}
	if snapshot.DeadMan != nil {
		deadManSpec := exportDeadManSpec(*snapshot.DeadMan)
		notif.DeadMan = &deadManSpec
	}
	doc.Notifications = notif

	return doc
}

func normalizeAlertConfigForDeclarative(cfg alerts.AlertConfig) alerts.AlertConfig {
	var cloned alerts.AlertConfig
	if raw, err := json.Marshal(cfg); err == nil {
		_ = json.Unmarshal(raw, &cloned)
	} else {
		cloned = cfg
	}
	if cloned.ActivationTime != nil {
		utc := cloned.ActivationTime.UTC()
		cloned.ActivationTime = &utc
	}
	return cloned
}

func exportAvailabilityTargets(targets []config.AvailabilityTarget) []AvailabilityTargetSpec {
	if len(targets) == 0 {
		return []AvailabilityTargetSpec{}
	}
	normalized := make([]config.AvailabilityTarget, 0, len(targets))
	for _, target := range targets {
		normalized = append(normalized, config.NormalizeAvailabilityTarget(target))
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].ID != normalized[j].ID {
			return normalized[i].ID < normalized[j].ID
		}
		if normalized[i].Name != normalized[j].Name {
			return normalized[i].Name < normalized[j].Name
		}
		return normalized[i].Address < normalized[j].Address
	})

	out := make([]AvailabilityTargetSpec, 0, len(normalized))
	for _, target := range normalized {
		out = append(out, exportAvailabilityTarget(target))
	}
	return out
}

func exportAvailabilityTarget(target config.AvailabilityTarget) AvailabilityTargetSpec {
	spec := AvailabilityTargetSpec{
		ID:                            target.ID,
		Name:                          target.Name,
		TargetKind:                    target.TargetKind,
		Address:                       target.Address,
		Protocol:                      target.Protocol,
		Port:                          target.Port,
		Path:                          target.Path,
		UDPMode:                       target.UDPMode,
		UDPRequest:                    target.UDPRequest,
		UDPExpected:                   target.UDPExpected,
		Enabled:                       target.Enabled,
		PollIntervalSecs:              target.PollIntervalSecs,
		TimeoutMillis:                 target.TimeoutMillis,
		FailureThreshold:              target.FailureThreshold,
		LinkedResourceID:              target.LinkedResourceID,
		CertificateMonitoringDisabled: target.CertificateMonitoringDisabled,
		CertificateExpiryWarningDays:  target.CertificateExpiryWarningDays,
		ObservationLocationIDs:        append([]string(nil), target.EffectiveObservationLocationIDs()...),
	}
	if target.DockerPort != nil {
		dp := *target.DockerPort
		spec.DockerPort = &dp
	}
	if target.HTTP != nil {
		httpSpec := &AvailabilityHTTPSpec{
			Method: target.HTTP.Method,
			Authentication: AvailabilityHTTPAuthSpec{
				Type:     target.HTTP.Authentication.Type,
				Username: target.HTTP.Authentication.Username,
			},
			ExpectedStatusMin: target.HTTP.ExpectedStatusMin,
			ExpectedStatusMax: target.HTTP.ExpectedStatusMax,
			TextContains:      target.HTTP.TextContains,
			JSONPath:          target.HTTP.JSONPath,
			JSONEquals:        target.HTTP.JSONEquals,
		}
		if target.HTTP.Body != nil && *target.HTTP.Body != "" {
			httpSpec.Body = preservedSecret(defaultRedactedHint)
		}
		if target.HTTP.Authentication.Password != nil && *target.HTTP.Authentication.Password != "" {
			httpSpec.Authentication.Password = preservedSecret(defaultRedactedHint)
		}
		if target.HTTP.Authentication.BearerToken != nil && *target.HTTP.Authentication.BearerToken != "" {
			httpSpec.Authentication.BearerToken = preservedSecret(defaultRedactedHint)
		}
		if len(target.HTTP.Headers) > 0 {
			httpSpec.Headers = make([]AvailabilityHTTPHeaderSpec, 0, len(target.HTTP.Headers))
			for _, hdr := range target.HTTP.Headers {
				hdrSpec := AvailabilityHTTPHeaderSpec{
					ID:   hdr.ID,
					Name: hdr.Name,
				}
				if hdr.Value != nil && *hdr.Value != "" {
					hdrSpec.Value = preservedSecret(defaultRedactedHint)
				}
				httpSpec.Headers = append(httpSpec.Headers, hdrSpec)
			}
		}
		spec.HTTP = httpSpec
	}
	return spec
}

func exportOperatorStates(states []unified.ResourceOperatorState) []OperatorStateSpec {
	if len(states) == 0 {
		return []OperatorStateSpec{}
	}
	normalized := make([]unified.ResourceOperatorState, 0, len(states))
	for _, state := range states {
		norm := normalizeOperatorStateForDeclarative(state)
		if norm.CanonicalID == "" || norm.IsEmpty() {
			continue
		}
		normalized = append(normalized, norm)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].CanonicalID < normalized[j].CanonicalID
	})

	out := make([]OperatorStateSpec, 0, len(normalized))
	for _, state := range normalized {
		out = append(out, OperatorStateSpec{
			CanonicalID:           state.CanonicalID,
			MonitoringMode:        state.MonitoringMode,
			LifecycleState:        state.LifecycleState,
			NeverAutoRemediate:    state.NeverAutoRemediate,
			AutoRemediationPolicy: cloneAutoRemediationPolicy(state.AutoRemediationPolicy),
			MaintenanceStartAt:    cloneTimePtrUTC(state.MaintenanceStartAt),
			MaintenanceEndAt:      cloneTimePtrUTC(state.MaintenanceEndAt),
			MaintenanceRecurrence: cloneRecurringMaintenanceWindow(state.MaintenanceRecurrence),
			MaintenanceScope:      state.MaintenanceScope,
			MaintenanceReason:     state.MaintenanceReason,
			Criticality:           state.Criticality,
			Note:                  state.Note,
		})
	}
	return out
}

func normalizeOperatorStateForDeclarative(state unified.ResourceOperatorState) unified.ResourceOperatorState {
	norm := unified.NormalizeResourceOperatorState(state)
	norm.MaintenanceStartAt = cloneTimePtrUTC(norm.MaintenanceStartAt)
	norm.MaintenanceEndAt = cloneTimePtrUTC(norm.MaintenanceEndAt)
	norm.MaintenanceRecurrence = cloneRecurringMaintenanceWindow(norm.MaintenanceRecurrence)
	norm.AutoRemediationPolicy = cloneAutoRemediationPolicy(norm.AutoRemediationPolicy)
	norm.SetAt = time.Time{}
	norm.SetBy = ""
	return norm
}

func cloneTimePtrUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func cloneAutoRemediationPolicy(p unified.AutoRemediationPolicy) unified.AutoRemediationPolicy {
	out := unified.AutoRemediationPolicy{
		Enabled: p.Enabled,
	}
	if len(p.CapabilityNames) > 0 {
		out.CapabilityNames = append([]string(nil), p.CapabilityNames...)
	}
	if p.Window != nil {
		w := *p.Window
		out.Window = &w
	}
	return out
}

func cloneRecurringMaintenanceWindow(w *unified.RecurringMaintenanceWindow) *unified.RecurringMaintenanceWindow {
	if w == nil {
		return nil
	}
	cloned := *w
	if len(w.Weekdays) > 0 {
		cloned.Weekdays = append([]string(nil), w.Weekdays...)
	}
	return &cloned
}

func normalizeEmailForDeclarative(cfg notifications.EmailConfig) notifications.EmailConfig {
	out := cfg
	out.Provider = strings.TrimSpace(out.Provider)
	out.SMTPHost = strings.TrimSpace(out.SMTPHost)
	out.Username = strings.TrimSpace(out.Username)
	out.From = strings.TrimSpace(out.From)
	out.SubjectPrefix = strings.TrimSpace(out.SubjectPrefix)
	out.BrandName = strings.TrimSpace(out.BrandName)
	out.FooterNote = strings.TrimSpace(out.FooterNote)
	out.MinimumSeverity = normalizeMinimumSeverity(out.MinimumSeverity)
	out.TagFilter = normalizeStringSet(out.TagFilter, true)
	if len(out.TagFilter) == 0 {
		out.TagMode = ""
	} else if strings.EqualFold(strings.TrimSpace(out.TagMode), "any") {
		out.TagMode = "any"
	} else {
		out.TagMode = "all"
	}
	out.To = normalizeStringSet(out.To, true)
	if out.SMTPPort < 0 {
		out.SMTPPort = 0
	}
	if out.RateLimit < 0 {
		out.RateLimit = 0
	}
	return out
}

func normalizeMinimumSeverity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "warning":
		return "warning"
	case "critical":
		return "critical"
	default:
		return "all"
	}
}

func normalizeStringSet(values []string, caseInsensitive bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := trimmed
		if caseInsensitive {
			key = strings.ToLower(trimmed)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func exportEmailSpec(cfg notifications.EmailConfig) EmailSpec {
	norm := normalizeEmailForDeclarative(cfg)
	spec := EmailSpec{
		Enabled:         norm.Enabled,
		Provider:        norm.Provider,
		SMTPHost:        norm.SMTPHost,
		SMTPPort:        norm.SMTPPort,
		Username:        norm.Username,
		From:            norm.From,
		To:              append([]string(nil), norm.To...),
		TLS:             norm.TLS,
		StartTLS:        norm.StartTLS,
		RateLimit:       norm.RateLimit,
		TagFilter:       append([]string(nil), norm.TagFilter...),
		TagMode:         norm.TagMode,
		MinimumSeverity: norm.MinimumSeverity,
		SubjectPrefix:   norm.SubjectPrefix,
		BrandName:       norm.BrandName,
		FooterNote:      norm.FooterNote,
	}
	if norm.Password != "" {
		spec.Password = preservedSecret(defaultRedactedHint)
	}
	return spec
}

func normalizeWebhookForDeclarative(hook notifications.WebhookConfig) notifications.WebhookConfig {
	norm := notifications.NormalizeWebhookConfig(hook)
	norm.ID = strings.TrimSpace(norm.ID)
	norm.Name = strings.TrimSpace(norm.Name)
	norm.URL = strings.TrimSpace(norm.URL)
	norm.Method = strings.ToUpper(strings.TrimSpace(norm.Method))
	norm.Service = strings.TrimSpace(norm.Service)
	norm.Mention = strings.TrimSpace(norm.Mention)
	if len(norm.Headers) == 0 {
		norm.Headers = nil
	}
	if len(norm.CustomFields) == 0 {
		norm.CustomFields = nil
	}
	if len(norm.TagFilter) == 0 {
		norm.TagFilter = nil
	}
	return norm
}

func exportWebhookSpecs(webhooks []notifications.WebhookConfig) []WebhookSpec {
	if len(webhooks) == 0 {
		return []WebhookSpec{}
	}
	normalized := make([]notifications.WebhookConfig, 0, len(webhooks))
	for _, hook := range webhooks {
		normalized = append(normalized, normalizeWebhookForDeclarative(hook))
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].ID != normalized[j].ID {
			return normalized[i].ID < normalized[j].ID
		}
		return normalized[i].Name < normalized[j].Name
	})

	out := make([]WebhookSpec, 0, len(normalized))
	for _, hook := range normalized {
		spec := WebhookSpec{
			ID:              hook.ID,
			Name:            hook.Name,
			Method:          hook.Method,
			Enabled:         hook.Enabled,
			Service:         hook.Service,
			Template:        hook.Template,
			Mention:         hook.Mention,
			TagFilter:       append([]string(nil), hook.TagFilter...),
			TagMode:         hook.TagMode,
			MinimumSeverity: hook.MinimumSeverity,
		}
		if hook.URL != "" {
			spec.URL = preservedSecret(redactWebhookURLHint(hook.URL))
		}
		if hook.SigningSecret != "" {
			spec.SigningSecret = preservedSecret(defaultRedactedHint)
		}
		if len(hook.Headers) > 0 {
			spec.Headers = make(map[string]SecretField, len(hook.Headers))
			for k, v := range hook.Headers {
				if v != "" {
					spec.Headers[k] = preservedSecretValue(defaultRedactedHint)
				}
			}
		}
		if len(hook.CustomFields) > 0 {
			spec.CustomFields = make(map[string]SecretField, len(hook.CustomFields))
			for k, v := range hook.CustomFields {
				if v != "" {
					spec.CustomFields[k] = preservedSecretValue(defaultRedactedHint)
				}
			}
		}
		out = append(out, spec)
	}
	return out
}

func exportAppriseSpec(cfg notifications.AppriseConfig) AppriseSpec {
	norm := notifications.NormalizeAppriseConfig(cfg)
	spec := AppriseSpec{
		Enabled:         norm.Enabled,
		Mode:            norm.Mode,
		CLIPath:         norm.CLIPath,
		TimeoutSeconds:  norm.TimeoutSeconds,
		ServerURL:       norm.ServerURL,
		APIKeyHeader:    norm.APIKeyHeader,
		SkipTLSVerify:   norm.SkipTLSVerify,
		MinimumSeverity: norm.MinimumSeverity,
	}
	if norm.ConfigKey != "" {
		spec.ConfigKey = preservedSecret(defaultRedactedHint)
	}
	if norm.APIKey != "" {
		spec.APIKey = preservedSecret(defaultRedactedHint)
	}
	if len(norm.Targets) > 0 {
		spec.Targets = make([]SecretField, 0, len(norm.Targets))
		for _, target := range norm.Targets {
			spec.Targets = append(spec.Targets, preservedSecretValue(redactAppriseTargetHint(target)))
		}
	}
	return spec
}

func exportDeadManSpec(cfg notifications.DeadManConfig) DeadManSpec {
	norm := notifications.NormalizeDeadManConfig(cfg)
	spec := DeadManSpec{}
	if norm.PingURL != "" {
		spec.PingURL = preservedSecret(redactSchemeHostHint(norm.PingURL))
	}
	return spec
}
