package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// validateImportSections distinguishes an actual configuration backup from JSON
// that would silently decode to a zero-value ExportData. Every emitted format,
// including 4.0, contains these sections. Later additions remain optional, and
// unknown fields/versions retain their existing best-effort compatibility.
func validateImportSections(data []byte) error {
	var sections struct {
		Nodes    json.RawMessage `json:"nodes"`
		Alerts   json.RawMessage `json:"alerts"`
		Email    json.RawMessage `json:"email"`
		Webhooks json.RawMessage `json:"webhooks"`
		System   json.RawMessage `json:"system"`
	}
	if err := json.Unmarshal(data, &sections); err != nil {
		return fmt.Errorf("invalid configuration backup: %w", err)
	}
	for _, section := range []struct {
		name string
		data json.RawMessage
	}{
		{"nodes", sections.Nodes}, {"alerts", sections.Alerts},
		{"email", sections.Email}, {"system", sections.System},
	} {
		if len(section.data) == 0 || bytes.Equal(bytes.TrimSpace(section.data), []byte("null")) {
			return fmt.Errorf("configuration backup is missing the %s section", section.name)
		}
	}
	// An empty webhook inventory is legitimately encoded as null by older
	// exports as well as by a fresh installation. Absence is not the same.
	if len(sections.Webhooks) == 0 {
		return fmt.Errorf("configuration backup is missing the webhooks section")
	}
	return nil
}
