package telemetry

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSchemaV17FieldNamesArePinned(t *testing.T) {
	if TelemetrySchemaVersion != 17 {
		t.Fatalf("TelemetrySchemaVersion = %d, want 17", TelemetrySchemaVersion)
	}
	want := map[string]string{
		"AIProviderClass":                                                      "ai_provider_class",
		"PulseIntelligencePatrolAutonomyLevel":                                 "pulse_intelligence_patrol_autonomy_level",
		"PulseIntelligencePatrolInputTokensBucket30d":                          "pulse_intelligence_patrol_input_tokens_bucket_30d",
		"PulseIntelligencePatrolOutputTokensBucket30d":                         "pulse_intelligence_patrol_output_tokens_bucket_30d",
		"PulseIntelligencePatrolInvestigationOutcomeFixVerified30d":            "pulse_intelligence_patrol_investigation_outcome_fix_verified_30d",
		"PulseIntelligencePatrolInvestigationOutcomeFixQueued30d":              "pulse_intelligence_patrol_investigation_outcome_fix_queued_30d",
		"PulseIntelligencePatrolInvestigationOutcomeFixExecuted30d":            "pulse_intelligence_patrol_investigation_outcome_fix_executed_30d",
		"PulseIntelligencePatrolInvestigationOutcomeFixRejected30d":            "pulse_intelligence_patrol_investigation_outcome_fix_rejected_30d",
		"PulseIntelligencePatrolInvestigationOutcomeFixFailed30d":              "pulse_intelligence_patrol_investigation_outcome_fix_failed_30d",
		"PulseIntelligencePatrolInvestigationOutcomeFixVerificationUnknown30d": "pulse_intelligence_patrol_investigation_outcome_fix_verification_unknown_30d",
		"PulseIntelligencePatrolInvestigationOutcomeResolved30d":               "pulse_intelligence_patrol_investigation_outcome_resolved_30d",
		"PulseIntelligencePatrolInvestigationOutcomeNeedsAttention30d":         "pulse_intelligence_patrol_investigation_outcome_needs_attention_30d",
		"PulseIntelligencePatrolInvestigationOutcomeCannotFix30d":              "pulse_intelligence_patrol_investigation_outcome_cannot_fix_30d",
		"PulseIntelligencePatrolInvestigationOutcomeTimedOut30d":               "pulse_intelligence_patrol_investigation_outcome_timed_out_30d",
		"PulseIntelligencePatrolInvestigationOutcomeInProgress30d":             "pulse_intelligence_patrol_investigation_outcome_in_progress_30d",
		"PulseIntelligencePatrolInvestigationOutcomeFailed30d":                 "pulse_intelligence_patrol_investigation_outcome_failed_30d",
		"PulseIntelligencePatrolInvestigationOutcomeOther30d":                  "pulse_intelligence_patrol_investigation_outcome_other_30d",
	}
	pingType := reflect.TypeOf(Ping{})
	for goName, jsonName := range want {
		field, ok := pingType.FieldByName(goName)
		if !ok {
			t.Errorf("Ping missing field %s", goName)
			continue
		}
		tag := field.Tag.Get("json")
		// The four strings are never omitted: an absent value must mean a
		// pre-v17 sender, not an install that happened to be idle.
		if tag != jsonName {
			t.Errorf("Ping.%s json tag = %q, want %q (no omitempty)", goName, tag, jsonName)
		}
		if _, ok := reflect.TypeOf(Snapshot{}).FieldByName(goName); !ok {
			t.Errorf("Snapshot missing field %s", goName)
		}
	}
	if len(PatrolInvestigationOutcomeBucketValues()) != 13 {
		t.Fatalf("outcome bucket vocabulary = %d values, want 13", len(PatrolInvestigationOutcomeBucketValues()))
	}
}

func TestBuildPingForSnapshotCarriesSchemaV17Fields(t *testing.T) {
	ping := BuildPingForSnapshot(Snapshot{
		AIProviderClass:                                                      AIProviderClassLocal,
		PulseIntelligencePatrolAutonomyLevel:                                 config.PatrolAutonomyApproval,
		PulseIntelligencePatrolInputTokensBucket30d:                          PatrolInputTokens5M20M,
		PulseIntelligencePatrolOutputTokensBucket30d:                         PatrolOutputTokens100K500K,
		PulseIntelligencePatrolInvestigationOutcomeFixVerified30d:            1,
		PulseIntelligencePatrolInvestigationOutcomeFixQueued30d:              2,
		PulseIntelligencePatrolInvestigationOutcomeFixExecuted30d:            3,
		PulseIntelligencePatrolInvestigationOutcomeFixRejected30d:            4,
		PulseIntelligencePatrolInvestigationOutcomeFixFailed30d:              5,
		PulseIntelligencePatrolInvestigationOutcomeFixVerificationUnknown30d: 6,
		PulseIntelligencePatrolInvestigationOutcomeResolved30d:               7,
		PulseIntelligencePatrolInvestigationOutcomeNeedsAttention30d:         8,
		PulseIntelligencePatrolInvestigationOutcomeCannotFix30d:              9,
		PulseIntelligencePatrolInvestigationOutcomeTimedOut30d:               10,
		PulseIntelligencePatrolInvestigationOutcomeInProgress30d:             11,
		PulseIntelligencePatrolInvestigationOutcomeFailed30d:                 12,
		PulseIntelligencePatrolInvestigationOutcomeOther30d:                  13,
	})
	if ping.AIProviderClass != AIProviderClassLocal ||
		ping.PulseIntelligencePatrolAutonomyLevel != config.PatrolAutonomyApproval ||
		ping.PulseIntelligencePatrolInputTokensBucket30d != PatrolInputTokens5M20M ||
		ping.PulseIntelligencePatrolOutputTokensBucket30d != PatrolOutputTokens100K500K {
		t.Fatalf("v17 string fields not carried: %+v", ping)
	}
	got := []int{
		ping.PulseIntelligencePatrolInvestigationOutcomeFixVerified30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeFixQueued30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeFixExecuted30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeFixRejected30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeFixFailed30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeFixVerificationUnknown30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeResolved30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeNeedsAttention30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeCannotFix30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeTimedOut30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeInProgress30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeFailed30d,
		ping.PulseIntelligencePatrolInvestigationOutcomeOther30d,
	}
	for i, value := range got {
		if value != i+1 {
			t.Fatalf("outcome field %d = %d, want %d", i, value, i+1)
		}
	}
}

func TestClassifyAIProviderClass(t *testing.T) {
	enabled := func(mutate func(*config.AIConfig)) *config.AIConfig {
		cfg := &config.AIConfig{Enabled: true}
		mutate(cfg)
		return cfg
	}
	cases := []struct {
		name string
		cfg  *config.AIConfig
		want string
	}{
		{"nil config", nil, AIProviderClassNone},
		{"ai disabled", &config.AIConfig{Enabled: false, Model: "anthropic:claude-x"}, AIProviderClassNone},
		{"no model selected", enabled(func(c *config.AIConfig) {}), AIProviderClassNone},
		{"ollama prefix", enabled(func(c *config.AIConfig) { c.Model = "ollama:qwen3:8b" }), AIProviderClassLocal},
		{"patrol model overrides default", enabled(func(c *config.AIConfig) {
			c.Model = "anthropic:claude-x"
			c.PatrolModel = "ollama:qwen3:8b"
		}), AIProviderClassLocal},
		{"anthropic key route", enabled(func(c *config.AIConfig) { c.Model = "anthropic:claude-x" }), AIProviderClassCloudBYOK},
		{"openai official", enabled(func(c *config.AIConfig) { c.Model = "openai:gpt-x" }), AIProviderClassCloudBYOK},
		{"openai official explicit base url", enabled(func(c *config.AIConfig) {
			c.Model = "openai:gpt-x"
			c.OpenAIBaseURL = "https://api.openai.com/v1"
		}), AIProviderClassCloudBYOK},
		{"openai compatible loopback", enabled(func(c *config.AIConfig) {
			c.Model = "openai:local-model"
			c.OpenAIBaseURL = "http://127.0.0.1:1234/v1"
		}), AIProviderClassLocal},
		{"openai compatible private range", enabled(func(c *config.AIConfig) {
			c.Model = "openai:local-model"
			c.OpenAIBaseURL = "http://192.168.1.20:8080/v1"
		}), AIProviderClassLocal},
		{"openai compatible mdns host", enabled(func(c *config.AIConfig) {
			c.Model = "openai:local-model"
			c.OpenAIBaseURL = "http://llm-box.local:8080/v1"
		}), AIProviderClassLocal},
		{"openai compatible unqualified host", enabled(func(c *config.AIConfig) {
			c.Model = "openai:local-model"
			c.OpenAIBaseURL = "http://gpu-node:11434/v1"
		}), AIProviderClassLocal},
		{"openai compatible public host", enabled(func(c *config.AIConfig) {
			c.Model = "openai:some-model"
			c.OpenAIBaseURL = "https://gateway.example.com/v1"
		}), AIProviderClassCloudBYOK},
		{"openrouter", enabled(func(c *config.AIConfig) { c.Model = "openrouter:meta-llama/x" }), AIProviderClassCloudBYOK},
		{"codex subscription", enabled(func(c *config.AIConfig) { c.Model = "codex-subscription:gpt-x" }), AIProviderClassCloudSubscription},
		{"claude subscription", enabled(func(c *config.AIConfig) { c.Model = "claude-subscription:claude-x" }), AIProviderClassCloudSubscription},
		{"retired hosted quickstart alias", enabled(func(c *config.AIConfig) { c.Model = config.DefaultAIModelQuickstart }), AIProviderClassHostedLegacy},
		{"retired hosted quickstart provider prefix", enabled(func(c *config.AIConfig) { c.Model = config.AIProviderQuickstart + ":pulse-hosted" }), AIProviderClassHostedLegacy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyAIProviderClass(tc.cfg); got != tc.want {
				t.Fatalf("ClassifyAIProviderClass = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyAIProviderClassNeverLeaksRouteDetail(t *testing.T) {
	cfg := &config.AIConfig{Enabled: true, Model: "openai:secret-model-name", OpenAIBaseURL: "https://corp-gateway.example.net/v1", OpenAIAPIKey: "sk-not-real"}
	got := ClassifyAIProviderClass(cfg)
	allowed := map[string]bool{}
	for _, value := range AIProviderClassValues() {
		allowed[value] = true
	}
	if !allowed[got] {
		t.Fatalf("ClassifyAIProviderClass returned %q outside the closed vocabulary", got)
	}
}

func TestPatrolTokenBuckets(t *testing.T) {
	input := []struct {
		total int64
		want  string
	}{
		{-5, PatrolTokenBucketZero}, {0, PatrolTokenBucketZero}, {1, PatrolInputTokensUnder1M}, {999_999, PatrolInputTokensUnder1M},
		{1_000_000, PatrolInputTokens1M5M}, {4_999_999, PatrolInputTokens1M5M}, {5_000_000, PatrolInputTokens5M20M},
		{19_999_999, PatrolInputTokens5M20M}, {20_000_000, PatrolInputTokens20MPlus}, {900_000_000, PatrolInputTokens20MPlus},
	}
	for _, tc := range input {
		if got := PatrolInputTokensBucket(tc.total); got != tc.want {
			t.Errorf("PatrolInputTokensBucket(%d) = %q, want %q", tc.total, got, tc.want)
		}
	}
	output := []struct {
		total int64
		want  string
	}{
		{0, PatrolTokenBucketZero}, {1, PatrolOutputTokensUnder100K}, {99_999, PatrolOutputTokensUnder100K},
		{100_000, PatrolOutputTokens100K500K}, {499_999, PatrolOutputTokens100K500K}, {500_000, PatrolOutputTokens500K2M},
		{1_999_999, PatrolOutputTokens500K2M}, {2_000_000, PatrolOutputTokens2MPlus},
	}
	for _, tc := range output {
		if got := PatrolOutputTokensBucket(tc.total); got != tc.want {
			t.Errorf("PatrolOutputTokensBucket(%d) = %q, want %q", tc.total, got, tc.want)
		}
	}
}

func TestNormalizePatrolAutonomyLevelForTelemetry(t *testing.T) {
	cases := map[string]string{
		"":           config.PatrolAutonomyMonitor,
		"monitor":    config.PatrolAutonomyMonitor,
		"approval":   config.PatrolAutonomyApproval,
		" Assisted ": config.PatrolAutonomyAssisted,
		"full":       config.PatrolAutonomyFull,
		"autonomous": config.PatrolAutonomyMonitor,
		"garbage":    config.PatrolAutonomyMonitor,
	}
	for in, want := range cases {
		if got := NormalizePatrolAutonomyLevelForTelemetry(in); got != want {
			t.Errorf("NormalizePatrolAutonomyLevelForTelemetry(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPatrolInvestigationOutcomeBucketPartitionsEveryState(t *testing.T) {
	cases := []struct {
		outcome, status, want string
	}{
		{"fix_verified", "completed", PatrolInvestigationOutcomeFixVerified},
		{"fix_queued", "completed", PatrolInvestigationOutcomeFixQueued},
		{"fix_executed", "completed", PatrolInvestigationOutcomeFixExecuted},
		{"fix_rejected", "completed", PatrolInvestigationOutcomeFixRejected},
		{"fix_failed", "completed", PatrolInvestigationOutcomeFixFailed},
		{"fix_verification_failed", "completed", PatrolInvestigationOutcomeFixFailed},
		{"fix_verification_unknown", "completed", PatrolInvestigationOutcomeFixVerificationUnknown},
		{"resolved", "completed", PatrolInvestigationOutcomeResolved},
		{"needs_attention", "needs_attention", PatrolInvestigationOutcomeNeedsAttention},
		{"cannot_fix", "completed", PatrolInvestigationOutcomeCannotFix},
		{"timed_out", "failed", PatrolInvestigationOutcomeTimedOut},
		{"", "pending", PatrolInvestigationOutcomeInProgress},
		{"", "running", PatrolInvestigationOutcomeInProgress},
		{"", "failed", PatrolInvestigationOutcomeFailed},
		{"", "needs_attention", PatrolInvestigationOutcomeNeedsAttention},
		{"", "completed", PatrolInvestigationOutcomeOther},
		{"", "", PatrolInvestigationOutcomeOther},
		{"something_new", "completed", PatrolInvestigationOutcomeOther},
	}
	var counts PatrolInvestigationOutcomeCounts
	for _, tc := range cases {
		if got := PatrolInvestigationOutcomeBucket(tc.outcome, tc.status); got != tc.want {
			t.Errorf("bucket(%q, %q) = %q, want %q", tc.outcome, tc.status, got, tc.want)
		}
		counts.Add(tc.outcome, tc.status)
	}
	if counts.Total() != len(cases) {
		t.Fatalf("outcome counts total = %d, want %d (every investigated finding must land in exactly one bucket)", counts.Total(), len(cases))
	}
	if counts.FixFailed != 2 || counts.InProgress != 2 || counts.Other != 3 || counts.NeedsAttention != 2 {
		t.Fatalf("unexpected partition: %+v", counts)
	}
}
