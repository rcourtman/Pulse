import { describe, expect, it } from 'vitest';

import type {
  PatrolCostProjection,
  PatrolModelGuidanceResponse,
  PatrolModelReadinessSnapshot,
} from '@/types/ai';
import {
  buildPatrolSetupConnectRequest,
  buildPatrolSetupModelRequest,
  getPatrolSetupCheckVerdict,
  getPatrolSetupCostLine,
  getPatrolSetupInitialProvider,
  getPatrolSetupIntervalAdjust,
  getPatrolSetupNoModelMessage,
  getPatrolSetupOllamaHint,
  getPatrolSetupProviderFailure,
  getPatrolSetupSuggestedRoute,
  PATROL_SETUP_PROVIDER_OPTIONS,
  pickPatrolSetupModel,
} from '../patrolSetupModel';

const guidance: PatrolModelGuidanceResponse = {
  rules: [
    {
      provider: 'ollama',
      model_prefix: 'qwen3:8b',
      model_exact: true,
      level: 'recommended',
      reason: 'r',
    },
    {
      provider: 'gemini',
      model_prefix: 'gemini-',
      exclude: ['!flash-lite'],
      level: 'caution',
      reason: 'c',
    },
    {
      provider: 'gemini',
      model_prefix: 'gemini-2.5-flash',
      exclude: ['lite', 'image', 'live'],
      level: 'suggested',
      reason: 's',
    },
    { provider: 'anthropic', model_prefix: 'claude-haiku-4-5', level: 'suggested', reason: 's' },
  ],
};

const projection = (overrides: Partial<PatrolCostProjection> = {}): PatrolCostProjection =>
  ({
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    model_route: 'anthropic:claude-haiku-4-5',
    billed_per_token: true,
    pricing_known: true,
    interval_minutes: 360,
    projected_30d_usd: 15.24,
    recommended_interval_minutes: 720,
    interval_estimates: [
      { interval_minutes: 360, projected_30d_usd: 15.24 },
      { interval_minutes: 720, projected_30d_usd: 7.62 },
    ],
    ...overrides,
  }) as PatrolCostProjection;

const readiness = (
  overrides: Partial<PatrolModelReadinessSnapshot>,
): PatrolModelReadinessSnapshot =>
  ({ status: 'fail', summary: '', ...overrides }) as PatrolModelReadinessSnapshot;

describe('patrolSetupModel', () => {
  it('offers the two routes most paid installs use and leaves the rest to Provider & Models', () => {
    expect(PATROL_SETUP_PROVIDER_OPTIONS.map((option) => option.value)).toEqual([
      'anthropic',
      'openai',
      'gemini',
      'deepseek',
      'openrouter',
      'ollama',
    ]);
  });

  it('starts on the provider readiness names, else Anthropic', () => {
    expect(getPatrolSetupInitialProvider('gemini')).toBe('gemini');
    expect(getPatrolSetupInitialProvider(' Ollama ')).toBe('ollama');
    expect(getPatrolSetupInitialProvider('groq')).toBe('anthropic');
    expect(getPatrolSetupInitialProvider(undefined)).toBe('anthropic');
  });

  it('builds the connect write only once the required field is filled', () => {
    expect(buildPatrolSetupConnectRequest('anthropic', { apiKey: '  ', ollamaUrl: '' })).toBeNull();
    expect(
      buildPatrolSetupConnectRequest('gemini', { apiKey: ' AIza-test ', ollamaUrl: '' }),
    ).toEqual({
      enabled: true,
      gemini_api_key: 'AIza-test',
    });
    expect(buildPatrolSetupConnectRequest('openrouter', { apiKey: 'k', ollamaUrl: '' })).toEqual({
      enabled: true,
      openrouter_api_key: 'k',
    });
    expect(
      buildPatrolSetupConnectRequest('ollama', { apiKey: 'ignored', ollamaUrl: '' }),
    ).toBeNull();
    expect(
      buildPatrolSetupConnectRequest('ollama', {
        apiKey: 'ignored',
        ollamaUrl: ' http://ollama:11434 ',
      }),
    ).toEqual({ enabled: true, ollama_base_url: 'http://ollama:11434' });
  });

  it('prices the guided route before anything is saved, recommended before suggested', () => {
    expect(getPatrolSetupSuggestedRoute('ollama', guidance)).toBe('ollama:qwen3:8b');
    expect(getPatrolSetupSuggestedRoute('gemini', guidance)).toBe('gemini:gemini-2.5-flash');
    expect(getPatrolSetupSuggestedRoute('anthropic', guidance)).toBe('anthropic:claude-haiku-4-5');
    expect(getPatrolSetupSuggestedRoute('openai', guidance)).toBe('');
    expect(getPatrolSetupSuggestedRoute('anthropic', null)).toBe('');
  });

  it('picks the guided model from the connected provider, alias before dated build', () => {
    const models = [
      { id: 'anthropic:claude-opus-4-1', provider: 'anthropic' },
      { id: 'anthropic:claude-haiku-4-5-20251001', provider: 'anthropic' },
      { id: 'anthropic:claude-haiku-4-5', provider: 'anthropic' },
      { id: 'ollama:qwen3:8b', provider: 'ollama' },
    ];
    expect(pickPatrolSetupModel('anthropic', models, guidance)).toBe('anthropic:claude-haiku-4-5');
    // Another provider's guided model is never picked for this one.
    expect(pickPatrolSetupModel('openai', models, guidance)).toBeNull();
  });

  it('never picks a caution model and returns null when nothing is guided', () => {
    const gemini = [
      { id: 'gemini:gemini-2.5-flash-lite', provider: 'gemini' },
      { id: 'gemini:gemini-2.5-flash', provider: 'gemini' },
    ];
    expect(pickPatrolSetupModel('gemini', gemini, guidance)).toBe('gemini:gemini-2.5-flash');
    expect(pickPatrolSetupModel('gemini', [gemini[0]], guidance)).toBeNull();
    expect(pickPatrolSetupModel('ollama', [], guidance)).toBeNull();
  });

  it('reads the provider from the id when the server omits it', () => {
    expect(pickPatrolSetupModel('ollama', [{ id: 'ollama:qwen3:8b' }], guidance)).toBe(
      'ollama:qwen3:8b',
    );
  });

  it('slows only the untouched 6-hour default for a per-token model', () => {
    expect(getPatrolSetupIntervalAdjust(projection(), 360)?.to).toBe(720);
    expect(getPatrolSetupIntervalAdjust(projection(), 60)).toBeNull();
    expect(getPatrolSetupIntervalAdjust(projection({ billed_per_token: false }), 360)).toBeNull();
  });

  it('gives the Assistant the same model only when it has none', () => {
    expect(
      buildPatrolSetupModelRequest('ollama:qwen3:8b', { sharedModel: '', intervalAdjust: null }),
    ).toEqual({ patrol_model: 'ollama:qwen3:8b', model: 'ollama:qwen3:8b' });
    expect(
      buildPatrolSetupModelRequest('anthropic:claude-haiku-4-5', {
        sharedModel: 'openai:gpt-5',
        intervalAdjust: getPatrolSetupIntervalAdjust(projection(), 360),
      }),
    ).toEqual({ patrol_model: 'anthropic:claude-haiku-4-5', patrol_interval_minutes: 720 });
  });

  it('states the monthly cost at the schedule the setup will save', () => {
    expect(getPatrolSetupCostLine(null)).toBe('');
    expect(getPatrolSetupCostLine(projection(), 360)).toBe(
      'About $7.62 a month on your provider bill, checking every 12 hours. Investigating new findings adds to this. Pulse slows the schedule from every 6 hours to keep the bill down.',
    );
    expect(getPatrolSetupCostLine(projection({ recommended_interval_minutes: 360 }), 360)).toBe(
      'About $15.24 a month on your provider bill, checking every 6 hours. Investigating new findings adds to this.',
    );
    expect(getPatrolSetupCostLine(projection({ billed_per_token: false }))).toBe(
      'No per-token bill: Patrol runs on your own hardware.',
    );
    expect(getPatrolSetupCostLine(projection({ pricing_known: false }))).toMatch(
      /no price on file/,
    );
  });

  it('calls a check ready only when it verified a Patrol mode', () => {
    expect(getPatrolSetupCheckVerdict(readiness({ max_verified_mode: 'approval' }))).toMatchObject({
      ready: true,
      headline: 'Patrol is ready',
    });
    const watchOnly = getPatrolSetupCheckVerdict(readiness({ max_verified_mode: 'monitor' }));
    expect(watchOnly.ready).toBe(true);
    expect(watchOnly.detail).toMatch(/Investigations need a model that also passes for Ask first/);
    const failed = getPatrolSetupCheckVerdict(
      readiness({ summary: 'No tool call.', recommendation: 'Pick another model.' }),
    );
    expect(failed).toEqual({
      ready: false,
      headline: 'The model did not pass Patrol’s check',
      detail: 'No tool call. Pick another model.',
    });
    expect(getPatrolSetupCheckVerdict(null).ready).toBe(false);
  });

  it('turns a refused provider test into a fix-it-here sentence', () => {
    expect(
      getPatrolSetupProviderFailure('Anthropic', {
        success: true,
        message: '',
        provider: 'anthropic',
      }),
    ).toBe('');
    expect(getPatrolSetupProviderFailure('Anthropic', null)).toBe('');
    // The provider answered; only its default model is missing, which setup replaces.
    expect(
      getPatrolSetupProviderFailure('Ollama', {
        success: false,
        message: 'Selected model unavailable',
        provider: 'ollama',
        cause: 'model_unavailable',
      }),
    ).toBe('');
    expect(
      getPatrolSetupProviderFailure('Anthropic', {
        success: false,
        message: 'Provider authentication issue',
        provider: 'anthropic',
        cause: 'provider_auth',
        summary: 'The provider rejected the configured credentials.',
        recommendation: 'Go to Provider & Models.',
      }),
    ).toBe(
      'Anthropic did not accept the connection: The provider rejected the configured credentials. Check the API key and connect again.',
    );
    expect(
      getPatrolSetupProviderFailure('Ollama', {
        success: false,
        message: 'Connection refused',
        provider: 'ollama',
        cause: 'provider_connection',
        recommendation: 'Check the server URL.',
      }),
    ).toBe('Ollama did not accept the connection: Connection refused. Check the server URL.');
  });

  it('tells an Ollama user to pull the model, before and after connecting', () => {
    expect(getPatrolSetupOllamaHint('ollama:qwen3:8b')).toContain('ollama pull qwen3:8b');
    expect(getPatrolSetupNoModelMessage('ollama', 'ollama:qwen3:8b')).toContain(
      'Run "ollama pull qwen3:8b" on the Ollama server',
    );
    expect(getPatrolSetupNoModelMessage('anthropic', '')).toMatch(/Provider & Models/);
  });
});
