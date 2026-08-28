package cost

import "testing"

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
