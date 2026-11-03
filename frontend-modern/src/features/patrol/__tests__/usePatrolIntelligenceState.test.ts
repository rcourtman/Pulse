import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  PATROL_MANUAL_SYNC_TIMEOUT_MS,
  getPatrolSavedReadinessWarning,
  patrolStartFailureMessage,
  recordPatrolControlStarterActivity,
  resolvePatrolBlockedActionCause,
} from '../usePatrolIntelligenceState';
import patrolIntelligenceStateSource from '../usePatrolIntelligenceState.ts?raw';
import type { AISettings, PatrolReadiness } from '@/types/ai';

const recordWorkflowPromptActivityMock = vi.hoisted(() => vi.fn());
const loggerDebugMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/workflowPrompts', async (importOriginal) => ({
  ...((await importOriginal()) as object),
  recordWorkflowPromptActivity: recordWorkflowPromptActivityMock,
}));

vi.mock('@/utils/logger', async (importOriginal) => ({
  ...((await importOriginal()) as object),
  logger: {
    debug: loggerDebugMock,
  },
}));

const settingsWithReadiness = (patrolReadiness: PatrolReadiness): AISettings => ({
  enabled: true,
  model: 'ollama:llama3',
  configured: true,
  custom_context: '',
  auth_method: 'api_key',
  oauth_connected: false,
  anthropic_configured: false,
  openai_configured: false,
  openrouter_configured: false,
  deepseek_configured: false,
  gemini_configured: false,
  ollama_configured: true,
  ollama_base_url: 'http://127.0.0.1:11434',
  ollama_keep_alive: '30s',
  configured_providers: ['ollama'],
  patrol_readiness: patrolReadiness,
});

describe('usePatrolIntelligenceState', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('bounds manual sync UI state with a timeout', () => {
    expect(PATROL_MANUAL_SYNC_TIMEOUT_MS).toBe(5000);
  });

  it('refuses paid Patrol modes in the state owner while governed fixes are locked', () => {
    // The header disables paid choices too; these guards keep the state owner
    // from starting a paid save if a caller reaches it anyway.
    expect(patrolIntelligenceStateSource).toContain(
      "if (controlLocked && level !== 'monitor') return;",
    );
    expect(patrolIntelligenceStateSource).toContain(
      'if (isUpdatingAutonomy() || autoFixLocked()) return;',
    );
  });

  it('shows the Pro default only when the server says it applies', () => {
    // A missing or non-boolean flag from an older server must not show the
    // notice, so the state owner accepts only an explicit true.
    expect(patrolIntelligenceStateSource).toContain(
      'setDefaultModeApplied(settings.default_mode_applied === true);',
    );
    expect(patrolIntelligenceStateSource).toContain('defaultModeApplied,');
  });

  it('keeps browser/network start failures distinct from backend rejections', () => {
    expect(patrolStartFailureMessage(new TypeError('Failed to fetch'))).toBe(
      'Could not reach Pulse to start Patrol: Failed to fetch',
    );
    expect(
      patrolStartFailureMessage(
        Object.assign(new Error('Patrol is already running.'), {
          code: 'patrol_already_running',
          status: 409,
        }),
      ),
    ).toBe('Patrol is already running.');
  });

  it('records route-backed Patrol mode starters with the shared content-free activity route', async () => {
    recordWorkflowPromptActivityMock.mockResolvedValueOnce(undefined);

    await recordPatrolControlStarterActivity();

    expect(recordWorkflowPromptActivityMock).toHaveBeenCalledWith({
      name: 'pulse_operations_loop',
      surface: 'patrol_control',
    });
  });

  it('keeps Patrol mode starter recording non-blocking when the marker route fails', async () => {
    const error = new Error('offline');
    recordWorkflowPromptActivityMock.mockRejectedValueOnce(error);

    await expect(recordPatrolControlStarterActivity()).resolves.toBeUndefined();

    expect(loggerDebugMock).toHaveBeenCalledWith(
      '[Patrol mode handoff] Failed to record Patrol workflow starter',
      error,
    );
  });

  describe('getPatrolSavedReadinessWarning', () => {
    it('stays quiet when the saved settings do not block Patrol readiness', () => {
      expect(getPatrolSavedReadinessWarning(null)).toBeNull();
      expect(
        getPatrolSavedReadinessWarning(
          settingsWithReadiness({
            status: 'warning',
            ready: true,
            summary: 'Patrol can run with reduced confidence.',
            checks: [],
          }),
        ),
      ).toBeNull();
    });

    it('warns that a not-ready Patrol setting was still saved', () => {
      expect(
        getPatrolSavedReadinessWarning(
          settingsWithReadiness({
            status: 'not_ready',
            ready: false,
            cause: 'model_unsupported_tools',
            summary: 'The selected model cannot run Patrol tools.',
            provider: 'ollama',
            model: 'ollama:deepseek-r1:7b',
            checks: [],
          }),
        ),
      ).toBe('Patrol setting was saved, but Patrol is not ready to run.');
    });
  });

  it('lets the runtime block cause drive the blocked-banner action (#1789)', () => {
    expect(resolvePatrolBlockedActionCause('budget_exhausted', 'none')).toBe('budget_exhausted');
    expect(resolvePatrolBlockedActionCause('none', 'model_unsupported_tools')).toBe(
      'model_unsupported_tools',
    );
    expect(resolvePatrolBlockedActionCause(undefined, ' provider_not_configured ')).toBe(
      'provider_not_configured',
    );
    expect(resolvePatrolBlockedActionCause('', '')).toBeUndefined();
  });
});
