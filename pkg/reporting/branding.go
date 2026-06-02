package reporting

import (
	"strings"
)

// ReportBrand describes custom report branding supplied by configuration.
// A brand is custom white-label material; an empty brand falls back to the
// built-in Pulse report identity.
type ReportBrand struct {
	DisplayName string
	LogoPath    string
	LogoData    []byte
	LogoFormat  string
}

// ReportBranding carries the provider default and the workspace override into
// the reporting layer, along with the entitlement decision for the request.
type ReportBranding struct {
	Entitled          bool
	ProviderDefault   ReportBrand
	WorkspaceOverride ReportBrand
}

// EffectiveBrand returns the white-label brand that should be rendered. The
// reporting layer enforces the entitlement gate so callers cannot accidentally
// render configured branding without the white_label capability.
func (b ReportBranding) EffectiveBrand() *ReportBrand {
	if !b.Entitled {
		return nil
	}
	if brand, ok := b.WorkspaceOverride.normalized(); ok {
		return &brand
	}
	if brand, ok := b.ProviderDefault.normalized(); ok {
		return &brand
	}
	return nil
}

func (b ReportBrand) normalized() (ReportBrand, bool) {
	normalized := ReportBrand{
		DisplayName: strings.TrimSpace(b.DisplayName),
		LogoPath:    strings.TrimSpace(b.LogoPath),
		LogoFormat:  normalizeLogoFormat(b.LogoFormat),
	}
	if len(b.LogoData) > 0 {
		normalized.LogoData = append([]byte(nil), b.LogoData...)
	}
	if normalized.DisplayName == "" && normalized.LogoPath == "" && len(normalized.LogoData) == 0 {
		return ReportBrand{}, false
	}
	return normalized, true
}

func normalizeLogoFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpg", "jpeg":
		return "jpg"
	case "png":
		return "png"
	case "gif":
		return "gif"
	default:
		return ""
	}
}
