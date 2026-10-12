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

vi.mock('@/api/aiChat', async (importOriginal) => ({
  ...((await importOriginal()) as object),
  AIChatAPI: {
    recordWorkflowPromptActivity: recordWorkflowPromptActivityMock,
  },
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

  it('bounds manual sync UI state with a generation-aware timeout', () => {
    const manualRefreshStart = patrolIntelligenceStateSource.indexOf(
      'async function handleRefreshPatrol()',
    );
    const manualRefreshBody = patrolIntelligenceStateSource.slice(
      manualRefreshStart,
      patrolIntelligenceStateSource.indexOf(
        'async function consumeRoutePatrolControlStarterHandoff()',
        manualRefreshStart,
      ),
    );

    expect(PATROL_MANUAL_SYNC_TIMEOUT_MS).toBe(5000);
    expect(patrolIntelligenceStateSource).toContain('let manualRefreshRequestId = 0;');
    expect(manualRefreshBody).toContain('const requestId = ++manualRefreshRequestId;');
    expect(manualRefreshBody).toContain('}, PATROL_MANUAL_SYNC_TIMEOUT_MS);');
    expect(manualRefreshBody).toContain('if (requestId === manualRefreshRequestId) {');
    expect(manualRefreshBody).toContain('setIsManualRefreshRunning(false);');
    expect(manualRefreshBody).toContain('clearManualRefreshTimeout();');
  });

  it('ignores load results that a newer background load superseded', () => {
    expect(patrolIntelligenceStateSource).toContain('let refreshRequestId = 0;');
    expect(patrolIntelligenceStateSource).toContain('const requestId = ++refreshRequestId;');
    expect(patrolIntelligenceStateSource).toContain('if (requestId === refreshRequestId) {');
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

  it('fails soft when Patrol data refreshes reject', () => {
    expect(patrolIntelligenceStateSource).toContain('const [patrolLoadError, setPatrolLoadError]');
    expect(patrolIntelligenceStateSource).toContain('rememberPatrolLoadError');
    expect(patrolIntelligenceStateSource).toContain('Promise.allSettled([');
    expect(patrolIntelligenceStateSource).toContain(
      "logger.debug('[Patrol] Failed to refresh Patrol data'",
    );
    expect(patrolIntelligenceStateSource).toContain('patrolLoadError,');
  });

  it('keeps manual status sync bounded to visible Patrol reads', () => {
    const loadAllDataStart = patrolIntelligenceStateSource.indexOf('async function loadAllData()');
    const loadAllDataEnd = patrolIntelligenceStateSource.indexOf(
      'async function loadVisiblePatrolData()',
      loadAllDataStart,
    );
    const loadAllDataBody = patrolIntelligenceStateSource.slice(loadAllDataStart, loadAllDataEnd);
    const manualRefreshStart = patrolIntelligenceStateSource.indexOf(
      'async function handleRefreshPatrol()',
    );
    const manualRefreshEnd = patrolIntelligenceStateSource.indexOf(
      'async function consumeRoutePatrolControlStarterHandoff()',
      manualRefreshStart,
    );
    const manualRefreshBody = patrolIntelligenceStateSource.slice(
      manualRefreshStart,
      manualRefreshEnd,
    );

    expect(patrolIntelligenceStateSource).toContain('async function loadVisiblePatrolData()');
    expect(patrolIntelligenceStateSource).toContain('refetchPatrolStatus()');
    expect(patrolIntelligenceStateSource).toContain(
      'aiIntelligenceStore.loadPatrolFindings({ includeResolved: true })',
    );
    expect(patrolIntelligenceStateSource).toContain('aiIntelligenceStore.loadPendingApprovals()');
    expect(patrolIntelligenceStateSource).toContain(
      'patrolRunHistory.refetch({ background: true })',
    );
    expect(patrolIntelligenceStateSource).toContain(
      'function loadSupportingPatrolDataInBackground()',
    );
    expect(loadAllDataBody).not.toContain('PATROL_MANUAL_SYNC_TIMEOUT_MS');
    expect(manualRefreshBody).toContain('await loadVisiblePatrolData();');
    expect(manualRefreshBody).toContain('loadSupportingPatrolDataInBackground();');
    expect(manualRefreshBody).toContain('PATROL_MANUAL_SYNC_TIMEOUT_MS');
    expect(manualRefreshBody).not.toContain('await loadAllData();');
  });

  it('keeps MCP readiness out of first-party Patrol workflow state', () => {
    expect(patrolIntelligenceStateSource).not.toContain('agentCapabilitiesManifest');
    expect(patrolIntelligenceStateSource).not.toContain('fetchAgentCapabilitiesManifest');
    expect(patrolIntelligenceStateSource).not.toContain('getAgentMCPOperationsLoopReadiness');
    expect(patrolIntelligenceStateSource).not.toContain('mcpOperationsLoopReadiness');
  });

  it('keeps legacy operations-loop status out of Patrol workspace state', () => {
    expect(patrolIntelligenceStateSource).not.toContain('fetchAgentOperationsLoopStatus');
    expect(patrolIntelligenceStateSource).not.toContain('AgentOperationsLoopStatus');
    expect(patrolIntelligenceStateSource).not.toContain('const [patrolWorkStatus');
    expect(patrolIntelligenceStateSource).not.toContain('const [patrolWorkStatusChecked');
    expect(patrolIntelligenceStateSource).not.toContain('loadPatrolWorkStatus');
    expect(patrolIntelligenceStateSource).not.toContain('patrolWorkStatus,');
    expect(patrolIntelligenceStateSource).not.toContain('patrolWorkStatusChecked,');
  });

  it('consumes route-backed Patrol mode starters before the initial status load', () => {
    expect(patrolIntelligenceStateSource).toContain("typeof window === 'undefined'");
    expect(patrolIntelligenceStateSource).toContain('window.location.search');
    expect(patrolIntelligenceStateSource).toContain('window.history.replaceState');
    expect(patrolIntelligenceStateSource).toContain('parsePatrolControlStarter');
    expect(patrolIntelligenceStateSource).toContain('PATROL_CONTROL_STARTER');
    expect(patrolIntelligenceStateSource).toContain('PATROL_CONTROL_STARTER_QUERY_PARAM');
    expect(patrolIntelligenceStateSource).toContain('recordPatrolControlStarterActivity()');
    expect(patrolIntelligenceStateSource).toContain(
      'await consumeRoutePatrolControlStarterHandoff();',
    );
  });

  it('records route-backed Patrol mode starters with the shared content-free activity route', async () => {
    recordWorkflowPromptActivityMock.mockResolvedValueOnce(undefined);

    await recordPatrolControlStarterActivity();

    expect(recordWorkflowPromptActivityMock).toHaveBeenCalledWith({
      name: 'pulse_operations_loop',
      surface: 'patrol_control',
    });
  });

  it('records successful direct Patrol mode changes as first-party starters', () => {
    expect(patrolIntelligenceStateSource).toContain('const controlLocked = autoFixLocked();');
    expect(patrolIntelligenceStateSource).toContain('const shouldRecordPatrolControlStarter');
    expect(patrolIntelligenceStateSource).toContain('!controlLocked &&');
    expect(patrolIntelligenceStateSource).toContain('await updatePatrolAutonomySettings({');
    expect(patrolIntelligenceStateSource).toContain('if (shouldRecordPatrolControlStarter) {');
    expect(patrolIntelligenceStateSource).toContain('await recordPatrolControlStarterActivity();');
    expect(patrolIntelligenceStateSource).toContain('await loadVisiblePatrolData();');
    expect(patrolIntelligenceStateSource).not.toContain('await loadPatrolWorkStatus();');
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
    expect(patrolIntelligenceStateSource).toContain(
      'const blockedCause = createMemo(() => patrolStatus()?.blocked_cause);',
    );
  });
});
