import type { SelectionCardOption } from '@/components/shared/SelectionCardGroup';
import type {
  AIProviderTestResult,
  AISettingsUpdateRequest,
  ModelInfo,
  PatrolCostProjection,
  PatrolModelGuidanceResponse,
  PatrolModelReadinessSnapshot,
} from '@/types/ai';
import {
  formatPatrolCostUSD,
  formatPatrolIntervalPhrase,
  getPatrolGuidedModelIds,
  getPatrolIntervalAutoAdjust,
  PATROL_COST_DEFAULT_INTERVAL_MINUTES,
  resolvePatrolModelGuidance,
  splitModelRoute,
  type PatrolIntervalAutoAdjust,
} from '@/utils/aiPatrolCostPresentation';

/**
 * One-page Patrol setup, shown on the Patrol page when no provider is
 * configured or Pulse Intelligence is off. It covers the two routes most
 * paid installs use (a cloud API key, or Ollama on their own hardware),
 * picks the Patrol model from Pulse's guidance table instead of a raw list,
 * prices it before anything is saved, and runs the Patrol model check in the
 * same step. Everything else (subscription CLIs, custom endpoints, other
 * providers, model overrides) stays on Provider & Models.
 */

export type PatrolSetupProvider =
  'anthropic' | 'openai' | 'gemini' | 'deepseek' | 'openrouter' | 'ollama';

export const PATROL_SETUP_PROVIDER_OPTIONS: SelectionCardOption<PatrolSetupProvider>[] = [
  { value: 'anthropic', title: 'Anthropic', description: 'Claude' },
  { value: 'openai', title: 'OpenAI', description: 'GPT' },
  { value: 'gemini', title: 'Gemini', description: 'Google' },
  { value: 'deepseek', title: 'DeepSeek', description: 'Low cost' },
  { value: 'openrouter', title: 'OpenRouter', description: 'Many models' },
  { value: 'ollama', title: 'Ollama', description: 'Your hardware' },
];

const PATROL_SETUP_API_KEY_FIELDS: Record<
  Exclude<PatrolSetupProvider, 'ollama'>,
  keyof AISettingsUpdateRequest
> = {
  anthropic: 'anthropic_api_key',
  openai: 'openai_api_key',
  gemini: 'gemini_api_key',
  deepseek: 'deepseek_api_key',
  openrouter: 'openrouter_api_key',
};

export const PATROL_SETUP_DEFAULT_OLLAMA_URL = 'http://localhost:11434';

/** Start on the provider readiness names (the one that refused), else Anthropic. */
export function getPatrolSetupInitialProvider(
  readinessProvider?: string | null,
): PatrolSetupProvider {
  const named = readinessProvider?.trim().toLowerCase();
  const match = PATROL_SETUP_PROVIDER_OPTIONS.find((option) => option.value === named);
  return match ? match.value : 'anthropic';
}

export type PatrolSetupPhase = 'idle' | 'connecting' | 'checking' | 'ready' | 'failed';

export interface PatrolSetupInput {
  apiKey: string;
  ollamaUrl: string;
}

/**
 * The settings write that connects the provider and turns Pulse
 * Intelligence on, or null while the required field is empty.
 */
export function buildPatrolSetupConnectRequest(
  provider: PatrolSetupProvider,
  input: PatrolSetupInput,
): AISettingsUpdateRequest | null {
  if (provider === 'ollama') {
    const url = input.ollamaUrl.trim();
    return url ? { enabled: true, ollama_base_url: url } : null;
  }
  const key = input.apiKey.trim();
  if (!key) return null;
  return { enabled: true, [PATROL_SETUP_API_KEY_FIELDS[provider]]: key };
}

/**
 * The provider:model route Pulse's guidance table names for a provider
 * (recommended before suggested), used to price Patrol before anything is
 * saved. The exact model id comes from the provider's own list after connect.
 */
export function getPatrolSetupSuggestedRoute(
  provider: PatrolSetupProvider,
  guidance: PatrolModelGuidanceResponse | null | undefined,
): string {
  const rules = (guidance?.rules ?? []).filter(
    (rule) =>
      rule.provider.toLowerCase() === provider &&
      (rule.level === 'recommended' || rule.level === 'suggested'),
  );
  const best = rules.find((rule) => rule.level === 'recommended') ?? rules[0];
  return best ? `${provider}:${best.model_prefix}` : '';
}

/**
 * Pick the Patrol model from the connected provider's list: the best guided
 * level wins, and among equals the shortest id, which is the undated alias
 * when the provider lists both (claude-haiku-4-5 before its dated build).
 */
export function pickPatrolSetupModel(
  provider: PatrolSetupProvider,
  models: Pick<ModelInfo, 'id' | 'provider'>[],
  guidance: PatrolModelGuidanceResponse | null | undefined,
): string | null {
  const own = models.filter(
    (model) => (model.provider?.trim() || splitModelRoute(model.id).provider) === provider,
  );
  const annotations = resolvePatrolModelGuidance(own, guidance);
  const ranked = getPatrolGuidedModelIds(annotations);
  if (ranked.length === 0) return null;
  const topLevel = annotations.get(ranked[0])?.level;
  const top = ranked.filter((id) => annotations.get(id)?.level === topLevel);
  return [...top].sort((a, b) => a.length - b.length || a.localeCompare(b))[0] ?? null;
}

/**
 * The schedule the setup saves with the model: a per-token model on the
 * untouched 6-hour default gets the cost model's slower schedule, exactly as
 * picking it on Patrol settings would. A schedule the operator chose stays.
 */
export function getPatrolSetupIntervalAdjust(
  projection: PatrolCostProjection | null | undefined,
  savedIntervalMinutes: number,
): PatrolIntervalAutoAdjust | null {
  return getPatrolIntervalAutoAdjust(projection, {
    currentIntervalMinutes: savedIntervalMinutes,
    savedIntervalMinutes,
  });
}

/** The settings write that selects the Patrol model (and schedule, when adjusted). */
export function buildPatrolSetupModelRequest(
  modelId: string,
  options: { sharedModel: string; intervalAdjust: PatrolIntervalAutoAdjust | null },
): AISettingsUpdateRequest {
  const request: AISettingsUpdateRequest = { patrol_model: modelId };
  // A fresh install has no shared model either; use the same one so the
  // provider the operator just connected also serves reports and other AI calls.
  if (!options.sharedModel.trim()) request.model = modelId;
  if (options.intervalAdjust) request.patrol_interval_minutes = options.intervalAdjust.to;
  return request;
}

function intervalEstimateUSD(projection: PatrolCostProjection, minutes: number): number | null {
  const match = projection.interval_estimates?.find((entry) => entry.interval_minutes === minutes);
  return match ? match.projected_30d_usd : null;
}

/**
 * One sentence for what Patrol will cost on this route, at the schedule the
 * setup will save. Empty until the preview loads.
 */
export function getPatrolSetupCostLine(
  projection: PatrolCostProjection | null | undefined,
  savedIntervalMinutes: number = PATROL_COST_DEFAULT_INTERVAL_MINUTES,
): string {
  if (!projection) return '';
  if (!projection.pricing_known) {
    return 'Pulse has no price on file for this model, so check the provider’s pricing page.';
  }
  if (!projection.billed_per_token) {
    return 'No per-token bill: Patrol runs on your own hardware.';
  }
  const adjust = getPatrolSetupIntervalAdjust(projection, savedIntervalMinutes);
  const minutes = adjust ? adjust.to : projection.interval_minutes;
  const usd = intervalEstimateUSD(projection, minutes) ?? projection.projected_30d_usd;
  const schedule = formatPatrolIntervalPhrase(minutes);
  const slowed = adjust
    ? ' Pulse slows the schedule from every 6 hours to keep the bill down.'
    : '';
  return `About ${formatPatrolCostUSD(usd)} a month on your provider bill, checking ${schedule}. Investigating new findings adds to this.${slowed}`;
}

export interface PatrolSetupCheckVerdict {
  ready: boolean;
  headline: string;
  detail: string;
}

/** Plain-language verdict for the Patrol model check the setup runs. */
export function getPatrolSetupCheckVerdict(
  result: PatrolModelReadinessSnapshot | null | undefined,
): PatrolSetupCheckVerdict {
  if (!result) {
    return {
      ready: false,
      headline: 'The Patrol model check did not finish',
      detail: 'Check again. If it keeps stopping, open Provider & Models to test the provider.',
    };
  }
  if (result.max_verified_mode === 'approval') {
    return {
      ready: true,
      headline: 'Patrol is ready',
      detail: 'The model passed Patrol’s check for Watch only and Ask first.',
    };
  }
  if (result.max_verified_mode === 'monitor') {
    return {
      ready: true,
      headline: 'Patrol is ready to watch',
      detail:
        'The model passed Patrol’s check for Watch only. Investigations need a model that also passes for Ask first.',
    };
  }
  const detail = [result.summary, result.recommendation]
    .map((part) => part?.trim())
    .filter(Boolean)
    .join(' ');
  return {
    ready: false,
    headline: 'The model did not pass Patrol’s check',
    detail: detail || 'Choose another model on Provider & Models and check again.',
  };
}

/**
 * Why the provider refused the connection, from the same provider test
 * Provider & Models runs, or '' when it answered. A rejected key is fixed
 * right here, so say that instead of pointing at the settings page.
 */
export function getPatrolSetupProviderFailure(
  providerTitle: string,
  result: AIProviderTestResult | null | undefined,
): string {
  if (!result || result.success) return '';
  // The test probes the provider's default model. A model-level cause
  // (model_unavailable and friends) means the provider answered; setup is
  // about to pick its own model, and the model list says what is missing.
  const cause = result.cause?.trim();
  if (cause && !cause.startsWith('provider_')) return '';
  const raw = (result.summary || result.message || '').trim();
  const summary = raw && !/[.!?]$/.test(raw) ? `${raw}.` : raw;
  const lead = `${providerTitle} did not accept the connection${summary ? `: ${summary}` : '.'}`;
  if (result.cause === 'provider_auth') return `${lead} Check the API key and connect again.`;
  const next = result.recommendation?.trim();
  return next ? `${lead} ${next}` : lead;
}

/** Before connecting Ollama: the model has to be on the server already. */
export function getPatrolSetupOllamaHint(suggestedRoute: string): string {
  const model = splitModelRoute(suggestedRoute).model || 'qwen3:8b';
  return `Install ${model} on the Ollama server first: ollama pull ${model}. It is about a 5 GB download and needs 8 GB of free memory.`;
}

/** What to say when the connected provider lists no model Pulse can pick. */
export function getPatrolSetupNoModelMessage(
  provider: PatrolSetupProvider,
  suggestedRoute: string,
): string {
  if (provider === 'ollama') {
    const model = splitModelRoute(suggestedRoute).model || 'qwen3:8b';
    return `Pulse reached Ollama, but ${model} is not installed there. Run "ollama pull ${model}" on the Ollama server, then try again. It is about a 5 GB download and needs 8 GB of free memory.`;
  }
  return 'Pulse connected, but none of this provider’s models has a Patrol suggestion. Choose a model on Provider & Models.';
}
