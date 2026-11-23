import { Component, For, Show, createMemo, createSignal, createUniqueId } from 'solid-js';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import {
  getSystemLogBufferSummary,
  getSystemLogLineClass,
  getSystemLogStreamPresentation,
  SYSTEM_LOG_LEVEL_OPTIONS,
  SYSTEM_LOGS_PANEL_COPY,
} from '@/utils/systemLogsPresentation';
import Download from 'lucide-solid/icons/download';
import Pause from 'lucide-solid/icons/pause';
import Play from 'lucide-solid/icons/play';
import Trash2 from 'lucide-solid/icons/trash-2';
import Terminal from 'lucide-solid/icons/terminal';
import { FormSelect } from '@/components/shared/FormSelect';
import { Button } from '@/components/shared/Button';
import { SearchInput } from '@/components/shared/SearchInput';
import { useSystemLogsPanelState, type SystemLogsPanelProps } from './useSystemLogsPanelState';

export const SystemLogsPanel: Component<SystemLogsPanelProps> = (props) => {
  const state = useSystemLogsPanelState(props);
  const [filterQuery, setFilterQuery] = createSignal('');
  const levelHelpId = createUniqueId();
  const displayHelpId = createUniqueId();
  const bundleHelpId = createUniqueId();

  const streamPresentation = () =>
    getSystemLogStreamPresentation(state.isPaused(), state.connectionState());

  const normalizedFilterQuery = createMemo(() => filterQuery().trim().toLowerCase());
  const filteredLogs = createMemo(() => {
    const query = normalizedFilterQuery();
    const lines = state.logs();
    if (!query) return lines;
    return lines.filter((line) => line.toLowerCase().includes(query));
  });

  return (
    <div class="space-y-6">
      <OperationsPanel title={SYSTEM_LOGS_PANEL_COPY.title}>
        {/* Controls */}
        <div class="p-4 sm:p-6 space-y-3">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
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
              selectBaseClass="form-select min-h-11 sm:min-h-9 text-sm py-2.5 px-3 rounded-md border-border bg-surface text-muted focus:ring-primary-500 focus:border-primary-500"
            >
              <For each={SYSTEM_LOG_LEVEL_OPTIONS}>
                {(option) => <option value={option.value}>{option.label}</option>}
              </For>
            </FormSelect>

            <div class="flex items-center space-x-2">
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
              <div class="h-6 w-px bg-surface-hover mx-2"></div>
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
          <div class="space-y-2 text-sm text-muted">
            <p id={levelHelpId}>{SYSTEM_LOGS_PANEL_COPY.levelHelp}</p>
            <p id={displayHelpId}>{SYSTEM_LOGS_PANEL_COPY.displayHelp}</p>
            <p id={bundleHelpId}>{SYSTEM_LOGS_PANEL_COPY.bundleHelp}</p>
          </div>
        </div>

        <div class="px-4 sm:px-6 space-y-3">
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

        {/* Terminal View */}
        <div class="p-4 sm:p-6 space-y-3">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <SearchInput
              value={filterQuery}
              onChange={setFilterQuery}
              placeholder="Filter log lines..."
              class="sm:max-w-md"
              typeToSearch={false}
              clearOnFocusedEscape
            />
            <Show when={normalizedFilterQuery().length > 0}>
              <div class="flex items-center gap-2 text-xs text-muted">
                <span role="status">
                  Showing {filteredLogs().length} of {state.logs().length} lines
                </span>
                <button
                  type="button"
                  onClick={() => setFilterQuery('')}
                  class="text-blue-600 dark:text-blue-400 hover:underline font-medium"
                >
                  Clear filter
                </button>
              </div>
            </Show>
          </div>

          <div
            ref={state.setLogContainer}
            class="bg-slate-950 text-slate-300 font-mono text-xs p-4 rounded-md h-[500px] overflow-y-auto whitespace-pre-wrap wrap-anywhere leading-relaxed border border-slate-800/80 scrollbar-thin scrollbar-thumb-slate-700 scrollbar-track-transparent"
          >
            <For each={filteredLogs()}>
              {(log) => (
                <div class="border-b border-slate-800/80 last:border-0 pb-0.5 mb-0.5 hover:bg-slate-900/70 px-1 -mx-1 rounded-sm">
                  <span class={getSystemLogLineClass(log)}>{log}</span>
                </div>
              )}
            </For>

            <Show
              when={
                normalizedFilterQuery().length > 0 &&
                state.logs().length > 0 &&
                filteredLogs().length === 0
              }
            >
              <div class="h-full flex flex-col items-center justify-center gap-2 text-center text-slate-400">
                <p>No log lines match &ldquo;{filterQuery().trim()}&rdquo;.</p>
                <button
                  type="button"
                  onClick={() => setFilterQuery('')}
                  class="text-blue-400 hover:underline"
                >
                  Clear filter
                </button>
              </div>
            </Show>

            {state.logs().length === 0 && !state.isLoading() && (
              <div class="h-full flex flex-col items-center justify-center ">
                <Terminal size={48} class="mb-4 opacity-50" />
                <p>
                  {state.canUseControls()
                    ? SYSTEM_LOGS_PANEL_COPY.emptyState
                    : SYSTEM_LOGS_PANEL_COPY.unavailableState}
                </p>
              </div>
            )}
          </div>

          <div class="text-xs text-muted flex flex-wrap gap-2 justify-between px-1 pt-1">
            <span>{getSystemLogBufferSummary(state.logs().length, state.maxLogs)}</span>
            <span role="status" class="flex items-center gap-2">
              <div class={`w-2 h-2 rounded-full ${streamPresentation().indicatorClass}`}></div>
              {streamPresentation().label}
            </span>
          </div>
          <p class="text-xs text-muted px-1">{SYSTEM_LOGS_PANEL_COPY.bufferHelp}</p>
        </div>
      </OperationsPanel>
    </div>
  );
};
