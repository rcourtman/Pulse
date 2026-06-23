import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { AIAPI } from '@/api/ai';
import {
  getPatrolAutonomySettings,
  getPatrolRunHistory,
  getPatrolStatus,
  triggerPatrolRun,
  updatePatrolAutonomySettings,
  type PatrolAutonomyLevel,
  type PatrolRunRecord,
  type PatrolRuntimeState,
  type PatrolStatus,
} from '@/api/patrol';
import {
  AIChatAPI,
  type AssistantWorkflowPromptActivitySurface,
  PULSE_OPERATIONS_LOOP_WORKFLOW_PROMPT_NAME,
  PULSE_PATROL_CONTROL_WORKFLOW_PROMPT_SURFACE,
  PULSE_PATROL_WORKFLOW_PROMPT_SURFACE,
} from '@/api/aiChat';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import {
  aiRuntimeSettings,
  loadAIRuntimeModels,
  loadAIRuntimeSettings,
  syncAIRuntimeSettings,
} from '@/stores/aiRuntimeState';
import { aiChatStore, type AIChatContext } from '@/stores/aiChat';
import { notificationStore } from '@/stores/notifications';
import { usePatrolStream } from '@/hooks/usePatrolStream';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import {
  getRuntimeCapabilityBlock,
  hasFeature,
  loadRuntimeCapabilities,
  runtimeCapabilities,
} from '@/stores/license';
import { PATROL_AUTONOMY_FEATURE_KEY } from './patrolAutonomyAvailability';
import type { AISettings } from '@/types/ai';
import {
  hasFindingInvestigationHandoffPointer,
  isPatrolRuntimeFinding,
} from '@/utils/aiFindingPresentation';
import { getCanonicalScopeResourceIds } from '@/utils/patrolFormat';
import { normalizePatrolRuntimeBlockedReason } from '@/utils/patrolRuntimePresentation';
import { logger } from '@/utils/logger';
import {
  parsePatrolControlStarter,
  PATROL_CONTROL_STARTER,
  PATROL_CONTROL_STARTER_QUERY_PARAM,
  PATROL_OPERATIONS_LOOP_STARTER_QUERY_PARAM,
} from '@/routing/resourceLinks';
import {
  buildPatrolAssistantApprovalBriefingInput,
  buildPatrolAssistantFindingHandoffFromUnifiedFinding,
  buildPatrolAssistantProposedFixBriefingInput,
  buildPatrolAssistantProposedFixBriefingInputFromApproval,
  patrolAssistantFindingHandoffRequiresApprovalMode,
  type PatrolAssistantApprovalBriefingInput,
  type PatrolAssistantFindingHandoff,
  type PatrolConfigurationFailureInput,
} from './patrolInvestigationContextModel';

type PatrolTab = 'findings' | 'history';

const PATROL_GOVERNED_ACTION_OUTCOMES = new Set([
  'fix_executed',
  'fix_failed',
  'fix_rejected',
  'fix_verified',
  'fix_verification_failed',
  'fix_verification_unknown',
]);
const PATROL_VERIFIED_OUTCOMES = new Set(['fix_verified']);
const PATROL_OPERATIONS_LOOP_WORKFLOW_PROMPT = PULSE_OPERATIONS_LOOP_WORKFLOW_PROMPT_NAME;

const recordPatrolWorkflowStarterActivityForSurface = async (
  surface: AssistantWorkflowPromptActivitySurface,
  logContext: string,
): Promise<void> => {
  try {
    await AIChatAPI.recordWorkflowPromptActivity({
      name: PULSE_OPERATIONS_LOOP_WORKFLOW_PROMPT_NAME,
      surface,
    });
  } catch (error) {
    logger.debug(`[${logContext}] Failed to record Patrol workflow starter`, error);
  }
};

export const PATROL_REFRESH_TIMEOUT_MS = 15000;
export const PATROL_MANUAL_SYNC_TIMEOUT_MS = 5000;

export function recordPatrolWorkflowStarterActivity(): void {
  void recordPatrolWorkflowStarterActivityForSurface(
    PULSE_PATROL_WORKFLOW_PROMPT_SURFACE,
    'Patrol',
  );
}

export function recordPatrolControlStarterActivity(): Promise<void> {
  return recordPatrolWorkflowStarterActivityForSurface(
    PULSE_PATROL_CONTROL_WORKFLOW_PROMPT_SURFACE,
    'Patrol mode handoff',
  );
}

interface PatrolAssistantWorkflowHandoffOptions {
  recordStarterActivity?: () => void;
  openAssistant?: (context: AIChatContext) => void;
}

export function openPatrolAssistantWorkflowHandoff(
  handoff: PatrolAssistantFindingHandoff,
  options: PatrolAssistantWorkflowHandoffOptions = {},
): void {
  const {
    recordStarterActivity = recordPatrolWorkflowStarterActivity,
    openAssistant = (context) => aiChatStore.open(context),
  } = options;

  recordStarterActivity();
  openAssistant({
    ...handoff.context,
    preferredWorkflowPromptName: PATROL_OPERATIONS_LOOP_WORKFLOW_PROMPT,
  });
}

const patrolErrorMessage = (error: unknown, fallback: string) =>
  error instanceof Error && error.message.trim() ? error.message : fallback;

const buildReadinessDetails = (
  readiness: NonNullable<AISettings['patrol_readiness']>,
): Record<string, string> => {
  const details: Record<string, string> = {
    status: readiness.status,
  };
  if (readiness.cause?.trim()) details.cause = readiness.cause.trim();
  if (readiness.summary?.trim()) details.summary = readiness.summary.trim();
  if (readiness.provider?.trim()) details.provider = readiness.provider.trim();
  if (readiness.model?.trim()) details.model = readiness.model.trim();
  return details;
};

export function resolvePatrolAutonomyLevelForSave(
  level: PatrolAutonomyLevel,
  fullModeUnlocked: boolean,
  autoFixLocked: boolean,
): PatrolAutonomyLevel {
  if (autoFixLocked) return 'monitor';
  if (level === 'full') {
    return fullModeUnlocked ? 'full' : 'assisted';
  }
  return level;
}

export function resolvePatrolAutonomySettingsForSave({
  level,
  fullModeUnlocked,
  autoFixLocked,
}: {
  level: PatrolAutonomyLevel;
  fullModeUnlocked: boolean;
  autoFixLocked: boolean;
}): { autonomyLevel: PatrolAutonomyLevel; fullModeUnlocked: boolean } {
  const canUseFullMode = !autoFixLocked && level === 'full' && fullModeUnlocked;
  const autonomyLevel = resolvePatrolAutonomyLevelForSave(level, canUseFullMode, autoFixLocked);

  return {
    autonomyLevel,
    fullModeUnlocked: autonomyLevel === 'full',
  };
}

export function buildPatrolSettingsReadinessFailure({
  settings,
  message,
  autonomyLevel,
  fullModeUnlocked,
  investigationBudget,
  investigationTimeoutSec,
  runtimeState,
  blockedReason,
  blockedCause,
}: {
  settings: AISettings | null | undefined;
  message?: string;
  autonomyLevel?: string;
  fullModeUnlocked?: boolean;
  investigationBudget?: number;
  investigationTimeoutSec?: number;
  runtimeState?: string;
  blockedReason?: string;
  blockedCause?: string;
}): PatrolConfigurationFailureInput | null {
  const readiness = settings?.patrol_readiness;
  if (!readiness || readiness.status !== 'not_ready') return null;

  return {
    message:
      message || readiness.summary || 'Patrol settings were saved, but Patrol is not ready to run.',
    code: 'patrol_readiness_not_ready',
    status: 409,
    saved: true,
    details: buildReadinessDetails(readiness),
    autonomyLevel,
    fullModeUnlocked,
    investigationBudget,
    investigationTimeoutSec,
    readiness: {
      status: readiness.status,
      cause: readiness.cause,
      summary: readiness.summary,
      provider: readiness.provider,
      model: readiness.model,
    },
    runtimeState,
    blockedReason,
    blockedCause,
  };
}

export function usePatrolIntelligenceState() {
  const [initialSurfaceReady, setInitialSurfaceReady] = createSignal(false);
  const [activeTab, setActiveTab] = createSignal<PatrolTab>('findings');
  const [findingsFilterOverride, setFindingsFilterOverride] = createSignal<
    'all' | 'active' | 'resolved' | 'approvals' | 'attention' | undefined
  >(undefined);
  const [isRefreshing, setIsRefreshing] = createSignal(false);
  const [isManualRefreshRunning, setIsManualRefreshRunning] = createSignal(false);
  const [autonomyLevel, setAutonomyLevel] = createSignal<PatrolAutonomyLevel>('monitor');
  const [isUpdatingAutonomy, setIsUpdatingAutonomy] = createSignal(false);
  const [activityRefreshTrigger, setActivityRefreshTrigger] = createSignal(0);
  const [manualRunRequested, setManualRunRequested] = createSignal(false);
  const [patrolEnabledLocal, setPatrolEnabledLocal] = createSignal<boolean>(true);
  const [liveRunStartedAt, setLiveRunStartedAt] = createSignal('');
  const [investigationBudget, setInvestigationBudget] = createSignal(15);
  const [investigationTimeout, setInvestigationTimeout] = createSignal(300);
  const [fullModeUnlocked, setFullModeUnlocked] = createSignal(false);
  const [isTogglingPatrol, setIsTogglingPatrol] = createSignal(false);
  const [isTriggeringPatrol, setIsTriggeringPatrol] = createSignal(false);
  const [selectedRun, setSelectedRun] = createSignal<PatrolRunRecord | null>(null);
  const [assistantHandoffFindingId, setAssistantHandoffFindingId] = createSignal('');
  const [patrolLoadError, setPatrolLoadError] = createSignal('');

  let safetyTimerRef: ReturnType<typeof setTimeout> | undefined;
  let scrollToFindingTimerRef: ReturnType<typeof setTimeout> | undefined;
  let findingScrollTimerRef: ReturnType<typeof setTimeout> | undefined;
  let refreshTimeoutRef: ReturnType<typeof setTimeout> | undefined;
  let manualRefreshTimeoutRef: ReturnType<typeof setTimeout> | undefined;
  let refreshRequestId = 0;
  let manualRefreshRequestId = 0;
  let refreshInterval: ReturnType<typeof setInterval>;
  let approvalPollInterval: ReturnType<typeof setInterval>;

  const clearSafetyTimer = () => {
    if (safetyTimerRef !== undefined) {
      clearTimeout(safetyTimerRef);
      safetyTimerRef = undefined;
    }
  };

  const clearScrollToFindingTimer = () => {
    if (scrollToFindingTimerRef !== undefined) {
      clearTimeout(scrollToFindingTimerRef);
      scrollToFindingTimerRef = undefined;
    }
  };

  const clearRefreshTimeout = () => {
    if (refreshTimeoutRef !== undefined) {
      clearTimeout(refreshTimeoutRef);
      refreshTimeoutRef = undefined;
    }
  };

  const clearManualRefreshTimeout = () => {
    if (manualRefreshTimeoutRef !== undefined) {
      clearTimeout(manualRefreshTimeoutRef);
      manualRefreshTimeoutRef = undefined;
    }
  };

  const finishRefresh = (requestId: number) => {
    if (requestId !== refreshRequestId) {
      return;
    }
    clearRefreshTimeout();
    setIsRefreshing(false);
  };

  const rememberPatrolLoadError = (error: unknown, fallback: string) => {
    const message = patrolErrorMessage(error, fallback);
    setPatrolLoadError(message);
    logger.debug('[Patrol] Failed to refresh Patrol data', error);
    return message;
  };

  const clearPatrolLoadError = () => {
    if (patrolLoadError()) {
      setPatrolLoadError('');
    }
  };

  const patrolStatusState = createNonSuspendingQuery<PatrolStatus | null, string>({
    source: () => 'patrol-status',
    cacheKey: () => 'patrol-status',
    fetcher: async () => {
      try {
        return await getPatrolStatus();
      } catch {
        return null;
      }
    },
    initialValue: null,
  });
  const patrolStatus = patrolStatusState.value;
  const refetchPatrolStatus = patrolStatusState.refetch;
  const historicalRegressionCount = createMemo(() => {
    const count = patrolStatus()?.trust?.regressed_at_least_once;
    return typeof count === 'number' && Number.isFinite(count) ? Math.max(0, count) : 0;
  });

  const patrolStream = usePatrolStream({
    running: () =>
      patrolEnabledLocal() && ((patrolStatus()?.running ?? false) || manualRunRequested()),
    onStart: () => {
      clearSafetyTimer();
    },
    onComplete: () => {
      setManualRunRequested(false);
      loadAllData();
    },
    onError: () => {
      setManualRunRequested(false);
      loadAllData();
    },
  });

  const autoFixLocked = createMemo(() => !hasFeature(PATROL_AUTONOMY_FEATURE_KEY));
  const autoFixCapabilityBlock = createMemo(() =>
    getRuntimeCapabilityBlock(PATROL_AUTONOMY_FEATURE_KEY),
  );
  const licenseRuntimeIdentity = createMemo(() => runtimeCapabilities()?.runtime);

  const applyPatrolAISettings = (data: AISettings | null | undefined) => {
    setPatrolEnabledLocal(data?.patrol_enabled ?? true);
  };

  createEffect(() => {
    applyPatrolAISettings(aiRuntimeSettings());
  });

  const surfaceSavedPatrolReadinessIssue = (
    settings: AISettings | null | undefined,
    message?: string,
  ) => {
    const failure = buildPatrolSettingsReadinessFailure({
      settings,
      message,
      autonomyLevel: autonomyLevel(),
      fullModeUnlocked: fullModeUnlocked(),
      investigationBudget: investigationBudget(),
      investigationTimeoutSec: investigationTimeout(),
      runtimeState: runtimeState(),
      blockedReason: blockedReason(),
      blockedCause: patrolStatus()?.blocked_cause,
    });
    if (failure) {
      notificationStore.warning(failure.message);
    }
  };

  async function handleTogglePatrol() {
    if (isTogglingPatrol()) return;
    setIsTogglingPatrol(true);
    const previousValue = patrolEnabledLocal();
    const newValue = !previousValue;
    setPatrolEnabledLocal(newValue);
    if (!newValue) {
      setManualRunRequested(false);
      clearSafetyTimer();
    }
    try {
      const data = await AIAPI.updateSettings({ patrol_enabled: newValue });
      syncAIRuntimeSettings(data);
      if (typeof data?.patrol_enabled === 'boolean') {
        setPatrolEnabledLocal(data.patrol_enabled);
      } else {
        setPatrolEnabledLocal(newValue);
      }
      surfaceSavedPatrolReadinessIssue(
        data,
        'Patrol setting was saved, but Patrol is not ready to run.',
      );
      if (refetchPatrolStatus) {
        refetchPatrolStatus();
      }
    } catch (err) {
      console.error('Failed to toggle patrol:', err);
      setPatrolEnabledLocal(previousValue);
      notificationStore.error(patrolErrorMessage(err, 'Failed to toggle patrol'));
    } finally {
      setIsTogglingPatrol(false);
    }
  }

  async function handleRunPatrol() {
    if (
      isTriggeringPatrol() ||
      !canTriggerPatrol() ||
      manualRunRequested() ||
      patrolStream.isStreaming()
    ) {
      return;
    }
    setIsTriggeringPatrol(true);
    setManualRunRequested(true);
    clearSafetyTimer();
    safetyTimerRef = setTimeout(() => {
      safetyTimerRef = undefined;
      if (manualRunRequested() && !patrolStream.isStreaming()) {
        setManualRunRequested(false);
        notificationStore.error('Patrol run did not start — connection timed out');
        loadAllData();
      }
    }, 15000);

    try {
      await triggerPatrolRun();
    } catch (err) {
      console.error('Failed to trigger patrol run:', err);
      setManualRunRequested(false);
      notificationStore.error(patrolErrorMessage(err, 'Failed to start patrol run'));
      clearSafetyTimer();
      return;
    } finally {
      setIsTriggeringPatrol(false);
    }

    void loadAllData().catch((err) => {
      console.error('Failed to refresh Patrol after starting run:', err);
    });
  }

  const patrolRunHistory = createNonSuspendingQuery<PatrolRunRecord[], number>({
    source: activityRefreshTrigger,
    cacheKey: () => 'patrol-run-history',
    fetcher: async () => {
      try {
        return await getPatrolRunHistory(30);
      } catch (err) {
        console.error('Failed to load patrol run history:', err);
        return [];
      }
    },
    initialValue: [],
  });

  const licenseRequired = createMemo(() => patrolStatus()?.license_required ?? false);
  const runtimeState = createMemo<PatrolRuntimeState>(() => {
    if (!patrolEnabledLocal()) return 'disabled';
    return patrolStatus()?.runtime_state ?? 'active';
  });
  const blockedReason = createMemo(() =>
    normalizePatrolRuntimeBlockedReason(patrolStatus()?.blocked_reason),
  );
  const blockedAt = createMemo(() => patrolStatus()?.blocked_at);
  const patrolReadiness = createMemo(() => patrolStatus()?.readiness ?? null);
  // Pulse records the last preflight result on AISettings.patrol_preflight
  // (provider/model/duration/recommendation) so we can surface a concrete
  // diagnosis on the readiness banner instead of a one-line "Provider
  // connection issue" that forces operators to spelunk dev tools.
  const patrolPreflight = createMemo(() => aiRuntimeSettings()?.patrol_preflight ?? null);
  const readinessBlocksPatrol = createMemo(() => patrolReadiness()?.status === 'not_ready');
  const showBlockedBanner = createMemo(() => runtimeState() === 'blocked');
  const showReadinessBanner = createMemo(() => {
    const readiness = patrolReadiness();
    return runtimeState() === 'active' && readiness !== null && readiness.status !== 'ready';
  });
  const canTriggerPatrol = createMemo(
    () => runtimeState() === 'active' && !readinessBlocksPatrol(),
  );
  const triggerPatrolDisabledReason = createMemo(() => {
    if (runtimeState() === 'disabled') return 'Patrol is disabled';
    if (runtimeState() === 'blocked') return blockedReason() || 'Patrol is paused';
    if (runtimeState() === 'running') return 'Patrol is already running';
    if (runtimeState() === 'unavailable') return 'Patrol service is unavailable';
    if (readinessBlocksPatrol()) return patrolReadiness()?.summary || 'Patrol is not ready';
    return '';
  });

  const shouldShowLiveRun = createMemo(
    () =>
      patrolEnabledLocal() &&
      ((patrolStatus()?.running ?? false) || manualRunRequested() || patrolStream.isStreaming()),
  );

  createEffect(() => {
    if (shouldShowLiveRun()) {
      if (!liveRunStartedAt()) {
        setLiveRunStartedAt(new Date().toISOString());
      }
      return;
    }
    if (liveRunStartedAt()) {
      setLiveRunStartedAt('');
    }
  });

  const selectedRunFindingIds = createMemo(() => {
    const run = selectedRun();
    if (!run) return undefined;
    return run.finding_ids;
  });
  const selectedRunHasFindingsSnapshot = createMemo(() => {
    const run = selectedRun();
    if (!run) return undefined;
    return run.finding_ids !== undefined;
  });

  const selectedRunScopeResourceIds = createMemo(() => getCanonicalScopeResourceIds(selectedRun()));
  const allPatrolFindings = createMemo(() => aiIntelligenceStore.patrolFindings);
  const patrolPendingApprovalCount = createMemo(
    () => aiIntelligenceStore.patrolPendingApprovals.length,
  );
  const selectedRunPatrolFindings = createMemo(() => {
    const run = selectedRun();
    if (!run) return null;
    if (run.finding_ids === undefined) return undefined;
    const snapshotFindingIds = new Set(run.finding_ids);
    return allPatrolFindings().filter((finding) => snapshotFindingIds.has(finding.id));
  });

  const intelligenceSummary = createMemo(() => aiIntelligenceStore.intelligenceSummary);
  const circuitBreakerStatus = createMemo(() => aiIntelligenceStore.circuitBreakerStatus);
  const liveRunRecord = createMemo<PatrolRunRecord | null>(() => {
    if (!shouldShowLiveRun()) return null;
    return {
      id: '__live__',
      started_at: liveRunStartedAt() || new Date().toISOString(),
      completed_at: '',
      duration_ms: 0,
      type: 'full',
      trigger_reason: 'manual',
      resources_checked: 0,
      nodes_checked: 0,
      guests_checked: 0,
      docker_checked: 0,
      storage_checked: 0,
      hosts_checked: 0,
      truenas_checked: 0,
      pbs_checked: 0,
      pmg_checked: 0,
      kubernetes_checked: 0,
      new_findings: 0,
      existing_findings: 0,
      rejected_findings: 0,
      resolved_findings: 0,
      auto_fix_count: 0,
      findings_summary: '',
      finding_ids: [],
      error_count: 0,
      status: 'healthy',
      triage_flags: 0,
      tool_call_count: 0,
    };
  });

  const displayRunHistory = createMemo(() => {
    const live = liveRunRecord();
    const history = patrolRunHistory.value() || [];
    return live ? [live, ...history] : history;
  });

  async function loadAutonomySettings() {
    try {
      const settings = await getPatrolAutonomySettings();
      if (!settings) return;
      const effectiveSettings = resolvePatrolAutonomySettingsForSave({
        level: settings.autonomy_level,
        fullModeUnlocked: settings.full_mode_unlocked,
        autoFixLocked: autoFixLocked(),
      });
      setAutonomyLevel(effectiveSettings.autonomyLevel);
      setFullModeUnlocked(effectiveSettings.fullModeUnlocked);
      setInvestigationBudget(settings.investigation_budget);
      setInvestigationTimeout(settings.investigation_timeout_sec);
    } catch (err) {
      console.error('Failed to load autonomy settings:', err);
    }
  }

  async function handleAutonomyChange(level: PatrolAutonomyLevel) {
    if (isUpdatingAutonomy()) return;
    const controlLocked = autoFixLocked();
    if (controlLocked && level !== 'monitor') return;

    const previousLevel = autonomyLevel();
    const previousFullModeUnlocked = fullModeUnlocked();
    const effectiveSettings = resolvePatrolAutonomySettingsForSave({
      level,
      fullModeUnlocked: level === 'full',
      autoFixLocked: controlLocked,
    });
    const shouldRecordPatrolControlStarter =
      !controlLocked &&
      (effectiveSettings.autonomyLevel !== previousLevel ||
        effectiveSettings.fullModeUnlocked !== previousFullModeUnlocked);
    setAutonomyLevel(effectiveSettings.autonomyLevel);
    setFullModeUnlocked(effectiveSettings.fullModeUnlocked);
    setIsUpdatingAutonomy(true);

    try {
      await updatePatrolAutonomySettings({
        autonomy_level: effectiveSettings.autonomyLevel,
        full_mode_unlocked: effectiveSettings.fullModeUnlocked,
        investigation_budget: investigationBudget(),
        investigation_timeout_sec: investigationTimeout(),
      });
      if (shouldRecordPatrolControlStarter) {
        await recordPatrolControlStarterActivity();
        await loadVisiblePatrolData();
      }
    } catch (err) {
      console.error('Failed to update autonomy:', err);
      setAutonomyLevel(previousLevel);
      setFullModeUnlocked(previousFullModeUnlocked);
      notificationStore.error((err as Error).message || 'Failed to update Patrol mode');
    } finally {
      setIsUpdatingAutonomy(false);
    }
  }

  function handleAssistantFindingHandoff(findingId: string) {
    setAssistantHandoffFindingId(findingId.trim());
  }

  const loadLatestInvestigationProposedFixBriefing = async (
    finding: ReturnType<typeof allPatrolFindings>[number],
    pendingApprovalBriefing: PatrolAssistantApprovalBriefingInput | undefined,
  ) => {
    if (finding.investigationRecord?.proposed_fix) {
      return undefined;
    }
    const hasInvestigationPointer =
      hasFindingInvestigationHandoffPointer(finding) || Boolean(pendingApprovalBriefing?.id);
    if (!hasInvestigationPointer) {
      return undefined;
    }
    if (
      !patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationOutcome: finding.investigationOutcome,
        remediationId: finding.remediationPlanId,
        pendingApproval: pendingApprovalBriefing,
        investigationRecord: finding.investigationRecord,
      })
    ) {
      return undefined;
    }

    try {
      const investigation = await AIAPI.getInvestigation(finding.id);
      return buildPatrolAssistantProposedFixBriefingInput(investigation?.proposed_fix);
    } catch {
      return undefined;
    }
  };

  async function openAssistantOperationsLoopForFinding(findingId: string): Promise<boolean> {
    const normalizedFindingId = findingId.trim();
    if (!normalizedFindingId) return false;

    const finding = allPatrolFindings().find((candidate) => candidate.id === normalizedFindingId);
    if (!finding) {
      return false;
    }

    await aiIntelligenceStore.loadPendingApprovals();
    const pendingApproval = aiIntelligenceStore.patrolPendingApprovals.find(
      (approval) => approval.toolId === 'investigation_fix' && approval.targetId === finding.id,
    );
    const pendingApprovalBriefing = buildPatrolAssistantApprovalBriefingInput(pendingApproval);
    const latestInvestigationProposedFix = await loadLatestInvestigationProposedFixBriefing(
      finding,
      pendingApprovalBriefing,
    );
    const proposedFix =
      latestInvestigationProposedFix ||
      buildPatrolAssistantProposedFixBriefingInputFromApproval(pendingApproval);
    const handoff = buildPatrolAssistantFindingHandoffFromUnifiedFinding(finding, {
      pendingApproval: pendingApprovalBriefing,
      proposedFix,
    });

    openPatrolAssistantWorkflowHandoff(handoff);
    setAssistantHandoffFindingId(finding.id);
    return true;
  }

  function startPolling() {
    clearInterval(refreshInterval);
    clearInterval(approvalPollInterval);
    refreshInterval = setInterval(() => loadAllData(), 60000);
    approvalPollInterval = setInterval(() => aiIntelligenceStore.loadPendingApprovals(), 10000);
  }

  function stopPolling() {
    clearInterval(refreshInterval);
    clearInterval(approvalPollInterval);
  }

  async function loadAllData() {
    const requestId = ++refreshRequestId;
    clearRefreshTimeout();
    setIsRefreshing(true);
    refreshTimeoutRef = setTimeout(() => {
      if (requestId === refreshRequestId) {
        refreshTimeoutRef = undefined;
        setIsRefreshing(false);
      }
    }, PATROL_REFRESH_TIMEOUT_MS);

    try {
      await Promise.all([
        aiIntelligenceStore.loadDashboardData(),
        refetchPatrolStatus(),
      ]);
      if (requestId === refreshRequestId) {
        clearPatrolLoadError();
        setActivityRefreshTrigger((prev) => prev + 1);
      }
    } catch (error) {
      if (requestId === refreshRequestId) {
        rememberPatrolLoadError(error, 'Patrol could not refresh.');
      }
    } finally {
      finishRefresh(requestId);
    }
  }

  async function loadVisiblePatrolData() {
    try {
      await Promise.all([
        refetchPatrolStatus(),
        aiIntelligenceStore.loadPatrolFindings(),
        aiIntelligenceStore.loadPendingApprovals(),
        patrolRunHistory.refetch({ background: true }),
      ]);
      clearPatrolLoadError();
    } catch (error) {
      rememberPatrolLoadError(error, 'Patrol could not refresh.');
    }
  }

  function loadSupportingPatrolDataInBackground() {
    void Promise.allSettled([
      aiIntelligenceStore.loadIntelligenceSummary(),
      aiIntelligenceStore.loadFindings(),
      aiIntelligenceStore.loadCircuitBreakerStatus(),
      aiIntelligenceStore.loadCorrelations(),
    ]);
  }

  async function handleRefreshPatrol() {
    if (isManualRefreshRunning()) return;
    const requestId = ++manualRefreshRequestId;
    clearManualRefreshTimeout();
    setIsManualRefreshRunning(true);
    manualRefreshTimeoutRef = setTimeout(() => {
      if (requestId === manualRefreshRequestId) {
        manualRefreshTimeoutRef = undefined;
        setIsManualRefreshRunning(false);
      }
    }, PATROL_MANUAL_SYNC_TIMEOUT_MS);

    try {
      await loadVisiblePatrolData();
      loadSupportingPatrolDataInBackground();
    } finally {
      if (requestId === manualRefreshRequestId) {
        clearManualRefreshTimeout();
        setIsManualRefreshRunning(false);
      }
    }
  }

  async function consumeRoutePatrolControlStarterHandoff() {
    if (typeof window === 'undefined') {
      return;
    }

    const starter = parsePatrolControlStarter(window.location.search);
    if (starter !== PATROL_CONTROL_STARTER) {
      return;
    }

    const nextUrl = new URL(window.location.href);
    nextUrl.searchParams.delete(PATROL_CONTROL_STARTER_QUERY_PARAM);
    nextUrl.searchParams.delete(PATROL_OPERATIONS_LOOP_STARTER_QUERY_PARAM);
    const nextPath = `${nextUrl.pathname}${nextUrl.search}${nextUrl.hash}`;

    await recordPatrolControlStarterActivity();
    window.history.replaceState(window.history.state, '', nextPath);
  }

  const summaryStats = () => {
    const patrolFindings = allPatrolFindings();
    const activeFindings = patrolFindings.filter((finding) => finding.status === 'active');

    return {
      criticalFindings: activeFindings.filter((finding) => finding.severity === 'critical').length,
      warningFindings: activeFindings.filter((finding) => finding.severity === 'warning').length,
      totalActive: activeFindings.length,
      fixedCount: patrolFindings.filter(
        (finding) =>
          finding.investigationOutcome === 'fix_verified' ||
          finding.investigationOutcome === 'fix_executed' ||
          finding.investigationOutcome === 'resolved',
      ).length,
      hasAnyPatrolFindings: patrolFindings.length > 0,
    };
  };

  const activePatrolFindings = () =>
    allPatrolFindings().filter((finding) => finding.status === 'active');
  const shouldShowPatrolSetupOnly = createMemo(() => {
    const activeFindings = activePatrolFindings();
    if (readinessBlocksPatrol() && activeFindings.length === 0) {
      return true;
    }
    return activeFindings.length > 0 && activeFindings.every(isPatrolRuntimeFinding);
  });
  const patrolWorkIssueEvidenceCount = createMemo(() => {
    const issueFindingCount = allPatrolFindings().filter(
      (finding) =>
        finding.status === 'active' ||
        finding.status === 'resolved' ||
        Boolean(finding.investigationSessionId) ||
        Boolean(finding.investigationStatus) ||
        Boolean(finding.investigationOutcome) ||
        Boolean(finding.remediationPlanId),
    ).length;
    const status = patrolStatus();
    const statusFindingCount =
      (status?.findings_count ?? 0) +
      (status?.fixed_count ?? 0) +
      (status?.trust?.resolved ?? 0) +
      (status?.trust?.fix_verified ?? 0);
    return issueFindingCount + patrolPendingApprovalCount() + statusFindingCount;
  });
  const patrolWorkEvidenceCount = createMemo(
    () =>
      (patrolRunHistory.value()?.length ?? 0) +
      allPatrolFindings().length +
      (patrolStatus()?.last_patrol_at || patrolStatus()?.last_activity_at ? 1 : 0),
  );
  const patrolGovernedActionCount = createMemo(
    () =>
      allPatrolFindings().filter(
        (finding) =>
          finding.investigationOutcome &&
          PATROL_GOVERNED_ACTION_OUTCOMES.has(finding.investigationOutcome),
      ).length,
  );
  const patrolRejectedDecisionCount = createMemo(
    () =>
      allPatrolFindings().filter((finding) => finding.investigationOutcome === 'fix_rejected')
        .length,
  );
  const patrolApprovedDecisionCount = createMemo(() =>
    Math.max(0, patrolGovernedActionCount() - patrolRejectedDecisionCount()),
  );
  const patrolVerifiedOutcomeCount = createMemo(
    () =>
      allPatrolFindings().filter(
        (finding) =>
          finding.investigationOutcome &&
          PATROL_VERIFIED_OUTCOMES.has(finding.investigationOutcome),
      ).length,
  );
  const findingsTabBadgeFindings = createMemo(() => {
    const snapshotFindings = selectedRunPatrolFindings();
    if (snapshotFindings === null) {
      return activePatrolFindings();
    }
    return snapshotFindings ?? [];
  });
  const findingsTabBadgeCount = createMemo(() => {
    const snapshotFindings = selectedRunPatrolFindings();
    if (snapshotFindings === null) {
      return activePatrolFindings().length;
    }
    if (snapshotFindings === undefined) {
      return undefined;
    }
    return snapshotFindings.length;
  });

  onMount(async () => {
    try {
      await consumeRoutePatrolControlStarterHandoff();
      const initialLoads = await Promise.allSettled([
        loadRuntimeCapabilities(),
        loadAllData(),
        loadAutonomySettings(),
        loadAIRuntimeModels(),
        loadAIRuntimeSettings(),
      ]);
      const failedLoad = initialLoads.find(
        (result): result is PromiseRejectedResult => result.status === 'rejected',
      );
      if (failedLoad) {
        rememberPatrolLoadError(failedLoad.reason, 'Patrol could not refresh.');
      }
    } finally {
      setInitialSurfaceReady(true);
    }
  });

  onMount(() => {
    startPolling();

    const handleVisibility = () => {
      if (document.hidden) {
        stopPolling();
      } else {
        loadAllData();
        startPolling();
      }
    };

    document.addEventListener('visibilitychange', handleVisibility);
    onCleanup(() => {
      document.removeEventListener('visibilitychange', handleVisibility);
      clearRefreshTimeout();
      clearManualRefreshTimeout();
    });
  });

  onCleanup(() => {
    stopPolling();
    clearSafetyTimer();
    clearScrollToFindingTimer();
    clearManualRefreshTimeout();
    if (findingScrollTimerRef !== undefined) {
      clearTimeout(findingScrollTimerRef);
      findingScrollTimerRef = undefined;
    }
  });

  return {
    activeTab,
    activePatrolFindings,
    activityRefreshTrigger,
    assistantHandoffFindingId,
    autonomyLevel,
    autoFixCapabilityBlock,
    autoFixLocked,
    blockedAt,
    blockedReason,
    canTriggerPatrol,
    circuitBreakerStatus,
    clearScrollToFindingTimer,
    displayRunHistory,
    findingsTabBadgeCount,
    findingsTabBadgeFindings,
    findingsFilterOverride,
    fullModeUnlocked,
    handleAutonomyChange,
    handleAssistantFindingHandoff,
    handleRefreshPatrol,
    handleRunPatrol,
    handleTogglePatrol,
    historicalRegressionCount,
    initialSurfaceReady,
    intelligenceSummary,
    isManualRefreshRunning,
    isRefreshing,
    isTogglingPatrol,
    isTriggeringPatrol,
    isUpdatingAutonomy,
    licenseRequired,
    loadAllData,
    licenseRuntimeIdentity,
    manualRunRequested,
    openAssistantOperationsLoopForFinding,
    patrolEnabledLocal,
    patrolApprovedDecisionCount,
    patrolGovernedActionCount,
    patrolPreflight,
    patrolLoadError,
    patrolPendingApprovalCount,
    patrolReadiness,
    patrolRejectedDecisionCount,
    patrolRunHistory,
    patrolVerifiedOutcomeCount,
    patrolWorkEvidenceCount,
    patrolWorkIssueEvidenceCount,
    runtimeState,
    patrolStatus,
    patrolStream,
    recordPatrolWorkflowStarterActivity,
    selectedRun,
    selectedRunFindingIds,
    selectedRunHasFindingsSnapshot,
    selectedRunScopeResourceIds,
    setActiveTab,
    setFindingsFilterOverride,
    setFullModeUnlocked,
    setSelectedRun,
    setFindingScrollTimer: (timer: ReturnType<typeof setTimeout> | undefined) => {
      findingScrollTimerRef = timer;
    },
    setScrollToFindingTimer: (timer: ReturnType<typeof setTimeout> | undefined) => {
      scrollToFindingTimerRef = timer;
    },
    showBlockedBanner,
    showReadinessBanner,
    shouldShowPatrolSetupOnly,
    summaryStats,
    triggerPatrolDisabledReason,
  };
}

export type PatrolIntelligenceState = ReturnType<typeof usePatrolIntelligenceState>;
