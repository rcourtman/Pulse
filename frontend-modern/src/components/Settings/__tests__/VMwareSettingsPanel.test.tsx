import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { VMwareConnection } from '@/api/vmware';
import { VMwareSettingsPanel } from '../VMwareSettingsPanel';
import type { VMwareSettingsPanelState } from '../useVMwareSettingsPanelState';

const mockState = vi.hoisted(() => ({
  closeDeleteDialog: vi.fn(),
  closeDialog: vi.fn(),
  connectionFailure: vi.fn(() => null),
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
    mockState.connectionFailure.mockReturnValue(null);
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
        enabled: true,
        poll: {
          lastAttemptAt: new Date(Date.now() - 30_000).toISOString(),
          lastError: {
            at: new Date(Date.now() - 30_000).toISOString(),
            category: 'auth',
            message: 'VMware authentication failed while creating the VI JSON API session',
          },
        },
      },
      {
        id: 'conn-3',
        name: 'partial-vcenter',
        host: 'partial.lab.local',
        username: 'readonly@vsphere.local',
        insecureSkipVerify: false,
        enabled: true,
        poll: {
          lastSuccessAt: new Date(Date.now() - 120_000).toISOString(),
        },
        observed: {
          collectedAt: new Date(Date.now() - 120_000).toISOString(),
          hosts: 2,
          vms: 18,
          datastores: 4,
          viRelease: '8.0.3',
          degraded: true,
          issueCount: 3,
          issues: [
            {
              stage: 'signals',
              category: 'permission',
              message: 'VMware permissions are insufficient for host overall status',
              occurrences: 2,
            },
          ],
        },
      },
    ]);

    renderPanel();

    expect(screen.getByText('VMware vSphere platform integration')).toBeInTheDocument();
    expect(screen.getByText('VMware connections')).toBeInTheDocument();
    expect(screen.getByText('lab-vcenter')).toBeInTheDocument();
    expect(screen.getByText('Healthy')).toBeInTheDocument();
    expect(screen.getByText('staging-vcenter')).toBeInTheDocument();
    expect(screen.getByText('Runtime failing')).toBeInTheDocument();
    expect(screen.getByText('Authentication failed')).toBeInTheDocument();
    expect(
      screen.getByText('VMware authentication failed while creating the VI JSON API session'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Verify the username, password, and account scope in vCenter before retrying.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('partial-vcenter')).toBeInTheDocument();
    expect(screen.getByText('Degraded')).toBeInTheDocument();
    expect(screen.getByText('3 hosts')).toBeInTheDocument();
    expect(screen.getByText('42 vms')).toBeInTheDocument();
    expect(screen.getByText('6 datastores')).toBeInTheDocument();
    expect(
      within(screen.getByTestId('vmware-connection-conn-1')).getByText('VI JSON 8.0.3'),
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId('vmware-connection-conn-3')).getByText('3 degraded reads'),
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId('vmware-connection-conn-3')).getByText(
        /VMware permissions are insufficient for host overall status/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Permissions are insufficient')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Grant the minimum VMware read privileges required for phase-1 inventory and health reads, then retry.',
      ),
    ).toBeInTheDocument();
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

  it('renders categorized draft test guidance inside the connection dialog', () => {
    mockState.dialogOpen.mockReturnValue(true);
    mockState.connectionFailure.mockReturnValue({
      title: 'Unsupported vCenter version',
      message: 'VMware vCenter 6.7 is below the supported VI JSON release floor',
      guidance:
        'Use a supported vCenter release within the current VI JSON phase-1 floor, then retry this connection test.',
      tone: 'warning',
      category: 'unsupported_version',
      code: 'vmware_connection_failed',
    });

    renderPanel();

    expect(screen.getByTestId('vmware-connection-test-feedback')).toBeInTheDocument();
    expect(screen.getByText('Unsupported vCenter version')).toBeInTheDocument();
    expect(
      screen.getByText('VMware vCenter 6.7 is below the supported VI JSON release floor'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Use a supported vCenter release within the current VI JSON phase-1 floor, then retry this connection test.',
      ),
    ).toBeInTheDocument();
  });
});
