import type { AIProvider } from '@/types/ai';

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
