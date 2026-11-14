import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import type { MonitoredSystemLedgerPreviewResponse } from '@/api/monitoredSystemLedger';
import {
  VMwareAPI,
  isRedactedVMwareSecret,
  type VMwareConnection,
  type VMwareConnectionInput,
  type VMwareConnectionTestResult,
} from '@/api/vmware';
import { apiErrorCode, apiErrorDetailField, apiErrorStatus } from '@/api/responseUtils';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import {
  buildMonitoredSystemImpactPreviewUnavailableState,
  type MonitoredSystemImpactPreviewUnavailableState,
} from '@/utils/monitoredSystemPresentation';
import {
  buildVMwareConnectionFailurePresentation,
  type VMwareConnectionFailurePresentation,
} from './vmwareConnectionFailurePresentation';

export interface VMwareConnectionFormState {
  name: string;
  host: string;
  port: string;
  username: string;
  password: string;
  insecureSkipVerify: boolean;
  enabled: boolean;
  monitorVms: boolean;
  monitorHosts: boolean;
  monitorDatastores: boolean;
  hasStoredPassword: boolean;
}

export interface VMwareConnectionTestWarning {
  title: string;
  message: string;
  guidance: string;
}

const REDACTED_SECRET = '********';

const createEmptyFormState = (): VMwareConnectionFormState => ({
  name: '',
  host: '',
  port: '443',
  username: '',
  password: '',
  insecureSkipVerify: false,
  enabled: true,
  monitorVms: true,
  monitorHosts: true,
  monitorDatastores: true,
  hasStoredPassword: false,
});

const buildFormStateFromConnection = (connection: VMwareConnection): VMwareConnectionFormState => ({
  name: connection.name || '',
  host: connection.host || '',
  port: connection.port ? String(connection.port) : '443',
  username: connection.username || '',
  password: '',
  insecureSkipVerify: connection.insecureSkipVerify,
  enabled: connection.enabled,
  monitorVms: connection.monitorVms,
  monitorHosts: connection.monitorHosts,
  monitorDatastores: connection.monitorDatastores,
  hasStoredPassword: isRedactedVMwareSecret(connection.password),
});

const parseOptionalPort = (value: string): number | undefined => {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  if (!Number.isInteger(parsed) || parsed < 1 || parsed > 65535) {
    throw new Error('Port must be a whole number between 1 and 65535');
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

const getVMwareErrorMessage = (error: unknown, fallback: string): string =>
  apiErrorDetailField(error, 'error') ?? getErrorMessage(error, fallback);

const isDestinationConfigurationError = (error: unknown) =>
  apiErrorCode(error) === 'destination_not_allowed' ||
  apiErrorCode(error) === 'vmware_invalid_config' ||
  apiErrorDetailField(error, 'category') === 'invalid_config';

const buildConnectionTestWarning = (
  result: VMwareConnectionTestResult,
): VMwareConnectionTestWarning | null => {
  if (!result.degraded) return null;
  const diagnostic =
    result.issues.find((issue) => issue.message?.trim())?.message?.trim() ||
    `${result.issueCount || result.issues.length || 1} optional VMware read${
      (result.issueCount || result.issues.length || 1) === 1 ? '' : 's'
    } could not be completed.`;
  const release = result.viRelease?.trim();
  return {
    title: 'VMware connection has limited data access',
    message: diagnostic,
    guidance: `Core inventory, authentication, and API compatibility checks succeeded${
      release ? ` using VI JSON ${release}` : ''
    }. Pulse will keep monitoring and report affected optional data as degraded.`,
  };
};

const monitoredSystemImpactPreviewUnavailableStateFromError = (error: unknown) => {
  return buildMonitoredSystemImpactPreviewUnavailableState({
    code: apiErrorCode(error),
    reason: apiErrorDetailField(error, 'reason'),
  });
};

const buildConnectionInput = (form: VMwareConnectionFormState): VMwareConnectionInput => {
  const port = parseOptionalPort(form.port);
  const name = form.name.trim();
  const host = form.host.trim();
  const username = form.username.trim();

  return {
    ...(name ? { name } : {}),
    host,
    ...(port !== undefined ? { port } : {}),
    username,
    password: form.password.trim() || (form.hasStoredPassword ? REDACTED_SECRET : ''),
    insecureSkipVerify: form.insecureSkipVerify,
    enabled: form.enabled,
    monitorVms: form.monitorVms,
    monitorHosts: form.monitorHosts,
    monitorDatastores: form.monitorDatastores,
  };
};

// Settings constructs this hook for every tab. Keep both inventory and actions
// with the org/user/capability reader that admitted them, not just its first load.
export function useVMwareSettingsPanelState({
  canLoad,
  currentUser,
}: {
  canLoad?: () => boolean;
  currentUser?: () => string | undefined;
} = {}) {
  const [connections, setConnections] = createSignal<VMwareConnection[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [loadingError, setLoadingError] = createSignal<string | null>(null);
  const [featureDisabled, setFeatureDisabled] = createSignal(false);
  const [featureDisabledMessage, setFeatureDisabledMessage] = createSignal('');
  const [dialogOpen, setDialogOpen] = createSignal(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = createSignal(false);
  const [editingConnectionId, setEditingConnectionId] = createSignal<string | null>(null);
  const [pendingDeleteConnection, setPendingDeleteConnection] =
    createSignal<VMwareConnection | null>(null);
  const [form, setForm] = createSignal<VMwareConnectionFormState>(createEmptyFormState());
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [deleting, setDeleting] = createSignal(false);
  const [previewing, setPreviewing] = createSignal(false);
  const [connectionFailure, setConnectionFailure] =
    createSignal<VMwareConnectionFailurePresentation | null>(null);
  const [connectionTestWarning, setConnectionTestWarning] =
    createSignal<VMwareConnectionTestWarning | null>(null);
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
  const isCurrentConnection = (connection: VMwareConnection) =>
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
    setConnectionFailure(null);
    setConnectionTestWarning(null);
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
      // A returned user/org name does not revive an earlier reader's promises.
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
    if (!disposed) resetReader();
    return false;
  };
  const canAct = () =>
    ensureReader() && accessAllowed() && !accessRejected() && !featureDisabled() && !busy();
  const withdrawOnDenial = (error: unknown) => {
    if (![401, 403].includes(apiErrorStatus(error) ?? 0)) return false;
    resetReader(false);
    setAccessRejected(true);
    setLoadingError('VMware settings access could not be verified. Reload before making changes.');
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
      const nextConnections = await VMwareAPI.listConnections(
        requestOptions(accepted, controller.signal),
      );
      if (!isCurrent()) return;
      batch(() => {
        setConnections(nextConnections);
        if (
          editingConnectionId() &&
          !nextConnections.some((connection) => connection.id === editingConnectionId())
        )
          resetDialogState();
        if (
          pendingDeleteConnection() &&
          !nextConnections.some((connection) => connection.id === pendingDeleteConnection()?.id)
        )
          resetDeleteDialogState();
        setAccessRejected(false);
        setFeatureDisabled(false);
        setFeatureDisabledMessage('');
      });
    } catch (error) {
      if (!isCurrent()) return;
      if (withdrawOnDenial(error)) return;
      batch(() => {
        setConnections([]);
        resetDialogState();
        resetDeleteDialogState();
      });
      if (apiErrorStatus(error) === 404) {
        setFeatureDisabled(true);
        setFeatureDisabledMessage(
          getErrorMessage(error, 'VMware integration has been explicitly disabled'),
        );
        return;
      }
      setLoadingError(getErrorMessage(error, 'Failed to load VMware connections'));
      logger.error('[VMware Settings] Failed to load connections', error);
    } finally {
      if (isCurrent()) {
        readController = undefined;
        setLoading(false);
      }
    }
  };

  createEffect(
    on([accessAllowed, () => currentUser?.()], (current, previous) => {
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
  const openEditDialog = (connection: VMwareConnection) => {
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
  const openDeleteDialog = (connection: VMwareConnection) => {
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
  const updateForm = (patch: Partial<VMwareConnectionFormState>) => {
    if (!ensureReader() || !accessAllowed() || accessRejected() || !dialogOpen() || saving())
      return;
    dialogVersion += 1;
    batch(() => {
      setConnectionFailure(null);
      setConnectionTestWarning(null);
      clearPreview();
      setForm((current) => ({ ...current, ...patch }));
    });
  };
  const notifyDenial = (error: unknown, connectionAttempt = false) => {
    // This exact refusal describes the requested destination, not loss of
    // settings access. Only connection attempts may retain their admitted
    // reader/draft; inventory, deletion and every other 401/403 still withdraw.
    if (
      connectionAttempt &&
      apiErrorStatus(error) === 403 &&
      apiErrorCode(error) === 'destination_not_allowed'
    )
      return false;
    if (!withdrawOnDenial(error)) return false;
    notificationStore.error(loadingError()!);
    return true;
  };

  const testCurrentForm = async () => {
    if (!canAct() || !dialogOpen()) return false;
    const accepted = reader();
    const version = dialogVersion;
    const isCurrent = () => isCurrentReader(accepted) && version === dialogVersion;
    batch(() => {
      setTesting(true);
      setConnectionFailure(null);
      setConnectionTestWarning(null);
    });
    try {
      const payload = buildConnectionInput(form());
      const result = editingConnectionId()
        ? await VMwareAPI.testSavedConnection(
            editingConnectionId()!,
            payload,
            requestOptions(accepted),
          )
        : await VMwareAPI.testConnection(payload, requestOptions(accepted));
      if (!isCurrent()) return false;
      const warning = buildConnectionTestWarning(result);
      setConnectionTestWarning(warning);
      if (warning) notificationStore.warning(warning.message);
      else notificationStore.success('VMware connection successful');
      return true;
    } catch (error) {
      if (!isCurrentReader(accepted)) return false;
      // A changed draft obsoletes its test result, but not this reader's denial.
      if (notifyDenial(error, true) || !isCurrent()) return false;
      const failure = buildVMwareConnectionFailurePresentation({
        code: apiErrorCode(error),
        category: apiErrorDetailField(error, 'category'),
        message: getVMwareErrorMessage(error, 'VMware connection failed'),
        fallback: 'VMware connection failed',
      });
      setConnectionFailure(failure);
      // Destination feedback is already beside this correctable draft. A
      // duplicate toast can cover its actions, especially on a phone.
      if (!isDestinationConfigurationError(error)) notificationStore.error(failure.message);
      logger.error('[VMware Settings] Connection test failed', error);
      return false;
    } finally {
      if (isCurrentReader(accepted)) setTesting(false);
    }
  };

  const testSavedConnection = async (connection: VMwareConnection) => {
    if (!canAct() || !isCurrentConnection(connection)) return;
    const accepted = reader();
    setTesting(true);
    try {
      const result = await VMwareAPI.testSavedConnection(
        connection.id,
        undefined,
        requestOptions(accepted),
      );
      if (!isCurrentReader(accepted)) return;
      const warning = buildConnectionTestWarning(result);
      if (warning) {
        notificationStore.warning(
          `VMware connection for ${connection.name || connection.host} has limited data access: ${warning.message}`,
        );
      } else {
        notificationStore.success(
          `VMware connection successful for ${connection.name || connection.host}`,
        );
      }
    } catch (error) {
      if (!isCurrentReader(accepted) || notifyDenial(error, true)) return;
      const failure = buildVMwareConnectionFailurePresentation({
        code: apiErrorCode(error),
        category: apiErrorDetailField(error, 'category'),
        message: getVMwareErrorMessage(error, 'VMware connection failed'),
        fallback: 'VMware connection failed',
        defaultGuidance:
          'Retry the saved connection test after reviewing the VMware runtime status for this vCenter.',
        defaultTitle: 'Saved connection test failed',
      });
      notificationStore.error(failure.message);
      logger.error('[VMware Settings] Saved connection test failed', error);
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
      setConnectionFailure(null);
      clearPreview();
    });
    try {
      const payload = buildConnectionInput(form());
      const preview = editingConnectionId()
        ? await VMwareAPI.previewSavedConnection(
            editingConnectionId()!,
            payload,
            requestOptions(accepted),
          )
        : await VMwareAPI.previewConnection(payload, requestOptions(accepted));
      if (!isCurrent()) return null;
      setMonitoredSystemPreview(preview);
      return preview;
    } catch (error) {
      if (!isCurrentReader(accepted)) return null;
      if (notifyDenial(error, true) || !isCurrent()) return null;
      const unavailableState = monitoredSystemImpactPreviewUnavailableStateFromError(error);
      if (unavailableState) {
        setMonitoredSystemPreviewUnavailableState(unavailableState);
        logger.error('[VMware Settings] Preview failed', error);
        return null;
      }
      const failure = buildVMwareConnectionFailurePresentation({
        code: apiErrorCode(error),
        category: apiErrorDetailField(error, 'category'),
        message: getVMwareErrorMessage(error, 'Could not preview monitored-system impact'),
        fallback: 'Could not preview monitored-system impact',
      });
      setMonitoredSystemPreviewGenericError(failure.message);
      if (
        apiErrorCode(error) === 'vmware_connection_failed' ||
        isDestinationConfigurationError(error)
      )
        setConnectionFailure(failure);
      logger.error('[VMware Settings] Preview failed', error);
      return null;
    } finally {
      if (isCurrentReader(accepted)) setPreviewing(false);
    }
  };

  const saveCurrentForm = async () => {
    if (!canAct() || !dialogOpen()) return;
    const accepted = reader();
    setSaving(true);
    setConnectionFailure(null);
    try {
      const payload = buildConnectionInput(form());
      const editingID = editingConnectionId();
      if (editingID) await VMwareAPI.updateConnection(editingID, payload, requestOptions(accepted));
      else await VMwareAPI.createConnection(payload, requestOptions(accepted));
      if (!isCurrentReader(accepted)) return;
      notificationStore.success(
        editingID ? 'VMware connection updated' : 'VMware connection added',
      );
      batch(resetDialogState);
      setSaving(false);
      await loadConnections();
    } catch (error) {
      if (!isCurrentReader(accepted) || notifyDenial(error, true)) return;
      setMonitoredSystemPreviewUnavailableState(null);
      if (isDestinationConfigurationError(error)) {
        setConnectionFailure(
          buildVMwareConnectionFailurePresentation({
            code: apiErrorCode(error),
            category: apiErrorDetailField(error, 'category'),
            message: getVMwareErrorMessage(error, 'Failed to save VMware connection'),
            fallback: 'Failed to save VMware connection',
          }),
        );
      } else notificationStore.error(getErrorMessage(error, 'Failed to save VMware connection'));
      logger.error('[VMware Settings] Save failed', error);
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
      await VMwareAPI.deleteConnection(connection.id, requestOptions(accepted));
      if (!isCurrentReader(accepted)) return;
      notificationStore.success(`Removed ${connection.name || connection.host}`);
      resetDeleteDialogState();
      setDeleting(false);
      await loadConnections();
    } catch (error) {
      if (!isCurrentReader(accepted) || notifyDenial(error)) return;
      notificationStore.error(getErrorMessage(error, 'Failed to remove VMware connection'));
      logger.error('[VMware Settings] Delete failed', error);
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
    connectionFailure,
    connectionTestWarning,
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

export type VMwareSettingsPanelState = ReturnType<typeof useVMwareSettingsPanelState>;

export default useVMwareSettingsPanelState;
