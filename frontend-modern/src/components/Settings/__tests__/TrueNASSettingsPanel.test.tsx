import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
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
    pollIntervalSeconds: '60',
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
  monitoredSystemPreview: vi.fn(() => null),
  monitoredSystemPreviewError: vi.fn(() => null),
  monitoredSystemPreviewErrorTitle: vi.fn(() => null),
  monitoredSystemAdmissionSaveBlocked: vi.fn(() => false),
  previewCurrentForm: vi.fn(),
  previewing: vi.fn(() => false),
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
      pollIntervalSeconds: '60',
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
    mockState.monitoredSystemPreview.mockReturnValue(null);
    mockState.monitoredSystemPreviewError.mockReturnValue(null);
    mockState.monitoredSystemPreviewErrorTitle.mockReturnValue(null);
    mockState.monitoredSystemAdmissionSaveBlocked.mockReturnValue(false);
    mockState.previewing.mockReturnValue(false);
    mockState.saving.mockReturnValue(false);
    mockState.testing.mockReturnValue(false);
  });

  afterEach(() => {
    cleanup();
  });

  const renderPanel = () =>
    render(() => (
      <TrueNASSettingsPanel state={mockState as unknown as TrueNASSettingsPanelState} />
    ));

  it('renders the settings shell and existing connections', () => {
    mockState.connections.mockReturnValue([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        port: 443,
        apiKey: '********',
        pollIntervalSeconds: 60,
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
        poll: {
          intervalSeconds: 60,
          lastSuccessAt: new Date(Date.now() - 60_000).toISOString(),
        },
        observed: {
          host: 'tower',
          resourceId: 'tower',
          collectedAt: new Date(Date.now() - 60_000).toISOString(),
          systems: 1,
          storagePools: 2,
          datasets: 12,
          apps: 4,
          disks: 8,
          recoveryArtifacts: 18,
        },
      },
      {
        id: 'conn-2',
        name: 'vault',
        host: 'vault.local',
        username: 'admin',
        useHttps: true,
        insecureSkipVerify: false,
        enabled: false,
        pollIntervalSeconds: 300,
      },
    ]);

    renderPanel();

    expect(screen.getByText('TrueNAS platform integration')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS connections')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('API key auth')).toBeInTheDocument();
    expect(screen.getByText('Healthy')).toBeInTheDocument();
    expect(screen.getByText('Paused')).toBeInTheDocument();
    expect(screen.getByText('Poll every 1 minute')).toBeInTheDocument();
    expect(screen.getByText('Poll every 5 minutes')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Infrastructure' })).toHaveAttribute(
      'href',
      '/infrastructure?source=truenas&resource=tower',
    );
    expect(screen.getByRole('link', { name: 'Workloads' })).toHaveAttribute(
      'href',
      '/workloads?type=app-container&platform=truenas&agent=tower',
    );
    expect(screen.getByRole('link', { name: 'Storage' })).toHaveAttribute(
      'href',
      '/storage?source=truenas&node=tower',
    );
    expect(screen.getByRole('link', { name: 'Recovery' })).toHaveAttribute(
      'href',
      '/recovery?platform=truenas&node=tower',
    );
    expect(screen.getByRole('button', { name: 'Add TrueNAS connection' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Add TrueNAS connection' }));
    expect(mockState.openCreateDialog).toHaveBeenCalledTimes(1);

    fireEvent.click(
      within(screen.getByTestId('truenas-connection-conn-1')).getByRole('button', { name: 'Test' }),
    );
    expect(mockState.testSavedConnection).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'conn-1' }),
    );
  });

  it('shows the feature gate warning when the backend path is disabled', () => {
    mockState.featureDisabled.mockReturnValue(true);
    mockState.featureDisabledMessage.mockReturnValue(
      'TrueNAS integration has been explicitly disabled',
    );

    renderPanel();

    expect(screen.getByText('TrueNAS integration is disabled')).toBeInTheDocument();
    expect(
      screen.getByText('TrueNAS integration has been explicitly disabled'),
    ).toBeInTheDocument();
    expect(screen.getByText(/PULSE_ENABLE_TRUENAS=false/)).toBeInTheDocument();
    expect(screen.queryByText('TrueNAS connections')).not.toBeInTheDocument();
  });

  it('renders monitored-system preview inside the dialog and blocks save over the limit', () => {
    mockState.dialogOpen.mockReturnValue(true);
    mockState.monitoredSystemPreview.mockReturnValue({
      current_count: 9,
      projected_count: 11,
      additional_count: 2,
      limit: 10,
      would_exceed_limit: true,
      effect: 'splits_existing',
      current_systems: [],
      projected_systems: [
        {
          name: 'tower',
          type: 'truenas-system',
          status: 'online',
          source: 'truenas',
          status_explanation: { summary: '', reasons: [] },
          latest_included_signal: { name: '', type: '', at: '' },
          explanation: { summary: '', reasons: [], surfaces: [] },
        },
      ],
      current_system: null,
      projected_system: null,
    });
    mockState.monitoredSystemAdmissionSaveBlocked.mockReturnValue(true);

    renderPanel();

    expect(screen.getByText('This change exceeds your monitored-system limit')).toBeInTheDocument();
    expect(screen.getByText(/Current usage 9 \/ 10/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add connection' })).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: 'Preview impact' }));
    expect(mockState.previewCurrentForm).toHaveBeenCalledTimes(1);
  });

  it('renders canonical unavailable guidance and blocks save while preview usage is unsettled', () => {
    mockState.dialogOpen.mockReturnValue(true);
    mockState.monitoredSystemPreviewErrorTitle.mockReturnValue(
      'Monitored-system capacity is temporarily unavailable',
    );
    mockState.monitoredSystemPreviewError.mockReturnValue(
      'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    );
    mockState.monitoredSystemAdmissionSaveBlocked.mockReturnValue(true);

    renderPanel();

    expect(
      screen.getByText('Monitored-system capacity is temporarily unavailable'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add connection' })).toBeDisabled();
  });
});
