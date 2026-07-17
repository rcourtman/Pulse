import { describe, expect, it } from 'vitest';

import {
  formatAIModelRouteLabel,
  getAIProviderDisplayName,
  getProviderFromModelId,
  isPulseOwnedLocalModelRoute,
} from '@/utils/aiProviderPresentation';

// `modelIdForLabel`, `explicitModelNameForLabel`, `baseModelLabel`,
// `transportProviderForLabel`, `titleizeRouteNamespace`, `titleizeModelToken`,
// `titleizeModelRouteId`, `gatewayFallbackLabel`, and
// `directProviderRouteFallbackLabel` are module-private (not exported); their
// branches are exercised transitively through the four exported entry points
// below, asserting on their observable outputs.

describe('getAIProviderDisplayName — branch coverage', () => {
  it('returns the curated display name for every known provider key (map hit arm)', () => {
    // AI_PROVIDER_DISPLAY_NAMES[provider] arm — sample across simple, multi-word,
    // parenthesised, and hyphenated values to lock the map contents.
    expect(getAIProviderDisplayName('anthropic')).toBe('Anthropic');
    expect(getAIProviderDisplayName('openai')).toBe('OpenAI');
    expect(getAIProviderDisplayName('gemini')).toBe('Google Gemini');
    expect(getAIProviderDisplayName('codex-subscription')).toBe('Codex subscription (local)');
    expect(getAIProviderDisplayName('claude-subscription')).toBe('Claude subscription (local)');
    expect(getAIProviderDisplayName('pulse')).toBe('Pulse');
    expect(getAIProviderDisplayName('together')).toBe('Together AI');
  });

  it('falls back to the raw provider string for an unknown key (|| arm)', () => {
    // AI_PROVIDER_DISPLAY_NAMES[provider] === undefined -> || provider fallback.
    expect(getAIProviderDisplayName('not-a-real-provider')).toBe('not-a-real-provider');
  });

  it('returns the empty string for an empty-string key (both arms collapse to "")', () => {
    // map miss -> '' || '' -> ''. Documents the boundary behaviour.
    expect(getAIProviderDisplayName('')).toBe('');
  });
});

describe('getProviderFromModelId — branch coverage', () => {
  it('returns the substring before the first colon when colonIndex > 0 (colon branch)', () => {
    expect(getProviderFromModelId('anthropic:claude-3-opus')).toBe('anthropic');
    expect(getProviderFromModelId('openrouter:anthropic/claude')).toBe('openrouter');
    expect(getProviderFromModelId('pulse:local-inventory')).toBe('pulse');
  });

  it('skips the colon branch when the colon is at index 0 (colonIndex === 0 boundary)', () => {
    // ':foo' -> colonIndex 0 -> 0 > 0 false -> falls through to keyword detection
    // -> no keyword match -> default 'ollama'.
    expect(getProviderFromModelId(':something')).toBe('ollama');
  });

  it('maps each openrouter regex prefix variant to "openrouter"', () => {
    // Exercises every OR clause of
    //   /^(openai|anthropic|google|deepseek|meta-llama|mistralai|x-ai|xai|cohere|qwen)\//
    expect(getProviderFromModelId('openai/gpt-4o')).toBe('openrouter');
    expect(getProviderFromModelId('anthropic/claude-3')).toBe('openrouter');
    expect(getProviderFromModelId('google/gemini-pro')).toBe('openrouter');
    expect(getProviderFromModelId('deepseek/deepseek-chat')).toBe('openrouter');
    expect(getProviderFromModelId('meta-llama/llama-3')).toBe('openrouter');
    expect(getProviderFromModelId('mistralai/mistral-large')).toBe('openrouter');
    expect(getProviderFromModelId('x-ai/grok')).toBe('openrouter');
    expect(getProviderFromModelId('xai/grok-2')).toBe('openrouter');
    expect(getProviderFromModelId('cohere/command-r')).toBe('openrouter');
    expect(getProviderFromModelId('qwen/qwen-2.5')).toBe('openrouter');
  });

  it('maps each anthropic keyword (claude/opus/sonnet/haiku) to "anthropic"', () => {
    expect(getProviderFromModelId('claude-3-opus')).toBe('anthropic');
    expect(getProviderFromModelId('my-opus-model')).toBe('anthropic');
    expect(getProviderFromModelId('sonnet-4')).toBe('anthropic');
    expect(getProviderFromModelId('haiku-turbo')).toBe('anthropic');
  });

  it('maps each openai keyword (gpt/o1/o3/o4) to "openai"', () => {
    expect(getProviderFromModelId('gpt-4o-mini')).toBe('openai');
    expect(getProviderFromModelId('o1-preview')).toBe('openai');
    expect(getProviderFromModelId('o3-mini')).toBe('openai');
    expect(getProviderFromModelId('o4-analysis')).toBe('openai');
  });

  it('maps a deepseek-keyword id to "deepseek"', () => {
    expect(getProviderFromModelId('deepseek-v3')).toBe('deepseek');
  });

  it('maps a gemini-keyword id to "gemini"', () => {
    expect(getProviderFromModelId('gemini-1.5-pro')).toBe('gemini');
  });

  it('maps a glm-keyword id to "zai"', () => {
    expect(getProviderFromModelId('glm-4.5')).toBe('zai');
  });

  it('maps a groq-keyword id to "groq"', () => {
    expect(getProviderFromModelId('groq-llama-3')).toBe('groq');
  });

  it('maps a mistral-keyword id to "mistral"', () => {
    expect(getProviderFromModelId('mistral-7b')).toBe('mistral');
  });

  it('maps a cerebras-keyword id to "cerebras"', () => {
    expect(getProviderFromModelId('cerebras-llama3')).toBe('cerebras');
  });

  it('falls back to "ollama" when no prefix/keyword matches (default arm)', () => {
    // Includes the empty-string boundary, which has no colon and no keyword.
    expect(getProviderFromModelId('random-model')).toBe('ollama');
    expect(getProviderFromModelId('')).toBe('ollama');
  });
});

describe('isPulseOwnedLocalModelRoute — branch coverage', () => {
  it('returns true for the canonical pulse:local-inventory route', () => {
    expect(isPulseOwnedLocalModelRoute('pulse:local-inventory')).toBe(true);
  });

  it('returns true for the canonical pulse:mock-assistant route', () => {
    expect(isPulseOwnedLocalModelRoute('pulse:mock-assistant')).toBe(true);
  });

  it('honours the .trim() normaliser for whitespace-padded routes', () => {
    // Boolean(LOCAL_MODEL_ROUTE_LABELS[modelId.trim()]) — verifies trim arm.
    expect(isPulseOwnedLocalModelRoute('  pulse:local-inventory  ')).toBe(true);
  });

  it('returns false for an unknown pulse-namespaced route (Boolean(undefined) arm)', () => {
    expect(isPulseOwnedLocalModelRoute('pulse:unknown')).toBe(false);
  });

  it('returns false for a generic non-pulse id', () => {
    expect(isPulseOwnedLocalModelRoute('anthropic:claude-3')).toBe(false);
  });

  it('returns false for the empty string (misses the map)', () => {
    expect(isPulseOwnedLocalModelRoute('')).toBe(false);
  });
});

describe('formatAIModelRouteLabel — branch coverage', () => {
  describe('string input — direct (non-gateway) path', () => {
    it('builds "Provider: Model" for a colon-namespaced direct-route id', () => {
      // transportProvider -> getProviderFromModelId('anthropic:...') colon arm.
      // directProviderRouteFallbackLabel returns the formatted pair.
      expect(formatAIModelRouteLabel('anthropic:claude-3-opus')).toBe('Anthropic: Claude 3 Opus');
    });

    it('returns the raw id when there is no separator and no keyword match', () => {
      // directProviderRouteFallbackLabel returns '' (separator <= 0) -> baseModelLabel
      // -> id.split(':').pop() || id falls back to the whole id.
      expect(formatAIModelRouteLabel('random-model')).toBe('random-model');
    });

    it('returns the curated Pulse label for a pulse-owned local route', () => {
      // directProviderRouteFallbackLabel early-returns '' for pulse-owned routes
      // -> baseModelLabel returns the LOCAL_MODEL_ROUTE_LABELS entry verbatim.
      expect(formatAIModelRouteLabel('pulse:local-inventory')).toBe('Pulse inventory');
      expect(formatAIModelRouteLabel('pulse:mock-assistant')).toBe('Pulse mock Assistant');
    });

    it('falls back to the full id when the separator sits at the end of the id', () => {
      // directProviderRouteFallbackLabel: separator === id.length - 1 -> return ''.
      // baseModelLabel: id.split(':').pop() === '' -> || id returns the whole id.
      expect(formatAIModelRouteLabel('mistral:')).toBe('mistral:');
    });

    it('returns the empty string for an empty-string model', () => {
      // transportProviderForLabel: id ? getProviderFromModelId(id) : '' -> ''.
      // !provider short-circuits the main guard, label resolves to ''.
      expect(formatAIModelRouteLabel('')).toBe('');
    });
  });

  describe('string input — gateway (openrouter) path', () => {
    it('builds "Upstream: Model via OpenRouter" for a slash-separated gateway payload', () => {
      // gatewayFallbackLabel: payload has slash at index > 0, both halves non-empty.
      // label does not already contain "OpenRouter" -> via suffix appended.
      expect(formatAIModelRouteLabel('openrouter:anthropic/claude-3-opus')).toBe(
        'Anthropic: Claude 3 Opus via OpenRouter',
      );
    });

    it('titleizes known upstream namespaces (meta-llama) and 8b-style model tokens', () => {
      // Also exercises titleizeModelToken /^[0-9]+b$/ -> '8B' arm.
      expect(formatAIModelRouteLabel('openrouter:meta-llama/llama-3-8b')).toBe(
        'Meta Llama: Llama 3 8B via OpenRouter',
      );
    });

    it('titleizes an unknown upstream namespace via the split/title-case fallback', () => {
      // titleizeRouteNamespace: not in UPSTREAM_PROVIDER_DISPLAY_NAMES -> split/->
      // map(charAt(0).upper + slice) fallback.
      expect(formatAIModelRouteLabel('openrouter:custom-ns/some-model')).toBe(
        'Custom Ns: Some Model via OpenRouter',
      );
    });

    it('falls back to baseModelLabel and appends via-suffix when payload has no slash', () => {
      // gatewayFallbackLabel: slashIndex <= 0 -> return ''.
      expect(formatAIModelRouteLabel('openrouter:gpt-4o')).toBe('gpt-4o via OpenRouter');
    });

    it('treats slashIndex === 0 (slash at payload start) as no usable namespace', () => {
      // gatewayFallbackLabel: slashIndex <= 0 boundary at exactly 0.
      expect(formatAIModelRouteLabel('openrouter:/foo')).toBe('/foo via OpenRouter');
    });

    it('treats slashIndex === payload.length - 1 (slash at payload end) as no usable model', () => {
      // gatewayFallbackLabel: trailing-slash boundary.
      expect(formatAIModelRouteLabel('openrouter:foo/')).toBe('foo/ via OpenRouter');
    });

    it('suppresses the via-suffix when label already contains the provider name', () => {
      // label.toLowerCase().includes(providerName.toLowerCase()) true arm.
      expect(formatAIModelRouteLabel('openrouter:openrouter/auto')).toBe('OpenRouter: Auto');
    });

    it('returns the bare payload when the upstream namespace normalises to empty', () => {
      // titleizeRouteNamespace('~/foo' slice 0..slashIndex = '~') -> normalised ''
      // -> !upstreamProvider -> gatewayFallbackLabel returns '' -> baseModelLabel.
      expect(formatAIModelRouteLabel('openrouter:~/foo')).toBe('~/foo via OpenRouter');
    });
  });

  describe('object input — transportProviderForLabel branches', () => {
    it('honours an explicit provider override that is a gateway provider', () => {
      // transportProviderForLabel: model.provider?.trim() truthy -> return provider.
      // Gateway path taken from the override alone (no colon/slash in id).
      expect(formatAIModelRouteLabel({ id: 'foo', provider: 'openrouter' })).toBe(
        'foo via OpenRouter',
      );
    });

    it('honours an explicit provider override that is a non-gateway provider', () => {
      // Same transportProvider arm, but direct path. id has no separator so
      // directProviderRouteFallbackLabel returns '' -> baseModelLabel returns id.
      expect(formatAIModelRouteLabel({ id: 'foo', provider: 'mistral' })).toBe('foo');
    });

    it('early-returns "" inside directProviderRouteFallbackLabel when the slice-provider is a gateway provider', () => {
      // Override is non-gateway (mistral) -> direct path. id 'openrouter:foo'
      // slices to 'openrouter' which IS in GATEWAY_MODEL_PROVIDERS -> return ''.
      expect(formatAIModelRouteLabel({ id: 'openrouter:foo', provider: 'mistral' })).toBe('foo');
    });

    it('derives the provider from the id when no override is supplied', () => {
      // transportProviderForLabel: model.provider undefined -> falls to
      // getProviderFromModelId(id) -> 'anthropic' via the colon branch.
      expect(formatAIModelRouteLabel({ id: 'anthropic:claude-3-opus' })).toBe(
        'Anthropic: Claude 3 Opus',
      );
    });

    it('returns "" when the object has neither provider nor a non-empty id', () => {
      // transportProviderForLabel: id ? getProviderFromModelId(id) : '' -> ''.
      expect(formatAIModelRouteLabel({ id: '' })).toBe('');
    });
  });

  describe('object input — explicitModelNameForLabel branches', () => {
    it('uses the explicit name when it differs from id and routePayload', () => {
      // explicitModelNameForLabel returns the trimmed name -> baseModelLabel uses it.
      expect(
        formatAIModelRouteLabel({ id: 'anthropic:claude-3-opus', name: 'My Custom Claude' }),
      ).toBe('My Custom Claude');
    });

    it('drops the explicit name when it equals the whole id (name === id arm)', () => {
      // explicitModelNameForLabel: name === id -> return ''. baseModelLabel falls
      // through to id.split(':').pop().
      expect(formatAIModelRouteLabel({ id: 'foo', name: 'foo' })).toBe('foo');
    });

    it('drops the explicit name when it equals the route payload (name === routePayload arm)', () => {
      // explicitModelNameForLabel: routePayload === name -> return ''. Falls back
      // through directProviderRouteFallbackLabel.
      expect(formatAIModelRouteLabel({ id: 'anthropic:claude', name: 'claude' })).toBe(
        'Anthropic: Claude',
      );
    });

    it('treats a whitespace-only name as absent (name?.trim() falsy arm)', () => {
      // explicitModelNameForLabel: '   '.trim() === '' -> !name -> return ''.
      expect(formatAIModelRouteLabel({ id: 'foo', name: '   ' })).toBe('foo');
    });
  });
});
