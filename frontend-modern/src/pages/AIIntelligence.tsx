/**
 * Patrol Page
 *
 * Central hub for Patrol intelligence - AI-powered findings with investigation support.
 */

import { createSignal, createEffect, onMount, onCleanup, createMemo, createResource, For, Show } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { FindingsPanel } from '@/components/AI/FindingsPanel';
import {
  getPatrolStatus,
  getPatrolAutonomySettings,
  updatePatrolAutonomySettings,
  triggerPatrolRun,
  getPatrolRunHistory,
  type PatrolStatus,
  type PatrolAutonomyLevel,
  type PatrolRunRecord,
} from '@/api/patrol';
import { apiFetchJSON } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';

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
  patrol_auto_fix?: boolean;
  auto_fix_model?: string;
}

import ActivityIcon from 'lucide-solid/icons/activity';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import PlayIcon from 'lucide-solid/icons/play';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import XIcon from 'lucide-solid/icons/x';

import SparklesIcon from 'lucide-solid/icons/sparkles';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import SettingsIcon from 'lucide-solid/icons/settings';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { TogglePrimitive, Toggle } from '@/components/shared/Toggle';
import { ApprovalBanner, PatrolStatusBar, RunHistoryPanel, CountdownTimer } from '@/components/patrol';
import { usePatrolStream } from '@/hooks/usePatrolStream';
import {
  getUpgradeActionUrlOrFallback,
  hasFeature,
  licenseStatus,
  loadLicenseStatus,
  startProTrial,
} from '@/stores/license';
import { formatRelativeTime } from '@/utils/format';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import {
  formatTriggerReason,
  groupModelsByProvider,
} from '@/utils/patrolFormat';



// Schedule presets in minutes
const SCHEDULE_PRESETS = [
  { value: 0, label: 'Disabled' },
  { value: 10, label: '10 min' },
  { value: 15, label: '15 min' },
  { value: 30, label: '30 min' },
  { value: 60, label: '1 hour' },
  { value: 180, label: '3 hours' },
  { value: 360, label: '6 hours' },
  { value: 720, label: '12 hours' },
  { value: 1440, label: '24 hours' },
];

type PatrolTab = 'findings' | 'history';

export function AIIntelligence() {
  const [activeTab, setActiveTab] = createSignal<PatrolTab>('findings');
  const [findingsFilterOverride, setFindingsFilterOverride] = createSignal<'all' | 'active' | 'resolved' | 'approvals' | 'attention' | undefined>(undefined);
  const [isRefreshing, setIsRefreshing] = createSignal(false);
  const [autonomyLevel, setAutonomyLevel] = createSignal<PatrolAutonomyLevel>('monitor');
  const [isUpdatingAutonomy, setIsUpdatingAutonomy] = createSignal(false);

  // Trigger to refresh patrol activity visualizations
  const [activityRefreshTrigger, setActivityRefreshTrigger] = createSignal(0);

  // Optimistic running state — set immediately on "Run Patrol" click to avoid race with backend
  const [manualRunRequested, setManualRunRequested] = createSignal(false);
  const [patrolEnabledLocal, setPatrolEnabledLocal] = createSignal<boolean>(true);
  const [liveRunStartedAt, setLiveRunStartedAt] = createSignal('');

  // Safety timer ref — hoisted so onStart can clear it when SSE connects
  let safetyTimerRef: ReturnType<typeof setTimeout> | undefined;
  let scrollToFindingTimerRef: ReturnType<typeof setTimeout> | undefined;
  let findingScrollTimerRef: ReturnType<typeof setTimeout> | undefined;

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

  // Live patrol streaming
  const patrolStream = usePatrolStream({
    running: () => patrolEnabledLocal() && ((patrolStatus()?.running ?? false) || manualRunRequested()),
    onStart: () => {
      // SSE connected — clear the safety timeout
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

  // Advanced autonomy settings
  const [investigationBudget, setInvestigationBudget] = createSignal(15);
  const [investigationTimeout, setInvestigationTimeout] = createSignal(300);
  const [showAdvancedSettings, setShowAdvancedSettings] = createSignal(false);
  const [isSavingAdvanced, setIsSavingAdvanced] = createSignal(false);
  const [fullModeUnlocked, setFullModeUnlocked] = createSignal(false);
  let advancedSettingsRef: HTMLDivElement | undefined;
  let patrolModelSelectRef: HTMLSelectElement | undefined;

  // Close popover when clicking outside
  const handleClickOutside = (e: MouseEvent) => {
    if (advancedSettingsRef && !advancedSettingsRef.contains(e.target as Node)) {
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

  onCleanup(() => {
    document.removeEventListener('mousedown', handleClickOutside);
    clearSafetyTimer();
    clearScrollToFindingTimer();
  });

  // AI settings state
  const [availableModels, setAvailableModels] = createSignal<ModelInfo[]>([]);
  const [patrolModel, setPatrolModel] = createSignal<string>('');
  const [defaultModel, setDefaultModel] = createSignal<string>('');
  const [patrolInterval, setPatrolInterval] = createSignal<number>(360);
  const [isUpdatingSettings, setIsUpdatingSettings] = createSignal(false);
  const [isTogglingPatrol, setIsTogglingPatrol] = createSignal(false);
  const [isTriggeringPatrol, setIsTriggeringPatrol] = createSignal(false);
  const [alertTriggeredAnalysis, setAlertTriggeredAnalysis] = createSignal<boolean>(false);
  const [startingTrial, setStartingTrial] = createSignal(false);

  const canStartTrial = createMemo(() => {
    const state = licenseStatus()?.subscription_state;
    if (!state) return false;
    return state !== 'active' && state !== 'trial';
  });

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      await startProTrial();
      notificationStore.success('Pro trial started');
    } catch (err) {
      const statusCode = (err as { status?: number } | null)?.status;
      if (statusCode === 409) {
        notificationStore.error('Trial already used');
      } else if (statusCode === 429) {
        notificationStore.error('Try again later');
      } else {
        notificationStore.error(err instanceof Error ? err.message : 'Failed to start Pro trial');
      }
    } finally {
      setStartingTrial(false);
    }
  };



  // Re-apply patrol model select value when models load after settings
  // (select value is ignored by the browser if no matching option exists yet)
  createEffect(() => {
    const model = patrolModel();
    const models = availableModels();
    if (patrolModelSelectRef && models.length > 0 && model) {
      patrolModelSelectRef.value = model;
    }
  });

  // Detect when saved patrol model is no longer in the available models list
  const patrolModelStale = createMemo(() => {
    const model = patrolModel();
    const models = availableModels();
    if (!model || models.length === 0) return false;
    return !models.some(m => m.id === model);
  });



  // License feature gates
  const alertAnalysisLocked = createMemo(() => !hasFeature('ai_alerts'));
  const autoFixLocked = createMemo(() => !hasFeature('ai_autofix'));
  const [selectedRun, setSelectedRun] = createSignal<PatrolRunRecord | null>(null);

  const scheduleOptions = createMemo(() => {
    const current = patrolInterval();
    const options = [...SCHEDULE_PRESETS];
    if (Number.isFinite(current) && !options.some((opt) => opt.value === current)) {
      options.push({ value: current, label: `${current} min` });
      options.sort((a, b) => a.value - b.value);
    }
    return options;
  });



  // Load available models
  async function loadModels() {
    try {
      const data = await apiFetchJSON<{ models: ModelInfo[] }>('/api/ai/models');
      setAvailableModels(data?.models || []);
    } catch (err) {
      console.error('Failed to load models:', err);
    }
  }

  // Load AI settings
  async function loadAISettings() {
    try {
      const data = await apiFetchJSON<AISettings>('/api/settings/ai');
      if (!data) return;
      setPatrolModel(data.patrol_model || '');
      setDefaultModel(data.model || '');
      setPatrolInterval(data.patrol_interval_minutes ?? 360);
      setPatrolEnabledLocal(data.patrol_enabled ?? true);
      setAlertTriggeredAnalysis(!alertAnalysisLocked() && data.alert_triggered_analysis !== false);

    } catch (err) {
      console.error('Failed to load AI settings:', err);
    }
  }

  // Toggle patrol on/off
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
      setPatrolEnabledLocal(previousValue); // Rollback
      notificationStore.error('Failed to toggle patrol');
    } finally {
      setIsTogglingPatrol(false);
    }
  }

  async function handleRunPatrol() {
    if (isTriggeringPatrol() || !canTriggerPatrol() || manualRunRequested() || patrolStream.isStreaming()) return;
    setIsTriggeringPatrol(true);
    setManualRunRequested(true);

    // Safety timeout: if SSE never connects within 15s, clear optimistic state.
    // Cleared early via onStart callback when the SSE connection opens.
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
      // Clear safety timer on API error
      clearSafetyTimer();
    } finally {
      setIsTriggeringPatrol(false);
    }
  }

  // Update patrol model
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

  // Update patrol interval
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
      // Refetch patrol status so the countdown timer reflects the new interval
      refetchPatrolStatus();
    } catch (err) {
      console.error('Failed to update patrol interval:', err);
      notificationStore.error('Failed to update patrol schedule');
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  // Toggle alert-triggered analysis
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


  // Fetch patrol status (license_required reflects auto-fix, not patrol access)
  const [patrolStatus, { refetch: refetchPatrolStatus }] = createResource<PatrolStatus | null>(async () => {
    try {
      return await getPatrolStatus();
    } catch {
      return null;
    }
  });

  const [patrolRunHistory] = createResource(
    () => activityRefreshTrigger(),
    async () => {
      try {
        return await getPatrolRunHistory(30);
      } catch (err) {
        console.error('Failed to load patrol run history:', err);
        return [];
      }
    }
  );

  const licenseRequired = createMemo(() => patrolStatus()?.license_required ?? false);
  const upgradeUrl = createMemo(() => getUpgradeActionUrlOrFallback('ai_autofix'));
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
    () => patrolEnabledLocal() && ((patrolStatus()?.running ?? false) || manualRunRequested() || patrolStream.isStreaming()),
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
    if (!run || !run.finding_ids || run.finding_ids.length === 0) return null;
    return run.finding_ids;
  });

  // Live in-progress run entry for history list
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
      tool_call_count: 0,
    };
  });

  // Combined run history: live entry (if any) prepended to real history
  const displayRunHistory = createMemo(() => {
    const live = liveRunRecord();
    const history = patrolRunHistory() || [];
    return live ? [live, ...history] : history;
  });

  // Load autonomy settings
  async function loadAutonomySettings() {
    try {
      const settings = await getPatrolAutonomySettings();
      if (!settings) return;
      setAutonomyLevel(settings.autonomy_level);
      setFullModeUnlocked(settings.full_mode_unlocked);
      setInvestigationBudget(settings.investigation_budget);
      setInvestigationTimeout(settings.investigation_timeout_sec);
    } catch (err) {
      console.error('Failed to load autonomy settings:', err);
    }
  }

  // Update autonomy level (optimistic UI)
  // When user picks "Auto-fix" (assisted), the actual backend level depends on whether
  // the "auto-fix critical issues" toggle is on — if so, we send 'full', otherwise 'assisted'.
  async function handleAutonomyChange(level: PatrolAutonomyLevel) {
    if (isUpdatingAutonomy()) return;

    const previousLevel = autonomyLevel();
    const effectiveLevel = level === 'assisted' && fullModeUnlocked() ? 'full' : level;
    setAutonomyLevel(effectiveLevel); // Optimistic update
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
      setAutonomyLevel(previousLevel); // Rollback on error
      notificationStore.error((err as Error).message || 'Failed to update autonomy level');
    } finally {
      setIsUpdatingAutonomy(false);
    }
  }

  // Save advanced settings
  // When the "auto-fix critical issues" toggle changes, adjust the autonomy level:
  //   - Toggle on + currently assisted → switch to full
  //   - Toggle off + currently full → switch to assisted
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
      // Update local state from server response (handles auto-downgrade)
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

  onMount(async () => {
    await Promise.all([loadLicenseStatus(), loadAllData(), loadAutonomySettings(), loadModels(), loadAISettings()]);
  });

  // Polling intervals — paused when tab is hidden to save resources
  let refreshInterval: ReturnType<typeof setInterval>;
  let approvalPollInterval: ReturnType<typeof setInterval>;

  function startPolling() {
    clearInterval(refreshInterval);
    clearInterval(approvalPollInterval);
    refreshInterval = setInterval(() => loadAllData(), 60000);
    // Approval polling: 10s interval for 5-min expiry approvals
    approvalPollInterval = setInterval(() => aiIntelligenceStore.loadPendingApprovals(), 10000);
  }

  function stopPolling() {
    clearInterval(refreshInterval);
    clearInterval(approvalPollInterval);
  }

  onMount(() => {
    startPolling();

    const handleVisibility = () => {
      if (document.hidden) {
        stopPolling();
      } else {
        // Refresh immediately on tab return, then resume polling
        loadAllData();
        startPolling();
      }
    };
    document.addEventListener('visibilitychange', handleVisibility);
    onCleanup(() => document.removeEventListener('visibilitychange', handleVisibility));
  });
  onCleanup(() => {
    stopPolling();
    if (safetyTimerRef !== undefined) {
      clearTimeout(safetyTimerRef);
      safetyTimerRef = undefined;
    }
    if (findingScrollTimerRef !== undefined) {
      clearTimeout(findingScrollTimerRef);
      findingScrollTimerRef = undefined;
    }
  });

  async function loadAllData() {
    setIsRefreshing(true);
    try {
      await Promise.all([
        aiIntelligenceStore.loadFindings(),
        aiIntelligenceStore.loadCircuitBreakerStatus(),
        aiIntelligenceStore.loadPendingApprovals(),
        refetchPatrolStatus(),
      ]);
      // Trigger refresh of patrol status bar
      setActivityRefreshTrigger(prev => prev + 1);
    } finally {
      setIsRefreshing(false);
    }
  }

  const summaryStats = () => {
    const allFindings = aiIntelligenceStore.findings;
    // Only count Patrol findings (exclude threshold alerts)
    const patrolFindings = allFindings.filter(f =>
      f.source !== 'threshold' && !f.isThreshold && !f.alertId
    );
    const activeFindings = patrolFindings.filter(f => f.status === 'active');

    const criticalCount = activeFindings.filter(f => f.severity === 'critical').length;
    const warningCount = activeFindings.filter(f => f.severity === 'warning').length;
    const totalActive = activeFindings.length;
    const fixedCount = patrolFindings.filter(f =>
      f.investigationOutcome === 'fix_verified' ||
      f.investigationOutcome === 'fix_executed' ||
      f.investigationOutcome === 'resolved'
    ).length;

    return {
      criticalFindings: criticalCount,
      warningFindings: warningCount,
      totalActive,
      fixedCount,
      hasAnyPatrolFindings: patrolFindings.length > 0,
    };
  };

  return (
    <div class="h-full flex flex-col bg-base">
      {/* Header */}
      <div class="flex-shrink-0 bg-surface border-b border-border px-4 py-3">
        {/* Top row: Title and refresh */}
        <div class="flex items-center justify-between gap-4 mb-3">
          <div class="flex items-center gap-3">
            <PulsePatrolLogo class="w-6 h-6 text-slate-700 dark:text-slate-200" />
            <div title="Pulse Patrol constantly monitors your infrastructure, investigates alerts, and can automatically fix issues based on your autonomy settings.">
              <h1 class="text-lg font-semibold text-base-content">Patrol</h1>
              <p class="text-sm text-muted">
                Pulse Patrol monitoring and analysis
              </p>
            </div>
          </div>

          <div class="flex items-center gap-3">
            {/* Last/Next patrol timing - only show if patrol has run */}
            <Show when={patrolStatus()?.last_patrol_at}>
              <div class="hidden sm:flex items-center gap-3 text-xs text-muted">
                <span>Last: {formatRelativeTime(patrolStatus()?.last_patrol_at, { compact: true, emptyText: 'Never' })}</span>
                <Show when={patrolStatus()?.next_patrol_at}>
                  <span class="text-slate-300 dark:text-slate-600">|</span>
                  <CountdownTimer
                    targetDate={patrolStatus()!.next_patrol_at!}
                    prefix="Next run: "
                    class="font-variant-numeric tabular-nums font-medium text-blue-600 dark:text-blue-400"
                  />
                </Show>
              </div>
            </Show>

            {/* Run Patrol Button */}
            <button
              onClick={() => handleRunPatrol()}
              disabled={isTriggeringPatrol() || !canTriggerPatrol() || manualRunRequested() || patrolStream.isStreaming()}
              title={triggerPatrolDisabledReason()}
              class="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-slate-300 dark:disabled:bg-slate-600 disabled:text-slate-500 rounded-md transition-colors"
            >
              <PlayIcon class={`w-4 h-4 ${(isTriggeringPatrol() || manualRunRequested() || patrolStream.isStreaming()) ? 'animate-pulse' : ''}`} />
              {isTriggeringPatrol() ? 'Starting…' : (manualRunRequested() || patrolStream.isStreaming()) ? 'Running…' : 'Run Patrol'}
            </button>

            {/* Refresh Button */}
            <button
              onClick={() => loadAllData()}
              disabled={isRefreshing()}
              class="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-slate-700 dark:text-slate-300 bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-md hover:bg-slate-50 dark:hover:bg-slate-600 disabled:opacity-50 transition-colors"
            >
              <RefreshCwIcon class={`w-4 h-4 ${isRefreshing() ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>
        </div>

        {/* Settings row - Simplified for Enterprise Feel */}
        <div class="flex items-center gap-4 mt-2 mb-1">
          {/* Global Patrol Toggle */}
          <div class="flex items-center gap-2 bg-surface-hover px-3 py-1.5 rounded-md border border-border">
            <TogglePrimitive
              checked={patrolEnabledLocal()}
              disabled={isTogglingPatrol()}
              onToggle={handleTogglePatrol}
              size="sm"
              ariaLabel="Toggle Patrol"
            />
            <span class="text-sm font-medium text-slate-700 dark:text-slate-300">
              {patrolEnabledLocal() ? 'Patrol Active' : 'Patrol Disabled'}
            </span>
          </div>

          <div class="flex-1"></div>

          {/* Configuration Popover */}
          <div class="relative" ref={advancedSettingsRef}>
            <button
              onClick={() => setShowAdvancedSettings(!showAdvancedSettings())}
              disabled={!patrolEnabledLocal()}
              class={`flex items-center gap-2 px-3 py-1.5 text-sm font-medium rounded-md transition-all shadow-sm ${showAdvancedSettings()
                ? 'bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-800'
                : 'bg-white text-slate-700 border border-slate-200 hover:bg-slate-50 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700 dark:hover:bg-slate-750'
                } ${!patrolEnabledLocal() ? 'opacity-50 cursor-not-allowed hidden' : ''}`}
            >
              <SettingsIcon class="w-4 h-4" />
              Configure Patrol
            </button>

            <Show when={showAdvancedSettings()}>
              <div class="absolute right-0 top-10 z-50 w-[340px] p-5 bg-surface rounded-md shadow-sm border border-border animate-slide-up transform origin-top-right">
                <div class="flex items-center justify-between mb-5 pb-3 border-b border-slate-100 dark:border-slate-700">
                  <h4 class="text-base font-semibold tracking-tight text-base-content">Patrol Configuration</h4>
                  <button
                    onClick={() => setShowAdvancedSettings(false)}
                    class="p-1 rounded-md text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-700 dark:hover:text-slate-300 transition-colors"
                  >
                    <XIcon class="w-4 h-4" />
                  </button>
                </div>

                <div class="space-y-6">
                  {/* Model & Schedule grouped */}
                  <div class="grid grid-cols-2 gap-4">
                    <div class="space-y-1.5">
                      <label class="text-xs font-semibold uppercase tracking-wider text-muted">AI Model</label>
                      <select
                        ref={patrolModelSelectRef}
                        value={patrolModel()}
                        onChange={(e) => handleModelChange(e.currentTarget.value)}
                        disabled={isUpdatingSettings() || !patrolEnabledLocal()}
                        class="w-full text-sm bg-base border border-border rounded-md py-2 pl-3 pr-8 text-slate-700 dark:text-slate-300 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50"
                      >
                        <option value="">Default ({defaultModel().split(':').pop() || 'not set'})</option>
                        <Show when={patrolModelStale()}>
                          <option value={patrolModel()} disabled>
                            {patrolModel().split(':').pop()} (unavailable)
                          </option>
                        </Show>
                        {Array.from(groupModelsByProvider(availableModels()).entries()).map(([provider, models]) => (
                          <optgroup label={provider.charAt(0).toUpperCase() + provider.slice(1)}>
                            {models.map((model) => (
                              <option value={model.id}>
                                {model.name || model.id.split(':').pop()}
                              </option>
                            ))}
                          </optgroup>
                        ))}
                      </select>
                    </div>

                    <div class="space-y-1.5">
                      <label class="text-xs font-semibold uppercase tracking-wider text-muted">Run Every</label>
                      <select
                        value={patrolInterval()}
                        onChange={(e) => handleIntervalChange(parseInt(e.currentTarget.value))}
                        disabled={isUpdatingSettings() || !patrolEnabledLocal()}
                        class="w-full text-sm bg-base border border-border rounded-md py-2 pl-3 pr-8 text-slate-700 dark:text-slate-300 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50"
                      >
                        <For each={scheduleOptions()}>
                          {(preset) => (
                            <option value={preset.value}>{preset.label}</option>
                          )}
                        </For>
                      </select>
                    </div>
                  </div>

                  {/* Operational Mode */}
                  <div class="space-y-2">
                    <div class="flex items-center justify-between">
                      <label class="text-xs font-semibold uppercase tracking-wider text-muted flex items-center gap-1.5">
                        Operational Mode
                        <div class="relative group">
                          <CircleHelpIcon class="w-3.5 h-3.5 text-slate-400 cursor-help" />
                          <div class="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 hidden group-hover:block w-64 p-3 bg-slate-800 text-white rounded-md shadow-md text-xs z-50 pointer-events-none before:absolute before:top-full before:left-1/2 before:-translate-x-1/2 before:border-4 before:border-transparent before:border-t-slate-800">
                            <strong>Monitor:</strong> Detect only.<br />
                            <strong>Investigate:</strong> Detect & propose fixes.<br />
                            <strong>Auto-fix:</strong> Execute safe fixes automatically.
                          </div>
                        </div>
                      </label>
                    </div>

                    <div class="flex items-center bg-base rounded-md p-1 border border-slate-200 dark:border-slate-800 shadow-inner">
                      <For each={(['monitor', 'approval', 'assisted'] as PatrolAutonomyLevel[])}>
                        {(level) => {
                          const isProLocked = () => autoFixLocked() && level === 'assisted';
                          const isDisabled = () => !patrolEnabledLocal() || isProLocked();
                          const isActive = () => level === 'assisted'
                            ? autonomyLevel() === 'assisted' || autonomyLevel() === 'full'
                            : autonomyLevel() === level;

                          return (
                            <button
                              onClick={() => handleAutonomyChange(level)}
                              disabled={isDisabled()}
                              title={isProLocked() ? 'Upgrade to Pro for automatic fixes' : undefined}
                              class={`flex-1 py-1.5 px-2 text-xs font-semibold rounded-md transition-all duration-200 ${isActive()
                                ? 'bg-white dark:bg-slate-700 text-blue-600 dark:text-blue-400 shadow-[0_1px_3px_rgba(0,0,0,0.1)]'
                                : isDisabled()
                                  ? 'text-slate-400 dark:text-slate-600'
                                  : 'text-muted hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800'
                                } ${isDisabled() ? 'opacity-50 cursor-not-allowed' : ''}`}
                            >
                              {level === 'monitor' ? 'Monitor' : level === 'approval' ? 'Investigate' : 'Auto-fix'}
                            </button>
                          );
                        }}
                      </For>
                    </div>
                  </div>

                  {/* Toggles */}
                  <div class="space-y-4 pt-4 border-t border-slate-100 dark:border-slate-700">
                    <div class="flex items-start justify-between gap-3">
                      <div class="flex-1">
                        <label class="text-sm font-medium text-base-content">Alert-Triggered Analysis</label>
                        <p class="text-[11px] text-muted mt-0.5 leading-tight">
                          Analyze infrastructure automatically when critical alerts fire.
                        </p>
                      </div>
                      <Toggle
                        checked={alertTriggeredAnalysis()}
                        onChange={(e) => handleAlertTriggeredAnalysisChange(e.currentTarget.checked)}
                        disabled={isUpdatingSettings() || alertAnalysisLocked()}
                      />
                    </div>

                    <Show when={alertAnalysisLocked()}>
                      <div class="-my-1 pl-1 text-[11px] text-slate-500">
                        <a href={getUpgradeActionUrlOrFallback('ai_alerts')} target="_blank" class="text-indigo-500 font-medium hover:underline">Upgrade</a> to enable.
                        <Show when={canStartTrial()}>
                          <button type="button" onClick={handleStartTrial} disabled={startingTrial()} class="ml-1 text-indigo-500 hover:underline">Start free trial</button>
                        </Show>
                      </div>
                    </Show>

                    <div class="flex items-start justify-between gap-3">
                      <div class="flex-1">
                        <label class="text-sm font-medium text-red-600 dark:text-red-400">Auto-fix critical issues</label>
                        <p class="text-[11px] text-muted mt-0.5 leading-tight">
                          Permit Patrol to execute critical remediations without approval.
                        </p>
                      </div>
                      <Toggle
                        checked={!autoFixLocked() && fullModeUnlocked()}
                        onChange={(e) => setFullModeUnlocked(e.currentTarget.checked)}
                        disabled={autoFixLocked() || !(autonomyLevel() === 'assisted' || autonomyLevel() === 'full')}
                      />
                    </div>

                    <Show when={autoFixLocked()}>
                      <div class="-mt-1 pl-1 text-[11px] text-slate-500">
                        <a href={getUpgradeActionUrlOrFallback('ai_autofix')} target="_blank" class="text-indigo-500 font-medium hover:underline">Upgrade</a> to unlock auto-fix.
                        <Show when={canStartTrial()}>
                          <button type="button" onClick={handleStartTrial} disabled={startingTrial()} class="ml-1 text-indigo-500 hover:underline">Start free trial</button>
                        </Show>
                      </div>
                    </Show>
                  </div>

                  {/* Save Footer */}
                  <div class="pt-4 border-t border-slate-100 dark:border-slate-700">
                    <button
                      onClick={saveAdvancedSettings}
                      disabled={isSavingAdvanced()}
                      class="w-full py-2.5 text-sm font-medium text-white bg-slate-900 dark:bg-white dark:text-slate-900 hover:bg-slate-800 dark:hover:bg-slate-100 rounded-md shadow-sm transition-all focus:ring-2 focus:ring-offset-2 focus:ring-slate-900 disabled:opacity-70 flex items-center justify-center gap-2"
                    >
                      <Show when={isSavingAdvanced()}>
                        <div class="animate-spin w-4 h-4 border-2 border-current border-t-transparent rounded-full"></div>
                      </Show>
                      <Show when={!isSavingAdvanced()}>Apply Configuration</Show>
                    </button>
                  </div>

                </div>
              </div>
            </Show>
          </div>
        </div>
      </div>

      {/* Live patrol streaming status bar */}
      <Show when={patrolStream.isStreaming()}>
        <div class="flex-shrink-0 bg-blue-50 dark:bg-blue-900 border-b border-blue-200 dark:border-blue-800 px-4 py-2">
          <div class="flex items-center gap-3 text-sm">
            <div class="flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-blue-500 animate-pulse" />
              <span class="font-medium text-blue-800 dark:text-blue-200">Patrol running</span>
            </div>
            <Show when={patrolStream.phase()}>
              <span class="text-blue-700 dark:text-blue-300">{patrolStream.phase()}</span>
            </Show>
            <Show when={patrolStream.currentTool()}>
              <span class="text-blue-600 dark:text-blue-400 font-mono text-xs bg-blue-100 dark:bg-blue-900 px-1.5 py-0.5 rounded">
                {patrolStream.currentTool()}
              </span>
            </Show>
            <Show when={patrolStream.tokens() > 0}>
              <span class="text-blue-500 dark:text-blue-400 text-xs ml-auto">
                {patrolStream.tokens().toLocaleString()} tokens
              </span>
            </Show>
          </div>
        </div>
      </Show>

      <Show when={licenseRequired() && !showBlockedBanner()}>
        <div class="flex-shrink-0 bg-blue-50 dark:bg-blue-900 border-b border-blue-200 dark:border-blue-800 px-3 py-2">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs text-blue-700 dark:text-blue-300">
              <a class="text-indigo-600 dark:text-indigo-400 font-semibold hover:underline" href={upgradeUrl()} target="_blank" rel="noopener noreferrer" onClick={() => trackUpgradeClicked('ai_intelligence_banner', 'ai_autofix')}>Upgrade to Pro</a>
              {' '}to unlock automatic fixes and alert-triggered analysis.
            </p>
          </div>
        </div>
      </Show>

      <Show when={showBlockedBanner()}>
        <div class="flex-shrink-0 bg-amber-50 dark:bg-amber-900 border-b border-amber-200 dark:border-amber-800 px-4 py-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 p-1.5 bg-amber-100 dark:bg-amber-900 rounded-md">
                <ShieldAlertIcon class="w-4 h-4 text-amber-600 dark:text-amber-400" />
              </div>
              <div>
                <p class="text-sm font-semibold text-amber-900 dark:text-amber-100">
                  Patrol paused
                </p>
                <p class="text-xs text-amber-700 dark:text-amber-300">
                  {blockedReason()}
                </p>
                <Show when={blockedAt()}>
                  <p class="text-[10px] text-amber-700 dark:text-amber-300">
                    Blocked {formatRelativeTime(blockedAt(), { compact: true })}
                  </p>
                </Show>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <a
                href="/settings/system-ai"
                class="inline-flex items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold text-amber-900 dark:text-amber-100 bg-amber-100 dark:bg-amber-900 border border-amber-200 dark:border-amber-700 rounded-md hover:bg-amber-200 dark:hover:bg-amber-900 transition-colors"
              >
                <SettingsIcon class="w-3.5 h-3.5" />
                Open AI Settings
              </a>
              <Show when={licenseRequired()}>
                <a
                  href={upgradeUrl()}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold text-white bg-amber-600 hover:bg-amber-700 rounded-md transition-colors"
                >
                  <SparklesIcon class="w-3.5 h-3.5" />
                  Upgrade
                </a>
              </Show>
            </div>
          </div>
        </div>
      </Show>



      {/* Content */}
      <div class={`flex-1 overflow-auto p-4 transition-opacity ${!patrolEnabledLocal() ? 'opacity-50 pointer-events-none' : ''}`}>
        <div class="space-y-4">
          {/* Approval Banner */}
          <ApprovalBanner
            onScrollToFinding={(findingId) => {
              setActiveTab('findings');
              setFindingsFilterOverride('approvals');
              // Allow SolidJS to re-render with new filter before scrolling
              clearScrollToFindingTimer();
              scrollToFindingTimerRef = setTimeout(() => {
                scrollToFindingTimerRef = undefined;
                const el = document.getElementById(`finding-${findingId}`);
                el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
                findingScrollTimerRef = undefined;
              }, 100);
            }}
          />

          {/* Status Bar (replaces Activity tab) */}
          <PatrolStatusBar
            enabled={patrolEnabledLocal()}
            refreshTrigger={activityRefreshTrigger()}
          />

          {/* Summary Cards */}
          <Show
            when={summaryStats().criticalFindings > 0 || summaryStats().warningFindings > 0 || summaryStats().fixedCount > 0}
            fallback={
              <Show when={patrolStatus()?.last_patrol_at}>
                <div class="flex items-center gap-2 px-4 py-3 bg-surface rounded-md border border-border">
                  <CheckCircleIcon class="w-4 h-4 text-green-500 dark:text-green-400" />
                  <span class="text-sm text-muted">No issues found</span>
                </div>
              </Show>
            }
          >
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              {/* Critical */}
              <div class="bg-surface rounded-md border border-border p-3">
                <div class="flex items-center gap-2">
                  <div class={`p-1.5 rounded-md border ${summaryStats().criticalFindings > 0
                    ? 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800'
                    : 'bg-surface border-border'
                    }`}>
                    <ShieldAlertIcon class={`w-4 h-4 ${summaryStats().criticalFindings > 0
                      ? 'text-red-500 dark:text-red-400'
                      : 'text-muted'
                      }`} />
                  </div>
                  <div>
                    <p class="text-xs text-muted">Critical</p>
                    <p class={`text-lg font-bold ${summaryStats().criticalFindings > 0
                      ? 'text-red-600 dark:text-red-400'
                      : 'text-muted'
                      }`}>
                      {summaryStats().criticalFindings}
                    </p>
                  </div>
                </div>
              </div>

              {/* Warnings */}
              <div class="bg-surface rounded-md border border-border p-3">
                <div class="flex items-center gap-2">
                  <div class={`p-1.5 rounded-md border ${summaryStats().warningFindings > 0
                    ? 'bg-amber-50 dark:bg-amber-900 border-amber-200 dark:border-amber-800'
                    : 'bg-surface border-border'
                    }`}>
                    <ActivityIcon class={`w-4 h-4 ${summaryStats().warningFindings > 0
                      ? 'text-amber-500 dark:text-amber-400'
                      : 'text-muted'
                      }`} />
                  </div>
                  <div>
                    <p class="text-xs text-muted">Warnings</p>
                    <p class={`text-lg font-bold ${summaryStats().warningFindings > 0
                      ? 'text-amber-600 dark:text-amber-400'
                      : 'text-muted'
                      }`}>
                      {summaryStats().warningFindings}
                    </p>
                  </div>
                </div>
              </div>

              {/* Fixed (issues resolved by Patrol) */}
              <div class="bg-surface rounded-md border border-border p-3">
                <div class="flex items-center gap-2">
                  <div class={`p-1.5 rounded-md border ${summaryStats().fixedCount > 0
                    ? 'bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800'
                    : 'bg-surface border-border'
                    }`}>
                    <CheckCircleIcon class={`w-4 h-4 ${summaryStats().fixedCount > 0
                      ? 'text-green-500 dark:text-green-400'
                      : 'text-muted'
                      }`} />
                  </div>
                  <div>
                    <p class="text-xs text-muted">Fixed</p>
                    <p class={`text-lg font-bold ${summaryStats().fixedCount > 0
                      ? 'text-green-600 dark:text-green-400'
                      : 'text-muted'
                      }`}>
                      {summaryStats().fixedCount}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </Show>

          {/* Tab Bar */}
          <div class="flex items-center gap-1 border-b border-border">
            <button
              type="button"
              onClick={() => setActiveTab('findings')}
              class={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${activeTab() === 'findings'
                ? 'border-blue-500 text-base-content'
                : 'border-transparent text-muted hover:text-slate-700 dark:hover:text-slate-300 hover:border-slate-300 dark:hover:border-slate-600'
                }`}
            >
              Findings
              <Show when={summaryStats().totalActive > 0}>
                <span class={`ml-1.5 px-1.5 py-0.5 text-xs rounded-full ${summaryStats().criticalFindings > 0
                  ? 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                  : 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'
                  }`}>
                  {summaryStats().totalActive}
                </span>
              </Show>
            </button>
            <button
              type="button"
              onClick={() => { setActiveTab('history'); setFindingsFilterOverride(undefined); }}
              class={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${activeTab() === 'history'
                ? 'border-blue-500 text-base-content'
                : 'border-transparent text-muted hover:text-slate-700 dark:hover:text-slate-300 hover:border-slate-300 dark:hover:border-slate-600'
                }`}
            >
              Run History
              <Show when={displayRunHistory().length > 0}>
                <span class="ml-1.5 px-1.5 py-0.5 text-xs rounded-full bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300">
                  {displayRunHistory().length}
                </span>
              </Show>
            </button>
          </div>

          {/* Tab Content */}
          <Show when={activeTab() === 'findings'}>
            <Show when={selectedRun()}>
              {(run) => (
                <div class="flex items-center justify-between px-3 py-2 rounded-md bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 text-xs text-blue-700 dark:text-blue-300">
                  <span>
                    Filtered to run {formatRelativeTime(run().started_at, { compact: true })} ({formatTriggerReason(run().trigger_reason)})
                  </span>
                  <button
                    type="button"
                    onClick={() => setSelectedRun(null)}
                    class="font-medium hover:underline"
                  >
                    Clear filter
                  </button>
                </div>
              )}
            </Show>

            <FindingsPanel
              nextPatrolAt={patrolStatus()?.next_patrol_at}
              lastPatrolAt={patrolStatus()?.last_patrol_at}
              patrolIntervalMs={patrolStatus()?.interval_ms}
              filterOverride={selectedRunFindingIds() ? 'all' : findingsFilterOverride()}
              filterFindingIds={selectedRunFindingIds() ?? undefined}
              scopeResourceIds={selectedRun()?.scope_resource_ids}
              scopeResourceTypes={selectedRun()?.scope_resource_types}
              showScopeWarnings={Boolean(selectedRunFindingIds()?.length)}
            />
          </Show>

          <Show when={activeTab() === 'history'}>
            <RunHistoryPanel
              runs={displayRunHistory()}
              loading={patrolRunHistory.loading}
              selectedRun={selectedRun()}
              onSelectRun={setSelectedRun}
              patrolStream={patrolStream}
            />
          </Show>
        </div>
      </div>

    </div>
  );
}

export default AIIntelligence;
