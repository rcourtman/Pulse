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
import { renderMarkdown } from '@/components/AI/aiChatUtils';

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

// Local patrol enabled state (synced with AI settings)
// We use this instead of patrolStatus().enabled for immediate UI feedback
import BrainCircuitIcon from 'lucide-solid/icons/brain-circuit';
import ActivityIcon from 'lucide-solid/icons/activity';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import PlayIcon from 'lucide-solid/icons/play';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import XIcon from 'lucide-solid/icons/x';
import FlaskConicalIcon from 'lucide-solid/icons/flask-conical';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import SettingsIcon from 'lucide-solid/icons/settings';
import ServerIcon from 'lucide-solid/icons/server';
import MonitorIcon from 'lucide-solid/icons/monitor';
import BoxIcon from 'lucide-solid/icons/box';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import GlobeIcon from 'lucide-solid/icons/globe';
import DatabaseIcon from 'lucide-solid/icons/database';
import SearchIcon from 'lucide-solid/icons/search';
import WrenchIcon from 'lucide-solid/icons/wrench';
import ClockIcon from 'lucide-solid/icons/clock';
import ZapIcon from 'lucide-solid/icons/zap';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import FilterXIcon from 'lucide-solid/icons/filter-x';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { TogglePrimitive, Toggle } from '@/components/shared/Toggle';
import { ApprovalBanner, PatrolStatusBar, RunToolCallTrace } from '@/components/patrol';
import { usePatrolStream } from '@/hooks/usePatrolStream';
import { hasFeature } from '@/stores/license';

const INFO_BANNER_DISMISSED_KEY = 'patrol-info-banner-dismissed';

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
  const [isRefreshing, setIsRefreshing] = createSignal(false);
  const [autonomyLevel, setAutonomyLevel] = createSignal<PatrolAutonomyLevel>('monitor');
  const [isUpdatingAutonomy, setIsUpdatingAutonomy] = createSignal(false);
  const [showInfoBanner, setShowInfoBanner] = createSignal(
    localStorage.getItem(INFO_BANNER_DISMISSED_KEY) !== 'true'
  );
  // Trigger to refresh patrol activity visualizations
  const [activityRefreshTrigger, setActivityRefreshTrigger] = createSignal(0);

  // Optimistic running state — set immediately on "Run Patrol" click to avoid race with backend
  const [manualRunRequested, setManualRunRequested] = createSignal(false);

  // Live patrol streaming
  const patrolStream = usePatrolStream({
    running: () => (patrolStatus()?.running ?? false) || manualRunRequested(),
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
  });

  // AI settings state
  const [availableModels, setAvailableModels] = createSignal<ModelInfo[]>([]);
  const [patrolModel, setPatrolModel] = createSignal<string>('');
  const [defaultModel, setDefaultModel] = createSignal<string>('');
  const [patrolInterval, setPatrolInterval] = createSignal<number>(360);
  const [patrolEnabledLocal, setPatrolEnabledLocal] = createSignal<boolean>(true);
  const [isUpdatingSettings, setIsUpdatingSettings] = createSignal(false);
  const [isTogglingPatrol, setIsTogglingPatrol] = createSignal(false);
  const [isTriggeringPatrol, setIsTriggeringPatrol] = createSignal(false);
  const [alertTriggeredAnalysis, setAlertTriggeredAnalysis] = createSignal<boolean>(true);
  const [patrolAutoFix, setPatrolAutoFix] = createSignal<boolean>(false);
  const [autoFixModel, setAutoFixModel] = createSignal<string>('');

  // Re-apply patrol model select value when models load after settings
  // (select value is ignored by the browser if no matching option exists yet)
  createEffect(() => {
    const model = patrolModel();
    const models = availableModels();
    if (patrolModelSelectRef && models.length > 0 && model) {
      patrolModelSelectRef.value = model;
    }
  });

  // License feature gates
  const alertAnalysisLocked = createMemo(() => !hasFeature('ai_alerts'));
  const autoFixLocked = createMemo(() => !hasFeature('ai_autofix'));
  const [selectedRun, setSelectedRun] = createSignal<PatrolRunRecord | null>(null);
  const [showRunAnalysis, setShowRunAnalysis] = createSignal(true);
  // TODO: Wire up scope context and token usage display in patrol run details
  // const scopeContext = createMemo(() => splitScopeContext(selectedRun()?.scope_context));
  // const runTokenUsage = createMemo(() => formatTokenUsage(selectedRun()));
  // TODO: Wire up selected run findings display
  // const selectedRunFindings = createMemo(() => { ... });
  // TODO: Wire up scope drift detection in patrol run details
  // const scopeDrift = createMemo(() => { ... });

  const scheduleOptions = createMemo(() => {
    const current = patrolInterval();
    const options = [...SCHEDULE_PRESETS];
    if (Number.isFinite(current) && !options.some((opt) => opt.value === current)) {
      options.push({ value: current, label: `${current} min` });
      options.sort((a, b) => a.value - b.value);
    }
    return options;
  });

  function dismissInfoBanner() {
    localStorage.setItem(INFO_BANNER_DISMISSED_KEY, 'true');
    setShowInfoBanner(false);
  }

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
      setPatrolModel(data.patrol_model || '');
      setDefaultModel(data.model || '');
      setPatrolInterval(data.patrol_interval_minutes ?? 360);
      setPatrolEnabledLocal(data.patrol_enabled ?? true);
      setAlertTriggeredAnalysis(data.alert_triggered_analysis !== false);
      setPatrolAutoFix(data.patrol_auto_fix ?? false);
      setAutoFixModel(data.auto_fix_model || '');
    } catch (err) {
      console.error('Failed to load AI settings:', err);
    }
  }

  // Toggle patrol on/off
  async function handleTogglePatrol() {
    if (isTogglingPatrol()) return;
    setIsTogglingPatrol(true);
    const newValue = !patrolEnabledLocal();
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
    } finally {
      setIsTogglingPatrol(false);
    }
  }

  async function handleRunPatrol() {
    if (isTriggeringPatrol() || !canTriggerPatrol() || manualRunRequested() || patrolStream.isStreaming()) return;
    setIsTriggeringPatrol(true);
    setManualRunRequested(true);

    // Safety timeout: if SSE never connects within 15s, clear optimistic state
    const safetyTimer = setTimeout(() => {
      if (manualRunRequested() && !patrolStream.isStreaming()) {
        setManualRunRequested(false);
        loadAllData();
      }
    }, 15000);

    try {
      await triggerPatrolRun();
      await loadAllData();
    } catch (err) {
      console.error('Failed to trigger patrol run:', err);
      setManualRunRequested(false);
      clearTimeout(safetyTimer);
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
    } catch (err) {
      console.error('Failed to update patrol interval:', err);
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
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  // Toggle auto-fix mode
  async function handleAutoFixChange(enabled: boolean) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    const previous = patrolAutoFix();
    setPatrolAutoFix(enabled);
    try {
      await apiFetchJSON('/api/settings/ai/update', {
        method: 'PUT',
        body: JSON.stringify({ patrol_auto_fix: enabled }),
      });
    } catch (err) {
      console.error('Failed to update auto-fix mode:', err);
      setPatrolAutoFix(previous);
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  // Update auto-fix model
  async function handleAutoFixModelChange(modelId: string) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    try {
      await apiFetchJSON('/api/settings/ai/update', {
        method: 'PUT',
        body: JSON.stringify({ auto_fix_model: modelId }),
      });
      setAutoFixModel(modelId);
    } catch (err) {
      console.error('Failed to update auto-fix model:', err);
    } finally {
      setIsUpdatingSettings(false);
    }
  }

  // Strip leaked provider tool-call markup (e.g. DeepSeek DSML) from AI analysis text
  function sanitizeAnalysis(text: string | undefined): string {
    if (!text) return '';
    // Remove DeepSeek DSML blocks: <｜DSML｜...>...</｜DSML｜...>
    return text.replace(/<｜DSML｜[^>]*>[\s\S]*?<\/｜DSML｜[^>]*>/g, '')
               .replace(/<｜DSML｜[^>]*>/g, '')
               .trim();
  }

  // Group models by provider
  function groupModelsByProvider(models: ModelInfo[]) {
    const groups = new Map<string, ModelInfo[]>();
    for (const model of models) {
      const [provider] = model.id.split(':');
      if (!groups.has(provider)) {
        groups.set(provider, []);
      }
      groups.get(provider)!.push(model);
    }
    return groups;
  }

  // Format relative time
  function formatRelativeTime(dateStr: string | undefined): string {
    if (!dateStr) return 'Never';
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(Math.abs(diffMs) / 60000);
    const diffHours = Math.floor(Math.abs(diffMs) / 3600000);

    if (diffMs < 0) {
      // Future time
      if (diffMins < 60) return `in ${diffMins}m`;
      return `in ${diffHours}h`;
    } else {
      // Past time
      if (diffMins < 1) return 'just now';
      if (diffMins < 60) return `${diffMins}m ago`;
      if (diffHours < 24) return `${diffHours}h ago`;
      return date.toLocaleDateString();
    }
  }

  function formatTriggerReason(reason?: string): string {
    switch (reason) {
      case 'scheduled':
        return 'Scheduled';
      case 'manual':
        return 'Manual';
      case 'startup':
        return 'Startup';
      case 'alert_fired':
        return 'Alert fired';
      case 'alert_cleared':
        return 'Alert cleared';
      case 'anomaly':
        return 'Anomaly';
      case 'user_action':
        return 'User action';
      case 'config_changed':
        return 'Config change';
      default:
        return reason ? reason.replace(/_/g, ' ') : 'Unknown';
    }
  }

  function formatScope(run?: PatrolRunRecord | null): string {
    if (!run) return '';
    const idCount = run.scope_resource_ids?.length ?? 0;
    if (idCount > 0) return `Scoped to ${idCount} resource${idCount === 1 ? '' : 's'}`;
    const types = run.scope_resource_types ?? [];
    if (types.length > 0) return `Scoped to ${types.join(', ')}`;
    if (run.type === 'scoped') return 'Scoped';
    return '';
  }

  // TODO: Uncomment when patrol run detail panel is wired up
  // function splitScopeContext(context?: string) { ... }
  // function formatTokenUsage(run?: PatrolRunRecord | null) { ... }

  function formatDurationMs(ms?: number): string {
    if (!ms || ms <= 0) return '';
    if (ms < 1000) return `${ms}ms`;
    const seconds = Math.round(ms / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.round(seconds / 60);
    return `${minutes}m`;
  }


  // Fetch patrol status to check license
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
  const upgradeUrl = createMemo(() => patrolStatus()?.upgrade_url || 'https://pulserelay.pro/');
  const blockedReason = createMemo(() => patrolStatus()?.blocked_reason?.trim() ?? '');
  const blockedAt = createMemo(() => patrolStatus()?.blocked_at);
  const showBlockedBanner = createMemo(() => patrolEnabledLocal() && !!blockedReason());
  const canTriggerPatrol = createMemo(() => patrolEnabledLocal() && !showBlockedBanner());
  const triggerPatrolDisabledReason = createMemo(() => {
    if (!patrolEnabledLocal()) return 'Patrol is disabled';
    if (showBlockedBanner()) return blockedReason() || 'Patrol is paused';
    return '';
  });

  const selectedRunFindingIds = createMemo(() => {
    const run = selectedRun();
    if (!run || !run.finding_ids || run.finding_ids.length === 0) return null;
    return run.finding_ids;
  });

  // Live in-progress run entry for history list
  const liveRunRecord = createMemo<PatrolRunRecord | null>(() => {
    if (!patrolStream.isStreaming() && !manualRunRequested()) return null;
    return {
      id: '__live__',
      started_at: new Date().toISOString(),
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
      setAutonomyLevel(settings.autonomy_level);
      setFullModeUnlocked(settings.full_mode_unlocked);
      setInvestigationBudget(settings.investigation_budget);
      setInvestigationTimeout(settings.investigation_timeout_sec);
    } catch (err) {
      console.error('Failed to load autonomy settings:', err);
    }
  }

  // Update autonomy level (optimistic UI)
  async function handleAutonomyChange(level: PatrolAutonomyLevel) {
    if (isUpdatingAutonomy()) return;

    const previousLevel = autonomyLevel();
    setAutonomyLevel(level); // Optimistic update
    setIsUpdatingAutonomy(true);

    try {
      const currentSettings = await getPatrolAutonomySettings();
      await updatePatrolAutonomySettings({
        ...currentSettings,
        autonomy_level: level,
      });
    } catch (err) {
      console.error('Failed to update autonomy:', err);
      setAutonomyLevel(previousLevel); // Rollback on error
    } finally {
      setIsUpdatingAutonomy(false);
    }
  }

  // Save advanced settings
  async function saveAdvancedSettings() {
    setIsSavingAdvanced(true);
    try {
      const result = await updatePatrolAutonomySettings({
        autonomy_level: autonomyLevel(),
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
    } finally {
      setIsSavingAdvanced(false);
    }
  }

  onMount(async () => {
    await Promise.all([loadAllData(), loadAutonomySettings(), loadModels(), loadAISettings()]);
  });

  let refreshInterval: ReturnType<typeof setInterval>;
  onMount(() => {
    refreshInterval = setInterval(() => {
      loadAllData();
    }, 60000);
  });
  onCleanup(() => clearInterval(refreshInterval));

  // Approval polling: 10s interval for 5-min expiry approvals
  let approvalPollInterval: ReturnType<typeof setInterval>;
  onMount(() => {
    approvalPollInterval = setInterval(() => {
      aiIntelligenceStore.loadPendingApprovals();
    }, 10000);
  });
  onCleanup(() => clearInterval(approvalPollInterval));

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
    const resolvedFindings = patrolFindings.filter(f => f.status === 'resolved');

    const criticalCount = activeFindings.filter(f => f.severity === 'critical').length;
    const warningCount = activeFindings.filter(f => f.severity === 'warning').length;
    const watchCount = activeFindings.filter(f => f.severity === 'watch').length;
    const infoCount = activeFindings.filter(f => f.severity === 'info').length;
    const investigatingCount = patrolFindings.filter(f => f.investigationStatus === 'running').length;
    const totalActive = activeFindings.length;
    const fixedCount = resolvedFindings.length;

    return {
      criticalFindings: criticalCount,
      warningFindings: warningCount,
      watchFindings: watchCount,
      infoFindings: infoCount,
      investigatingCount,
      totalActive,
      fixedCount,
      hasAnyPatrolFindings: patrolFindings.length > 0,
    };
  };

  return (
    <div class="h-full flex flex-col bg-gray-50 dark:bg-gray-900">
      {/* Header */}
      <div class="flex-shrink-0 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-3">
        {/* Top row: Title and refresh */}
        <div class="flex items-center justify-between gap-4 mb-3">
          <div class="flex items-center gap-3">
            <PulsePatrolLogo class="w-6 h-6 text-gray-700 dark:text-gray-200" />
            <div>
              <h1 class="text-lg font-semibold text-gray-900 dark:text-white">Patrol</h1>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                Pulse Patrol monitoring and analysis
              </p>
            </div>
          </div>

          <div class="flex items-center gap-3">
            {/* Last/Next patrol timing - only show if patrol has run */}
            <Show when={patrolStatus()?.last_patrol_at}>
              <div class="hidden sm:flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
                <span>Last: {formatRelativeTime(patrolStatus()?.last_patrol_at)}</span>
                <Show when={patrolStatus()?.next_patrol_at}>
                  <span class="text-gray-300 dark:text-gray-600">|</span>
                  <span>Next: {formatRelativeTime(patrolStatus()?.next_patrol_at)}</span>
                </Show>
              </div>
            </Show>

            {/* Run Patrol Button */}
            <button
              onClick={() => handleRunPatrol()}
              disabled={isTriggeringPatrol() || !canTriggerPatrol() || manualRunRequested() || patrolStream.isStreaming()}
              title={triggerPatrolDisabledReason()}
              class="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 dark:disabled:bg-gray-600 disabled:text-gray-500 rounded-md transition-colors"
            >
              <PlayIcon class={`w-4 h-4 ${(isTriggeringPatrol() || manualRunRequested() || patrolStream.isStreaming()) ? 'animate-pulse' : ''}`} />
              {isTriggeringPatrol() ? 'Starting…' : (manualRunRequested() || patrolStream.isStreaming()) ? 'Running…' : 'Run Patrol'}
            </button>

            {/* Refresh Button */}
            <button
              onClick={() => loadAllData()}
              disabled={isRefreshing()}
              class="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors"
            >
              <RefreshCwIcon class={`w-4 h-4 ${isRefreshing() ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>
        </div>

        {/* Settings row */}
        <div class="flex flex-wrap items-center gap-4">
          {/* Patrol Toggle */}
          <div class="flex items-center gap-2">
            <TogglePrimitive
              checked={patrolEnabledLocal()}
              disabled={isTogglingPatrol()}
              onToggle={handleTogglePatrol}
              size="sm"
              ariaLabel="Toggle Patrol"
            />
            <span class="text-sm text-gray-600 dark:text-gray-400">
              {patrolEnabledLocal() ? 'On' : 'Off'}
            </span>
          </div>

          <div class="h-4 w-px bg-gray-200 dark:bg-gray-700" />

          {/* Model Selector */}
          <div class="flex items-center gap-2">
            <span class="text-xs text-gray-500 dark:text-gray-400">Model:</span>
            <select
              ref={patrolModelSelectRef}
              value={patrolModel()}
              onChange={(e) => handleModelChange(e.currentTarget.value)}
              disabled={isUpdatingSettings() || !patrolEnabledLocal()}
              class="text-xs bg-gray-100 dark:bg-gray-700 border-0 rounded-md py-1 pl-2 pr-6 text-gray-700 dark:text-gray-300 focus:ring-1 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">Default ({defaultModel().split(':').pop() || 'not set'})</option>
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

          <div class="h-4 w-px bg-gray-200 dark:bg-gray-700" />

          {/* Schedule Selector */}
          <div class="flex items-center gap-2">
            <span class="text-xs text-gray-500 dark:text-gray-400">Every:</span>
            <select
              value={patrolInterval()}
              onChange={(e) => handleIntervalChange(parseInt(e.currentTarget.value))}
              disabled={isUpdatingSettings() || !patrolEnabledLocal() || licenseRequired()}
              class="text-xs bg-gray-100 dark:bg-gray-700 border-0 rounded-md py-1 pl-2 pr-6 text-gray-700 dark:text-gray-300 focus:ring-1 focus:ring-blue-500 disabled:opacity-50"
            >
              <For each={scheduleOptions()}>
                {(preset) => (
                  <option value={preset.value}>{preset.label}</option>
                )}
              </For>
            </select>
          </div>

          <div class="h-4 w-px bg-gray-200 dark:bg-gray-700" />

          {/* Autonomy Level Selector */}
          <div class="flex items-center gap-1.5">
            <span class="text-xs text-gray-500 dark:text-gray-400">Mode:</span>
            <div class="flex items-center bg-gray-100 dark:bg-gray-700 rounded-lg p-0.5">
              <For each={(['monitor', 'approval', 'assisted', 'full'] as PatrolAutonomyLevel[])}>
                {(level) => {
                  const isFullLocked = () => level === 'full' && !fullModeUnlocked();
                  const isDisabled = () => !patrolEnabledLocal() || isFullLocked();
                  return (
                    <button
                      onClick={() => handleAutonomyChange(level)}
                      disabled={isDisabled()}
                      title={isFullLocked() ? 'Enable in Advanced Settings (⚙️) first' : undefined}
                      class={`px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${
                        autonomyLevel() === level
                          ? level === 'full'
                            ? 'bg-red-500 dark:bg-red-600 text-white shadow-sm'
                            : 'bg-white dark:bg-gray-600 text-gray-900 dark:text-white shadow-sm'
                          : isFullLocked()
                            ? 'text-gray-400 dark:text-gray-500'
                            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                      } ${isDisabled() ? 'opacity-50 cursor-not-allowed' : ''}`}
                    >
                      {level === 'monitor' ? 'Monitor' : level === 'approval' ? 'Approval' : level === 'assisted' ? 'Assisted' : 'Full'}
                    </button>
                  );
                }}
              </For>
            </div>
            <div class="relative group">
              <CircleHelpIcon class="w-4 h-4 text-gray-400 dark:text-gray-500 cursor-help" />
              <div class="absolute left-0 top-6 z-50 hidden group-hover:block w-72 p-3 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 text-xs">
                <div class="space-y-2">
                  <div>
                    <span class="font-semibold text-gray-900 dark:text-white">Monitor</span>
                    <p class="text-gray-600 dark:text-gray-400">Detect issues only. No automated investigation.</p>
                  </div>
                  <div>
                    <span class="font-semibold text-gray-900 dark:text-white">Approval</span>
                    <p class="text-gray-600 dark:text-gray-400">Patrol investigates findings. All fixes require your approval.</p>
                  </div>
                  <div>
                    <span class="font-semibold text-gray-900 dark:text-white">Assisted</span>
                    <p class="text-gray-600 dark:text-gray-400">Auto-fix warnings. Critical findings still need approval.</p>
                  </div>
                  <div>
                    <span class="font-semibold text-red-600 dark:text-red-400">Full</span>
                    <p class="text-gray-600 dark:text-gray-400">Auto-fix everything, including critical. Must be enabled in ⚙️ settings first.</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Advanced Settings Gear */}
            <div class="relative" ref={advancedSettingsRef}>
                <button
                  onClick={() => setShowAdvancedSettings(!showAdvancedSettings())}
                  disabled={!patrolEnabledLocal()}
                  class={`p-1 rounded transition-colors ${
                    showAdvancedSettings()
                      ? 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30'
                      : 'text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300'
                  } ${!patrolEnabledLocal() ? 'opacity-50 cursor-not-allowed' : ''}`}
                  title="Advanced investigation settings"
                >
                  <SettingsIcon class="w-4 h-4" />
                </button>

                {/* Advanced Settings Popover */}
                <Show when={showAdvancedSettings()}>
                  <div class="absolute right-0 top-8 z-50 w-72 p-4 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700">
                    <div class="flex items-center justify-between mb-3">
                      <h4 class="text-sm font-semibold text-gray-900 dark:text-white">Advanced Settings</h4>
                      <button
                        onClick={() => setShowAdvancedSettings(false)}
                        class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                      >
                        <XIcon class="w-4 h-4" />
                      </button>
                    </div>

                    <div class="space-y-4">
                      {/* Full Mode Unlock */}
                      <div>
                        <div class="flex items-start justify-between gap-3">
                          <div class="flex-1">
                            <label class="text-xs font-medium text-red-600 dark:text-red-400">Enable Full Mode</label>
                            <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5">
                              I understand that Full mode will auto-fix ALL findings including critical issues, without asking for approval.
                            </p>
                          </div>
                          <Toggle
                            checked={fullModeUnlocked()}
                            onChange={(e) => setFullModeUnlocked(e.currentTarget.checked)}
                          />
                        </div>
                        <Show when={fullModeUnlocked()}>
                          <p class="text-[10px] text-amber-600 dark:text-amber-400 mt-2 flex items-center gap-1">
                            <ShieldAlertIcon class="w-3 h-3 flex-shrink-0" />
                            Full mode is available. Click Save to apply.
                          </p>
                        </Show>
                        <Show when={!fullModeUnlocked() && autonomyLevel() === 'full'}>
                          <p class="text-[10px] text-amber-600 dark:text-amber-400 mt-2 flex items-center gap-1">
                            <ShieldAlertIcon class="w-3 h-3 flex-shrink-0" />
                            Saving will downgrade to Assisted mode.
                          </p>
                        </Show>
                      </div>

                      {/* Alert-Triggered Analysis */}
                      <div class="pt-3 border-t border-gray-200 dark:border-gray-700">
                        <div class="flex items-center justify-between gap-3">
                          <div class="flex-1">
                            <label class="text-xs font-medium text-gray-700 dark:text-gray-300 flex items-center gap-1.5">
                              Alert-Triggered Analysis
                              <span class="px-1 py-0.5 text-[9px] font-medium bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">Efficient</span>
                            </label>
                            <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5">
                              Analyze infrastructure when alerts fire.
                            </p>
                          </div>
                          <Toggle
                            checked={alertTriggeredAnalysis()}
                            onChange={(e) => handleAlertTriggeredAnalysisChange(e.currentTarget.checked)}
                            disabled={isUpdatingSettings() || alertAnalysisLocked()}
                          />
                        </div>
                        <Show when={alertAnalysisLocked()}>
                          <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-1">
                            <a class="text-indigo-600 dark:text-indigo-400 font-medium hover:underline" href="https://pulserelay.pro/" target="_blank" rel="noreferrer">Upgrade to Pro</a>
                            {' '}to enable alert-triggered analysis.
                          </p>
                        </Show>
                      </div>

                      {/* Auto-Fix Mode */}
                      <div class="pt-3 border-t border-gray-200 dark:border-gray-700">
                        <div class="flex items-center justify-between gap-3">
                          <div class="flex-1">
                            <label class="text-xs font-medium text-gray-700 dark:text-gray-300 flex items-center gap-1.5">
                              Auto-Fix
                              <span class="px-1 py-0.5 text-[9px] font-medium bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 rounded">Advanced</span>
                            </label>
                            <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5">
                              Allow Patrol to execute fixes automatically.
                            </p>
                          </div>
                          <Toggle
                            checked={patrolAutoFix()}
                            onChange={(e) => handleAutoFixChange(e.currentTarget.checked)}
                            disabled={isUpdatingSettings() || autoFixLocked()}
                          />
                        </div>
                        <Show when={autoFixLocked()}>
                          <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-1">
                            <a class="text-indigo-600 dark:text-indigo-400 font-medium hover:underline" href="https://pulserelay.pro/" target="_blank" rel="noreferrer">Upgrade to Pro</a>
                            {' '}to enable auto-fix.
                          </p>
                        </Show>
                        <Show when={!autoFixLocked() && patrolAutoFix()}>
                          <p class="text-[10px] text-red-600 dark:text-red-400 mt-1 flex items-center gap-1">
                            <AlertTriangleIcon class="w-3 h-3 flex-shrink-0" />
                            Auto-Fix is ON. Patrol will attempt automatic remediation.
                          </p>
                        </Show>
                      </div>

                      {/* Auto-Fix Model - only when auto-fix is enabled */}
                      <Show when={patrolAutoFix() && !autoFixLocked()}>
                        <div>
                          <label class="text-xs text-gray-600 dark:text-gray-400">Fix Model</label>
                          <select
                            value={autoFixModel()}
                            onChange={(e) => handleAutoFixModelChange(e.currentTarget.value)}
                            disabled={isUpdatingSettings()}
                            class="mt-1 w-full text-xs bg-gray-100 dark:bg-gray-700 border-0 rounded-md py-1.5 pl-2 pr-6 text-gray-700 dark:text-gray-300 focus:ring-1 focus:ring-blue-500 disabled:opacity-50"
                          >
                            <option value="">Use patrol model</option>
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
                      </Show>

                      {/* Save Button (for investigation limits + full mode unlock) */}
                      <button
                        onClick={saveAdvancedSettings}
                        disabled={isSavingAdvanced()}
                        class="w-full px-3 py-2 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 rounded-lg transition-colors flex items-center justify-center gap-2"
                      >
                        <Show when={isSavingAdvanced()}>
                          <div class="animate-spin h-3 w-3 border-2 border-white border-t-transparent rounded-full"></div>
                        </Show>
                        <Show when={!isSavingAdvanced()}>Save</Show>
                      </button>
                    </div>
                  </div>
                </Show>
              </div>
          </div>
        </div>
      </div>

      {/* Live patrol streaming status bar */}
      <Show when={patrolStream.isStreaming()}>
        <div class="flex-shrink-0 bg-blue-50 dark:bg-blue-900/20 border-b border-blue-200 dark:border-blue-800 px-4 py-2">
          <div class="flex items-center gap-3 text-sm">
            <div class="flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-blue-500 animate-pulse" />
              <span class="font-medium text-blue-800 dark:text-blue-200">Patrol running</span>
            </div>
            <Show when={patrolStream.phase()}>
              <span class="text-blue-700 dark:text-blue-300">{patrolStream.phase()}</span>
            </Show>
            <Show when={patrolStream.currentTool()}>
              <span class="text-blue-600 dark:text-blue-400 font-mono text-xs bg-blue-100 dark:bg-blue-900/40 px-1.5 py-0.5 rounded">
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
        <div class="flex-shrink-0 bg-blue-50 dark:bg-blue-900/20 border-b border-blue-200 dark:border-blue-800 px-4 py-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 p-1.5 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                <SparklesIcon class="w-4 h-4 text-blue-600 dark:text-blue-400" />
              </div>
              <div>
                <p class="text-sm font-semibold text-blue-900 dark:text-blue-100">
                  Pulse Patrol requires Pulse Pro
                </p>
                <p class="text-xs text-blue-700 dark:text-blue-300">
                  Upgrade to enable AI analysis, investigations, and auto-fix.
                </p>
              </div>
            </div>
            <div class="flex items-center gap-3">
              <a
                href={upgradeUrl()}
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center justify-center gap-2 px-4 py-2 text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors"
              >
                <SparklesIcon class="w-3.5 h-3.5" />
                Upgrade to Pulse Pro
              </a>
              <span class="text-[10px] text-blue-700 dark:text-blue-300">
                Already licensed? Activate in Settings → License.
              </span>
            </div>
          </div>
        </div>
      </Show>

      <Show when={showBlockedBanner()}>
        <div class="flex-shrink-0 bg-amber-50 dark:bg-amber-900/20 border-b border-amber-200 dark:border-amber-800 px-4 py-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 p-1.5 bg-amber-100 dark:bg-amber-900/40 rounded-lg">
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
                  <p class="text-[10px] text-amber-700/80 dark:text-amber-300/80">
                    Blocked {formatRelativeTime(blockedAt())}
                  </p>
                </Show>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <a
                href="/settings/system-ai"
                class="inline-flex items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold text-amber-900 dark:text-amber-100 bg-amber-100 dark:bg-amber-900/40 border border-amber-200 dark:border-amber-700 rounded-lg hover:bg-amber-200/70 dark:hover:bg-amber-900/60 transition-colors"
              >
                <SettingsIcon class="w-3.5 h-3.5" />
                Open AI Settings
              </a>
              <Show when={licenseRequired()}>
                <a
                  href={upgradeUrl()}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold text-white bg-amber-600 hover:bg-amber-700 rounded-lg transition-colors"
                >
                  <SparklesIcon class="w-3.5 h-3.5" />
                  Upgrade
                </a>
              </Show>
            </div>
          </div>
        </div>
      </Show>

      {/* Info Banner */}
      {showInfoBanner() && (
        <div class="flex-shrink-0 bg-gray-50 dark:bg-gray-800/50 border-b border-gray-200 dark:border-gray-700 px-4 py-3">
          <div class="flex items-start gap-3">
            <div class="flex-shrink-0 p-1.5 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
              <FlaskConicalIcon class="w-4 h-4 text-blue-600 dark:text-blue-400" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  Patrol Autonomy
                </h3>
                <span class="px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 rounded">
                  BETA
                </span>
              </div>
              <p class="text-xs text-gray-600 dark:text-gray-300 mb-2">
                <strong>How it works:</strong> Pulse constantly monitors your infrastructure. When alert thresholds
                are crossed, findings are created automatically. In <strong>Approval</strong>, <strong>Assisted</strong>, or <strong>Full</strong> mode,
                Pulse Patrol investigates these findings - querying nodes, checking logs, and running diagnostics to
                identify root causes. It then suggests fixes (Approval), applies safe fixes (Assisted), or applies all fixes (Full).
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                This is experimental. In Assisted mode, critical findings still require approval. Full mode (requires unlock in ⚙️) auto-fixes everything.
              </p>
            </div>
            <button
              onClick={dismissInfoBanner}
              class="flex-shrink-0 p-1 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
              title="Dismiss"
            >
              <XIcon class="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* Content */}
      <div class={`flex-1 overflow-auto p-4 transition-opacity ${!patrolEnabledLocal() ? 'opacity-50 pointer-events-none' : ''}`}>
        <div class="space-y-4">
          {/* Approval Banner */}
          <ApprovalBanner
            onScrollToFinding={(findingId) => {
              setActiveTab('findings');
              // Scroll to the finding after tab switch
              requestAnimationFrame(() => {
                const el = document.getElementById(`finding-${findingId}`);
                el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
              });
            }}
          />

          {/* Status Bar (replaces Activity tab) */}
          <PatrolStatusBar
            enabled={patrolEnabledLocal()}
            refreshTrigger={activityRefreshTrigger()}
          />

          {/* Summary Cards */}
          <div class="grid grid-cols-2 lg:grid-cols-5 gap-3">
            {/* Critical */}
            <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <div class="flex items-center gap-2">
                <div class={`p-1.5 rounded ${
                  summaryStats().criticalFindings > 0
                    ? 'bg-red-100 dark:bg-red-900/30'
                    : 'bg-gray-100 dark:bg-gray-700'
                }`}>
                  <ShieldAlertIcon class={`w-4 h-4 ${
                    summaryStats().criticalFindings > 0
                      ? 'text-red-600 dark:text-red-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`} />
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">Critical</p>
                  <p class={`text-lg font-bold ${
                    summaryStats().criticalFindings > 0
                      ? 'text-red-600 dark:text-red-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`}>
                    {summaryStats().criticalFindings}
                  </p>
                </div>
              </div>
            </div>

            {/* Warnings */}
            <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <div class="flex items-center gap-2">
                <div class={`p-1.5 rounded ${
                  summaryStats().warningFindings > 0
                    ? 'bg-amber-100 dark:bg-amber-900/30'
                    : 'bg-gray-100 dark:bg-gray-700'
                }`}>
                  <ActivityIcon class={`w-4 h-4 ${
                    summaryStats().warningFindings > 0
                      ? 'text-amber-600 dark:text-amber-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`} />
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">Warnings</p>
                  <p class={`text-lg font-bold ${
                    summaryStats().warningFindings > 0
                      ? 'text-amber-600 dark:text-amber-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`}>
                    {summaryStats().warningFindings}
                  </p>
                </div>
              </div>
            </div>

            {/* Investigating (only meaningful in Approval/Auto mode) */}
            <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <div class="flex items-center gap-2">
                <div class={`p-1.5 rounded ${
                  summaryStats().investigatingCount > 0
                    ? 'bg-blue-100 dark:bg-blue-900/30'
                    : 'bg-gray-100 dark:bg-gray-700'
                }`}>
                  <BrainCircuitIcon class={`w-4 h-4 ${
                    summaryStats().investigatingCount > 0
                      ? 'text-blue-600 dark:text-blue-400 animate-pulse'
                      : 'text-gray-400 dark:text-gray-500'
                  }`} />
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {autonomyLevel() === 'monitor' ? 'Investigating' : 'Investigating'}
                  </p>
                  <p class={`text-lg font-bold ${
                    summaryStats().investigatingCount > 0
                      ? 'text-blue-600 dark:text-blue-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`}>
                    {autonomyLevel() === 'monitor' ? '—' : summaryStats().investigatingCount}
                  </p>
                </div>
              </div>
            </div>

            {/* Awaiting Approval */}
            <div class={`rounded-lg border p-3 ${
              aiIntelligenceStore.pendingApprovalCount > 0
                ? 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-700'
                : 'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700'
            }`}>
              <div class="flex items-center gap-2">
                <div class={`p-1.5 rounded ${
                  aiIntelligenceStore.pendingApprovalCount > 0
                    ? 'bg-amber-100 dark:bg-amber-900/30'
                    : 'bg-gray-100 dark:bg-gray-700'
                }`}>
                  <ShieldAlertIcon class={`w-4 h-4 ${
                    aiIntelligenceStore.pendingApprovalCount > 0
                      ? 'text-amber-600 dark:text-amber-400 animate-pulse'
                      : 'text-gray-400 dark:text-gray-500'
                  }`} />
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">Awaiting Approval</p>
                  <p class={`text-lg font-bold ${
                    aiIntelligenceStore.pendingApprovalCount > 0
                      ? 'text-amber-600 dark:text-amber-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`}>
                    {aiIntelligenceStore.pendingApprovalCount}
                  </p>
                </div>
              </div>
            </div>

            {/* Fixed (issues resolved by Patrol) */}
            <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <div class="flex items-center gap-2">
                <div class={`p-1.5 rounded ${
                  summaryStats().fixedCount > 0
                    ? 'bg-green-100 dark:bg-green-900/30'
                    : 'bg-gray-100 dark:bg-gray-700'
                }`}>
                  <CheckCircleIcon class={`w-4 h-4 ${
                    summaryStats().fixedCount > 0
                      ? 'text-green-600 dark:text-green-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`} />
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">Fixed</p>
                  <p class={`text-lg font-bold ${
                    summaryStats().fixedCount > 0
                      ? 'text-green-600 dark:text-green-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`}>
                    {summaryStats().fixedCount}
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Tab Bar */}
          <div class="flex items-center gap-1 border-b border-gray-200 dark:border-gray-700">
            <button
              type="button"
              onClick={() => setActiveTab('findings')}
              class={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab() === 'findings'
                  ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
              }`}
            >
              Findings
              <Show when={summaryStats().totalActive > 0}>
                <span class={`ml-1.5 px-1.5 py-0.5 text-xs rounded-full ${
                  summaryStats().criticalFindings > 0
                    ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                }`}>
                  {summaryStats().totalActive}
                </span>
              </Show>
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('history')}
              class={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab() === 'history'
                  ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
              }`}
            >
              Run History
              <Show when={displayRunHistory().length > 0}>
                <span class="ml-1.5 px-1.5 py-0.5 text-xs rounded-full bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                  {displayRunHistory().length}
                </span>
              </Show>
            </button>
          </div>

          {/* Tab Content */}
          <Show when={activeTab() === 'findings'}>
            <Show when={selectedRun()}>
              {(run) => (
                <div class="flex items-center justify-between px-3 py-2 rounded-md bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 text-xs text-blue-700 dark:text-blue-300">
                  <span>
                    Filtered to run {formatRelativeTime(run().started_at)} ({formatTriggerReason(run().trigger_reason)})
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
              filterOverride={selectedRunFindingIds() ? 'all' : undefined}
              filterFindingIds={selectedRunFindingIds() ?? undefined}
              scopeResourceIds={selectedRun()?.scope_resource_ids}
              scopeResourceTypes={selectedRun()?.scope_resource_types}
              showScopeWarnings={Boolean(selectedRunFindingIds()?.length)}
            />
          </Show>

          <Show when={activeTab() === 'history'}>
            <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
              <div class="flex items-center justify-between mb-4">
                <div>
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Patrol Run History</h2>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    Select a run to filter findings to that snapshot
                  </p>
                </div>
                <Show when={selectedRun()}>
                  <button
                    type="button"
                    onClick={() => setSelectedRun(null)}
                    class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    Clear filter
                  </button>
                </Show>
              </div>

              <Show when={patrolRunHistory.loading}>
                <div class="text-xs text-gray-500 dark:text-gray-400">Loading run history…</div>
              </Show>

              <Show when={!patrolRunHistory.loading && displayRunHistory().length === 0}>
                <div class="text-center py-8">
                  <RefreshCwIcon class="w-12 h-12 mx-auto text-gray-300 dark:text-gray-600 mb-3" />
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    No patrol runs yet. Trigger a run to populate history.
                  </p>
                </div>
              </Show>

              <Show when={!patrolRunHistory.loading && displayRunHistory().length > 0}>
                <div class="space-y-2">
                  <For each={displayRunHistory()}>
                    {(run) => {
                      // Live in-progress entry
                      if (run.id === '__live__') {
                        const hasError = () => !!patrolStream.errorMessage();
                        return (
                          <div class={`rounded-md border transition-colors ${
                            hasError()
                              ? 'border-red-300 dark:border-red-700 bg-red-50 dark:bg-red-900/20'
                              : 'border-blue-300 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20'
                          }`}>
                            <div class="px-3 py-2">
                              <div class="flex flex-wrap items-center gap-2 text-xs">
                                <Show when={!hasError()}>
                                  <span class="relative flex h-2.5 w-2.5">
                                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75" />
                                    <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-blue-500" />
                                  </span>
                                  <span class="font-medium text-blue-800 dark:text-blue-200">Running now</span>
                                </Show>
                                <Show when={hasError()}>
                                  <ShieldAlertIcon class="w-3.5 h-3.5 text-red-500" />
                                  <span class="font-medium text-red-800 dark:text-red-200">Error</span>
                                </Show>
                                <Show when={!hasError() && patrolStream.phase()}>
                                  <span class="text-blue-700 dark:text-blue-300">{patrolStream.phase()}</span>
                                </Show>
                                <Show when={!hasError() && patrolStream.currentTool()}>
                                  <span class="font-mono text-[11px] bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-400 px-1.5 py-0.5 rounded">
                                    {patrolStream.currentTool()}
                                  </span>
                                </Show>
                                <Show when={!hasError() && patrolStream.tokens() > 0}>
                                  <span class="text-blue-500 dark:text-blue-400 ml-auto">
                                    {patrolStream.tokens().toLocaleString()} tokens
                                  </span>
                                </Show>
                              </div>
                              <Show when={hasError()}>
                                <p class="mt-1.5 text-xs text-red-700 dark:text-red-300">
                                  {patrolStream.errorMessage()}
                                </p>
                              </Show>
                            </div>
                          </div>
                        );
                      }
                      const scopeSummary = formatScope(run);
                      const duration = formatDurationMs(run.duration_ms);
                      const isSelected = () => selectedRun()?.id === run.id;
                      return (
                        <div class={`rounded-md border transition-colors ${
                          isSelected()
                            ? 'border-blue-300 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20'
                            : 'border-gray-200 dark:border-gray-700'
                        }`}>
                          <button
                            type="button"
                            onClick={() => setSelectedRun(isSelected() ? null : run)}
                            class={`w-full text-left px-3 py-2 rounded-md transition-colors ${
                              !isSelected() ? 'hover:bg-gray-50 dark:hover:bg-gray-700/40' : ''
                            }`}
                          >
                            <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                              <span class="text-gray-900 dark:text-gray-100 font-medium">
                                {formatRelativeTime(run.started_at)}
                              </span>
                              <span class={`px-1.5 py-0.5 rounded ${
                                run.status === 'critical'
                                  ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                                  : run.status === 'issues_found'
                                    ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                                    : run.status === 'error'
                                      ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                                      : 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                              }`}>
                                {run.status.replace(/_/g, ' ')}
                              </span>
                              <span>{formatTriggerReason(run.trigger_reason)}</span>
                              <Show when={scopeSummary}>
                                <span>• {scopeSummary}</span>
                              </Show>
                              <Show when={duration}>
                                <span>• {duration}</span>
                              </Show>
                              <Show when={run.resources_checked}>
                                <span>• {run.resources_checked} resources</span>
                              </Show>
                              <Show when={run.new_findings}>
                                <span>• {run.new_findings} new</span>
                              </Show>
                              <Show when={run.rejected_findings}>
                                <span class="text-gray-400 dark:text-gray-500">• {run.rejected_findings} rejected</span>
                              </Show>
                            </div>
                          </button>

                          {/* Inline expansion details */}
                          <Show when={isSelected()}>
                            <div class="px-3 pb-3 border-t border-blue-200 dark:border-blue-800 mt-0">

                              {/* Section 1: Narrative Summary */}
                              <div class="mt-3 flex items-start gap-2 text-sm text-gray-700 dark:text-gray-200">
                                <SparklesIcon class="w-4 h-4 text-blue-500 dark:text-blue-400 mt-0.5 flex-shrink-0" />
                                <p>
                                  {run.resources_checked > 0
                                    ? <>Scanned <strong>{run.resources_checked}</strong> resource{run.resources_checked !== 1 ? 's' : ''}{' '}
                                        {formatDurationMs(run.duration_ms) ? <>in <strong>{formatDurationMs(run.duration_ms)}</strong></> : ''}{' '}
                                        {run.tool_call_count > 0 ? <>using <strong>{run.tool_call_count}</strong> tool call{run.tool_call_count !== 1 ? 's' : ''}</> : ''}.{' '}
                                      </>
                                    : <>Patrol completed{formatDurationMs(run.duration_ms) ? <> in <strong>{formatDurationMs(run.duration_ms)}</strong></> : ''}.{' '}</>
                                  }
                                  {run.new_findings > 0
                                    ? <>Found <strong>{run.new_findings}</strong> new issue{run.new_findings !== 1 ? 's' : ''}{run.auto_fix_count > 0 ? <>, auto-fixed <strong>{run.auto_fix_count}</strong></> : ''}.</>
                                    : <span class="text-green-600 dark:text-green-400">All clear — no new issues.</span>
                                  }
                                </p>
                              </div>

                              {/* Section 2: Resources Scanned */}
                              <Show when={run.resources_checked > 0}>
                                <div class="mt-3">
                                  <div class="flex items-center gap-1.5 mb-2">
                                    <SearchIcon class="w-3.5 h-3.5 text-gray-400 dark:text-gray-500" />
                                    <span class="text-[10px] font-semibold tracking-wider uppercase text-gray-500 dark:text-gray-400">
                                      Resources Scanned ({run.resources_checked})
                                    </span>
                                  </div>
                                  <div class="flex flex-wrap gap-1.5">
                                    <Show when={run.nodes_checked > 0}>
                                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                                        <ServerIcon class="w-3 h-3" /> {run.nodes_checked} node{run.nodes_checked !== 1 ? 's' : ''}
                                      </span>
                                    </Show>
                                    <Show when={run.guests_checked > 0}>
                                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-purple-50 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300">
                                        <MonitorIcon class="w-3 h-3" /> {run.guests_checked} VM{run.guests_checked !== 1 ? 's' : ''}
                                      </span>
                                    </Show>
                                    <Show when={run.docker_checked > 0}>
                                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-cyan-50 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300">
                                        <BoxIcon class="w-3 h-3" /> {run.docker_checked} container{run.docker_checked !== 1 ? 's' : ''}
                                      </span>
                                    </Show>
                                    <Show when={run.storage_checked > 0}>
                                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                                        <HardDriveIcon class="w-3 h-3" /> {run.storage_checked} storage
                                      </span>
                                    </Show>
                                    <Show when={run.hosts_checked > 0}>
                                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-300">
                                        <GlobeIcon class="w-3 h-3" /> {run.hosts_checked} host{run.hosts_checked !== 1 ? 's' : ''}
                                      </span>
                                    </Show>
                                    <Show when={run.pbs_checked > 0}>
                                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300">
                                        <DatabaseIcon class="w-3 h-3" /> {run.pbs_checked} PBS
                                      </span>
                                    </Show>
                                    <Show when={run.kubernetes_checked > 0}>
                                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">
                                        <ActivityIcon class="w-3 h-3" /> {run.kubernetes_checked} K8s
                                      </span>
                                    </Show>
                                  </div>
                                </div>
                              </Show>

                              {/* Section 3: Outcomes */}
                              <div class="mt-3">
                                <div class="flex items-center gap-1.5 mb-2">
                                  <ShieldAlertIcon class="w-3.5 h-3.5 text-gray-400 dark:text-gray-500" />
                                  <span class="text-[10px] font-semibold tracking-wider uppercase text-gray-500 dark:text-gray-400">
                                    Outcomes
                                  </span>
                                </div>
                                <div class="flex flex-wrap gap-1.5">
                                  <Show when={run.new_findings > 0}>
                                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300">
                                      <AlertTriangleIcon class="w-3 h-3" /> {run.new_findings} new
                                    </span>
                                  </Show>
                                  <Show when={run.existing_findings > 0}>
                                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                                      <ActivityIcon class="w-3 h-3" /> {run.existing_findings} existing
                                    </span>
                                  </Show>
                                  <Show when={run.resolved_findings > 0}>
                                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300">
                                      <CheckCircleIcon class="w-3 h-3" /> {run.resolved_findings} resolved
                                    </span>
                                  </Show>
                                  <Show when={run.auto_fix_count > 0}>
                                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300">
                                      <WrenchIcon class="w-3 h-3" /> {run.auto_fix_count} auto-fixed
                                    </span>
                                  </Show>
                                  <Show when={run.rejected_findings > 0}>
                                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-gray-50 text-gray-400 dark:bg-gray-800 dark:text-gray-500">
                                      <FilterXIcon class="w-3 h-3" /> {run.rejected_findings} rejected
                                    </span>
                                  </Show>
                                  <Show when={run.status === 'healthy' && run.new_findings === 0}>
                                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300">
                                      <CheckCircleIcon class="w-3 h-3" /> All clear
                                    </span>
                                  </Show>
                                </div>
                              </div>

                              {/* Section 4: AI Effort Bar */}
                              <div class="mt-3 flex flex-wrap items-center gap-3 px-3 py-2 rounded-md bg-gray-50 dark:bg-gray-900/50 text-xs text-gray-500 dark:text-gray-400">
                                <Show when={formatDurationMs(run.duration_ms)}>
                                  <span class="inline-flex items-center gap-1">
                                    <ClockIcon class="w-3.5 h-3.5" /> {formatDurationMs(run.duration_ms)}
                                  </span>
                                </Show>
                                <Show when={run.tool_call_count > 0}>
                                  <span class="inline-flex items-center gap-1">
                                    <ZapIcon class="w-3.5 h-3.5" /> {run.tool_call_count} tool call{run.tool_call_count !== 1 ? 's' : ''}
                                  </span>
                                </Show>
                                <Show when={(run.input_tokens || 0) + (run.output_tokens || 0) > 0}>
                                  <span class="inline-flex items-center gap-1">
                                    <BrainCircuitIcon class="w-3.5 h-3.5" /> {((run.input_tokens || 0) + (run.output_tokens || 0)).toLocaleString()} tokens
                                  </span>
                                </Show>
                                <Show when={run.type === 'scoped'}>
                                  <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400 text-[10px] font-medium">
                                    {formatScope(run) || 'Scoped'}
                                  </span>
                                </Show>
                              </div>

                              {/* Section 5: Patrol Analysis */}
                              <Show when={run.ai_analysis}>
                                <div class="mt-3">
                                  <div class="flex items-center justify-between">
                                    <div class="flex items-center gap-1.5">
                                      <BrainCircuitIcon class="w-3.5 h-3.5 text-gray-400 dark:text-gray-500" />
                                      <span class="text-[10px] font-semibold tracking-wider uppercase text-gray-500 dark:text-gray-400">
                                        Patrol Analysis
                                      </span>
                                    </div>
                                    <button
                                      type="button"
                                      onClick={() => setShowRunAnalysis(!showRunAnalysis())}
                                      class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                                    >
                                      {showRunAnalysis() ? 'Collapse' : 'Expand'}
                                    </button>
                                  </div>
                                  <Show when={showRunAnalysis()}>
                                    <div
                                      class="mt-2 p-3 rounded bg-white dark:bg-gray-900 text-sm leading-relaxed text-gray-700 dark:text-gray-200 max-h-64 overflow-auto prose prose-sm dark:prose-invert prose-headings:text-sm prose-headings:mt-2 prose-headings:mb-1 prose-p:my-1 prose-ul:my-1 prose-li:my-0"
                                      innerHTML={renderMarkdown(sanitizeAnalysis(run.ai_analysis))}
                                    />
                                  </Show>
                                </div>
                              </Show>

                              {/* Section 6: Tool Call Trace */}
                              <RunToolCallTrace runId={run.id} toolCallCount={run.tool_call_count} />

                              {/* Section 7: Inline Findings */}
                              <Show when={run.finding_ids?.length}>
                                <div class="mt-3 pt-3 border-t border-blue-200 dark:border-blue-800">
                                  <FindingsPanel
                                    filterFindingIds={run.finding_ids}
                                    filterOverride="all"
                                    showControls={false}
                                  />
                                </div>
                              </Show>
                            </div>
                          </Show>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </Show>

            </div>
          </Show>
        </div>
      </div>

    </div>
  );
}

export default AIIntelligence;
