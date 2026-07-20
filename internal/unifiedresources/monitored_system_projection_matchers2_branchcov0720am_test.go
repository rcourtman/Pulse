package unifiedresources

import "testing"

// Branch-coverage tests (round 2) for the remaining per-source selector
// predicates in monitored_system_projection.go:
//
//   - monitoredSystemReplacementSelectorMatchesDocker
//   - monitoredSystemReplacementSelectorMatchesTrueNAS
//   - monitoredSystemReplacementSelectorMatchesPBS
//   - monitoredSystemReplacementSelectorMatchesVMware
//
// and for the dispatcher that routes by DataSource:
//
//   - monitoredSystemReplacementSelectorMatches
//
// The sibling file monitored_system_projection_branchcov0720am_test.go
// already covers the Agent/Proxmox/PMG/K8s per-type matchers and defines the
// shared agentMatchResource / proxmoxMatchResource / pmgMatchResource /
// k8sMatchResource fixtures; this file reuses those where needed and defines
// DISTINCT fixture helpers (suffixed R2) for the four source types it owns.
//
// Each per-type predicate has (a) a nil-source-data early-return guard and
// (b) a short-circuit OR chain over several selector arms. To get full branch
// coverage of the OR chain we drive one sub-case per arm in which ONLY that
// arm matches (all earlier arms evaluate to false), plus a no-match case
// (every arm false), a whitespace-only case, and the nil-source-data guard.
//
// For the dispatcher we drive one sub-case per DataSource route (proving the
// route lands in the correct arm by giving each resource ONLY that source's
// data and a selector keyed on a field unique to that source's struct, so a
// misroute would hit the wrong matcher's nil-source guard) plus the
// unmatched/default arm.
//
// All assertions are behavioural: they assert the returned boolean for
// representative inputs that drive each branch. No change-detector
// string-pinning, no re-imported constants.

// ---------------------------------------------------------------------------
// shared fixtures (distinct from those in the sibling test file)
// ---------------------------------------------------------------------------

// dockerMatchResourceR2 is a Docker-backed Resource whose selector-distinct
// fields are all unique so each OR-chain arm can be driven in isolation.
func dockerMatchResourceR2() Resource {
	return Resource{
		ID:   "docker-res-r2",
		Name: "docker-row-name-r2",
		Docker: &DockerData{
			HostSourceID: "docker-hostsrc-r2",
			AgentID:      "docker-agent-r2",
			Hostname:     "docker.example.test",
			MachineID:    "docker-machine-r2",
		},
		Identity: ResourceIdentity{
			MachineID: "docker-identity-machine-r2",
		},
	}
}

func truenasMatchResourceR2() Resource {
	return Resource{
		ID:   "truenas-res-r2",
		Name: "truenas-row-name-r2",
		TrueNAS: &TrueNASData{
			Hostname: "truenas.example.test",
		},
		Identity: ResourceIdentity{
			MachineID: "truenas-machine-r2",
		},
	}
}

func pbsMatchResourceR2() Resource {
	return Resource{
		ID:   "pbs-res-r2",
		Name: "pbs-row-name-r2",
		PBS: &PBSData{
			InstanceID: "pbs-inst-r2",
			Hostname:   "pbs.example.test",
			HostURL:    "https://pbs.example.test:8007",
		},
	}
}

func vmwareMatchResourceR2() Resource {
	return Resource{
		ID:   "vmware-res-r2",
		Name: "vmware-row-name-r2",
		VMware: &VMwareData{
			ConnectionID:    "vmware-conn-r2",
			HostUUID:        "vmware-hostuuid-r2",
			RuntimeHostName: "vmware-runtime.example.test",
			VCenterHost:     "vcenter.example.test",
		},
		Identity: ResourceIdentity{
			DMIUUID: "vmware-dmiuuid-r2",
		},
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatchesDocker
// ---------------------------------------------------------------------------

func TestBranchcov0720amR2_DockerMatcher(t *testing.T) {
	cases := []struct {
		name     string
		selector MonitoredSystemReplacementSelector
		resource func() Resource
		want     bool
	}{
		// nil-source-data guard -> false.
		{
			name:     "nil_docker_returns_false",
			selector: MonitoredSystemReplacementSelector{ResourceID: "docker-hostsrc-r2"},
			resource: func() Resource {
				r := dockerMatchResourceR2()
				r.Docker = nil
				return r
			},
			want: false,
		},
		// Empty / zero selector -> every arm false -> false.
		{
			name:     "empty_selector_no_match",
			selector: MonitoredSystemReplacementSelector{},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     false,
		},
		// Canonical ResourceID arm only (selector.ResourceID == resource.ID).
		{
			name:     "canonical_resource_id_match",
			selector: MonitoredSystemReplacementSelector{ResourceID: "docker-res-r2"},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     true,
		},
		// Docker HostSourceID arm only (selector.ResourceID == Docker.HostSourceID).
		{
			name:     "resource_id_matches_docker_host_source_id",
			selector: MonitoredSystemReplacementSelector{ResourceID: "docker-hostsrc-r2"},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     true,
		},
		// Docker AgentID arm only.
		{
			name:     "agent_id_matches_docker_agent_id",
			selector: MonitoredSystemReplacementSelector{AgentID: "docker-agent-r2"},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     true,
		},
		// Docker MachineID arm only.
		{
			name:     "machine_id_matches_docker_machine_id",
			selector: MonitoredSystemReplacementSelector{MachineID: "docker-machine-r2"},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches Docker.Hostname candidate.
		{
			name:     "host_via_docker_hostname",
			selector: MonitoredSystemReplacementSelector{Hostname: "docker.example.test"},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches resource.Name candidate.
		{
			name:     "host_via_resource_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "docker-row-name-r2"},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     true,
		},
		// Host arm via selector.HostURL-derived hostname (extractHostname path).
		{
			name:     "host_via_selector_host_url_extract",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://docker.example.test:2376"},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     true,
		},
		// Non-matching populated selector -> false.
		{
			name: "populated_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "different-id",
				AgentID:    "different-agent",
				MachineID:  "different-machine",
				Hostname:   "different.example.test",
			},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     false,
		},
		// Whitespace-only selector fields -> false (trimmedEqualFold needs
		// non-empty on both sides after trimming; host map ends up empty).
		{
			name: "whitespace_only_selector_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "   ",
				AgentID:    "\t",
				MachineID:  " ",
				Hostname:   "  ",
				HostURL:    "\t",
			},
			resource: func() Resource { return dockerMatchResourceR2() },
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemReplacementSelectorMatchesDocker(tc.selector, tc.resource())
			if got != tc.want {
				t.Fatalf("monitoredSystemReplacementSelectorMatchesDocker(%+v) = %v, want %v",
					tc.selector, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatchesTrueNAS
// ---------------------------------------------------------------------------

func TestBranchcov0720amR2_TrueNASMatcher(t *testing.T) {
	cases := []struct {
		name     string
		selector MonitoredSystemReplacementSelector
		resource func() Resource
		want     bool
	}{
		// nil-source-data guard -> false.
		{
			name:     "nil_truenas_returns_false",
			selector: MonitoredSystemReplacementSelector{MachineID: "truenas-machine-r2"},
			resource: func() Resource {
				r := truenasMatchResourceR2()
				r.TrueNAS = nil
				return r
			},
			want: false,
		},
		// Empty selector -> no match.
		{
			name:     "empty_selector_no_match",
			selector: MonitoredSystemReplacementSelector{},
			resource: func() Resource { return truenasMatchResourceR2() },
			want:     false,
		},
		// Canonical ResourceID arm only.
		{
			name:     "canonical_resource_id_match",
			selector: MonitoredSystemReplacementSelector{ResourceID: "truenas-res-r2"},
			resource: func() Resource { return truenasMatchResourceR2() },
			want:     true,
		},
		// Identity.MachineID arm only.
		{
			name:     "machine_id_matches_identity_machine_id",
			selector: MonitoredSystemReplacementSelector{MachineID: "truenas-machine-r2"},
			resource: func() Resource { return truenasMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches TrueNAS.Hostname candidate.
		{
			name:     "host_via_truenas_hostname",
			selector: MonitoredSystemReplacementSelector{Hostname: "truenas.example.test"},
			resource: func() Resource { return truenasMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches resource.Name candidate.
		{
			name:     "host_via_resource_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "truenas-row-name-r2"},
			resource: func() Resource { return truenasMatchResourceR2() },
			want:     true,
		},
		// Host arm via selector.HostURL-derived hostname (extractHostname path).
		{
			name:     "host_via_selector_host_url_extract",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://truenas.example.test:443"},
			resource: func() Resource { return truenasMatchResourceR2() },
			want:     true,
		},
		// Non-matching populated selector -> false.
		{
			name: "populated_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "different-id",
				MachineID:  "different-machine",
				Hostname:   "different.example.test",
			},
			resource: func() Resource { return truenasMatchResourceR2() },
			want:     false,
		},
		// Whitespace-only selector fields -> false.
		{
			name: "whitespace_only_selector_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "   ",
				MachineID:  "\t",
				Hostname:   "  ",
				HostURL:    " ",
			},
			resource: func() Resource { return truenasMatchResourceR2() },
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemReplacementSelectorMatchesTrueNAS(tc.selector, tc.resource())
			if got != tc.want {
				t.Fatalf("monitoredSystemReplacementSelectorMatchesTrueNAS(%+v) = %v, want %v",
					tc.selector, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatchesPBS
// ---------------------------------------------------------------------------

func TestBranchcov0720amR2_PBSMatcher(t *testing.T) {
	cases := []struct {
		name     string
		selector MonitoredSystemReplacementSelector
		resource func() Resource
		want     bool
	}{
		// nil-source-data guard -> false.
		{
			name:     "nil_pbs_returns_false",
			selector: MonitoredSystemReplacementSelector{ResourceID: "pbs-inst-r2"},
			resource: func() Resource {
				r := pbsMatchResourceR2()
				r.PBS = nil
				return r
			},
			want: false,
		},
		// Empty selector -> no match.
		{
			name:     "empty_selector_no_match",
			selector: MonitoredSystemReplacementSelector{},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     false,
		},
		// Canonical ResourceID arm only.
		{
			name:     "canonical_resource_id_match",
			selector: MonitoredSystemReplacementSelector{ResourceID: "pbs-res-r2"},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     true,
		},
		// PBS InstanceID arm only (selector.ResourceID == PBS.InstanceID).
		{
			name:     "resource_id_matches_pbs_instance_id",
			selector: MonitoredSystemReplacementSelector{ResourceID: "pbs-inst-r2"},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     true,
		},
		// Direct HostURL arm only (trimmedEqualFold(selector.HostURL, PBS.HostURL)).
		// selector.HostURL equals PBS.HostURL exactly so this arm fires; host
		// arm not reached because trimmedEqualFold short-circuits first.
		{
			name:     "host_url_matches_pbs_host_url",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://pbs.example.test:8007"},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches PBS.Hostname candidate.
		// (selector.HostURL left empty so the direct HostURL arm stays false.)
		{
			name:     "host_via_pbs_hostname",
			selector: MonitoredSystemReplacementSelector{Hostname: "pbs.example.test"},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches resource.Name candidate.
		{
			name:     "host_via_resource_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "pbs-row-name-r2"},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     true,
		},
		// Host arm via selector.HostURL-derived hostname matching the PBS.HostURL
		// candidate (extractHostname path). The selector.HostURL differs from
		// PBS.HostURL in scheme/port so the direct trimmedEqualFold arm is
		// false, but both extract to the same hostname.
		{
			name:     "host_via_selector_host_url_extract",
			selector: MonitoredSystemReplacementSelector{HostURL: "http://pbs.example.test:80"},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     true,
		},
		// Non-matching populated selector -> false.
		{
			name: "populated_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "different",
				HostURL:    "https://different.example.test:8007",
				Hostname:   "different.example.test",
			},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     false,
		},
		// Whitespace-only selector fields -> false.
		{
			name: "whitespace_only_selector_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "   ",
				HostURL:    "\t",
				Hostname:   "  ",
			},
			resource: func() Resource { return pbsMatchResourceR2() },
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemReplacementSelectorMatchesPBS(tc.selector, tc.resource())
			if got != tc.want {
				t.Fatalf("monitoredSystemReplacementSelectorMatchesPBS(%+v) = %v, want %v",
					tc.selector, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatchesVMware
// ---------------------------------------------------------------------------

func TestBranchcov0720amR2_VMwareMatcher(t *testing.T) {
	cases := []struct {
		name     string
		selector MonitoredSystemReplacementSelector
		resource func() Resource
		want     bool
	}{
		// nil-source-data guard -> false.
		{
			name:     "nil_vmware_returns_false",
			selector: MonitoredSystemReplacementSelector{ResourceID: "vmware-conn-r2"},
			resource: func() Resource {
				r := vmwareMatchResourceR2()
				r.VMware = nil
				return r
			},
			want: false,
		},
		// Empty selector -> no match.
		{
			name:     "empty_selector_no_match",
			selector: MonitoredSystemReplacementSelector{},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     false,
		},
		// Canonical ResourceID arm only.
		{
			name:     "canonical_resource_id_match",
			selector: MonitoredSystemReplacementSelector{ResourceID: "vmware-res-r2"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// VMware ConnectionID arm only (selector.ResourceID == VMware.ConnectionID).
		{
			name:     "resource_id_matches_vmware_connection_id",
			selector: MonitoredSystemReplacementSelector{ResourceID: "vmware-conn-r2"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// VMware HostUUID arm only (selector.MachineID == VMware.HostUUID).
		{
			name:     "machine_id_matches_vmware_host_uuid",
			selector: MonitoredSystemReplacementSelector{MachineID: "vmware-hostuuid-r2"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// Identity.DMIUUID arm only (selector.MachineID == Identity.DMIUUID).
		{
			name:     "machine_id_matches_identity_dmi_uuid",
			selector: MonitoredSystemReplacementSelector{MachineID: "vmware-dmiuuid-r2"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// Name arm only (selector.Name == resource.Name).
		{
			name:     "name_matches_resource_name",
			selector: MonitoredSystemReplacementSelector{Name: "vmware-row-name-r2"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches VMware.RuntimeHostName.
		{
			name:     "host_via_vmware_runtime_host_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "vmware-runtime.example.test"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches VMware.VCenterHost candidate.
		{
			name:     "host_via_vmware_vcenter_host",
			selector: MonitoredSystemReplacementSelector{Hostname: "vcenter.example.test"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// Host arm only: selector.Hostname matches resource.Name candidate.
		{
			name:     "host_via_resource_name",
			selector: MonitoredSystemReplacementSelector{Hostname: "vmware-row-name-r2"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// Host arm via selector.HostURL-derived hostname (extractHostname path).
		{
			name:     "host_via_selector_host_url_extract",
			selector: MonitoredSystemReplacementSelector{HostURL: "https://vmware-runtime.example.test:443"},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     true,
		},
		// Non-matching populated selector -> false.
		{
			name: "populated_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "different",
				MachineID:  "different-uuid",
				Name:       "different-name",
				Hostname:   "different.example.test",
			},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     false,
		},
		// Whitespace-only selector fields -> false.
		{
			name: "whitespace_only_selector_no_match",
			selector: MonitoredSystemReplacementSelector{
				ResourceID: "   ",
				MachineID:  "\t",
				Name:       " ",
				Hostname:   "  ",
				HostURL:    "\t",
			},
			resource: func() Resource { return vmwareMatchResourceR2() },
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := monitoredSystemReplacementSelectorMatchesVMware(tc.selector, tc.resource())
			if got != tc.want {
				t.Fatalf("monitoredSystemReplacementSelectorMatchesVMware(%+v) = %v, want %v",
					tc.selector, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// monitoredSystemReplacementSelectorMatches (dispatcher)
// ---------------------------------------------------------------------------

// dispatcherRouteResource builds a resource that carries ONLY the data for the
// given source's matcher, so a correct dispatch reaches a matcher that sees
// non-nil source data while any misroute hits the wrong matcher's nil-source
// guard. The returned selector targets a field unique to that source's struct,
// which makes a correct dispatch observable as `true` and any misroute `false`.
func dispatcherRouteResource(source DataSource) (Resource, MonitoredSystemReplacementSelector) {
	switch source {
	case SourceAgent:
		r := agentMatchResource() // shared fixture from sibling test file
		return r, MonitoredSystemReplacementSelector{AgentID: r.Agent.AgentID}
	case SourceDocker:
		r := dockerMatchResourceR2()
		return r, MonitoredSystemReplacementSelector{ResourceID: r.Docker.HostSourceID}
	case SourceProxmox:
		r := proxmoxMatchResource() // shared fixture from sibling test file
		return r, MonitoredSystemReplacementSelector{ResourceID: r.Proxmox.SourceID}
	case SourceTrueNAS:
		r := truenasMatchResourceR2()
		return r, MonitoredSystemReplacementSelector{MachineID: r.Identity.MachineID}
	case SourcePBS:
		r := pbsMatchResourceR2()
		return r, MonitoredSystemReplacementSelector{ResourceID: r.PBS.InstanceID}
	case SourcePMG:
		r := pmgMatchResource() // shared fixture from sibling test file
		return r, MonitoredSystemReplacementSelector{ResourceID: r.PMG.InstanceID}
	case SourceK8s:
		r := k8sMatchResource() // shared fixture from sibling test file
		return r, MonitoredSystemReplacementSelector{ResourceID: r.Kubernetes.ClusterID}
	case SourceVMware:
		r := vmwareMatchResourceR2()
		return r, MonitoredSystemReplacementSelector{ResourceID: r.VMware.ConnectionID}
	default:
		return Resource{}, MonitoredSystemReplacementSelector{}
	}
}

func TestBranchcov0720amR2_Dispatcher(t *testing.T) {
	// One sub-case per DataSource route: a correct dispatch must route into the
	// matching per-source predicate, which (given a resource carrying only that
	// source's data and a selector keyed on a source-specific field) returns
	// true. A misroute would hit the wrong matcher's nil-source guard -> false.
	routedSources := []DataSource{
		SourceAgent,
		SourceDocker,
		SourceProxmox,
		SourceTrueNAS,
		SourcePBS,
		SourcePMG,
		SourceK8s,
		SourceVMware,
	}

	for _, source := range routedSources {
		source := source
		t.Run("route_"+string(source), func(t *testing.T) {
			resource, selector := dispatcherRouteResource(source)
			got := monitoredSystemReplacementSelectorMatches(source, selector, resource)
			if !got {
				t.Fatalf("monitoredSystemReplacementSelectorMatches(source=%q) = false, want true (route misdispatched?)",
					source)
			}
		})
	}

	// Default / unmatched arm: an unrecognized source returns false even when
	// the resource + selector would match under a real route.
	t.Run("default_unrecognized_source_returns_false", func(t *testing.T) {
		resource, selector := dispatcherRouteResource(SourceDocker)
		got := monitoredSystemReplacementSelectorMatches(SourceDocker+"_bogus", selector, resource)
		if got {
			t.Fatalf("monitoredSystemReplacementSelectorMatches(unrecognized source) = true, want false")
		}
	})

	// Empty source string also falls through to the default arm.
	t.Run("default_empty_source_returns_false", func(t *testing.T) {
		resource, selector := dispatcherRouteResource(SourceVMware)
		got := monitoredSystemReplacementSelectorMatches("", selector, resource)
		if got {
			t.Fatalf("monitoredSystemReplacementSelectorMatches(empty source) = true, want false")
		}
	})
}
