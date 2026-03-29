import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TrueNASSettingsPanel } from '../TrueNASSettingsPanel';
import type { TrueNASSettingsPanelState } from '../useTrueNASSettingsPanelState';

const mockState = vi.hoisted(() => ({
  closeDeleteDialog: vi.fn(),
  closeDialog: vi.fn(),
  connections: vi.fn(() => []),
  deleteDialogOpen: vi.fn(() => false),
  deletePendingConnection: vi.fn(),
  deleting: vi.fn(() => false),
  dialogOpen: vi.fn(() => false),
  editingConnection: vi.fn(() => null),
  featureDisabled: vi.fn(() => false),
  featureDisabledMessage: vi.fn(() => ''),
  form: vi.fn(() => ({
    name: '',
    host: '',
    port: '',
    authMode: 'apiKey' as const,
    apiKey: '',
    username: '',
    password: '',
    useHttps: true,
    insecureSkipVerify: false,
    fingerprint: '',
    enabled: true,
    hasStoredApiKey: false,
    hasStoredPassword: false,
  })),
  loadConnections: vi.fn(),
  loading: vi.fn(() => false),
  loadingError: vi.fn(() => null),
  openCreateDialog: vi.fn(),
  openDeleteDialog: vi.fn(),
  openEditDialog: vi.fn(),
  pendingDeleteConnection: vi.fn(() => null),
  saveCurrentForm: vi.fn(),
  saving: vi.fn(() => false),
  testCurrentForm: vi.fn(),
  testSavedConnection: vi.fn(),
  testing: vi.fn(() => false),
  updateForm: vi.fn(),
}));

describe('TrueNASSettingsPanel', () => {
  beforeEach(() => {
    Object.values(mockState).forEach((value) => {
      if (typeof value === 'function' && 'mockReset' in value) {
        value.mockReset();
      }
    });
    mockState.connections.mockReturnValue([]);
    mockState.deleteDialogOpen.mockReturnValue(false);
    mockState.deleting.mockReturnValue(false);
    mockState.dialogOpen.mockReturnValue(false);
    mockState.editingConnection.mockReturnValue(null);
    mockState.featureDisabled.mockReturnValue(false);
    mockState.featureDisabledMessage.mockReturnValue('');
    mockState.form.mockReturnValue({
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
    mockState.loading.mockReturnValue(false);
    mockState.loadingError.mockReturnValue(null);
    mockState.pendingDeleteConnection.mockReturnValue(null);
    mockState.saving.mockReturnValue(false);
    mockState.testing.mockReturnValue(false);
  });

  afterEach(() => {
    cleanup();
  });

  const renderPanel = () => render(() => <TrueNASSettingsPanel state={mockState as unknown as TrueNASSettingsPanelState} />);

  it('renders the settings shell and existing connections', () => {
    mockState.connections.mockReturnValue([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        port: 443,
        apiKey: '********',
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
      },
    ]);

    renderPanel();

    expect(screen.getByText('TrueNAS platform integration')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS connections')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('API key auth')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add TrueNAS connection' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Add TrueNAS connection' }));
    expect(mockState.openCreateDialog).toHaveBeenCalledTimes(1);
  });

  it('shows the feature gate warning when the backend path is disabled', () => {
    mockState.featureDisabled.mockReturnValue(true);
    mockState.featureDisabledMessage.mockReturnValue('truenas_disabled');

    renderPanel();

    expect(screen.getByText('TrueNAS integration is disabled')).toBeInTheDocument();
    expect(screen.getByText('truenas_disabled')).toBeInTheDocument();
    expect(screen.queryByText('TrueNAS connections')).not.toBeInTheDocument();
  });
});
