import { Component, Show } from 'solid-js';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import Activity from 'lucide-solid/icons/activity';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Download from 'lucide-solid/icons/download';
import { DiagnosticsResultsPanel } from '@/components/Settings/DiagnosticsResultsPanel';
import { useDiagnosticsPanelState } from '@/components/Settings/useDiagnosticsPanelState';
import { formatUptime } from '@/components/Settings/diagnosticsModel';

export const DiagnosticsPanel: Component = () => {
  const { diagnosticsData, exportDiagnostics, exportLoading, loading, runDiagnostics } =
    useDiagnosticsPanelState();

  return (
    <div class="space-y-6">
      <OperationsPanel
        title="System Diagnostics"
        description="Connection health, configuration status, and troubleshooting tools"
        icon={<Activity class="w-5 h-5 sm:w-5 sm:h-5" />}
        action={
          <div class="flex items-center justify-between sm:justify-end gap-3 flex-wrap">
            <Show when={diagnosticsData()}>
              <div class="text-left sm:text-right text-xs text-muted">
                <div>Version {diagnosticsData()?.version}</div>
                <div>Uptime: {formatUptime(diagnosticsData()?.uptime || 0)}</div>
              </div>
            </Show>
            <button
              type="button"
              onClick={runDiagnostics}
              disabled={loading()}
              class="flex min-h-10 sm:min-h-9 min-w-10 items-center gap-2 px-3 sm:px-4 py-2.5 rounded-md font-medium text-sm transition-colors whitespace-nowrap bg-blue-600 hover:bg-blue-700 text-white disabled:opacity-50 disabled:bg-surface disabled:text-muted"
            >
              <RefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
              <span class="sm:hidden">{loading() ? '...' : 'Run'}</span>
              <span class="hidden sm:inline">{loading() ? 'Running...' : 'Run Diagnostics'}</span>
            </button>
          </div>
        }
      >
        <div class="px-4 sm:px-6 py-3 sm:py-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <p class="text-xs text-muted">Test all connections and inspect runtime configuration</p>
          <Show when={diagnosticsData()}>
            <div class="flex items-center gap-2 flex-wrap">
              <button
                type="button"
                onClick={() => exportDiagnostics(false)}
                disabled={exportLoading()}
                class="flex min-h-10 sm:min-h-9 items-center gap-1.5 px-3 py-2 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover transition-colors"
              >
                <Download class="w-3.5 h-3.5" />
                Full
              </button>
              <button
                type="button"
                onClick={() => exportDiagnostics(true)}
                disabled={exportLoading()}
                class="flex min-h-10 sm:min-h-9 items-center gap-1.5 px-3 py-2 text-sm font-medium text-green-700 dark:text-green-300 bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 rounded-md hover:bg-green-100 dark:hover:bg-green-900 transition-colors"
              >
                <Download class="w-3.5 h-3.5" />
                GitHub
              </button>
            </div>
          </Show>
        </div>
      </OperationsPanel>
      <DiagnosticsResultsPanel
        diagnosticsData={diagnosticsData()}
        loading={loading()}
        onRunDiagnostics={runDiagnostics}
      />
    </div>
  );
};

export default DiagnosticsPanel;
