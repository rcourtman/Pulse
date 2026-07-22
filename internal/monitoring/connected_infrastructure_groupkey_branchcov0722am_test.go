package monitoring

import (
	"testing"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0722"`) for connectedInfrastructureGroupKey in
// connected_infrastructure.go. The function had 0.0% coverage because the
// production callers go through resolver.GroupIDForResource first and only
// reach this fallback key builder for a minority of resources.
//
// Every arm of the function is exercised directly here:
//
//   - the "kubernetes" surfaceKind arm (ClusterID present, whitespace-only,
//     and a nil Kubernetes block -> resource-ID fallback)
//   - the machine-ID arm via connectedInfrastructureMachineID (Agent, Docker
//     and Identity precedence, whitespace trimming)
//   - the hostname arm via connectedInfrastructureHostname (lower-casing of
//     the returned key plus the agent -> docker -> pbs -> pmg -> truenas ->
//     canonical -> Name candidate order)
//   - the Canonical PrimaryID arm (including a whitespace-only PrimaryID
//     falling through)
//   - the final "resource:" fallback when nothing else matches
//
// Conventions match sibling in-package tests in this directory: stdlib
// `testing` only, table-driven subtests, no testify. Each case is value-in ->
// value-out and builds a unifiedresources.Resource fixture inline, exactly
// like connected_infrastructure_test.go.
func TestBranchcov0722GroupKeyKubernetesSurface(t *testing.T) {
	cases := []struct {
		name     string
		resource unifiedresources.Resource
		want     string
	}{
		// Arm: surfaceKind == "kubernetes" AND Kubernetes != nil AND the
		// trimmed ClusterID is non-empty -> "kubernetes:" + trimmed ClusterID.
		{"non-empty cluster id is trimmed",
			unifiedresources.Resource{
				ID:         "k8s-resource",
				Kubernetes: &unifiedresources.K8sData{ClusterID: "  cluster-1  "},
			}, "kubernetes:cluster-1"},

		// Arm: surfaceKind == "kubernetes" but ClusterID is whitespace-only
		// (trimmed value empty) -> falls back to the resource-ID form.
		{"whitespace-only cluster id falls back to resource id",
			unifiedresources.Resource{
				ID:         "k8s-resource",
				Kubernetes: &unifiedresources.K8sData{ClusterID: "   "},
			}, "kubernetes:k8s-resource"},

		// Arm: surfaceKind == "kubernetes" but the Kubernetes block is nil
		// -> falls back to the resource-ID form.
		{"nil kubernetes block falls back to resource id",
			unifiedresources.Resource{
				ID: "k8s-resource",
			}, "kubernetes:k8s-resource"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectedInfrastructureGroupKey(tc.resource, "kubernetes")
			if got != tc.want {
				t.Fatalf("connectedInfrastructureGroupKey(_, \"kubernetes\") = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBranchcov0722GroupKeyMachineIDArm(t *testing.T) {
	cases := []struct {
		name     string
		resource unifiedresources.Resource
		want     string
	}{
		// Arm: connectedInfrastructureMachineID returns a non-empty value
		// -> "machine:" + machineID. Agent.MachineID wins and is trimmed.
		{"agent machine id wins and is trimmed",
			unifiedresources.Resource{
				ID:    "host-resource",
				Agent: &unifiedresources.AgentData{MachineID: "  agent-machine  "},
			}, "machine:agent-machine"},

		// Precedence: Docker.MachineID when Agent is nil.
		{"docker machine id when agent absent",
			unifiedresources.Resource{
				ID:     "host-resource",
				Docker: &unifiedresources.DockerData{MachineID: "docker-machine"},
			}, "machine:docker-machine"},

		// Precedence: Identity.MachineID when Agent and Docker are both nil.
		{"identity machine id when agent and docker absent",
			unifiedresources.Resource{
				ID:       "host-resource",
				Identity: unifiedresources.ResourceIdentity{MachineID: "identity-machine"},
			}, "machine:identity-machine"},

		// Precedence: Agent wins over Docker AND Identity when all three set.
		{"agent wins over docker and identity",
			unifiedresources.Resource{
				ID:       "host-resource",
				Agent:    &unifiedresources.AgentData{MachineID: "agent-machine"},
				Docker:   &unifiedresources.DockerData{MachineID: "docker-machine"},
				Identity: unifiedresources.ResourceIdentity{MachineID: "identity-machine"},
			}, "machine:agent-machine"},

		// Precedence: Docker wins over Identity when Agent is nil.
		{"docker wins over identity",
			unifiedresources.Resource{
				ID:       "host-resource",
				Docker:   &unifiedresources.DockerData{MachineID: "docker-machine"},
				Identity: unifiedresources.ResourceIdentity{MachineID: "identity-machine"},
			}, "machine:docker-machine"},

		// Branch: Agent present but MachineID whitespace-only -> falls
		// through to Docker (whitespace is treated as empty by the trim
		// check) and then to Identity.
		{"agent with whitespace-only machine id falls through to identity",
			unifiedresources.Resource{
				ID:       "host-resource",
				Agent:    &unifiedresources.AgentData{MachineID: "   "},
				Identity: unifiedresources.ResourceIdentity{MachineID: "identity-machine"},
			}, "machine:identity-machine"},

		// NOTE: the all-empty fallthrough (machineID returns "" so the
		// function moves to the hostname/primary/resource arms) is exercised
		// by the hostname and fallback tests below, whose fixtures deliberately
		// leave every MachineID unset.
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectedInfrastructureGroupKey(tc.resource, "agent")
			if got != tc.want {
				t.Fatalf("connectedInfrastructureGroupKey(_, \"agent\") = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBranchcov0722GroupKeyHostnameArm(t *testing.T) {
	cases := []struct {
		name     string
		resource unifiedresources.Resource
		want     string
	}{
		// Arm: connectedInfrastructureHostname returns a non-empty value
		// -> "host:" + lower-cased hostname. Agent.Hostname is the highest
		// precedence candidate and is lower-cased in the returned key.
		{"agent hostname lowercased",
			unifiedresources.Resource{
				ID:    "host-resource",
				Agent: &unifiedresources.AgentData{Hostname: "Agent.Local"},
			}, "host:agent.local"},

		// Precedence: Docker.Hostname when Agent is absent.
		{"docker hostname when agent absent",
			unifiedresources.Resource{
				ID:     "host-resource",
				Docker: &unifiedresources.DockerData{Hostname: "Docker.Local"},
			}, "host:docker.local"},

		// Precedence: Docker.DisplayName when Docker.Hostname is empty
		// (the docker candidate helper tries Hostname then DisplayName).
		{"docker display name when docker hostname empty",
			unifiedresources.Resource{
				ID:     "host-resource",
				Docker: &unifiedresources.DockerData{DisplayName: "Docker-Display"},
			}, "host:docker-display"},

		// Precedence: PBS.Hostname.
		{"pbs hostname",
			unifiedresources.Resource{
				ID:  "host-resource",
				PBS: &unifiedresources.PBSData{Hostname: "PBS.Local"},
			}, "host:pbs.local"},

		// Precedence: PMG.Hostname.
		{"pmg hostname",
			unifiedresources.Resource{
				ID:  "host-resource",
				PMG: &unifiedresources.PMGData{Hostname: "PMG.Local"},
			}, "host:pmg.local"},

		// Precedence: TrueNAS.Hostname.
		{"truenas hostname",
			unifiedresources.Resource{
				ID:      "host-resource",
				TrueNAS: &unifiedresources.TrueNASData{Hostname: "TrueNAS.Local"},
			}, "host:truenas.local"},

		// Precedence: Canonical.Hostname.
		{"canonical hostname",
			unifiedresources.Resource{
				ID:        "host-resource",
				Canonical: &unifiedresources.CanonicalIdentity{Hostname: "Canonical.Local"},
			}, "host:canonical.local"},

		// Final candidate: resource.Name, trimmed and lower-cased.
		{"resource name fallback trimmed and lowercased",
			unifiedresources.Resource{
				ID:   "host-resource",
				Name: "  Named-Host  ",
			}, "host:named-host"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectedInfrastructureGroupKey(tc.resource, "agent")
			if got != tc.want {
				t.Fatalf("connectedInfrastructureGroupKey(_, \"agent\") = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBranchcov0722GroupKeyPrimaryAndResourceFallback(t *testing.T) {
	cases := []struct {
		name     string
		resource unifiedresources.Resource
		want     string
	}{
		// Arm: Canonical != nil AND trimmed PrimaryID != "" ->
		// "primary:" + trimmed PrimaryID. The fixture leaves every
		// MachineID/hostname unset so the earlier arms are skipped.
		{"canonical primary id trimmed",
			unifiedresources.Resource{
				ID:        "host-resource",
				Canonical: &unifiedresources.CanonicalIdentity{PrimaryID: "  primary-1  "},
			}, "primary:primary-1"},

		// Arm: Canonical.PrimaryID is whitespace-only (trimmed empty) ->
		// falls through to the resource fallback.
		{"whitespace-only primary id falls through to resource",
			unifiedresources.Resource{
				ID:        "host-resource",
				Canonical: &unifiedresources.CanonicalIdentity{PrimaryID: "   "},
			}, "resource:host-resource"},

		// Arm: Canonical is nil -> falls through to the resource fallback.
		{"nil canonical falls through to resource",
			unifiedresources.Resource{
				ID: "host-resource",
			}, "resource:host-resource"},

		// Final fallback: nothing matches -> "resource:" + resource.ID.
		// A zero-value resource yields the bare "resource:" prefix.
		{"zero resource returns bare resource prefix",
			unifiedresources.Resource{}, "resource:"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectedInfrastructureGroupKey(tc.resource, "agent")
			if got != tc.want {
				t.Fatalf("connectedInfrastructureGroupKey(_, \"agent\") = %q, want %q", got, tc.want)
			}
		})
	}
}
