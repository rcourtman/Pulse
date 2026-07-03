import type { AIProvider, ModelInfo } from '@/types/ai';

export const PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  deepseek: 'DeepSeek',
  requesty: 'Requesty',
  gemini: 'Google Gemini',
  ollama: 'Ollama',
};

const KNOWN_PROVIDERS: AIProvider[] = ['anthropic', 'openai', 'deepseek', 'gemini', 'ollama', 'requesty'];

export function getProviderFromModelId(modelId: string): string {
  const colonIndex = modelId.indexOf(':');
  if (colonIndex > 0) {
    const prefix = modelId.substring(0, colonIndex);
    if (KNOWN_PROVIDERS.includes(prefix as AIProvider)) {
      return prefix;
    }
  }

  if (modelId.includes('/')) {
    return 'openai';
  }

  const name = colonIndex > 0 ? modelId.substring(0, colonIndex) : modelId;
  if (name.startsWith('claude') || name.startsWith('opus') || name.startsWith('sonnet') || name.startsWith('haiku')) {
    return 'anthropic';
  }
  if (name.startsWith('gpt') || name.startsWith('o1') || name.startsWith('o3') || name.startsWith('o4')) {
    return 'openai';
  }
  if (name.startsWith('deepseek')) {
    return 'deepseek';
  }
  if (name.startsWith('gemini')) {
    return 'gemini';
  }
  return 'ollama';
}

export function getProviderForModel(model: Pick<ModelInfo, 'id'> & Partial<Pick<ModelInfo, 'provider'>>): string {
  if (model.provider && KNOWN_PROVIDERS.includes(model.provider)) {
    return model.provider;
  }
  return getProviderFromModelId(model.id);
}

export function groupModelsByProvider<T extends Pick<ModelInfo, 'id'> & Partial<Pick<ModelInfo, 'provider'>>>(models: T[]): Map<string, T[]> {
  const grouped = new Map<string, T[]>();

  for (const model of models) {
    const provider = getProviderForModel(model);
    const existing = grouped.get(provider) || [];
    existing.push(model);
    grouped.set(provider, existing);
  }

  return grouped;
}
