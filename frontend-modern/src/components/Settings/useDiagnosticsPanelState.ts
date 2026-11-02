import { batch, createEffect, createSignal, onCleanup, untrack } from 'solid-js';
import { apiFetchJSON, getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { DIAGNOSTICS_PANEL_COPY } from '@/utils/diagnosticsPresentation';
import {
  buildDiagnosticsExportFilename,
  sanitizeDiagnosticsData,
  stripInternalAnalyticsDiagnosticsFields,
  type DiagnosticsData,
} from '@/components/Settings/diagnosticsModel';

export interface DiagnosticsPanelProps {
  currentUser?: string;
  accessKey?: string;
  canRead?: boolean;
}

export const useDiagnosticsPanelState = (props: DiagnosticsPanelProps = {}) => {
  const [loading, setLoading] = createSignal(false);
  const [result, setResult] = createSignal<DiagnosticsData | null>(null);
  const [exportLoading, setExportLoading] = createSignal(false);
  const [error, setError] = createSignal('');
  const [notice, setNotice] = createSignal('');
  const currentOrg = () => getOrgID() || 'default';
  const reader = () => ({
    org: currentOrg(),
    user: props.currentUser,
    access: props.accessKey,
    allowed: props.canRead !== false,
  });
  const [context, setContext] = createSignal(reader());
  let disposed = false;
  let controller: AbortController | undefined;
  const isCurrentReader = () =>
    !disposed &&
    context().allowed &&
    props.canRead !== false &&
    context().org === currentOrg() &&
    context().user === props.currentUser &&
    context().access === props.accessKey;
  // Withhold immediately, even before the effect handles a changed prop.
  const diagnosticsData = () => (isCurrentReader() ? result() : null);

  const resetReader = () => {
    controller?.abort();
    controller = undefined;
    batch(() => {
      setContext(reader());
      setResult(null);
      setLoading(false);
      setExportLoading(false);
      setError('');
      setNotice('');
    });
    // Diagnostics can probe live hosts. A new reader must explicitly Run;
    // neither switching workspace nor recovering access starts those probes.
  };
  createEffect(() => {
    const next = reader();
    const previous = untrack(context);
    // A copied status with unchanged reader facts must not discard a result.
    if (
      previous.org !== next.org ||
      previous.user !== next.user ||
      previous.access !== next.access ||
      previous.allowed !== next.allowed
    )
      resetReader();
  });
  const unsubscribe = eventBus.on('org_switched', resetReader);
  onCleanup(() => {
    disposed = true;
    unsubscribe();
    resetReader();
  });

  const runDiagnostics = async () => {
    if (!isCurrentReader() || controller) return;
    const acceptedReader = context();
    const request = new AbortController();
    controller = request;
    const isCurrent = () =>
      isCurrentReader() && context() === acceptedReader && controller === request;
    batch(() => {
      // An earlier successful check is not evidence of current access or health.
      setResult(null);
      setError('');
      setNotice('');
      setLoading(true);
    });
    try {
      const data = (await apiFetchJSON('/api/diagnostics', {
        expectedOrgID: acceptedReader.org,
        signal: request.signal,
        retry: false,
      })) as DiagnosticsData;
      if (!isCurrent()) return;
      if (
        !data ||
        typeof data.version !== 'string' ||
        !Array.isArray(data.nodes) ||
        !Array.isArray(data.pbs) ||
        [...data.nodes, ...data.pbs].some(
          (node) =>
            !node ||
            typeof node.name !== 'string' ||
            typeof node.host !== 'string' ||
            typeof node.connected !== 'boolean',
        ) ||
        !data.system ||
        typeof data.system !== 'object' ||
        Array.isArray(data.system) ||
        !Array.isArray(data.errors) ||
        data.errors.some((entry) => typeof entry !== 'string')
      ) {
        throw new Error('Invalid diagnostics result');
      }
      setResult(stripInternalAnalyticsDiagnosticsFields(data));
      setNotice(DIAGNOSTICS_PANEL_COPY.runSuccess);
    } catch {
      if (isCurrent()) setError(DIAGNOSTICS_PANEL_COPY.runError);
    } finally {
      if (controller === request) {
        controller = undefined;
        setLoading(false);
      }
    }
  };

  const exportDiagnostics = async (sanitize: boolean) => {
    if (!isCurrentReader() || loading() || exportLoading()) return;
    setExportLoading(true);
    setError('');
    setNotice('');
    let url: string | undefined;
    let anchor: HTMLAnchorElement | undefined;
    try {
      const data = diagnosticsData();
      if (!data) {
        setError(DIAGNOSTICS_PANEL_COPY.exportRequiresResult);
        return;
      }

      const exportData = sanitize
        ? sanitizeDiagnosticsData(data)
        : stripInternalAnalyticsDiagnosticsFields(data);
      const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
      url = URL.createObjectURL(blob);
      anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = buildDiagnosticsExportFilename(sanitize);
      document.body.appendChild(anchor);
      anchor.click();
      if (isCurrentReader()) {
        setNotice(
          sanitize
            ? DIAGNOSTICS_PANEL_COPY.exportGithubSuccess
            : DIAGNOSTICS_PANEL_COPY.exportFullSuccess,
        );
      }
    } catch {
      if (isCurrentReader()) setError(DIAGNOSTICS_PANEL_COPY.exportError);
    } finally {
      anchor?.remove();
      if (url) URL.revokeObjectURL(url);
      setExportLoading(false);
    }
  };

  return {
    diagnosticsData,
    error: () => (isCurrentReader() ? error() : ''),
    exportDiagnostics,
    exportLoading,
    loading,
    notice: () => (isCurrentReader() ? notice() : ''),
    runDiagnostics,
  };
};
