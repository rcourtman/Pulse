package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestLoadProfileDataPlaintextWithCrypto(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cm, err := crypto.NewCryptoManagerAt(tempDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt: %v", err)
	}
	cp.crypto = cm

	writeJSON := func(name string, payload interface{}) {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, name), data, 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	t.Run("AgentProfiles", func(t *testing.T) {
		writeJSON("agent_profiles.json", []models.AgentProfile{
			{
				ID:      "profile-1",
				Name:    "Prod",
				Version: 1,
				Config:  models.AgentConfigMap{"interval": "1m"},
			},
		})

		profiles, err := cp.LoadAgentProfiles()
		if err != nil {
			t.Fatalf("LoadAgentProfiles: %v", err)
		}
		if len(profiles) != 1 || profiles[0].ID != "profile-1" {
			t.Fatalf("unexpected profiles: %+v", profiles)
		}
	})

	t.Run("Assignments", func(t *testing.T) {
		writeJSON("agent_profile_assignments.json", []models.AgentProfileAssignment{
			{
				AgentID:        "agent-1",
				ProfileID:      "profile-1",
				ProfileVersion: 1,
			},
		})

		assignments, err := cp.LoadAgentProfileAssignments()
		if err != nil {
			t.Fatalf("LoadAgentProfileAssignments: %v", err)
		}
		if len(assignments) != 1 || assignments[0].AgentID != "agent-1" {
			t.Fatalf("unexpected assignments: %+v", assignments)
		}
	})

	t.Run("Versions", func(t *testing.T) {
		writeJSON("profile-versions.json", []models.AgentProfileVersion{
			{
				ProfileID: "profile-1",
				Version:   1,
				Name:      "Prod",
				Config:    models.AgentConfigMap{"interval": "1m"},
			},
		})

		versions, err := cp.LoadAgentProfileVersions()
		if err != nil {
			t.Fatalf("LoadAgentProfileVersions: %v", err)
		}
		if len(versions) != 1 || versions[0].ProfileID != "profile-1" {
			t.Fatalf("unexpected versions: %+v", versions)
		}
	})

	t.Run("DeploymentStatus", func(t *testing.T) {
		writeJSON("profile-deployments.json", []models.ProfileDeploymentStatus{
			{
				AgentID:          "agent-1",
				ProfileID:        "profile-1",
				AssignedVersion:  1,
				DeployedVersion:  1,
				DeploymentStatus: "pending",
			},
		})

		status, err := cp.LoadProfileDeploymentStatus()
		if err != nil {
			t.Fatalf("LoadProfileDeploymentStatus: %v", err)
		}
		if len(status) != 1 || status[0].AgentID != "agent-1" {
			t.Fatalf("unexpected deployment status: %+v", status)
		}
	})
}
