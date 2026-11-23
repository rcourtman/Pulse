package pulsecli

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli/authstore"
	"github.com/spf13/cobra"
)

const (
	doctorStatusPass = "pass"
	doctorStatusWarn = "warn"
	doctorStatusFail = "fail"
)

// DoctorDeps allows deterministic testing of `pulse doctor` without touching
// the host keychain or network. All fields are optional.
type DoctorDeps struct {
	HTTPClient HTTPDoer
	Store      *authstore.Store
	Getenv     func(string) string
}

type doctorOptions struct {
	APIURL  string
	Profile string
	JSON    bool
}

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

type doctorReport struct {
	OK              bool          `json:"ok"`
	Profile         string        `json:"profile,omitempty"`
	BaseURL         string        `json:"baseUrl,omitempty"`
	URLSource       string        `json:"urlSource,omitempty"`
	TokenSource     string        `json:"tokenSource,omitempty"`
	TokenBackend    string        `json:"tokenBackend,omitempty"`
	ManifestVersion string        `json:"manifestVersion,omitempty"`
	CapabilityCount int           `json:"capabilityCount,omitempty"`
	ResourceCount   int           `json:"resourceCount,omitempty"`
	Checks          []doctorCheck `json:"checks"`
}

// NewDoctorCommand constructs the `pulse doctor` diagnostic command.
func NewDoctorCommand(deps *DoctorDeps) *cobra.Command {
	var opts doctorOptions

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Verify Pulse CLI credentials, TLS reachability, manifest version, and token access",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.APIURL = resolveFlagString(cmd, "api-url", opts.APIURL)
			opts.Profile = resolveFlagString(cmd, "profile", opts.Profile)
			opts.JSON = resolveFlagBool(cmd, "json", opts.JSON)
			return runDoctor(cmd, deps, opts)
		},
	}

	cmd.Flags().StringVar(&opts.APIURL, "api-url", "", "Override Pulse server URL")
	cmd.Flags().StringVar(&opts.Profile, "profile", "", "Pulse profile name to verify (defaults to active default profile)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output diagnostic report as JSON")
	return cmd
}

func runDoctor(cmd *cobra.Command, deps *DoctorDeps, opts doctorOptions) error {
	report := doctorReport{
		OK:     true,
		Checks: make([]doctorCheck, 0, 4),
	}

	resolved, err := ResolveTargetDetailed(TargetResolverDeps{
		Store:  doctorDepsStore(deps),
		Getenv: doctorDepsGetenv(deps),
	}, opts.APIURL, opts.Profile)
	if err != nil {
		report.OK = false
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "credentials",
			Status: doctorStatusFail,
			Detail: err.Error(),
			Fix:    doctorCredentialsFix(opts.Profile),
		})
		return emitDoctorReport(cmd, report, opts.JSON)
	}

	report.BaseURL = resolved.BaseURL
	report.URLSource = resolved.URLSource
	report.TokenSource = resolved.TokenSource
	report.Profile = resolved.ProfileName
	report.TokenBackend = resolved.TokenBackend

	credDetail := fmt.Sprintf("URL %s (%s), token (%s)", resolved.BaseURL, formatSourceLabel(resolved.URLSource, resolved.ProfileName), formatTokenSourceLabel(resolved))
	report.Checks = append(report.Checks, doctorCheck{
		Name:   "credentials",
		Status: doctorStatusPass,
		Detail: credDetail,
	})

	// Transport / TLS check.
	switch {
	case strings.HasPrefix(resolved.BaseURL, "https://"):
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "transport",
			Status: doctorStatusPass,
			Detail: "HTTPS transport configured",
		})
	case isLoopbackURL(resolved.BaseURL):
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "transport",
			Status: doctorStatusPass,
			Detail: "Local loopback HTTP transport",
		})
	default:
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "transport",
			Status: doctorStatusWarn,
			Detail: "Plain HTTP configured for non-local host. API tokens travel unencrypted",
			Fix:    "Enable HTTPS on Pulse and re-run \"pulse auth login https://<host>\"",
		})
	}

	client := pulseCLISurfaceClient(doctorHTTPClient(deps))

	// Capabilities manifest check.
	manifest, err := agentcapabilities.FetchManifest(cmd.Context(), client, resolved.BaseURL)
	if err != nil {
		report.OK = false
		fix := fmt.Sprintf("Verify Pulse is running and reachable at %s%s", resolved.BaseURL, agentcapabilities.AgentCapabilitiesPath)
		if isTLSError(err) {
			fix = "Check that Pulse's TLS certificate is valid and trusted by your OS certificate store"
		}
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "manifest",
			Status: doctorStatusFail,
			Detail: err.Error(),
			Fix:    fix,
		})
		return emitDoctorReport(cmd, report, opts.JSON)
	}

	report.ManifestVersion = strings.TrimSpace(manifest.Version)
	report.CapabilityCount = len(manifest.Capabilities)
	if report.ManifestVersion == "" || report.CapabilityCount == 0 {
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "manifest",
			Status: doctorStatusWarn,
			Detail: fmt.Sprintf("Manifest returned version %q with %d capabilities", report.ManifestVersion, report.CapabilityCount),
			Fix:    "Upgrade Pulse server to a version serving the complete /api/agent/capabilities manifest",
		})
	} else {
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "manifest",
			Status: doctorStatusPass,
			Detail: fmt.Sprintf("Manifest %s reachable (%d capabilities)", report.ManifestVersion, report.CapabilityCount),
		})
	}

	// Token validity check via GET /api/agent/fleet-context.
	probe, err := probeFleetContext(cmd.Context(), client, resolved.BaseURL, resolved.Token)
	if err != nil {
		report.OK = false
		fix := fmt.Sprintf("Verify the token in %s and re-run \"pulse auth login %s\"", externalAgentsNavLabel, resolved.BaseURL)
		switch probe.StatusCode {
		case http.StatusUnauthorized:
			fix = fmt.Sprintf("Token was rejected. Create a new token in %s and re-run \"pulse auth login %s\"", externalAgentsNavLabel, resolved.BaseURL)
		case http.StatusForbidden:
			fix = fmt.Sprintf("Token needs monitoring:read scope. Create an \"External agent (read-only)\" token in %s and re-run \"pulse auth login %s\"", externalAgentsNavLabel, resolved.BaseURL)
		}
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "fleet_read",
			Status: doctorStatusFail,
			Detail: err.Error(),
			Fix:    fix,
		})
		return emitDoctorReport(cmd, report, opts.JSON)
	}

	report.ResourceCount = probe.ResourceCount
	report.Checks = append(report.Checks, doctorCheck{
		Name:   "fleet_read",
		Status: doctorStatusPass,
		Detail: fmt.Sprintf("Authenticated GET %s succeeded (%d visible resources)", agentcapabilities.FleetContextCapabilityPath, probe.ResourceCount),
	})

	return emitDoctorReport(cmd, report, opts.JSON)
}

func emitDoctorReport(cmd *cobra.Command, report doctorReport, asJSON bool) error {
	if asJSON {
		if err := writeJSONIndent(cmd.OutOrStdout(), report); err != nil {
			return err
		}
	} else {
		out := cmd.OutOrStdout()
		for _, check := range report.Checks {
			label := strings.ToUpper(check.Status)
			fmt.Fprintf(out, "[%s] %-12s %s\n", label, check.Name, check.Detail)
			if check.Fix != "" {
				fmt.Fprintf(out, "       Fix: %s\n", check.Fix)
			}
		}
		if report.OK {
			fmt.Fprintf(out, "\nPulse CLI is ready.\n")
		}
	}

	failing := 0
	for _, check := range report.Checks {
		if check.Status == doctorStatusFail {
			failing++
		}
	}
	if failing > 0 {
		return fmt.Errorf("pulse doctor found %d failing check(s)", failing)
	}
	return nil
}

func doctorCredentialsFix(profile string) string {
	profile = strings.TrimSpace(profile)
	if profile != "" && profile != authstore.DefaultProfileName {
		return fmt.Sprintf("Run \"pulse auth login <url> --profile %s\" or set PULSE_API_URL and PULSE_API_TOKEN", profile)
	}
	return "Run \"pulse auth login <url>\" or set PULSE_API_URL and PULSE_API_TOKEN"
}

func formatSourceLabel(source, profile string) string {
	if source == "profile" && profile != "" {
		return "profile:" + profile
	}
	return source
}

func formatTokenSourceLabel(resolved ResolvedTarget) string {
	if resolved.TokenSource == "profile" {
		if resolved.TokenBackend != "" {
			return fmt.Sprintf("profile:%s via %s", resolved.ProfileName, resolved.TokenBackend)
		}
		return "profile:" + resolved.ProfileName
	}
	return resolved.TokenSource
}

func isTLSError(err error) bool {
	if err == nil {
		return false
	}
	var unknownAuth x509.UnknownAuthorityError
	var certInvalid x509.CertificateInvalidError
	var hostErr x509.HostnameError
	if errors.As(err, &unknownAuth) || errors.As(err, &certInvalid) || errors.As(err, &hostErr) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "x509:") || strings.Contains(lower, "tls:")
}

func doctorHTTPClient(deps *DoctorDeps) HTTPDoer {
	if deps != nil && deps.HTTPClient != nil {
		return deps.HTTPClient
	}
	return nil
}

func doctorDepsStore(deps *DoctorDeps) *authstore.Store {
	if deps != nil {
		return deps.Store
	}
	return nil
}

func doctorDepsGetenv(deps *DoctorDeps) func(string) string {
	if deps != nil {
		return deps.Getenv
	}
	return nil
}
