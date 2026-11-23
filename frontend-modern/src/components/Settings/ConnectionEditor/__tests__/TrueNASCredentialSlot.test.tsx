import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { TrueNASConnection } from '@/api/truenas';
import type { TrueNASSettingsPanelState } from '../../useTrueNASSettingsPanelState';
import { TrueNASCredentialSlot } from '../CredentialSlots/TrueNASCredentialSlot';

type MockForm = ReturnType<TrueNASSettingsPanelState['form']>;

function createMockTrueNASState(overrides: Partial<MockForm> = {}) {
  const [dialogOpen, setDialogOpen] = createSignal(false);
  const [saving] = createSignal(false);
  const [testing] = createSignal(false);
  const [previewing] = createSignal(false);
  const [form, setForm] = createSignal<MockForm>({
    name: 'truenas-lab',
    host: 'truenas.local',
    port: '443',
    pollIntervalSeconds: '60',
    authMode: 'apiKey',
    username: 'truenas_admin',
    apiKey: '',
    password: '',
    hasStoredApiKey: false,
    hasStoredPassword: false,
    fingerprint: '',
    useHttps: true,
    insecureSkipVerify: false,
    enabled: true,
    monitorDatasets: true,
    monitorPools: true,
    monitorReplication: true,
    ...overrides,
  });

  const openCreateDialog = vi.fn(() => setDialogOpen(true));
  const openEditDialog = vi.fn((conn: TrueNASConnection) => {
    setForm((current) => ({
      ...current,
      name: conn.name,
      host: conn.host,
      enabled: conn.enabled,
      hasStoredApiKey: true,
    }));
    setDialogOpen(true);
  });
  const closeDialog = vi.fn(() => setDialogOpen(false));
  const updateForm = vi.fn((patch: Partial<MockForm>) =>
    setForm((current) => ({ ...current, ...patch })),
  );
  const testCurrentForm = vi.fn(async () => {});
  const previewCurrentForm = vi.fn(async () => {});
  const saveCurrentForm = vi.fn(async () => {
    setDialogOpen(false);
  });

  const state = {
    dialogOpen,
    saving,
    testing,
    previewing,
    form,
    featureDisabled: () => false,
    featureDisabledMessage: () => null,
    connectionFeedback: () => null,
    monitoredSystemPreview: () => null,
    monitoredSystemPreviewError: () => null,
    monitoredSystemPreviewErrorTitle: () => null,
    openCreateDialog,
    openEditDialog,
    closeDialog,
    updateForm,
    testCurrentForm,
    previewCurrentForm,
    saveCurrentForm,
  } as unknown as TrueNASSettingsPanelState;

  return {
    state,
    openCreateDialog,
    openEditDialog,
    closeDialog,
    updateForm,
    testCurrentForm,
    saveCurrentForm,
  };
}

const existingConnection: TrueNASConnection = {
  id: 'tn-1',
  name: 'truenas-primary',
  host: '10.0.0.20',
  port: 443,
  useHttps: true,
  insecureSkipVerify: false,
  enabled: true,
  username: 'truenas_admin',
  pollIntervalSeconds: 60,
  monitorDatasets: true,
  monitorPools: true,
  monitorReplication: true,
};

describe('TrueNASCredentialSlot', () => {
  afterEach(() => cleanup());

  it('does not call onSaved when Cancel closes the dialog', async () => {
    const { state, closeDialog } = createMockTrueNASState();
    const onCancel = vi.fn();
    const onSaved = vi.fn();

    render(() => <TrueNASCredentialSlot state={state} onCancel={onCancel} onSaved={onSaved} />);

    expect(screen.getByLabelText('Enable polling immediately')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(closeDialog).toHaveBeenCalledTimes(1);
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSaved).not.toHaveBeenCalled();
  });

  it('calls onSaved after a successful save closes the dialog', async () => {
    const { state, saveCurrentForm } = createMockTrueNASState();
    const onCancel = vi.fn();
    const onSaved = vi.fn();

    render(() => <TrueNASCredentialSlot state={state} onCancel={onCancel} onSaved={onSaved} />);

    fireEvent.click(screen.getByRole('button', { name: 'Add connection' }));

    await waitFor(() => expect(saveCurrentForm).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(onCancel).not.toHaveBeenCalled();
  });

  it('uses Pause/Resume as the single enabled control in edit mode and stays synced', async () => {
    const { state } = createMockTrueNASState();
    const [enabled, setEnabled] = createSignal(true);
    const onToggleEnabled = vi.fn(() => setEnabled((value) => !value));

    render(() => (
      <TrueNASCredentialSlot
        state={state}
        editingConnection={existingConnection}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
        onToggleEnabled={onToggleEnabled}
        connectionEnabled={enabled()}
      />
    ));

    expect(screen.queryByLabelText('Enable polling immediately')).not.toBeInTheDocument();
    const pauseButton = screen.getByRole('button', { name: 'Pause connection' });
    fireEvent.click(pauseButton);

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Resume connection' })).toBeInTheDocument(),
    );
    expect(state.form().enabled).toBe(false);
  });
});
