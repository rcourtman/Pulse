import { createMemo, createSignal, onMount } from 'solid-js';
import type { MonitoredSystemLedgerPreviewResponse } from '@/api/monitoredSystemLedger';
import {
  VMwareAPI,
  isRedactedVMwareSecret,
  type VMwareConnection,
  type VMwareConnectionInput,
} from '@/api/vmware';
import {
  apiErrorCode,
  apiErrorDetailField,
  apiErrorMonitoredSystemPreview,
  apiErrorStatus,
} from '@/api/responseUtils';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
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
  hasStoredPassword: boolean;
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
  };
};

export function useVMwareSettingsPanelState() {
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
  const [monitoredSystemPreview, setMonitoredSystemPreview] =
    createSignal<MonitoredSystemLedgerPreviewResponse | null>(null);
  const [monitoredSystemPreviewError, setMonitoredSystemPreviewError] = createSignal<string | null>(
    null,
  );

  const editingConnection = createMemo(
    () => connections().find((connection) => connection.id === editingConnectionId()) ?? null,
  );

  const loadConnections = async () => {
    setLoading(true);
    setLoadingError(null);
    try {
      const nextConnections = await VMwareAPI.listConnections();
      setConnections(nextConnections);
      setFeatureDisabled(false);
      setFeatureDisabledMessage('');
    } catch (error) {
      if (apiErrorStatus(error) === 404) {
        setFeatureDisabled(true);
        setFeatureDisabledMessage(
          getErrorMessage(error, 'VMware integration has been explicitly disabled'),
        );
        setConnections([]);
        return;
      }
      const message = getErrorMessage(error, 'Failed to load VMware connections');
      setLoadingError(message);
      logger.error('[VMware Settings] Failed to load connections', error);
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    void loadConnections();
  });

  const openCreateDialog = () => {
    setEditingConnectionId(null);
    setForm(createEmptyFormState());
    setConnectionFailure(null);
    setMonitoredSystemPreview(null);
    setMonitoredSystemPreviewError(null);
    setDialogOpen(true);
  };

  const openEditDialog = (connection: VMwareConnection) => {
    setEditingConnectionId(connection.id);
    setForm(buildFormStateFromConnection(connection));
    setConnectionFailure(null);
    setMonitoredSystemPreview(null);
    setMonitoredSystemPreviewError(null);
    setDialogOpen(true);
  };

  const resetDialogState = () => {
    setDialogOpen(false);
    setEditingConnectionId(null);
    setForm(createEmptyFormState());
    setConnectionFailure(null);
    setMonitoredSystemPreview(null);
    setMonitoredSystemPreviewError(null);
  };

  const closeDialog = () => {
    if (saving() || testing()) return;
    resetDialogState();
  };

  const openDeleteDialog = (connection: VMwareConnection) => {
    setPendingDeleteConnection(connection);
    setDeleteDialogOpen(true);
  };

  const resetDeleteDialogState = () => {
    setDeleteDialogOpen(false);
    setPendingDeleteConnection(null);
  };

  const closeDeleteDialog = () => {
    if (deleting()) return;
    resetDeleteDialogState();
  };

  const updateForm = (patch: Partial<VMwareConnectionFormState>) => {
    setConnectionFailure(null);
    setMonitoredSystemPreview(null);
    setMonitoredSystemPreviewError(null);
    setForm((current) => ({ ...current, ...patch }));
  };

  const testCurrentForm = async () => {
    setTesting(true);
    setConnectionFailure(null);
    try {
      const payload = buildConnectionInput(form());
      if (editingConnectionId()) {
        await VMwareAPI.testSavedConnection(editingConnectionId()!, payload);
      } else {
        await VMwareAPI.testConnection(payload);
      }
      notificationStore.success('VMware connection successful');
      return true;
    } catch (error) {
      const failure = buildVMwareConnectionFailurePresentation({
        code: apiErrorCode(error),
        category: apiErrorDetailField(error, 'category'),
        message: getVMwareErrorMessage(error, 'VMware connection failed'),
        fallback: 'VMware connection failed',
      });
      setConnectionFailure(failure);
      notificationStore.error(failure.message);
      logger.error('[VMware Settings] Connection test failed', error);
      return false;
    } finally {
      setTesting(false);
    }
  };

  const testSavedConnection = async (connection: VMwareConnection) => {
    setTesting(true);
    try {
      await VMwareAPI.testSavedConnection(connection.id);
      notificationStore.success(
        `VMware connection successful for ${connection.name || connection.host}`,
      );
    } catch (error) {
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
      setTesting(false);
      await loadConnections();
    }
  };

  const previewCurrentForm = async () => {
    setPreviewing(true);
    setConnectionFailure(null);
    setMonitoredSystemPreviewError(null);
    try {
      const payload = buildConnectionInput(form());
      const preview = editingConnectionId()
        ? await VMwareAPI.previewSavedConnection(editingConnectionId()!, payload)
        : await VMwareAPI.previewConnection(payload);
      setMonitoredSystemPreview(preview);
      return preview;
    } catch (error) {
      const failure = buildVMwareConnectionFailurePresentation({
        code: apiErrorCode(error),
        category: apiErrorDetailField(error, 'category'),
        message: getVMwareErrorMessage(error, 'Could not preview monitored-system impact'),
        fallback: 'Could not preview monitored-system impact',
      });
      setMonitoredSystemPreview(null);
      setMonitoredSystemPreviewError(failure.message);
      if (
        apiErrorCode(error) === 'vmware_connection_failed' ||
        apiErrorCode(error) === 'vmware_invalid_config'
      ) {
        setConnectionFailure(failure);
      }
      logger.error('[VMware Settings] Preview failed', error);
      return null;
    } finally {
      setPreviewing(false);
    }
  };

  const saveCurrentForm = async () => {
    if (monitoredSystemPreview()?.would_exceed_limit) {
      notificationStore.error('This change would exceed your monitored-system limit');
      return;
    }
    setSaving(true);
    try {
      const payload = buildConnectionInput(form());
      if (editingConnectionId()) {
        await VMwareAPI.updateConnection(editingConnectionId()!, payload);
        notificationStore.success('VMware connection updated');
      } else {
        await VMwareAPI.createConnection(payload);
        notificationStore.success('VMware connection added');
      }
      resetDialogState();
      await loadConnections();
    } catch (error) {
      const preview = apiErrorMonitoredSystemPreview(error);
      if (preview) {
        setMonitoredSystemPreview(preview);
        setMonitoredSystemPreviewError(null);
      }
      const message = getErrorMessage(error, 'Failed to save VMware connection');
      notificationStore.error(message);
      logger.error('[VMware Settings] Save failed', error);
    } finally {
      setSaving(false);
    }
  };

  const deletePendingConnection = async () => {
    const connection = pendingDeleteConnection();
    if (!connection) return;
    setDeleting(true);
    try {
      await VMwareAPI.deleteConnection(connection.id);
      notificationStore.success(`Removed ${connection.name || connection.host}`);
      resetDeleteDialogState();
      await loadConnections();
    } catch (error) {
      const message = getErrorMessage(error, 'Failed to remove VMware connection');
      notificationStore.error(message);
      logger.error('[VMware Settings] Delete failed', error);
    } finally {
      setDeleting(false);
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
