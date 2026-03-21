/**
 * Patrol Intelligence Surface
 *
 * Canonical Patrol runtime surface for findings, runs, and patrol controls.
 */

import { createMemo, For, Show } from 'solid-js';
import { FindingsPanel } from '@/components/AI/FindingsPanel';
import { getFindingSeverityToneClasses } from '@/utils/aiFindingPresentation';
import {
  getPatrolSummaryPresentation,
  PATROL_NO_ISSUES_LABEL,
} from '@/utils/patrolSummaryPresentation';

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
import { PageHeader } from '@/components/shared/PageHeader';
import { TogglePrimitive, Toggle } from '@/components/shared/Toggle';
import {
  ApprovalBanner,
  PatrolStatusBar,
  RunHistoryPanel,
  CountdownTimer,
} from '@/components/patrol';
import { formatRelativeTime } from '@/utils/format';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';
import { formatTriggerReason, groupModelsByProvider } from '@/utils/patrolFormat';
import { getAIQuickstartCreditsPresentation } from '@/utils/aiQuickstartPresentation';
import { buildPatrolScheduleOptions } from '@/utils/aiPatrolSchedulePresentation';
import { ResourcePolicySummary } from '@/components/Infrastructure/ResourcePolicySummary';
import { ResourceCorrelationSummary } from '@/components/Infrastructure/ResourceCorrelationSummary';
import { ResourceChangeSummary } from '@/components/Infrastructure/ResourceChangeSummary';
import { usePatrolIntelligenceState } from './usePatrolIntelligenceState';

export function PatrolIntelligenceSurface() {
  const {
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
    setFindingScrollTimer,
    setFindingsFilterOverride,
    setFullModeUnlocked,
    setPatrolModelSelectRef,
    setScrollToFindingTimer,
    setSelectedRun,
    setShowAdvancedSettings,
    setShowInvestigationContext,
    showAdvancedSettings,
    showBlockedBanner,
    showInvestigationContext,
    startingTrial,
    summaryStats,
    triggerPatrolDisabledReason,
    upgradeUrl,
  } = usePatrolIntelligenceState();

  const quickstartPresentation = createMemo(() =>
    getAIQuickstartCreditsPresentation(
      patrolStatus()?.quickstart_credits_remaining ?? 0,
      patrolStatus()?.quickstart_credits_total ?? 0,
    ),
  );
  const criticalSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('critical', summaryStats().criticalFindings > 0),
  );
  const warningSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('warning', summaryStats().warningFindings > 0),
  );
  const fixedSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('success', summaryStats().fixedCount > 0),
  );
  const scheduleOptions = createMemo(() => buildPatrolScheduleOptions(patrolInterval()));
  const patrolModelStale = createMemo(() => {
    const model = patrolModel();
    const models = availableModels();
    if (!model || models.length === 0) return false;
    return !models.some((candidate) => candidate.id === model);
  });

  return (
    <div class="h-full flex flex-col bg-base">
      {/* Header */}
      <div class="flex-shrink-0 bg-surface border-b border-border px-4 py-3">
        <PageHeader
          id="patrol-title"
          title={
            <span
              class="inline-flex items-center gap-3"
              title="Pulse Patrol constantly monitors your infrastructure, investigates alerts, and can automatically fix issues based on your autonomy settings."
            >
              <PulsePatrolLogo class="w-6 h-6 text-base-content" />
              <span>Patrol</span>
            </span>
          }
          description="Pulse Patrol monitoring and analysis"
          class="mb-3"
          actions={
            <div class="flex flex-wrap items-center justify-end gap-3">
              <Show when={patrolStatus()?.last_patrol_at}>
                <div class="hidden sm:flex items-center gap-3 text-xs text-muted">
                  <span>
                    Last:{' '}
                    {formatRelativeTime(patrolStatus()?.last_patrol_at, {
                      compact: true,
                      emptyText: 'Never',
                    })}
                  </span>
                  <Show when={patrolStatus()?.next_patrol_at}>
                    <span class="text-muted">|</span>
                    <CountdownTimer
                      targetDate={patrolStatus()!.next_patrol_at!}
                      prefix="Next run: "
                      class="font-variant-numeric tabular-nums font-medium text-blue-600 dark:text-blue-400"
                    />
                  </Show>
                </div>
              </Show>

              <button
                onClick={() => handleRunPatrol()}
                disabled={
                  isTriggeringPatrol() ||
                  !canTriggerPatrol() ||
                  manualRunRequested() ||
                  patrolStream.isStreaming()
                }
                title={triggerPatrolDisabledReason()}
                class="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-surface-alt disabled:text-muted rounded-md transition-colors"
              >
                <PlayIcon
                  class={`w-4 h-4 ${isTriggeringPatrol() || manualRunRequested() || patrolStream.isStreaming() ? 'animate-pulse' : ''}`}
                />
                {isTriggeringPatrol()
                  ? 'Starting…'
                  : manualRunRequested() || patrolStream.isStreaming()
                    ? 'Running…'
                    : 'Run Patrol'}
              </button>

              <button
                onClick={() => loadAllData()}
                disabled={isRefreshing()}
                class="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50 transition-colors"
              >
                <RefreshCwIcon class={`w-4 h-4 ${isRefreshing() ? 'animate-spin' : ''}`} />
                Refresh
              </button>
            </div>
          }
        />

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
            <span class="text-sm font-medium text-base-content">
              {patrolEnabledLocal() ? 'Patrol Active' : 'Patrol Disabled'}
            </span>
          </div>

          {/* Quickstart Credits Badge */}
          <Show
            when={
              patrolStatus()?.using_quickstart ||
              (patrolStatus()?.quickstart_credits_total &&
                patrolStatus()!.quickstart_credits_total! > 0 &&
                patrolStatus()!.quickstart_credits_remaining !== undefined)
            }
          >
            <div
              class={`flex items-center gap-1.5 px-3 py-1.5 rounded-md border text-xs font-medium ${quickstartPresentation().className}`}
              title={quickstartPresentation().title}
            >
              <Show
                when={(patrolStatus()?.quickstart_credits_remaining ?? 0) > 0}
                fallback={<span>{quickstartPresentation().summary}</span>}
              >
                <span>{quickstartPresentation().summary}</span>
              </Show>
            </div>
          </Show>

          <div class="flex-1"></div>

          {/* Configuration Popover */}
          <div class="relative" ref={setAdvancedSettingsRef}>
            <button
              onClick={() => setShowAdvancedSettings(!showAdvancedSettings())}
              disabled={!patrolEnabledLocal()}
              class={`flex items-center gap-2 px-3 py-1.5 text-sm font-medium rounded-md transition-all shadow-sm ${showAdvancedSettings() ? 'bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-800' : ' text-base-content border border-border hover:bg-surface-alt'} ${!patrolEnabledLocal() ? 'opacity-50 cursor-not-allowed hidden' : ''}`}
            >
              <SettingsIcon class="w-4 h-4" />
              Configure Patrol
            </button>

            <Show when={showAdvancedSettings()}>
              <div class="absolute right-0 top-10 z-50 w-[340px] p-5 bg-surface rounded-md shadow-sm border border-border animate-slide-up transform origin-top-right">
                <div class="flex items-center justify-between mb-5 pb-3 border-b border-border-subtle">
                  <h4 class="text-base font-semibold tracking-tight text-base-content">
                    Patrol Configuration
                  </h4>
                  <button
                    onClick={() => setShowAdvancedSettings(false)}
                    class="p-1 rounded-md hover:text-base-content hover:bg-surface-hover transition-colors"
                  >
                    <XIcon class="w-4 h-4" />
                  </button>
                </div>

                <div class="space-y-6">
                  {/* Model & Schedule grouped */}
                  <div class="grid grid-cols-2 gap-4">
                    <div class="space-y-1.5">
                      <label class="text-xs font-semibold uppercase tracking-wider text-muted">
                        AI Model
                      </label>
                      <select
                        ref={setPatrolModelSelectRef}
                        value={patrolModel()}
                        onChange={(e) => handleModelChange(e.currentTarget.value)}
                        disabled={isUpdatingSettings() || !patrolEnabledLocal()}
                        class="w-full text-sm bg-base border border-border rounded-md py-2 pl-3 pr-8 text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50"
                      >
                        <option value="">
                          Default ({defaultModel().split(':').pop() || 'not set'})
                        </option>
                        <Show when={patrolModelStale()}>
                          <option value={patrolModel()} disabled>
                            {patrolModel().split(':').pop()} (unavailable)
                          </option>
                        </Show>
                        {Array.from(groupModelsByProvider(availableModels()).entries()).map(
                          ([provider, models]) => (
                            <optgroup label={provider.charAt(0).toUpperCase() + provider.slice(1)}>
                              {models.map((model) => (
                                <option value={model.id}>
                                  {model.name || model.id.split(':').pop()}
                                </option>
                              ))}
                            </optgroup>
                          ),
                        )}
                      </select>
                    </div>

                    <div class="space-y-1.5">
                      <label class="text-xs font-semibold uppercase tracking-wider text-muted">
                        Run Every
                      </label>
                      <select
                        value={patrolInterval()}
                        onChange={(e) => handleIntervalChange(parseInt(e.currentTarget.value))}
                        disabled={isUpdatingSettings() || !patrolEnabledLocal()}
                        class="w-full text-sm bg-base border border-border rounded-md py-2 pl-3 pr-8 text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50"
                      >
                        <For each={scheduleOptions()}>
                          {(preset) => <option value={preset.value}>{preset.label}</option>}
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
                          <CircleHelpIcon class="w-3.5 h-3.5 cursor-help" />
                          <div class="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 hidden group-hover:block w-64 p-3 bg-surface text-white rounded-md shadow-md text-xs z-50 pointer-events-none before:absolute before:top-full before:left-1/2 before:-translate-x-1/2 before:border-4 before:border-transparent before:border-t-slate-800">
                            <strong>Monitor:</strong> Detect only.
                            <br />
                            <strong>Investigate:</strong> Detect & propose fixes.
                            <br />
                            <strong>Auto-fix:</strong> Execute safe fixes automatically.
                          </div>
                        </div>
                      </label>
                    </div>

                    <div class="flex items-center bg-base rounded-md p-1 border shadow-inner">
                      <For each={['monitor', 'approval', 'assisted'] as const}>
                        {(level) => {
                          const isProLocked = () =>
                            autoFixLocked() && (level === 'approval' || level === 'assisted');
                          const isDisabled = () => !patrolEnabledLocal() || isProLocked();
                          const isActive = () =>
                            level === 'assisted'
                              ? autonomyLevel() === 'assisted' || autonomyLevel() === 'full'
                              : autonomyLevel() === level;

                          return (
                            <button
                              onClick={() => handleAutonomyChange(level)}
                              disabled={isDisabled()}
                              title={
                                isProLocked()
                                  ? level === 'approval'
                                    ? 'Upgrade to Pro to investigate findings'
                                    : 'Upgrade to Pro for automatic fixes'
                                  : undefined
                              }
                              class={`flex-1 py-1.5 px-2 text-xs font-semibold rounded-md transition-all duration-200 ${isActive() ? ' text-blue-600 dark:text-blue-400 shadow-[0_1px_3px_rgba(0,0,0,0.1)]' : isDisabled() ? ' ' : 'text-muted hover:text-base-content hover:bg-surface-hover'} ${isDisabled() ? 'opacity-50 cursor-not-allowed' : ''}`}
                            >
                              {level === 'monitor'
                                ? 'Monitor'
                                : level === 'approval'
                                  ? 'Investigate'
                                  : 'Auto-fix'}
                            </button>
                          );
                        }}
                      </For>
                    </div>
                    <Show when={autoFixLocked()}>
                      <div class="pl-1 text-[11px] text-slate-500">
                        <a
                          href={upgradeUrl()}
                          target="_blank"
                          class="text-indigo-500 font-medium hover:underline"
                        >
                          Upgrade to Pro
                        </a>{' '}
                        to unlock investigation and auto-fix.
                        <Show when={canStartTrial()}>
                          {' '}
                          <button
                            type="button"
                            onClick={handleStartTrial}
                            disabled={startingTrial()}
                            class="text-indigo-500 hover:underline"
                          >
                            Start free trial
                          </button>
                        </Show>
                      </div>
                    </Show>
                  </div>

                  {/* Toggles */}
                  <div class="space-y-4 pt-4 border-t border-border-subtle">
                    <div class="flex items-start justify-between gap-3">
                      <div class="flex-1">
                        <label class="text-sm font-medium text-base-content">
                          Alert-Triggered Analysis
                        </label>
                        <p class="text-[11px] text-muted mt-0.5 leading-tight">
                          Analyze infrastructure automatically when critical alerts fire.
                        </p>
                      </div>
                      <Toggle
                        checked={alertTriggeredAnalysis()}
                        onChange={(e) =>
                          handleAlertTriggeredAnalysisChange(e.currentTarget.checked)
                        }
                        disabled={isUpdatingSettings() || alertAnalysisLocked()}
                      />
                    </div>

                    <Show when={alertAnalysisLocked()}>
                      <div class="-my-1 pl-1 text-[11px]">
                        <a
                          href={alertAnalysisUpgradeUrl()}
                          target="_blank"
                          class="text-indigo-500 font-medium hover:underline"
                        >
                          Upgrade
                        </a>{' '}
                        to enable.
                        <Show when={canStartTrial()}>
                          <button
                            type="button"
                            onClick={handleStartTrial}
                            disabled={startingTrial()}
                            class="ml-1 text-indigo-500 hover:underline"
                          >
                            Start free trial
                          </button>
                        </Show>
                      </div>
                    </Show>

                    <div class="flex items-start justify-between gap-3">
                      <div class="flex-1">
                        <label class="text-sm font-medium text-base-content">
                          Event-Triggered Patrols
                        </label>
                        <p class="text-[11px] text-muted mt-0.5 leading-tight">
                          Run extra patrols when alerts fire or anomalies are detected.
                        </p>
                      </div>
                      <Toggle
                        checked={patrolEventTriggers()}
                        onChange={(e) => handlePatrolEventTriggersChange(e.currentTarget.checked)}
                        disabled={isUpdatingSettings() || !patrolEnabledLocal()}
                      />
                    </div>

                    <div class="flex items-start justify-between gap-3">
                      <div class="flex-1">
                        <label class="text-sm font-medium text-red-600 dark:text-red-400">
                          Auto-fix critical issues
                        </label>
                        <p class="text-[11px] text-muted mt-0.5 leading-tight">
                          Permit Patrol to execute critical remediations without approval.
                        </p>
                      </div>
                      <Toggle
                        checked={!autoFixLocked() && fullModeUnlocked()}
                        onChange={(e) => setFullModeUnlocked(e.currentTarget.checked)}
                        disabled={
                          autoFixLocked() ||
                          !(autonomyLevel() === 'assisted' || autonomyLevel() === 'full')
                        }
                      />
                    </div>
                  </div>

                  {/* Save Footer */}
                  <div class="pt-4 border-t border-border-subtle">
                    <button
                      onClick={saveAdvancedSettings}
                      disabled={isSavingAdvanced()}
                      class="w-full py-2.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md shadow-sm transition-all focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-70 flex items-center justify-center gap-2"
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
              <a
                class="text-indigo-600 dark:text-indigo-400 font-semibold hover:underline"
                href={upgradeUrl()}
                target="_blank"
                rel="noopener noreferrer"
                onClick={() => trackUpgradeClicked('ai_intelligence_banner', 'ai_autofix')}
              >
                Upgrade to Pro
              </a>{' '}
              to unlock automatic fixes and alert-triggered analysis.
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
                <p class="text-xs text-amber-700 dark:text-amber-300">{blockedReason()}</p>
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
      <div
        class={`flex-1 overflow-auto p-4 transition-opacity ${!patrolEnabledLocal() ? 'opacity-50 pointer-events-none' : ''}`}
      >
        <div class="space-y-4">
          {/* Approval Banner */}
          <ApprovalBanner
            onScrollToFinding={(findingId) => {
              setActiveTab('findings');
              setFindingsFilterOverride('approvals');
              clearScrollToFindingTimer();
              setScrollToFindingTimer(
                setTimeout(() => {
                  setScrollToFindingTimer(undefined);
                  const el = document.getElementById(`finding-${findingId}`);
                  el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
                  setFindingScrollTimer(undefined);
                }, 100),
              );
            }}
          />

          {/* Status Bar (replaces Activity tab) */}
          <PatrolStatusBar
            enabled={patrolEnabledLocal()}
            refreshTrigger={activityRefreshTrigger()}
          />

          <Show when={intelligenceSummary()}>
            {(summary) => (
              <section class="rounded-md border border-border bg-surface p-4">
                <div class="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                      Patrol summary
                    </p>
                    <h2 class="mt-1 text-lg font-semibold text-base-content">
                      Health {summary().overall_health.grade} ·{' '}
                      {Math.round(summary().overall_health.score)}/100
                    </h2>
                    <p class="mt-1 text-sm text-muted">{summary().overall_health.prediction}</p>
                  </div>

                  <div class="flex flex-wrap items-center gap-2">
                    <span class="rounded-full border border-border-subtle bg-base px-2.5 py-1 text-xs font-medium text-base-content">
                      Critical {summary().findings_count.critical}
                    </span>
                    <span class="rounded-full border border-border-subtle bg-base px-2.5 py-1 text-xs font-medium text-base-content">
                      Warning {summary().findings_count.warning}
                    </span>
                  </div>
                </div>

                <Show when={hasInvestigationContext()}>
                  <div class="mt-4 rounded-md border border-border-subtle bg-base p-3">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                          Investigation context
                        </p>
                        <p class="mt-1 text-sm text-muted">
                          Secondary change and policy signals for deeper investigation.
                        </p>
                        <Show when={investigationContextSummary()}>
                          <p class="mt-1 text-xs text-base-content">
                            {investigationContextSummary()}
                          </p>
                        </Show>
                      </div>

                      <button
                        type="button"
                        onClick={() => setShowInvestigationContext((value) => !value)}
                        class="inline-flex items-center rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
                      >
                        {showInvestigationContext() ? 'Hide context' : 'Show context'}
                      </button>
                    </div>

                    <Show when={showInvestigationContext()}>
                      <div class="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
                        <Show when={recentChangeCount() > 0}>
                          <ResourceChangeSummary
                            class="space-y-0"
                            title="Recent changes"
                            subtitle="Last 24 hours"
                            changes={summary().recent_changes}
                            maxChanges={3}
                            compact
                          />
                        </Show>

                        <div class="space-y-4">
                          <Show when={correlations().length > 0}>
                            <ResourceCorrelationSummary
                              title="Correlations"
                              correlations={correlations()}
                              summaryText={`${correlationTotal()} total`}
                            />
                          </Show>

                          <ResourcePolicySummary
                            posture={policyPosture()}
                            title="Policy posture"
                          />
                        </div>
                      </div>
                    </Show>
                  </div>
                </Show>
              </section>
            )}
          </Show>

          {/* Summary Cards */}
          <Show
            when={
              summaryStats().criticalFindings > 0 ||
              summaryStats().warningFindings > 0 ||
              summaryStats().fixedCount > 0
            }
            fallback={
              <Show when={patrolStatus()?.last_patrol_at}>
                <div class="flex items-center gap-2 px-4 py-3 bg-surface rounded-md border border-border">
                  <CheckCircleIcon class="w-4 h-4 text-green-500 dark:text-green-400" />
                  <span class="text-sm text-muted">{PATROL_NO_ISSUES_LABEL}</span>
                </div>
              </Show>
            }
          >
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              {/* Critical */}
              <div class="bg-surface rounded-md border border-border p-3">
                <div class="flex items-center gap-2">
                  <div
                    class={`p-1.5 rounded-md border ${criticalSummaryPresentation().iconContainerClass}`}
                  >
                    <ShieldAlertIcon class={`w-4 h-4 ${criticalSummaryPresentation().iconClass}`} />
                  </div>
                  <div>
                    <p class="text-xs text-muted">Critical</p>
                    <p class={`text-lg font-bold ${criticalSummaryPresentation().valueClass}`}>
                      {summaryStats().criticalFindings}
                    </p>
                  </div>
                </div>
              </div>

              {/* Warnings */}
              <div class="bg-surface rounded-md border border-border p-3">
                <div class="flex items-center gap-2">
                  <div
                    class={`p-1.5 rounded-md border ${warningSummaryPresentation().iconContainerClass}`}
                  >
                    <ActivityIcon class={`w-4 h-4 ${warningSummaryPresentation().iconClass}`} />
                  </div>
                  <div>
                    <p class="text-xs text-muted">Warnings</p>
                    <p class={`text-lg font-bold ${warningSummaryPresentation().valueClass}`}>
                      {summaryStats().warningFindings}
                    </p>
                  </div>
                </div>
              </div>

              {/* Fixed (issues resolved by Patrol) */}
              <div class="bg-surface rounded-md border border-border p-3">
                <div class="flex items-center gap-2">
                  <div
                    class={`p-1.5 rounded-md border ${fixedSummaryPresentation().iconContainerClass}`}
                  >
                    <CheckCircleIcon class={`w-4 h-4 ${fixedSummaryPresentation().iconClass}`} />
                  </div>
                  <div>
                    <p class="text-xs text-muted">Fixed</p>
                    <p class={`text-lg font-bold ${fixedSummaryPresentation().valueClass}`}>
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
              class={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab() === 'findings'
                  ? 'border-blue-500 text-base-content'
                  : 'border-transparent text-muted hover:text-base-content hover:border-border'
              }`}
            >
              Findings
              <Show when={summaryStats().totalActive > 0}>
                <span
                  class={`ml-1.5 px-1.5 py-0.5 text-xs rounded-full ${getFindingSeverityToneClasses(summaryStats().criticalFindings > 0 ? 'critical' : 'warning')}`}
                >
                  {summaryStats().totalActive}
                </span>
              </Show>
            </button>
            <button
              type="button"
              onClick={() => {
                setActiveTab('history');
                setFindingsFilterOverride(undefined);
              }}
              class={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab() === 'history'
                  ? 'border-blue-500 text-base-content'
                  : 'border-transparent text-muted hover:text-base-content hover:border-border'
              }`}
            >
              Runs
              <Show when={displayRunHistory().length > 0}>
                <span class="ml-1.5 px-1.5 py-0.5 text-xs rounded-full bg-surface-alt text-muted">
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
                    Filtered to run {formatRelativeTime(run().started_at, { compact: true })} (
                    {formatTriggerReason(run().trigger_reason)})
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
              filterOverride={selectedRun() ? 'all' : findingsFilterOverride()}
              filterFindingIds={selectedRunFindingIds()}
              scopeResourceIds={selectedRunScopeResourceIds()}
              scopeResourceTypes={selectedRun()?.scope_resource_types}
              showScopeWarnings={Boolean(selectedRun())}
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

export default PatrolIntelligenceSurface;
