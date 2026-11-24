package config

import (
	"os"
	"reflect"
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

func TestAvailabilityHTTPContractNormalizesAndValidatesCertificateFingerprint(t *testing.T) {
	target := AvailabilityTarget{
		Address: "https://service.local/health", Protocol: AvailabilityProbeHTTPS, Enabled: true,
		HTTP: &AvailabilityHTTPConfig{TLSFingerprintSHA256: " AB:" + strings.Repeat("cd:", 30) + "EF "},
	}
	normalized := NormalizeAvailabilityTarget(target)
	if want := "ab" + strings.Repeat("cd", 30) + "ef"; normalized.HTTP.TLSFingerprintSHA256 != want {
		t.Fatalf("fingerprint = %q, want %q", normalized.HTTP.TLSFingerprintSHA256, want)
	}
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want a SHA-256 fingerprint accepted", err)
	}

	for _, fingerprint := range []string{"abcd", strings.Repeat("zz", 32), strings.Repeat("ab", 20)} {
		target.HTTP = &AvailabilityHTTPConfig{TLSFingerprintSHA256: fingerprint}
		if err := NormalizeAvailabilityTarget(target).Validate(); err == nil {
			t.Fatalf("Validate() error = nil for fingerprint %q, want SHA-256 validation", fingerprint)
		}
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

func TestAvailabilityTargetValidateAcceptsEveryTargetKind(t *testing.T) {
	gear := map[AvailabilityTargetKind]bool{
		AvailabilityTargetRouter:      true,
		AvailabilityTargetSwitch:      true,
		AvailabilityTargetAccessPoint: true,
	}
	for _, kind := range []AvailabilityTargetKind{
		AvailabilityTargetMachine,
		AvailabilityTargetService,
		AvailabilityTargetDevice,
		AvailabilityTargetRouter,
		AvailabilityTargetSwitch,
		AvailabilityTargetAccessPoint,
	} {
		target := NewAvailabilityTarget()
		target.Name = "thing"
		target.Address = "192.168.0.1"
		target.TargetKind = kind
		if err := target.Validate(); err != nil {
			t.Fatalf("Validate() with kind %q = %v, want nil", kind, err)
		}
		if got := kind.IsNetworkGear(); got != gear[kind] {
			t.Fatalf("%q.IsNetworkGear() = %v, want %v", kind, got, gear[kind])
		}
	}

	// A kind arrives from the API in any case and is stored in one.
	spelled := NewAvailabilityTarget()
	spelled.Address = "192.168.0.3"
	spelled.TargetKind = AvailabilityTargetKind(" Access_Point ")
	if got := NormalizeAvailabilityTarget(spelled).TargetKind; got != AvailabilityTargetAccessPoint {
		t.Fatalf("normalized kind = %q, want %q", got, AvailabilityTargetAccessPoint)
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

func TestNormalizeAvailabilityTargetBuildsCanonicalObservationLocationSet(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:                "service.local",
		Protocol:               AvailabilityProbeICMP,
		Enabled:                true,
		ObservationLocationIDs: []string{" agent:edge-b ", "local", "agent:edge-a", "agent:edge-a"},
	})
	want := []string{AvailabilityObservationLocationLocal, "agent:edge-a", "agent:edge-b"}
	if !reflect.DeepEqual(target.ObservationLocationIDs, want) {
		t.Fatalf("ObservationLocationIDs = %#v, want %#v", target.ObservationLocationIDs, want)
	}
	if target.ProbeAgentID != "" {
		t.Fatalf("ProbeAgentID = %q, multi-location targets must not claim one owner", target.ProbeAgentID)
	}
	if !target.IncludesLocalObservation() || !reflect.DeepEqual(target.AssignedProbeAgentIDs(), []string{"edge-a", "edge-b"}) {
		t.Fatalf("normalized locations = local %v agents %#v", target.IncludesLocalObservation(), target.AssignedProbeAgentIDs())
	}
}

func TestAvailabilityExecutionConfigChangedTracksObservationLocationSet(t *testing.T) {
	base := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:                "service.local",
		Protocol:               AvailabilityProbeICMP,
		Enabled:                true,
		ObservationLocationIDs: []string{AvailabilityObservationLocationLocal, "agent:edge-a"},
	})
	reordered := base
	reordered.ObservationLocationIDs = []string{"agent:edge-a", AvailabilityObservationLocationLocal}
	if AvailabilityExecutionConfigChanged(base, reordered) {
		t.Fatal("location ordering changed the execution revision")
	}
	expanded := base
	expanded.ObservationLocationIDs = append(expanded.ObservationLocationIDs, "agent:edge-b")
	if !AvailabilityExecutionConfigChanged(base, expanded) {
		t.Fatal("adding an observation location did not change the execution revision")
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

func TestAvailabilityDNSValidationAndRevision(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  "192.168.1.53",
		Protocol: AvailabilityProbeDNS,
		Enabled:  true,
		DNS: &AvailabilityDNSConfig{
			QueryName: " example.com ",
		},
	})
	if target.Port != DefaultAvailabilityDNSPort {
		t.Fatalf("default DNS port = %d, want %d", target.Port, DefaultAvailabilityDNSPort)
	}
	if target.DNS.RecordType != "A" || target.DNS.Transport != AvailabilityDNSTransportUDP {
		t.Fatalf("unexpected DNS defaults: %+v", target.DNS)
	}
	if err := target.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	next := target
	nextDNS := *target.DNS
	nextDNS.ExpectedValue = "93.184.216.34"
	next.DNS = &nextDNS
	if !AvailabilityExecutionConfigChanged(target, next) {
		t.Fatal("expected DNS expectedValue edit to trigger config revision")
	}

	missingDNS := AvailabilityTarget{
		TargetKind: AvailabilityTargetService,
		Address:    "192.168.1.53",
		Protocol:   AvailabilityProbeDNS,
		Enabled:    true,
	}
	if err := missingDNS.Validate(); err == nil {
		t.Fatal("DNS target without DNS config should fail validation")
	}

	badRecord := target
	badRecord.DNS = &AvailabilityDNSConfig{QueryName: "example.com", RecordType: "AXFR"}
	if err := badRecord.Validate(); err == nil {
		t.Fatal("unsupported DNS record type should fail validation")
	}

	wrongProtocol := AvailabilityTarget{
		TargetKind: AvailabilityTargetService,
		Address:    "192.168.1.53",
		Protocol:   AvailabilityProbeICMP,
		Enabled:    true,
		DNS:        &AvailabilityDNSConfig{QueryName: "example.com"},
	}
	if err := wrongProtocol.Validate(); err == nil {
		t.Fatal("non-DNS target with DNS config should fail validation")
	}
}

func TestAvailabilityTargetPushDefaultsValidationAndPersistence(t *testing.T) {
	token, err := GenerateAvailabilityPushToken()
	if err != nil {
		t.Fatalf("GenerateAvailabilityPushToken() error = %v", err)
	}
	if len(token) < 32 {
		t.Fatalf("expected high-entropy token length >= 32, got %d", len(token))
	}

	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Name:     "Nightly restic backup",
		Protocol: AvailabilityProbePush,
		Enabled:  true,
		Push: &AvailabilityPushConfig{
			Token:           token,
			ScheduleType:    AvailabilityPushScheduleInterval,
			IntervalSeconds: 86400,
			GraceSeconds:    600,
		},
	})
	if err := target.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error for valid interval push target: %v", err)
	}
	if target.FailureThreshold != 1 {
		t.Fatalf("FailureThreshold = %d, want 1", target.FailureThreshold)
	}
	if !reflect.DeepEqual(target.ObservationLocationIDs, []string{AvailabilityObservationLocationLocal}) {
		t.Fatalf("ObservationLocationIDs = %#v, want [pulse:local]", target.ObservationLocationIDs)
	}

	// Token rotation should not bump execution config revision.
	rotated := target
	rotatedPush := *target.Push
	rotatedPush.Token, _ = GenerateAvailabilityPushToken()
	rotated.Push = &rotatedPush
	if AvailabilityExecutionConfigChanged(target, rotated) {
		t.Fatalf("token rotation should not trigger AvailabilityExecutionConfigChanged")
	}

	// Schedule changes should bump execution config revision.
	cronTarget := target
	cronPush := *target.Push
	cronPush.ScheduleType = AvailabilityPushScheduleCron
	cronPush.CronExpression = "0 3 * * *"
	cronTarget.Push = &cronPush
	cronTarget = NormalizeAvailabilityTarget(cronTarget)
	if err := cronTarget.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error for valid cron push target: %v", err)
	}
	if !AvailabilityExecutionConfigChanged(target, cronTarget) {
		t.Fatalf("changing push schedule should trigger AvailabilityExecutionConfigChanged")
	}

	// Missing name is rejected.
	noName := target
	noName.Name = "   "
	if err := noName.Validate(); err == nil {
		t.Fatalf("expected error for push target without name")
	}

	// Invalid cron is rejected.
	badCron := cronTarget
	badCronPush := *cronTarget.Push
	badCronPush.CronExpression = "invalid cron"
	badCron.Push = &badCronPush
	if err := badCron.Validate(); err == nil {
		t.Fatalf("expected error for invalid push cron expression")
	}

	// Push config on non-push protocol is rejected.
	wrongProto := AvailabilityTarget{
		Address:  "1.1.1.1",
		Protocol: AvailabilityProbeICMP,
		Push:     target.Push,
	}
	if err := wrongProto.Validate(); err == nil {
		t.Fatalf("expected error for push config on ICMP target")
	}

	// Encrypted persistence round-trip preserves token and schedule.
	persistence := NewConfigPersistence(t.TempDir())
	if err := persistence.SaveAvailabilityTargets([]AvailabilityTarget{cronTarget}); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}
	loaded, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Push == nil || loaded[0].Push.Token != cronTarget.Push.Token || loaded[0].Push.CronExpression != "0 3 * * *" {
		t.Fatalf("loaded push target mismatch: %+v", loaded)
	}
}
