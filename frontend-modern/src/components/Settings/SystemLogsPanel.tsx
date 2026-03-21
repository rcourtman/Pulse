import { Component, For } from 'solid-js';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import {
  getSystemLogLineClass,
  getSystemLogStreamPresentation,
} from '@/utils/systemLogsPresentation';
import Download from 'lucide-solid/icons/download';
import Pause from 'lucide-solid/icons/pause';
import Play from 'lucide-solid/icons/play';
import Trash2 from 'lucide-solid/icons/trash-2';
import Terminal from 'lucide-solid/icons/terminal';
import { useSystemLogsPanelState } from './useSystemLogsPanelState';

export const SystemLogsPanel: Component = () => {
  const state = useSystemLogsPanelState();

  const streamPresentation = () => getSystemLogStreamPresentation(state.isPaused());

  return (
    <div class="space-y-6">
      <OperationsPanel
        title="System Logs"
        description="Stream real-time system logs and download support bundles."
        icon={<Terminal class="w-5 h-5" strokeWidth={2} />}
      >
        {/* Controls */}
        <div class="p-4 sm:p-6">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div class="flex items-center space-x-3">
              <label class="text-sm font-medium text-base-content">Log Level:</label>
              <select
                value={state.level()}
                onChange={(e) => void state.handleLevelChange(e.currentTarget.value)}
                class="form-select min-h-10 sm:min-h-9 text-sm py-2.5 px-3 rounded-md border-border bg-surface text-muted focus:ring-primary-500 focus:border-primary-500"
              >
                <option value="debug">Debug</option>
                <option value="info">Info</option>
                <option value="warn">Warn</option>
                <option value="error">Error</option>
              </select>
            </div>

            <div class="flex items-center space-x-2">
              <button
                onClick={state.togglePaused}
                class={`min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 rounded transition-colors ${
                  streamPresentation().pauseButtonClass
                }`}
                title={state.isPaused() ? 'Resume Stream' : 'Pause Stream'}
              >
                {state.isPaused() ? <Play size={18} /> : <Pause size={18} />}
              </button>
              <button
                onClick={state.clearLogs}
                class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 rounded hover:bg-surface-hover text-muted transition-colors"
                title="Clear Logs"
              >
                <Trash2 size={18} />
              </button>
              <div class="h-6 w-px bg-surface-hover mx-2"></div>
              <button
                onClick={state.handleDownload}
                class="min-h-10 sm:min-h-9 flex items-center space-x-2 px-3 py-2.5 bg-primary-600 text-white rounded-md hover:bg-primary-700 text-sm font-medium transition-colors"
              >
                <Download size={16} />
                <span>Support Bundle</span>
              </button>
            </div>
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
                <div class="animate-enter border-b border-border-subtle last:border-0 pb-0.5 mb-0.5 hover:bg-surface-hover px-1 -mx-1 rounded">
                  <span class={getSystemLogLineClass(log)}>{log}</span>
                </div>
              )}
            </For>

            {state.logs().length === 0 && !state.isLoading() && (
              <div class="h-full flex flex-col items-center justify-center ">
                <Terminal size={48} class="mb-4 opacity-50" />
                <p>Waiting for logs...</p>
              </div>
            )}
          </div>

          <div class="text-xs text-muted flex justify-between px-1 pt-4">
            <span>
              Buffer: {state.logs().length} / {state.maxLogs} lines
            </span>
            <span class="flex items-center gap-2">
              <div class={`w-2 h-2 rounded-full ${streamPresentation().indicatorClass}`}></div>
              {streamPresentation().label}
            </span>
          </div>
        </div>
      </OperationsPanel>
    </div>
  );
};
