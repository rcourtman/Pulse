package cost

import (
	"math"
	"testing"
)

func TestLookupPriceUsesCurrentAnthropicOpusPricing(t *testing.T) {
	tests := []struct {
		model  string
		input  float64
		output float64
		asOf   string
	}{
		{model: "claude-opus-5", input: 5.00, output: 25.00, asOf: "2026-08-28"},
		{model: "claude-opus-5-20260801", input: 5.00, output: 25.00, asOf: "2026-08-28"},
		{model: "claude-opus-4-8", input: 5.00, output: 25.00, asOf: "2026-08-28"},
		{model: "claude-opus-4-7", input: 5.00, output: 25.00, asOf: "2026-08-28"},
		{model: "claude-opus-4-6", input: 5.00, output: 25.00, asOf: "2026-08-28"},
		{model: "claude-opus-4-5-20251101", input: 5.00, output: 25.00, asOf: "2026-08-28"},
		{model: "claude-opus-4-1-20250805", input: 15.00, output: 75.00, asOf: pricingAsOf},
		{model: "claude-opus-4-20250514", input: 15.00, output: 75.00, asOf: pricingAsOf},
		{model: "claude-opus-20240229", input: 15.00, output: 75.00, asOf: pricingAsOf},
	}

	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			price, ok := lookupPrice("anthropic", test.model, 50_000)
			if !ok {
				t.Fatal("expected Anthropic Opus pricing to resolve")
			}
			if price.InputUSDPerMTok != test.input || price.OutputUSDPerMTok != test.output {
				t.Fatalf("unexpected Anthropic Opus price: %+v", price)
			}
			if price.AsOf != test.asOf {
				t.Fatalf("Anthropic Opus pricing date = %q, want %q", price.AsOf, test.asOf)
			}
		})
	}
}

func TestLookupPriceUsesCurrentAnthropicHaiku45Pricing(t *testing.T) {
	price, ok := lookupPrice("anthropic", "claude-haiku-4-5-20251001", 50_000)
	if !ok {
		t.Fatal("expected Anthropic Haiku 4.5 pricing to resolve")
	}
	if price.InputUSDPerMTok != 1.00 || price.OutputUSDPerMTok != 5.00 {
		t.Fatalf("unexpected Anthropic Haiku 4.5 price: %+v", price)
	}
	if price.AsOf != "2026-08-28" {
		t.Fatalf("Anthropic Haiku 4.5 pricing date = %q, want 2026-08-28", price.AsOf)
	}
}

func TestLookupPriceUsesCurrentAnthropicGeneration5Pricing(t *testing.T) {
	tests := []struct {
		model  string
		input  float64
		output float64
	}{
		{model: "claude-fable-5", input: 10.00, output: 50.00},
		{model: "claude-fable-5-20260609", input: 10.00, output: 50.00},
		{model: "claude-mythos-5", input: 10.00, output: 50.00},
		{model: "claude-mythos-5-20260609", input: 10.00, output: 50.00},
		{model: "claude-sonnet-5", input: 2.00, output: 10.00},
		{model: "claude-sonnet-5-20260630", input: 2.00, output: 10.00},
	}

	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			price, ok := lookupPrice("anthropic", test.model, 50_000)
			if !ok {
				t.Fatal("expected current Anthropic generation 5 pricing to resolve")
			}
			if price.InputUSDPerMTok != test.input || price.OutputUSDPerMTok != test.output {
				t.Fatalf("unexpected Anthropic generation 5 price: %+v", price)
			}
			if price.AsOf != "2026-08-29" {
				t.Fatalf("Anthropic generation 5 pricing date = %q, want 2026-08-29", price.AsOf)
			}
		})
	}
}

func TestEstimateUsageUSDPricesAnthropicCacheBucketsAtCacheRates(t *testing.T) {
	usage := TokenUsage{InputTokens: 100_000, OutputTokens: 10_000, CacheCreationInputTokens: 200_000, CacheReadInputTokens: 1_000_000}
	usd, ok, price := EstimateUsageUSD("anthropic", "claude-sonnet-5", usage)
	if !ok {
		t.Fatal("expected known pricing for claude-sonnet-5")
	}
	// Sonnet 5 input is 2.00/M, so a 5-minute cache write is 2.50/M and a
	// cache read 0.20/M.
	if price.CacheWriteUSDPerMTok != 2.5 || price.CacheReadUSDPerMTok != 0.2 {
		t.Fatalf("unexpected cache rates: %+v", price)
	}
	want := 0.1*2.00 + 0.01*10.00 + 0.2*2.50 + 1.0*0.20
	if math.Abs(usd-want) > 1e-9 {
		t.Fatalf("usd = %f, want %f", usd, want)
	}
	// Charging the whole prompt at the input rate overstates a cached run.
	flat, _, _ := EstimateUSD("anthropic", "claude-sonnet-5", usage.PromptTokens(), usage.OutputTokens)
	if flat <= usd {
		t.Fatalf("flat input-rate estimate %f should exceed cache-aware %f", flat, usd)
	}
}

func TestEstimateUsageUSDFallsBackToInputRateWithoutCachePrices(t *testing.T) {
	usage := TokenUsage{InputTokens: 100_000, CacheReadInputTokens: 100_000}
	usd, ok, price := EstimateUsageUSD("openai", "gpt-4o-mini", usage)
	if !ok {
		t.Fatal("expected known pricing for gpt-4o-mini")
	}
	if price.CacheWriteUSDPerMTok != 0 || price.CacheReadUSDPerMTok != 0 {
		t.Fatalf("openai rows carry no cache prices: %+v", price)
	}
	// Both buckets at 0.15/M: conservative, never below the provider's bill.
	if want := 0.2 * 0.15; math.Abs(usd-want) > 1e-9 {
		t.Fatalf("usd = %f, want %f", usd, want)
	}
	// Without cache buckets the two estimators agree exactly.
	a, _, _ := EstimateUSD("openai", "gpt-4o-mini", 100_000, 2_000)
	b, _, _ := EstimateUsageUSD("openai", "gpt-4o-mini", TokenUsage{InputTokens: 100_000, OutputTokens: 2_000})
	if a != b {
		t.Fatalf("EstimateUSD %f != EstimateUsageUSD %f", a, b)
	}
}
