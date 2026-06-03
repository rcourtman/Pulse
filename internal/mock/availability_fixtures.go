package mock

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	mockAvailabilityProbeICMP = "icmp"
	mockAvailabilityProbeTCP  = "tcp"
	mockAvailabilityProbeHTTP = "http"

	mockAvailabilityTargetMachine = "machine"
	mockAvailabilityTargetService = "service"
	mockAvailabilityTargetDevice  = "device"

	mockAvailabilityDefaultPollIntervalSecs = 60
	mockAvailabilityDefaultTimeoutMillis    = 2000
	mockAvailabilityDefaultFailureThreshold = 2
)

// AvailabilityTargetFixture is the config-free fixture contract for an
// agentless availability endpoint. The API layer converts it into
// config.AvailabilityTarget when serving configuration-shaped responses.
type AvailabilityTargetFixture struct {
	ID               string
	Name             string
	TargetKind       string
	Address          string
	Protocol         string
	Port             int
	Path             string
	Enabled          bool
	PollIntervalSecs int
	TimeoutMillis    int
	FailureThreshold int
}

// AvailabilityFixture describes an agentless mock endpoint that represents
// infrastructure Pulse cannot manage through an API or installed agent.
type AvailabilityFixture struct {
	Target              AvailabilityTargetFixture
	Available           bool
	LastChecked         time.Time
	LastSuccess         time.Time
	LatencyMillis       int64
	ConsecutiveFailures int
	LastError           string
}

func AvailabilityFixtures() []AvailabilityFixture {
	if !IsMockEnabled() {
		return nil
	}
	return CurrentFixtureGraph().AvailabilityFixtures
}

func normalizeAvailabilityTargetFixture(target AvailabilityTargetFixture) AvailabilityTargetFixture {
	target.ID = strings.TrimSpace(target.ID)
	target.Name = strings.TrimSpace(target.Name)
	target.TargetKind = strings.ToLower(strings.TrimSpace(target.TargetKind))
	if target.TargetKind == "" {
		target.TargetKind = mockAvailabilityTargetService
	}
	target.Protocol = strings.ToLower(strings.TrimSpace(target.Protocol))
	if target.Protocol == "" {
		target.Protocol = mockAvailabilityProbeICMP
	}
	if target.Protocol == mockAvailabilityProbeHTTP {
		target.Address = strings.TrimSpace(target.Address)
	} else {
		target.Address = normalizeAvailabilityFixtureAddress(target.Address)
	}
	target.Path = strings.TrimSpace(target.Path)
	if target.PollIntervalSecs <= 0 {
		target.PollIntervalSecs = mockAvailabilityDefaultPollIntervalSecs
	}
	if target.TimeoutMillis <= 0 {
		target.TimeoutMillis = mockAvailabilityDefaultTimeoutMillis
	}
	if target.FailureThreshold <= 0 {
		target.FailureThreshold = mockAvailabilityDefaultFailureThreshold
	}
	return target
}

func (t AvailabilityTargetFixture) displayName() string {
	if name := strings.TrimSpace(t.Name); name != "" {
		return name
	}
	return strings.TrimSpace(t.Address)
}

func (t AvailabilityTargetFixture) probeAddress() string {
	return normalizeAvailabilityFixtureAddress(t.Address)
}

func (t AvailabilityTargetFixture) effectivePollIntervalSecs() int {
	if t.PollIntervalSecs > 0 {
		return t.PollIntervalSecs
	}
	return mockAvailabilityDefaultPollIntervalSecs
}

func (t AvailabilityTargetFixture) effectiveTimeoutMillis() int {
	if t.TimeoutMillis > 0 {
		return t.TimeoutMillis
	}
	return mockAvailabilityDefaultTimeoutMillis
}

func (t AvailabilityTargetFixture) effectiveFailureThreshold() int {
	if t.FailureThreshold > 0 {
		return t.FailureThreshold
	}
	return mockAvailabilityDefaultFailureThreshold
}

func normalizeAvailabilityFixtureAddress(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		if u, err := url.Parse(value); err == nil && strings.TrimSpace(u.Hostname()) != "" {
			return strings.TrimSpace(u.Hostname())
		}
	}
	if host, _, err := net.SplitHostPort(value); err == nil && strings.TrimSpace(host) != "" {
		return strings.Trim(strings.TrimSpace(host), "[]")
	}
	return strings.Trim(value, "[]")
}

func defaultAvailabilityFixtures(now time.Time) []AvailabilityFixture {
	base := normalizeAvailabilityFixtureTime(now)
	return []AvailabilityFixture{
		{
			Target: normalizeAvailabilityTargetFixture(AvailabilityTargetFixture{
				ID:               "mock-availability-ups",
				Name:             "Rack UPS network card",
				TargetKind:       mockAvailabilityTargetDevice,
				Address:          "ups-rack-a.lab.local",
				Protocol:         mockAvailabilityProbeICMP,
				Enabled:          true,
				PollIntervalSecs: 30,
				TimeoutMillis:    1000,
				FailureThreshold: 2,
			}),
			Available:     true,
			LastChecked:   base.Add(-8 * time.Second),
			LastSuccess:   base.Add(-8 * time.Second),
			LatencyMillis: 3,
		},
		{
			Target: normalizeAvailabilityTargetFixture(AvailabilityTargetFixture{
				ID:               "mock-availability-mqtt-meter",
				Name:             "MQTT power meter",
				TargetKind:       mockAvailabilityTargetDevice,
				Address:          "power-meter-01.lab.local",
				Protocol:         mockAvailabilityProbeTCP,
				Port:             1883,
				Enabled:          true,
				PollIntervalSecs: 30,
				TimeoutMillis:    1500,
				FailureThreshold: 2,
			}),
			Available:     true,
			LastChecked:   base.Add(-14 * time.Second),
			LastSuccess:   base.Add(-14 * time.Second),
			LatencyMillis: 7,
		},
		{
			Target: normalizeAvailabilityTargetFixture(AvailabilityTargetFixture{
				ID:               "mock-availability-esphome-greenhouse",
				Name:             "ESPHome greenhouse sensor",
				TargetKind:       mockAvailabilityTargetDevice,
				Address:          "greenhouse-sensor.lab.local",
				Protocol:         mockAvailabilityProbeTCP,
				Port:             6053,
				Enabled:          true,
				PollIntervalSecs: 30,
				TimeoutMillis:    1500,
				FailureThreshold: 2,
			}),
			Available:     true,
			LastChecked:   base.Add(-5 * time.Second),
			LastSuccess:   base.Add(-5 * time.Second),
			LatencyMillis: 11,
		},
		{
			Target: normalizeAvailabilityTargetFixture(AvailabilityTargetFixture{
				ID:               "mock-availability-solar-inverter",
				Name:             "Solar inverter web panel",
				TargetKind:       mockAvailabilityTargetDevice,
				Address:          "http://solar-inverter.lab.local/",
				Protocol:         mockAvailabilityProbeHTTP,
				Path:             "/status",
				Enabled:          true,
				PollIntervalSecs: 60,
				TimeoutMillis:    2000,
				FailureThreshold: 2,
			}),
			Available:           false,
			LastChecked:         base.Add(-18 * time.Second),
			LastSuccess:         base.Add(-3 * time.Minute),
			ConsecutiveFailures: 1,
			LastError:           "http probe returned 503 Service Unavailable",
		},
		{
			Target: normalizeAvailabilityTargetFixture(AvailabilityTargetFixture{
				ID:               "mock-availability-door-controller",
				Name:             "Workshop door controller",
				TargetKind:       mockAvailabilityTargetDevice,
				Address:          "10.24.40.45",
				Protocol:         mockAvailabilityProbeICMP,
				Enabled:          true,
				PollIntervalSecs: 30,
				TimeoutMillis:    1000,
				FailureThreshold: 2,
			}),
			Available:           false,
			LastChecked:         base.Add(-11 * time.Second),
			LastSuccess:         base.Add(-9 * time.Minute),
			ConsecutiveFailures: 3,
			LastError:           "icmp probe timed out",
		},
	}
}

func cloneAvailabilityFixtures(in []AvailabilityFixture) []AvailabilityFixture {
	if in == nil {
		return nil
	}
	out := make([]AvailabilityFixture, len(in))
	copy(out, in)
	return out
}

func rebaseAvailabilityFixtures(fixtures []AvailabilityFixture, now time.Time) []AvailabilityFixture {
	target := normalizeAvailabilityFixtureTime(now)
	if len(fixtures) == 0 {
		return defaultAvailabilityFixtures(target)
	}
	out := cloneAvailabilityFixtures(fixtures)
	anchor := availabilityFixturesFreshness(fixtures)
	if anchor.IsZero() {
		anchor = target
	}
	shift := target.Sub(anchor)
	for i := range out {
		out[i].Target = normalizeAvailabilityTargetFixture(out[i].Target)
		out[i].LastChecked = shiftTime(fixtures[i].LastChecked, shift, target)
		out[i].LastSuccess = shiftTime(fixtures[i].LastSuccess, shift, target)
	}
	return out
}

func availabilityFixtureRecords(fixtures []AvailabilityFixture, now time.Time) []unifiedresources.IngestRecord {
	if len(fixtures) == 0 {
		return nil
	}
	target := normalizeAvailabilityFixtureTime(now)
	out := make([]unifiedresources.IngestRecord, 0, len(fixtures))
	for _, fixture := range fixtures {
		record, ok := availabilityFixtureRecord(fixture, target)
		if ok {
			out = append(out, record)
		}
	}
	return out
}

func availabilityFixtureRecord(fixture AvailabilityFixture, now time.Time) (unifiedresources.IngestRecord, bool) {
	target := normalizeAvailabilityTargetFixture(fixture.Target)
	if strings.TrimSpace(target.ID) == "" {
		return unifiedresources.IngestRecord{}, false
	}
	lastSeen := fixture.LastChecked
	if lastSeen.IsZero() {
		lastSeen = now
	}
	data := &unifiedresources.AvailabilityData{
		TargetID:            target.ID,
		Name:                target.displayName(),
		TargetKind:          target.TargetKind,
		Address:             target.Address,
		Protocol:            string(target.Protocol),
		Port:                target.Port,
		Path:                target.Path,
		Enabled:             target.Enabled,
		Available:           fixture.Available,
		LastChecked:         fixture.LastChecked,
		LastSuccess:         fixture.LastSuccess,
		LatencyMillis:       fixture.LatencyMillis,
		ConsecutiveFailures: fixture.ConsecutiveFailures,
		LastError:           fixture.LastError,
		FailureThreshold:    target.effectiveFailureThreshold(),
		PollIntervalSeconds: target.effectivePollIntervalSecs(),
		TimeoutMillis:       target.effectiveTimeoutMillis(),
	}
	resource := unifiedresources.Resource{
		Type:         unifiedresources.ResourceTypeNetworkEndpoint,
		Technology:   string(target.Protocol),
		Name:         target.displayName(),
		Status:       availabilityFixtureResourceStatus(target, fixture),
		LastSeen:     lastSeen,
		UpdatedAt:    now,
		Sources:      []unifiedresources.DataSource{unifiedresources.SourceAvailability},
		Tags:         availabilityFixtureTags(target),
		Availability: data,
	}
	if incident := availabilityFixtureIncident(target, fixture, lastSeen); incident != nil {
		resource.Incidents = []unifiedresources.ResourceIncident{*incident}
	}

	return unifiedresources.IngestRecord{
		SourceID: target.ID,
		Resource: resource,
		Identity: availabilityFixtureIdentity(target),
	}, true
}

func availabilityFixtureResourceStatus(target AvailabilityTargetFixture, fixture AvailabilityFixture) unifiedresources.ResourceStatus {
	if !target.Enabled {
		return unifiedresources.StatusUnknown
	}
	if fixture.LastChecked.IsZero() {
		return unifiedresources.StatusUnknown
	}
	if fixture.Available {
		return unifiedresources.StatusOnline
	}
	if fixture.ConsecutiveFailures >= target.effectiveFailureThreshold() {
		return unifiedresources.StatusOffline
	}
	return unifiedresources.StatusWarning
}

func availabilityFixtureIncident(target AvailabilityTargetFixture, fixture AvailabilityFixture, startedAt time.Time) *unifiedresources.ResourceIncident {
	if !target.Enabled || fixture.Available || fixture.LastChecked.IsZero() {
		return nil
	}
	if fixture.ConsecutiveFailures < target.effectiveFailureThreshold() {
		return nil
	}
	summary := fmt.Sprintf("%s is unreachable by %s probe", target.displayName(), strings.ToUpper(target.Protocol))
	if strings.TrimSpace(fixture.LastError) != "" {
		summary += ": " + fixture.LastError
	}
	return &unifiedresources.ResourceIncident{
		Provider:  string(unifiedresources.SourceAvailability),
		NativeID:  target.ID,
		Code:      "availability_unreachable",
		Severity:  storagehealth.RiskCritical,
		Source:    string(unifiedresources.SourceAvailability),
		Summary:   summary,
		StartedAt: startedAt,
	}
}

func availabilityFixtureIdentity(target AvailabilityTargetFixture) unifiedresources.ResourceIdentity {
	identity := unifiedresources.ResourceIdentity{}
	if ip := net.ParseIP(target.probeAddress()); ip != nil {
		identity.IPAddresses = []string{ip.String()}
		return identity
	}
	if host := target.probeAddress(); host != "" {
		identity.Hostnames = []string{host}
	}
	return identity
}

func availabilityFixtureTags(target AvailabilityTargetFixture) []string {
	tags := []string{"agentless", "no-agent"}
	if target.TargetKind != "" {
		tags = append(tags, target.TargetKind)
	}
	switch target.Protocol {
	case mockAvailabilityProbeHTTP:
		tags = append(tags, "web-interface")
	case mockAvailabilityProbeTCP:
		if target.Port == 1883 {
			tags = append(tags, "mqtt")
		}
		if target.Port == 6053 || strings.Contains(strings.ToLower(target.displayName()), "esphome") {
			tags = append(tags, "esphome")
		}
	}
	return tags
}

func availabilityFixturesFreshness(fixtures []AvailabilityFixture) time.Time {
	var freshness time.Time
	for _, fixture := range fixtures {
		for _, candidate := range []time.Time{fixture.LastChecked, fixture.LastSuccess} {
			if candidate.IsZero() {
				continue
			}
			if freshness.IsZero() || candidate.After(freshness) {
				freshness = candidate
			}
		}
	}
	return freshness
}

func normalizeAvailabilityFixtureTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}
