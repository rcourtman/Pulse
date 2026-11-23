package declarativeconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	domainAlerts              = "alerts"
	domainAvailabilityTargets = "availabilityTargets"
	domainOperatorStates      = "operatorStates"
	domainNotifications       = "notifications"

	actionCreate = "create"
	actionUpdate = "update"
	actionDelete = "delete"
)

// Plan validates desired against Pulse's canonical validators, resolves all
// secret references against current, computes deterministic diffs respecting
// opts.Prune, and produces a PlanHash that binds both the current snapshot and
// the resolved target state. No secret values appear in PlanResult.
func Plan(current Snapshot, desired Document, opts PlanOptions) (PlanResult, ResolvedState, error) {
	if strings.TrimSpace(desired.APIVersion) != APIVersion {
		return PlanResult{}, ResolvedState{}, fmt.Errorf("%w: apiVersion must be %q", ErrInvalidDocument, APIVersion)
	}
	if strings.TrimSpace(desired.Kind) != Kind {
		return PlanResult{}, ResolvedState{}, fmt.Errorf("%w: kind must be %q", ErrInvalidDocument, Kind)
	}

	changes := make([]ConfigChange, 0)
	summary := PlanSummary{}
	resolved := ResolvedState{}

	if desired.Alerts != nil {
		alertChanges, unchanged, err := planAlerts(current, desired.Alerts, &resolved)
		if err != nil {
			return PlanResult{}, ResolvedState{}, err
		}
		changes = append(changes, alertChanges...)
		summary.Unchanged += unchanged
	}

	if desired.AvailabilityTargets != nil {
		targetChanges, unchanged, err := planAvailabilityTargets(current.AvailabilityTargets, *desired.AvailabilityTargets, opts.Prune, &resolved)
		if err != nil {
			return PlanResult{}, ResolvedState{}, err
		}
		changes = append(changes, targetChanges...)
		summary.Unchanged += unchanged
	}

	if desired.OperatorStates != nil {
		stateChanges, unchanged, err := planOperatorStates(current.OperatorStates, *desired.OperatorStates, opts.Prune, &resolved)
		if err != nil {
			return PlanResult{}, ResolvedState{}, err
		}
		changes = append(changes, stateChanges...)
		summary.Unchanged += unchanged
	}

	if desired.Notifications != nil {
		notifChanges, unchanged, err := planNotifications(current, desired.Notifications, opts.Prune, &resolved)
		if err != nil {
			return PlanResult{}, ResolvedState{}, err
		}
		changes = append(changes, notifChanges...)
		summary.Unchanged += unchanged
	}

	sortChanges(changes)
	for _, change := range changes {
		switch change.Action {
		case actionCreate:
			summary.Creates++
		case actionUpdate:
			summary.Updates++
		case actionDelete:
			summary.Deletes++
		}
	}
	summary.HasChanges = len(changes) > 0

	planHash, err := computePlanHash(current, desired, resolved, opts)
	if err != nil {
		return PlanResult{}, ResolvedState{}, fmt.Errorf("compute plan hash: %w", err)
	}

	result := PlanResult{
		APIVersion: APIVersion,
		Kind:       Kind,
		PlanHash:   planHash,
		Prune:      opts.Prune,
		Summary:    summary,
		Changes:    changes,
	}
	return result, resolved, nil
}

func planAlerts(current Snapshot, desired *AlertsSection, resolved *ResolvedState) ([]ConfigChange, int, error) {
	changes := make([]ConfigChange, 0, 2)
	unchanged := 0

	if desired.Config != nil {
		nextCfg := normalizeAlertConfigForDeclarative(*desired.Config)
		if nextCfg.Schedule.QuietHours.Enabled && strings.TrimSpace(nextCfg.Schedule.QuietHours.Timezone) != "" {
			if _, err := time.LoadLocation(strings.TrimSpace(nextCfg.Schedule.QuietHours.Timezone)); err != nil {
				return nil, 0, fmt.Errorf("%w: invalid quiet hours timezone %q", ErrInvalidDocument, nextCfg.Schedule.QuietHours.Timezone)
			}
		}

		if current.AlertConfig == nil {
			resolved.AlertsChanged = true
			resolved.AlertConfig = &nextCfg
			changes = append(changes, ConfigChange{
				Domain:  domainAlerts,
				Action:  actionCreate,
				Key:     "config",
				Summary: "Create alert configuration",
			})
		} else {
			prevCfg := normalizeAlertConfigForDeclarative(*current.AlertConfig)
			diffFields, changed, err := diffJSONTopLevelFields(prevCfg, nextCfg)
			if err != nil {
				return nil, 0, fmt.Errorf("diff alert config: %w", err)
			}
			if changed {
				resolved.AlertsChanged = true
				resolved.AlertConfig = &nextCfg
				changes = append(changes, ConfigChange{
					Domain:  domainAlerts,
					Action:  actionUpdate,
					Key:     "config",
					Summary: formatUpdateSummary("Update alert configuration", diffFields),
					Fields:  diffFields,
				})
			} else {
				unchanged++
			}
		}
	}

	if desired.IntentPolicies != nil {
		if err := alerts.ValidateAlertIntentPolicyDocument(*desired.IntentPolicies); err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
		}
		nextDoc := alerts.NormalizeAlertIntentPolicyDocument(*desired.IntentPolicies)
		if current.AlertIntentPolicies == nil {
			resolved.AlertIntentChanged = true
			resolved.AlertIntentPolicies = &nextDoc
			changes = append(changes, ConfigChange{
				Domain:  domainAlerts,
				Action:  actionCreate,
				Key:     "intentPolicies",
				Summary: "Create alert intent policies",
			})
		} else {
			prevDoc := alerts.NormalizeAlertIntentPolicyDocument(*current.AlertIntentPolicies)
			fields := diffAlertIntentPolicies(prevDoc, nextDoc)
			if len(fields) > 0 {
				if nextDoc.Revision <= prevDoc.Revision {
					nextDoc.Revision = prevDoc.Revision + 1
				}
				resolved.AlertIntentChanged = true
				resolved.AlertIntentPolicies = &nextDoc
				changes = append(changes, ConfigChange{
					Domain:  domainAlerts,
					Action:  actionUpdate,
					Key:     "intentPolicies",
					Summary: formatUpdateSummary("Update alert intent policies", fields),
					Fields:  fields,
				})
			} else {
				unchanged++
			}
		}
	}

	return changes, unchanged, nil
}

func diffAlertIntentPolicies(prev, next alerts.AlertIntentPolicyDocument) []string {
	fields := make([]string, 0, 4)
	if prev.SchemaVersion != next.SchemaVersion {
		fields = append(fields, "schemaVersion")
	}
	if !reflect.DeepEqual(prev.Defaults, next.Defaults) {
		fields = append(fields, "defaults")
	}
	if !reflect.DeepEqual(prev.ResourceTypes, next.ResourceTypes) {
		fields = append(fields, "resourceTypes")
	}
	if !reflect.DeepEqual(prev.Resources, next.Resources) {
		fields = append(fields, "resources")
	}
	sort.Strings(fields)
	return fields
}

func planAvailabilityTargets(current []config.AvailabilityTarget, desired []AvailabilityTargetSpec, prune bool, resolved *ResolvedState) ([]ConfigChange, int, error) {
	currentByID := make(map[string]config.AvailabilityTarget, len(current))
	for _, item := range current {
		norm := config.NormalizeAvailabilityTarget(item)
		currentByID[norm.ID] = norm
	}

	seen := make(map[string]struct{}, len(desired))
	finalTargets := make([]config.AvailabilityTarget, 0, len(desired))
	changes := make([]ConfigChange, 0)
	unchanged := 0

	for i, spec := range desired {
		id := strings.TrimSpace(spec.ID)
		if id == "" {
			return nil, 0, fmt.Errorf("%w: availabilityTargets[%d].id is required", ErrInvalidDocument, i)
		}
		if _, dup := seen[id]; dup {
			return nil, 0, fmt.Errorf("%w: duplicate availability target id %q", ErrInvalidDocument, id)
		}
		seen[id] = struct{}{}

		prev, existed := currentByID[id]
		candidate, err := resolveAvailabilityTargetSpec(spec, prev, existed)
		if err != nil {
			return nil, 0, err
		}
		if err := candidate.Validate(); err != nil {
			return nil, 0, fmt.Errorf("%w: availability target %q: %v", ErrInvalidDocument, id, err)
		}

		if !existed {
			candidate.ConfigRevision = 1
			finalTargets = append(finalTargets, candidate)
			changes = append(changes, ConfigChange{
				Domain:  domainAvailabilityTargets,
				Action:  actionCreate,
				Key:     id,
				Summary: fmt.Sprintf("Create availability target %q", candidate.DisplayName()),
			})
			continue
		}

		candidate.ConfigRevision = prev.ConfigRevision
		if candidate.ConfigRevision <= 0 {
			candidate.ConfigRevision = 1
		}
		if config.AvailabilityExecutionConfigChanged(prev, candidate) {
			candidate.ConfigRevision++
		}

		fields := diffAvailabilityTargets(prev, candidate)
		finalTargets = append(finalTargets, candidate)
		if len(fields) > 0 {
			changes = append(changes, ConfigChange{
				Domain:  domainAvailabilityTargets,
				Action:  actionUpdate,
				Key:     id,
				Summary: formatUpdateSummary(fmt.Sprintf("Update availability target %q", candidate.DisplayName()), fields),
				Fields:  fields,
			})
		} else {
			unchanged++
		}
	}

	remainingIDs := make([]string, 0)
	for id := range currentByID {
		if _, ok := seen[id]; !ok {
			remainingIDs = append(remainingIDs, id)
		}
	}
	sort.Strings(remainingIDs)

	for _, id := range remainingIDs {
		prev := currentByID[id]
		if prune {
			changes = append(changes, ConfigChange{
				Domain:  domainAvailabilityTargets,
				Action:  actionDelete,
				Key:     id,
				Summary: fmt.Sprintf("Delete availability target %q", prev.DisplayName()),
			})
		} else {
			finalTargets = append(finalTargets, prev)
			unchanged++
		}
	}

	sort.Slice(finalTargets, func(i, j int) bool {
		if finalTargets[i].ID != finalTargets[j].ID {
			return finalTargets[i].ID < finalTargets[j].ID
		}
		if finalTargets[i].Name != finalTargets[j].Name {
			return finalTargets[i].Name < finalTargets[j].Name
		}
		return finalTargets[i].Address < finalTargets[j].Address
	})

	if len(changes) > 0 {
		resolved.AvailabilityTargetsChanged = true
		resolved.AvailabilityTargets = finalTargets
	}
	return changes, unchanged, nil
}

func resolveAvailabilityTargetSpec(spec AvailabilityTargetSpec, prev config.AvailabilityTarget, existed bool) (config.AvailabilityTarget, error) {
	id := strings.TrimSpace(spec.ID)
	target := config.AvailabilityTarget{
		ID:                            id,
		Name:                          spec.Name,
		TargetKind:                    spec.TargetKind,
		Address:                       spec.Address,
		Protocol:                      spec.Protocol,
		Port:                          spec.Port,
		Path:                          spec.Path,
		UDPMode:                       spec.UDPMode,
		UDPRequest:                    spec.UDPRequest,
		UDPExpected:                   spec.UDPExpected,
		Enabled:                       spec.Enabled,
		PollIntervalSecs:              spec.PollIntervalSecs,
		TimeoutMillis:                 spec.TimeoutMillis,
		FailureThreshold:              spec.FailureThreshold,
		LinkedResourceID:              spec.LinkedResourceID,
		CertificateMonitoringDisabled: spec.CertificateMonitoringDisabled,
		CertificateExpiryWarningDays:  spec.CertificateExpiryWarningDays,
		ObservationLocationIDs:        append([]string(nil), spec.ObservationLocationIDs...),
	}
	if spec.DockerPort != nil {
		dp := *spec.DockerPort
		target.DockerPort = &dp
	}

	if spec.HTTP != nil {
		originCandidate := config.NormalizeAvailabilityTarget(target)
		sameOrigin := existed && prev.HTTP != nil && sameAvailabilityHTTPOrigin(prev, originCandidate)
		denyReason := "has no existing saved value"
		if existed && prev.HTTP != nil && !sameOrigin {
			denyReason = "cannot be preserved because the HTTP endpoint origin changed"
		}

		nextAuthType := config.AvailabilityHTTPAuthType(strings.ToLower(strings.TrimSpace(string(spec.HTTP.Authentication.Type))))
		if nextAuthType == "" {
			nextAuthType = config.AvailabilityHTTPAuthNone
		}

		var prevBody, prevPass, prevBearer string
		prevHeaders := make(map[string]string)
		if sameOrigin && prev.HTTP != nil {
			if prev.HTTP.Body != nil {
				prevBody = *prev.HTTP.Body
			}
			if prev.HTTP.Authentication.Type == nextAuthType {
				if prev.HTTP.Authentication.Password != nil {
					prevPass = *prev.HTTP.Authentication.Password
				}
				if prev.HTTP.Authentication.BearerToken != nil {
					prevBearer = *prev.HTTP.Authentication.BearerToken
				}
			}
			for _, hdr := range prev.HTTP.Headers {
				if hdr.Value != nil {
					prevHeaders[hdr.ID] = *hdr.Value
				}
			}
		}

		bodyVal, err := resolveSecretField(spec.HTTP.Body, prevBody, sameOrigin, denyReason, fmt.Sprintf("availabilityTargets[%s].http.body", id))
		if err != nil {
			return config.AvailabilityTarget{}, err
		}

		authDenyReason := denyReason
		authPreserve := sameOrigin
		if sameOrigin && prev.HTTP != nil && prev.HTTP.Authentication.Type != nextAuthType {
			authPreserve = false
			authDenyReason = "cannot be preserved because HTTP authentication type changed"
		}

		passVal, err := resolveSecretField(spec.HTTP.Authentication.Password, prevPass, authPreserve, authDenyReason, fmt.Sprintf("availabilityTargets[%s].http.authentication.password", id))
		if err != nil {
			return config.AvailabilityTarget{}, err
		}
		bearerVal, err := resolveSecretField(spec.HTTP.Authentication.BearerToken, prevBearer, authPreserve, authDenyReason, fmt.Sprintf("availabilityTargets[%s].http.authentication.bearerToken", id))
		if err != nil {
			return config.AvailabilityTarget{}, err
		}

		httpCfg := &config.AvailabilityHTTPConfig{
			Method: spec.HTTP.Method,
			Authentication: config.AvailabilityHTTPAuthentication{
				Type:     nextAuthType,
				Username: spec.HTTP.Authentication.Username,
			},
			ExpectedStatusMin: spec.HTTP.ExpectedStatusMin,
			ExpectedStatusMax: spec.HTTP.ExpectedStatusMax,
			TextContains:      spec.HTTP.TextContains,
			JSONPath:          spec.HTTP.JSONPath,
			JSONEquals:        spec.HTTP.JSONEquals,
		}
		if bodyVal != "" {
			v := bodyVal
			httpCfg.Body = &v
		}
		if passVal != "" {
			v := passVal
			httpCfg.Authentication.Password = &v
		}
		if bearerVal != "" {
			v := bearerVal
			httpCfg.Authentication.BearerToken = &v
		}

		if len(spec.HTTP.Headers) > 0 {
			httpCfg.Headers = make([]config.AvailabilityHTTPHeader, 0, len(spec.HTTP.Headers))
			for j, hdr := range spec.HTTP.Headers {
				hdrID := strings.TrimSpace(hdr.ID)
				if hdrID == "" {
					return config.AvailabilityTarget{}, fmt.Errorf("%w: availabilityTargets[%s].http.headers[%d].id is required", ErrInvalidDocument, id, j)
				}
				hdrVal, err := resolveSecretField(hdr.Value, prevHeaders[hdrID], sameOrigin, denyReason, fmt.Sprintf("availabilityTargets[%s].http.headers[%s].value", id, hdrID))
				if err != nil {
					return config.AvailabilityTarget{}, err
				}
				resolvedHdr := config.AvailabilityHTTPHeader{
					ID:   hdrID,
					Name: hdr.Name,
				}
				if hdrVal != "" {
					v := hdrVal
					resolvedHdr.Value = &v
				}
				httpCfg.Headers = append(httpCfg.Headers, resolvedHdr)
			}
		}

		target.HTTP = httpCfg
	}

	return config.NormalizeAvailabilityTarget(target), nil
}

func diffAvailabilityTargets(prev, next config.AvailabilityTarget) []string {
	fields := make([]string, 0)
	if prev.Name != next.Name {
		fields = append(fields, "name")
	}
	if prev.TargetKind != next.TargetKind {
		fields = append(fields, "targetKind")
	}
	if prev.Address != next.Address {
		fields = append(fields, "address")
	}
	if prev.Protocol != next.Protocol {
		fields = append(fields, "protocol")
	}
	if prev.Port != next.Port {
		fields = append(fields, "port")
	}
	if prev.Path != next.Path {
		fields = append(fields, "path")
	}
	if prev.UDPMode != next.UDPMode {
		fields = append(fields, "udpMode")
	}
	if prev.UDPRequest != next.UDPRequest {
		fields = append(fields, "udpRequest")
	}
	if prev.UDPExpected != next.UDPExpected {
		fields = append(fields, "udpExpectedResponse")
	}
	if prev.Enabled != next.Enabled {
		fields = append(fields, "enabled")
	}
	if prev.PollIntervalSecs != next.PollIntervalSecs {
		fields = append(fields, "pollIntervalSeconds")
	}
	if prev.TimeoutMillis != next.TimeoutMillis {
		fields = append(fields, "timeoutMillis")
	}
	if prev.FailureThreshold != next.FailureThreshold {
		fields = append(fields, "failureThreshold")
	}
	if prev.LinkedResourceID != next.LinkedResourceID {
		fields = append(fields, "linkedResourceId")
	}
	if prev.CertificateMonitoringDisabled != next.CertificateMonitoringDisabled {
		fields = append(fields, "certificateMonitoringDisabled")
	}
	if prev.CertificateExpiryWarningDays != next.CertificateExpiryWarningDays {
		fields = append(fields, "certificateExpiryWarningDays")
	}
	if !reflect.DeepEqual(prev.EffectiveObservationLocationIDs(), next.EffectiveObservationLocationIDs()) {
		fields = append(fields, "observationLocationIds")
	}
	if !reflect.DeepEqual(prev.DockerPort, next.DockerPort) {
		fields = append(fields, "dockerPort")
	}
	if !reflect.DeepEqual(prev.HTTP, next.HTTP) {
		fields = append(fields, diffAvailabilityHTTP(prev.HTTP, next.HTTP)...)
	}
	sort.Strings(fields)
	return fields
}

func diffAvailabilityHTTP(prev, next *config.AvailabilityHTTPConfig) []string {
	if prev == nil || next == nil {
		return []string{"http"}
	}
	fields := make([]string, 0)
	if prev.Method != next.Method {
		fields = append(fields, "http.method")
	}
	if prev.ExpectedStatusMin != next.ExpectedStatusMin {
		fields = append(fields, "http.expectedStatusMin")
	}
	if prev.ExpectedStatusMax != next.ExpectedStatusMax {
		fields = append(fields, "http.expectedStatusMax")
	}
	if prev.TextContains != next.TextContains {
		fields = append(fields, "http.textContains")
	}
	if prev.JSONPath != next.JSONPath {
		fields = append(fields, "http.jsonPath")
	}
	if prev.JSONEquals != next.JSONEquals {
		fields = append(fields, "http.jsonEquals")
	}
	if stringPtrValue(prev.Body) != stringPtrValue(next.Body) {
		fields = append(fields, "http.body")
	}
	if prev.Authentication.Type != next.Authentication.Type {
		fields = append(fields, "http.authentication.type")
	}
	if prev.Authentication.Username != next.Authentication.Username {
		fields = append(fields, "http.authentication.username")
	}
	if stringPtrValue(prev.Authentication.Password) != stringPtrValue(next.Authentication.Password) {
		fields = append(fields, "http.authentication.password")
	}
	if stringPtrValue(prev.Authentication.BearerToken) != stringPtrValue(next.Authentication.BearerToken) {
		fields = append(fields, "http.authentication.bearerToken")
	}
	if !reflect.DeepEqual(prev.Headers, next.Headers) {
		fields = append(fields, "http.headers")
	}
	return fields
}

func stringPtrValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func planOperatorStates(current []unified.ResourceOperatorState, desired []OperatorStateSpec, prune bool, resolved *ResolvedState) ([]ConfigChange, int, error) {
	currentByID := make(map[string]unified.ResourceOperatorState, len(current))
	for _, state := range current {
		norm := normalizeOperatorStateForDeclarative(state)
		if norm.CanonicalID == "" || norm.IsEmpty() {
			continue
		}
		currentByID[norm.CanonicalID] = norm
	}

	seen := make(map[string]struct{}, len(desired))
	changes := make([]ConfigChange, 0)
	upserts := make([]unified.ResourceOperatorState, 0)
	clears := make([]string, 0)
	unchanged := 0

	for i, spec := range desired {
		id := strings.TrimSpace(spec.CanonicalID)
		if id == "" {
			return nil, 0, fmt.Errorf("%w: operatorStates[%d].canonicalId is required", ErrInvalidDocument, i)
		}
		if _, dup := seen[id]; dup {
			return nil, 0, fmt.Errorf("%w: duplicate operator state canonicalId %q", ErrInvalidDocument, id)
		}
		seen[id] = struct{}{}

		candidate := unified.ResourceOperatorState{
			CanonicalID:           id,
			MonitoringMode:        spec.MonitoringMode,
			LifecycleState:        spec.LifecycleState,
			NeverAutoRemediate:    spec.NeverAutoRemediate,
			AutoRemediationPolicy: cloneAutoRemediationPolicy(spec.AutoRemediationPolicy),
			MaintenanceStartAt:    cloneTimePtrUTC(spec.MaintenanceStartAt),
			MaintenanceEndAt:      cloneTimePtrUTC(spec.MaintenanceEndAt),
			MaintenanceRecurrence: cloneRecurringMaintenanceWindow(spec.MaintenanceRecurrence),
			MaintenanceScope:      spec.MaintenanceScope,
			MaintenanceReason:     spec.MaintenanceReason,
			Criticality:           spec.Criticality,
			Note:                  spec.Note,
		}
		if err := unified.ValidateResourceOperatorState(candidate); err != nil {
			return nil, 0, fmt.Errorf("%w: operator state %q: %v", ErrInvalidDocument, id, err)
		}
		candidate = normalizeOperatorStateForDeclarative(candidate)

		prev, existed := currentByID[id]
		if candidate.IsEmpty() {
			if existed {
				clears = append(clears, id)
				changes = append(changes, ConfigChange{
					Domain:  domainOperatorStates,
					Action:  actionDelete,
					Key:     id,
					Summary: fmt.Sprintf("Clear operator state for %q", id),
				})
			} else {
				unchanged++
			}
			continue
		}

		if !existed {
			upserts = append(upserts, candidate)
			changes = append(changes, ConfigChange{
				Domain:  domainOperatorStates,
				Action:  actionCreate,
				Key:     id,
				Summary: fmt.Sprintf("Set operator state for %q", id),
			})
			continue
		}

		fields := diffOperatorStates(prev, candidate)
		if len(fields) > 0 {
			upserts = append(upserts, candidate)
			changes = append(changes, ConfigChange{
				Domain:  domainOperatorStates,
				Action:  actionUpdate,
				Key:     id,
				Summary: formatUpdateSummary(fmt.Sprintf("Update operator state for %q", id), fields),
				Fields:  fields,
			})
		} else {
			unchanged++
		}
	}

	remainingIDs := make([]string, 0)
	for id := range currentByID {
		if _, ok := seen[id]; !ok {
			remainingIDs = append(remainingIDs, id)
		}
	}
	sort.Strings(remainingIDs)

	for _, id := range remainingIDs {
		if prune {
			clears = append(clears, id)
			changes = append(changes, ConfigChange{
				Domain:  domainOperatorStates,
				Action:  actionDelete,
				Key:     id,
				Summary: fmt.Sprintf("Clear operator state for %q", id),
			})
		} else {
			unchanged++
		}
	}

	sort.Slice(upserts, func(i, j int) bool {
		return upserts[i].CanonicalID < upserts[j].CanonicalID
	})
	sort.Strings(clears)

	if len(upserts) > 0 || len(clears) > 0 {
		resolved.OperatorStatesChanged = true
		resolved.OperatorStatesToUpsert = upserts
		resolved.OperatorStatesToClear = clears
	}
	return changes, unchanged, nil
}

func diffOperatorStates(prev, next unified.ResourceOperatorState) []string {
	fields := make([]string, 0)
	if prev.MonitoringMode != next.MonitoringMode {
		fields = append(fields, "monitoringMode")
	}
	if prev.LifecycleState != next.LifecycleState {
		fields = append(fields, "lifecycleState")
	}
	if prev.NeverAutoRemediate != next.NeverAutoRemediate {
		fields = append(fields, "neverAutoRemediate")
	}
	if !reflect.DeepEqual(prev.AutoRemediationPolicy, next.AutoRemediationPolicy) {
		fields = append(fields, "autoRemediationPolicy")
	}
	if !equalTimePtr(prev.MaintenanceStartAt, next.MaintenanceStartAt) {
		fields = append(fields, "maintenanceStartAt")
	}
	if !equalTimePtr(prev.MaintenanceEndAt, next.MaintenanceEndAt) {
		fields = append(fields, "maintenanceEndAt")
	}
	if !reflect.DeepEqual(prev.MaintenanceRecurrence, next.MaintenanceRecurrence) {
		fields = append(fields, "maintenanceRecurrence")
	}
	if prev.MaintenanceScope != next.MaintenanceScope {
		fields = append(fields, "maintenanceScope")
	}
	if prev.MaintenanceReason != next.MaintenanceReason {
		fields = append(fields, "maintenanceReason")
	}
	if prev.Criticality != next.Criticality {
		fields = append(fields, "criticality")
	}
	if prev.Note != next.Note {
		fields = append(fields, "note")
	}
	sort.Strings(fields)
	return fields
}

func equalTimePtr(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.UTC().Equal(b.UTC())
}

func planNotifications(current Snapshot, desired *NotificationsSection, prune bool, resolved *ResolvedState) ([]ConfigChange, int, error) {
	changes := make([]ConfigChange, 0)
	unchanged := 0

	if desired.Email != nil {
		emailChanges, u, err := planEmail(current.Email, desired.Email, resolved)
		if err != nil {
			return nil, 0, err
		}
		changes = append(changes, emailChanges...)
		unchanged += u
	}

	if desired.Webhooks != nil {
		hookChanges, u, err := planWebhooks(current.Webhooks, *desired.Webhooks, prune, resolved)
		if err != nil {
			return nil, 0, err
		}
		changes = append(changes, hookChanges...)
		unchanged += u
	}

	if desired.Apprise != nil {
		appriseChanges, u, err := planApprise(current.Apprise, desired.Apprise, resolved)
		if err != nil {
			return nil, 0, err
		}
		changes = append(changes, appriseChanges...)
		unchanged += u
	}

	if desired.DeadMan != nil {
		deadManChanges, u, err := planDeadMan(current.DeadMan, desired.DeadMan, resolved)
		if err != nil {
			return nil, 0, err
		}
		changes = append(changes, deadManChanges...)
		unchanged += u
	}

	return changes, unchanged, nil
}

func planEmail(current *notifications.EmailConfig, desired *EmailSpec, resolved *ResolvedState) ([]ConfigChange, int, error) {
	if desired.SMTPPort < 0 || desired.SMTPPort > 65535 {
		return nil, 0, fmt.Errorf("%w: notifications.email.port must be between 0 and 65535", ErrInvalidDocument)
	}
	if desired.RateLimit < 0 {
		return nil, 0, fmt.Errorf("%w: notifications.email.rateLimit must be non-negative", ErrInvalidDocument)
	}

	var savedPass string
	hasSaved := false
	if current != nil {
		savedPass = current.Password
		hasSaved = savedPass != ""
	}
	password, err := resolveSecretField(desired.Password, savedPass, hasSaved, "", "notifications.email.password")
	if err != nil {
		return nil, 0, err
	}

	next := normalizeEmailForDeclarative(notifications.EmailConfig{
		Enabled:         desired.Enabled,
		Provider:        desired.Provider,
		SMTPHost:        desired.SMTPHost,
		SMTPPort:        desired.SMTPPort,
		Username:        desired.Username,
		Password:        password,
		From:            desired.From,
		To:              append([]string(nil), desired.To...),
		TLS:             desired.TLS,
		StartTLS:        desired.StartTLS,
		RateLimit:       desired.RateLimit,
		TagFilter:       append([]string(nil), desired.TagFilter...),
		TagMode:         desired.TagMode,
		MinimumSeverity: desired.MinimumSeverity,
		SubjectPrefix:   desired.SubjectPrefix,
		BrandName:       desired.BrandName,
		FooterNote:      desired.FooterNote,
	})

	if current == nil {
		resolved.EmailChanged = true
		resolved.Email = &next
		return []ConfigChange{{
			Domain:  domainNotifications,
			Action:  actionCreate,
			Key:     "email",
			Summary: "Configure email notifications",
		}}, 0, nil
	}

	prev := normalizeEmailForDeclarative(*current)
	fields := diffEmailConfigs(prev, next)
	if len(fields) > 0 {
		resolved.EmailChanged = true
		resolved.Email = &next
		return []ConfigChange{{
			Domain:  domainNotifications,
			Action:  actionUpdate,
			Key:     "email",
			Summary: formatUpdateSummary("Update email notifications", fields),
			Fields:  fields,
		}}, 0, nil
	}
	return nil, 1, nil
}

func diffEmailConfigs(prev, next notifications.EmailConfig) []string {
	fields := make([]string, 0)
	if prev.Enabled != next.Enabled {
		fields = append(fields, "enabled")
	}
	if prev.Provider != next.Provider {
		fields = append(fields, "provider")
	}
	if prev.SMTPHost != next.SMTPHost {
		fields = append(fields, "server")
	}
	if prev.SMTPPort != next.SMTPPort {
		fields = append(fields, "port")
	}
	if prev.Username != next.Username {
		fields = append(fields, "username")
	}
	if prev.Password != next.Password {
		fields = append(fields, "password")
	}
	if prev.From != next.From {
		fields = append(fields, "from")
	}
	if !reflect.DeepEqual(prev.To, next.To) {
		fields = append(fields, "to")
	}
	if prev.TLS != next.TLS {
		fields = append(fields, "tls")
	}
	if prev.StartTLS != next.StartTLS {
		fields = append(fields, "startTLS")
	}
	if prev.RateLimit != next.RateLimit {
		fields = append(fields, "rateLimit")
	}
	if !reflect.DeepEqual(prev.TagFilter, next.TagFilter) {
		fields = append(fields, "tagFilter")
	}
	if prev.TagMode != next.TagMode {
		fields = append(fields, "tagFilterMode")
	}
	if prev.MinimumSeverity != next.MinimumSeverity {
		fields = append(fields, "minimumSeverity")
	}
	if prev.SubjectPrefix != next.SubjectPrefix {
		fields = append(fields, "subjectPrefix")
	}
	if prev.BrandName != next.BrandName {
		fields = append(fields, "brandName")
	}
	if prev.FooterNote != next.FooterNote {
		fields = append(fields, "footerNote")
	}
	sort.Strings(fields)
	return fields
}

func planWebhooks(current []notifications.WebhookConfig, desired []WebhookSpec, prune bool, resolved *ResolvedState) ([]ConfigChange, int, error) {
	currentByID := make(map[string]notifications.WebhookConfig, len(current))
	for _, hook := range current {
		norm := normalizeWebhookForDeclarative(hook)
		currentByID[norm.ID] = norm
	}

	seen := make(map[string]struct{}, len(desired))
	finalHooks := make([]notifications.WebhookConfig, 0, len(desired))
	changes := make([]ConfigChange, 0)
	unchanged := 0

	for i, spec := range desired {
		id := strings.TrimSpace(spec.ID)
		if id == "" {
			return nil, 0, fmt.Errorf("%w: notifications.webhooks[%d].id is required", ErrInvalidDocument, i)
		}
		if _, dup := seen[id]; dup {
			return nil, 0, fmt.Errorf("%w: duplicate webhook id %q", ErrInvalidDocument, id)
		}
		seen[id] = struct{}{}

		prev, existed := currentByID[id]
		resolvedHook, err := resolveWebhookSpec(spec, prev, existed)
		if err != nil {
			return nil, 0, err
		}
		if err := validateWebhookSpec(resolvedHook); err != nil {
			return nil, 0, fmt.Errorf("%w: webhook %q: %v", ErrInvalidDocument, id, err)
		}

		finalHooks = append(finalHooks, resolvedHook)
		if !existed {
			changes = append(changes, ConfigChange{
				Domain:  domainNotifications,
				Action:  actionCreate,
				Key:     "webhook:" + id,
				Summary: fmt.Sprintf("Create webhook %q", resolvedHook.Name),
			})
			continue
		}

		fields := diffWebhookConfigs(prev, resolvedHook)
		if len(fields) > 0 {
			changes = append(changes, ConfigChange{
				Domain:  domainNotifications,
				Action:  actionUpdate,
				Key:     "webhook:" + id,
				Summary: formatUpdateSummary(fmt.Sprintf("Update webhook %q", resolvedHook.Name), fields),
				Fields:  fields,
			})
		} else {
			unchanged++
		}
	}

	remainingIDs := make([]string, 0)
	for id := range currentByID {
		if _, ok := seen[id]; !ok {
			remainingIDs = append(remainingIDs, id)
		}
	}
	sort.Strings(remainingIDs)

	for _, id := range remainingIDs {
		prev := currentByID[id]
		if prune {
			changes = append(changes, ConfigChange{
				Domain:  domainNotifications,
				Action:  actionDelete,
				Key:     "webhook:" + id,
				Summary: fmt.Sprintf("Delete webhook %q", prev.Name),
			})
		} else {
			finalHooks = append(finalHooks, prev)
			unchanged++
		}
	}

	sort.Slice(finalHooks, func(i, j int) bool {
		if finalHooks[i].ID != finalHooks[j].ID {
			return finalHooks[i].ID < finalHooks[j].ID
		}
		return finalHooks[i].Name < finalHooks[j].Name
	})

	if err := notifications.ValidateWebhookInventory(finalHooks); err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}

	if len(changes) > 0 {
		resolved.WebhooksChanged = true
		resolved.Webhooks = finalHooks
	}
	return changes, unchanged, nil
}

func resolveWebhookSpec(spec WebhookSpec, prev notifications.WebhookConfig, existed bool) (notifications.WebhookConfig, error) {
	id := strings.TrimSpace(spec.ID)
	urlVal, err := resolveSecretField(spec.URL, prev.URL, existed, "", fmt.Sprintf("notifications.webhooks[%s].url", id))
	if err != nil {
		return notifications.WebhookConfig{}, err
	}
	signingVal, err := resolveSecretField(spec.SigningSecret, prev.SigningSecret, existed, "", fmt.Sprintf("notifications.webhooks[%s].signingSecret", id))
	if err != nil {
		return notifications.WebhookConfig{}, err
	}

	var headers map[string]string
	if len(spec.Headers) > 0 {
		headers = make(map[string]string, len(spec.Headers))
		for _, k := range sortedMapKeys(spec.Headers) {
			field := spec.Headers[k]
			saved := lookupHeaderValue(prev.Headers, k)
			val, err := resolveSecretField(&field, saved, existed, "", fmt.Sprintf("notifications.webhooks[%s].headers[%s]", id, k))
			if err != nil {
				return notifications.WebhookConfig{}, err
			}
			if val != "" {
				headers[k] = val
			}
		}
	}

	var customFields map[string]string
	if len(spec.CustomFields) > 0 {
		customFields = make(map[string]string, len(spec.CustomFields))
		for _, k := range sortedMapKeys(spec.CustomFields) {
			field := spec.CustomFields[k]
			saved := prev.CustomFields[k]
			val, err := resolveSecretField(&field, saved, existed, "", fmt.Sprintf("notifications.webhooks[%s].customFields[%s]", id, k))
			if err != nil {
				return notifications.WebhookConfig{}, err
			}
			if val != "" {
				customFields[k] = val
			}
		}
	}

	return normalizeWebhookForDeclarative(notifications.WebhookConfig{
		ID:              id,
		Name:            spec.Name,
		URL:             urlVal,
		Method:          spec.Method,
		Headers:         headers,
		Enabled:         spec.Enabled,
		Service:         spec.Service,
		Template:        spec.Template,
		CustomFields:    customFields,
		Mention:         spec.Mention,
		TagFilter:       append([]string(nil), spec.TagFilter...),
		TagMode:         spec.TagMode,
		MinimumSeverity: spec.MinimumSeverity,
		SigningSecret:   signingVal,
	}), nil
}

func lookupHeaderValue(headers map[string]string, key string) string {
	if len(headers) == 0 {
		return ""
	}
	if v, ok := headers[key]; ok {
		return v
	}
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func validateWebhookSpec(hook notifications.WebhookConfig) error {
	if strings.TrimSpace(hook.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(hook.URL) == "" {
		return fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(hook.URL)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") || strings.TrimSpace(parsed.Hostname()) == "" {
		return fmt.Errorf("url must be a valid http or https URL")
	}
	return nil
}

func diffWebhookConfigs(prev, next notifications.WebhookConfig) []string {
	fields := make([]string, 0)
	if prev.Name != next.Name {
		fields = append(fields, "name")
	}
	if prev.URL != next.URL {
		fields = append(fields, "url")
	}
	if prev.Method != next.Method {
		fields = append(fields, "method")
	}
	if !reflect.DeepEqual(prev.Headers, next.Headers) {
		fields = append(fields, "headers")
	}
	if prev.Enabled != next.Enabled {
		fields = append(fields, "enabled")
	}
	if prev.Service != next.Service {
		fields = append(fields, "service")
	}
	if prev.Template != next.Template {
		fields = append(fields, "template")
	}
	if !reflect.DeepEqual(prev.CustomFields, next.CustomFields) {
		fields = append(fields, "customFields")
	}
	if prev.Mention != next.Mention {
		fields = append(fields, "mention")
	}
	if !reflect.DeepEqual(prev.TagFilter, next.TagFilter) {
		fields = append(fields, "tagFilter")
	}
	if prev.TagMode != next.TagMode {
		fields = append(fields, "tagFilterMode")
	}
	if prev.MinimumSeverity != next.MinimumSeverity {
		fields = append(fields, "minimumSeverity")
	}
	if prev.SigningSecret != next.SigningSecret {
		fields = append(fields, "signingSecret")
	}
	sort.Strings(fields)
	return fields
}

func planApprise(current *notifications.AppriseConfig, desired *AppriseSpec, resolved *ResolvedState) ([]ConfigChange, int, error) {
	mode := strings.ToLower(strings.TrimSpace(string(desired.Mode)))
	if mode != "" && mode != string(notifications.AppriseModeCLI) && mode != string(notifications.AppriseModeHTTP) {
		return nil, 0, fmt.Errorf("%w: notifications.apprise.mode must be cli or http", ErrInvalidDocument)
	}

	var prevNorm notifications.AppriseConfig
	hasPrev := false
	if current != nil {
		prevNorm = notifications.NormalizeAppriseConfig(*current)
		hasPrev = true
	}

	configKey, err := resolveSecretField(desired.ConfigKey, prevNorm.ConfigKey, hasPrev, "", "notifications.apprise.configKey")
	if err != nil {
		return nil, 0, err
	}
	apiKey, err := resolveSecretField(desired.APIKey, prevNorm.APIKey, hasPrev, "", "notifications.apprise.apiKey")
	if err != nil {
		return nil, 0, err
	}

	targets := make([]string, 0, len(desired.Targets))
	for i := range desired.Targets {
		field := desired.Targets[i]
		savedTarget := ""
		if hasPrev && i < len(prevNorm.Targets) {
			savedTarget = prevNorm.Targets[i]
		}
		targetVal, err := resolveSecretField(&field, savedTarget, hasPrev && savedTarget != "", "", fmt.Sprintf("notifications.apprise.targets[%d]", i))
		if err != nil {
			return nil, 0, err
		}
		trimmed := strings.TrimSpace(targetVal)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			return nil, 0, fmt.Errorf("%w: notifications.apprise.targets[%d] cannot start with a dash", ErrInvalidDocument, i)
		}
		targets = append(targets, trimmed)
	}

	next := notifications.NormalizeAppriseConfig(notifications.AppriseConfig{
		Enabled:         desired.Enabled,
		Mode:            desired.Mode,
		Targets:         targets,
		CLIPath:         desired.CLIPath,
		TimeoutSeconds:  desired.TimeoutSeconds,
		ServerURL:       desired.ServerURL,
		ConfigKey:       configKey,
		APIKey:          apiKey,
		APIKeyHeader:    desired.APIKeyHeader,
		SkipTLSVerify:   desired.SkipTLSVerify,
		MinimumSeverity: desired.MinimumSeverity,
	})

	if !hasPrev {
		resolved.AppriseChanged = true
		resolved.Apprise = &next
		return []ConfigChange{{
			Domain:  domainNotifications,
			Action:  actionCreate,
			Key:     "apprise",
			Summary: "Configure Apprise notifications",
		}}, 0, nil
	}

	fields := diffAppriseConfigs(prevNorm, next)
	if len(fields) > 0 {
		resolved.AppriseChanged = true
		resolved.Apprise = &next
		return []ConfigChange{{
			Domain:  domainNotifications,
			Action:  actionUpdate,
			Key:     "apprise",
			Summary: formatUpdateSummary("Update Apprise notifications", fields),
			Fields:  fields,
		}}, 0, nil
	}
	return nil, 1, nil
}

func diffAppriseConfigs(prev, next notifications.AppriseConfig) []string {
	fields := make([]string, 0)
	if prev.Enabled != next.Enabled {
		fields = append(fields, "enabled")
	}
	if prev.Mode != next.Mode {
		fields = append(fields, "mode")
	}
	if !reflect.DeepEqual(prev.Targets, next.Targets) {
		fields = append(fields, "targets")
	}
	if prev.CLIPath != next.CLIPath {
		fields = append(fields, "cliPath")
	}
	if prev.TimeoutSeconds != next.TimeoutSeconds {
		fields = append(fields, "timeoutSeconds")
	}
	if prev.ServerURL != next.ServerURL {
		fields = append(fields, "serverUrl")
	}
	if prev.ConfigKey != next.ConfigKey {
		fields = append(fields, "configKey")
	}
	if prev.APIKey != next.APIKey {
		fields = append(fields, "apiKey")
	}
	if prev.APIKeyHeader != next.APIKeyHeader {
		fields = append(fields, "apiKeyHeader")
	}
	if prev.SkipTLSVerify != next.SkipTLSVerify {
		fields = append(fields, "skipTlsVerify")
	}
	if prev.MinimumSeverity != next.MinimumSeverity {
		fields = append(fields, "minimumSeverity")
	}
	sort.Strings(fields)
	return fields
}

func planDeadMan(current *notifications.DeadManConfig, desired *DeadManSpec, resolved *ResolvedState) ([]ConfigChange, int, error) {
	var prevNorm notifications.DeadManConfig
	if current != nil {
		prevNorm = notifications.NormalizeDeadManConfig(*current)
	}

	pingURL, err := resolveSecretField(desired.PingURL, prevNorm.PingURL, prevNorm.PingURL != "", "", "notifications.deadMan.pingUrl")
	if err != nil {
		return nil, 0, err
	}

	next := notifications.NormalizeDeadManConfig(notifications.DeadManConfig{
		PingURL: pingURL,
	})
	if err := notifications.ValidateDeadManPingURL(next.PingURL); err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}

	if prevNorm.PingURL == next.PingURL {
		return nil, 1, nil
	}

	resolved.DeadManChanged = true
	resolved.DeadMan = &next

	switch {
	case prevNorm.PingURL == "" && next.PingURL != "":
		return []ConfigChange{{
			Domain:  domainNotifications,
			Action:  actionCreate,
			Key:     "deadMan",
			Summary: "Configure dead-man watchdog ping URL",
			Fields:  []string{"pingUrl"},
		}}, 0, nil
	case prevNorm.PingURL != "" && next.PingURL == "":
		return []ConfigChange{{
			Domain:  domainNotifications,
			Action:  actionDelete,
			Key:     "deadMan",
			Summary: "Clear dead-man watchdog ping URL",
			Fields:  []string{"pingUrl"},
		}}, 0, nil
	default:
		return []ConfigChange{{
			Domain:  domainNotifications,
			Action:  actionUpdate,
			Key:     "deadMan",
			Summary: "Update dead-man watchdog ping URL",
			Fields:  []string{"pingUrl"},
		}}, 0, nil
	}
}

func diffJSONTopLevelFields(prev, next any) ([]string, bool, error) {
	prevMap, err := toJSONObject(prev)
	if err != nil {
		return nil, false, err
	}
	nextMap, err := toJSONObject(next)
	if err != nil {
		return nil, false, err
	}
	keySet := make(map[string]struct{}, len(prevMap)+len(nextMap))
	for k := range prevMap {
		keySet[k] = struct{}{}
	}
	for k := range nextMap {
		keySet[k] = struct{}{}
	}
	fields := make([]string, 0)
	for k := range keySet {
		if !reflect.DeepEqual(prevMap[k], nextMap[k]) {
			fields = append(fields, k)
		}
	}
	sort.Strings(fields)
	return fields, len(fields) > 0, nil
}

func toJSONObject(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func formatUpdateSummary(prefix string, fields []string) string {
	if len(fields) == 0 {
		return prefix
	}
	if len(fields) <= 3 {
		return fmt.Sprintf("%s (%s)", prefix, strings.Join(fields, ", "))
	}
	return fmt.Sprintf("%s (%s, and %d more)", prefix, strings.Join(fields[:3], ", "), len(fields)-3)
}

func domainRank(domain string) int {
	switch domain {
	case domainAlerts:
		return 0
	case domainAvailabilityTargets:
		return 1
	case domainOperatorStates:
		return 2
	case domainNotifications:
		return 3
	default:
		return 4
	}
}

func sortChanges(changes []ConfigChange) {
	sort.Slice(changes, func(i, j int) bool {
		di, dj := domainRank(changes[i].Domain), domainRank(changes[j].Domain)
		if di != dj {
			return di < dj
		}
		if changes[i].Key != changes[j].Key {
			return changes[i].Key < changes[j].Key
		}
		return changes[i].Action < changes[j].Action
	})
}

type planHashInput struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Prune      bool             `json:"prune"`
	Sections   planHashSections `json:"sections"`
	Current    normalizedSnap   `json:"current"`
	Resolved   ResolvedState    `json:"resolved"`
}

type planHashSections struct {
	AlertConfig         bool `json:"alertConfig"`
	AlertIntentPolicies bool `json:"alertIntentPolicies"`
	AvailabilityTargets bool `json:"availabilityTargets"`
	OperatorStates      bool `json:"operatorStates"`
	Email               bool `json:"email"`
	Webhooks            bool `json:"webhooks"`
	Apprise             bool `json:"apprise"`
	DeadMan             bool `json:"deadMan"`
}

type normalizedSnap struct {
	AlertConfig         *alerts.AlertConfig               `json:"alertConfig,omitempty"`
	AlertIntentPolicies *alerts.AlertIntentPolicyDocument `json:"alertIntentPolicies,omitempty"`
	AvailabilityTargets []config.AvailabilityTarget       `json:"availabilityTargets,omitempty"`
	OperatorStates      []unified.ResourceOperatorState   `json:"operatorStates,omitempty"`
	Email               *notifications.EmailConfig        `json:"email,omitempty"`
	Webhooks            []notifications.WebhookConfig     `json:"webhooks,omitempty"`
	Apprise             *notifications.AppriseConfig      `json:"apprise,omitempty"`
	DeadMan             *notifications.DeadManConfig      `json:"deadMan,omitempty"`
}

func normalizeSnapshotForHash(snap Snapshot) normalizedSnap {
	out := normalizedSnap{}
	if snap.AlertConfig != nil {
		cfg := normalizeAlertConfigForDeclarative(*snap.AlertConfig)
		out.AlertConfig = &cfg
	}
	if snap.AlertIntentPolicies != nil {
		doc := alerts.NormalizeAlertIntentPolicyDocument(*snap.AlertIntentPolicies)
		out.AlertIntentPolicies = &doc
	}
	if len(snap.AvailabilityTargets) > 0 {
		targets := make([]config.AvailabilityTarget, 0, len(snap.AvailabilityTargets))
		for _, t := range snap.AvailabilityTargets {
			targets = append(targets, config.NormalizeAvailabilityTarget(t))
		}
		sort.Slice(targets, func(i, j int) bool {
			if targets[i].ID != targets[j].ID {
				return targets[i].ID < targets[j].ID
			}
			return targets[i].Name < targets[j].Name
		})
		out.AvailabilityTargets = targets
	}
	if len(snap.OperatorStates) > 0 {
		states := make([]unified.ResourceOperatorState, 0, len(snap.OperatorStates))
		for _, s := range snap.OperatorStates {
			norm := normalizeOperatorStateForDeclarative(s)
			if norm.CanonicalID == "" || norm.IsEmpty() {
				continue
			}
			states = append(states, norm)
		}
		sort.Slice(states, func(i, j int) bool {
			return states[i].CanonicalID < states[j].CanonicalID
		})
		out.OperatorStates = states
	}
	if snap.Email != nil {
		email := normalizeEmailForDeclarative(*snap.Email)
		out.Email = &email
	}
	if len(snap.Webhooks) > 0 {
		hooks := make([]notifications.WebhookConfig, 0, len(snap.Webhooks))
		for _, h := range snap.Webhooks {
			hooks = append(hooks, normalizeWebhookForDeclarative(h))
		}
		sort.Slice(hooks, func(i, j int) bool {
			if hooks[i].ID != hooks[j].ID {
				return hooks[i].ID < hooks[j].ID
			}
			return hooks[i].Name < hooks[j].Name
		})
		out.Webhooks = hooks
	}
	if snap.Apprise != nil {
		apprise := notifications.NormalizeAppriseConfig(*snap.Apprise)
		out.Apprise = &apprise
	}
	if snap.DeadMan != nil {
		dm := notifications.NormalizeDeadManConfig(*snap.DeadMan)
		out.DeadMan = &dm
	}
	return out
}

func computePlanHash(current Snapshot, desired Document, resolved ResolvedState, opts PlanOptions) (string, error) {
	sections := planHashSections{
		AlertConfig:         desired.Alerts != nil && desired.Alerts.Config != nil,
		AlertIntentPolicies: desired.Alerts != nil && desired.Alerts.IntentPolicies != nil,
		AvailabilityTargets: desired.AvailabilityTargets != nil,
		OperatorStates:      desired.OperatorStates != nil,
		Email:               desired.Notifications != nil && desired.Notifications.Email != nil,
		Webhooks:            desired.Notifications != nil && desired.Notifications.Webhooks != nil,
		Apprise:             desired.Notifications != nil && desired.Notifications.Apprise != nil,
		DeadMan:             desired.Notifications != nil && desired.Notifications.DeadMan != nil,
	}

	payload := planHashInput{
		APIVersion: APIVersion,
		Kind:       Kind,
		Prune:      opts.Prune,
		Sections:   sections,
		Current:    normalizeSnapshotForHash(current),
		Resolved:   resolved,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
