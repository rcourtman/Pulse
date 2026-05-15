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
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import {
  aiRuntimeModels,
  aiRuntimeSettings,
  loadAIRuntimeModels,
  loadAIRuntimeSettings,
  syncAIRuntimeSettings,
} from '@/stores/aiRuntimeState';
import { aiChatStore } from '@/stores/aiChat';
import { notificationStore } from '@/stores/notifications';
import { usePatrolStream } from '@/hooks/usePatrolStream';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { hasFeature, loadRuntimeCapabilities } from '@/stores/license';
import type { AISettings } from '@/types/ai';
import { getCanonicalScopeResourceIds } from '@/utils/patrolFormat';
import { normalizePatrolRuntimeBlockedReason } from '@/utils/patrolRuntimePresentation';
import {
  buildPatrolConfigurationFailureHandoff,
  buildPatrolInvestigationContextSummary,
  selectPatrolSupportingRecentChanges,
  type PatrolConfigurationFailureInput,
} from './patrolInvestigationContextModel';

type PatrolTab = 'findings' | 'history';

type PatrolAPIError = Error & {
  code?: string;
  details?: Record<string, string>;
  status?: number;
};

export const PATROL_REFRESH_TIMEOUT_MS = 15000;

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
  if (level === 'assisted' || level === 'full') {
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
  const canUseFullMode =
    !autoFixLocked && (level === 'assisted' || level === 'full') && fullModeUnlocked;

  return {
    autonomyLevel: resolvePatrolAutonomyLevelForSave(level, canUseFullMode, autoFixLocked),
    fullModeUnlocked: canUseFullMode,
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
  const [showInvestigationContext, setShowInvestigationContext] = createSignal(false);
  const [findingsFilterOverride, setFindingsFilterOverride] = createSignal<
    'all' | 'active' | 'resolved' | 'approvals' | 'attention' | undefined
  >(undefined);
  const [isRefreshing, setIsRefreshing] = createSignal(false);
  const [autonomyLevel, setAutonomyLevel] = createSignal<PatrolAutonomyLevel>('monitor');
  const [isUpdatingAutonomy, setIsUpdatingAutonomy] = createSignal(false);
  const [activityRefreshTrigger, setActivityRefreshTrigger] = createSignal(0);
  const [manualRunRequested, setManualRunRequested] = createSignal(false);
  const [patrolEnabledLocal, setPatrolEnabledLocal] = createSignal<boolean>(true);
  const [liveRunStartedAt, setLiveRunStartedAt] = createSignal('');
  const [investigationBudget, setInvestigationBudget] = createSignal(15);
  const [investigationTimeout, setInvestigationTimeout] = createSignal(300);
  const [showAdvancedSettings, setShowAdvancedSettings] = createSignal(false);
  const [isSavingAdvanced, setIsSavingAdvanced] = createSignal(false);
  const [advancedSettingsError, setAdvancedSettingsError] =
    createSignal<PatrolConfigurationFailureInput | null>(null);
  const [fullModeUnlocked, setFullModeUnlocked] = createSignal(false);
  const availableModels = aiRuntimeModels;
  const [patrolModel, setPatrolModel] = createSignal<string>('');
  const [defaultModel, setDefaultModel] = createSignal<string>('');
  const [patrolInterval, setPatrolInterval] = createSignal<number>(360);
  const [isUpdatingSettings, setIsUpdatingSettings] = createSignal(false);
  const [isTogglingPatrol, setIsTogglingPatrol] = createSignal(false);
  const [isTriggeringPatrol, setIsTriggeringPatrol] = createSignal(false);
  const [alertTriggeredAnalysis, setAlertTriggeredAnalysis] = createSignal<boolean>(false);
  const [patrolAlertTriggers, setPatrolAlertTriggers] = createSignal<boolean>(true);
  const [patrolAnomalyTriggers, setPatrolAnomalyTriggers] = createSignal<boolean>(true);
  const [selectedRun, setSelectedRun] = createSignal<PatrolRunRecord | null>(null);
  const [patrolModelSelectElement, setPatrolModelSelectElement] = createSignal<HTMLSelectElement>();

  let advancedSettingsRef: HTMLDivElement | undefined;
  let safetyTimerRef: ReturnType<typeof setTimeout> | undefined;
  let scrollToFindingTimerRef: ReturnType<typeof setTimeout> | undefined;
  let findingScrollTimerRef: ReturnType<typeof setTimeout> | undefined;
  let refreshTimeoutRef: ReturnType<typeof setTimeout> | undefined;
  let refreshRequestId = 0;
  let refreshInterval: ReturnType<typeof setInterval>;
  let approvalPollInterval: ReturnType<typeof setInterval>;

  const setAdvancedSettingsRef = (element: HTMLDivElement | undefined) => {
    advancedSettingsRef = element;
  };

  const setPatrolModelSelectRef = (element: HTMLSelectElement | undefined) => {
    setPatrolModelSelectElement(() => element);
  };

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

  const finishRefresh = (requestId: number) => {
    if (requestId !== refreshRequestId) {
      return;
    }
    clearRefreshTimeout();
    setIsRefreshing(false);
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

  const handleClickOutside = (event: MouseEvent) => {
    if (advancedSettingsRef && !advancedSettingsRef.contains(event.target as Node)) {
      if (advancedSettingsError()) return;
      setShowAdvancedSettings(false);
    }
  };

  createEffect(() => {
    if (showAdvancedSettings()) {
      document.addEventListener('mousedown', handleClickOutside);
    } else {
      document.removeEventListener('mousedown', handleClickOutside);
      setAdvancedSettingsError(null);
    }
  });

  createEffect(() => {
    const model = patrolModel();
    const select = patrolModelSelectElement();
    // Track model catalog changes so a valid selected model is reapplied after
    // async options are mounted into the configuration popover.
    availableModels();
    if (select) {
      select.value = model;
    }
  });

  const alertAnalysisLocked = createMemo(() => !hasFeature('ai_alerts'));
  const autoFixLocked = createMemo(() => !hasFeature('ai_autofix'));

  const applyPatrolAISettings = (data: AISettings | null | undefined) => {
    setPatrolModel(data?.patrol_model || '');
    setDefaultModel(data?.model || '');
    setPatrolInterval(data?.patrol_interval_minutes ?? 360);
    setPatrolEnabledLocal(data?.patrol_enabled ?? true);
    setAlertTriggeredAnalysis(!alertAnalysisLocked() && data?.alert_triggered_analysis !== false);
    const legacyEventTriggersEnabled = data?.patrol_event_triggers_enabled !== false;
    setPatrolAlertTriggers(data?.patrol_alert_triggers_enabled ?? legacyEventTriggersEnabled);
    setPatrolAnomalyTriggers(data?.patrol_anomaly_triggers_enabled ?? legacyEventTriggersEnabled);
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
    setAdvancedSettingsError(failure);
  };

  async function handleTogglePatrol() {
    if (isTogglingPatrol()) return;
    setIsTogglingPatrol(true);
    setAdvancedSettingsError(null);
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
      if (typeof data?.patrol_interval_minutes === 'number') {
        setPatrolInterval(data.patrol_interval_minutes);
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
      await loadAllData();
    } catch (err) {
      console.error('Failed to trigger patrol run:', err);
      setManualRunRequested(false);
      notificationStore.error(patrolErrorMessage(err, 'Failed to start patrol run'));
      clearSafetyTimer();
    } finally {
      setIsTriggeringPatrol(false);
    }
  }

  async function handleModelChange(modelId: string) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    setAdvancedSettingsError(null);
    try {
      const updated = await AIAPI.updateSettings({ patrol_model: modelId });
      syncAIRuntimeSettings(updated);
      setPatrolModel(updated.patrol_model || modelId);
      surfaceSavedPatrolReadinessIssue(
        updated,
        'Patrol model was saved, but Patrol is not ready to run.',
      );
      await refetchPatrolStatus();
    } catch (err) {
      console.error('Failed to update patrol model:', err);
      setAdvancedSettingsError(buildAdvancedSettingsFailure(err, 'Failed to update Patrol model'));
      notificationStore.error(patrolErrorMessage(err, 'Failed to update patrol model'));
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  async function handleIntervalChange(minutes: number) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    setAdvancedSettingsError(null);
    try {
      const updated = await AIAPI.updateSettings({ patrol_interval_minutes: minutes });
      syncAIRuntimeSettings(updated);
      setPatrolInterval(updated.patrol_interval_minutes ?? minutes);
      setPatrolEnabledLocal((updated.patrol_interval_minutes ?? minutes) > 0);
      surfaceSavedPatrolReadinessIssue(
        updated,
        'Patrol schedule was saved, but Patrol is not ready to run.',
      );
      refetchPatrolStatus();
    } catch (err) {
      console.error('Failed to update patrol interval:', err);
      notificationStore.error(patrolErrorMessage(err, 'Failed to update patrol schedule'));
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  async function handleAlertTriggeredAnalysisChange(enabled: boolean) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    setAdvancedSettingsError(null);
    const previous = alertTriggeredAnalysis();
    setAlertTriggeredAnalysis(enabled);
    try {
      const updated = await AIAPI.updateSettings({ alert_triggered_analysis: enabled });
      syncAIRuntimeSettings(updated);
      surfaceSavedPatrolReadinessIssue(
        updated,
        'Patrol setting was saved, but Patrol is not ready to run.',
      );
    } catch (err) {
      console.error('Failed to update alert-triggered analysis:', err);
      setAlertTriggeredAnalysis(previous);
      notificationStore.error(patrolErrorMessage(err, 'Failed to update alert analysis setting'));
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  async function handlePatrolAlertTriggersChange(enabled: boolean) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    setAdvancedSettingsError(null);
    const previous = patrolAlertTriggers();
    setPatrolAlertTriggers(enabled);
    try {
      const updated = await AIAPI.updateSettings({ patrol_alert_triggers_enabled: enabled });
      syncAIRuntimeSettings(updated);
      surfaceSavedPatrolReadinessIssue(
        updated,
        'Patrol trigger setting was saved, but Patrol is not ready to run.',
      );
    } catch (err) {
      console.error('Failed to update alert-triggered patrols:', err);
      setPatrolAlertTriggers(previous);
      notificationStore.error(
        patrolErrorMessage(err, 'Failed to update alert-triggered Patrol setting'),
      );
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  async function handlePatrolAnomalyTriggersChange(enabled: boolean) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    setAdvancedSettingsError(null);
    const previous = patrolAnomalyTriggers();
    setPatrolAnomalyTriggers(enabled);
    try {
      const updated = await AIAPI.updateSettings({ patrol_anomaly_triggers_enabled: enabled });
      syncAIRuntimeSettings(updated);
      surfaceSavedPatrolReadinessIssue(
        updated,
        'Patrol trigger setting was saved, but Patrol is not ready to run.',
      );
    } catch (err) {
      console.error('Failed to update anomaly-triggered patrols:', err);
      setPatrolAnomalyTriggers(previous);
      notificationStore.error(
        patrolErrorMessage(err, 'Failed to update anomaly-triggered Patrol setting'),
      );
    } finally {
      setIsUpdatingSettings(false);
    }
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
  const selectedRunPatrolFindings = createMemo(() => {
    const run = selectedRun();
    if (!run) return null;
    if (run.finding_ids === undefined) return undefined;
    const snapshotFindingIds = new Set(run.finding_ids);
    return allPatrolFindings().filter((finding) => snapshotFindingIds.has(finding.id));
  });

  const intelligenceSummary = createMemo(() => aiIntelligenceStore.intelligenceSummary);
  const circuitBreakerStatus = createMemo(() => aiIntelligenceStore.circuitBreakerStatus);
  const policyPosture = createMemo(() => intelligenceSummary()?.policy_posture);
  const supportingRecentChanges = createMemo(() =>
    selectPatrolSupportingRecentChanges(intelligenceSummary()?.recent_changes),
  );
  const supportingRecentChangeCount = createMemo(() => {
    if (Array.isArray(intelligenceSummary()?.recent_changes)) {
      return supportingRecentChanges().length;
    }
    return intelligenceSummary()?.recent_changes_count;
  });
  const investigationContext = createMemo(() =>
    buildPatrolInvestigationContextSummary({
      recentChangesCount: supportingRecentChangeCount(),
      correlations: aiIntelligenceStore.correlations,
      policyPosture: policyPosture(),
    }),
  );
  const correlationTotal = createMemo(() => investigationContext().correlationCount);
  const correlations = createMemo(() => aiIntelligenceStore.correlations?.correlations ?? []);
  const recentChangeCount = createMemo(() => investigationContext().recentChangeCount);
  const hasInvestigationContext = createMemo(() => investigationContext().hasContext);
  const investigationContextSummary = createMemo(() => investigationContext().summaryText);

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
    if (autoFixLocked() && level !== 'monitor') return;

    const previousLevel = autonomyLevel();
    const previousFullModeUnlocked = fullModeUnlocked();
    const effectiveSettings = resolvePatrolAutonomySettingsForSave({
      level,
      fullModeUnlocked: fullModeUnlocked(),
      autoFixLocked: autoFixLocked(),
    });
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
    } catch (err) {
      console.error('Failed to update autonomy:', err);
      setAutonomyLevel(previousLevel);
      setFullModeUnlocked(previousFullModeUnlocked);
      notificationStore.error((err as Error).message || 'Failed to update autonomy level');
    } finally {
      setIsUpdatingAutonomy(false);
    }
  }

  const buildAdvancedSettingsFailure = (
    err: unknown,
    fallback = 'Failed to save Patrol configuration',
  ): PatrolConfigurationFailureInput => {
    const apiError = err as PatrolAPIError;
    const message = patrolErrorMessage(err, fallback);
    const readiness = patrolReadiness();
    const apiDetails = apiError.details ?? {};
    const hasReadinessDetails =
      Boolean(apiDetails.status || apiDetails.provider || apiDetails.model) ||
      apiError.code === 'patrol_readiness_not_ready';
    const readinessSummary = apiDetails.readiness_summary ?? apiDetails.summary;
    return {
      message,
      code: apiError.code,
      status: apiError.status,
      details: apiError.details,
      autonomyLevel: autonomyLevel(),
      fullModeUnlocked: fullModeUnlocked(),
      investigationBudget: investigationBudget(),
      investigationTimeoutSec: investigationTimeout(),
      readiness: readiness
        ? {
            status: readiness.status,
            cause: readiness.cause,
            summary: readiness.summary,
            provider: readiness.provider,
            model: readiness.model,
          }
        : hasReadinessDetails
          ? {
              status: apiDetails.status,
              cause: apiDetails.cause,
              summary:
                readinessSummary ||
                (apiError.code === 'patrol_readiness_not_ready' ? message : undefined),
              provider: apiDetails.provider,
              model: apiDetails.model,
            }
          : null,
      runtimeState: runtimeState(),
      blockedReason: blockedReason(),
      blockedCause: patrolStatus()?.blocked_cause,
    };
  };

  async function saveAdvancedSettings() {
    setIsSavingAdvanced(true);
    setAdvancedSettingsError(null);
    try {
      const effectiveSettings = resolvePatrolAutonomySettingsForSave({
        level: autonomyLevel(),
        fullModeUnlocked: fullModeUnlocked(),
        autoFixLocked: autoFixLocked(),
      });
      setAutonomyLevel(effectiveSettings.autonomyLevel);
      setFullModeUnlocked(effectiveSettings.fullModeUnlocked);

      const result = await updatePatrolAutonomySettings({
        autonomy_level: effectiveSettings.autonomyLevel,
        full_mode_unlocked: effectiveSettings.fullModeUnlocked,
        investigation_budget: investigationBudget(),
        investigation_timeout_sec: investigationTimeout(),
      });
      if (result.settings) {
        setAutonomyLevel(result.settings.autonomy_level);
        setFullModeUnlocked(result.settings.full_mode_unlocked);
      }
      setAdvancedSettingsError(null);
      setShowAdvancedSettings(false);
    } catch (err) {
      console.error('Failed to save advanced settings:', err);
      const failure = buildAdvancedSettingsFailure(err);
      setAdvancedSettingsError(failure);
      await refetchPatrolStatus();
    } finally {
      setIsSavingAdvanced(false);
    }
  }

  function openAdvancedSettingsErrorInAssistant() {
    const failure = advancedSettingsError();
    if (!failure) return;
    const handoff = buildPatrolConfigurationFailureHandoff(failure);
    aiChatStore.open(handoff.context);
    setShowAdvancedSettings(false);
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
      await Promise.all([aiIntelligenceStore.loadDashboardData(), refetchPatrolStatus()]);
      if (requestId === refreshRequestId) {
        setActivityRefreshTrigger((prev) => prev + 1);
      }
    } finally {
      finishRefresh(requestId);
    }
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
  const shouldSurfaceInvestigationContext = createMemo(() => {
    if (!hasInvestigationContext()) {
      return false;
    }

    if (selectedRun()) {
      return true;
    }

    if (activePatrolFindings().length > 0) {
      return true;
    }

    const overallHealth = intelligenceSummary()?.overall_health;
    if (!overallHealth) {
      return false;
    }

    return overallHealth.grade !== 'A' || overallHealth.factors.length > 0;
  });
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
      await Promise.all([
        loadRuntimeCapabilities(),
        loadAllData(),
        loadAutonomySettings(),
        loadAIRuntimeModels(),
        loadAIRuntimeSettings(),
      ]);
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
    });
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleClickOutside);
    stopPolling();
    clearSafetyTimer();
    clearScrollToFindingTimer();
    if (findingScrollTimerRef !== undefined) {
      clearTimeout(findingScrollTimerRef);
      findingScrollTimerRef = undefined;
    }
  });

  return {
    activeTab,
    activePatrolFindings,
    activityRefreshTrigger,
    advancedSettingsError,
    alertAnalysisLocked,
    alertTriggeredAnalysis,
    autonomyLevel,
    autoFixLocked,
    availableModels,
    blockedAt,
    blockedReason,
    canTriggerPatrol,
    circuitBreakerStatus,
    correlationTotal,
    correlations,
    clearScrollToFindingTimer,
    defaultModel,
    displayRunHistory,
    findingsTabBadgeCount,
    findingsTabBadgeFindings,
    findingsFilterOverride,
    fullModeUnlocked,
    handleAlertTriggeredAnalysisChange,
    handleAutonomyChange,
    handleIntervalChange,
    handleModelChange,
    handlePatrolAlertTriggersChange,
    handlePatrolAnomalyTriggersChange,
    handleRunPatrol,
    handleTogglePatrol,
    hasInvestigationContext,
    initialSurfaceReady,
    intelligenceSummary,
    investigationContextSummary,
    isRefreshing,
    isSavingAdvanced,
    isTogglingPatrol,
    isTriggeringPatrol,
    isUpdatingSettings,
    licenseRequired,
    loadAllData,
    manualRunRequested,
    openAdvancedSettingsErrorInAssistant,
    patrolEnabledLocal,
    patrolAlertTriggers,
    patrolAnomalyTriggers,
    patrolInterval,
    patrolModel,
    patrolPreflight,
    patrolReadiness,
    patrolRunHistory,
    runtimeState,
    patrolStatus,
    patrolStream,
    policyPosture,
    recentChangeCount,
    saveAdvancedSettings,
    selectedRun,
    selectedRunFindingIds,
    selectedRunHasFindingsSnapshot,
    selectedRunScopeResourceIds,
    setActiveTab,
    setAdvancedSettingsRef,
    setFindingsFilterOverride,
    setFullModeUnlocked,
    setPatrolModelSelectRef,
    setSelectedRun,
    setShowAdvancedSettings,
    setShowInvestigationContext,
    setFindingScrollTimer: (timer: ReturnType<typeof setTimeout> | undefined) => {
      findingScrollTimerRef = timer;
    },
    setScrollToFindingTimer: (timer: ReturnType<typeof setTimeout> | undefined) => {
      scrollToFindingTimerRef = timer;
    },
    showAdvancedSettings,
    showBlockedBanner,
    showReadinessBanner,
    showInvestigationContext,
    shouldSurfaceInvestigationContext,
    summaryStats,
    supportingRecentChanges,
    triggerPatrolDisabledReason,
  };
}

export type PatrolIntelligenceState = ReturnType<typeof usePatrolIntelligenceState>;
