import {
  Component,
  For,
  Show,
  createMemo,
  createSignal,
  createUniqueId,
  onCleanup,
} from 'solid-js';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import { SearchField } from '@/components/shared/SearchField';
import {
  getSystemLogBufferSummary,
  getSystemLogLevelBadgeClass,
  getSystemLogLineClass,
  getSystemLogStreamPresentation,
  parseSystemLogLine,
  SYSTEM_LOG_LEVEL_OPTIONS,
  SYSTEM_LOGS_PANEL_COPY,
  type SystemLogLevel,
} from '@/utils/systemLogsPresentation';
import { copyToClipboard } from '@/utils/clipboard';
import Check from 'lucide-solid/icons/check';
import Copy from 'lucide-solid/icons/copy';
import Download from 'lucide-solid/icons/download';
import Pause from 'lucide-solid/icons/pause';
import Play from 'lucide-solid/icons/play';
import Trash2 from 'lucide-solid/icons/trash-2';
import Terminal from 'lucide-solid/icons/terminal';
import { FormSelect } from '@/components/shared/FormSelect';
import { Button } from '@/components/shared/Button';
import { useSystemLogsPanelState, type SystemLogsPanelProps } from './useSystemLogsPanelState';

type LogLevelFilter = 'all' | SystemLogLevel;

const LOG_LEVEL_FILTERS: readonly { value: LogLevelFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'error', label: 'Error' },
  { value: 'warn', label: 'Warn' },
  { value: 'info', label: 'Info' },
  { value: 'debug', label: 'Debug' },
];

export const SystemLogsPanel: Component<SystemLogsPanelProps> = (props) => {
  const state = useSystemLogsPanelState(props);
  const levelHelpId = createUniqueId();
  const displayHelpId = createUniqueId();
  const bundleHelpId = createUniqueId();
  const [searchQuery, setSearchQuery] = createSignal('');
  const [levelFilter, setLevelFilter] = createSignal<LogLevelFilter>('all');
  const [copiedLogs, setCopiedLogs] = createSignal(false);
  let copyTimer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => {
    if (copyTimer) clearTimeout(copyTimer);
  });

  const streamPresentation = () =>
    getSystemLogStreamPresentation(state.isPaused(), state.connectionState());

  const parsedLogs = createMemo(() => state.logs().map(parseSystemLogLine));
  const filteredLogs = createMemo(() => {
    const query = searchQuery().trim().toLowerCase();
    const activeLevel = levelFilter();
    return parsedLogs().filter((entry) => {
      if (activeLevel !== 'all' && entry.level !== activeLevel) return false;
      if (query && !entry.raw.toLowerCase().includes(query)) return false;
      return true;
    });
  });

  const hasActiveFilter = () => searchQuery().trim().length > 0 || levelFilter() !== 'all';

  const handleCopyVisibleLogs = async () => {
    const text = filteredLogs()
      .map((entry) => entry.raw)
      .join('\n');
    if (!text) return;
    const copied = await copyToClipboard(text);
    if (!copied) return;
    setCopiedLogs(true);
    if (copyTimer) clearTimeout(copyTimer);
    copyTimer = setTimeout(() => setCopiedLogs(false), 2000);
  };

  return (
    <div class="space-y-6">
      <OperationsPanel>
        {/* Controls */}
        <div class="p-3 sm:p-5 space-y-3 bg-surface-alt/50">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <FormSelect
              id={`system-log-level-${levelHelpId}`}
              label={SYSTEM_LOGS_PANEL_COPY.levelLabel}
              aria-describedby={levelHelpId}
              disabled={!state.canUseControls() || state.changingLevel()}
              value={state.level()}
              onChange={(e) => void state.handleLevelChange(e.currentTarget.value)}
              fieldBaseClass="flex"
              fieldClass="items-center space-x-3"
              labelClass="text-sm font-medium text-base-content whitespace-nowrap"
              selectBaseClass="form-select min-h-11 sm:min-h-9 text-sm py-2 px-3 rounded-md border border-border bg-surface text-base-content focus:ring-primary-500 focus:border-primary-500 disabled:opacity-50"
            >
              <For each={SYSTEM_LOG_LEVEL_OPTIONS}>
                {(option) => <option value={option.value}>{option.label}</option>}
              </For>
            </FormSelect>

            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={state.togglePaused}
                aria-label={streamPresentation().toggleTitle}
                aria-describedby={displayHelpId}
                aria-pressed={state.isPaused()}
                class={`min-h-11 sm:min-h-9 min-w-11 sm:min-w-9 p-2.5 rounded transition-colors ${
                  streamPresentation().pauseButtonClass
                }`}
                title={streamPresentation().toggleTitle}
              >
                {state.isPaused() ? (
                  <Play size={18} aria-hidden="true" />
                ) : (
                  <Pause size={18} aria-hidden="true" />
                )}
              </button>
              <button
                type="button"
                onClick={state.clearLogs}
                aria-label={SYSTEM_LOGS_PANEL_COPY.clearTitle}
                aria-describedby={displayHelpId}
                class="min-h-11 sm:min-h-9 min-w-11 sm:min-w-9 p-2.5 rounded-sm hover:bg-surface-hover text-muted transition-colors"
                title={SYSTEM_LOGS_PANEL_COPY.clearTitle}
              >
                <Trash2 size={18} aria-hidden="true" />
              </button>
              <Button
                type="button"
                variant="secondary"
                size="settingsActionXs"
                class="gap-1.5"
                disabled={filteredLogs().length === 0}
                onClick={() => void handleCopyVisibleLogs()}
              >
                {copiedLogs() ? (
                  <Check size={14} aria-hidden="true" />
                ) : (
                  <Copy size={14} aria-hidden="true" />
                )}
                <span>{copiedLogs() ? 'Copied' : 'Copy logs'}</span>
              </Button>
              <div class="hidden h-6 w-px bg-border sm:block"></div>
              <Button
                type="button"
                disabled={!state.canUseControls()}
                onClick={state.handleDownload}
                aria-describedby={bundleHelpId}
                variant="primaryFlat"
                size="settingsAction"
                class="gap-2"
              >
                <Download size={16} aria-hidden="true" />
                <span>{SYSTEM_LOGS_PANEL_COPY.downloadLabel}</span>
              </Button>
            </div>
          </div>

          {/* Search and client-side level filter bar */}
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex-1 min-w-0 sm:max-w-sm">
              <SearchField
                value={searchQuery()}
                onChange={setSearchQuery}
                title="Filter log output"
                placeholder="Filter log lines..."
              />
            </div>
            <div
              class="flex flex-wrap items-center gap-1"
              role="group"
              aria-label="Filter by log level"
            >
              <For each={LOG_LEVEL_FILTERS}>
                {(chip) => (
                  <button
                    type="button"
                    aria-pressed={levelFilter() === chip.value}
                    onClick={() => setLevelFilter(chip.value)}
                    class={`min-h-9 sm:min-h-8 rounded-md border px-2.5 py-1 text-xs font-medium transition-colors ${
                      levelFilter() === chip.value
                        ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900/30 dark:text-blue-200'
                        : 'border-border bg-surface text-muted hover:bg-surface-hover hover:text-base-content'
                    }`}
                  >
                    {chip.label}
                  </button>
                )}
              </For>
            </div>
          </div>

          <div class="grid gap-1.5 rounded-md border border-border/60 bg-surface px-3 py-2 text-[11px] leading-snug text-muted">
            <p id={levelHelpId}>{SYSTEM_LOGS_PANEL_COPY.levelHelp}</p>
            <p id={displayHelpId}>{SYSTEM_LOGS_PANEL_COPY.displayHelp}</p>
            <p id={bundleHelpId}>{SYSTEM_LOGS_PANEL_COPY.bundleHelp}</p>
          </div>
        </div>

        <Show when={state.error() || state.notice() || state.canReconnect()}>
          <div class="px-4 sm:px-6 py-3 space-y-2">
            <Show when={state.error()}>
              <p role="alert" class="text-sm text-red-600 dark:text-red-400">
                {state.error()}
              </p>
            </Show>
            <Show when={state.notice()}>
              <p role="status" class="text-sm text-muted">
                {state.notice()}
              </p>
            </Show>
            <Show when={state.canReconnect()}>
              <Button type="button" size="settingsAction" onClick={() => void state.reconnect()}>
                Reconnect logs
              </Button>
            </Show>
          </div>
        </Show>

        {/* Terminal View */}
        <div class="p-3 sm:p-5">
          <div
            ref={state.setLogContainer}
            class="bg-slate-950 text-slate-300 font-mono text-xs p-3 sm:p-4 rounded-md h-[420px] sm:h-[500px] overflow-y-auto whitespace-pre-wrap wrap-anywhere leading-relaxed border border-border scrollbar-thin scrollbar-thumb-slate-700 scrollbar-track-transparent"
          >
            <For each={filteredLogs()}>
              {(entry) => (
                <div class="border-b border-slate-800/80 last:border-0 pb-1 mb-1 hover:bg-slate-900/70 px-1.5 -mx-1 rounded-sm">
                  <Show
                    when={entry.isStructured}
                    fallback={<span class={getSystemLogLineClass(entry.raw)}>{entry.raw}</span>}
                  >
                    <div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                      <Show when={entry.timestamp}>
                        <span class="shrink-0 text-slate-500 tabular-nums">{entry.timestamp}</span>
                      </Show>
                      <Show when={entry.level !== 'unknown'}>
                        <span
                          class={`shrink-0 rounded-sm border px-1 py-0 text-[10px] font-semibold uppercase ${getSystemLogLevelBadgeClass(entry.level)}`}
                        >
                          {entry.level}
                        </span>
                      </Show>
                      <span class={getSystemLogLineClass(entry.raw)}>{entry.message}</span>
                      <Show when={entry.fields}>
                        <span class="text-slate-400">{entry.fields}</span>
                      </Show>
                    </div>
                  </Show>
                </div>
              )}
            </For>

            <Show when={filteredLogs().length === 0 && !state.isLoading()}>
              <div class="h-full flex flex-col items-center justify-center text-slate-400">
                <Terminal size={40} class="mb-3 opacity-50" aria-hidden="true" />
                <p>
                  {hasActiveFilter() && state.logs().length > 0
                    ? 'No log lines match the current filter.'
                    : state.canUseControls()
                      ? SYSTEM_LOGS_PANEL_COPY.emptyState
                      : SYSTEM_LOGS_PANEL_COPY.unavailableState}
                </p>
                <Show when={hasActiveFilter() && state.logs().length > 0}>
                  <Button
                    type="button"
                    variant="secondary"
                    size="xs"
                    class="mt-2"
                    onClick={() => {
                      setSearchQuery('');
                      setLevelFilter('all');
                    }}
                  >
                    Clear filter
                  </Button>
                </Show>
              </div>
            </Show>
          </div>

          <div class="text-xs text-muted flex flex-wrap gap-2 justify-between px-1 pt-3">
            <span>
              {hasActiveFilter()
                ? `Showing ${filteredLogs().length} of ${state.logs().length} lines`
                : getSystemLogBufferSummary(state.logs().length, state.maxLogs)}
            </span>
            <span role="status" class="flex items-center gap-2">
              <span
                aria-hidden="true"
                class={`inline-block w-2 h-2 rounded-full ${streamPresentation().indicatorClass}`}
              />
              {streamPresentation().label}
            </span>
          </div>
          <p class="text-[11px] text-muted px-1 pt-1.5">{SYSTEM_LOGS_PANEL_COPY.bufferHelp}</p>
        </div>
      </OperationsPanel>
    </div>
  );
};
