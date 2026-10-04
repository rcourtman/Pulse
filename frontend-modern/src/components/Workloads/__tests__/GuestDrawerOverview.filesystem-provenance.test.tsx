import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal, type ComponentProps } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GuestDrawerOverview } from '../GuestDrawerOverview';
import { guestDiskDeferrals } from '../__fixtures__/guestDiskDeferrals';
import type { WorkloadGuest } from '@/types/workloads';

vi.mock('../GuestPhysicalDisks', () => ({ GuestPhysicalDisks: () => null }));

const path = '/srv/backups/covered-filesystem';
const disk = { mountpoint: path, device: '/dev/vda1', type: 'ext4', total: 10 * 1024 ** 3, used: 9 * 1024 ** 3, usage: 90 };
const guest = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest => ({
  id: 'lab:pve-a:101', vmid: 101, name: 'backup-guest', node: 'pve-a', instance: 'lab',
  status: 'running', type: 'qemu', cpu: 0.1, cpus: 2,
  memory: { total: 1024 ** 3, used: 256 * 1024 ** 2, free: 768 * 1024 ** 2, usage: 25 },
  disk, disks: [disk], networkIn: 0, networkOut: 0, diskRead: 0, diskWrite: 0,
  uptime: 3600, template: false, tags: [], lock: '', lastBackup: 0,
  lastSeen: '2026-10-04T21:27:00Z', ...overrides,
});
const props = (value: WorkloadGuest): ComponentProps<typeof GuestDrawerOverview> => ({
  guest: value, guestOsSummary: 'Linux', agentHeading: 'Guest agent', agentLabel: 'QEMU 9.2',
  agentTitle: 'Previously observed agent version', hasAgentInfo: true, hasFilesystemDetails: true,
  hasNetworkInterfaces: false, hasOsInfo: true, hasWorkloadActionAgent: false,
  showInGuestAgentInstallCue: false, ipAddresses: [], networkInterfaces: [], normalizedTags: [],
  backupPresentation: null, workloadActionAgentTitle: '',
});
const filesystemRow = () => screen.getByText(path).closest('tr')!;
afterEach(cleanup);

describe('filesystem details do not promote retained usage to current health', () => {
  it.each(guestDiskDeferrals)('keeps prev-%s values labelled, without a current utilization bar', (reason) => {
    render(() => <GuestDrawerOverview {...props(guest({ diskStatusReason: `prev-${reason}` }))} />);
    const row = within(filesystemRow());
    expect(row.getByText('Last known 90% · 9.0 GB/10.0 GB · EXT4')).toBeVisible();
    expect(row.queryByRole('progressbar')).not.toBeInTheDocument();
    expect(screen.getByText(/^Using last known disk stats\./)).toBeVisible();
  });

  it.each([...guestDiskDeferrals.map(([reason]) => reason), 'permission-denied', 'future-private-reason'])('declines unqualified numeric usage for %s', (reason) => {
    render(() => <GuestDrawerOverview {...props(guest({ diskStatusReason: reason }))} />);
    const row = within(filesystemRow());
    expect(row.getByText('Usage unavailable · ?/10.0 GB · EXT4')).toBeVisible();
    expect(row.queryByRole('progressbar')).not.toBeInTheDocument();
    expect(row.queryByText(/90%|9\.0 GB/)).not.toBeInTheDocument();
    expect(screen.queryByText('future-private-reason')).not.toBeInTheDocument();
  });

  it('does not fabricate retained usage from capacity, negative or missing bytes', () => {
    const [value, setValue] = createSignal(guest({ diskStatusReason: 'prev-vm-locked', disks: [{ ...disk, used: undefined }] }));
    render(() => <GuestDrawerOverview {...props(value())} />);
    for (const input of [undefined, -1, NaN]) {
      setValue({ ...value(), disks: [{ ...disk, used: input }] });
      expect(within(filesystemRow()).getByText('Usage unavailable · ?/10.0 GB · EXT4')).toBeVisible();
      expect(within(filesystemRow()).queryByRole('progressbar')).not.toBeInTheDocument();
    }
  });

  it('withdraws a numeric bar on explicit unavailability even without a reason', () => {
    render(() => <GuestDrawerOverview {...props(guest({ telemetryAvailability: { disk: false } }))} />);
    expect(within(filesystemRow()).getByText('Usage unavailable · ?/10.0 GB · EXT4')).toBeVisible();
    expect(within(filesystemRow()).queryByRole('progressbar')).not.toBeInTheDocument();
  });

  it('keeps retained values until the reason clears, then updates the same filesystem', () => {
    const [value, setValue] = createSignal(guest({ diskStatusReason: 'prev-vm-locked', lock: 'backup', backupInProgress: true }));
    render(() => <GuestDrawerOverview {...props(value())} />);
    const initialRow = filesystemRow();
    setValue({ ...value(), lock: '', backupInProgress: false });
    expect(within(filesystemRow()).getByText(/^Last known 90%/)).toBeVisible();
    expect(within(filesystemRow()).queryByRole('progressbar')).not.toBeInTheDocument();
    setValue({ ...value(), diskStatusReason: '', disks: [{ ...disk, used: 3 * 1024 ** 3, usage: 30 }] });
    expect(filesystemRow()).toBe(initialRow);
    expect(within(filesystemRow()).getByText('30% · 3.0 GB/10.0 GB · EXT4')).toBeVisible();
    expect(within(filesystemRow()).getByRole('progressbar', { name: `Filesystem ${path} utilization` })).toHaveAttribute('aria-valuenow', '30');
    expect(screen.queryByText(/Using last known/)).not.toBeInTheDocument();
  });

  it.each(['qemu', 'lxc'])('preserves fresh %s evidence with no deferral', (type) => {
    render(() => <GuestDrawerOverview {...props(guest({ type }))} />);
    expect(within(filesystemRow()).getByText('90% · 9.0 GB/10.0 GB · EXT4')).toBeVisible();
    expect(within(filesystemRow()).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '90');
    expect(within(filesystemRow()).queryByText(/Last known|Usage unavailable/)).not.toBeInTheDocument();
  });

  it('does not attach VM deferral semantics to an independent LXC reading', () => {
    render(() => <GuestDrawerOverview {...props(guest({ type: 'lxc', diskStatusReason: 'prev-vm-locked' }))} />);
    expect(within(filesystemRow()).getByText('90% · 9.0 GB/10.0 GB · EXT4')).toBeVisible();
    expect(within(filesystemRow()).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '90');
  });
});
