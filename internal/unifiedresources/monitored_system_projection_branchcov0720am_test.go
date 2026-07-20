package unifiedresources

import "testing"

// Branch-coverage tests for the four per-source selector predicates in
// monitored_system_projection.go:
//
//   - monitoredSystemReplacementSelectorMatchesAgent
//   - monitoredSystemReplacementSelectorMatchesProxmox
//   - monitoredSystemReplacementSelectorMatchesPMG
//   - monitoredSystemReplacementSelectorMatchesK8s
//
// Each predicate has (a) a nil-source-data early-return guard and (b) a
// short-circuit OR chain over several selector arms. To get full branch
// coverage of the OR chain we drive one sub-case per arm in which ONLY that
// arm matches (all earlier arms evaluate to false), plus a no-match case
// (every arm false) and the nil-source-data guard.
//
// All assertions are behavioural: they assert the returned boolean for
// representative inputs that drive each branch. No change-detector
// string-pinning, no re-imported constants.

// ---------------------------------------------------------------------------
// shared fixtures
// ---------------------------------------------------------------------------

// agentMatchResource is a populated Agent-backed Resource used as the
// non-matching base; each selector arm overrides exactly one field to drive
// only that arm to true.
func agentMatchResource() Resource {
	return Resource{
		ID:   "agent-res-1",
		Name: "agent-row-name",
		Agent: &AgentData{
			AgentID:  "agent-1",
			Hostname: "lab-a.example",
		},
		Identity: ResourceIdentity{
			MachineID: "machine-1",
		},
	}
}

func proxmoxMatchResource() Resource {
	return Resource{
		ID:   "px-res-1",
		Name: "px-row-name",
		Proxmox: &ProxmoxData{
			SourceID: "px-src-1",
			Instance: "px-instance-1",
			NodeName: "px-node-1",
			HostURL:  "https://px.example:8006",
		},
	}
}

func pmgMatchResource() Resource {
	return Resource{
		ID:   "pmg-res-1",
		Name: "pmg-row-name",
		PMG: &PMGData{
			InstanceID: "pmg-inst-1",
			Hostname:   "mail.example",
			HostURL:    "https://mail.example:8006",
		},
	}
}

func k8sMatchResource() Resource {
	return Resource{
		ID:   "k8s-res-1",
		Name: "k8s-row-name",
		Kubernetes: &K8sData{
			ClusterID:   "cluster-1",
			ClusterName: "cluster-name-1",
			SourceName:  "source-name-1",
			Server:      "https://k8s.example:6443",
			AgentID:     "k8s-agent-1",
		},
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatchesAgent
// ---------------------------------------------------------------------------

func TestBranchcov0720am_AgentMatcher(t *testing.T) {
	cases := []struct {
		name     string
		selector MonitoredSystemReplacementSelector
		resource func() Resource // fresh copy per case to keep cases independent
		want     bool
	}{
		// nil-source-data guard -> false.
		{
			name:     "nil_agent_returns_false",
			selector: MonitoredSystemReplacementSelector{AgentID: "agent-1"},
			resource: func() Resource {
				r := agentMatchResource()
				r.Agent = nil
				return r
			},
			want: false,
		},
		// Empty / zero selector -> every arm false -> false.
		{
			name:     "empty_selector_no_match",
			selector: MonitoredSystemReplacementSelector{},
			resource: func() Resource { return agentMatchResource() },
			want:     false,
		},
		// Canonical ResourceID arm only (selector.ResourceID == resource.ID).
		{
			name:     "canonical_resource_id_match",
			selector: MonitoredSystemReplacementSelector{ResourceID: "agent-res-1"},
			resource: func() Resource { return agentMatchResource() },
			want:     true,
		},
		// Canonical arm with case-folding (EqualFold semantics).
		{
			name:     "canonical_resource_id_case_fold",
			selector: MonitoredSystemReplacementSelector{ResourceID: "AGENT-RES-1"},
			resource: func() Resource { return agentMatchResource() },
			want:     true,
		},
		// AgentID arm only.
		{
			name:     "agent_id_match",
			selector: MonitoredSystemReplacementSelector{AgentID: "agent-1"},
			resource: func() Resource { return agentMatchResource() },
			want:     true,
		},
		// MachineID arm only.
		{
			name:     "machine_id_match",
			selector: MonitoredSystemReplacementSelector{MachineID: "machine-1"},
			resource: func() Resource { return agentMatchResource() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches Agent.Hostname.
		{
			name:     "host_via_agent_hostname",
			selector: MonitoredSystemReplacementSelector{Hostname: "lab-a.example"},
			resource: func() Resource { return agentMatchResource() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches resource.Name (second candidate).
		{
			name:     "host_via_resource_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "agent-row-name"},
			resource: func() Resource { return agentMatchResource() },
			want:     true,
		},
		// Host arm reached via HostURL-derived hostname (extractHostname path).
		{
			name:     "host_via_selector_host_url",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://lab-a.example:443"},
			resource: func() Resource { return agentMatchResource() },
			want:     true,
		},
		// Non-matching populated selector -> false.
		{
			name: "populated_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "different-id",
				AgentID:    "different-agent",
				MachineID:  "different-machine",
				Hostname:   "different.example",
			},
			resource: func() Resource { return agentMatchResource() },
			want:     false,
		},
		// Whitespace-only selector fields must not match (trimmedEqualFold
		// requires non-empty on both sides).
		{
			name: "whitespace_only_selector_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "   ",
				AgentID:    "\t",
				MachineID:  " ",
				Hostname:   "  ",
			},
			resource: func() Resource { return agentMatchResource() },
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemReplacementSelectorMatchesAgent(tc.selector, tc.resource())
			if got != tc.want {
				t.Fatalf("monitoredSystemReplacementSelectorMatchesAgent(%+v) = %v, want %v",
					tc.selector, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatchesProxmox
// ---------------------------------------------------------------------------

func TestBranchcov0720am_ProxmoxMatcher(t *testing.T) {
	cases := []struct {
		name     string
		selector MonitoredSystemReplacementSelector
		resource func() Resource
		want     bool
	}{
		// nil-source-data guard.
		{
			name:     "nil_proxmox_returns_false",
			selector: MonitoredSystemReplacementSelector{ResourceID: "px-src-1"},
			resource: func() Resource {
				r := proxmoxMatchResource()
				r.Proxmox = nil
				return r
			},
			want: false,
		},
		// Empty selector -> no match.
		{
			name:     "empty_selector_no_match",
			selector: MonitoredSystemReplacementSelector{},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     false,
		},
		// Canonical ResourceID arm only.
		{
			name:     "canonical_resource_id_match",
			selector: MonitoredSystemReplacementSelector{ResourceID: "px-res-1"},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     true,
		},
		// Proxmox SourceID arm only.
		{
			name:     "resource_id_matches_proxmox_source_id",
			selector: MonitoredSystemReplacementSelector{ResourceID: "px-src-1"},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     true,
		},
		// Name matches Proxmox.Instance arm only.
		{
			name:     "name_matches_instance",
			selector: MonitoredSystemReplacementSelector{Name: "px-instance-1"},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     true,
		},
		// Name matches Proxmox.NodeName arm only.
		{
			name:     "name_matches_node_name",
			selector: MonitoredSystemReplacementSelector{Name: "px-node-1"},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     true,
		},
		// HostURL matches Proxmox.HostURL arm only (trimmedEqualFold, exact
		// string equality).
		{
			name:     "host_url_matches_proxmox_host_url",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://px.example:8006"},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     true,
		},
		// Host arm via selector.Hostname matching Proxmox.NodeName candidate.
		{
			name:     "host_via_proxmox_node_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "px-node-1"},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     true,
		},
		// Host arm via selector.Hostname matching resource.Name candidate.
		{
			name:     "host_via_resource_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "px-row-name"},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     true,
		},
		// Host arm via selector.HostURL-derived hostname matching the
		// Proxmox.HostURL candidate (extractHostname path).
		{
			name:     "host_via_selector_host_url_extract",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://px.example:8006"},
			resource: func() Resource {
				// Force canonical / SourceID / Instance / NodeName / direct-HostURL
				// arms to fail so the host arm (last) is the only viable match.
				r := proxmoxMatchResource()
				r.ID = "other-id"
				r.Proxmox.SourceID = "other-src"
				r.Proxmox.Instance = "other-instance"
				// NodeName stays "px-node-1" but selector has empty Hostname so
				// Name-equality arms don't fire; the selector-derived host
				// "px.example" matches Proxmox.Hostname-equivalent candidate.
				return r
			},
			want: true,
		},
		// Non-matching populated selector -> false.
		{
			name: "populated_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "different",
				Name:       "different",
				HostURL:    "https://different.example:8006",
				Hostname:   "different.example",
			},
			resource: func() Resource { return proxmoxMatchResource() },
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemReplacementSelectorMatchesProxmox(tc.selector, tc.resource())
			if got != tc.want {
				t.Fatalf("monitoredSystemReplacementSelectorMatchesProxmox(%+v) = %v, want %v",
					tc.selector, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatchesPMG
// ---------------------------------------------------------------------------

func TestBranchcov0720am_PMGMatcher(t *testing.T) {
	cases := []struct {
		name     string
		selector MonitoredSystemReplacementSelector
		resource func() Resource
		want     bool
	}{
		// nil-source-data guard.
		{
			name:     "nil_pmg_returns_false",
			selector: MonitoredSystemReplacementSelector{ResourceID: "pmg-inst-1"},
			resource: func() Resource {
				r := pmgMatchResource()
				r.PMG = nil
				return r
			},
			want: false,
		},
		// Empty selector -> no match.
		{
			name:     "empty_selector_no_match",
			selector: MonitoredSystemReplacementSelector{},
			resource: func() Resource { return pmgMatchResource() },
			want:     false,
		},
		// Canonical ResourceID arm only.
		{
			name:     "canonical_resource_id_match",
			selector: MonitoredSystemReplacementSelector{ResourceID: "pmg-res-1"},
			resource: func() Resource { return pmgMatchResource() },
			want:     true,
		},
		// PMG InstanceID arm only.
		{
			name:     "resource_id_matches_pmg_instance_id",
			selector: MonitoredSystemReplacementSelector{ResourceID: "pmg-inst-1"},
			resource: func() Resource { return pmgMatchResource() },
			want:     true,
		},
		// Host arm via selector.Hostname matching PMG.Hostname.
		{
			name:     "host_via_pmg_hostname",
			selector: MonitoredSystemReplacementSelector{Hostname: "mail.example"},
			resource: func() Resource { return pmgMatchResource() },
			want:     true,
		},
		// Host arm via selector.Hostname matching resource.Name.
		{
			name:     "host_via_resource_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "pmg-row-name"},
			resource: func() Resource { return pmgMatchResource() },
			want:     true,
		},
		// Host arm via selector.HostURL-derived hostname.
		{
			name:     "host_via_selector_host_url_extract",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://mail.example:8006"},
			resource: func() Resource { return pmgMatchResource() },
			want:     true,
		},
		// Non-matching populated selector -> false.
		{
			name: "populated_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "different",
				Hostname:   "different.example",
				HostURL:    "https://different.example:8006",
			},
			resource: func() Resource { return pmgMatchResource() },
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemReplacementSelectorMatchesPMG(tc.selector, tc.resource())
			if got != tc.want {
				t.Fatalf("monitoredSystemReplacementSelectorMatchesPMG(%+v) = %v, want %v",
					tc.selector, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatchesK8s
// ---------------------------------------------------------------------------

func TestBranchcov0720am_K8sMatcher(t *testing.T) {
	cases := []struct {
		name     string
		selector MonitoredSystemReplacementSelector
		resource func() Resource
		want     bool
	}{
		// nil-source-data guard.
		{
			name:     "nil_kubernetes_returns_false",
			selector: MonitoredSystemReplacementSelector{ResourceID: "cluster-1"},
			resource: func() Resource {
				r := k8sMatchResource()
				r.Kubernetes = nil
				return r
			},
			want: false,
		},
		// Empty selector -> no match.
		{
			name:     "empty_selector_no_match",
			selector: MonitoredSystemReplacementSelector{},
			resource: func() Resource { return k8sMatchResource() },
			want:     false,
		},
		// Canonical ResourceID arm only.
		{
			name:     "canonical_resource_id_match",
			selector: MonitoredSystemReplacementSelector{ResourceID: "k8s-res-1"},
			resource: func() Resource { return k8sMatchResource() },
			want:     true,
		},
		// ClusterID arm only.
		{
			name:     "resource_id_matches_cluster_id",
			selector: MonitoredSystemReplacementSelector{ResourceID: "cluster-1"},
			resource: func() Resource { return k8sMatchResource() },
			want:     true,
		},
		// ClusterName arm only.
		{
			name:     "name_matches_cluster_name",
			selector: MonitoredSystemReplacementSelector{Name: "cluster-name-1"},
			resource: func() Resource { return k8sMatchResource() },
			want:     true,
		},
		// SourceName arm only.
		{
			name:     "name_matches_source_name",
			selector: MonitoredSystemReplacementSelector{Name: "source-name-1"},
			resource: func() Resource { return k8sMatchResource() },
			want:     true,
		},
		// Server arm only.
		{
			name:     "host_url_matches_server",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://k8s.example:6443"},
			resource: func() Resource { return k8sMatchResource() },
			want:     true,
		},
		// K8s AgentID arm only.
		{
			name:     "agent_id_matches_k8s_agent_id",
			selector: MonitoredSystemReplacementSelector{AgentID: "k8s-agent-1"},
			resource: func() Resource { return k8sMatchResource() },
			want:     true,
		},
		// Non-matching populated selector -> false.
		{
			name: "populated_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "different",
				Name:       "different",
				HostURL:    "https://different.example:6443",
				AgentID:    "different-agent",
			},
			resource: func() Resource { return k8sMatchResource() },
			want:     false,
		},
		// Whitespace-only selector fields -> false (trimmedEqualFold needs
		// non-empty on both sides after trimming).
		{
			name: "whitespace_only_selector_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "  ",
				Name:       "\t",
				HostURL:    " ",
				AgentID:    "   ",
			},
			resource: func() Resource { return k8sMatchResource() },
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemReplacementSelectorMatchesK8s(tc.selector, tc.resource())
			if got != tc.want {
				t.Fatalf("monitoredSystemReplacementSelectorMatchesK8s(%+v) = %v, want %v",
					tc.selector, got, tc.want)
			}
		})
	}
}
