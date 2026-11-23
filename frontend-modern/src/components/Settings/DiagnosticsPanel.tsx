import { Component, Show, createUniqueId } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Download from 'lucide-solid/icons/download';
import { DiagnosticsResultsPanel } from '@/components/Settings/DiagnosticsResultsPanel';
import {
  useDiagnosticsPanelState,
  type DiagnosticsPanelProps,
} from '@/components/Settings/useDiagnosticsPanelState';
import { formatUptime } from '@/components/Settings/diagnosticsModel';
import { DIAGNOSTICS_PANEL_COPY } from '@/utils/diagnosticsPresentation';
import { TROUBLESHOOTING_DOC_URL } from '@/utils/docsLinks';

export const DiagnosticsPanel: Component<DiagnosticsPanelProps> = (props) => {
  const {
    diagnosticsData,
    error,
    exportDiagnostics,
    exportLoading,
    loading,
    notice,
    runDiagnostics,
  } = useDiagnosticsPanelState(props);
  const runSafetyId = createUniqueId();
  const exportSafetyId = createUniqueId();

  return (
    <Show
      when={props.canRead !== false}
      fallback={
        <Card
          data-settings-panel
          padding="none"
          border={false}
          class="overflow-hidden border border-border"
        >
          <p class="p-4 text-sm text-muted">{DIAGNOSTICS_PANEL_COPY.accessUnavailable}</p>
        </Card>
      }
    >
      <div class="space-y-6">
        <Card
          data-settings-panel
          padding="none"
          border={false}
          class="overflow-hidden border border-border divide-y divide-border"
        >
          <div class="px-4 sm:px-6 py-3 sm:py-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between bg-surface-alt">
            <div class="min-w-0 space-y-1">
              <p class="text-sm text-base-content font-medium">{DIAGNOSTICS_PANEL_COPY.summary}</p>
              <Show when={diagnosticsData()}>
                <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
                  <span>
                    {DIAGNOSTICS_PANEL_COPY.versionLabel}: {diagnosticsData()?.version}
                  </span>
                  <span>
                    {DIAGNOSTICS_PANEL_COPY.uptimeLabel}:{' '}
                    {formatUptime(diagnosticsData()?.uptime || 0)}
                  </span>
                </div>
              </Show>
            </div>
            <Show when={diagnosticsData() || loading()}>
              <div class="flex items-center justify-end gap-2 flex-wrap shrink-0">
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
            </Show>
          </div>
          <div class="px-4 sm:px-6 py-3 sm:py-4 space-y-2 text-xs text-muted">
            <Show when={error()}>
              <p role="alert" class="text-sm text-red-700 dark:text-red-300">
                {error()}
              </p>
            </Show>
            <Show when={notice()}>
              <p role="status" class="text-sm text-base-content">
                {notice()}
              </p>
            </Show>
            <p id={runSafetyId}>{DIAGNOSTICS_PANEL_COPY.runSafety}</p>
            <p id={exportSafetyId}>{DIAGNOSTICS_PANEL_COPY.exportSafety}</p>
            <div class="pt-1">
              <ExternalTextLink
                href={`${TROUBLESHOOTING_DOC_URL}#collect-diagnostics-safely`}
                class="inline-flex min-h-11 sm:min-h-0 items-center text-xs font-medium text-blue-600 underline underline-offset-2 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
              >
                Safe diagnostics and sharing
              </ExternalTextLink>
            </div>
          </div>
          <Show when={diagnosticsData()}>
            <div class="px-4 sm:px-6 py-3 sm:py-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
              <p class="text-xs text-muted">Export displayed diagnostics snapshot:</p>
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
            </div>
          </Show>
        </Card>
        <DiagnosticsResultsPanel
          diagnosticsData={diagnosticsData()}
          loading={loading()}
          onRunDiagnostics={runDiagnostics}
          runDescriptionId={runSafetyId}
        />
      </div>
    </Show>
  );
};

export default DiagnosticsPanel;
