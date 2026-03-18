package config

import (
	"bytes"
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
		filePath := filepath.Join(tempDir, "agent_profiles.json")
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
		rewritten, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile rewritten profiles: %v", err)
		}
		plain, err := json.Marshal([]models.AgentProfile{
			{
				ID:      "profile-1",
				Name:    "Prod",
				Version: 1,
				Config:  models.AgentConfigMap{"interval": "1m"},
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal plain profiles: %v", err)
		}
		if bytes.Equal(rewritten, plain) {
			t.Fatalf("expected plaintext profiles file to be rewritten encrypted")
		}
	})

	t.Run("Assignments", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "agent_profile_assignments.json")
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
		rewritten, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile rewritten assignments: %v", err)
		}
		plain, err := json.Marshal([]models.AgentProfileAssignment{
			{
				AgentID:        "agent-1",
				ProfileID:      "profile-1",
				ProfileVersion: 1,
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal plain assignments: %v", err)
		}
		if bytes.Equal(rewritten, plain) {
			t.Fatalf("expected plaintext assignments file to be rewritten encrypted")
		}
	})

	t.Run("Versions", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "profile-versions.json")
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
		rewritten, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile rewritten versions: %v", err)
		}
		plain, err := json.Marshal([]models.AgentProfileVersion{
			{
				ProfileID: "profile-1",
				Version:   1,
				Name:      "Prod",
				Config:    models.AgentConfigMap{"interval": "1m"},
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal plain versions: %v", err)
		}
		if bytes.Equal(rewritten, plain) {
			t.Fatalf("expected plaintext versions file to be rewritten encrypted")
		}
	})

	t.Run("DeploymentStatus", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "profile-deployments.json")
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
		rewritten, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile rewritten deployment status: %v", err)
		}
		plain, err := json.Marshal([]models.ProfileDeploymentStatus{
			{
				AgentID:          "agent-1",
				ProfileID:        "profile-1",
				AssignedVersion:  1,
				DeployedVersion:  1,
				DeploymentStatus: "pending",
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal plain deployment status: %v", err)
		}
		if bytes.Equal(rewritten, plain) {
			t.Fatalf("expected plaintext deployment status file to be rewritten encrypted")
		}
	})
}
