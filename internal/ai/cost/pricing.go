package cost

import "strings"

// TokenPrice represents a price per million tokens for a model.
// Prices are estimates intended for cross-provider budgeting, not billing reconciliation.
type TokenPrice struct {
	InputUSDPerMTok  float64
	OutputUSDPerMTok float64
	// CacheWriteUSDPerMTok and CacheReadUSDPerMTok price the prompt-cache
	// buckets a provider reports beside ordinary input tokens. Zero means this
	// table has no separate cache price for the provider, and the bucket is
	// charged at the ordinary input rate, the conservative choice.
	CacheWriteUSDPerMTok float64
	CacheReadUSDPerMTok  float64
	AsOf                 string
}

// TokenUsage is one call's token counts by billing bucket. InputTokens is the
// provider's ordinary, uncached input count; the cache buckets are disjoint
// from it, so PromptTokens is their sum.
type TokenUsage struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
}

// PromptTokens is every token sent to the model, cached or not.
func (u TokenUsage) PromptTokens() int64 {
	return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// Anthropic prices prompt-cache writes and reads as multiples of the model's
// input rate: a 5-minute cache write costs 1.25x and a cache read 0.1x
// (https://platform.claude.com/docs/en/about-claude/pricing, read 2026-10-03).
// Pulse only requests the 5-minute cache, so the 1-hour write rate is never
// charged.
const (
	anthropicCacheWriteMultiplier = 1.25
	anthropicCacheReadMultiplier  = 0.10
)

// EstimateUSD returns an estimated USD cost for the given provider/model and token counts.
// If the model pricing is unknown, ok is false and usd is 0.
func EstimateUSD(provider, model string, inputTokens, outputTokens int64) (usd float64, ok bool, price TokenPrice) {
	return EstimateUsageUSD(provider, model, TokenUsage{InputTokens: inputTokens, OutputTokens: outputTokens})
}

// EstimateUsageUSD prices every bucket of a call's usage: ordinary input and
// output at the model's rates, and prompt-cache writes and reads at the
// provider's cache rates when the table knows them, otherwise at the input
// rate. Tier selection uses the whole prompt, cached or not, because that is
// the context size providers tier on.
func EstimateUsageUSD(provider, model string, usage TokenUsage) (usd float64, ok bool, price TokenPrice) {
	price, ok = lookupPrice(provider, model, usage.PromptTokens())
	if !ok {
		return 0, false, TokenPrice{}
	}
	cacheWrite := price.CacheWriteUSDPerMTok
	if cacheWrite <= 0 {
		cacheWrite = price.InputUSDPerMTok
	}
	cacheRead := price.CacheReadUSDPerMTok
	if cacheRead <= 0 {
		cacheRead = price.InputUSDPerMTok
	}
	usd = (float64(usage.InputTokens)/1_000_000.0)*price.InputUSDPerMTok +
		(float64(usage.OutputTokens)/1_000_000.0)*price.OutputUSDPerMTok +
		(float64(usage.CacheCreationInputTokens)/1_000_000.0)*cacheWrite +
		(float64(usage.CacheReadInputTokens)/1_000_000.0)*cacheRead
	return usd, true, price
}

type modelPrice struct {
	Pattern string
	Tiers   []priceTier
	AsOf    string
}

type priceTier struct {
	// MaxInputTokens is inclusive. Zero means no upper bound.
	MaxInputTokens   int64
	InputUSDPerMTok  float64
	OutputUSDPerMTok float64
}

const pricingAsOf = "2026-08-29"

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
		// Anthropic first-party standard API prices, checked from
		// https://platform.claude.com/docs/en/about-claude/pricing on 2026-08-29.
		// Keep version-specific rows before the legacy family fallbacks because
		// lookupPrice returns the first matching prefix.
		flatPriceAsOf("claude-fable-5*", 10.00, 50.00, "2026-08-29"),
		flatPriceAsOf("claude-mythos-5*", 10.00, 50.00, "2026-08-29"),
		flatPriceAsOf("claude-opus-5*", 5.00, 25.00, "2026-08-28"),
		flatPriceAsOf("claude-opus-4-8*", 5.00, 25.00, "2026-08-28"),
		flatPriceAsOf("claude-opus-4-7*", 5.00, 25.00, "2026-08-28"),
		flatPriceAsOf("claude-opus-4-6*", 5.00, 25.00, "2026-08-28"),
		flatPriceAsOf("claude-opus-4-5*", 5.00, 25.00, "2026-08-28"),
		flatPrice("claude-opus*", 15.00, 75.00),
		flatPriceAsOf("claude-sonnet-5*", 2.00, 10.00, "2026-08-29"),
		flatPrice("claude-sonnet*", 3.00, 15.00),
		flatPriceAsOf("claude-haiku-4-5*", 1.00, 5.00, "2026-08-28"),
		flatPrice("claude-haiku*", 0.25, 1.25),
	},
	"deepseek": {
		// DeepSeek docs include an input cache-hit discount; this uses cache-miss rates for conservative estimates.
		flatPrice("deepseek-v4-flash*", 0.14, 0.28),
		flatPrice("deepseek-v4-pro*", 0.435, 0.87),
		flatPrice("deepseek-*", 0.14, 0.28),
	},
	"openrouter": {
		// OpenRouter model-catalog list prices, checked from the public Models API
		// (https://openrouter.ai/api/v1/models) on the per-route review date.
		// Keep these route-specific: OpenRouter aliases, variants, and provider
		// routing can have different prices and must remain unknown until reviewed.
		flatPriceAsOf("anthropic/claude-opus-4.8", 5.00, 25.00, "2026-07-14"),
		flatPriceAsOf("anthropic/claude-sonnet-5", 2.00, 10.00, "2026-07-14"),
		flatPriceAsOf("deepseek/deepseek-v4-flash", 0.09, 0.18, "2026-07-14"),
		// Introductory standard rates through 2026-12-31. Recheck when the
		// published standard price changes on 2027-01-01. Batch/alias routes
		// are deliberately not covered by this exact model ID.
		flatPriceAsOf("google/gemini-3.8-flash", 0.75, 3.75, "2026-09-06"),
		// Exact funded route reviewed from the Models API on 2026-09-07.
		// Its override starts at min_prompt_tokens=272000. Cache discounts
		// are omitted so budget estimates remain conservative.
		{Pattern: "openai/gpt-6-astra", AsOf: "2026-09-07", Tiers: []priceTier{
			{MaxInputTokens: 271999, InputUSDPerMTok: 10, OutputUSDPerMTok: 50},
			{InputUSDPerMTok: 20, OutputUSDPerMTok: 75},
		}},
		flatPriceAsOf("nvidia/nemotron-3.5-lightning:free", 0, 0, "2026-08-14"),
		flatPriceAsOf("nvidia/nemotron-3-super-120b-a12b:free", 0, 0, "2026-08-14"),
		flatPriceAsOf("nvidia/nemotron-3-ultra-550b-a55b:free", 0, 0, "2026-08-15"),
	},
	"gemini": {
		// Standard introductory rates, verified 2026-09-06. Recheck 2027-01-01.
		flatPriceAsOf("gemini-3.8-flash", 0.75, 3.75, "2026-09-06"),
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

func flatPriceAsOf(pattern string, inputUSDPerMTok, outputUSDPerMTok float64, asOf string) modelPrice {
	price := flatPrice(pattern, inputUSDPerMTok, outputUSDPerMTok)
	price.AsOf = asOf
	return price
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
			asOf := p.AsOf
			if asOf == "" {
				asOf = pricingAsOf
			}
			price := TokenPrice{
				InputUSDPerMTok:  tier.InputUSDPerMTok,
				OutputUSDPerMTok: tier.OutputUSDPerMTok,
				AsOf:             asOf,
			}
			if provider == "anthropic" {
				price.CacheWriteUSDPerMTok = tier.InputUSDPerMTok * anthropicCacheWriteMultiplier
				price.CacheReadUSDPerMTok = tier.InputUSDPerMTok * anthropicCacheReadMultiplier
			}
			return price, true
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
