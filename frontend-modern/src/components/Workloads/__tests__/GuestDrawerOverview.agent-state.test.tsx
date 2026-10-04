import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal, type ComponentProps } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GuestDrawerOverview } from '../GuestDrawerOverview';
import type { WorkloadGuest } from '@/types/workloads';

vi.mock('../GuestPhysicalDisks', () => ({ GuestPhysicalDisks: () => null }));

const guest = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest => ({
  id: 'lab:pve-a:101',
  vmid: 101,
  name: 'backup-guest',
  node: 'pve-a',
  instance: 'lab',
  status: 'running',
  type: 'qemu',
  cpu: 0.1,
  cpus: 2,
  memory: { total: 1024 ** 3, used: 256 * 1024 ** 2, free: 768 * 1024 ** 2, usage: 25 },
  disk: { total: 10 * 1024 ** 3, used: 5 * 1024 ** 3, usage: 50 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  tags: [],
  lock: '',
  lastBackup: 0,
  lastSeen: '2026-10-04T14:00:00Z',
  ...overrides,
});

function overviewProps(value: WorkloadGuest): ComponentProps<typeof GuestDrawerOverview> {
  return {
    guest: value,
    guestOsSummary: 'Linux',
    agentHeading: value.agentKind === 'pulse' ? 'Pulse Agent' : 'Guest agent',
    agentLabel: value.agentVersion ? `QEMU ${value.agentVersion}` : '',
    agentTitle: 'Previously observed agent version',
    hasAgentInfo: Boolean(value.agentVersion),
    hasFilesystemDetails: false,
    hasNetworkInterfaces: false,
    hasOsInfo: true,
    hasWorkloadActionAgent: false,
    showInGuestAgentInstallCue: false,
    ipAddresses: [],
    networkInterfaces: [],
    normalizedTags: [],
    backupPresentation: null,
    workloadActionAgentTitle: '',
  };
}

const readRow = () => screen.getByText('Guest-agent reads').closest('tr')!;

afterEach(cleanup);

describe('guest agent coverage requires current state, not cached metadata', () => {
  it.each([
    ['deferred', 'Deferred'],
    ['expected-unreachable', 'Unreachable'],
    ['not-running', 'Not running'],
    ['disabled', 'Disabled'],
    ['available', 'Reported available'],
    ['future-state-private-detail', 'Unknown'],
  ])('shows %s independently of an old version or its absence', (state, label) => {
    const [value, setValue] = createSignal(guest({ agentVersion: '9.2', guestAgentStatus: state }));
    render(() => <GuestDrawerOverview {...overviewProps(value())} />);
    expect(within(readRow()).getByText(label)).toBeVisible();
    expect(screen.queryByText(/agent connected/i)).not.toBeInTheDocument();
    expect(screen.queryByText('future-state-private-detail')).not.toBeInTheDocument();
    setValue({ ...value(), agentVersion: '' });
    expect(within(readRow()).getByText(label)).toBeVisible();
    expect(screen.queryByText('QEMU 9.2')).not.toBeInTheDocument();
  });

  it('does not turn a version-only observation into liveness', () => {
    render(() => <GuestDrawerOverview {...overviewProps(guest({ agentVersion: '9.2' }))} />);
    expect(screen.getByText('Guest agent observed')).toBeVisible();
    expect(within(readRow()).getByText('Unknown')).toBeVisible();
    expect(screen.queryByText(/connected/i)).not.toBeInTheDocument();
  });

  it('does not hide deferred guest reads behind parent-node action ownership', () => {
    render(() => (
      <GuestDrawerOverview
        {...overviewProps(guest({ guestAgentStatus: 'deferred' }))}
        hasWorkloadActionAgent={true}
        workloadActionAgentTitle="Assigned parent-node action path"
      />
    ));
    expect(screen.getByText('Node agent assigned')).toBeVisible();
    expect(within(readRow()).getByText('Deferred')).toBeVisible();
  });

  it('withdraws cached connectivity across the same guest lock, uncertainty and recovery', () => {
    const [value, setValue] = createSignal(
      guest({ agentVersion: '9.2', guestAgentStatus: 'available' }),
    );
    render(() => <GuestDrawerOverview {...overviewProps(value())} />);
    expect(within(readRow()).getByText('Reported available')).toBeVisible();
    setValue({
      ...value(),
      guestAgentStatus: 'available',
      lock: 'backup',
      backupInProgress: true,
      diskStatusReason: 'prev-vm-locked',
    });
    expect(within(readRow()).getByText('Deferred')).toBeVisible();
    expect(screen.getByText('QEMU 9.2')).toBeVisible();
    setValue({
      ...value(),
      lock: '',
      backupInProgress: false,
      guestAgentStatus: 'expected-unreachable',
      diskStatusReason: 'agent-timeout',
    });
    expect(within(readRow()).getByText('Unreachable')).toBeVisible();
    setValue({ ...value(), guestAgentStatus: 'available', diskStatusReason: '' });
    expect(within(readRow()).getByText('Reported available')).toBeVisible();
    expect(screen.queryByText(/connected/i)).not.toBeInTheDocument();
  });

  it('keeps Pulse Agent metadata separate from QEMU read availability', () => {
    render(() => (
      <GuestDrawerOverview
        {...overviewProps(
          guest({
            agentKind: 'pulse',
            agentVersion: '6.4.5',
            guestAgentStatus: 'expected-unreachable',
          }),
        )}
      />
    ));
    expect(screen.getByText('Pulse Agent observed')).toBeVisible();
    expect(within(readRow()).getByText('Unreachable')).toBeVisible();
    expect(screen.queryByText('Pulse Agent connected')).not.toBeInTheDocument();
  });

  it('does not add QEMU state to a non-Proxmox VM or container from a version alone', () => {
    const [value, setValue] = createSignal(
      guest({
        type: 'vm',
        platformType: 'vmware-vsphere',
        platformScopes: ['vmware-vsphere'],
        agentKind: 'pulse',
        agentVersion: '6.4.5',
      }),
    );
    render(() => <GuestDrawerOverview {...overviewProps(value())} />);
    expect(screen.getByText('Pulse Agent observed')).toBeVisible();
    expect(screen.queryByText('Guest-agent reads')).not.toBeInTheDocument();
    setValue({
      ...value(),
      type: 'lxc',
      platformType: 'proxmox-pve',
      platformScopes: ['proxmox-pve'],
    });
    expect(screen.queryByText('Guest-agent reads')).not.toBeInTheDocument();
  });
});
