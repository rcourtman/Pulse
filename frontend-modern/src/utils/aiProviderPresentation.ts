import type { AIProvider, ModelInfo } from '@/types/ai';

export const AI_PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  openrouter: 'OpenRouter',
  deepseek: 'DeepSeek',
  gemini: 'Google Gemini',
  ollama: 'Ollama',
};

export const getAIProviderDisplayName = (provider: string): string =>
  AI_PROVIDER_DISPLAY_NAMES[provider] || provider;

export function getProviderFromModelId(modelId: string): AIProvider | string {
  const colonIndex = modelId.indexOf(':');
  if (colonIndex > 0) {
    return modelId.substring(0, colonIndex);
  }
  if (
    /^(openai|anthropic|google|deepseek|meta-llama|mistralai|x-ai|xai|cohere|qwen)\//.test(modelId)
  ) {
    return 'openrouter';
  }
  if (
    modelId.includes('claude') ||
    modelId.includes('opus') ||
    modelId.includes('sonnet') ||
    modelId.includes('haiku')
  ) {
    return 'anthropic';
  }
  if (
    modelId.includes('gpt') ||
    modelId.includes('o1') ||
    modelId.includes('o3') ||
    modelId.includes('o4')
  ) {
    return 'openai';
  }
  if (modelId.includes('deepseek')) {
    return 'deepseek';
  }
  if (modelId.includes('gemini')) {
    return 'gemini';
  }
  return 'ollama';
}

type AIModelRouteLabelInput =
  | string
  | (Pick<ModelInfo, 'id'> & Partial<Pick<ModelInfo, 'name' | 'provider'>>);

const GATEWAY_MODEL_PROVIDERS = new Set<string>(['openrouter']);

const modelIdForLabel = (model: AIModelRouteLabelInput): string =>
  typeof model === 'string' ? model.trim() : model.id.trim();

const baseModelLabel = (model: AIModelRouteLabelInput): string => {
  if (typeof model !== 'string') {
    const name = model.name?.trim();
    if (name) {
      return name;
    }
  }
  const id = modelIdForLabel(model);
  return id.split(':').pop() || id;
};

const transportProviderForLabel = (model: AIModelRouteLabelInput): string => {
  if (typeof model !== 'string') {
    const provider = model.provider?.trim();
    if (provider) {
      return provider;
    }
  }
  const id = modelIdForLabel(model);
  return id ? getProviderFromModelId(id) : '';
};

export const formatAIModelRouteLabel = (model: AIModelRouteLabelInput): string => {
  const label = baseModelLabel(model);
  const provider = transportProviderForLabel(model);
  if (!provider || !GATEWAY_MODEL_PROVIDERS.has(provider)) {
    return label;
  }
  const providerName = getAIProviderDisplayName(provider);
  if (!providerName || label.toLowerCase().includes(providerName.toLowerCase())) {
    return label;
  }
  return `${label} via ${providerName}`;
};
