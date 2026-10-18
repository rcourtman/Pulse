import { Component, For, createUniqueId } from 'solid-js';
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
import { useSystemLogsPanelState } from './useSystemLogsPanelState';

export const SystemLogsPanel: Component = () => {
  const state = useSystemLogsPanelState();
  const levelHelpId = createUniqueId();
  const displayHelpId = createUniqueId();
  const bundleHelpId = createUniqueId();

  const streamPresentation = () => getSystemLogStreamPresentation(state.isPaused());

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

        {/* Terminal View */}
        <div class="p-4 sm:p-6">
          <div
            ref={state.setLogContainer}
            class="bg-slate-950 text-slate-300 font-mono text-xs p-4 rounded-md h-[500px] overflow-y-auto whitespace-pre-wrap leading-relaxed border border-border-subtle scrollbar-thin scrollbar-thumb-slate-700 scrollbar-track-transparent"
          >
            <For each={state.logs()}>
              {(log) => (
                <div class="border-b border-border-subtle last:border-0 pb-0.5 mb-0.5 hover:bg-surface-hover px-1 -mx-1 rounded-sm">
                  <span class={getSystemLogLineClass(log)}>{log}</span>
                </div>
              )}
            </For>

            {state.logs().length === 0 && !state.isLoading() && (
              <div class="h-full flex flex-col items-center justify-center ">
                <Terminal size={48} class="mb-4 opacity-50" />
                <p>{SYSTEM_LOGS_PANEL_COPY.emptyState}</p>
              </div>
            )}
          </div>

          <div class="text-xs text-muted flex justify-between px-1 pt-4">
            <span>{getSystemLogBufferSummary(state.logs().length, state.maxLogs)}</span>
            <span class="flex items-center gap-2">
              <div class={`w-2 h-2 rounded-full ${streamPresentation().indicatorClass}`}></div>
              {streamPresentation().label}
            </span>
          </div>
          <p class="text-xs text-muted px-1 pt-2">{SYSTEM_LOGS_PANEL_COPY.bufferHelp}</p>
        </div>
      </OperationsPanel>
    </div>
  );
};
