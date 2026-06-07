import type { AIProvider, ModelInfo } from '@/types/ai';

export const AI_PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  openrouter: 'OpenRouter',
  deepseek: 'DeepSeek',
  gemini: 'Google Gemini',
  ollama: 'Ollama',
  pulse: 'Pulse',
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
const LOCAL_MODEL_ROUTE_LABELS: Record<string, string> = {
  'pulse:local-inventory': 'Pulse inventory',
  'pulse:mock-assistant': 'Pulse mock Assistant',
};
export const isPulseOwnedLocalModelRoute = (modelId: string): boolean =>
  Boolean(LOCAL_MODEL_ROUTE_LABELS[modelId.trim()]);
const UPSTREAM_PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic',
  'arcee-ai': 'Arcee AI',
  'bytedance-seed': 'ByteDance Seed',
  cohere: 'Cohere',
  deepseek: 'DeepSeek',
  google: 'Google',
  'ibm-granite': 'IBM',
  inclusionai: 'inclusionAI',
  kwaipilot: 'Kwaipilot',
  meta: 'Meta',
  'meta-llama': 'Meta Llama',
  minimax: 'MiniMax',
  mistral: 'Mistral',
  mistralai: 'Mistral',
  moonshotai: 'MoonshotAI',
  nvidia: 'NVIDIA',
  openai: 'OpenAI',
  openrouter: 'OpenRouter',
  perceptron: 'Perceptron',
  poolside: 'Poolside',
  qwen: 'Qwen',
  rekaai: 'Reka',
  stepfun: 'StepFun',
  tencent: 'Tencent',
  xiaomi: 'Xiaomi',
  xai: 'xAI',
  'x-ai': 'xAI',
  'z-ai': 'Z.ai',
};
const MODEL_TOKEN_DISPLAY_NAMES: Record<string, string> = {
  api: 'API',
  deepseek: 'DeepSeek',
  flash: 'Flash',
  gemini: 'Gemini',
  gpt: 'GPT',
  haiku: 'Haiku',
  instruct: 'Instruct',
  llama: 'Llama',
  mini: 'Mini',
  opus: 'Opus',
  plus: 'Plus',
  pro: 'Pro',
  qwen: 'Qwen',
  r: 'R',
  sonnet: 'Sonnet',
  turbo: 'Turbo',
  v: 'V',
};

const modelIdForLabel = (model: AIModelRouteLabelInput): string =>
  typeof model === 'string' ? model.trim() : model.id.trim();

const explicitModelNameForLabel = (model: AIModelRouteLabelInput): string => {
  if (typeof model === 'string') return '';
  const name = model.name?.trim();
  if (!name) return '';
  const id = modelIdForLabel(model);
  const routePayload = id.split(':').pop()?.trim();
  if (name === id || (routePayload && name === routePayload)) return '';
  return name;
};

const baseModelLabel = (model: AIModelRouteLabelInput): string => {
  const id = modelIdForLabel(model);
  const localLabel = LOCAL_MODEL_ROUTE_LABELS[id];
  if (localLabel) {
    return localLabel;
  }
  const name = explicitModelNameForLabel(model);
  if (name) return name;
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

const titleizeRouteNamespace = (namespace: string): string => {
  const normalized = namespace.trim().toLowerCase().replace(/^~+/, '');
  if (!normalized) return '';
  const known = UPSTREAM_PROVIDER_DISPLAY_NAMES[normalized];
  if (known) return known;
  return normalized
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((token) => token.charAt(0).toUpperCase() + token.slice(1))
    .join(' ');
};

const titleizeModelToken = (token: string): string => {
  const normalized = token.trim().toLowerCase();
  if (!normalized) return '';
  const known = MODEL_TOKEN_DISPLAY_NAMES[normalized];
  if (known) return known;
  const alphaNumeric = normalized.match(/^([a-z]+)([0-9].*)$/);
  if (alphaNumeric) {
    const prefix = MODEL_TOKEN_DISPLAY_NAMES[alphaNumeric[1]] || titleizeModelToken(alphaNumeric[1]);
    return `${prefix}${alphaNumeric[2]}`;
  }
  if (/^[rv][0-9]/.test(normalized)) {
    return normalized.charAt(0).toUpperCase() + normalized.slice(1);
  }
  return normalized.charAt(0).toUpperCase() + normalized.slice(1);
};

const titleizeModelRouteId = (modelId: string): string =>
  modelId
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map(titleizeModelToken)
    .join(' ');

const gatewayFallbackLabel = (model: AIModelRouteLabelInput): string => {
  if (explicitModelNameForLabel(model)) return '';
  const id = modelIdForLabel(model);
  const payload = id.includes(':') ? id.split(':').slice(1).join(':') : id;
  const slashIndex = payload.indexOf('/');
  if (slashIndex <= 0 || slashIndex === payload.length - 1) return '';
  const upstreamProvider = titleizeRouteNamespace(payload.slice(0, slashIndex));
  const modelLabel = titleizeModelRouteId(payload.slice(slashIndex + 1));
  if (!upstreamProvider || !modelLabel) return '';
  return `${upstreamProvider}: ${modelLabel}`;
};

const directProviderRouteFallbackLabel = (model: AIModelRouteLabelInput): string => {
  if (explicitModelNameForLabel(model)) return '';
  const id = modelIdForLabel(model);
  if (isPulseOwnedLocalModelRoute(id)) return '';
  const separator = id.indexOf(':');
  if (separator <= 0 || separator === id.length - 1) return '';
  const provider = id.slice(0, separator);
  if (GATEWAY_MODEL_PROVIDERS.has(provider)) return '';
  const providerName = getAIProviderDisplayName(provider);
  const modelLabel = titleizeModelRouteId(id.slice(separator + 1));
  if (!providerName || !modelLabel) return '';
  return `${providerName}: ${modelLabel}`;
};

export const formatAIModelRouteLabel = (model: AIModelRouteLabelInput): string => {
  const provider = transportProviderForLabel(model);
  const label =
    provider && GATEWAY_MODEL_PROVIDERS.has(provider)
      ? gatewayFallbackLabel(model) || baseModelLabel(model)
      : directProviderRouteFallbackLabel(model) || baseModelLabel(model);
  if (!provider || !GATEWAY_MODEL_PROVIDERS.has(provider)) {
    return label;
  }
  const providerName = getAIProviderDisplayName(provider);
  if (!providerName || label.toLowerCase().includes(providerName.toLowerCase())) {
    return label;
  }
  return `${label} via ${providerName}`;
};
