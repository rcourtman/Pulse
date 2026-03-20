import { createSignal } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { showError, showSuccess } from '@/utils/toast';
import {
  buildDiagnosticsExportFilename,
  sanitizeDiagnosticsData,
  type DiagnosticsData,
} from '@/components/Settings/diagnosticsModel';

export const useDiagnosticsPanelState = () => {
  const [loading, setLoading] = createSignal(false);
  const [diagnosticsData, setDiagnosticsData] = createSignal<DiagnosticsData | null>(null);
  const [exportLoading, setExportLoading] = createSignal(false);

  const runDiagnostics = async () => {
    setLoading(true);
    try {
      const data = (await apiFetchJSON('/api/diagnostics')) as DiagnosticsData;
      setDiagnosticsData(data);
      showSuccess('Diagnostics completed');
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to run diagnostics');
    } finally {
      setLoading(false);
    }
  };

  const exportDiagnostics = async (sanitize: boolean) => {
    setExportLoading(true);
    try {
      const data = diagnosticsData();
      if (!data) {
        showError('Run diagnostics first');
        return;
      }

      const exportData = sanitize ? sanitizeDiagnosticsData(data) : data;
      const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = buildDiagnosticsExportFilename(sanitize);
      document.body.appendChild(anchor);
      anchor.click();
      document.body.removeChild(anchor);
      URL.revokeObjectURL(url);
      showSuccess(`Diagnostics exported (${sanitize ? 'sanitized' : 'full'})`);
    } finally {
      setExportLoading(false);
    }
  };

  return {
    diagnosticsData,
    exportDiagnostics,
    exportLoading,
    loading,
    runDiagnostics,
  };
};
