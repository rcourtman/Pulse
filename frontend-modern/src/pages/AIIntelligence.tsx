/**
 * Patrol Page
 *
 * Central hub for Patrol intelligence - AI-powered findings with investigation support.
 */

import { createSignal, createEffect, onMount, onCleanup, createMemo, createResource, For, Show } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { UnifiedFindingsPanel } from '@/components/AI/UnifiedFindingsPanel';
import {
  getPatrolStatus,
  getPatrolAutonomySettings,
  updatePatrolAutonomySettings,
  type PatrolStatus,
  type PatrolAutonomyLevel,
} from '@/api/patrol';
import { apiFetchJSON } from '@/utils/apiClient';

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
}

// Local patrol enabled state (synced with AI settings)
// We use this instead of patrolStatus().enabled for immediate UI feedback
import BrainCircuitIcon from 'lucide-solid/icons/brain-circuit';
import ActivityIcon from 'lucide-solid/icons/activity';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import XIcon from 'lucide-solid/icons/x';
import FlaskConicalIcon from 'lucide-solid/icons/flask-conical';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import SettingsIcon from 'lucide-solid/icons/settings';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { TogglePrimitive, Toggle } from '@/components/shared/Toggle';

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

export function AIIntelligence() {
  const [isRefreshing, setIsRefreshing] = createSignal(false);
  const [autonomyLevel, setAutonomyLevel] = createSignal<PatrolAutonomyLevel>('monitor');
  const [isUpdatingAutonomy, setIsUpdatingAutonomy] = createSignal(false);
  const [showInfoBanner, setShowInfoBanner] = createSignal(
    localStorage.getItem(INFO_BANNER_DISMISSED_KEY) !== 'true'
  );

  // Advanced autonomy settings
  const [investigationBudget, setInvestigationBudget] = createSignal(15);
  const [investigationTimeout, setInvestigationTimeout] = createSignal(300);
  const [criticalRequireApproval, setCriticalRequireApproval] = createSignal(true);
  const [showAdvancedSettings, setShowAdvancedSettings] = createSignal(false);
  const [isSavingAdvanced, setIsSavingAdvanced] = createSignal(false);
  let advancedSettingsRef: HTMLDivElement | undefined;

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
      setAvailableModels(data.models || []);
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
      const data = await apiFetchJSON<AISettings>('/api/settings/ai', {
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

  // Update patrol model
  async function handleModelChange(modelId: string) {
    if (isUpdatingSettings()) return;
    setIsUpdatingSettings(true);
    try {
      await apiFetchJSON('/api/settings/ai', {
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
      await apiFetchJSON('/api/settings/ai', {
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

  // Fetch patrol status to check license
  const [patrolStatus, { refetch: refetchPatrolStatus }] = createResource<PatrolStatus | null>(async () => {
    try {
      return await getPatrolStatus();
    } catch {
      return null;
    }
  });

  const licenseRequired = createMemo(() => patrolStatus()?.license_required ?? false);
  const upgradeUrl = createMemo(() => patrolStatus()?.upgrade_url || 'https://pulserelay.pro/');

  // Load autonomy settings
  async function loadAutonomySettings() {
    try {
      const settings = await getPatrolAutonomySettings();
      setAutonomyLevel(settings.autonomy_level);
      setInvestigationBudget(settings.investigation_budget);
      setInvestigationTimeout(settings.investigation_timeout_sec);
      setCriticalRequireApproval(settings.critical_require_approval);
    } catch (err) {
      console.error('Failed to load autonomy settings:', err);
    }
  }

  // Update autonomy level
  async function handleAutonomyChange(level: PatrolAutonomyLevel) {
    if (isUpdatingAutonomy()) return;
    setIsUpdatingAutonomy(true);
    try {
      const currentSettings = await getPatrolAutonomySettings();
      await updatePatrolAutonomySettings({
        ...currentSettings,
        autonomy_level: level,
      });
      setAutonomyLevel(level);
    } catch (err) {
      console.error('Failed to update autonomy:', err);
    } finally {
      setIsUpdatingAutonomy(false);
    }
  }

  // Save advanced settings
  async function saveAdvancedSettings() {
    setIsSavingAdvanced(true);
    try {
      await updatePatrolAutonomySettings({
        autonomy_level: autonomyLevel(),
        investigation_budget: investigationBudget(),
        investigation_timeout_sec: investigationTimeout(),
        critical_require_approval: criticalRequireApproval(),
      });
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

  async function loadAllData() {
    setIsRefreshing(true);
    try {
      await Promise.all([
        aiIntelligenceStore.loadFindings(),
        aiIntelligenceStore.loadCircuitBreakerStatus(),
        refetchPatrolStatus(),
      ]);
    } finally {
      setIsRefreshing(false);
    }
  }

  const summaryStats = () => {
    const findings = aiIntelligenceStore.findings;
    const activeFindings = findings.filter(f => f.status === 'active');

    const criticalCount = activeFindings.filter(f => f.severity === 'critical').length;
    const warningCount = activeFindings.filter(f => f.severity === 'warning').length;
    const watchCount = activeFindings.filter(f => f.severity === 'watch').length;
    const infoCount = activeFindings.filter(f => f.severity === 'info').length;
    const investigatingCount = findings.filter(f => f.investigationStatus === 'running').length;
    const totalActive = activeFindings.length;

    return {
      criticalFindings: criticalCount,
      warningFindings: warningCount,
      watchFindings: watchCount,
      infoFindings: infoCount,
      investigatingCount,
      totalActive,
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
              value={patrolModel()}
              onChange={(e) => handleModelChange(e.currentTarget.value)}
              disabled={isUpdatingSettings() || !patrolEnabledLocal()}
              class="text-xs bg-gray-100 dark:bg-gray-700 border-0 rounded-md py-1 pl-2 pr-6 text-gray-700 dark:text-gray-300 focus:ring-1 focus:ring-blue-500 disabled:opacity-50"
            >
              <option value="">Default ({defaultModel().split(':').pop() || 'not set'})</option>
              <For each={Array.from(groupModelsByProvider(availableModels()).entries())}>
                {([provider, models]) => (
                  <optgroup label={provider.charAt(0).toUpperCase() + provider.slice(1)}>
                    <For each={models}>
                      {(model) => (
                        <option value={model.id}>
                          {model.name || model.id.split(':').pop()}
                        </option>
                      )}
                    </For>
                  </optgroup>
                )}
              </For>
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
              <For each={(['monitor', 'approval', 'full'] as PatrolAutonomyLevel[])}>
                {(level) => (
                  <button
                    onClick={() => handleAutonomyChange(level)}
                    disabled={isUpdatingAutonomy() || !patrolEnabledLocal()}
                    class={`px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${
                      autonomyLevel() === level
                        ? 'bg-white dark:bg-gray-600 text-gray-900 dark:text-white shadow-sm'
                        : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                    } ${isUpdatingAutonomy() || !patrolEnabledLocal() ? 'opacity-50 cursor-not-allowed' : ''}`}
                  >
                    {level === 'monitor' ? 'Monitor' : level === 'approval' ? 'Approval' : 'Auto'}
                  </button>
                )}
              </For>
            </div>
            <div class="relative group">
              <CircleHelpIcon class="w-4 h-4 text-gray-400 dark:text-gray-500 cursor-help" />
              <div class="absolute left-0 top-6 z-50 hidden group-hover:block w-64 p-3 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 text-xs">
                <div class="space-y-2">
                  <div>
                    <span class="font-semibold text-gray-900 dark:text-white">Monitor</span>
                    <p class="text-gray-600 dark:text-gray-400">Detect issues only. No automated investigation.</p>
                  </div>
                  <div>
                    <span class="font-semibold text-gray-900 dark:text-white">Approval</span>
                    <p class="text-gray-600 dark:text-gray-400">Patrol investigates findings. Fixes require your approval.</p>
                  </div>
                  <div>
                    <span class="font-semibold text-gray-900 dark:text-white">Auto</span>
                    <p class="text-gray-600 dark:text-gray-400">Patrol investigates and applies safe fixes. Critical fixes still need approval.</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Advanced Settings Gear */}
            <Show when={autonomyLevel() !== 'monitor'}>
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
                      <h4 class="text-sm font-semibold text-gray-900 dark:text-white">Investigation Limits</h4>
                      <button
                        onClick={() => setShowAdvancedSettings(false)}
                        class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                      >
                        <XIcon class="w-4 h-4" />
                      </button>
                    </div>

                    <div class="space-y-4">
                      {/* Turn Budget */}
                      <div>
                        <div class="flex items-center justify-between mb-1">
                          <label class="text-xs text-gray-600 dark:text-gray-400">Turn Budget</label>
                          <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{investigationBudget()} turns</span>
                        </div>
                        <input
                          type="range"
                          min="5"
                          max="30"
                          step="1"
                          value={investigationBudget()}
                          onInput={(e) => setInvestigationBudget(parseInt(e.currentTarget.value, 10))}
                          disabled={isSavingAdvanced()}
                          class="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer dark:bg-gray-700"
                        />
                        <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-1">Max tool calls per investigation</p>
                      </div>

                      {/* Timeout */}
                      <div>
                        <div class="flex items-center justify-between mb-1">
                          <label class="text-xs text-gray-600 dark:text-gray-400">Timeout</label>
                          <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{Math.round(investigationTimeout() / 60)} min</span>
                        </div>
                        <input
                          type="range"
                          min="60"
                          max="1800"
                          step="60"
                          value={investigationTimeout()}
                          onInput={(e) => setInvestigationTimeout(parseInt(e.currentTarget.value, 10))}
                          disabled={isSavingAdvanced()}
                          class="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer dark:bg-gray-700"
                        />
                        <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-1">Max time per investigation (1-30 min)</p>
                      </div>

                      {/* Critical Require Approval */}
                      <div class="flex items-center justify-between pt-2 border-t border-gray-200 dark:border-gray-700">
                        <div>
                          <label class="text-xs text-gray-700 dark:text-gray-300">Critical requires approval</label>
                          <p class="text-[10px] text-gray-500 dark:text-gray-400">Always ask before fixing critical issues</p>
                        </div>
                        <Toggle
                          checked={criticalRequireApproval()}
                          onChange={(e) => setCriticalRequireApproval(e.currentTarget.checked)}
                          disabled={isSavingAdvanced()}
                        />
                      </div>

                      <Show when={!criticalRequireApproval()}>
                        <p class="text-[10px] text-amber-600 dark:text-amber-400 flex items-center gap-1">
                          <ShieldAlertIcon class="w-3 h-3" />
                          Critical fixes will execute without approval
                        </p>
                      </Show>

                      {/* Save Button */}
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
            </Show>
          </div>
        </div>
      </div>

      <Show when={licenseRequired()}>
        <div class="flex-shrink-0 bg-blue-50 dark:bg-blue-900/20 border-b border-blue-200 dark:border-blue-800 px-4 py-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 p-1.5 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
                <SparklesIcon class="w-4 h-4 text-blue-600 dark:text-blue-400" />
              </div>
              <div>
                <p class="text-sm font-semibold text-blue-900 dark:text-blue-100">
                  Unlock LLM-backed Patrol with Pulse Pro
                </p>
                <p class="text-xs text-blue-700 dark:text-blue-300">
                  Heuristic Patrol remains available. Upgrade to enable AI analysis, investigations, and auto-fix.
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
                are crossed, findings are created automatically. In <strong>Approval</strong> or <strong>Auto</strong> mode,
                Pulse Patrol investigates these findings - querying nodes, checking logs, and running diagnostics to
                identify root causes. It then suggests fixes (Approval) or applies them automatically (Auto).
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                This is experimental. Critical and destructive actions always require approval.
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

            {/* Watch + Info (lower severity) */}
            <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <div class="flex items-center gap-2">
                <div class={`p-1.5 rounded ${
                  (summaryStats().watchFindings + summaryStats().infoFindings) > 0
                    ? 'bg-blue-100 dark:bg-blue-900/30'
                    : 'bg-gray-100 dark:bg-gray-700'
                }`}>
                  <ActivityIcon class={`w-4 h-4 ${
                    (summaryStats().watchFindings + summaryStats().infoFindings) > 0
                      ? 'text-blue-600 dark:text-blue-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`} />
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">Watch / Info</p>
                  <p class={`text-lg font-bold ${
                    (summaryStats().watchFindings + summaryStats().infoFindings) > 0
                      ? 'text-blue-600 dark:text-blue-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`}>
                    {summaryStats().watchFindings + summaryStats().infoFindings}
                  </p>
                </div>
              </div>
            </div>

            {/* Fixed (issues resolved by Patrol) */}
            <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <div class="flex items-center gap-2">
                <div class={`p-1.5 rounded ${
                  (patrolStatus()?.fixed_count || 0) > 0
                    ? 'bg-green-100 dark:bg-green-900/30'
                    : 'bg-gray-100 dark:bg-gray-700'
                }`}>
                  <CheckCircleIcon class={`w-4 h-4 ${
                    (patrolStatus()?.fixed_count || 0) > 0
                      ? 'text-green-600 dark:text-green-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`} />
                </div>
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400">Fixed</p>
                  <p class={`text-lg font-bold ${
                    (patrolStatus()?.fixed_count || 0) > 0
                      ? 'text-green-600 dark:text-green-400'
                      : 'text-gray-400 dark:text-gray-500'
                  }`}>
                    {patrolStatus()?.fixed_count || 0}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <UnifiedFindingsPanel />
        </div>
      </div>
    </div>
  );
}

export default AIIntelligence;
