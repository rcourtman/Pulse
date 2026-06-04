package cost

import "strings"

// TokenPrice represents a price per million tokens for a model.
// Prices are estimates intended for cross-provider budgeting, not billing reconciliation.
type TokenPrice struct {
	InputUSDPerMTok  float64
	OutputUSDPerMTok float64
	AsOf             string
}

// EstimateUSD returns an estimated USD cost for the given provider/model and token counts.
// If the model pricing is unknown, ok is false and usd is 0.
func EstimateUSD(provider, model string, inputTokens, outputTokens int64) (usd float64, ok bool, price TokenPrice) {
	price, ok = lookupPrice(provider, model, inputTokens)
	if !ok {
		return 0, false, TokenPrice{}
	}

	usd = (float64(inputTokens)/1_000_000.0)*price.InputUSDPerMTok +
		(float64(outputTokens)/1_000_000.0)*price.OutputUSDPerMTok
	return usd, true, price
}

type modelPrice struct {
	Pattern string
	Tiers   []priceTier
}

type priceTier struct {
	// MaxInputTokens is inclusive. Zero means no upper bound.
	MaxInputTokens   int64
	InputUSDPerMTok  float64
	OutputUSDPerMTok float64
}

const pricingAsOf = "2026-06-04"

// PricingAsOf indicates the effective date of the pricing table used for estimation.
func PricingAsOf() string {
	return pricingAsOf
}

// NOTE: Keep this table small and conservative.
// The goal is quick estimation and relative comparisons, not exact billing.
var providerPrices = map[string][]modelPrice{
	"openai": {
		flatPrice("gpt-4o-mini*", 0.15, 0.60),
		flatPrice("gpt-4o*", 5.00, 15.00),
	},
	"anthropic": {
		flatPrice("claude-opus*", 15.00, 75.00),
		flatPrice("claude-sonnet*", 3.00, 15.00),
		flatPrice("claude-haiku*", 0.25, 1.25),
	},
	"deepseek": {
		// DeepSeek docs include an input cache-hit discount; this uses cache-miss rates for conservative estimates.
		flatPrice("deepseek-v4-flash*", 0.14, 0.28),
		flatPrice("deepseek-v4-pro*", 0.435, 0.87),
		flatPrice("deepseek-*", 0.14, 0.28),
	},
	"gemini": {
		// Gemini Developer API standard paid-tier pricing, checked from
		// https://ai.google.dev/gemini-api/docs/pricing on 2026-06-04.
		flatPrice("gemini-3.5-flash*", 1.50, 9.00),
		tieredPrice("gemini-3.1-pro-preview*", priceTier{MaxInputTokens: 200_000, InputUSDPerMTok: 2.00, OutputUSDPerMTok: 12.00}, priceTier{InputUSDPerMTok: 4.00, OutputUSDPerMTok: 18.00}),
		tieredPrice("gemini-3.1-pro*", priceTier{MaxInputTokens: 200_000, InputUSDPerMTok: 2.00, OutputUSDPerMTok: 12.00}, priceTier{InputUSDPerMTok: 4.00, OutputUSDPerMTok: 18.00}),
		flatPrice("gemini-3.1-flash-live-preview*", 0.75, 4.50),
		flatPrice("gemini-3.1-flash-image*", 0.50, 3.00),
		flatPrice("gemini-3.1-flash-lite*", 0.25, 1.50),
		flatPrice("gemini-3-pro-image*", 2.00, 12.00),
		tieredPrice("gemini-3-pro-preview*", priceTier{MaxInputTokens: 200_000, InputUSDPerMTok: 2.00, OutputUSDPerMTok: 12.00}, priceTier{InputUSDPerMTok: 4.00, OutputUSDPerMTok: 18.00}),
		tieredPrice("gemini-3-pro*", priceTier{MaxInputTokens: 200_000, InputUSDPerMTok: 2.00, OutputUSDPerMTok: 12.00}, priceTier{InputUSDPerMTok: 4.00, OutputUSDPerMTok: 18.00}),
		flatPrice("gemini-3-flash-preview*", 0.50, 3.00),
		flatPrice("gemini-3-flash*", 0.50, 3.00),
		tieredPrice("gemini-2.5-pro*", priceTier{MaxInputTokens: 200_000, InputUSDPerMTok: 1.25, OutputUSDPerMTok: 10.00}, priceTier{InputUSDPerMTok: 2.50, OutputUSDPerMTok: 15.00}),
		flatPrice("gemini-2.5-flash-lite-preview*", 0.10, 0.40),
		flatPrice("gemini-2.5-flash-lite*", 0.10, 0.40),
		flatPrice("gemini-2.5-flash*", 0.30, 2.50),
		flatPrice("gemini-2.0-flash-lite*", 0.075, 0.30),
		flatPrice("gemini-2.0-flash*", 0.10, 0.40),
		flatPrice("gemini-1.5-pro*", 1.25, 5.00),
		flatPrice("gemini-1.5-flash*", 0.075, 0.30),
		flatPrice("gemini-*", 0.30, 2.50), // Default to current Flash pricing.
	},
	"ollama": {
		flatPrice("*", 0, 0),
	},
}

func flatPrice(pattern string, inputUSDPerMTok, outputUSDPerMTok float64) modelPrice {
	return modelPrice{
		Pattern: pattern,
		Tiers: []priceTier{{
			InputUSDPerMTok:  inputUSDPerMTok,
			OutputUSDPerMTok: outputUSDPerMTok,
		}},
	}
}

func tieredPrice(pattern string, tiers ...priceTier) modelPrice {
	return modelPrice{
		Pattern: pattern,
		Tiers:   tiers,
	}
}

func lookupPrice(provider, model string, inputTokens int64) (TokenPrice, bool) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.ToLower(strings.TrimSpace(model))
	if provider == "" || model == "" {
		return TokenPrice{}, false
	}

	prices, ok := providerPrices[provider]
	if !ok {
		return TokenPrice{}, false
	}

	for _, p := range prices {
		if matchPattern(model, strings.ToLower(p.Pattern)) {
			tier, ok := selectPriceTier(p.Tiers, inputTokens)
			if !ok {
				return TokenPrice{}, false
			}
			return TokenPrice{
				InputUSDPerMTok:  tier.InputUSDPerMTok,
				OutputUSDPerMTok: tier.OutputUSDPerMTok,
				AsOf:             pricingAsOf,
			}, true
		}
	}
	return TokenPrice{}, false
}

func selectPriceTier(tiers []priceTier, inputTokens int64) (priceTier, bool) {
	if len(tiers) == 0 {
		return priceTier{}, false
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	for _, tier := range tiers {
		if tier.MaxInputTokens == 0 || inputTokens <= tier.MaxInputTokens {
			return tier, true
		}
	}
	return tiers[len(tiers)-1], true
}

func matchPattern(model, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(model, strings.TrimSuffix(pattern, "*"))
	}
	return model == pattern
}
