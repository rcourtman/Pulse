package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

const aiProfileDescription = "Managed by Pulse AI"

// MCPAgentProfileManager manages agent profiles for MCP tools.
type MCPAgentProfileManager struct {
	persistence    *config.ConfigPersistence
	licenseService *license.Service
	validator      *models.ProfileValidator
}

func NewMCPAgentProfileManager(persistence *config.ConfigPersistence, licenseService *license.Service) *MCPAgentProfileManager {
	return &MCPAgentProfileManager{
		persistence:    persistence,
		licenseService: licenseService,
		validator:      models.NewProfileValidator(),
	}
}

func (m *MCPAgentProfileManager) ApplyAgentScope(_ context.Context, agentID, agentLabel string, settings map[string]interface{}) (string, string, bool, error) {
	if err := m.requireLicense(); err != nil {
		return "", "", false, err
	}
	if m.persistence == nil {
		return "", "", false, fmt.Errorf("profile persistence unavailable")
	}
	if strings.TrimSpace(agentID) == "" {
		return "", "", false, fmt.Errorf("agent ID is required")
	}
	if len(settings) == 0 {
		return "", "", false, fmt.Errorf("settings are required")
	}
	if err := m.validateSettings(settings); err != nil {
		return "", "", false, err
	}

	profileName := buildScopeProfileName(agentLabel, agentID)
	now := time.Now()
	username := "ai"

	profiles, err := m.persistence.LoadAgentProfiles()
	if err != nil {
		return "", "", false, fmt.Errorf("failed to load profiles: %w", err)
	}

	created := true
	var profile models.AgentProfile
	for i := range profiles {
		if profiles[i].Name == profileName {
			created = false
			profiles[i].Config = settings
			profiles[i].Description = aiProfileDescription
			profiles[i].UpdatedAt = now
			profiles[i].UpdatedBy = username
			profiles[i].Version++
			profile = profiles[i]
			break
		}
	}

	if created {
		profile = models.AgentProfile{
			ID:          uuid.New().String(),
			Name:        profileName,
			Description: aiProfileDescription,
			Config:      settings,
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   username,
			UpdatedBy:   username,
		}
		profiles = append(profiles, profile)
	}

	if err := m.persistence.SaveAgentProfiles(profiles); err != nil {
		return "", "", false, fmt.Errorf("failed to save profile: %w", err)
	}

	if err := m.saveVersion(profile, "AI scope update"); err != nil {
		log.Warn().Err(err).Msg("Failed to record profile version history")
	}

	changeAction := "update"
	if created {
		changeAction = "create"
	}
	m.logChange(models.ProfileChangeLog{
		ID:          uuid.New().String(),
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Action:      changeAction,
		OldVersion:  profile.Version - 1,
		NewVersion:  profile.Version,
		User:        username,
		Timestamp:   now,
	})

	if err := m.assignProfile(agentID, profile, username); err != nil {
		return "", "", created, err
	}

	return profile.ID, profile.Name, created, nil
}

func (m *MCPAgentProfileManager) AssignProfile(_ context.Context, agentID, profileID string) (string, error) {
	if err := m.requireLicense(); err != nil {
		return "", err
	}
	if m.persistence == nil {
		return "", fmt.Errorf("profile persistence unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	profileID = strings.TrimSpace(profileID)
	if agentID == "" || profileID == "" {
		return "", fmt.Errorf("agent ID and profile ID are required")
	}

	profiles, err := m.persistence.LoadAgentProfiles()
	if err != nil {
		return "", fmt.Errorf("failed to load profiles: %w", err)
	}

	var profile models.AgentProfile
	found := false
	for _, p := range profiles {
		if p.ID == profileID {
			profile = p
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("profile %s not found", profileID)
	}

	if err := m.assignProfile(agentID, profile, "ai"); err != nil {
		return "", err
	}

	return profile.Name, nil
}

func (m *MCPAgentProfileManager) GetAgentScope(_ context.Context, agentID string) (*tools.AgentScope, error) {
	if err := m.requireLicense(); err != nil {
		return nil, err
	}
	if m.persistence == nil {
		return nil, fmt.Errorf("profile persistence unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}

	assignments, err := m.persistence.LoadAgentProfileAssignments()
	if err != nil {
		return nil, fmt.Errorf("failed to load assignments: %w", err)
	}

	var assignment *models.AgentProfileAssignment
	for i := range assignments {
		if assignments[i].AgentID == agentID {
			assignment = &assignments[i]
			break
		}
	}
	if assignment == nil {
		return nil, nil
	}

	profiles, err := m.persistence.LoadAgentProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to load profiles: %w", err)
	}

	for _, profile := range profiles {
		if profile.ID == assignment.ProfileID {
			return &tools.AgentScope{
				AgentID:        agentID,
				ProfileID:      profile.ID,
				ProfileName:    profile.Name,
				ProfileVersion: assignment.ProfileVersion,
				Settings:       profile.Config,
			}, nil
		}
	}

	return nil, nil
}

func (m *MCPAgentProfileManager) assignProfile(agentID string, profile models.AgentProfile, username string) error {
	assignments, err := m.persistence.LoadAgentProfileAssignments()
	if err != nil {
		return fmt.Errorf("failed to load assignments: %w", err)
	}

	trimmed := []models.AgentProfileAssignment{}
	for _, a := range assignments {
		if a.AgentID != agentID {
			trimmed = append(trimmed, a)
		}
	}

	trimmed = append(trimmed, models.AgentProfileAssignment{
		AgentID:        agentID,
		ProfileID:      profile.ID,
		ProfileVersion: profile.Version,
		UpdatedAt:      time.Now(),
		AssignedBy:     username,
	})

	if err := m.persistence.SaveAgentProfileAssignments(trimmed); err != nil {
		return fmt.Errorf("failed to save assignment: %w", err)
	}

	m.logChange(models.ProfileChangeLog{
		ID:          uuid.New().String(),
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Action:      "assign",
		AgentID:     agentID,
		User:        username,
		Timestamp:   time.Now(),
	})

	return nil
}

func (m *MCPAgentProfileManager) validateSettings(settings map[string]interface{}) error {
	if m.validator == nil {
		return nil
	}
	result := m.validator.Validate(settings)
	if !result.Valid || len(result.Warnings) > 0 {
		return fmt.Errorf("invalid settings: %s", formatValidationIssues(result))
	}
	return nil
}

func formatValidationIssues(result models.ValidationResult) string {
	parts := make([]string, 0, len(result.Errors)+len(result.Warnings))
	for _, err := range result.Errors {
		parts = append(parts, fmt.Sprintf("%s (%s)", err.Key, err.Message))
	}
	for _, warn := range result.Warnings {
		parts = append(parts, fmt.Sprintf("warning: %s (%s)", warn.Key, warn.Message))
	}
	if len(parts) == 0 {
		return "unknown validation error"
	}
	return strings.Join(parts, "; ")
}

func (m *MCPAgentProfileManager) saveVersion(profile models.AgentProfile, note string) error {
	versions, err := m.persistence.LoadAgentProfileVersions()
	if err != nil {
		return err
	}

	versions = append(versions, models.AgentProfileVersion{
		ProfileID:   profile.ID,
		Version:     profile.Version,
		Name:        profile.Name,
		Description: profile.Description,
		Config:      profile.Config,
		ParentID:    profile.ParentID,
		CreatedAt:   time.Now(),
		CreatedBy:   "ai",
		ChangeNote:  note,
	})

	return m.persistence.SaveAgentProfileVersions(versions)
}

func (m *MCPAgentProfileManager) logChange(entry models.ProfileChangeLog) {
	if err := m.persistence.AppendProfileChangeLog(entry); err != nil {
		log.Warn().Err(err).Msg("Failed to log profile change")
	}
}

func (m *MCPAgentProfileManager) requireLicense() error {
	if m.licenseService == nil {
		return nil
	}
	return m.licenseService.RequireFeature(license.FeatureAgentProfiles)
}

func buildScopeProfileName(agentLabel, agentID string) string {
	label := strings.TrimSpace(agentLabel)
	if label == "" || strings.EqualFold(label, agentID) {
		return fmt.Sprintf("AI Scope: %s", agentID)
	}
	return fmt.Sprintf("AI Scope: %s (%s)", label, agentID)
}
