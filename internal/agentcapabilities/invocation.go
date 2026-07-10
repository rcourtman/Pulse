package agentcapabilities

import (
	"fmt"
	"sort"
	"strings"
)

// MutationTarget names what an individual tool invocation can change.
// Workflow kind (read/write/resolve) drives FSM transitions; mutation
// target drives safety policy: control-level gating and request-scoped
// mutation-deny policies key on it, never on workflow kind alone.
type MutationTarget string

const (
	// MutationNone: the invocation changes nothing durable.
	MutationNone MutationTarget = "none"
	// MutationPulseState: the invocation changes Pulse's own records
	// (findings, alerts, knowledge) but no customer infrastructure.
	MutationPulseState MutationTarget = "pulse_state"
	// MutationInfrastructure: the invocation can change customer
	// infrastructure. Blocked at read-only control level and under
	// deny-infrastructure-mutations request policy, before any handler.
	MutationInfrastructure MutationTarget = "infrastructure"
)

// InvocationClass is the classification of one concrete tool invocation.
type InvocationClass struct {
	Kind     ToolCallKind
	Mutation MutationTarget
}

// FailClosedInvocationClass is what missing, malformed, or unknown
// invocations classify as: a newly introduced or fabricated subaction can
// never bypass governed-mutation checks by being unclassified.
func FailClosedInvocationClass() InvocationClass {
	return InvocationClass{Kind: ToolCallKindWrite, Mutation: MutationInfrastructure}
}

// InvocationDescriptor is the registry-owned classification contract for
// one tool. A tool is either static (every invocation has one class) or
// discriminator-based (the named argument selects the subaction, and Cases
// must exactly cover the schema enum for that argument; registration
// asserts the coverage).
type InvocationDescriptor struct {
	// Discriminator is the argument key whose value selects the
	// subaction. Empty for static tools.
	Discriminator string
	// Static is the classification for every invocation of a static tool.
	Static *InvocationClass
	// Cases maps each declared discriminator enum value to its class.
	Cases map[string]InvocationClass
}

// Classify resolves the invocation class for a concrete argument map.
// Missing, malformed, or unknown discriminator values fail closed.
func (d InvocationDescriptor) Classify(args map[string]interface{}) InvocationClass {
	if d.Static != nil {
		return *d.Static
	}
	if d.Discriminator == "" || len(d.Cases) == 0 {
		return FailClosedInvocationClass()
	}
	raw, ok := args[d.Discriminator]
	if !ok {
		return FailClosedInvocationClass()
	}
	value, ok := raw.(string)
	if !ok {
		return FailClosedInvocationClass()
	}
	class, ok := d.Cases[strings.ToLower(strings.TrimSpace(value))]
	if !ok {
		return FailClosedInvocationClass()
	}
	return class
}

// Validate checks the descriptor's own shape and, for discriminator-based
// descriptors, that its cases exactly cover the given schema enum values.
// Registration fails on missing or extra cases so the classification
// contract can never drift from the offered schema.
func (d InvocationDescriptor) Validate(toolName string, enumValues []string) error {
	if d.Static != nil {
		if d.Discriminator != "" || len(d.Cases) != 0 {
			return fmt.Errorf("tool %q invocation descriptor must be static or discriminator-based, not both", toolName)
		}
		return nil
	}
	if d.Discriminator == "" {
		return fmt.Errorf("tool %q invocation descriptor declares neither static class nor discriminator", toolName)
	}
	if len(enumValues) == 0 {
		return fmt.Errorf("tool %q discriminator %q has no schema enum to cover", toolName, d.Discriminator)
	}
	want := map[string]bool{}
	for _, v := range enumValues {
		want[strings.ToLower(strings.TrimSpace(v))] = true
	}
	var missing, extra []string
	for v := range want {
		if _, ok := d.Cases[v]; !ok {
			missing = append(missing, v)
		}
	}
	for v := range d.Cases {
		if !want[v] {
			extra = append(extra, v)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		return fmt.Errorf("tool %q invocation descriptor does not exactly cover schema enum for %q (missing=%v extra=%v)", toolName, d.Discriminator, missing, extra)
	}
	return nil
}

func staticClass(kind ToolCallKind, mutation MutationTarget) InvocationDescriptor {
	class := InvocationClass{Kind: kind, Mutation: mutation}
	return InvocationDescriptor{Static: &class}
}

// registryInvocationDescriptors is the canonical classification table for
// every Pulse registry tool. Workflow kinds intentionally match the
// historical shared classifier so FSM transitions do not change; mutation
// targets are the safety-policy layer on top.
//
// Kubernetes's discriminator is `type`, not `action` - the historical
// hard-coded classifier read `action` and therefore classified every
// Kubernetes invocation (including scale/restart/delete_pod/exec) as read.
var registryInvocationDescriptors = map[string]InvocationDescriptor{
	PulseQueryToolName:     staticClass(ToolCallKindResolve, MutationNone),
	PulseMetricsToolName:   staticClass(ToolCallKindRead, MutationNone),
	PulseStorageToolName:   staticClass(ToolCallKindRead, MutationNone),
	PulsePMGToolName:       staticClass(ToolCallKindRead, MutationNone),
	PulseSummarizeToolName: staticClass(ToolCallKindRead, MutationNone),
	// pulse_read's exec subaction dispatches only structurally read-only
	// commands: the handler's execution-intent classifier rejects
	// WriteOrUnknown commands before dispatch.
	PulseReadToolName:    staticClass(ToolCallKindRead, MutationNone),
	PulseControlToolName: staticClass(ToolCallKindWrite, MutationInfrastructure),
	// pulse_file_edit is write-only: file inspection routes through
	// pulse_read, so this tool never advertises a read subaction.
	PulseFileEditToolName: staticClass(ToolCallKindWrite, MutationInfrastructure),
	PulseDiscoveryToolName: {
		Discriminator: "action",
		Cases: map[string]InvocationClass{
			"get":  {Kind: ToolCallKindResolve, Mutation: MutationNone},
			"list": {Kind: ToolCallKindResolve, Mutation: MutationNone},
			// run collects evidence into the discovery cache only; it
			// does not mutate customer infrastructure.
			"run": {Kind: ToolCallKindResolve, Mutation: MutationNone},
		},
	},
	PulseAlertsToolName: {
		Discriminator: "action",
		Cases: map[string]InvocationClass{
			"list":     {Kind: ToolCallKindRead, Mutation: MutationNone},
			"findings": {Kind: ToolCallKindRead, Mutation: MutationNone},
			"resolved": {Kind: ToolCallKindRead, Mutation: MutationNone},
			"resolve":  {Kind: ToolCallKindWrite, Mutation: MutationPulseState},
			"dismiss":  {Kind: ToolCallKindWrite, Mutation: MutationPulseState},
		},
	},
	PulseKubernetesToolName: {
		Discriminator: "type",
		Cases: map[string]InvocationClass{
			"clusters":    {Kind: ToolCallKindRead, Mutation: MutationNone},
			"nodes":       {Kind: ToolCallKindRead, Mutation: MutationNone},
			"pods":        {Kind: ToolCallKindRead, Mutation: MutationNone},
			"deployments": {Kind: ToolCallKindRead, Mutation: MutationNone},
			"logs":        {Kind: ToolCallKindRead, Mutation: MutationNone},
			"scale":       {Kind: ToolCallKindWrite, Mutation: MutationInfrastructure},
			"restart":     {Kind: ToolCallKindWrite, Mutation: MutationInfrastructure},
			"delete_pod":  {Kind: ToolCallKindWrite, Mutation: MutationInfrastructure},
			"exec":        {Kind: ToolCallKindWrite, Mutation: MutationInfrastructure},
		},
	},
	PulseDockerToolName: {
		Discriminator: "action",
		Cases: map[string]InvocationClass{
			"updates":  {Kind: ToolCallKindRead, Mutation: MutationNone},
			"services": {Kind: ToolCallKindRead, Mutation: MutationNone},
			"tasks":    {Kind: ToolCallKindRead, Mutation: MutationNone},
			"swarm":    {Kind: ToolCallKindRead, Mutation: MutationNone},
			// check_updates queues a read-only scan command on the
			// agent; it changes nothing on the container estate.
			"check_updates": {Kind: ToolCallKindWrite, Mutation: MutationNone},
			"control":       {Kind: ToolCallKindWrite, Mutation: MutationInfrastructure},
			"update":        {Kind: ToolCallKindWrite, Mutation: MutationInfrastructure},
		},
	},
	PulseKnowledgeToolName: {
		Discriminator: "action",
		Cases: map[string]InvocationClass{
			"recall":    {Kind: ToolCallKindRead, Mutation: MutationNone},
			"incidents": {Kind: ToolCallKindRead, Mutation: MutationNone},
			"correlate": {Kind: ToolCallKindRead, Mutation: MutationNone},
			"remember":  {Kind: ToolCallKindWrite, Mutation: MutationPulseState},
		},
	},
	PatrolGetFindingsToolName:    staticClass(ToolCallKindRead, MutationNone),
	PatrolReportFindingToolName:  staticClass(ToolCallKindWrite, MutationPulseState),
	PatrolResolveFindingToolName: staticClass(ToolCallKindWrite, MutationPulseState),
}

// InvocationDescriptorFor returns the canonical invocation descriptor for
// a registry tool name.
func InvocationDescriptorFor(toolName string) (InvocationDescriptor, bool) {
	d, ok := registryInvocationDescriptors[strings.TrimSpace(toolName)]
	return d, ok
}

// ClassifyRegisteredInvocation classifies a concrete invocation of a
// registry tool. Unknown tool names fail closed: a tool without a
// descriptor cannot be assumed safe.
func ClassifyRegisteredInvocation(toolName string, args map[string]interface{}) InvocationClass {
	descriptor, ok := InvocationDescriptorFor(toolName)
	if !ok {
		return FailClosedInvocationClass()
	}
	return descriptor.Classify(args)
}
