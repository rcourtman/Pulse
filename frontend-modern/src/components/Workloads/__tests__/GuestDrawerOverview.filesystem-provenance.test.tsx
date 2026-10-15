import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal, type ComponentProps } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GuestDrawerOverview } from '../GuestDrawerOverview';
import { guestDiskDeferrals } from '../__fixtures__/guestDiskDeferrals';
import type { WorkloadGuest } from '@/types/workloads';

vi.mock('../GuestPhysicalDisks', () => ({ GuestPhysicalDisks: () => null }));

const path = '/srv/backups/covered-filesystem';
const unavailableDisk = {
  cpu: true,
  memory: true,
  disk: false,
  networkIO: true,
  diskIO: true,
  uptime: true,
} satisfies NonNullable<WorkloadGuest['telemetryAvailability']>;
const disk = {
  mountpoint: path,
  device: '/dev/vda1',
  type: 'ext4',
  total: 10 * 1024 ** 3,
  used: 9 * 1024 ** 3,
  usage: 90,
};
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
  disk,
  disks: [disk],
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  tags: [],
  lock: '',
  lastBackup: 0,
  lastSeen: '2026-10-04T21:27:00Z',
  ...overrides,
});
const props = (value: WorkloadGuest): ComponentProps<typeof GuestDrawerOverview> => ({
  guest: value,
  guestOsSummary: 'Linux',
  agentHeading: 'Guest agent',
  agentLabel: 'QEMU 9.2',
  agentTitle: 'Previously observed agent version',
  hasAgentInfo: true,
  hasFilesystemDetails: true,
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
const filesystemRow = () => screen.getByText(path).closest('tr')!;
afterEach(cleanup);

describe('filesystem details do not promote retained usage to current health', () => {
  it('withdraws stopped VM usage without waiting for a read-error reason', () => {
    const [value, setValue] = createSignal(guest());
    render(() => <GuestDrawerOverview {...props(value())} />);
    const details = screen.getByTestId('guest-technical-details');
    expect(within(filesystemRow()).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '90');
    setValue({ ...value(), status: 'stopped', diskStatusReason: undefined });
    expect(within(filesystemRow()).queryByRole('progressbar')).not.toBeInTheDocument();
    expect(within(filesystemRow()).getByText('Usage unavailable · ?/10.0 GB · EXT4')).toBeVisible();
    expect(
      screen.getByText('Guest filesystem stats unavailable while the VM is stopped.'),
    ).toBeVisible();
    expect(screen.getByTestId('guest-technical-details')).toBe(details);
    // Restarting the guest cannot clear an explicit provider read deferral.
    setValue({ ...value(), status: 'running', diskStatusReason: 'prev-agent-busy' });
    expect(within(filesystemRow()).getByText(/^Last known 90%/)).toBeVisible();
    expect(within(filesystemRow()).queryByRole('progressbar')).not.toBeInTheDocument();
    setValue(guest({ disks: [{ ...disk, used: 3 * 1024 ** 3, usage: 30 }] }));
    expect(within(filesystemRow()).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '30');
    expect(screen.getByTestId('guest-technical-details')).toBe(details);
  });

  it.each(['permission-denied', 'prev-permission-denied'])(
    'shows scoped-access guidance for %s and clears it only on a new reading',
    (reason) => {
      const [value, setValue] = createSignal(guest({ diskStatusReason: reason }));
      render(() => <GuestDrawerOverview {...props(value())} />);
      const message = screen.getByText(/Do not widen intentionally scoped access/);
      expect(message).toBeVisible();
      expect(message).toHaveTextContent('If this VM is an intended monitoring target');
      expect(message).toHaveTextContent('does not establish guest or filesystem health');
      expect(message).toHaveTextContent('Defer access changes and live probes during backups');
      const row = within(filesystemRow());
      expect(row.queryByRole('progressbar')).not.toBeInTheDocument();
      expect(
        row.getByText(
          reason.startsWith('prev-')
            ? 'Last known 90% · 9.00 GB/10.0 GB · EXT4'
            : 'Usage unavailable · ?/10.0 GB · EXT4',
        ),
      ).toBeVisible();
      expect(screen.getByRole('link', { name: 'Filesystem reading guidance' })).toHaveAttribute(
        'href',
        '/docs/VM_DISK_MONITORING#a-missing-reading-is-not-an-installation-diagnosis',
      );
      setValue(guest({ diskStatusReason: '' }));
      expect(
        screen.queryByText(/Do not widen intentionally scoped access/),
      ).not.toBeInTheDocument();
      expect(within(filesystemRow()).getByRole('progressbar')).toHaveAttribute(
        'aria-valuenow',
        '90',
      );
    },
  );

  it.each(guestDiskDeferrals)(
    'keeps prev-%s values labelled, without a current utilization bar',
    (reason) => {
      render(() => (
        <GuestDrawerOverview {...props(guest({ diskStatusReason: `prev-${reason}` }))} />
      ));
      const row = within(filesystemRow());
      expect(row.getByText('Last known 90% · 9.00 GB/10.0 GB · EXT4')).toBeVisible();
      expect(row.queryByRole('progressbar')).not.toBeInTheDocument();
      expect(screen.getByText(/^Using last known disk stats\./)).toBeVisible();
    },
  );

  it.each([
    ...guestDiskDeferrals.map(([reason]) => reason),
    'permission-denied',
    'future-private-reason',
  ])('declines unqualified numeric usage for %s', (reason) => {
    render(() => <GuestDrawerOverview {...props(guest({ diskStatusReason: reason }))} />);
    const row = within(filesystemRow());
    expect(row.getByText('Usage unavailable · ?/10.0 GB · EXT4')).toBeVisible();
    expect(row.queryByRole('progressbar')).not.toBeInTheDocument();
    expect(row.queryByText(/90%|9\.00 GB/)).not.toBeInTheDocument();
    expect(screen.queryByText('future-private-reason')).not.toBeInTheDocument();
  });

  it('does not fabricate retained usage from capacity, negative or missing bytes', () => {
    const [value, setValue] = createSignal(
      guest({ diskStatusReason: 'prev-vm-locked', disks: [{ ...disk, used: undefined }] }),
    );
    render(() => <GuestDrawerOverview {...props(value())} />);
    for (const input of [undefined, -1, NaN]) {
      setValue({ ...value(), disks: [{ ...disk, used: input }] });
      expect(
        within(filesystemRow()).getByText('Usage unavailable · ?/10.0 GB · EXT4'),
      ).toBeVisible();
      expect(within(filesystemRow()).queryByRole('progressbar')).not.toBeInTheDocument();
    }
  });

  it('withdraws a numeric bar on explicit unavailability even without a reason', () => {
    render(() => (
      <GuestDrawerOverview {...props(guest({ telemetryAvailability: unavailableDisk }))} />
    ));
    expect(within(filesystemRow()).getByText('Usage unavailable · ?/10.0 GB · EXT4')).toBeVisible();
    expect(within(filesystemRow()).queryByRole('progressbar')).not.toBeInTheDocument();
  });

  it('explains explicit unavailability even before any filesystem sample', () => {
    render(() => (
      <GuestDrawerOverview
        {...props(guest({ disks: [], telemetryAvailability: unavailableDisk }))}
        hasFilesystemDetails={false}
      />
    ));
    expect(screen.getByText('Filesystems')).toBeVisible();
    expect(screen.getByText('Filesystem usage is unavailable.')).toBeVisible();
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
  });

  it('keeps retained values until the reason clears, then resumes the same drawer', () => {
    const [value, setValue] = createSignal(
      guest({ diskStatusReason: 'prev-vm-locked', lock: 'backup', backupInProgress: true }),
    );
    render(() => <GuestDrawerOverview {...props(value())} />);
    const initialDrawer = screen.getByTestId('guest-technical-details');
    setValue({ ...value(), lock: '', backupInProgress: false });
    expect(within(filesystemRow()).getByText(/^Last known 90%/)).toBeVisible();
    expect(within(filesystemRow()).queryByRole('progressbar')).not.toBeInTheDocument();
    setValue({
      ...value(),
      diskStatusReason: '',
      disks: [{ ...disk, used: 3 * 1024 ** 3, usage: 30 }],
    });
    expect(screen.getByTestId('guest-technical-details')).toBe(initialDrawer);
    expect(within(filesystemRow()).getByText('30% · 3.00 GB/10.0 GB · EXT4')).toBeVisible();
    expect(
      within(filesystemRow()).getByRole('progressbar', { name: `Filesystem ${path} utilization` }),
    ).toHaveAttribute('aria-valuenow', '30');
    expect(screen.queryByText(/Using last known/)).not.toBeInTheDocument();
  });

  it.each(['qemu', 'lxc'])('preserves fresh %s evidence with no deferral', (type) => {
    render(() => <GuestDrawerOverview {...props(guest({ type }))} />);
    expect(within(filesystemRow()).getByText('90% · 9.00 GB/10.0 GB · EXT4')).toBeVisible();
    expect(within(filesystemRow()).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '90');
    expect(
      within(filesystemRow()).queryByText(/Last known|Usage unavailable/),
    ).not.toBeInTheDocument();
  });

  it('does not attach VM deferral semantics to an independent LXC reading', () => {
    render(() => (
      <GuestDrawerOverview {...props(guest({ type: 'lxc', diskStatusReason: 'prev-vm-locked' }))} />
    ));
    expect(within(filesystemRow()).getByText('90% · 9.00 GB/10.0 GB · EXT4')).toBeVisible();
    expect(within(filesystemRow()).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '90');
  });
});

describe('independent aggregate percentages do not invent byte facts', () => {
  it.each([Infinity, NaN, -1, 0])(
    'withholds invalid capacity %s even with a valid current percentage',
    (total) => {
      render(() => (
        <GuestDrawerOverview
          {...props(guest({ disk: { total, used: 50, usage: 50 }, disks: [] }))}
          currentMetrics={{ disk: 50 }}
        />
      ));
      expect(screen.queryByText(/of .*disk$/)).not.toBeInTheDocument();
    },
  );
});
