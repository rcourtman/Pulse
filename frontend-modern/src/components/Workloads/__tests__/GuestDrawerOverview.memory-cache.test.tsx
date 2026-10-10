import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal, type ComponentProps } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { GuestDrawerOverview } from '../GuestDrawerOverview';
import type { WorkloadGuest } from '@/types/workloads';

vi.mock('../GuestPhysicalDisks', () => ({ GuestPhysicalDisks: () => null }));

const observedAt = '2026-10-04T13:00:00Z';

const guest = (source: string, overrides: Partial<WorkloadGuest> = {}): WorkloadGuest => ({
  id: 'lab:pve-a:101',
  vmid: 101,
  name: 'memory-guest',
  node: 'pve-a',
  instance: 'lab',
  status: 'running',
  type: 'qemu',
  cpu: 0.1,
  cpus: 2,
  memory: {
    total: 1024 ** 3,
    used: 960 * 1024 ** 2,
    free: 64 * 1024 ** 2,
    usage: 94,
    observation: { state: 'current', source, observedAt },
  },
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

const overviewProps = (value: WorkloadGuest): ComponentProps<typeof GuestDrawerOverview> => ({
  guest: value,
  guestOsSummary: 'Linux',
  agentHeading: 'Guest agent',
  agentLabel: '',
  agentTitle: '',
  hasAgentInfo: false,
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
});

beforeEach(() => {
  vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-04T14:00:00Z'));
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('guest drawer cache-inclusive memory note', () => {
  it('explains a cache-inclusive VM reading as visible text with the fix', () => {
    render(() => <GuestDrawerOverview {...overviewProps(guest('status-mem'))} />);
    expect(screen.getByText('Memory reading').closest('tr')).toHaveTextContent(
      'Current · Proxmox (may include cache)',
    );
    const note = screen.getByText('About this reading').closest('tr')!;
    // Stacked, so the long note spans the card instead of a 7rem value column.
    expect(screen.getByText('About this reading').closest('td')).toHaveAttribute('colspan', '2');
    expect(screen.getByText('Memory reading').closest('td')).not.toHaveAttribute('colspan');
    expect(note).toHaveTextContent('a high percentage alone does not mean it is short of memory');
    expect(note).toHaveTextContent('Pulse agent in the VM');
    expect(note).toHaveTextContent('QEMU guest agent on a Linux VM');
  });

  it('follows the guest when a better source replaces the cache-inclusive one', () => {
    const [value, setValue] = createSignal(guest('status-mem'));
    render(() => <GuestDrawerOverview {...overviewProps(value())} />);
    expect(screen.getByText('About this reading')).toBeInTheDocument();
    setValue(guest('agent'));
    expect(screen.queryByText('About this reading')).not.toBeInTheDocument();
    expect(screen.getByText('Memory reading').closest('tr')).not.toHaveTextContent('cache');
    setValue(guest('cluster-resources', { type: 'lxc' }));
    expect(screen.getByText('About this reading').closest('tr')).toHaveTextContent('container');
  });

  it('keeps the caveat on a retained reading', () => {
    const retained = guest('status-mem');
    retained.memory = {
      ...retained.memory,
      observation: { state: 'last-known', source: 'status-mem', observedAt },
    };
    render(() => <GuestDrawerOverview {...overviewProps(retained)} />);
    expect(screen.getByText('Memory reading').closest('tr')).toHaveTextContent(
      'Last known · Proxmox (may include cache)',
    );
    expect(screen.getByText('About this reading')).toBeInTheDocument();
  });

  it('gives a container the Pulse agent only', () => {
    render(() => (
      <GuestDrawerOverview {...overviewProps(guest('cluster-resources', { type: 'lxc' }))} />
    ));
    const note = screen.getByText('About this reading').closest('tr')!;
    expect(note).toHaveTextContent('Pulse agent in the container');
    expect(note).not.toHaveTextContent('QEMU');
  });

  it.each(['guest-agent-meminfo', 'agent'])(
    'adds no note to a cache-aware %s reading',
    (source) => {
      render(() => <GuestDrawerOverview {...overviewProps(guest(source))} />);
      expect(screen.getByText('Memory reading').closest('tr')).not.toHaveTextContent('cache');
      expect(screen.queryByText('About this reading')).not.toBeInTheDocument();
    },
  );
});
