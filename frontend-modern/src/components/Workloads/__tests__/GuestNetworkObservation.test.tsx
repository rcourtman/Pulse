import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal, type ComponentProps } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GuestDrawerOverview } from '../GuestDrawerOverview';
import type { WorkloadGuest } from '@/types/workloads';

vi.mock('../GuestPhysicalDisks', () => ({ GuestPhysicalDisks: () => null }));
afterEach(cleanup);
const interfaces = [
  {
    name: 'eth0',
    addresses: ['192.0.2.80'],
    mac: '02:00:00:00:00:80',
    rxBytes: 1024,
    txBytes: 2048,
  },
];
const guest = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest => ({
  id: 'fixture:pve-a:101',
  vmid: 101,
  name: 'network-guest',
  node: 'pve-a',
  instance: 'fixture',
  type: 'qemu',
  status: 'running',
  cpu: 0,
  cpus: 2,
  memory: { total: 1024, used: 0, free: 1024, usage: 0 },
  disk: { total: 1024, used: 0, free: 1024, usage: 0 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 100,
  template: false,
  tags: [],
  lastBackup: 0,
  lastSeen: '2026-10-20T12:00:00Z',
  ipAddresses: ['192.0.2.80'],
  networkInterfaces: interfaces,
  ...overrides,
});
const props = (value: WorkloadGuest): ComponentProps<typeof GuestDrawerOverview> => ({
  guest: value,
  guestOsSummary: '',
  agentHeading: 'Guest agent',
  agentLabel: '',
  agentTitle: '',
  hasAgentInfo: false,
  hasFilesystemDetails: false,
  hasNetworkInterfaces: true,
  hasOsInfo: false,
  hasWorkloadActionAgent: false,
  showInGuestAgentInstallCue: false,
  ipAddresses: value.ipAddresses!,
  networkInterfaces: value.networkInterfaces!,
  normalizedTags: [],
  backupPresentation: null,
  workloadActionAgentTitle: '',
});

describe('native power does not renew retained guest network details', () => {
  it.each(['paused', 'suspended', 'stopped'])(
    'qualifies both selected network producers on a %s VM without losing identity',
    (status) => {
      const [value, setValue] = createSignal(guest());
      render(() => <GuestDrawerOverview {...props(value())} />);
      const details = screen.getByTestId('guest-technical-details') as HTMLDetailsElement;
      details.open = true;
      for (const networkInterfacesFromAgent of [true, false]) {
        setValue(guest({ status, networkInterfacesFromAgent, agentStale: false }));
        expect(
          screen.getByText(
            `Using last known interface details. Proxmox reports this VM ${status}; retained addresses and counters do not establish a current connection.`,
          ),
        ).toBeVisible();
        expect(screen.getByText('Last known IP 192.0.2.80')).toBeVisible();
        expect(screen.getByText('eth0')).toBeVisible();
        expect(screen.getByText(/RX 1.00 KB \/ TX 2.00 KB/)).toBeVisible();
        expect(screen.getByTestId('guest-technical-details')).toBe(details);
      }
      setValue(guest({ networkInterfacesFromAgent: false }));
      expect(screen.queryByText(/Using last known interface details/)).not.toBeInTheDocument();
      expect(screen.queryByText('Last known IP 192.0.2.80')).not.toBeInTheDocument();
      expect(screen.getByTestId('guest-technical-details')).toBe(details);
    },
  );
});
