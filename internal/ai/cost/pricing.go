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
	price, ok = lookupPrice(provider, model)
	if !ok {
		return 0, false, TokenPrice{}
	}

	usd = (float64(inputTokens)/1_000_000.0)*price.InputUSDPerMTok +
		(float64(outputTokens)/1_000_000.0)*price.OutputUSDPerMTok
	return usd, true, price
}

type modelPrice struct {
	Pattern          string
	InputUSDPerMTok  float64
	OutputUSDPerMTok float64
}

const pricingAsOf = "2025-12"

// PricingAsOf indicates the effective date of the pricing table used for estimation.
func PricingAsOf() string {
	return pricingAsOf
}

// NOTE: Keep this table small and conservative.
// The goal is quick estimation and relative comparisons, not exact billing.
var providerPrices = map[string][]modelPrice{
	"openai": {
		{Pattern: "gpt-4o*", InputUSDPerMTok: 5.00, OutputUSDPerMTok: 15.00},
		{Pattern: "gpt-4o-mini*", InputUSDPerMTok: 0.15, OutputUSDPerMTok: 0.60},
	},
	"anthropic": {
		{Pattern: "claude-opus*", InputUSDPerMTok: 15.00, OutputUSDPerMTok: 75.00},
		{Pattern: "claude-sonnet*", InputUSDPerMTok: 3.00, OutputUSDPerMTok: 15.00},
		{Pattern: "claude-haiku*", InputUSDPerMTok: 0.25, OutputUSDPerMTok: 1.25},
	},
	"deepseek": {
		// DeepSeek docs include an "input cache hit" discount; this uses cache-miss rates for conservative estimates.
		{Pattern: "deepseek-*", InputUSDPerMTok: 0.28, OutputUSDPerMTok: 0.42},
	},
	"ollama": {
		{Pattern: "*", InputUSDPerMTok: 0, OutputUSDPerMTok: 0},
	},
}

func lookupPrice(provider, model string) (TokenPrice, bool) {
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
			return TokenPrice{
				InputUSDPerMTok:  p.InputUSDPerMTok,
				OutputUSDPerMTok: p.OutputUSDPerMTok,
				AsOf:             pricingAsOf,
			}, true
		}
	}
	return TokenPrice{}, false
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
