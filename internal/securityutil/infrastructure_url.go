package securityutil

import (
	"errors"
	"fmt"
	"net/url"
)

// NormalizeInfrastructureBaseURL keeps the ordinary base-URL policy, but must
// not return url.Parse's copy of rejected input in an error. Configuration can
// accidentally put credentials in userinfo, a query, or even invalid syntax.
func NormalizeInfrastructureBaseURL(raw, defaultScheme string) (*url.URL, error) {
	u, err := NormalizeHTTPBaseURL(raw, defaultScheme)
	var parseError *url.Error
	if errors.As(err, &parseError) {
		return nil, fmt.Errorf("URL syntax is invalid")
	}
	return u, err
}

// InfrastructureEndpointDiagnostic is a display-only label, never a routing or
// trust identity. Only validated base URLs may enter logs and error messages.
// Invalid input stays in its owning configuration, not in a diagnostic label.
func InfrastructureEndpointDiagnostic(raw string) string {
	u, err := NormalizeInfrastructureBaseURL(raw, "https")
	if err != nil {
		return "[invalid endpoint]"
	}
	return u.String()
}

func InfrastructureEndpointsDiagnostic(raw []string) []string {
	labels := make([]string, len(raw))
	for i, endpoint := range raw {
		labels[i] = InfrastructureEndpointDiagnostic(endpoint)
	}
	return labels
}
