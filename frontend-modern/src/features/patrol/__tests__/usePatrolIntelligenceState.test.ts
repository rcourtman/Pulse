import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  PATROL_MANUAL_SYNC_TIMEOUT_MS,
  PATROL_REFRESH_TIMEOUT_MS,
  buildPatrolSettingsReadinessFailure,
  openPatrolAssistantWorkflowHandoff,
  patrolStartFailureMessage,
  recordPatrolControlStarterActivity,
  recordPatrolWorkflowStarterActivity,
  resolvePatrolAutonomyLevelForSave,
  resolvePatrolAutonomySettingsForSave,
} from '../usePatrolIntelligenceState';
import patrolIntelligenceStateSource from '../usePatrolIntelligenceState.ts?raw';
import type { AIChatContext } from '@/stores/aiChat';
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

  it('bounds refresh UI state with a generation-aware timeout', () => {
    expect(PATROL_REFRESH_TIMEOUT_MS).toBe(15000);
    expect(PATROL_MANUAL_SYNC_TIMEOUT_MS).toBe(5000);
    expect(patrolIntelligenceStateSource).toContain('let refreshRequestId = 0;');
    expect(patrolIntelligenceStateSource).toContain('let manualRefreshRequestId = 0;');
    expect(patrolIntelligenceStateSource).toContain('const requestId = ++refreshRequestId;');
    expect(patrolIntelligenceStateSource).toContain('if (requestId === refreshRequestId) {');
    expect(patrolIntelligenceStateSource).toContain('setIsRefreshing(false);');
    expect(patrolIntelligenceStateSource).toContain('setIsManualRefreshRunning(false);');
    expect(patrolIntelligenceStateSource).toContain('handleRefreshPatrol');
    expect(patrolIntelligenceStateSource).toContain('clearRefreshTimeout();');
    expect(patrolIntelligenceStateSource).toContain('clearManualRefreshTimeout();');
    expect(patrolIntelligenceStateSource).toContain('PATROL_MANUAL_SYNC_TIMEOUT_MS');
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
    expect(patrolIntelligenceStateSource).toContain('setInitialSurfaceReady(true);');
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
    expect(patrolIntelligenceStateSource).toContain('aiIntelligenceStore.loadPatrolFindings()');
    expect(patrolIntelligenceStateSource).toContain('aiIntelligenceStore.loadPendingApprovals()');
    expect(patrolIntelligenceStateSource).toContain(
      'patrolRunHistory.refetch({ background: true })',
    );
    expect(patrolIntelligenceStateSource).toContain(
      'function loadSupportingPatrolDataInBackground()',
    );
    expect(loadAllDataBody).toContain('PATROL_REFRESH_TIMEOUT_MS');
    expect(loadAllDataBody).not.toContain('PATROL_MANUAL_SYNC_TIMEOUT_MS');
    expect(manualRefreshBody).toContain('await loadVisiblePatrolData();');
    expect(manualRefreshBody).toContain('loadSupportingPatrolDataInBackground();');
    expect(manualRefreshBody).toContain('PATROL_MANUAL_SYNC_TIMEOUT_MS');
    expect(manualRefreshBody).not.toContain('PATROL_REFRESH_TIMEOUT_MS');
    expect(manualRefreshBody).not.toContain('await loadAllData();');
  });

  it('counts rejected Patrol fixes as governed action decisions', () => {
    expect(patrolIntelligenceStateSource).toContain('const PATROL_GOVERNED_ACTION_OUTCOMES');
    expect(patrolIntelligenceStateSource).toContain("'fix_rejected'");
    expect(patrolIntelligenceStateSource).toContain('patrolGovernedActionCount');
  });

  it('separates generic Patrol runs from issue-backed Patrol work evidence', () => {
    expect(patrolIntelligenceStateSource).toContain('const patrolWorkIssueEvidenceCount');
    expect(patrolIntelligenceStateSource).toContain("finding.status === 'active'");
    expect(patrolIntelligenceStateSource).toContain("finding.status === 'resolved'");
    expect(patrolIntelligenceStateSource).toContain('status?.findings_count');
    expect(patrolIntelligenceStateSource).toContain('status?.trust?.fix_verified');
    expect(patrolIntelligenceStateSource).toContain('patrolWorkIssueEvidenceCount,');
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

  it('records direct Patrol workflow starters with the shared content-free activity route', () => {
    recordWorkflowPromptActivityMock.mockResolvedValueOnce(undefined);

    recordPatrolWorkflowStarterActivity();

    expect(recordWorkflowPromptActivityMock).toHaveBeenCalledWith({
      name: 'pulse_operations_loop',
      surface: 'pulse_patrol',
    });
    expect(patrolIntelligenceStateSource).toContain('recordPatrolWorkflowStarterActivity,');
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

  it('keeps direct Patrol starter recording non-blocking when the marker route fails', async () => {
    const error = new Error('offline');
    recordWorkflowPromptActivityMock.mockRejectedValueOnce(error);

    recordPatrolWorkflowStarterActivity();
    await Promise.resolve();

    expect(loggerDebugMock).toHaveBeenCalledWith(
      '[Patrol] Failed to record Patrol workflow starter',
      error,
    );
  });

  it('records a Patrol starter before opening the Assistant workflow handoff', () => {
    const callOrder: string[] = [];
    const recordStarterActivity = vi.fn(() => callOrder.push('record'));
    const openAssistant = vi.fn((context: AIChatContext) => {
      callOrder.push('open');
      expect(context).toMatchObject({
        targetType: 'storage',
        targetId: 'storage-1',
        findingId: 'finding-1',
        autonomousMode: false,
        preferredWorkflowPromptName: 'pulse_operations_loop',
      });
      expect(context.handoffContext).toBe('Scoped Patrol context');
    });

    openPatrolAssistantWorkflowHandoff(
      {
        context: {
          targetType: 'storage',
          targetId: 'storage-1',
          findingId: 'finding-1',
          autonomousMode: false,
          handoffContext: 'Scoped Patrol context',
          preferredWorkflowPromptName: 'legacy_prompt',
        },
      },
      {
        recordStarterActivity,
        openAssistant,
      },
    );

    expect(recordStarterActivity).toHaveBeenCalledWith();
    expect(openAssistant).toHaveBeenCalledTimes(1);
    expect(callOrder).toEqual(['record', 'open']);
  });

  describe('resolvePatrolAutonomyLevelForSave', () => {
    it('clamps stale paid autonomy to monitor when governed fixes are locked', () => {
      expect(resolvePatrolAutonomyLevelForSave('full', true, true)).toBe('monitor');
      expect(resolvePatrolAutonomyLevelForSave('assisted', false, true)).toBe('monitor');
      expect(resolvePatrolAutonomyLevelForSave('approval', false, true)).toBe('monitor');
    });

    it('preserves paid autonomy choices when governed fixes are available', () => {
      expect(resolvePatrolAutonomyLevelForSave('assisted', false, false)).toBe('assisted');
      expect(resolvePatrolAutonomyLevelForSave('assisted', true, false)).toBe('assisted');
      expect(resolvePatrolAutonomyLevelForSave('full', true, false)).toBe('full');
      expect(resolvePatrolAutonomyLevelForSave('full', false, false)).toBe('assisted');
      expect(resolvePatrolAutonomyLevelForSave('approval', false, false)).toBe('approval');
    });
  });

  describe('resolvePatrolAutonomySettingsForSave', () => {
    it('clears stale full-mode state when governed fixes are locked', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'full',
          fullModeUnlocked: true,
          autoFixLocked: true,
        }),
      ).toEqual({ autonomyLevel: 'monitor', fullModeUnlocked: false });
    });

    it('does not carry full-mode state into non-remediation modes', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'monitor',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'monitor', fullModeUnlocked: false });

      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'approval',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'approval', fullModeUnlocked: false });
    });

    it('keeps full control tied to the explicit full selection', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'assisted',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'assisted', fullModeUnlocked: false });

      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'full',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'full', fullModeUnlocked: true });

      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'full',
          fullModeUnlocked: false,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'assisted', fullModeUnlocked: false });
    });
  });

  describe('buildPatrolSettingsReadinessFailure', () => {
    it('ignores settings snapshots that do not block Patrol readiness', () => {
      expect(
        buildPatrolSettingsReadinessFailure({
          settings: settingsWithReadiness({
            status: 'warning',
            ready: true,
            summary: 'Patrol can run with reduced confidence.',
            checks: [],
          }),
        }),
      ).toBeNull();
    });

    it('builds a saved configuration issue from a not-ready settings response', () => {
      expect(
        buildPatrolSettingsReadinessFailure({
          settings: settingsWithReadiness({
            status: 'not_ready',
            ready: false,
            cause: 'model_unsupported_tools',
            summary: 'The selected model cannot run Patrol tools.',
            provider: 'ollama',
            model: 'ollama:deepseek-r1:7b',
            checks: [],
          }),
          autonomyLevel: 'monitor',
          fullModeUnlocked: false,
          investigationBudget: 15,
          investigationTimeoutSec: 300,
          runtimeState: 'blocked',
          blockedReason: 'Connect a tool-capable Patrol model.',
        }),
      ).toEqual({
        message: 'The selected model cannot run Patrol tools.',
        code: 'patrol_readiness_not_ready',
        status: 409,
        saved: true,
        details: {
          status: 'not_ready',
          cause: 'model_unsupported_tools',
          summary: 'The selected model cannot run Patrol tools.',
          provider: 'ollama',
          model: 'ollama:deepseek-r1:7b',
        },
        autonomyLevel: 'monitor',
        fullModeUnlocked: false,
        investigationBudget: 15,
        investigationTimeoutSec: 300,
        readiness: {
          status: 'not_ready',
          cause: 'model_unsupported_tools',
          summary: 'The selected model cannot run Patrol tools.',
          provider: 'ollama',
          model: 'ollama:deepseek-r1:7b',
        },
        runtimeState: 'blocked',
        blockedReason: 'Connect a tool-capable Patrol model.',
        blockedCause: undefined,
      });
    });
  });
});
