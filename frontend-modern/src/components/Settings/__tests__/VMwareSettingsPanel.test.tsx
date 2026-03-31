import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { VMwareConnection } from '@/api/vmware';
import { VMwareSettingsPanel } from '../VMwareSettingsPanel';
import type { VMwareSettingsPanelState } from '../useVMwareSettingsPanelState';

const mockState = vi.hoisted(() => ({
  closeDeleteDialog: vi.fn(),
  closeDialog: vi.fn(),
  connections: vi.fn((): VMwareConnection[] => []),
  deleteDialogOpen: vi.fn(() => false),
  deletePendingConnection: vi.fn(),
  deleting: vi.fn(() => false),
  dialogOpen: vi.fn(() => false),
  editingConnection: vi.fn((): VMwareConnection | null => null),
  featureDisabled: vi.fn(() => false),
  featureDisabledMessage: vi.fn(() => ''),
  form: vi.fn(() => ({
    name: '',
    host: '',
    port: '443',
    username: '',
    password: '',
    insecureSkipVerify: false,
    enabled: true,
    hasStoredPassword: false,
  })),
  loadConnections: vi.fn(),
  loading: vi.fn(() => false),
  loadingError: vi.fn(() => null),
  openCreateDialog: vi.fn(),
  openDeleteDialog: vi.fn(),
  openEditDialog: vi.fn(),
  pendingDeleteConnection: vi.fn((): VMwareConnection | null => null),
  saveCurrentForm: vi.fn(),
  saving: vi.fn(() => false),
  testCurrentForm: vi.fn(),
  testSavedConnection: vi.fn(),
  testing: vi.fn(() => false),
  updateForm: vi.fn(),
}));

describe('VMwareSettingsPanel', () => {
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
      port: '443',
      username: '',
      password: '',
      insecureSkipVerify: false,
      enabled: true,
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

  const renderPanel = () =>
    render(() => <VMwareSettingsPanel state={mockState as unknown as VMwareSettingsPanelState} />);

  it('renders the settings shell and existing connections', () => {
    mockState.connections.mockReturnValue([
      {
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: false,
        enabled: true,
        poll: {
          lastSuccessAt: new Date(Date.now() - 60_000).toISOString(),
        },
        observed: {
          collectedAt: new Date(Date.now() - 60_000).toISOString(),
          hosts: 3,
          vms: 42,
          datastores: 6,
          viRelease: '8.0.3',
        },
      },
      {
        id: 'conn-2',
        name: 'staging-vcenter',
        host: 'staging.lab.local',
        username: 'operator@vsphere.local',
        insecureSkipVerify: true,
        enabled: false,
      },
    ]);

    renderPanel();

    expect(screen.getByText('VMware vSphere platform integration')).toBeInTheDocument();
    expect(screen.getByText('VMware connections')).toBeInTheDocument();
    expect(screen.getByText('lab-vcenter')).toBeInTheDocument();
    expect(screen.getByText('Healthy')).toBeInTheDocument();
    expect(
      within(screen.getByTestId('vmware-connection-conn-2')).getAllByText('Disabled'),
    ).toHaveLength(2);
    expect(screen.getByText('3 hosts')).toBeInTheDocument();
    expect(screen.getByText('42 vms')).toBeInTheDocument();
    expect(screen.getByText('6 datastores')).toBeInTheDocument();
    expect(screen.getByText('VI JSON 8.0.3')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add VMware connection' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Add VMware connection' }));
    expect(mockState.openCreateDialog).toHaveBeenCalledTimes(1);

    fireEvent.click(
      within(screen.getByTestId('vmware-connection-conn-1')).getByRole('button', { name: 'Test' }),
    );
    expect(mockState.testSavedConnection).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'conn-1' }),
    );
  });

  it('shows the feature gate warning when the backend path is disabled', () => {
    mockState.featureDisabled.mockReturnValue(true);
    mockState.featureDisabledMessage.mockReturnValue(
      'VMware integration has been explicitly disabled',
    );

    renderPanel();

    expect(screen.getByText('VMware integration is disabled')).toBeInTheDocument();
    expect(screen.getByText('VMware integration has been explicitly disabled')).toBeInTheDocument();
    expect(screen.getByText(/PULSE_ENABLE_VMWARE=false/)).toBeInTheDocument();
    expect(screen.queryByText('VMware connections')).not.toBeInTheDocument();
  });
});
