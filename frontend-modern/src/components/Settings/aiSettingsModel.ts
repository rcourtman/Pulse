import type { SelectionCardOption } from '@/components/shared/SelectionCardGroup';
import type { AIProviderHealthStatus } from '@/utils/aiProviderHealthPresentation';
import { getProviderFromModelId } from '@/utils/aiProviderPresentation';
import type {
  AIProvider,
  AIProviderTestResult,
  AISettings as AISettingsType,
  ModelInfo,
} from '@/types/ai';

export type AIProviderCredentialsFormState = {
  anthropicApiKey: string;
  openaiApiKey: string;
  openrouterApiKey: string;
  deepseekApiKey: string;
  zaiApiKey: string;
  groqApiKey: string;
  mistralApiKey: string;
  cerebrasApiKey: string;
  togetherApiKey: string;
  fireworksApiKey: string;
  geminiApiKey: string;
  ollamaBaseUrl: string;
  ollamaKeepAlive: string;
  openaiBaseUrl: string;
  zaiBaseUrl: string;
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
  extraFields?: Array<{
    label: string;
    helpContentId?: string;
    inputField: keyof AIProviderCredentialsFormState;
    placeholder: string;
    type?: 'url' | 'text';
    helperText?: string;
  }>;
  clearTitle: string;
};

export type ProviderHealthState = {
  status: AIProviderHealthStatus;
  message: string;
  model?: string;
  cause?: string;
  summary?: string;
  recommendation?: string;
  action?: string;
};

export type ProviderTestResult = AIProviderTestResult;

export type AIAvailableModel = ModelInfo;

export const AI_PROVIDERS: AIProvider[] = [
  'anthropic',
  'openai',
  'openrouter',
  'deepseek',
  'gemini',
  'zai',
  'groq',
  'mistral',
  'cerebras',
  'together',
  'fireworks',
  'ollama',
];

export const AI_SETUP_PROVIDER_OPTIONS: SelectionCardOption<AIProvider>[] = [
  { value: 'anthropic', title: 'Anthropic', description: 'Claude' },
  { value: 'openai', title: 'OpenAI', description: 'ChatGPT' },
  { value: 'openrouter', title: 'OpenRouter', description: 'Gateway' },
  { value: 'deepseek', title: 'DeepSeek', description: 'V4' },
  { value: 'gemini', title: 'Gemini', description: 'Google' },
  { value: 'zai', title: 'Z.ai', description: 'GLM' },
  { value: 'groq', title: 'Groq', description: 'Fast' },
  { value: 'mistral', title: 'Mistral', description: 'Models' },
  { value: 'cerebras', title: 'Cerebras', description: 'Inference' },
  { value: 'together', title: 'Together', description: 'Open models' },
  { value: 'fireworks', title: 'Fireworks', description: 'Open models' },
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
    extraFields: [
      {
        label: 'Custom Base URL',
        helpContentId: 'ai.openai.baseUrl',
        inputField: 'openaiBaseUrl',
        placeholder: 'https://api.together.xyz/v1 (optional)',
        type: 'url',
      },
    ],
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
    provider: 'zai',
    title: 'Z.ai',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'zaiApiKey',
    placeholder: 'Z.AI API key',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Open docs →',
    actionLinkHref: 'https://docs.z.ai/guides/develop/openai/python',
    helperText: 'Uses https://api.z.ai/api/paas/v4 automatically.',
    extraFields: [
      {
        label: 'Custom Base URL',
        inputField: 'zaiBaseUrl',
        placeholder: 'https://api.z.ai/api/coding/paas/v4 (coding plan)',
        type: 'url',
        helperText: 'Override for a z.ai coding subscription.',
      },
    ],
    clearTitle: 'Clear API key',
  },
  {
    provider: 'groq',
    title: 'Groq',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'groqApiKey',
    placeholder: 'gsk_...',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://console.groq.com/keys',
    helperText: 'Uses https://api.groq.com/openai/v1 automatically.',
    clearTitle: 'Clear API key',
  },
  {
    provider: 'mistral',
    title: 'Mistral',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'mistralApiKey',
    placeholder: 'Mistral API key',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://console.mistral.ai/api-keys',
    helperText: 'Uses https://api.mistral.ai/v1 automatically.',
    clearTitle: 'Clear API key',
  },
  {
    provider: 'cerebras',
    title: 'Cerebras',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'cerebrasApiKey',
    placeholder: 'Cerebras API key',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Open docs →',
    actionLinkHref: 'https://inference-docs.cerebras.ai/resources/openai',
    helperText: 'Uses https://api.cerebras.ai/v1 automatically.',
    clearTitle: 'Clear API key',
  },
  {
    provider: 'together',
    title: 'Together AI',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'togetherApiKey',
    placeholder: 'Together API key',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://api.together.ai/settings/api-keys',
    helperText: 'Uses https://api.together.xyz/v1 automatically.',
    clearTitle: 'Clear API key',
  },
  {
    provider: 'fireworks',
    title: 'Fireworks AI',
    configuredLabel: 'Configured',
    inputType: 'password',
    inputField: 'fireworksApiKey',
    placeholder: 'Fireworks API key',
    configuredPlaceholder: '••••••••••• (configured)',
    actionLinkLabel: 'Get API key →',
    actionLinkHref: 'https://fireworks.ai/account/api-keys',
    helperText: 'Uses https://api.fireworks.ai/inference/v1 automatically.',
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
    extraFields: [
      {
        label: 'Keep Alive',
        helpContentId: 'ai.ollama.keepAlive',
        inputField: 'ollamaKeepAlive',
        placeholder: '30s, 5m, 24h, 0, or blank',
        type: 'text',
        helperText: 'Clear to use the Ollama server default.',
      },
    ],
    clearTitle: 'Clear Ollama URL',
  },
];

export const createInitialProviderHealth = (): Record<AIProvider, ProviderHealthState> => ({
  anthropic: { status: 'not_configured', message: '' },
  openai: { status: 'not_configured', message: '' },
  openrouter: { status: 'not_configured', message: '' },
  deepseek: { status: 'not_configured', message: '' },
  gemini: { status: 'not_configured', message: '' },
  zai: { status: 'not_configured', message: '' },
  groq: { status: 'not_configured', message: '' },
  mistral: { status: 'not_configured', message: '' },
  cerebras: { status: 'not_configured', message: '' },
  together: { status: 'not_configured', message: '' },
  fireworks: { status: 'not_configured', message: '' },
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
    case 'zai':
      return Boolean(settings.zai_configured);
    case 'groq':
      return Boolean(settings.groq_configured);
    case 'mistral':
      return Boolean(settings.mistral_configured);
    case 'cerebras':
      return Boolean(settings.cerebras_configured);
    case 'together':
      return Boolean(settings.together_configured);
    case 'fireworks':
      return Boolean(settings.fireworks_configured);
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
