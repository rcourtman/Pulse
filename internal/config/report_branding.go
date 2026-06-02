package config

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	ReportBrandDisplayNameMaxLength = 120
	ReportBrandLogoPathMaxLength    = 1024
	ReportBrandLogoBase64MaxLength  = 48 * 1024
)

// DecodeReportBrandLogoBase64 decodes a plain base64 value or a data URL value.
func DecodeReportBrandLogoBase64(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if idx := strings.Index(value, ";base64,"); idx >= 0 {
		value = value[idx+len(";base64,"):]
	}
	if len(value) > ReportBrandLogoBase64MaxLength {
		return nil, fmt.Errorf("logoBase64 must be <= %d characters", ReportBrandLogoBase64MaxLength)
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("logoBase64 must be valid base64")
	}
	return decoded, nil
}

func CanonicalReportBrandLogoFormat(format string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "":
		return "", true
	case "png":
		return "png", true
	case "jpg", "jpeg":
		return "jpg", true
	case "gif":
		return "gif", true
	default:
		return "", false
	}
}
