package actionplanner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const DefaultPlanTTL = 5 * time.Minute

var ErrCapabilityNotFound = errors.New("resource capability is not advertised")

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func AsValidationError(err error) (*ValidationError, bool) {
	var target *ValidationError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

type Planner struct {
	Now func() time.Time
	TTL time.Duration
}

func (p Planner) Plan(req unified.ActionRequest, resource unified.Resource) (unified.ActionPlan, error) {
	req = normalizeRequest(req)
	if err := validateRequest(req); err != nil {
		return unified.ActionPlan{}, err
	}

	resourceID := unified.CanonicalResourceID(resource.ID)
	if resourceID == "" {
		return unified.ActionPlan{}, &ValidationError{Field: "resourceId", Message: "resource has no canonical id"}
	}
	if req.ResourceID != resourceID {
		return unified.ActionPlan{}, &ValidationError{Field: "resourceId", Message: "request resource does not match planned resource"}
	}

	capability, ok := findCapability(resource.Capabilities, req.CapabilityName)
	if !ok {
		return unified.ActionPlan{}, ErrCapabilityNotFound
	}
	if err := validateParams(req.Params, capability.Params); err != nil {
		return unified.ActionPlan{}, err
	}

	plannedAt := p.now()
	ttl := p.ttl()
	policy := normalizeApprovalPolicy(capability.MinimumApprovalLevel)
	requiresApproval := policy == unified.ApprovalAdmin || policy == unified.ApprovalMultiFactor
	resourceVersion := ResourceVersion(resource)
	policyVersion := PolicyVersion(capability)
	actionID := actionID(req, resourceVersion, policyVersion)

	plan := unified.ActionPlan{
		ActionID:             actionID,
		RequestID:            req.RequestID,
		Allowed:              true,
		RequiresApproval:     requiresApproval,
		ApprovalPolicy:       policy,
		PredictedBlastRadius: predictedBlastRadius(resource),
		RollbackAvailable:    false,
		Message:              planMessage(resource, capability, policy),
		PlannedAt:            plannedAt,
		ExpiresAt:            plannedAt.Add(ttl),
		ResourceVersion:      resourceVersion,
		PolicyVersion:        policyVersion,
		Preflight:            buildPreflight(resource, capability, req, actionID, policy, plannedAt),
	}
	plan.PlanHash = planHash(req, plan)
	plan.Preflight = unified.NormalizeActionPreflight(plan.Preflight, req, plan)

	return plan, nil
}

func ResourceVersion(resource unified.Resource) string {
	payload := struct {
		ID             string                                  `json:"id"`
		Type           unified.ResourceType                    `json:"type"`
		Technology     string                                  `json:"technology,omitempty"`
		Name           string                                  `json:"name"`
		Status         unified.ResourceStatus                  `json:"status"`
		AISafeSummary  string                                  `json:"aiSafeSummary,omitempty"`
		Sources        []unified.DataSource                    `json:"sources,omitempty"`
		Identity       normalizedIdentity                      `json:"identity,omitempty"`
		ParentID       string                                  `json:"parentId,omitempty"`
		Capabilities   []normalizedCapabilityForResourceHash   `json:"capabilities,omitempty"`
		Relationships  []normalizedRelationship                `json:"relationships,omitempty"`
		RecentChanges  []normalizedChange                      `json:"recentChanges,omitempty"`
		IncidentCode   string                                  `json:"incidentCode,omitempty"`
		IncidentStatus storageIncidentHashFields               `json:"incidentStatus,omitempty"`
		SourceStatus   map[unified.DataSource]sourceStatusHash `json:"sourceStatus,omitempty"`
	}{
		ID:            unified.CanonicalResourceID(resource.ID),
		Type:          unified.CanonicalResourceType(resource.Type),
		Technology:    strings.TrimSpace(resource.Technology),
		Name:          strings.TrimSpace(resource.Name),
		Status:        resource.Status,
		AISafeSummary: strings.TrimSpace(resource.AISafeSummary),
		Sources:       normalizeDataSources(resource.Sources),
		Identity:      normalizeIdentity(resource.Identity),
		Capabilities:  normalizeCapabilitiesForResourceHash(resource.Capabilities),
		Relationships: normalizeRelationships(unified.ResourceRelationshipsWithCanonicalParent(resource)),
		RecentChanges: normalizeChanges(resource.RecentChanges),
		IncidentCode:  strings.TrimSpace(resource.IncidentCode),
		IncidentStatus: storageIncidentHashFields{
			Severity: string(resource.IncidentSeverity),
			Summary:  strings.TrimSpace(resource.IncidentSummary),
			Category: strings.TrimSpace(resource.IncidentCategory),
			Priority: resource.IncidentPriority,
		},
		SourceStatus: normalizeSourceStatus(resource.SourceStatus),
	}
	if resource.ParentID != nil {
		payload.ParentID = unified.CanonicalResourceID(*resource.ParentID)
	}
	return "resource:sha256:" + hashJSON(payload, 12)
}

func PolicyVersion(capability unified.ResourceCapability) string {
	payload := struct {
		Name                 string                      `json:"name"`
		Type                 unified.CapabilityType      `json:"type"`
		Description          string                      `json:"description"`
		MinimumApprovalLevel unified.ActionApprovalLevel `json:"minimumApprovalLevel"`
		Platform             string                      `json:"platform,omitempty"`
		InternalHandler      string                      `json:"internalHandler,omitempty"`
		Params               []unified.CapabilityParam   `json:"params,omitempty"`
		NormalizedPolicy     unified.ActionApprovalLevel `json:"normalizedPolicy"`
		ParamNames           []string                    `json:"paramNames,omitempty"`
	}{
		Name:                 strings.TrimSpace(capability.Name),
		Type:                 capability.Type,
		Description:          strings.TrimSpace(capability.Description),
		MinimumApprovalLevel: capability.MinimumApprovalLevel,
		Platform:             strings.TrimSpace(capability.Platform),
		InternalHandler:      strings.TrimSpace(capability.InternalHandler),
		Params:               normalizeCapabilityParams(capability.Params),
		NormalizedPolicy:     normalizeApprovalPolicy(capability.MinimumApprovalLevel),
		ParamNames:           sortedCapabilityParamNames(capability.Params),
	}
	return "policy:sha256:" + hashJSON(payload, 12)
}

func (p Planner) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

func (p Planner) ttl() time.Duration {
	if p.TTL > 0 {
		return p.TTL
	}
	return DefaultPlanTTL
}

func normalizeRequest(req unified.ActionRequest) unified.ActionRequest {
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ResourceID = unified.CanonicalResourceID(req.ResourceID)
	req.CapabilityName = strings.TrimSpace(req.CapabilityName)
	req.Reason = strings.TrimSpace(req.Reason)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	return req
}

func validateRequest(req unified.ActionRequest) error {
	if req.RequestID == "" {
		return &ValidationError{Field: "requestId", Message: "request id is required"}
	}
	if req.ResourceID == "" {
		return &ValidationError{Field: "resourceId", Message: "resource id is required"}
	}
	if req.CapabilityName == "" {
		return &ValidationError{Field: "capabilityName", Message: "capability name is required"}
	}
	if req.Reason == "" {
		return &ValidationError{Field: "reason", Message: "reason is required"}
	}
	if req.RequestedBy == "" {
		return &ValidationError{Field: "requestedBy", Message: "requester is required"}
	}
	return nil
}

func findCapability(capabilities []unified.ResourceCapability, name string) (unified.ResourceCapability, bool) {
	name = strings.TrimSpace(name)
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == name {
			return capability, true
		}
	}
	return unified.ResourceCapability{}, false
}

func validateParams(params map[string]any, specs []unified.CapabilityParam) error {
	specByName := make(map[string]unified.CapabilityParam, len(specs))
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return &ValidationError{Field: "params", Message: "capability declares an unnamed parameter"}
		}
		if _, exists := specByName[name]; exists {
			return &ValidationError{Field: "params." + name, Message: "capability declares duplicate parameter"}
		}
		spec.Name = name
		specByName[name] = spec
	}

	for name := range params {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || trimmed != name {
			return &ValidationError{Field: "params", Message: "parameter names must be non-empty and trimmed"}
		}
		if _, ok := specByName[name]; !ok {
			return &ValidationError{Field: "params." + name, Message: "parameter is not declared by this capability"}
		}
	}

	for _, spec := range specByName {
		value, exists := params[spec.Name]
		if !exists || isEmptyParamValue(value) {
			if spec.Required {
				return &ValidationError{Field: "params." + spec.Name, Message: "required parameter is missing"}
			}
			continue
		}
		if err := validateParamValue(value, spec); err != nil {
			return err
		}
	}
	return nil
}

func isEmptyParamValue(value any) bool {
	if value == nil {
		return true
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

func validateParamValue(value any, spec unified.CapabilityParam) error {
	if err := validateParamType(value, spec); err != nil {
		return err
	}
	if len(spec.Enum) > 0 {
		valueString := enumString(value)
		for _, candidate := range spec.Enum {
			if valueString == strings.TrimSpace(candidate) {
				return nil
			}
		}
		return &ValidationError{Field: "params." + spec.Name, Message: "parameter value is outside the allowed enum"}
	}
	if spec.Pattern != "" {
		valueString, ok := value.(string)
		if !ok {
			return &ValidationError{Field: "params." + spec.Name, Message: "pattern validation requires a string value"}
		}
		matched, err := regexp.MatchString(spec.Pattern, valueString)
		if err != nil {
			return &ValidationError{Field: "params." + spec.Name, Message: "capability declares an invalid pattern"}
		}
		if !matched {
			return &ValidationError{Field: "params." + spec.Name, Message: "parameter value does not match the required pattern"}
		}
	}
	return nil
}

func validateParamType(value any, spec unified.CapabilityParam) error {
	typ := strings.ToLower(strings.TrimSpace(spec.Type))
	switch typ {
	case "", "any":
		return nil
	case "string":
		if _, ok := value.(string); ok {
			return nil
		}
	case "bool", "boolean":
		if _, ok := value.(bool); ok {
			return nil
		}
	case "int", "integer":
		if isInteger(value) {
			return nil
		}
	case "number", "float", "float64":
		if isNumber(value) {
			return nil
		}
	case "object", "map":
		if isMap(value) {
			return nil
		}
	case "array", "list":
		if isSlice(value) {
			return nil
		}
	default:
		return &ValidationError{Field: "params." + spec.Name, Message: "capability declares unsupported parameter type " + typ}
	}
	return &ValidationError{Field: "params." + spec.Name, Message: "parameter must be " + typ}
}

func isInteger(value any) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return math.Trunc(float64(v)) == float64(v)
	case float64:
		return math.Trunc(v) == v
	case json.Number:
		_, err := v.Int64()
		return err == nil
	default:
		return false
	}
}

func isNumber(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		return true
	default:
		return false
	}
}

func isMap(value any) bool {
	if _, ok := value.(map[string]any); ok {
		return true
	}
	rv := reflect.ValueOf(value)
	return rv.IsValid() && rv.Kind() == reflect.Map
}

func isSlice(value any) bool {
	rv := reflect.ValueOf(value)
	return rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array)
}

func enumString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(value)
	}
}

func normalizeApprovalPolicy(level unified.ActionApprovalLevel) unified.ActionApprovalLevel {
	switch level {
	case unified.ApprovalNone, unified.ApprovalDryRun, unified.ApprovalAdmin, unified.ApprovalMultiFactor:
		return level
	default:
		return unified.ApprovalAdmin
	}
}

func actionID(req unified.ActionRequest, resourceVersion string, policyVersion string) string {
	payload := struct {
		RequestID       string         `json:"requestId"`
		ResourceID      string         `json:"resourceId"`
		CapabilityName  string         `json:"capabilityName"`
		Params          map[string]any `json:"params"`
		Reason          string         `json:"reason"`
		RequestedBy     string         `json:"requestedBy"`
		ResourceVersion string         `json:"resourceVersion"`
		PolicyVersion   string         `json:"policyVersion"`
	}{
		RequestID:       req.RequestID,
		ResourceID:      req.ResourceID,
		CapabilityName:  req.CapabilityName,
		Params:          req.Params,
		Reason:          req.Reason,
		RequestedBy:     req.RequestedBy,
		ResourceVersion: resourceVersion,
		PolicyVersion:   policyVersion,
	}
	return "act_" + hashJSON(payload, 16)
}

func planHash(req unified.ActionRequest, plan unified.ActionPlan) string {
	payload := struct {
		ActionID             string                      `json:"actionId"`
		Request              unified.ActionRequest       `json:"request"`
		Allowed              bool                        `json:"allowed"`
		RequiresApproval     bool                        `json:"requiresApproval"`
		ApprovalPolicy       unified.ActionApprovalLevel `json:"approvalPolicy"`
		PredictedBlastRadius []string                    `json:"predictedBlastRadius"`
		RollbackAvailable    bool                        `json:"rollbackAvailable"`
		ResourceVersion      string                      `json:"resourceVersion"`
		PolicyVersion        string                      `json:"policyVersion"`
	}{
		ActionID:             plan.ActionID,
		Request:              req,
		Allowed:              plan.Allowed,
		RequiresApproval:     plan.RequiresApproval,
		ApprovalPolicy:       plan.ApprovalPolicy,
		PredictedBlastRadius: append([]string(nil), plan.PredictedBlastRadius...),
		RollbackAvailable:    plan.RollbackAvailable,
		ResourceVersion:      plan.ResourceVersion,
		PolicyVersion:        plan.PolicyVersion,
	}
	return "sha256:" + hashJSON(payload, 32)
}

func predictedBlastRadius(resource unified.Resource) []string {
	seen := map[string]struct{}{}
	add := func(id string) {
		id = unified.CanonicalResourceID(id)
		if id == "" {
			return
		}
		seen[id] = struct{}{}
	}

	resourceID := unified.CanonicalResourceID(resource.ID)
	add(resourceID)
	for _, relationship := range unified.ResourceRelationshipsWithCanonicalParent(resource) {
		if !relationship.Active {
			continue
		}
		add(relationship.SourceID)
		add(relationship.TargetID)
	}

	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	if resourceID != "" {
		for i, id := range out {
			if id == resourceID {
				copy(out[1:i+1], out[0:i])
				out[0] = resourceID
				break
			}
		}
	}
	return out
}

func planMessage(resource unified.Resource, capability unified.ResourceCapability, policy unified.ActionApprovalLevel) string {
	target := displayResourceName(resource)
	action := strings.TrimSpace(capability.Name)
	if target == "" {
		target = unified.CanonicalResourceID(resource.ID)
	}
	switch policy {
	case unified.ApprovalDryRun:
		return fmt.Sprintf("Plan created for %s on %s. Policy allows planning/dry-run only; execution is not performed by this endpoint.", action, target)
	case unified.ApprovalNone:
		return fmt.Sprintf("Plan created for %s on %s. Execution is not performed by this endpoint.", action, target)
	default:
		return fmt.Sprintf("Plan created for %s on %s. Execution requires %s approval and is not performed by this endpoint.", action, target, policy)
	}
}

func buildPreflight(resource unified.Resource, capability unified.ResourceCapability, req unified.ActionRequest, actionID string, policy unified.ActionApprovalLevel, plannedAt time.Time) *unified.ActionPreflight {
	resourceID := unified.CanonicalResourceID(resource.ID)
	action := strings.TrimSpace(capability.Name)
	description := strings.TrimSpace(capability.Description)
	intendedChange := description
	if intendedChange == "" {
		intendedChange = fmt.Sprintf("Run %s on %s", action, resourceID)
	}

	safetyChecks := []string{
		"Resource was resolved from the unified resource registry.",
		"Capability is advertised by the resource contract.",
		"This endpoint plans only; it does not approve or execute the action.",
	}
	switch policy {
	case unified.ApprovalDryRun:
		safetyChecks = append(safetyChecks, "Capability policy is dry-run-only.")
	case unified.ApprovalNone:
		safetyChecks = append(safetyChecks, "Capability policy allows execution without additional approval.")
	default:
		safetyChecks = append(safetyChecks, fmt.Sprintf("Execution requires %s approval.", policy))
	}

	return &unified.ActionPreflight{
		Target:          resourceID,
		CurrentState:    currentState(resource),
		IntendedChange:  intendedChange,
		DryRunAvailable: false,
		DryRunSummary:   "No provider-supported dry run is advertised for this capability.",
		SafetyChecks:    safetyChecks,
		VerificationSteps: []string{
			"Refresh the resource and confirm the expected state after execution.",
			"Review /api/audit/actions/" + actionID + "/events for lifecycle evidence.",
		},
		GeneratedAt: plannedAt,
	}
}

func currentState(resource unified.Resource) string {
	name := displayResourceName(resource)
	status := strings.TrimSpace(string(resource.Status))
	if name == "" {
		name = unified.CanonicalResourceID(resource.ID)
	}
	if status == "" {
		return name
	}
	return fmt.Sprintf("%s is %s", name, status)
}

func displayResourceName(resource unified.Resource) string {
	if resource.Canonical != nil && strings.TrimSpace(resource.Canonical.DisplayName) != "" {
		return strings.TrimSpace(resource.Canonical.DisplayName)
	}
	if strings.TrimSpace(resource.Name) != "" {
		return strings.TrimSpace(resource.Name)
	}
	return unified.CanonicalResourceID(resource.ID)
}

type normalizedIdentity struct {
	MachineID    string   `json:"machineId,omitempty"`
	DMIUUID      string   `json:"dmiUuid,omitempty"`
	Hostnames    []string `json:"hostnames,omitempty"`
	IPAddresses  []string `json:"ipAddresses,omitempty"`
	MACAddresses []string `json:"macAddresses,omitempty"`
	ClusterName  string   `json:"clusterName,omitempty"`
}

type normalizedCapabilityForResourceHash struct {
	Name                 string                      `json:"name"`
	Type                 unified.CapabilityType      `json:"type"`
	Description          string                      `json:"description"`
	MinimumApprovalLevel unified.ActionApprovalLevel `json:"minimumApprovalLevel"`
	Platform             string                      `json:"platform,omitempty"`
	Params               []unified.CapabilityParam   `json:"params,omitempty"`
}

type normalizedRelationship struct {
	SourceID   string                   `json:"sourceId"`
	TargetID   string                   `json:"targetId"`
	Type       unified.RelationshipType `json:"type"`
	Confidence float64                  `json:"confidence"`
	Active     bool                     `json:"active"`
	Discoverer string                   `json:"discoverer"`
	ObservedAt time.Time                `json:"observedAt,omitempty"`
	LastSeenAt time.Time                `json:"lastSeenAt,omitempty"`
	Metadata   map[string]any           `json:"metadata,omitempty"`
}

type normalizedChange struct {
	ID               string                      `json:"id"`
	ResourceID       string                      `json:"resourceId"`
	Kind             unified.ChangeKind          `json:"kind"`
	From             string                      `json:"from,omitempty"`
	To               string                      `json:"to,omitempty"`
	SourceType       unified.ChangeSourceType    `json:"sourceType"`
	SourceAdapter    unified.ChangeSourceAdapter `json:"sourceAdapter"`
	Confidence       unified.ChangeConfidence    `json:"confidence"`
	RelatedResources []string                    `json:"relatedResources,omitempty"`
}

type storageIncidentHashFields struct {
	Severity string `json:"severity,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Category string `json:"category,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

type sourceStatusHash struct {
	Status   string    `json:"status"`
	LastSeen time.Time `json:"lastSeen,omitempty"`
	Error    string    `json:"error,omitempty"`
}

func normalizeDataSources(sources []unified.DataSource) []unified.DataSource {
	out := make([]unified.DataSource, 0, len(sources))
	seen := map[unified.DataSource]struct{}{}
	for _, source := range sources {
		normalized := unified.DataSource(strings.ToLower(strings.TrimSpace(string(source))))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func normalizeIdentity(identity unified.ResourceIdentity) normalizedIdentity {
	return normalizedIdentity{
		MachineID:    strings.TrimSpace(identity.MachineID),
		DMIUUID:      strings.TrimSpace(identity.DMIUUID),
		Hostnames:    sortedTrimmedStrings(identity.Hostnames),
		IPAddresses:  sortedTrimmedStrings(identity.IPAddresses),
		MACAddresses: sortedTrimmedStrings(identity.MACAddresses),
		ClusterName:  strings.TrimSpace(identity.ClusterName),
	}
}

func normalizeCapabilitiesForResourceHash(capabilities []unified.ResourceCapability) []normalizedCapabilityForResourceHash {
	out := make([]normalizedCapabilityForResourceHash, 0, len(capabilities))
	for _, capability := range capabilities {
		out = append(out, normalizedCapabilityForResourceHash{
			Name:                 strings.TrimSpace(capability.Name),
			Type:                 capability.Type,
			Description:          strings.TrimSpace(capability.Description),
			MinimumApprovalLevel: capability.MinimumApprovalLevel,
			Platform:             strings.TrimSpace(capability.Platform),
			Params:               normalizeCapabilityParams(capability.Params),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Platform < out[j].Platform
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func normalizeCapabilityParams(params []unified.CapabilityParam) []unified.CapabilityParam {
	out := make([]unified.CapabilityParam, 0, len(params))
	for _, param := range params {
		param.Name = strings.TrimSpace(param.Name)
		param.Type = strings.ToLower(strings.TrimSpace(param.Type))
		param.Pattern = strings.TrimSpace(param.Pattern)
		param.Description = strings.TrimSpace(param.Description)
		param.Enum = sortedTrimmedStrings(param.Enum)
		out = append(out, param)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedCapabilityParamNames(params []unified.CapabilityParam) []string {
	names := make([]string, 0, len(params))
	for _, param := range params {
		if name := strings.TrimSpace(param.Name); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func normalizeRelationships(relationships []unified.ResourceRelationship) []normalizedRelationship {
	out := make([]normalizedRelationship, 0, len(relationships))
	for _, relationship := range relationships {
		out = append(out, normalizedRelationship{
			SourceID:   unified.CanonicalResourceID(relationship.SourceID),
			TargetID:   unified.CanonicalResourceID(relationship.TargetID),
			Type:       relationship.Type,
			Confidence: relationship.Confidence,
			Active:     relationship.Active,
			Discoverer: strings.TrimSpace(relationship.Discoverer),
			ObservedAt: relationship.ObservedAt.UTC(),
			LastSeenAt: relationship.LastSeenAt.UTC(),
			Metadata:   relationship.Metadata,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceID != out[j].SourceID {
			return out[i].SourceID < out[j].SourceID
		}
		if out[i].TargetID != out[j].TargetID {
			return out[i].TargetID < out[j].TargetID
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func normalizeChanges(changes []unified.ResourceChange) []normalizedChange {
	out := make([]normalizedChange, 0, len(changes))
	for _, change := range changes {
		out = append(out, normalizedChange{
			ID:               strings.TrimSpace(change.ID),
			ResourceID:       unified.CanonicalResourceID(change.ResourceID),
			Kind:             change.Kind,
			From:             strings.TrimSpace(change.From),
			To:               strings.TrimSpace(change.To),
			SourceType:       change.SourceType,
			SourceAdapter:    change.SourceAdapter,
			Confidence:       change.Confidence,
			RelatedResources: sortedCanonicalResourceIDs(change.RelatedResources),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ResourceID != out[j].ResourceID {
			return out[i].ResourceID < out[j].ResourceID
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func normalizeSourceStatus(status map[unified.DataSource]unified.SourceStatus) map[unified.DataSource]sourceStatusHash {
	if len(status) == 0 {
		return nil
	}
	out := make(map[unified.DataSource]sourceStatusHash, len(status))
	for source, sourceStatus := range status {
		normalizedSource := unified.DataSource(strings.ToLower(strings.TrimSpace(string(source))))
		if normalizedSource == "" {
			continue
		}
		out[normalizedSource] = sourceStatusHash{
			Status:   strings.TrimSpace(sourceStatus.Status),
			LastSeen: sourceStatus.LastSeen.UTC(),
			Error:    strings.TrimSpace(sourceStatus.Error),
		}
	}
	return out
}

func sortedCanonicalResourceIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = unified.CanonicalResourceID(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func sortedTrimmedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hashJSON(value any, bytes int) string {
	payload, err := json.Marshal(value)
	if err != nil {
		payload = []byte(fmt.Sprintf("%#v", value))
	}
	sum := sha256.Sum256(payload)
	if bytes <= 0 || bytes > len(sum) {
		bytes = len(sum)
	}
	return hex.EncodeToString(sum[:bytes])
}
