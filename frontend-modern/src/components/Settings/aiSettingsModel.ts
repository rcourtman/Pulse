import type { SelectionCardOption } from '@/components/shared/SelectionCardGroup';
import type { AIProviderHealthStatus } from '@/utils/aiProviderHealthPresentation';
import { getProviderFromModelId } from '@/utils/aiProviderPresentation';
import type { AIProvider, AISettings as AISettingsType } from '@/types/ai';

export type AIProviderCredentialsFormState = {
  anthropicApiKey: string;
  openaiApiKey: string;
  openrouterApiKey: string;
  deepseekApiKey: string;
  geminiApiKey: string;
  ollamaBaseUrl: string;
  openaiBaseUrl: string;
};

export type AIProviderConfig = {
  provider: AIProvider;
  title: string;
  configuredLabel?: string;
  inputType: 'password' | 'url';
  inputField: keyof AIProviderCredentialsFormState;
  placeholder: string;
  configuredPlaceholder?: string;
  actionLinkLabel: string;
  actionLinkHref: string;
  actionLinkSuffix?: string;
  helperText?: string;
  extraField?: {
    label: string;
    helpContentId?: string;
    inputField: 'openaiBaseUrl';
    placeholder: string;
  };
  clearTitle: string;
};

export type ProviderHealthState = {
  status: AIProviderHealthStatus;
  message: string;
};

export type ProviderTestResult = {
  provider: AIProvider;
  success: boolean;
  message: string;
};

export type AIAvailableModel = {
  id: string;
  name: string;
  description?: string;
};

export const AI_PROVIDERS: AIProvider[] = [
  'anthropic',
  'openai',
  'openrouter',
  'deepseek',
  'gemini',
  'ollama',
];

export const AI_SETUP_PROVIDER_OPTIONS: SelectionCardOption<AIProvider>[] = [
  { value: 'anthropic', title: 'Anthropic', description: 'Claude' },
  { value: 'openai', title: 'OpenAI', description: 'ChatGPT' },
  { value: 'openrouter', title: 'OpenRouter', description: 'Gateway' },
  { value: 'deepseek', title: 'DeepSeek', description: 'V3' },
  { value: 'gemini', title: 'Gemini', description: 'Google' },
  { value: 'ollama', title: 'Ollama', description: 'Local' },
];

export const AI_PROVIDER_CONFIGS: AIProviderConfig[] = [
  {
    provider: 'anthropic',
    title: 'Anthropic',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'anthropicApiKey',
    placeholder: 'sk-ant-...',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://console.anthropic.com/settings/keys',
    clearTitle: 'Clear API key',
  },
  {
    provider: 'openai',
    title: 'OpenAI',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'openaiApiKey',
    placeholder: 'sk-...',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://platform.openai.com/api-keys',
    clearTitle: 'Clear API key',
    extraField: {
      label: 'Custom Base URL',
      helpContentId: 'ai.openai.baseUrl',
      inputField: 'openaiBaseUrl',
      placeholder: 'https://api.together.xyz/v1 (optional)',
    },
  },
  {
    provider: 'openrouter',
    title: 'OpenRouter',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'openrouterApiKey',
    placeholder: 'sk-or-...',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://openrouter.ai/keys',
    helperText: 'Uses https://openrouter.ai/api/v1 automatically.',
    clearTitle: 'Clear API key',
  },
  {
    provider: 'deepseek',
    title: 'DeepSeek',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'deepseekApiKey',
    placeholder: 'sk-...',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://platform.deepseek.com/api_keys',
    clearTitle: 'Clear API key',
  },
  {
    provider: 'gemini',
    title: 'Google Gemini',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'geminiApiKey',
    placeholder: 'AIza...',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://aistudio.google.com/app/apikey',
    clearTitle: 'Clear API key',
  },
  {
    provider: 'ollama',
    title: 'Ollama',
    configuredLabel: 'Available',
    inputType: 'url',
    inputField: 'ollamaBaseUrl',
    placeholder: 'http://localhost:11434',
    actionLinkLabel: 'Learn about Ollama →',
    actionLinkHref: 'https://ollama.ai',
    actionLinkSuffix: ' · Free & local',
    clearTitle: 'Clear Ollama URL',
  },
];

export const createInitialProviderHealth = (): Record<AIProvider, ProviderHealthState> => ({
  anthropic: { status: 'not_configured', message: '' },
  openai: { status: 'not_configured', message: '' },
  openrouter: { status: 'not_configured', message: '' },
  deepseek: { status: 'not_configured', message: '' },
  gemini: { status: 'not_configured', message: '' },
  ollama: { status: 'not_configured', message: '' },
});

export function getAIProviderConfig(provider: AIProvider): AIProviderConfig {
  const config = AI_PROVIDER_CONFIGS.find((entry) => entry.provider === provider);
  if (!config) {
    throw new Error(`Unknown AI provider: ${provider}`);
  }
  return config;
}

export function isAIProviderConfigured(
  provider: AIProvider | string,
  settings: AISettingsType | null,
): boolean {
  if (!settings) return false;
  switch (provider) {
    case 'anthropic':
      return settings.anthropic_configured;
    case 'openai':
      return settings.openai_configured;
    case 'openrouter':
      return settings.openrouter_configured;
    case 'deepseek':
      return settings.deepseek_configured;
    case 'gemini':
      return settings.gemini_configured;
    case 'ollama':
      return settings.ollama_configured;
    default:
      return false;
  }
}

export function isModelProviderConfigured(
  modelId: string,
  settings: AISettingsType | null,
): boolean {
  const provider = getProviderFromModelId(modelId);
  return isAIProviderConfigured(provider, settings);
}

export function groupModelsByProvider(models: AIAvailableModel[]): Map<string, AIAvailableModel[]> {
  const grouped = new Map<string, AIAvailableModel[]>();

  for (const model of models) {
    const provider = getProviderFromModelId(model.id);
    const existing = grouped.get(provider) || [];
    existing.push(model);
    grouped.set(provider, existing);
  }

  return grouped;
}
