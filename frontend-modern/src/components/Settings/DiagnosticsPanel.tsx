import { Component, Show, createUniqueId } from 'solid-js';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Download from 'lucide-solid/icons/download';
import { DiagnosticsResultsPanel } from '@/components/Settings/DiagnosticsResultsPanel';
import { useDiagnosticsPanelState } from '@/components/Settings/useDiagnosticsPanelState';
import { formatUptime } from '@/components/Settings/diagnosticsModel';
import { DIAGNOSTICS_PANEL_COPY } from '@/utils/diagnosticsPresentation';
import { TROUBLESHOOTING_DOC_URL } from '@/utils/docsLinks';

export const DiagnosticsPanel: Component = () => {
  const { diagnosticsData, exportDiagnostics, exportLoading, loading, runDiagnostics } =
    useDiagnosticsPanelState();
  const runSafetyId = createUniqueId();
  const exportSafetyId = createUniqueId();

  return (
    <div class="space-y-6">
      <OperationsPanel
        title={DIAGNOSTICS_PANEL_COPY.title}
        action={
          <div class="flex items-center justify-between sm:justify-end gap-3 flex-wrap">
            <Show when={diagnosticsData()}>
              <div class="text-left sm:text-right text-xs text-muted">
                <div>
                  {DIAGNOSTICS_PANEL_COPY.versionLabel} {diagnosticsData()?.version}
                </div>
                <div>
                  {DIAGNOSTICS_PANEL_COPY.uptimeLabel}:{' '}
                  {formatUptime(diagnosticsData()?.uptime || 0)}
                </div>
              </div>
            </Show>
            <button
              type="button"
              onClick={runDiagnostics}
              disabled={loading()}
              aria-label={
                loading()
                  ? DIAGNOSTICS_PANEL_COPY.runningActionLabel
                  : DIAGNOSTICS_PANEL_COPY.runActionLabel
              }
              aria-describedby={runSafetyId}
              class="flex min-h-11 sm:min-h-9 min-w-11 sm:min-w-10 items-center gap-2 px-3 sm:px-4 py-2.5 rounded-md font-medium text-sm transition-colors whitespace-nowrap bg-blue-600 hover:bg-blue-700 text-white disabled:opacity-50 disabled:bg-surface disabled:text-muted"
            >
              <RefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
              <span class="sm:hidden">
                {loading()
                  ? DIAGNOSTICS_PANEL_COPY.runningActionLabel
                  : DIAGNOSTICS_PANEL_COPY.runShortLabel}
              </span>
              <span class="hidden sm:inline">
                {loading()
                  ? DIAGNOSTICS_PANEL_COPY.runningActionLabel
                  : DIAGNOSTICS_PANEL_COPY.runActionLabel}
              </span>
            </button>
          </div>
        }
      >
        <div class="px-4 sm:px-6 py-3 sm:py-4 space-y-2 text-sm text-muted">
          <p id={runSafetyId}>{DIAGNOSTICS_PANEL_COPY.runSafety}</p>
          <p id={exportSafetyId}>{DIAGNOSTICS_PANEL_COPY.exportSafety}</p>
          <a
            href={`${TROUBLESHOOTING_DOC_URL}#collect-diagnostics-safely`}
            class="inline-flex min-h-11 items-center text-link hover:underline"
          >
            Safe diagnostics and sharing
          </a>
        </div>
        <div class="px-4 sm:px-6 py-3 sm:py-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <p class="text-xs text-muted">{DIAGNOSTICS_PANEL_COPY.summary}</p>
          <Show when={diagnosticsData()}>
            <div class="flex items-center gap-2 flex-wrap">
              <button
                type="button"
                onClick={() => exportDiagnostics(false)}
                disabled={exportLoading() || loading()}
                aria-describedby={exportSafetyId}
                class="flex min-h-11 sm:min-h-9 items-center gap-1.5 px-3 py-2 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover transition-colors"
              >
                <Download class="w-3.5 h-3.5" />
                {DIAGNOSTICS_PANEL_COPY.exportFullLabel}
              </button>
              <button
                type="button"
                onClick={() => exportDiagnostics(true)}
                disabled={exportLoading() || loading()}
                aria-describedby={exportSafetyId}
                class="flex min-h-11 sm:min-h-9 items-center gap-1.5 px-3 py-2 text-sm font-medium text-green-700 dark:text-green-300 bg-green-50 dark:bg-green-900/25 border border-green-200 dark:border-green-800 rounded-md hover:bg-green-100 dark:hover:bg-green-900/25 transition-colors"
              >
                <Download class="w-3.5 h-3.5" />
                {DIAGNOSTICS_PANEL_COPY.exportGithubLabel}
              </button>
            </div>
          </Show>
        </div>
      </OperationsPanel>
      <DiagnosticsResultsPanel
        diagnosticsData={diagnosticsData()}
        loading={loading()}
        onRunDiagnostics={runDiagnostics}
        runDescriptionId={runSafetyId}
      />
    </div>
  );
};

export default DiagnosticsPanel;
