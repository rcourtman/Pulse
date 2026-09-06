package cost

import (
	"math"
	"testing"
)

func TestGemini38FlashReviewedRoutePricing(t *testing.T) {
	for _, route := range []struct{ provider, model string }{
		{"gemini", "gemini-3.8-flash"},
		{"openrouter", "google/gemini-3.8-flash"},
	} {
		t.Run(route.provider, func(t *testing.T) {
			provider, model := ResolveProviderAndModel(route.provider, route.provider+":"+route.model, "gemini-3.8-flash")
			if provider != route.provider || model != route.model {
				t.Fatalf("usage lost the requested billing route: %s:%s", provider, model)
			}
			// Token counts from the live unhealthy-container qualification.
			usd, known, price := EstimateUSD(provider, model, 16068, 778)
			if !known || math.Abs(usd-0.0149685) > 1e-10 {
				t.Fatalf("live usage estimate = %f, known=%t", usd, known)
			}
			if price.InputUSDPerMTok != 0.75 || price.OutputUSDPerMTok != 3.75 || price.AsOf != "2026-09-06" {
				t.Fatalf("reviewed standard rates/date missing: %+v", price)
			}
			usd, known, _ = EstimateUSD(provider, model, 0, 0)
			if !known || usd != 0 {
				t.Fatalf("zero observed usage = %f, known=%t", usd, known)
			}
		})
	}
}

func TestGemini38OpenRouterPricingDoesNotGuessVariantRates(t *testing.T) {
	for _, model := range []string{
		"google/gemini-3.8-flash:batch",
		"google/gemini-3.8-flash:free",
		"google/gemini-3.8-flash-preview",
		"google/gemini-3.8-flash-cyber",
		"google/gemini-3.9-flash",
	} {
		if usd, known, _ := EstimateUSD("openrouter", model, 16068, 778); known || usd != 0 {
			t.Errorf("unreviewed route %q received an estimate: %f, known=%t", model, usd, known)
		}
	}
}
