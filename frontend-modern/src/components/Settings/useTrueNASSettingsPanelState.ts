import { createMemo, createSignal, onMount } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { apiErrorStatus } from '@/api/responseUtils';
import {
  TrueNASAPI,
  isRedactedTrueNASSecret,
  type TrueNASConnection,
  type TrueNASConnectionInput,
} from '@/api/truenas';

type TrueNASAuthMode = 'apiKey' | 'userpass';

export interface TrueNASConnectionFormState {
  name: string;
  host: string;
  port: string;
  authMode: TrueNASAuthMode;
  apiKey: string;
  username: string;
  password: string;
  useHttps: boolean;
  insecureSkipVerify: boolean;
  fingerprint: string;
  enabled: boolean;
  hasStoredApiKey: boolean;
  hasStoredPassword: boolean;
}

const REDACTED_SECRET = '********';

const createEmptyFormState = (): TrueNASConnectionFormState => ({
  name: '',
  host: '',
  port: '',
  authMode: 'apiKey',
  apiKey: '',
  username: '',
  password: '',
  useHttps: true,
  insecureSkipVerify: false,
  fingerprint: '',
  enabled: true,
  hasStoredApiKey: false,
  hasStoredPassword: false,
});

const buildFormStateFromConnection = (connection: TrueNASConnection): TrueNASConnectionFormState => {
  const hasStoredAPIKey = isRedactedTrueNASSecret(connection.apiKey);
  const hasStoredPassword = isRedactedTrueNASSecret(connection.password);
  const authMode: TrueNASAuthMode =
    hasStoredAPIKey || Boolean((connection.apiKey || '').trim()) ? 'apiKey' : 'userpass';

  return {
    name: connection.name || '',
    host: connection.host || '',
    port: connection.port ? String(connection.port) : '',
    authMode,
    apiKey: '',
    username: authMode === 'userpass' ? connection.username || '' : '',
    password: '',
    useHttps: connection.useHttps,
    insecureSkipVerify: connection.insecureSkipVerify,
    fingerprint: connection.fingerprint || '',
    enabled: connection.enabled,
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

const buildConnectionInput = (form: TrueNASConnectionFormState): TrueNASConnectionInput => {
  const port = parseOptionalPort(form.port);
  const name = form.name.trim();
  const host = form.host.trim();
  const fingerprint = form.fingerprint.trim();

  const input: TrueNASConnectionInput = {
    ...(name ? { name } : {}),
    host,
    ...(port !== undefined ? { port } : {}),
    useHttps: form.useHttps,
    insecureSkipVerify: form.insecureSkipVerify,
    ...(fingerprint ? { fingerprint } : {}),
    enabled: form.enabled,
  };

  if (form.authMode === 'apiKey') {
    input.apiKey = form.apiKey.trim() || (form.hasStoredApiKey ? REDACTED_SECRET : '');
    input.username = '';
    input.password = '';
  } else {
    input.apiKey = '';
    input.username = form.username.trim();
    input.password = form.password.trim() || (form.hasStoredPassword ? REDACTED_SECRET : '');
  }

  return input;
};

export function useTrueNASSettingsPanelState() {
  const [connections, setConnections] = createSignal<TrueNASConnection[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [loadingError, setLoadingError] = createSignal<string | null>(null);
  const [featureDisabled, setFeatureDisabled] = createSignal(false);
  const [featureDisabledMessage, setFeatureDisabledMessage] = createSignal<string>('');
  const [dialogOpen, setDialogOpen] = createSignal(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = createSignal(false);
  const [editingConnectionId, setEditingConnectionId] = createSignal<string | null>(null);
  const [pendingDeleteConnection, setPendingDeleteConnection] = createSignal<TrueNASConnection | null>(
    null,
  );
  const [form, setForm] = createSignal<TrueNASConnectionFormState>(createEmptyFormState());
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [deleting, setDeleting] = createSignal(false);

  const editingConnection = createMemo(() =>
    connections().find((connection) => connection.id === editingConnectionId()) ?? null,
  );

  const loadConnections = async () => {
    setLoading(true);
    setLoadingError(null);
    try {
      const nextConnections = await TrueNASAPI.listConnections();
      setConnections(nextConnections);
      setFeatureDisabled(false);
      setFeatureDisabledMessage('');
    } catch (error) {
      if (apiErrorStatus(error) === 404) {
        setFeatureDisabled(true);
        setFeatureDisabledMessage(getErrorMessage(error, 'TrueNAS integration is not enabled'));
        setConnections([]);
        return;
      }
      const message = getErrorMessage(error, 'Failed to load TrueNAS connections');
      setLoadingError(message);
      logger.error('[TrueNAS Settings] Failed to load connections', error);
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
    setDialogOpen(true);
  };

  const openEditDialog = (connection: TrueNASConnection) => {
    setEditingConnectionId(connection.id);
    setForm(buildFormStateFromConnection(connection));
    setDialogOpen(true);
  };

  const resetDialogState = () => {
    setDialogOpen(false);
    setEditingConnectionId(null);
    setForm(createEmptyFormState());
  };

  const closeDialog = () => {
    if (saving() || testing()) return;
    resetDialogState();
  };

  const openDeleteDialog = (connection: TrueNASConnection) => {
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

  const updateForm = (patch: Partial<TrueNASConnectionFormState>) =>
    setForm((current) => ({ ...current, ...patch }));

  const testCurrentForm = async () => {
    setTesting(true);
    try {
      await TrueNASAPI.testConnection(buildConnectionInput(form()));
      notificationStore.success('TrueNAS connection successful');
      return true;
    } catch (error) {
      const message = getErrorMessage(error, 'TrueNAS connection failed');
      notificationStore.error(message);
      logger.error('[TrueNAS Settings] Connection test failed', error);
      return false;
    } finally {
      setTesting(false);
    }
  };

  const testSavedConnection = async (connection: TrueNASConnection) => {
    setTesting(true);
    try {
      await TrueNASAPI.testConnection(buildConnectionInput(buildFormStateFromConnection(connection)));
      notificationStore.success(`TrueNAS connection successful for ${connection.name || connection.host}`);
    } catch (error) {
      const message = getErrorMessage(error, 'TrueNAS connection failed');
      notificationStore.error(message);
      logger.error('[TrueNAS Settings] Saved connection test failed', error);
    } finally {
      setTesting(false);
    }
  };

  const saveCurrentForm = async () => {
    setSaving(true);
    try {
      const payload = buildConnectionInput(form());
      if (editingConnectionId()) {
        await TrueNASAPI.updateConnection(editingConnectionId()!, payload);
        notificationStore.success('TrueNAS connection updated');
      } else {
        await TrueNASAPI.createConnection(payload);
        notificationStore.success('TrueNAS connection added');
      }
      resetDialogState();
      await loadConnections();
    } catch (error) {
      const message = getErrorMessage(error, 'Failed to save TrueNAS connection');
      notificationStore.error(message);
      logger.error('[TrueNAS Settings] Save failed', error);
    } finally {
      setSaving(false);
    }
  };

  const deletePendingConnection = async () => {
    const connection = pendingDeleteConnection();
    if (!connection) return;
    setDeleting(true);
    try {
      await TrueNASAPI.deleteConnection(connection.id);
      notificationStore.success(`Removed ${connection.name || connection.host}`);
      resetDeleteDialogState();
      await loadConnections();
    } catch (error) {
      const message = getErrorMessage(error, 'Failed to remove TrueNAS connection');
      notificationStore.error(message);
      logger.error('[TrueNAS Settings] Delete failed', error);
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
    loadConnections,
    loading,
    loadingError,
    openCreateDialog,
    openDeleteDialog,
    openEditDialog,
    pendingDeleteConnection,
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
