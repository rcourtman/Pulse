import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import type { MonitoredSystemLedgerPreviewResponse } from '@/api/monitoredSystemLedger';
import { apiErrorCode, apiErrorDetailField, apiErrorStatus } from '@/api/responseUtils';
import {
  TrueNASAPI,
  isRedactedTrueNASSecret,
  type TrueNASConnection,
  type TrueNASConnectionInput,
} from '@/api/truenas';
import {
  buildMonitoredSystemImpactPreviewUnavailableState,
  type MonitoredSystemImpactPreviewUnavailableState,
} from '@/utils/monitoredSystemPresentation';

type TrueNASAuthMode = 'apiKey' | 'userpass';

export interface TrueNASConnectionFormState {
  name: string;
  host: string;
  port: string;
  pollIntervalSeconds: string;
  authMode: TrueNASAuthMode;
  apiKey: string;
  username: string;
  password: string;
  useHttps: boolean;
  insecureSkipVerify: boolean;
  fingerprint: string;
  enabled: boolean;
  monitorDatasets: boolean;
  monitorPools: boolean;
  monitorReplication: boolean;
  hasStoredApiKey: boolean;
  hasStoredPassword: boolean;
}

const REDACTED_SECRET = '********';

const createEmptyFormState = (): TrueNASConnectionFormState => ({
  name: '',
  host: '',
  port: '',
  pollIntervalSeconds: '60',
  authMode: 'apiKey',
  apiKey: '',
  username: '',
  password: '',
  useHttps: true,
  insecureSkipVerify: false,
  fingerprint: '',
  enabled: true,
  monitorDatasets: true,
  monitorPools: true,
  monitorReplication: true,
  hasStoredApiKey: false,
  hasStoredPassword: false,
});

const buildFormStateFromConnection = (
  connection: TrueNASConnection,
): TrueNASConnectionFormState => {
  const hasStoredAPIKey = isRedactedTrueNASSecret(connection.apiKey);
  const hasStoredPassword = isRedactedTrueNASSecret(connection.password);
  const authMode: TrueNASAuthMode =
    hasStoredAPIKey || Boolean((connection.apiKey || '').trim()) ? 'apiKey' : 'userpass';

  return {
    name: connection.name || '',
    host: connection.host || '',
    port: connection.port ? String(connection.port) : '',
    pollIntervalSeconds: String(connection.pollIntervalSeconds ?? 60),
    authMode,
    apiKey: '',
    username: connection.username || '',
    password: '',
    useHttps: connection.useHttps,
    insecureSkipVerify: connection.insecureSkipVerify,
    fingerprint: connection.fingerprint || '',
    enabled: connection.enabled,
    monitorDatasets: connection.monitorDatasets,
    monitorPools: connection.monitorPools,
    monitorReplication: connection.monitorReplication,
    hasStoredApiKey: hasStoredAPIKey,
    hasStoredPassword: hasStoredPassword,
  };
};

const parseOptionalPort = (value: string): number | undefined => {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  if (!Number.isInteger(parsed) || parsed < 1 || parsed > 65535) {
    throw new Error('Port must be a whole number between 1 and 65535');
  }
  return parsed;
};

const parsePollIntervalSeconds = (value: string): number => {
  const trimmed = value.trim();
  if (!trimmed) return 60;
  const parsed = Number(trimmed);
  if (!Number.isInteger(parsed) || parsed < 1 || parsed > 3600) {
    throw new Error('Poll interval must be a whole number between 1 and 3600 seconds');
  }
  return parsed;
};

const getErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  if (
    error &&
    typeof error === 'object' &&
    'message' in error &&
    typeof (error as { message?: unknown }).message === 'string'
  ) {
    const message = (error as { message: string }).message.trim();
    if (message) {
      return message;
    }
  }
  return fallback;
};

// Connection tests fail with a generic top-level message ("Failed to connect
// to TrueNAS") while the actionable cause — e.g. the exact x509 verification
// error — rides in details.error. Prefer it so users can tell an untrusted CA
// from a hostname mismatch instead of reaching for Skip TLS Verification
// (#1602). Mirrors getVMwareErrorMessage.
const getTrueNASErrorMessage = (error: unknown, fallback: string): string =>
  apiErrorDetailField(error, 'error') ?? getErrorMessage(error, fallback);

const monitoredSystemImpactPreviewUnavailableStateFromError = (error: unknown) => {
  return buildMonitoredSystemImpactPreviewUnavailableState({
    code: apiErrorCode(error),
    reason: apiErrorDetailField(error, 'reason'),
  });
};

const buildConnectionInput = (form: TrueNASConnectionFormState): TrueNASConnectionInput => {
  const port = parseOptionalPort(form.port);
  const pollIntervalSeconds = parsePollIntervalSeconds(form.pollIntervalSeconds);
  const name = form.name.trim();
  const host = form.host.trim();
  const fingerprint = form.fingerprint.trim();
  const username = form.username.trim();

  if (form.authMode === 'apiKey' && !username) {
    throw new Error('TrueNAS API key owner username is required for supported authentication');
  }

  const input: TrueNASConnectionInput = {
    ...(name ? { name } : {}),
    host,
    ...(port !== undefined ? { port } : {}),
    pollIntervalSeconds,
    useHttps: form.useHttps,
    insecureSkipVerify: form.insecureSkipVerify,
    // Empty is meaningful when editing: it explicitly clears a previously
    // pinned certificate fingerprint. Omitting the field makes the backend's
    // merge preserve the stored value (#1631).
    fingerprint,
    enabled: form.enabled,
    monitorDatasets: form.monitorDatasets,
    monitorPools: form.monitorPools,
    monitorReplication: form.monitorReplication,
  };

  if (form.authMode === 'apiKey') {
    input.apiKey = form.apiKey.trim() || (form.hasStoredApiKey ? REDACTED_SECRET : '');
    input.username = username;
    input.password = '';
  } else {
    input.apiKey = '';
    input.username = username;
    input.password = form.password.trim() || (form.hasStoredPassword ? REDACTED_SECRET : '');
  }

  return input;
};

// `canLoad` gates the reader lifetime: /api/truenas/connections is RequireAdmin and
// this hook is constructed for every settings tab, so a non-admin session would
// otherwise 403 on any settings page. Defaults to enabled for callers (tests,
// stories) that have no capability to hand it.
export function useTrueNASSettingsPanelState({
  canLoad,
  currentUser,
}: {
  canLoad?: () => boolean;
  currentUser?: () => string | undefined;
} = {}) {
  const [connections, setConnections] = createSignal<TrueNASConnection[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [loadingError, setLoadingError] = createSignal<string | null>(null);
  const [featureDisabled, setFeatureDisabled] = createSignal(false);
  const [featureDisabledMessage, setFeatureDisabledMessage] = createSignal<string>('');
  const [dialogOpen, setDialogOpen] = createSignal(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = createSignal(false);
  const [editingConnectionId, setEditingConnectionId] = createSignal<string | null>(null);
  const [pendingDeleteConnection, setPendingDeleteConnection] =
    createSignal<TrueNASConnection | null>(null);
  const [form, setForm] = createSignal<TrueNASConnectionFormState>(createEmptyFormState());
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [deleting, setDeleting] = createSignal(false);
  const [previewing, setPreviewing] = createSignal(false);
  const [monitoredSystemPreview, setMonitoredSystemPreview] =
    createSignal<MonitoredSystemLedgerPreviewResponse | null>(null);
  const [monitoredSystemPreviewUnavailableState, setMonitoredSystemPreviewUnavailableState] =
    createSignal<MonitoredSystemImpactPreviewUnavailableState | null>(null);
  const [monitoredSystemPreviewGenericError, setMonitoredSystemPreviewGenericError] = createSignal<
    string | null
  >(null);
  const monitoredSystemPreviewErrorTitle = createMemo(
    () => monitoredSystemPreviewUnavailableState()?.title ?? null,
  );
  const monitoredSystemPreviewError = createMemo(
    () => monitoredSystemPreviewUnavailableState()?.message ?? monitoredSystemPreviewGenericError(),
  );
  const editingConnection = createMemo(
    () => connections().find((connection) => connection.id === editingConnectionId()) ?? null,
  );

  const accessAllowed = () => canLoad?.() ?? true;
  const currentOrg = () => getOrgID() || 'default';
  const snapshot = () => ({
    org: currentOrg(),
    user: currentUser?.(),
    allowed: accessAllowed(),
    controller: new AbortController(),
  });
  const [reader, setReader] = createSignal(snapshot());
  const [accessRejected, setAccessRejected] = createSignal(false);
  let disposed = false;
  let readController: AbortController | undefined;
  let dialogVersion = 0;
  const isCurrentReader = (accepted = reader()) =>
    !disposed &&
    accepted === reader() &&
    !accepted.controller.signal.aborted &&
    accepted.org === currentOrg() &&
    accepted.user === currentUser?.() &&
    accepted.allowed === accessAllowed();
  const requestOptions = (accepted = reader(), signal = accepted.controller.signal) => ({
    expectedOrgID: accepted.org,
    signal,
    retry: false,
  });
  const busy = () => saving() || testing() || deleting() || previewing();
  const isCurrentConnection = (connection: TrueNASConnection) =>
    connections().some((current) => unwrap(current) === unwrap(connection));

  const clearPreview = () => {
    setMonitoredSystemPreview(null);
    setMonitoredSystemPreviewUnavailableState(null);
    setMonitoredSystemPreviewGenericError(null);
  };
  const resetDialogState = () => {
    dialogVersion += 1;
    setDialogOpen(false);
    setEditingConnectionId(null);
    setForm(createEmptyFormState());
    clearPreview();
  };
  const resetDeleteDialogState = () => {
    setDeleteDialogOpen(false);
    setPendingDeleteConnection(null);
  };
  const resetReader = (reload = true) => {
    reader().controller.abort();
    readController?.abort();
    readController = undefined;
    batch(() => {
      // Object identity prevents a late completion being accepted on A -> B -> A.
      setReader(snapshot());
      resetDialogState();
      resetDeleteDialogState();
      setConnections([]);
      setLoadingError(null);
      setFeatureDisabled(false);
      setFeatureDisabledMessage('');
      setAccessRejected(false);
      setSaving(false);
      setTesting(false);
      setDeleting(false);
      setPreviewing(false);
      setLoading(false);
    });
    if (!disposed && reload) void loadConnections();
  };
  const ensureReader = () => {
    if (isCurrentReader()) return true;
    // A storage change without an org event also withdraws before any action.
    if (!disposed) resetReader();
    return false;
  };
  const canAct = () =>
    ensureReader() && accessAllowed() && !accessRejected() && !featureDisabled() && !busy();
  const withdrawOnDenial = (error: unknown) => {
    if (![401, 403].includes(apiErrorStatus(error) ?? 0)) return false;
    resetReader(false);
    setAccessRejected(true);
    setLoadingError('TrueNAS settings access could not be verified. Reload before making changes.');
    return true;
  };

  const loadConnections = async () => {
    if (!ensureReader() || !accessAllowed() || readController || busy()) return;
    const accepted = reader();
    const controller = new AbortController();
    readController = controller;
    const isCurrent = () => isCurrentReader(accepted) && readController === controller;
    batch(() => {
      setLoading(true);
      setLoadingError(null);
    });
    try {
      const nextConnections = await TrueNASAPI.listConnections(
        requestOptions(accepted, controller.signal),
      );
      if (!isCurrent()) return;
      batch(() => {
        setConnections(nextConnections);
        setAccessRejected(false);
        setFeatureDisabled(false);
        setFeatureDisabledMessage('');
      });
    } catch (error) {
      if (!isCurrent()) return;
      if (withdrawOnDenial(error)) return;
      setConnections([]);
      if (apiErrorStatus(error) === 404) {
        setFeatureDisabled(true);
        setFeatureDisabledMessage(
          getErrorMessage(error, 'TrueNAS integration has been explicitly disabled'),
        );
        return;
      }
      setLoadingError(getErrorMessage(error, 'Failed to load TrueNAS connections'));
      logger.error('[TrueNAS Settings] Failed to load connections', error);
    } finally {
      if (isCurrent()) {
        readController = undefined;
        setLoading(false);
      }
    }
  };

  createEffect(
    on([accessAllowed, () => currentUser?.()], (current, previous) => {
      // Copied security status objects must not erase a same-reader draft.
      if (!previous || current.some((value, index) => value !== previous[index])) resetReader();
    }),
  );
  const unsubscribeOrg = eventBus.on('org_switched', () => {
    if (!isCurrentReader()) resetReader();
  });
  onCleanup(() => {
    disposed = true;
    unsubscribeOrg();
    resetReader(false);
  });

  const openCreateDialog = () => {
    if (!canAct()) return;
    batch(() => {
      resetDialogState();
      resetDeleteDialogState();
      setDialogOpen(true);
    });
  };
  const openEditDialog = (connection: TrueNASConnection) => {
    if (!canAct() || !isCurrentConnection(connection)) return;
    batch(() => {
      resetDialogState();
      resetDeleteDialogState();
      setEditingConnectionId(connection.id);
      setForm(buildFormStateFromConnection(connection));
      setDialogOpen(true);
    });
  };
  const closeDialog = () => {
    if (saving() || testing()) return;
    batch(resetDialogState);
  };
  const openDeleteDialog = (connection: TrueNASConnection) => {
    if (!canAct() || !isCurrentConnection(connection)) return;
    batch(() => {
      resetDialogState();
      setPendingDeleteConnection(connection);
      setDeleteDialogOpen(true);
    });
  };
  const closeDeleteDialog = () => {
    if (!deleting()) resetDeleteDialogState();
  };
  const updateForm = (patch: Partial<TrueNASConnectionFormState>) => {
    if (!ensureReader() || !accessAllowed() || accessRejected() || !dialogOpen() || saving())
      return;
    dialogVersion += 1;
    batch(() => {
      clearPreview();
      setForm((current) => ({ ...current, ...patch }));
    });
  };
  const handleActionError = (error: unknown, fallback: string, log: string) => {
    if (withdrawOnDenial(error)) {
      notificationStore.error(loadingError()!);
      return;
    }
    notificationStore.error(getTrueNASErrorMessage(error, fallback));
    logger.error(log, error);
  };

  const testCurrentForm = async () => {
    if (!canAct() || !dialogOpen()) return false;
    const accepted = reader();
    const version = dialogVersion;
    const isCurrent = () => isCurrentReader(accepted) && version === dialogVersion;
    setTesting(true);
    try {
      const payload = buildConnectionInput(form());
      if (editingConnectionId()) {
        await TrueNASAPI.testSavedConnection(
          editingConnectionId()!,
          payload,
          requestOptions(accepted),
        );
      } else {
        await TrueNASAPI.testConnection(payload, requestOptions(accepted));
      }
      if (!isCurrent()) return false;
      notificationStore.success('TrueNAS connection successful');
      return true;
    } catch (error) {
      // Access belongs to the reader, not the tested draft. A Name/credential
      // edit may obsolete a result, but must not hide this reader's denial.
      if (!isCurrentReader(accepted)) return false;
      if (withdrawOnDenial(error)) notificationStore.error(loadingError()!);
      else if (isCurrent())
        handleActionError(
          error,
          'TrueNAS connection failed',
          '[TrueNAS Settings] Connection test failed',
        );
      return false;
    } finally {
      if (isCurrentReader(accepted)) setTesting(false);
    }
  };

  const testSavedConnection = async (connection: TrueNASConnection) => {
    if (!canAct() || !isCurrentConnection(connection)) return;
    const accepted = reader();
    setTesting(true);
    try {
      await TrueNASAPI.testSavedConnection(connection.id, undefined, requestOptions(accepted));
      if (!isCurrentReader(accepted)) return;
      notificationStore.success(
        `TrueNAS connection successful for ${connection.name || connection.host}`,
      );
    } catch (error) {
      if (isCurrentReader(accepted))
        handleActionError(
          error,
          'TrueNAS connection failed',
          '[TrueNAS Settings] Saved connection test failed',
        );
    } finally {
      if (isCurrentReader(accepted)) {
        setTesting(false);
        await loadConnections();
      }
    }
  };

  const previewCurrentForm = async () => {
    if (!canAct() || !dialogOpen()) return null;
    const accepted = reader();
    const version = dialogVersion;
    const isCurrent = () => isCurrentReader(accepted) && version === dialogVersion;
    batch(() => {
      setPreviewing(true);
      clearPreview();
    });
    try {
      const payload = buildConnectionInput(form());
      const preview = editingConnectionId()
        ? await TrueNASAPI.previewSavedConnection(
            editingConnectionId()!,
            payload,
            requestOptions(accepted),
          )
        : await TrueNASAPI.previewConnection(payload, requestOptions(accepted));
      if (!isCurrent()) return null;
      setMonitoredSystemPreview(preview);
      return preview;
    } catch (error) {
      // An obsolete reader cannot withdraw its replacement; an obsolete draft
      // on the same reader still must withdraw on lost access.
      if (!isCurrentReader(accepted)) return null;
      if (withdrawOnDenial(error)) {
        notificationStore.error(loadingError()!);
        return null;
      }
      if (!isCurrent()) return null;
      const unavailableState = monitoredSystemImpactPreviewUnavailableStateFromError(error);
      if (unavailableState) setMonitoredSystemPreviewUnavailableState(unavailableState);
      else
        setMonitoredSystemPreviewGenericError(
          getTrueNASErrorMessage(error, 'Could not preview monitored-system impact'),
        );
      logger.error('[TrueNAS Settings] Preview failed', error);
      return null;
    } finally {
      if (isCurrentReader(accepted)) setPreviewing(false);
    }
  };

  const saveCurrentForm = async () => {
    if (!canAct() || !dialogOpen()) return;
    const accepted = reader();
    setSaving(true);
    try {
      const payload = buildConnectionInput(form());
      if (editingConnectionId()) {
        await TrueNASAPI.updateConnection(
          editingConnectionId()!,
          payload,
          requestOptions(accepted),
        );
      } else {
        await TrueNASAPI.createConnection(payload, requestOptions(accepted));
      }
      if (!isCurrentReader(accepted)) return;
      notificationStore.success(
        editingConnectionId() ? 'TrueNAS connection updated' : 'TrueNAS connection added',
      );
      batch(resetDialogState);
      setSaving(false);
      await loadConnections();
    } catch (error) {
      if (!isCurrentReader(accepted)) return;
      setMonitoredSystemPreviewUnavailableState(null);
      handleActionError(
        error,
        'Failed to save TrueNAS connection',
        '[TrueNAS Settings] Save failed',
      );
    } finally {
      if (isCurrentReader(accepted)) setSaving(false);
    }
  };

  const deletePendingConnection = async () => {
    if (!canAct()) return;
    const connection = pendingDeleteConnection();
    if (!connection || !isCurrentConnection(connection)) return;
    const accepted = reader();
    setDeleting(true);
    try {
      await TrueNASAPI.deleteConnection(connection.id, requestOptions(accepted));
      if (!isCurrentReader(accepted)) return;
      notificationStore.success(`Removed ${connection.name || connection.host}`);
      resetDeleteDialogState();
      setDeleting(false);
      await loadConnections();
    } catch (error) {
      if (isCurrentReader(accepted))
        handleActionError(
          error,
          'Failed to remove TrueNAS connection',
          '[TrueNAS Settings] Delete failed',
        );
    } finally {
      if (isCurrentReader(accepted)) setDeleting(false);
    }
  };

  return {
    closeDeleteDialog,
    closeDialog,
    connections,
    deleteDialogOpen,
    deletePendingConnection,
    deleting,
    dialogOpen,
    editingConnection,
    featureDisabled,
    featureDisabledMessage,
    form,
    loadConnections,
    loading,
    loadingError,
    openCreateDialog,
    openDeleteDialog,
    openEditDialog,
    pendingDeleteConnection,
    previewCurrentForm,
    monitoredSystemPreview,
    monitoredSystemPreviewError,
    monitoredSystemPreviewErrorTitle,
    previewing,
    saveCurrentForm,
    saving,
    testCurrentForm,
    testSavedConnection,
    testing,
    updateForm,
  };
}

export type TrueNASSettingsPanelState = ReturnType<typeof useTrueNASSettingsPanelState>;

export default useTrueNASSettingsPanelState;
