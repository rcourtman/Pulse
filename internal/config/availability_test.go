package config

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeAvailabilityTargetPreservesHTTPAddress(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		ID:       " target-1 ",
		Name:     "  Status page ",
		Address:  " https://device.local/status?ready=1 ",
		Protocol: AvailabilityProbeHTTP,
		Path:     " health ",
		Enabled:  true,
	})

	if target.ID != "target-1" {
		t.Fatalf("ID = %q, want target-1", target.ID)
	}
	if target.Name != "Status page" {
		t.Fatalf("Name = %q, want Status page", target.Name)
	}
	if target.TargetKind != AvailabilityTargetService {
		t.Fatalf("TargetKind = %q, want %q", target.TargetKind, AvailabilityTargetService)
	}
	if target.Address != "https://device.local/status?ready=1" {
		t.Fatalf("Address = %q, want preserved HTTP URL", target.Address)
	}
	if target.Path != "health" {
		t.Fatalf("Path = %q, want health", target.Path)
	}
}

func TestAvailabilityExecutionConfigChangedTracksOnlyExecutionDefiningFields(t *testing.T) {
	base := NormalizeAvailabilityTarget(AvailabilityTarget{
		ID: "target-1", Name: "Gateway", Address: "gateway.local",
		Protocol: AvailabilityProbeTCP, Port: 443, Enabled: true,
		PollIntervalSecs: 60, TimeoutMillis: 1500, ProbeAgentID: "agent-1",
	})

	presentationOnly := base
	presentationOnly.Name = "Primary gateway"
	presentationOnly.LinkedResourceID = "node-1"
	presentationOnly.FailureThreshold = 5
	if AvailabilityExecutionConfigChanged(base, presentationOnly) {
		t.Fatal("presentation and alert-only edits changed the execution revision")
	}

	for name, mutate := range map[string]func(*AvailabilityTarget){
		"address":       func(target *AvailabilityTarget) { target.Address = "gateway-2.local" },
		"protocol":      func(target *AvailabilityTarget) { target.Protocol = AvailabilityProbeICMP },
		"port":          func(target *AvailabilityTarget) { target.Port = 8443 },
		"path":          func(target *AvailabilityTarget) { target.Path = "/health" },
		"timeout":       func(target *AvailabilityTarget) { target.TimeoutMillis = 2500 },
		"poll interval": func(target *AvailabilityTarget) { target.PollIntervalSecs = 120 },
		"probe agent":   func(target *AvailabilityTarget) { target.ProbeAgentID = "agent-2" },
	} {
		t.Run(name, func(t *testing.T) {
			next := base
			mutate(&next)
			if !AvailabilityExecutionConfigChanged(base, next) {
				t.Fatalf("%s edit did not change the execution revision", name)
			}
		})
	}
}

func TestNormalizeAvailabilityTargetReducesICMPAddressToHost(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  " https://device.local:8443/status ",
		Protocol: AvailabilityProbeICMP,
		Enabled:  true,
	})

	if target.Address != "device.local" {
		t.Fatalf("Address = %q, want device.local", target.Address)
	}
	if target.Port != 0 {
		t.Fatalf("Port = %d, want 0", target.Port)
	}
}

func TestNormalizeAvailabilityTargetAcceptsPingAlias(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  " https://device.local:8443/status ",
		Protocol: AvailabilityProbeProtocol(" Ping "),
		Enabled:  true,
	})

	if target.Protocol != AvailabilityProbeICMP {
		t.Fatalf("Protocol = %q, want %q", target.Protocol, AvailabilityProbeICMP)
	}
	if target.Address != "device.local" {
		t.Fatalf("Address = %q, want device.local", target.Address)
	}
}

func TestAvailabilityTargetProbeAddressUsesHTTPHostname(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  "http://solar-inverter.lab.local/status",
		Protocol: AvailabilityProbeHTTP,
		Enabled:  true,
	})

	if got := target.ProbeAddress(); got != "solar-inverter.lab.local" {
		t.Fatalf("ProbeAddress() = %q, want solar-inverter.lab.local", got)
	}
}

func TestAvailabilityTargetHTTPURLAppliesPortAndPath(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  "device.local/status",
		Protocol: AvailabilityProbeHTTP,
		Port:     8080,
		Path:     "health",
		Enabled:  true,
	})

	u, err := target.HTTPURL()
	if err != nil {
		t.Fatalf("HTTPURL() error = %v", err)
	}
	if got := u.String(); got != "http://device.local:8080/health" {
		t.Fatalf("HTTPURL() = %q, want http://device.local:8080/health", got)
	}
}

func TestAvailabilityTargetHTTPURLDoesNotEchoMalformedAddress(t *testing.T) {
	const sentinel = "address-secret-sentinel"
	target := AvailabilityTarget{
		Address:  "http://[" + sentinel,
		Protocol: AvailabilityProbeHTTP,
	}
	_, err := target.HTTPURL()
	if err == nil {
		t.Fatal("HTTPURL() error = nil, want malformed address error")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("HTTPURL() error leaked malformed address content: %v", err)
	}
}

func TestAvailabilityHTTPContractRejectsUnboundedOrExecutableShapes(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address: "https://service.local/health", Protocol: AvailabilityProbeHTTPS, Enabled: true,
		HTTP: &AvailabilityHTTPConfig{
			Method:            AvailabilityHTTPMethodGET,
			Authentication:    AvailabilityHTTPAuthentication{Type: AvailabilityHTTPAuthNone},
			ExpectedStatusMin: 200, ExpectedStatusMax: 299,
			JSONPath: "data[not-an-index].status",
		},
	})
	if err := target.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want bounded JSON path validation")
	}

	target.HTTP.JSONPath = "data.status"
	secret := "value"
	target.HTTP.Headers = []AvailabilityHTTPHeader{{ID: "reserved", Name: "Authorization", Value: &secret}}
	if err := target.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want reserved header validation")
	}
}

func TestAvailabilityTargetValidateRejectsTCPWithoutPort(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  "device.local",
		Protocol: AvailabilityProbeTCP,
		Enabled:  true,
	})

	if err := target.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want TCP port error")
	}
}

func TestAvailabilityTargetValidateAcceptsPingAlias(t *testing.T) {
	target := AvailabilityTarget{
		TargetKind: AvailabilityTargetService,
		Address:    "device.local",
		Protocol:   AvailabilityProbeProtocol("ping"),
		Enabled:    true,
	}

	if err := target.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestAvailabilityTargetValidateRejectsUnsupportedTargetKind(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		TargetKind: AvailabilityTargetKind("database"),
		Address:    "device.local",
		Protocol:   AvailabilityProbeICMP,
		Enabled:    true,
	})

	if err := target.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want target kind error")
	}
}

func TestAvailabilityTargetsRoundTripThroughPersistence(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())
	targets := []AvailabilityTarget{
		{
			ID:         "endpoint-1",
			Name:       "Energy monitor",
			TargetKind: AvailabilityTargetDevice,
			Address:    "device.local",
			Protocol:   AvailabilityProbeICMP,
			Enabled:    true,
		},
	}

	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}

	loaded, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("LoadAvailabilityTargets() length = %d, want 1", len(loaded))
	}
	if loaded[0].Name != "Energy monitor" {
		t.Fatalf("loaded name = %q, want Energy monitor", loaded[0].Name)
	}
	if loaded[0].TargetKind != AvailabilityTargetDevice {
		t.Fatalf("loaded target kind = %q, want %q", loaded[0].TargetKind, AvailabilityTargetDevice)
	}
	if loaded[0].PollIntervalSecs != DefaultAvailabilityPollIntervalSecs {
		t.Fatalf("poll interval = %d, want default", loaded[0].PollIntervalSecs)
	}
}

func TestAvailabilityHTTPContractSecretsRemainEncryptedAtRest(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())
	password := "plaintext-password-sentinel"
	token := "plaintext-token-sentinel"
	header := "plaintext-header-sentinel"
	body := "plaintext-body-sentinel"
	target := AvailabilityTarget{
		ID: "secure-http", Address: "https://service.local/health", Protocol: AvailabilityProbeHTTPS, Enabled: true,
		HTTP: &AvailabilityHTTPConfig{
			Method:         AvailabilityHTTPMethodPOST,
			Headers:        []AvailabilityHTTPHeader{{ID: "header-1", Name: "X-Health-Key", Value: &header}},
			Authentication: AvailabilityHTTPAuthentication{Type: AvailabilityHTTPAuthBasic, Username: "pulse", Password: &password},
			Body:           &body, ExpectedStatusMin: 200, ExpectedStatusMax: 299,
		},
	}
	bearerTarget := AvailabilityTarget{
		ID: "secure-bearer", Address: "https://service.local/bearer-health", Protocol: AvailabilityProbeHTTPS, Enabled: true,
		HTTP: &AvailabilityHTTPConfig{
			Method:            AvailabilityHTTPMethodGET,
			Authentication:    AvailabilityHTTPAuthentication{Type: AvailabilityHTTPAuthBearer, BearerToken: &token},
			ExpectedStatusMin: 200, ExpectedStatusMax: 299,
		},
	}
	if err := persistence.SaveAvailabilityTargets([]AvailabilityTarget{target, bearerTarget}); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}
	raw, err := os.ReadFile(persistence.availabilityFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", persistence.availabilityFile, err)
	}
	for _, secret := range []string{password, token, header, body} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("encrypted availability file contains plaintext secret %q", secret)
		}
	}
	loaded, err := persistence.LoadAvailabilityTargets()
	if err != nil || len(loaded) != 2 || loaded[0].HTTP == nil || loaded[1].HTTP == nil {
		t.Fatalf("LoadAvailabilityTargets() = %+v, %v", loaded, err)
	}
	if loaded[0].HTTP.Authentication.Password == nil || *loaded[0].HTTP.Authentication.Password != password ||
		loaded[0].HTTP.Headers[0].Value == nil || *loaded[0].HTTP.Headers[0].Value != header ||
		loaded[0].HTTP.Body == nil || *loaded[0].HTTP.Body != body {
		t.Fatalf("decrypted contract did not round trip: %+v", loaded[0].HTTP)
	}
	if loaded[1].HTTP.Authentication.BearerToken == nil || *loaded[1].HTTP.Authentication.BearerToken != token {
		t.Fatalf("decrypted bearer token did not round trip: %+v", loaded[1].HTTP)
	}
}

func TestNormalizeAvailabilityTargetTrimsProbeAgentID(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:      "device.local",
		Protocol:     AvailabilityProbeICMP,
		Enabled:      true,
		ProbeAgentID: "  agent-remote-1  ",
	})

	if target.ProbeAgentID != "agent-remote-1" {
		t.Fatalf("ProbeAgentID = %q, want agent-remote-1", target.ProbeAgentID)
	}

	local := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:      "device.local",
		Protocol:     AvailabilityProbeICMP,
		Enabled:      true,
		ProbeAgentID: "   ",
	})
	if local.ProbeAgentID != "" {
		t.Fatalf("ProbeAgentID = %q, want empty for a locally executed target", local.ProbeAgentID)
	}
	if err := local.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want probe assignment to stay an API-layer concern", err)
	}
}

func TestAvailabilityTargetProbeAgentIDRoundTripsThroughPersistence(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())
	targets := []AvailabilityTarget{
		{
			ID:           "endpoint-remote",
			Name:         "Remote branch gateway",
			TargetKind:   AvailabilityTargetDevice,
			Address:      "gateway.branch.local",
			Protocol:     AvailabilityProbeICMP,
			Enabled:      true,
			ProbeAgentID: " agent-branch ",
		},
		{
			ID:       "endpoint-local",
			Name:     "Local gateway",
			Address:  "gateway.local",
			Protocol: AvailabilityProbeICMP,
			Enabled:  true,
		},
	}

	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}

	loaded, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("LoadAvailabilityTargets() length = %d, want 2", len(loaded))
	}
	byID := map[string]AvailabilityTarget{}
	for _, target := range loaded {
		byID[target.ID] = target
	}
	if got := byID["endpoint-remote"].ProbeAgentID; got != "agent-branch" {
		t.Fatalf("persisted ProbeAgentID = %q, want agent-branch", got)
	}
	if got := byID["endpoint-local"].ProbeAgentID; got != "" {
		t.Fatalf("local target ProbeAgentID = %q, want empty", got)
	}
}

func TestAvailabilityTargetCertificateMonitoringDefaultsAndValidation(t *testing.T) {
	httpsTarget := AvailabilityTarget{
		Address:  "https://pulse.example.test",
		Protocol: AvailabilityProbeHTTPS,
		Enabled:  true,
	}
	if !httpsTarget.CertificateMonitoringEnabled() {
		t.Fatal("HTTPS certificate monitoring should be enabled by default")
	}
	if got := httpsTarget.EffectiveCertificateExpiryWarningDays(); got != DefaultCertificateExpiryWarningDays {
		t.Fatalf("warning days = %d, want %d", got, DefaultCertificateExpiryWarningDays)
	}

	httpsTarget.CertificateMonitoringDisabled = true
	if httpsTarget.CertificateMonitoringEnabled() {
		t.Fatal("explicit certificate monitoring opt-out was ignored")
	}

	httpTarget := AvailabilityTarget{
		Address:                      "http://pulse.example.test",
		Protocol:                     AvailabilityProbeHTTP,
		Enabled:                      true,
		CertificateExpiryWarningDays: 14,
	}
	if err := httpTarget.Validate(); err == nil {
		t.Fatal("non-HTTPS target accepted certificate monitoring settings")
	}

	httpsTarget.CertificateMonitoringDisabled = false
	httpsTarget.CertificateExpiryWarningDays = 3651
	if err := httpsTarget.Validate(); err == nil {
		t.Fatal("HTTPS target accepted an excessive certificate warning window")
	}
}
