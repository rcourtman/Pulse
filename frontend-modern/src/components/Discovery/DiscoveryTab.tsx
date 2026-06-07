import { Component, For, Show, createMemo } from 'solid-js';
import CheckIcon from 'lucide-solid/icons/check';
import CopyIcon from 'lucide-solid/icons/copy';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import type { ResourceType } from '../../types/discovery';
import {
  formatDiscoveryAge,
  getCategoryDisplayName,
  getConfidenceLevel,
} from '../../api/discovery';
import {
  getDiscoveryInitialEmptyState,
  getDiscoveryLoadingState,
  getDiscoveryNotesEmptyState,
  getDiscoveryApiAccessSettingsTarget,
  getDiscoveryAnalysisProviderBadgeClass,
  getDiscoveryCategoryBadgeClass,
  getDiscoveryCommandSettingsTarget,
  getDiscoveryObservedSourceLabel,
  getDiscoverySuggestedURLReason,
  getDiscoverySuggestedURLFallback,
  getDiscoverySuggestedURLActionClass,
  getDiscoverySuggestedURLCardClass,
  getDiscoverySuggestedURLCodeClass,
  getDiscoverySuggestedURLHeadingClass,
  getDiscoverySuggestedURLTextClass,
} from '@/utils/discoveryPresentation';
import { DiscoveryProvenanceMarker } from '@/components/shared/DiscoveryProvenanceMarker';
import {
  DISCOVERY_ANALYSIS_EXPLANATION,
  DISCOVERY_ANALYSIS_REASONING_LABEL,
} from '@/utils/resourceAnalysisPresentation';
import { useDiscoveryTabState } from './useDiscoveryTabState';

interface DiscoveryTabProps {
  resourceType: ResourceType;
  agentId?: string;
  resourceId: string;
  hostname: string;
  /** Whether commands are enabled for this agent (from agent config) */
  commandsEnabled?: boolean;
  /** Show the primary run action at the top of embedded drawer contexts. */
  showManualRunAction?: boolean;
}

interface CopyValueButtonProps {
  value?: string | null;
  copiedValue: () => string;
  onCopy: (value?: string | null) => void | Promise<void>;
  label: string;
  class?: string;
}

const CopyValueButton: Component<CopyValueButtonProps> = (props) => {
  const trimmedValue = () => (props.value || '').trim();
  const copied = () => Boolean(trimmedValue()) && props.copiedValue() === trimmedValue();

  return (
    <button
      type="button"
      class={
        props.class ||
        'inline-flex min-h-7 min-w-7 shrink-0 items-center justify-center rounded border border-border bg-surface px-2 text-muted transition-colors hover:bg-surface-hover hover:text-base-content'
      }
      onClick={() => void props.onCopy(trimmedValue())}
      disabled={!trimmedValue()}
      title={props.label}
      aria-label={props.label}
    >
      <Show when={copied()} fallback={<CopyIcon class="h-3.5 w-3.5" />}>
        <CheckIcon class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
      </Show>
    </button>
  );
};

interface CopyableCodeRowProps extends CopyValueButtonProps {
  value: string;
}

const CopyableCodeRow: Component<CopyableCodeRowProps> = (props) => (
  <div class="flex items-start gap-2 rounded bg-surface-alt px-2 py-1.5">
    <code class="min-w-0 flex-1 break-all font-mono text-xs text-base-content">{props.value}</code>
    <CopyValueButton
      value={props.value}
      copiedValue={props.copiedValue}
      onCopy={props.onCopy}
      label={props.label}
      class="inline-flex min-h-6 min-w-6 shrink-0 items-center justify-center rounded text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
    />
  </div>
);

export const DiscoveryTab: Component<DiscoveryTabProps> = (props) => {
  const {
    canTriggerDiscovery,
    connectedAgents,
    copiedDiscoveryValue,
    discovery,
    discoveryFeatureKnownDisabled,
    discoveryReadiness,
    discoveryInfo,
    editingNotes,
    handleCopyDiscoveryValue,
    handleSaveNotes,
    handleTriggerDiscovery,
    hasConnectedAgent,
    hasValidDiscovery,
    isScanning,
    liveElapsedSeconds,
    notesText,
    saveError,
    scanError,
    scanProgress,
    scanSuccess,
    setEditingNotes,
    setNotesText,
    setScanError,
    setShowCommandsPreview,
    setShowExplanation,
    showCommandsPreview,
    showExplanation,
    showLoadingSpinner,
    startEditingNotes,
    validDiscovery,
  } = useDiscoveryTabState(props);

  const suggestedURLReasonText = createMemo(() => {
    const current = discovery();
    if (!current) return '';
    return getDiscoverySuggestedURLReason(current).text;
  });

  const suggestedURLReasonTitle = createMemo(() => {
    const current = discovery();
    if (!current) return '';
    return getDiscoverySuggestedURLReason(current).title;
  });

  const confidenceInfo = createMemo(() => {
    const current = discovery();
    if (
      discovery.loading ||
      !current ||
      current.confidence === undefined ||
      current.confidence === null
    ) {
      return null;
    }
    return getConfidenceLevel(current.confidence);
  });
  const commandSettingsTarget = getDiscoveryCommandSettingsTarget();
  const apiAccessSettingsTarget = getDiscoveryApiAccessSettingsTarget();
  const showManualRunAction = () => props.showManualRunAction === true;
  const manualRunSummary = createMemo(() => {
    if (discovery.loading) return 'Checking saved discovery state';
    const updatedAt = discovery()?.updated_at;
    if (updatedAt) return `Last run: ${formatDiscoveryAge(updatedAt)}`;
    return 'No saved discovery run for this resource';
  });

  // `whitespace-normal` on the wrapper neutralises the `.table-fixed td/th`
  // global rule (white-space: nowrap) that bleeds into expanded-row content
  // via CSS inheritance, so explanatory copy and command descriptions wrap.
  return (
    <div class="space-y-4 whitespace-normal">
      <Show when={discoveryFeatureKnownDisabled()}>
        <div class="rounded border border-amber-200 bg-amber-50/80 p-3 shadow-sm dark:border-amber-800/50 dark:bg-amber-900/20">
          <div class="flex items-start gap-2.5">
            <TriangleAlertIcon class="mt-0.5 h-4 w-4 flex-shrink-0 text-amber-600 dark:text-amber-400" />
            <div class="text-xs text-amber-800 dark:text-amber-200">
              <p class="mb-1 font-medium">AI Discovery Disabled</p>
              <p class="text-amber-700 dark:text-amber-300">
                Enable infrastructure discovery in Settings -&gt; AI before using this tab.
              </p>
            </div>
          </div>
        </div>
      </Show>

      <Show when={!discoveryFeatureKnownDisabled()}>
        {/* Analysis provider badge - shown when a provider is configured */}
        <Show when={!discoveryInfo.loading && discoveryInfo()?.ai_provider}>
          <div class="flex items-center gap-2">
            <Show
              when={discoveryInfo()?.ai_provider?.is_local}
              fallback={
                <span class={getDiscoveryAnalysisProviderBadgeClass(false)}>
                  {/* Cloud icon */}
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"
                    />
                  </svg>
                  Analysis: {discoveryInfo()?.ai_provider?.label}
                </span>
              }
            >
              <span class={getDiscoveryAnalysisProviderBadgeClass(true)}>
                {/* Server/local icon */}
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
                  />
                </svg>
                Analysis: {discoveryInfo()?.ai_provider?.label}
              </span>
            </Show>
          </div>
        </Show>

        {/* Provider prerequisite — tab-wide for every resource type, styled to
            match the disabled banner above (both are tab-level prerequisites).
            Surfaces the "enabled but no provider" dead end that previously only
            showed for agents, so Discovery is never silently on-but-useless. */}
        <Show when={!discoveryInfo.loading && discoveryReadiness().status === 'needs_ai_provider'}>
          <div class="rounded border border-amber-200 bg-amber-50/80 p-3 shadow-sm dark:border-amber-800/50 dark:bg-amber-900/20">
            <div class="flex items-start gap-2.5">
              <TriangleAlertIcon class="mt-0.5 h-4 w-4 flex-shrink-0 text-amber-600 dark:text-amber-400" />
              <div class="text-xs text-amber-800 dark:text-amber-200">
                <p class="mb-1 font-medium">AI provider not configured</p>
                <p class="text-amber-700 dark:text-amber-300">
                  Discovery needs an AI provider to analyze what is running. Configure one in
                  Settings -&gt; AI before scanning.
                </p>
              </div>
            </div>
          </div>
        </Show>

        <Show when={showManualRunAction()}>
          <div class="rounded border border-border bg-surface p-3 shadow-sm">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div class="text-xs font-medium text-base-content">Discovery run</div>
                <p class="mt-0.5 text-xs text-muted">{manualRunSummary()}</p>
              </div>
              <button
                type="button"
                onClick={() => handleTriggerDiscovery(true)}
                disabled={isScanning() || !canTriggerDiscovery()}
                class="inline-flex min-h-9 items-center justify-center gap-1.5 rounded border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-50"
              >
                <Show
                  when={isScanning()}
                  fallback={
                    <>
                      <svg
                        class="h-3.5 w-3.5"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                        />
                      </svg>
                      Run Discovery
                    </>
                  }
                >
                  <span class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-slate-500 border-t-transparent" />
                  Scanning...
                </Show>
              </button>
            </div>
          </div>
        </Show>

        {/* "What Discovery Does" explanation - shown when no discovery exists yet */}
        <Show when={!discovery() && !isScanning() && !showLoadingSpinner() && showExplanation()}>
          <div class="rounded border border-amber-200 bg-amber-50 p-3 shadow-sm dark:border-amber-800 dark:bg-amber-900">
            <div class="flex items-start justify-between gap-3">
              <div class="flex items-start gap-2.5">
                <svg
                  class="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <div class="text-xs text-amber-800 dark:text-amber-200">
                  <p class="font-medium mb-1">What Discovery Does</p>
                  <p class="text-amber-700 dark:text-amber-300">{DISCOVERY_ANALYSIS_EXPLANATION}</p>
                </div>
              </div>
              <button
                onClick={() => setShowExplanation(false)}
                class="text-amber-500 hover:text-amber-700 dark:text-amber-400 dark:hover:text-amber-300 flex-shrink-0"
                title="Dismiss"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          </div>
        </Show>

        {/* Commands Preview - Expandable before first scan */}
        <Show
          when={
            !discovery() &&
            !isScanning() &&
            !discovery.loading &&
            !discoveryInfo.loading &&
            discoveryInfo()?.commands &&
            discoveryInfo()!.commands!.length > 0
          }
        >
          <details
            class="rounded border border-border bg-surface shadow-sm"
            open={showCommandsPreview()}
          >
            <summary
              class="p-2.5 text-xs font-medium text-base-content cursor-pointer hover:bg-surface-hover flex items-center gap-2"
              onClick={(e) => {
                e.preventDefault();
                setShowCommandsPreview(!showCommandsPreview());
              }}
            >
              <svg
                class={`w-3.5 h-3.5 transition-transform ${showCommandsPreview() ? 'rotate-90' : ''}`}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 5l7 7-7 7"
                />
              </svg>
              Commands that will run ({discoveryInfo()?.commands?.length || 0})
            </summary>
            <Show when={showCommandsPreview()}>
              <div class="px-3 pb-3 space-y-2">
                <For each={discoveryInfo()?.commands}>
                  {(cmd) => (
                    <div class="text-xs">
                      <div class="flex items-start gap-2">
                        <code class="text-[10px] px-1.5 py-0.5 rounded bg-surface-hover text-base-content font-mono break-all">
                          {cmd.command}
                        </code>
                      </div>
                      <p class="text-muted mt-0.5 pl-0.5">{cmd.description}</p>
                    </div>
                  )}
                </For>
              </div>
            </Show>
          </details>
        </Show>

        {/* Loading state - delayed to prevent flash for fast loads */}
        <Show when={showLoadingSpinner()}>
          <div class="flex items-center justify-center py-8">
            <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full"></div>
            <span class="ml-2 text-sm text-muted">{getDiscoveryLoadingState().text}</span>
          </div>
        </Show>

        {/* Scan Progress Bar */}
        <Show when={scanProgress() && isScanning()}>
          <div class="rounded border border-blue-200 bg-blue-50 p-3 shadow-sm dark:border-blue-800 dark:bg-blue-900">
            <div class="flex items-center justify-between mb-2">
              <div class="flex items-center gap-2">
                <div class="animate-spin h-4 w-4 border-2 border-blue-500 border-t-transparent rounded-full"></div>
                <span class="text-sm font-medium text-blue-700 dark:text-blue-300">
                  {scanProgress()?.current_step || 'Scanning...'}
                </span>
              </div>
              <span class="text-xs text-blue-600 dark:text-blue-400">
                {Math.round(scanProgress()?.percent_complete || 0)}%
              </span>
            </div>
            <div class="w-full bg-blue-200 dark:bg-blue-800 rounded-full h-2 overflow-hidden">
              <div
                class="bg-blue-500 h-2 rounded-full transition-all duration-300"
                style={{ width: `${scanProgress()?.percent_complete || 0}%` }}
              ></div>
            </div>
            <Show when={scanProgress()?.current_command}>
              <div class="mt-2 text-xs text-blue-600 dark:text-blue-400">
                Running: <code class="font-mono">{scanProgress()?.current_command}</code>
              </div>
            </Show>
            {/* Live elapsed time and hint */}
            <div class="mt-2 flex items-center justify-between text-xs text-blue-500 dark:text-blue-400">
              <span>Elapsed: {liveElapsedSeconds()}s</span>
              <span class="text-blue-400 dark:text-blue-500">
                Analysis time varies by model. You can navigate away — results save automatically.
              </span>
            </div>
          </div>
        </Show>

        {/* Scanning state without WebSocket progress - show live timer */}
        <Show when={isScanning() && !scanProgress()}>
          <div class="rounded border border-blue-200 bg-blue-50 p-3 shadow-sm dark:border-blue-800 dark:bg-blue-900">
            <div class="flex items-center gap-2 mb-2">
              <div class="animate-spin h-4 w-4 border-2 border-blue-500 border-t-transparent rounded-full"></div>
              <span class="text-sm font-medium text-blue-700 dark:text-blue-300">
                Running discovery...
              </span>
            </div>
            <div class="flex items-center justify-between text-xs text-blue-500 dark:text-blue-400">
              <span>Elapsed: {liveElapsedSeconds()}s</span>
              <span class="text-blue-400 dark:text-blue-500">
                Analysis time varies by model. You can navigate away — results save automatically.
              </span>
            </div>
          </div>
        </Show>

        {/* Scan Success */}
        <Show when={scanSuccess()}>
          <div
            role="status"
            aria-live="polite"
            class="mb-4 rounded-md border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900"
          >
            <div class="flex items-center gap-2">
              <svg
                class="w-5 h-5 text-green-500 dark:text-green-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M5 13l4 4L19 7"
                />
              </svg>
              <p class="text-sm font-medium text-green-800 dark:text-green-200">
                Discovery complete!
              </p>
            </div>
          </div>
        </Show>

        {/* Scan Error */}
        <Show when={scanError()}>
          <div class="mb-4 rounded-md border border-red-200 bg-red-50 p-4 dark:border-red-800 dark:bg-red-900">
            <div class="flex items-start gap-3">
              <svg
                class="w-5 h-5 text-red-500 dark:text-red-400 flex-shrink-0 mt-0.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <div>
                <p class="text-sm font-medium text-red-800 dark:text-red-200">Discovery Failed</p>
                <p class="text-sm text-red-700 dark:text-red-300 mt-1">{scanError()}</p>
              </div>
              <button
                onClick={() => setScanError(null)}
                class="ml-auto text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          </div>
        </Show>

        {/* No discovery yet - only show after initial fetch completes to prevent flash */}
        <Show when={!discovery() && !isScanning()}>
          <div class="text-center py-8">
            <div class="text-muted mb-4">
              <svg
                class="w-12 h-12 mx-auto mb-2 opacity-50"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.5"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
              <Show
                when={discovery.loading}
                fallback={
                  <>
                    <p class="text-sm">{getDiscoveryInitialEmptyState(false).title}</p>
                    <p class="text-xs text-muted mt-1">
                      {getDiscoveryInitialEmptyState(false).description}
                    </p>
                  </>
                }
              >
                <p class="text-sm">{getDiscoveryInitialEmptyState(true).title}</p>
                <p class="text-xs text-muted mt-1">
                  {getDiscoveryInitialEmptyState(true).description}
                </p>
              </Show>
            </div>

            {/* Command/connectivity prerequisites for agent deep scans, driven
                by the canonical readiness verdict. The provider prerequisite is
                surfaced tab-wide above; here we handle commands -> connectivity. */}
            <Show
              when={
                props.resourceType === 'agent' && !connectedAgents.loading && !discoveryInfo.loading
              }
            >
              <Show when={discoveryReadiness().status === 'needs_commands'}>
                <div class="mb-4 mx-auto max-w-md rounded-md border border-amber-200 bg-amber-50 p-3 text-left dark:border-amber-800 dark:bg-amber-900">
                  <div class="flex items-start gap-2">
                    <svg
                      class="w-4 h-4 text-amber-500 dark:text-amber-400 flex-shrink-0 mt-0.5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                      />
                    </svg>
                    <div class="text-xs">
                      <p class="font-medium text-amber-800 dark:text-amber-200">
                        Commands not enabled
                      </p>
                      <p class="text-amber-700 dark:text-amber-300 mt-0.5">
                        Discovery requires command execution. Enable Pulse commands from{' '}
                        <a href={commandSettingsTarget.href} class="underline hover:no-underline">
                          {commandSettingsTarget.label}
                        </a>
                        .
                      </p>
                    </div>
                  </div>
                </div>
              </Show>
              <Show when={discoveryReadiness().status === 'needs_connected_agent'}>
                <div class="mb-4 mx-auto max-w-md rounded-md border border-amber-200 bg-amber-50 p-3 text-left dark:border-amber-800 dark:bg-amber-900">
                  <div class="flex items-start gap-2">
                    <svg
                      class="w-4 h-4 text-amber-500 dark:text-amber-400 flex-shrink-0 mt-0.5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                      />
                    </svg>
                    <div class="text-xs">
                      <p class="font-medium text-amber-800 dark:text-amber-200">
                        Agent not connected for commands
                      </p>
                      <p class="text-amber-700 dark:text-amber-300 mt-0.5">
                        Commands are enabled, but the agent isn't connected via WebSocket. Check
                        that the API token has the{' '}
                        <code class="px-1 py-0.5 bg-amber-100 dark:bg-amber-800 rounded">
                          agent:exec
                        </code>{' '}
                        scope in{' '}
                        <a href={apiAccessSettingsTarget.href} class="underline hover:no-underline">
                          {apiAccessSettingsTarget.label}
                        </a>
                        .
                      </p>
                    </div>
                  </div>
                </div>
              </Show>
              {/* Kept literal, not `status === 'ready'`: the readiness verdict
                  treats unknown command state as ready (don't-block), but this
                  green "connected" claim must require a genuinely connected
                  agent with commands explicitly enabled. */}
              <Show when={props.commandsEnabled === true && hasConnectedAgent()}>
                <div class="mb-4 mx-auto max-w-md rounded-md border border-green-200 bg-green-50 p-3 text-left dark:border-green-800 dark:bg-green-900">
                  <div class="flex items-center gap-2">
                    <svg
                      class="w-4 h-4 text-green-500 dark:text-green-400 flex-shrink-0"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M5 13l4 4L19 7"
                      />
                    </svg>
                    <p class="text-xs font-medium text-green-800 dark:text-green-200">
                      Agent connected and ready for command execution
                    </p>
                  </div>
                </div>
              </Show>
            </Show>

            <Show when={!showManualRunAction()}>
              <button
                onClick={() => handleTriggerDiscovery(true)}
                disabled={isScanning() || !canTriggerDiscovery()}
                class="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                {isScanning() ? (
                  <span class="flex items-center">
                    <span class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full mr-2"></span>
                    Scanning...
                  </span>
                ) : discovery.loading ? (
                  'Run Discovery Now'
                ) : (
                  'Run Discovery'
                )}
              </button>
            </Show>
          </div>
        </Show>

        {/* Discovery exists but has no meaningful data - show re-scan option */}
        <Show when={!discovery.loading && discovery() && !hasValidDiscovery() && !isScanning()}>
          <div class="text-center py-8">
            <div class="text-muted mb-4">
              <svg
                class="w-12 h-12 mx-auto mb-2 opacity-50"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.5"
                  d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <p class="text-sm">Unknown Service</p>
              <p class="text-xs text-muted mt-1">
                Discovery completed but couldn't identify a known service.
              </p>
              <Show when={discovery()?.updated_at}>
                <p class="text-xs text-muted mt-2">
                  Last scanned: {formatDiscoveryAge(discovery()!.updated_at)}
                </p>
              </Show>
            </div>
            <Show when={!showManualRunAction()}>
              <button
                onClick={() => handleTriggerDiscovery(true)}
                disabled={isScanning() || !canTriggerDiscovery()}
                class="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                {isScanning() ? (
                  <span class="flex items-center justify-center">
                    <span class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full mr-2"></span>
                    Scanning...
                  </span>
                ) : (
                  'Re-scan Discovery'
                )}
              </button>
            </Show>
          </div>
        </Show>

        {/* Discovery data with valid results */}
        <Show when={validDiscovery()}>
          {(d) => (
            <div class="space-y-4">
              {/* Service Header */}
              <div class="rounded border border-border bg-surface p-3 shadow-sm">
                <div class="flex items-start justify-between">
                  <div>
                    <div class="flex flex-wrap items-center gap-1.5">
                      <h3 class="text-sm font-semibold text-base-content">
                        {d().service_name || 'Unknown Service'}
                      </h3>
                      <DiscoveryProvenanceMarker />
                    </div>
                    <Show when={d().service_version}>
                      <div class="mt-1 flex items-center gap-2">
                        <DiscoveryProvenanceMarker showLabel={false} />
                        <span class="text-xs text-muted">Version {d().service_version}</span>
                        <CopyValueButton
                          value={d().service_version}
                          copiedValue={copiedDiscoveryValue}
                          onCopy={handleCopyDiscoveryValue}
                          label="Copy service version"
                          class="inline-flex min-h-5 min-w-5 shrink-0 items-center justify-center rounded text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
                        />
                      </div>
                    </Show>
                  </div>
                  <Show when={d().category && d().category !== 'unknown'}>
                    <span class={getDiscoveryCategoryBadgeClass()}>
                      {getCategoryDisplayName(d().category)}
                    </span>
                  </Show>
                </div>

                <Show when={confidenceInfo()}>
                  <p class={`text-xs mt-2 ${confidenceInfo()!.color}`}>
                    {confidenceInfo()!.label} ({Math.round((d().confidence || 0) * 100)}%)
                  </p>
                </Show>
                <div class="mt-3 flex flex-wrap gap-2 text-[10px] text-muted">
                  <DiscoveryProvenanceMarker label={getDiscoveryObservedSourceLabel()} />
                  <Show when={d().updated_at}>
                    <span class="rounded border border-border bg-surface-alt px-2 py-0.5">
                      Last observed {formatDiscoveryAge(d().updated_at)}
                    </span>
                  </Show>
                  <span class="rounded border border-border bg-surface-alt px-2 py-0.5">
                    Available to Pulse Assistant
                  </span>
                </div>
              </div>

              <Show when={d().suggested_url || d().suggested_url_diagnostic}>
                <div class={getDiscoverySuggestedURLCardClass()}>
                  <div
                    class={`${getDiscoverySuggestedURLHeadingClass()} flex items-center gap-1.5`}
                  >
                    <span>Web Interface Suggestion</span>
                    <DiscoveryProvenanceMarker />
                  </div>
                  <Show
                    when={d().suggested_url}
                    fallback={
                      <div class="text-xs text-blue-800 dark:text-blue-200">
                        <p class="font-medium">{getDiscoverySuggestedURLFallback().title}</p>
                        <p class="mt-1 text-blue-700 dark:text-blue-300">
                          {
                            getDiscoverySuggestedURLFallback(d().suggested_url_diagnostic)
                              .description
                          }
                        </p>
                      </div>
                    }
                  >
                    <Show when={suggestedURLReasonText()}>
                      <p
                        class={`mb-1 text-[10px] ${getDiscoverySuggestedURLTextClass()}`}
                        title={suggestedURLReasonTitle()}
                      >
                        Why this URL: {suggestedURLReasonText()}
                      </p>
                    </Show>
                    <div class="flex items-start gap-2">
                      <code class={getDiscoverySuggestedURLCodeClass()}>{d().suggested_url}</code>
                      <a
                        href={d().suggested_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        class={getDiscoverySuggestedURLActionClass()}
                        title="Open suggested URL"
                        aria-label="Open suggested URL"
                      >
                        <ExternalLinkIcon class="h-3.5 w-3.5" />
                      </a>
                      <CopyValueButton
                        value={d().suggested_url}
                        copiedValue={copiedDiscoveryValue}
                        onCopy={handleCopyDiscoveryValue}
                        label="Copy suggested URL"
                        class={getDiscoverySuggestedURLActionClass()}
                      />
                    </div>
                    <p class={`mt-1.5 text-[11px] ${getDiscoverySuggestedURLTextClass()}`}>
                      Save it in the Web Interface URL field before Pulse uses it as a link.
                    </p>
                  </Show>
                </div>
              </Show>

              {/* CLI Access */}
              <Show when={d().cli_access}>
                <div class="rounded border border-border p-3 shadow-sm">
                  <div class="mb-2 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-base-content">
                    <span>CLI Access</span>
                    <DiscoveryProvenanceMarker showLabel={false} />
                  </div>
                  <CopyableCodeRow
                    value={d().cli_access}
                    copiedValue={copiedDiscoveryValue}
                    onCopy={handleCopyDiscoveryValue}
                    label="Copy CLI access"
                  />
                </div>
              </Show>

              {/* Configuration, Data & Log Paths */}
              <Show
                when={
                  d().config_paths?.length > 0 ||
                  d().data_paths?.length > 0 ||
                  d().log_paths?.length > 0
                }
              >
                <div class="rounded border border-border p-3 shadow-sm">
                  <Show when={d().config_paths?.length > 0}>
                    <div class="mb-3">
                      <div class="mb-1 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-base-content">
                        <span>Config Paths</span>
                        <DiscoveryProvenanceMarker showLabel={false} />
                      </div>
                      <div class="space-y-1">
                        <For each={d().config_paths}>
                          {(path) => (
                            <CopyableCodeRow
                              value={path}
                              copiedValue={copiedDiscoveryValue}
                              onCopy={handleCopyDiscoveryValue}
                              label="Copy config path"
                            />
                          )}
                        </For>
                      </div>
                    </div>
                  </Show>
                  <Show when={d().data_paths?.length > 0}>
                    <div class="mb-3">
                      <div class="mb-1 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-base-content">
                        <span>Data Paths</span>
                        <DiscoveryProvenanceMarker showLabel={false} />
                      </div>
                      <div class="space-y-1">
                        <For each={d().data_paths}>
                          {(path) => (
                            <CopyableCodeRow
                              value={path}
                              copiedValue={copiedDiscoveryValue}
                              onCopy={handleCopyDiscoveryValue}
                              label="Copy data path"
                            />
                          )}
                        </For>
                      </div>
                    </div>
                  </Show>
                  <Show when={d().log_paths?.length > 0}>
                    <div>
                      <div class="mb-1 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-base-content">
                        <span>Log Paths</span>
                        <DiscoveryProvenanceMarker showLabel={false} />
                      </div>
                      <div class="space-y-1">
                        <For each={d().log_paths}>
                          {(path) => (
                            <CopyableCodeRow
                              value={path}
                              copiedValue={copiedDiscoveryValue}
                              onCopy={handleCopyDiscoveryValue}
                              label="Copy log path"
                            />
                          )}
                        </For>
                      </div>
                    </div>
                  </Show>
                </div>
              </Show>

              {/* Ports */}
              <Show when={d().ports?.length > 0}>
                <div class="rounded border border-border p-3 shadow-sm">
                  <div class="mb-2 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-base-content">
                    <span>Listening Ports</span>
                    <DiscoveryProvenanceMarker showLabel={false} />
                  </div>
                  <div class="flex flex-wrap gap-1">
                    <For each={d().ports}>
                      {(port) => (
                        <button
                          type="button"
                          class="inline-flex items-center gap-1 rounded bg-surface-alt px-1.5 py-0.5 text-[10px] text-base-content transition-colors hover:bg-surface-hover"
                          onClick={() =>
                            void handleCopyDiscoveryValue(`${port.port}/${port.protocol}`)
                          }
                          title="Copy port"
                          aria-label={`Copy ${port.port}/${port.protocol}`}
                        >
                          <span>
                            {port.port}/{port.protocol}
                          </span>
                          <Show when={port.process}>
                            <span class="text-muted">({port.process})</span>
                          </Show>
                          <Show
                            when={copiedDiscoveryValue() === `${port.port}/${port.protocol}`}
                            fallback={<CopyIcon class="h-3 w-3 text-muted" />}
                          >
                            <CheckIcon class="h-3 w-3 text-emerald-600 dark:text-emerald-400" />
                          </Show>
                        </button>
                      )}
                    </For>
                  </div>
                </div>
              </Show>

              {/* Key Facts */}
              <Show when={d().facts?.length > 0}>
                <div class="rounded border border-border p-3 shadow-sm">
                  <div class="mb-2 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-base-content">
                    <span>Discovered Facts</span>
                    <DiscoveryProvenanceMarker showLabel={false} />
                  </div>
                  <div class="space-y-1.5">
                    <For each={d().facts.slice(0, 8)}>
                      {(fact) => (
                        <div class="flex items-center justify-between gap-2 text-xs">
                          <span class="min-w-0 text-muted truncate">{fact.key}</span>
                          <div class="flex min-w-0 items-center gap-1.5">
                            <span class="truncate font-medium text-base-content" title={fact.value}>
                              {fact.value}
                            </span>
                            <CopyValueButton
                              value={fact.value}
                              copiedValue={copiedDiscoveryValue}
                              onCopy={handleCopyDiscoveryValue}
                              label={`Copy ${fact.key}`}
                              class="inline-flex min-h-5 min-w-5 shrink-0 items-center justify-center rounded text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
                            />
                          </div>
                        </div>
                      )}
                    </For>
                  </div>
                </div>
              </Show>

              {/* User Notes */}
              <div class="rounded border border-border p-3 shadow-sm">
                <div class="flex items-center justify-between mb-2">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
                    Your Notes
                  </div>
                  <Show when={!editingNotes()}>
                    <button
                      onClick={startEditingNotes}
                      class="text-xs text-blue-600 dark:text-blue-400 hover:underline"
                    >
                      {d().user_notes ? 'Edit' : 'Add notes'}
                    </button>
                  </Show>
                </div>

                <Show
                  when={editingNotes()}
                  fallback={
                    <Show
                      when={d().user_notes}
                      fallback={
                        <p class="text-xs text-muted italic">
                          {getDiscoveryNotesEmptyState().text}
                        </p>
                      }
                    >
                      <p class="text-xs text-muted whitespace-pre-wrap">{d().user_notes}</p>
                    </Show>
                  }
                >
                  <div class="space-y-2">
                    <textarea
                      value={notesText()}
                      onInput={(e) => setNotesText(e.currentTarget.value)}
                      placeholder="Add notes about this resource (API tokens, passwords, important info)..."
                      class="w-full h-24 px-2 py-1.5 text-xs border border-border rounded bg-surface text-base-content focus:outline-none focus:ring-1 focus:ring-blue-500"
                    />
                    <Show when={saveError()}>
                      <p
                        role="alert"
                        aria-live="assertive"
                        class="text-xs text-red-600 dark:text-red-400"
                      >
                        {saveError()}
                      </p>
                    </Show>
                    <div class="flex gap-2">
                      <button
                        onClick={handleSaveNotes}
                        class="px-3 py-1 bg-blue-600 text-white text-xs rounded hover:bg-blue-700 transition-colors"
                      >
                        Save
                      </button>
                      <button
                        onClick={() => setEditingNotes(false)}
                        class="px-3 py-1 bg-surface-hover text-base-content text-xs rounded hover:bg-slate-300 transition-colors"
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                </Show>
              </div>

              {/* Analysis reasoning (collapsible) */}
              <Show when={d().ai_reasoning}>
                <details class="rounded border shadow-sm">
                  <summary class="p-3 text-[11px] font-medium uppercase tracking-wide text-base-content cursor-pointer hover:bg-surface-hover">
                    {DISCOVERY_ANALYSIS_REASONING_LABEL}
                  </summary>
                  <div class="px-3 pb-3">
                    <p class="text-xs text-muted">{d().ai_reasoning}</p>
                  </div>
                </details>
              </Show>

              {/* Scan Details / Raw Command Outputs (collapsible) */}
              <Show
                when={d().raw_command_output && Object.keys(d().raw_command_output!).length > 0}
              >
                <details class="rounded border shadow-sm">
                  <summary class="p-3 text-[11px] font-medium uppercase tracking-wide text-base-content cursor-pointer hover:bg-surface-hover">
                    Scan Details ({Object.keys(d().raw_command_output!).length} commands)
                  </summary>
                  <div class="px-3 pb-3 space-y-3">
                    <For each={Object.entries(d().raw_command_output!)}>
                      {([cmdName, output]) => (
                        <div>
                          <div class="text-xs font-medium text-base-content mb-1">{cmdName}</div>
                          <pre class="text-[10px] bg-surface-alt rounded p-2 overflow-x-auto text-muted max-h-32 overflow-y-auto">
                            {output || '(no output)'}
                          </pre>
                        </div>
                      )}
                    </For>
                  </div>
                </details>
              </Show>

              {/* Commands Run (for non-admin users who can't see full output) */}
              <Show when={!d().raw_command_output && d().scan_duration && d().scan_duration > 0}>
                <div class="rounded border border-border bg-surface p-3 shadow-sm">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-1">
                    Scan Info
                  </div>
                  <p class="text-xs text-muted">
                    Scan completed in {(d().scan_duration! / 1000).toFixed(1)}s. Full scan details
                    are available to administrators.
                  </p>
                </div>
              </Show>

              {/* Footer with Update button */}
              <div
                class={`flex items-center pt-2 border-t border-border ${showManualRunAction() ? 'justify-start' : 'justify-between'}`}
              >
                <span class="text-xs text-muted">
                  Last updated: {formatDiscoveryAge(d().updated_at)}
                </span>
                <Show when={!showManualRunAction()}>
                  <button
                    onClick={() => handleTriggerDiscovery(true)}
                    disabled={isScanning() || !canTriggerDiscovery()}
                    class="px-3 py-1.5 bg-surface-hover text-base-content text-xs rounded hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center gap-1.5"
                  >
                    <Show
                      when={isScanning()}
                      fallback={
                        <>
                          <svg
                            class="w-3.5 h-3.5"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                            />
                          </svg>
                          Update Discovery
                        </>
                      }
                    >
                      <span class="animate-spin h-3.5 w-3.5 border-2 border-slate-500 border-t-transparent rounded-full"></span>
                      Scanning...
                    </Show>
                  </button>
                </Show>
              </div>
            </div>
          )}
        </Show>
      </Show>
    </div>
  );
};

export default DiscoveryTab;
