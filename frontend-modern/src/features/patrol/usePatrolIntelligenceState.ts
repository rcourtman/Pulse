import {
  createEffect,
  createMemo,
  createResource,
  createSignal,
  onCleanup,
  onMount,
} from 'solid-js';
import {
  getPatrolAutonomySettings,
  getPatrolRunHistory,
  getPatrolStatus,
  triggerPatrolRun,
  updatePatrolAutonomySettings,
  type PatrolAutonomyLevel,
  type PatrolRunRecord,
  type PatrolStatus,
} from '@/api/patrol';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { apiFetchJSON } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { hasTriggeringAlert } from '@/utils/findingAlertIdentity';
import { usePatrolStream } from '@/hooks/usePatrolStream';
import {
  getUpgradeActionUrlOrFallback,
  hasFeature,
  licenseStatus,
  loadLicenseStatus,
  startProTrial,
} from '@/stores/license';
import { getCanonicalScopeResourceIds } from '@/utils/patrolFormat';
import {
  getProTrialStartedMessage,
  getTrialAlreadyUsedMessage,
  getTrialStartErrorMessage,
  getTrialTryAgainLaterMessage,
} from '@/utils/upgradePresentation';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';

interface ModelInfo {
  id: string;
  name: string;
  description: string;
  notable: boolean;
}

interface AISettings {
  patrol_model?: string;
  patrol_interval_minutes?: number;
  patrol_enabled?: boolean;
  model?: string;
  alert_triggered_analysis?: boolean;
  patrol_event_triggers_enabled?: boolean;
  patrol_auto_fix?: boolean;
  auto_fix_model?: string;
}

type PatrolTab = 'findings' | 'history';

export function usePatrolIntelligenceState() {
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
  const [fullModeUnlocked, setFullModeUnlocked] = createSignal(false);
  const [availableModels, setAvailableModels] = createSignal<ModelInfo[]>([]);
  const [patrolModel, setPatrolModel] = createSignal<string>('');
  const [defaultModel, setDefaultModel] = createSignal<string>('');
  const [patrolInterval, setPatrolInterval] = createSignal<number>(360);
  const [isUpdatingSettings, setIsUpdatingSettings] = createSignal(false);
  const [isTogglingPatrol, setIsTogglingPatrol] = createSignal(false);
  const [isTriggeringPatrol, setIsTriggeringPatrol] = createSignal(false);
  const [alertTriggeredAnalysis, setAlertTriggeredAnalysis] = createSignal<boolean>(false);
  const [patrolEventTriggers, setPatrolEventTriggers] = createSignal<boolean>(true);
  const [startingTrial, setStartingTrial] = createSignal(false);
  const [selectedRun, setSelectedRun] = createSignal<PatrolRunRecord | null>(null);

  let advancedSettingsRef: HTMLDivElement | undefined;
  let patrolModelSelectRef: HTMLSelectElement | undefined;
  let safetyTimerRef: ReturnType<typeof setTimeout> | undefined;
  let scrollToFindingTimerRef: ReturnType<typeof setTimeout> | undefined;
  let findingScrollTimerRef: ReturnType<typeof setTimeout> | undefined;
  let refreshInterval: ReturnType<typeof setInterval>;
  let approvalPollInterval: ReturnType<typeof setInterval>;

  const setAdvancedSettingsRef = (element: HTMLDivElement | undefined) => {
    advancedSettingsRef = element;
  };

  const setPatrolModelSelectRef = (element: HTMLSelectElement | undefined) => {
    patrolModelSelectRef = element;
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

  const [patrolStatus, { refetch: refetchPatrolStatus }] = createResource<PatrolStatus | null>(
    async () => {
      try {
        return await getPatrolStatus();
      } catch {
        return null;
      }
    },
  );

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
      setShowAdvancedSettings(false);
    }
  };

  createEffect(() => {
    if (showAdvancedSettings()) {
      document.addEventListener('mousedown', handleClickOutside);
    } else {
      document.removeEventListener('mousedown', handleClickOutside);
    }
  });

  createEffect(() => {
    const model = patrolModel();
    const models = availableModels();
    if (patrolModelSelectRef && models.length > 0 && model) {
      patrolModelSelectRef.value = model;
    }
  });

  const alertAnalysisLocked = createMemo(() => !hasFeature('ai_alerts'));
  const autoFixLocked = createMemo(() => !hasFeature('ai_autofix'));

  const canStartTrial = createMemo(() => {
    const state = licenseStatus()?.subscription_state;
    if (!state) return false;
    return state !== 'active' && state !== 'trial';
  });

  async function handleStartTrial() {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        if (typeof window !== 'undefined') {
          window.location.href = result.actionUrl;
        }
        return;
      }
      notificationStore.success(getProTrialStartedMessage());
    } catch (err) {
      const statusCode = (err as { status?: number } | null)?.status;
      if (statusCode === 409) {
        notificationStore.error(getTrialAlreadyUsedMessage());
      } else if (statusCode === 429) {
        notificationStore.error(getTrialTryAgainLaterMessage());
      } else {
        notificationStore.error(
          getTrialStartErrorMessage(err instanceof Error ? err.message : undefined, {
            branded: true,
          }),
        );
      }
    } finally {
      setStartingTrial(false);
    }
  }

  async function loadModels() {
    try {
      const data = await apiFetchJSON<{ models: ModelInfo[] }>('/api/ai/models');
      setAvailableModels(data?.models || []);
    } catch (err) {
      console.error('Failed to load models:', err);
    }
  }

  async function loadAISettings() {
    try {
      const data = await apiFetchJSON<AISettings>('/api/settings/ai');
      if (!data) return;
      setPatrolModel(data.patrol_model || '');
      setDefaultModel(data.model || '');
      setPatrolInterval(data.patrol_interval_minutes ?? 360);
      setPatrolEnabledLocal(data.patrol_enabled ?? true);
      setAlertTriggeredAnalysis(!alertAnalysisLocked() && data.alert_triggered_analysis !== false);
      setPatrolEventTriggers(data.patrol_event_triggers_enabled !== false);
    } catch (err) {
      console.error('Failed to load AI settings:', err);
    }
  }

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
      const data = await apiFetchJSON<AISettings>('/api/settings/ai/update', {
        method: 'PUT',
        body: JSON.stringify({ patrol_enabled: newValue }),
      });
      if (typeof data?.patrol_enabled === 'boolean') {
        setPatrolEnabledLocal(data.patrol_enabled);
      } else {
        setPatrolEnabledLocal(newValue);
      }
      if (typeof data?.patrol_interval_minutes === 'number') {
        setPatrolInterval(data.patrol_interval_minutes);
      }
      if (refetchPatrolStatus) {
        refetchPatrolStatus();
      }
    } catch (err) {
      console.error('Failed to toggle patrol:', err);
      setPatrolEnabledLocal(previousValue);
      notificationStore.error('Failed to toggle patrol');
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
      notificationStore.error('Failed to start patrol run');
      clearSafetyTimer();
    } finally {
      setIsTriggeringPatrol(false);
    }
  }

  async function handleModelChange(modelId: string) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    try {
      await apiFetchJSON('/api/settings/ai/update', {
        method: 'PUT',
        body: JSON.stringify({ patrol_model: modelId }),
      });
      setPatrolModel(modelId);
    } catch (err) {
      console.error('Failed to update patrol model:', err);
      notificationStore.error('Failed to update patrol model');
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  async function handleIntervalChange(minutes: number) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    try {
      await apiFetchJSON('/api/settings/ai/update', {
        method: 'PUT',
        body: JSON.stringify({ patrol_interval_minutes: minutes }),
      });
      setPatrolInterval(minutes);
      setPatrolEnabledLocal(minutes > 0);
      refetchPatrolStatus();
    } catch (err) {
      console.error('Failed to update patrol interval:', err);
      notificationStore.error('Failed to update patrol schedule');
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  async function handleAlertTriggeredAnalysisChange(enabled: boolean) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    const previous = alertTriggeredAnalysis();
    setAlertTriggeredAnalysis(enabled);
    try {
      await apiFetchJSON('/api/settings/ai/update', {
        method: 'PUT',
        body: JSON.stringify({ alert_triggered_analysis: enabled }),
      });
    } catch (err) {
      console.error('Failed to update alert-triggered analysis:', err);
      setAlertTriggeredAnalysis(previous);
      notificationStore.error('Failed to update alert analysis setting');
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  async function handlePatrolEventTriggersChange(enabled: boolean) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    const previous = patrolEventTriggers();
    setPatrolEventTriggers(enabled);
    try {
      await apiFetchJSON('/api/settings/ai/update', {
        method: 'PUT',
        body: JSON.stringify({ patrol_event_triggers_enabled: enabled }),
      });
    } catch (err) {
      console.error('Failed to update event-triggered patrols:', err);
      setPatrolEventTriggers(previous);
      notificationStore.error('Failed to update event triggers setting');
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  const [patrolRunHistory] = createResource(
    () => activityRefreshTrigger(),
    async () => {
      try {
        return await getPatrolRunHistory(30);
      } catch (err) {
        console.error('Failed to load patrol run history:', err);
        return [];
      }
    },
  );

  const licenseRequired = createMemo(() => patrolStatus()?.license_required ?? false);
  const upgradeUrl = createMemo(() => getUpgradeActionUrlOrFallback('ai_autofix'));
  const alertAnalysisUpgradeUrl = createMemo(() => getUpgradeActionUrlOrFallback('ai_alerts'));
  const blockedReason = createMemo(() => patrolStatus()?.blocked_reason?.trim() ?? '');
  const blockedAt = createMemo(() => patrolStatus()?.blocked_at);
  const showBlockedBanner = createMemo(() => patrolEnabledLocal() && !!blockedReason());
  const canTriggerPatrol = createMemo(() => patrolEnabledLocal() && !showBlockedBanner());
  const triggerPatrolDisabledReason = createMemo(() => {
    if (!patrolEnabledLocal()) return 'Patrol is disabled';
    if (showBlockedBanner()) return blockedReason() || 'Patrol is paused';
    return '';
  });

  createEffect((wasAutoFixLocked) => {
    const isAutoFixLocked = autoFixLocked();
    if (isAutoFixLocked && !wasAutoFixLocked) {
      trackPaywallViewed('ai_autofix', 'ai_intelligence');
    }
    return isAutoFixLocked;
  }, false);

  createEffect((wasAlertAnalysisLocked) => {
    const isAlertAnalysisLocked = alertAnalysisLocked();
    if (isAlertAnalysisLocked && !wasAlertAnalysisLocked) {
      trackPaywallViewed('ai_alerts', 'ai_intelligence');
    }
    return isAlertAnalysisLocked;
  }, false);

  createEffect((wasLicenseBannerVisible) => {
    const isLicenseBannerVisible = licenseRequired() && !showBlockedBanner();
    if (isLicenseBannerVisible && !wasLicenseBannerVisible) {
      trackPaywallViewed('ai_autofix', 'ai_intelligence_banner');
    }
    return isLicenseBannerVisible;
  }, false);

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

  const selectedRunScopeResourceIds = createMemo(() => getCanonicalScopeResourceIds(selectedRun()));

  const intelligenceSummary = createMemo(() => aiIntelligenceStore.intelligenceSummary);
  const policyPosture = createMemo(() => intelligenceSummary()?.policy_posture);
  const correlationTotal = createMemo(
    () =>
      aiIntelligenceStore.correlations?.count ??
      aiIntelligenceStore.correlations?.correlations?.length ??
      0,
  );
  const correlations = createMemo(() => aiIntelligenceStore.correlations?.correlations ?? []);
  const recentChangeCount = createMemo(
    () =>
      intelligenceSummary()?.recent_changes_count ?? intelligenceSummary()?.recent_changes?.length ?? 0,
  );
  const hasInvestigationContext = createMemo(
    () =>
      recentChangeCount() > 0 ||
      correlationTotal() > 0 ||
      (policyPosture()?.total_resources ?? 0) > 0,
  );
  const investigationContextSummary = createMemo(() => {
    const parts: string[] = [];
    if (recentChangeCount() > 0) {
      parts.push(`${recentChangeCount()} recent change${recentChangeCount() === 1 ? '' : 's'}`);
    }
    if (correlationTotal() > 0) {
      parts.push(`${correlationTotal()} correlation${correlationTotal() === 1 ? '' : 's'}`);
    }
    const governedResources = policyPosture()?.total_resources ?? 0;
    if (governedResources > 0) {
      parts.push(
        `${governedResources} governed resource${governedResources === 1 ? '' : 's'}`,
      );
    }
    return parts.join(' · ');
  });

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
    const history = patrolRunHistory() || [];
    return live ? [live, ...history] : history;
  });

  async function loadAutonomySettings() {
    try {
      const settings = await getPatrolAutonomySettings();
      if (!settings) return;
      const effectiveLevel =
        autoFixLocked() && settings.autonomy_level !== 'monitor'
          ? 'monitor'
          : settings.autonomy_level;
      setAutonomyLevel(effectiveLevel);
      setFullModeUnlocked(settings.full_mode_unlocked);
      setInvestigationBudget(settings.investigation_budget);
      setInvestigationTimeout(settings.investigation_timeout_sec);
    } catch (err) {
      console.error('Failed to load autonomy settings:', err);
    }
  }

  async function handleAutonomyChange(level: PatrolAutonomyLevel) {
    if (isUpdatingAutonomy()) return;
    if (autoFixLocked() && (level === 'approval' || level === 'assisted')) return;

    const previousLevel = autonomyLevel();
    const effectiveLevel = level === 'assisted' && fullModeUnlocked() ? 'full' : level;
    setAutonomyLevel(effectiveLevel);
    setIsUpdatingAutonomy(true);

    try {
      await updatePatrolAutonomySettings({
        autonomy_level: effectiveLevel,
        full_mode_unlocked: fullModeUnlocked(),
        investigation_budget: investigationBudget(),
        investigation_timeout_sec: investigationTimeout(),
      });
    } catch (err) {
      console.error('Failed to update autonomy:', err);
      setAutonomyLevel(previousLevel);
      notificationStore.error((err as Error).message || 'Failed to update autonomy level');
    } finally {
      setIsUpdatingAutonomy(false);
    }
  }

  async function saveAdvancedSettings() {
    setIsSavingAdvanced(true);
    try {
      let effectiveLevel = autonomyLevel();
      const inAutoFix = effectiveLevel === 'assisted' || effectiveLevel === 'full';
      if (inAutoFix) {
        effectiveLevel = fullModeUnlocked() ? 'full' : 'assisted';
      }

      const result = await updatePatrolAutonomySettings({
        autonomy_level: effectiveLevel,
        full_mode_unlocked: fullModeUnlocked(),
        investigation_budget: investigationBudget(),
        investigation_timeout_sec: investigationTimeout(),
      });
      if (result.settings) {
        setAutonomyLevel(result.settings.autonomy_level);
        setFullModeUnlocked(result.settings.full_mode_unlocked);
      }
      setShowAdvancedSettings(false);
    } catch (err) {
      console.error('Failed to save advanced settings:', err);
      notificationStore.error('Failed to save advanced settings');
    } finally {
      setIsSavingAdvanced(false);
    }
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
    setIsRefreshing(true);
    try {
      await Promise.all([aiIntelligenceStore.loadDashboardData(), refetchPatrolStatus()]);
      setActivityRefreshTrigger((prev) => prev + 1);
    } finally {
      setIsRefreshing(false);
    }
  }

  const summaryStats = () => {
    const allFindings = aiIntelligenceStore.findings;
    const patrolFindings = allFindings.filter(
      (finding) => finding.source !== 'threshold' && !finding.isThreshold && !hasTriggeringAlert(finding),
    );
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

  onMount(async () => {
    await Promise.all([
      loadLicenseStatus(),
      loadAllData(),
      loadAutonomySettings(),
      loadModels(),
      loadAISettings(),
    ]);
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
    onCleanup(() => document.removeEventListener('visibilitychange', handleVisibility));
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
    activityRefreshTrigger,
    alertAnalysisUpgradeUrl,
    alertAnalysisLocked,
    alertTriggeredAnalysis,
    autonomyLevel,
    autoFixLocked,
    availableModels,
    blockedAt,
    blockedReason,
    canStartTrial,
    canTriggerPatrol,
    correlationTotal,
    correlations,
    clearScrollToFindingTimer,
    defaultModel,
    displayRunHistory,
    findingsFilterOverride,
    fullModeUnlocked,
    handleAlertTriggeredAnalysisChange,
    handleAutonomyChange,
    handleIntervalChange,
    handleModelChange,
    handlePatrolEventTriggersChange,
    handleRunPatrol,
    handleStartTrial,
    handleTogglePatrol,
    hasInvestigationContext,
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
    patrolEnabledLocal,
    patrolEventTriggers,
    patrolInterval,
    patrolModel,
    patrolRunHistory,
    patrolStatus,
    patrolStream,
    policyPosture,
    recentChangeCount,
    saveAdvancedSettings,
    selectedRun,
    selectedRunFindingIds,
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
    showInvestigationContext,
    startingTrial,
    summaryStats,
    triggerPatrolDisabledReason,
    upgradeUrl,
  };
}
