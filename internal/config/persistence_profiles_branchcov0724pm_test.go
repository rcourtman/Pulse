package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests in this file use the TestBranchcov0724pm prefix so the scoped run
//
//	go test ./internal/config/ -run '^TestBranchcov0724pm' -count=1
//
// selects only them. They raise coverage for previously-uncovered (0.0%)
// persistence helpers in persistence.go and the PVEInstance.DeepCopy value
// helper in pve_instances.go:
//
//   - ConfigPersistence.GetConfigDir                (persistence.go:364)
//   - ConfigPersistence.SaveVMwareConfig            (persistence.go:690)
//   - ConfigPersistence.LoadVMwareConfig            (persistence.go:696)
//   - ConfigPersistence.SaveAgentProfiles           (persistence.go:3651)
//   - ConfigPersistence.SaveAgentProfileAssignments (persistence.go:3661)
//   - ConfigPersistence.SaveAgentProfileVersions    (persistence.go:3671)
//   - ConfigPersistence.SaveProfileDeploymentStatus (persistence.go:3681)
//   - ConfigPersistence.LoadProfileChangeLogs       (persistence.go:3686)
//   - ConfigPersistence.SaveProfileChangeLogs       (persistence.go:3691)
//   - ConfigPersistence.AppendProfileChangeLog      (persistence.go:3696)
//   - PVEInstance.DeepCopy                          (pve_instances.go:61)
//
// Every target is file-backed and exercised under t.TempDir (no network,
// daemon, SSH, or live database), so none is skipped on purity grounds.
// Each ConfigPersistence is constructed exactly the way the existing tests in
// this package do it: NewConfigPersistence(tempDir) + EnsureConfigDir, which
// also wires up encryption, so the encrypted save/load paths are exercised.

// newBranchcovCP builds a ConfigPersistence rooted at a fresh temp dir, the
// same way persistence_profiles_test.go does.
func newBranchcovCP(t *testing.T) *ConfigPersistence {
	t.Helper()
	cp := NewConfigPersistence(t.TempDir())
	require.NoError(t, cp.EnsureConfigDir())
	return cp
}

// ---------------------------------------------------------------------------
// GetConfigDir  (persistence.go:364)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmGetConfigDir(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	got := cp.GetConfigDir()
	require.NotEmpty(t, got)
	// For a clean, absolute t.TempDir path the resolution pipeline
	// (ResolveRuntimeDataDir -> NormalizeStorageDir/filepath.Clean) must return
	// the input unchanged. This is the observable contract of GetConfigDir, not
	// a tautology over the internal field.
	assert.Equal(t, tempDir, got,
		"GetConfigDir must return the configuration directory supplied at construction")

	// Tie the getter to real behaviour: persistence files must land under it.
	require.NoError(t, cp.SaveAgentProfiles(sampleAgentProfiles()))
	assert.FileExists(t, filepath.Join(got, "agent_profiles.json"),
		"persisted files must appear under the directory GetConfigDir reports")
}

// ---------------------------------------------------------------------------
// SaveVMwareConfig / LoadVMwareConfig  (persistence.go:690, :696)
// ---------------------------------------------------------------------------

func sampleVMwareInstances() []VMwareVCenterInstance {
	return []VMwareVCenterInstance{
		{
			ID: "vc-1", Name: "primary-vcenter", Host: "vcenter1.lan",
			Port: 443, Username: "administrator@vsphere.local", Password: "s3cret!",
			InsecureSkipVerify: true, Enabled: true,
			MonitorVMs: true, MonitorHosts: true, MonitorDatastores: false,
		},
		{
			ID: "vc-2", Name: "secondary", Host: "10.0.0.5",
			Port: 443, Username: "root", Password: "pw",
			InsecureSkipVerify: false, Enabled: false,
			MonitorVMs: false, MonitorHosts: true, MonitorDatastores: true,
		},
	}
}

func TestBranchcov0724pmVMwareConfigRoundTrip(t *testing.T) {
	want := sampleVMwareInstances()

	t.Run("populated slice survives an encrypted save/load round trip", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveVMwareConfig(want))

		got, err := cp.LoadVMwareConfig()
		require.NoError(t, err)
		require.Len(t, got, len(want))
		assert.Equal(t, want, got, "vmware instances must survive verbatim")
		assert.Equal(t, want[0].Password, got[0].Password, "sensitive fields must survive decryption")
	})

	t.Run("empty slice round trips as empty non-nil", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveVMwareConfig([]VMwareVCenterInstance{}))

		got, err := cp.LoadVMwareConfig()
		require.NoError(t, err)
		assert.NotNil(t, got, "saved empty slice must load as non-nil empty")
		assert.Empty(t, got)
	})

	t.Run("load before save returns empty slice and nil error", func(t *testing.T) {
		cp := newBranchcovCP(t)
		got, err := cp.LoadVMwareConfig()
		require.NoError(t, err, "missing file must be reported as empty slice, not an error")
		assert.NotNil(t, got, "missing file must yield a non-nil empty slice")
		assert.Empty(t, got)
	})
}

// ---------------------------------------------------------------------------
// SaveAgentProfiles / LoadAgentProfiles  (persistence.go:3651, :3646)
// ---------------------------------------------------------------------------

func sampleAgentProfiles() []models.AgentProfile {
	// Config map uses only JSON-stable scalar types (string/bool) so the
	// round trip can be asserted verbatim without interface{} type drift.
	return []models.AgentProfile{
		{
			ID: "profile-1", Name: "Production", Description: "prod agents",
			Config:    models.AgentConfigMap{"interval": "10s", "enabled": true},
			Version:   3,
			ParentID:  "profile-0",
			CreatedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt: time.Date(2025, 6, 7, 8, 9, 10, 0, time.UTC),
			CreatedBy: "admin",
			UpdatedBy: "ops",
		},
		{
			ID: "profile-2", Name: "Edge",
			Config:    models.AgentConfigMap{},
			Version:   1,
			CreatedAt: time.Date(2024, 12, 31, 23, 59, 0, 0, time.UTC),
			UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
}

func TestBranchcov0724pmAgentProfilesRoundTrip(t *testing.T) {
	want := sampleAgentProfiles()

	t.Run("populated slice survives an encrypted save/load round trip", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveAgentProfiles(want))

		got, err := cp.LoadAgentProfiles()
		require.NoError(t, err)
		require.Len(t, got, len(want))
		assert.Equal(t, want, got, "agent profiles must survive verbatim")
		assert.Equal(t, want[0].Config, got[0].Config, "config map must survive verbatim")
	})

	t.Run("empty slice round trips as empty non-nil", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveAgentProfiles([]models.AgentProfile{}))

		got, err := cp.LoadAgentProfiles()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("load before save returns empty slice and nil error", func(t *testing.T) {
		cp := newBranchcovCP(t)
		got, err := cp.LoadAgentProfiles()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

// ---------------------------------------------------------------------------
// SaveAgentProfileAssignments / LoadAgentProfileAssignments  (:3661, :3656)
// ---------------------------------------------------------------------------

func sampleAgentProfileAssignments() []models.AgentProfileAssignment {
	return []models.AgentProfileAssignment{
		{
			AgentID: "agent-1", ProfileID: "profile-1", ProfileVersion: 3,
			UpdatedAt:  time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC),
			AssignedBy: "admin",
		},
		{
			AgentID: "agent-2", ProfileID: "profile-2", ProfileVersion: 1,
			UpdatedAt: time.Date(2025, 3, 2, 12, 0, 0, 0, time.UTC),
		},
	}
}

func TestBranchcov0724pmAgentProfileAssignmentsRoundTrip(t *testing.T) {
	want := sampleAgentProfileAssignments()

	t.Run("populated slice survives an encrypted save/load round trip", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveAgentProfileAssignments(want))

		got, err := cp.LoadAgentProfileAssignments()
		require.NoError(t, err)
		require.Len(t, got, len(want))
		assert.Equal(t, want, got, "profile assignments must survive verbatim")
	})

	t.Run("empty slice round trips as empty non-nil", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveAgentProfileAssignments([]models.AgentProfileAssignment{}))

		got, err := cp.LoadAgentProfileAssignments()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("load before save returns empty slice and nil error", func(t *testing.T) {
		cp := newBranchcovCP(t)
		got, err := cp.LoadAgentProfileAssignments()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

// ---------------------------------------------------------------------------
// SaveAgentProfileVersions / LoadAgentProfileVersions  (:3671, :3666)
// ---------------------------------------------------------------------------

func sampleAgentProfileVersions() []models.AgentProfileVersion {
	return []models.AgentProfileVersion{
		{
			ProfileID: "profile-1", Version: 2, Name: "Production v2", Description: "tweaked",
			Config:     models.AgentConfigMap{"interval": "15s"},
			ParentID:   "profile-1",
			CreatedAt:  time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:  "admin",
			ChangeNote: "bumped interval",
		},
		{
			ProfileID: "profile-1", Version: 1, Name: "Production v1",
			Config:    models.AgentConfigMap{},
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
}

func TestBranchcov0724pmAgentProfileVersionsRoundTrip(t *testing.T) {
	want := sampleAgentProfileVersions()

	t.Run("populated slice survives an encrypted save/load round trip", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveAgentProfileVersions(want))

		got, err := cp.LoadAgentProfileVersions()
		require.NoError(t, err)
		require.Len(t, got, len(want))
		assert.Equal(t, want, got, "profile versions must survive verbatim")
	})

	t.Run("empty slice round trips as empty non-nil", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveAgentProfileVersions([]models.AgentProfileVersion{}))

		got, err := cp.LoadAgentProfileVersions()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("load before save returns empty slice and nil error", func(t *testing.T) {
		cp := newBranchcovCP(t)
		got, err := cp.LoadAgentProfileVersions()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

// ---------------------------------------------------------------------------
// SaveProfileDeploymentStatus / LoadProfileDeploymentStatus  (:3681, :3676)
// ---------------------------------------------------------------------------

func sampleProfileDeploymentStatus() []models.ProfileDeploymentStatus {
	return []models.ProfileDeploymentStatus{
		{
			AgentID: "agent-1", ProfileID: "profile-1",
			AssignedVersion: 3, DeployedVersion: 3,
			LastDeployedAt:   time.Date(2025, 3, 1, 12, 30, 0, 0, time.UTC),
			DeploymentStatus: "deployed",
		},
		{
			AgentID: "agent-2", ProfileID: "profile-1",
			AssignedVersion: 3, DeployedVersion: 2,
			LastDeployedAt:   time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC),
			DeploymentStatus: "pending",
			ErrorMessage:     "agent offline",
		},
	}
}

func TestBranchcov0724pmProfileDeploymentStatusRoundTrip(t *testing.T) {
	want := sampleProfileDeploymentStatus()

	t.Run("populated slice survives an encrypted save/load round trip", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveProfileDeploymentStatus(want))

		got, err := cp.LoadProfileDeploymentStatus()
		require.NoError(t, err)
		require.Len(t, got, len(want))
		assert.Equal(t, want, got, "deployment status must survive verbatim")
	})

	t.Run("empty slice round trips as empty non-nil", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveProfileDeploymentStatus([]models.ProfileDeploymentStatus{}))

		got, err := cp.LoadProfileDeploymentStatus()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("load before save returns empty slice and nil error", func(t *testing.T) {
		cp := newBranchcovCP(t)
		got, err := cp.LoadProfileDeploymentStatus()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

// ---------------------------------------------------------------------------
// SaveProfileChangeLogs / LoadProfileChangeLogs  (persistence.go:3691, :3686)
// ---------------------------------------------------------------------------

func sampleProfileChangeLogs() []models.ProfileChangeLog {
	return []models.ProfileChangeLog{
		{
			ID: "cl-1", ProfileID: "profile-1", ProfileName: "Production", Action: "create",
			NewVersion: 1, User: "admin",
			Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "cl-2", ProfileID: "profile-1", ProfileName: "Production", Action: "update",
			OldVersion: 1, NewVersion: 2, User: "ops",
			Timestamp: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			Details:   "interval change",
		},
	}
}

func TestBranchcov0724pmProfileChangeLogsRoundTrip(t *testing.T) {
	want := sampleProfileChangeLogs()

	t.Run("populated slice survives an encrypted save/load round trip", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveProfileChangeLogs(want))

		got, err := cp.LoadProfileChangeLogs()
		require.NoError(t, err)
		require.Len(t, got, len(want))
		assert.Equal(t, want, got, "change logs must survive verbatim")
	})

	t.Run("empty slice round trips as empty non-nil", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.SaveProfileChangeLogs([]models.ProfileChangeLog{}))

		got, err := cp.LoadProfileChangeLogs()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("load before save returns empty slice and nil error", func(t *testing.T) {
		cp := newBranchcovCP(t)
		got, err := cp.LoadProfileChangeLogs()
		require.NoError(t, err, "missing file must be reported as empty slice, not an error")
		assert.NotNil(t, got, "missing file must yield a non-nil empty slice")
		assert.Empty(t, got)
	})
}

// ---------------------------------------------------------------------------
// AppendProfileChangeLog  (persistence.go:3696)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmAppendProfileChangeLog(t *testing.T) {
	newEntry := models.ProfileChangeLog{
		ID: "cl-new", ProfileID: "profile-1", ProfileName: "Production", Action: "update",
		NewVersion: 3, User: "ops", Timestamp: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
	}

	t.Run("appends to empty log when no file exists yet", func(t *testing.T) {
		cp := newBranchcovCP(t)
		require.NoError(t, cp.AppendProfileChangeLog(newEntry))

		got, err := cp.LoadProfileChangeLogs()
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, newEntry, got[0])
	})

	t.Run("appends after existing logs rather than replacing them", func(t *testing.T) {
		cp := newBranchcovCP(t)
		existing := sampleProfileChangeLogs()
		require.NoError(t, cp.SaveProfileChangeLogs(existing))
		require.NoError(t, cp.AppendProfileChangeLog(newEntry))

		got, err := cp.LoadProfileChangeLogs()
		require.NoError(t, err)
		require.Len(t, got, len(existing)+1, "append must preserve existing entries and add one")
		// Original order and content preserved...
		for i := range existing {
			assert.Equal(t, existing[i], got[i], "existing entry %d must be preserved in place", i)
		}
		// ...and the new entry is last.
		assert.Equal(t, newEntry, got[len(existing)])
	})

	t.Run("trims to most recent 999 and keeps the new entry at capacity", func(t *testing.T) {
		cp := newBranchcovCP(t)
		base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		full := make([]models.ProfileChangeLog, 1000)
		for i := range full {
			full[i] = models.ProfileChangeLog{
				ID:        fmt.Sprintf("log-%04d", i),
				ProfileID: "profile-1",
				Action:    "update",
				Timestamp: base.Add(time.Duration(i) * time.Second),
			}
		}
		require.NoError(t, cp.SaveProfileChangeLogs(full))
		require.NoError(t, cp.AppendProfileChangeLog(newEntry))

		got, err := cp.LoadProfileChangeLogs()
		require.NoError(t, err)
		require.Len(t, got, 1000, "log must be capped at 1000 entries after append")

		// The oldest entry (full[0]) is dropped; the last 999 of the original
		// are retained in order, then the new entry is appended.
		assert.Equal(t, full[1].ID, got[0].ID, "oldest entry must be evicted")
		assert.Equal(t, full[999].ID, got[998].ID, "newest pre-existing entry must be retained")
		assert.Equal(t, newEntry, got[999], "appended entry must be last")
		assert.NotContains(t, idsOf(got), full[0].ID, "evicted entry must not appear anywhere")
	})

	t.Run("returns wrapped error when changelog file is corrupt", func(t *testing.T) {
		cp := newBranchcovCP(t)
		changeLogFile := filepath.Join(cp.GetConfigDir(), "profile-changelog.json")
		// Non-empty garbage: decryption fails -> plaintext fallback -> invalid
		// JSON -> loadSlice errors, which AppendProfileChangeLog must surface.
		require.NoError(t, os.WriteFile(changeLogFile, []byte("not-valid-json-at-all"), 0600))

		err := cp.AppendProfileChangeLog(newEntry)
		require.Error(t, err, "corrupt changelog must produce an error from AppendProfileChangeLog")
	})
}

// idsOf returns the IDs of each change-log entry, for set-membership checks.
func idsOf(logs []models.ProfileChangeLog) []string {
	out := make([]string, len(logs))
	for i := range logs {
		out[i] = logs[i].ID
	}
	return out
}

// ---------------------------------------------------------------------------
// PVEInstance.DeepCopy  (pve_instances.go:61)
// ---------------------------------------------------------------------------

func samplePVEInstance() PVEInstance {
	return PVEInstance{
		Name: "pve1", Host: "https://10.0.0.1:8006", User: "root@pam",
		Password: "secret", TokenName: "tok", TokenValue: "val",
		VerifySSL: false, MonitorVMs: true, MonitorContainers: true,
		IsCluster: true, ClusterName: "prod",
		ClusterEndpoints: []ClusterEndpoint{
			{NodeName: "node1", Host: "https://10.0.0.1:8006", IP: "10.0.0.1", Online: true, NativeNodeID: 1},
			{NodeName: "node2", Host: "https://10.0.0.2:8006", IP: "10.0.0.2", Online: false, NativeNodeID: 2},
		},
		ClusterNodeIdentities: []PVEClusterNodeIdentity{
			{ID: "id-1", NativeName: "node1", NativeAliases: []string{"alias-a", "alias-b"}},
			{ID: "id-2", NativeName: "node2"},
		},
	}
}

func TestBranchcov0724pmPVEInstanceDeepCopy(t *testing.T) {
	t.Run("deep copy is value-equal to the fully populated original", func(t *testing.T) {
		orig := samplePVEInstance()
		clone := orig.DeepCopy()
		assert.Equal(t, orig, clone, "DeepCopy must produce a value-equal copy")
	})

	t.Run("mutating the copy's nested slices does not affect the original", func(t *testing.T) {
		orig := samplePVEInstance()
		origEndpointsLen := len(orig.ClusterEndpoints)
		origIdentitiesLen := len(orig.ClusterNodeIdentities)
		origAliases := append([]string(nil), orig.ClusterNodeIdentities[0].NativeAliases...)
		origEp0Host := orig.ClusterEndpoints[0].Host

		clone := orig.DeepCopy()

		// Mutate the copy's nested slices and their nested alias slices.
		clone.ClusterEndpoints = append(clone.ClusterEndpoints, ClusterEndpoint{NodeName: "new"})
		clone.ClusterEndpoints[0].Host = "MUTATED-HOST"
		clone.ClusterNodeIdentities = append(clone.ClusterNodeIdentities, PVEClusterNodeIdentity{ID: "new-id"})
		clone.ClusterNodeIdentities[0].NativeAliases[0] = "MUTATED-ALIAS"
		clone.ClusterNodeIdentities[0].NativeAliases = append(clone.ClusterNodeIdentities[0].NativeAliases, "extra")

		// The original must be entirely unaffected.
		assert.Len(t, orig.ClusterEndpoints, origEndpointsLen, "appending to copy endpoints must not grow original")
		assert.Equal(t, origEp0Host, orig.ClusterEndpoints[0].Host, "mutating a copy endpoint field must not affect original")
		assert.Len(t, orig.ClusterNodeIdentities, origIdentitiesLen, "appending to copy identities must not grow original")
		assert.Equal(t, origAliases, orig.ClusterNodeIdentities[0].NativeAliases, "mutating copy alias slice must not affect original")
	})

	t.Run("instance with nil nested slices copies without panic", func(t *testing.T) {
		orig := PVEInstance{Name: "standalone", Host: "https://1.2.3.4:8006"}
		clone := orig.DeepCopy()
		assert.Equal(t, orig, clone)
		assert.Nil(t, clone.ClusterEndpoints, "nil nested slice must stay nil after copy")
		assert.Nil(t, clone.ClusterNodeIdentities)
	})

	t.Run("empty non-nil nested slices copy as empty", func(t *testing.T) {
		orig := PVEInstance{
			Name: "empty-cluster", IsCluster: true, ClusterName: "c",
			ClusterEndpoints:      []ClusterEndpoint{},
			ClusterNodeIdentities: []PVEClusterNodeIdentity{{ID: "x", NativeName: "n"}},
		}
		clone := orig.DeepCopy()
		assert.Equal(t, orig, clone)
		assert.NotNil(t, clone.ClusterEndpoints)
		assert.Empty(t, clone.ClusterEndpoints)
	})
}
