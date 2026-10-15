import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal, type ComponentProps } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { GuestDrawerOverview } from '../GuestDrawerOverview';
import { guestDiskDeferrals } from '../__fixtures__/guestDiskDeferrals';

const paths = ['/mnt/Plex/Media/Animation/Films', '/mnt/Plex/Media/Animation/Series'];
const props: ComponentProps<typeof GuestDrawerOverview> = {
  guest: {
    id: 'guest-101',
    name: 'media',
    vmid: 101,
    type: 'lxc',
    status: 'running',
    cpus: 4,
    node: 'pve-a',
    instance: 'lab',
    cpu: 0,
    disk: { total: 10 * 1024 ** 3, used: 5 * 1024 ** 3, usage: 50 },
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: ['media'],
    lock: '',
    lastSeen: '2026-09-22T13:00:00Z',
    memory: { total: 8 * 1024 ** 3, used: 2 * 1024 ** 3, free: 6 * 1024 ** 3, usage: 25 },
    disks: paths.map((mountpoint, index) => ({
      mountpoint,
      type: `mp${index}`,
      total: 10 * 1024 ** 3,
      used: 5 * 1024 ** 3,
      usage: index === 0 ? 50 : -1,
    })),
  },
  guestOsSummary: 'Debian 13',
  agentHeading: 'Agent',
  agentLabel: 'Connected',
  agentTitle: 'Connected',
  hasAgentInfo: true,
  hasFilesystemDetails: true,
  hasNetworkInterfaces: false,
  hasOsInfo: true,
  hasWorkloadActionAgent: false,
  showInGuestAgentInstallCue: false,
  ipAddresses: ['192.0.2.1'],
  networkInterfaces: [],
  normalizedTags: ['media'],
  backupPresentation: null,
  workloadActionAgentTitle: '',
};

describe('GuestDrawerOverview filesystem labels', () => {
  afterEach(cleanup);

  it('keeps full OS facts and phone reading guidance outside the closed Details', () => {
    const os = 'Microsoft Windows Server 2025 Datacenter Evaluation long-build-identity';
    render(() => (
      <GuestDrawerOverview
        {...props}
        guestOsSummary={os}
        hasFilesystemDetails={false}
        guest={{ ...props.guest, type: 'qemu', disks: [], diskStatusReason: 'agent-not-running' }}
      />
    ));
    const facts = screen.getByTestId('guest-drawer-facts');
    expect(facts).toHaveTextContent(os);
    const system = screen.getByText(os);
    expect(system.closest('details')).toBeNull();
    expect(system).toHaveClass('whitespace-normal', 'wrap-anywhere');
    expect(system).not.toHaveClass('truncate');
    expect(screen.getByTestId('guest-technical-details')).not.toHaveAttribute('open');
    const guidance = screen.getByRole('link', { name: 'Filesystem reading guidance' });
    expect(guidance.closest('details')).toBeNull();
    expect(guidance).toHaveClass('min-h-11', 'sm:min-h-0', 'text-base-content');
    expect(guidance).toHaveClass('focus-visible:outline-2');
    expect(guidance).toHaveAttribute(
      'href',
      '/docs/VM_DISK_MONITORING#a-missing-reading-is-not-an-installation-diagnosis',
    );
  });

  it.each(guestDiskDeferrals)(
    'explains %s in the existing drawer even before any disk sample',
    (reason, message) => {
      render(() => (
        <GuestDrawerOverview
          {...props}
          hasFilesystemDetails={false}
          guest={{ ...props.guest, type: 'qemu', disks: [], diskStatusReason: reason }}
        />
      ));
      expect(screen.getByText('Filesystems')).toBeInTheDocument();
      expect(screen.getByText(message)).toBeInTheDocument();
      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
      expect(screen.queryByText(/Using last known/)).not.toBeInTheDocument();
    },
  );

  it('retains disk rows with readable status, then withdraws the notice on fresh resumption', () => {
    const [guest, setGuest] = createSignal({
      ...props.guest,
      type: 'qemu' as const,
      diskStatusReason: 'prev-vm-locked',
    });
    render(() => <GuestDrawerOverview {...props} guest={guest()} />);
    expect(
      screen.getByText(/^Using last known disk stats\. Guest reads paused/).closest('td'),
    ).toHaveAttribute('colspan', '2');
    expect(screen.queryByText('prev-vm-locked')).not.toBeInTheDocument();
    for (const path of paths) expect(screen.getByText(path)).toBeInTheDocument();
    expect(screen.getByText('Last known 50% · 5.00 GB/10.0 GB · MP0')).toBeVisible();
    expect(screen.getByText('Usage unavailable · ?/10.0 GB · MP1')).toBeVisible();
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    setGuest({
      ...guest(),
      diskStatusReason: '',
      disks: guest().disks!.map((disk) => ({ ...disk, used: (disk.total ?? 0) * 0.75, usage: 75 })),
    });
    expect(screen.queryByText(/Using last known/)).not.toBeInTheDocument();
    expect(screen.queryByText('Status')).not.toBeInTheDocument();
    expect(
      screen.getByRole('progressbar', { name: `Filesystem ${paths[0]} utilization` }),
    ).toHaveAttribute('aria-valuenow', '75');
  });

  it.each(['paused', 'suspended'])(
    'labels a %s VM filesystem as last known without relying on a read failure',
    (status) => {
      const [guest, setGuest] = createSignal({ ...props.guest, type: 'qemu' as const });
      render(() => <GuestDrawerOverview {...props} guest={guest()} />);
      expect(screen.getByRole('progressbar')).toBeInTheDocument();
      setGuest({ ...guest(), status });
      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
      expect(screen.getByText('Last known 50% · 5.00 GB/10.0 GB · MP0')).toBeVisible();
      expect(
        screen.getByText(
          `Guest filesystem readings are not current while the VM is ${status}. Any displayed values are last known.`,
        ),
      ).toBeVisible();
      for (const path of paths) expect(screen.getByText(path)).toBeInTheDocument();
      // A new running report with a measured zero can still be current.
      setGuest({
        ...guest(),
        status: 'running',
        disks: [{ ...guest().disks![0], used: 0, usage: 0 }],
      });
      expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0');
      expect(screen.queryByText(/Any displayed values are last known/)).not.toBeInTheDocument();
    },
  );

  it('shows linked-agent RAID arrays beside the guest storage evidence', () => {
    render(() => (
      <GuestDrawerOverview
        {...props}
        guest={{
          ...props.guest,
          agentRaid: [
            {
              device: '/dev/md0',
              name: 'data',
              level: 'raid1',
              state: 'clean',
              totalDevices: 2,
              activeDevices: 2,
              workingDevices: 2,
              failedDevices: 0,
              spareDevices: 0,
            },
          ],
        }}
      />
    ));
    expect(screen.getByText('Guest RAID')).toBeInTheDocument();
    expect(screen.getByText('data')).toBeInTheDocument();
    expect(screen.getByText('raid1')).toBeInTheDocument();
    expect(screen.getByText('clean')).toBeInTheDocument();
  });

  it('shows shared-prefix paths above usage, including unknown usage, without relying on hover', () => {
    render(() => <GuestDrawerOverview {...props} />);
    for (const path of paths) {
      const label = screen.getByText(path);
      expect(label.tagName).toBe('SPAN');
      expect(label).toHaveClass('whitespace-normal', 'wrap-anywhere');
      expect(label.closest('td')).toHaveAttribute('colspan', '2');
    }
    expect(
      screen.getByRole('progressbar', { name: `Filesystem ${paths[0]} utilization` }),
    ).toHaveAttribute('aria-valuenow', '50');
    expect(
      screen.queryByRole('progressbar', { name: `Filesystem ${paths[1]} utilization` }),
    ).toBeNull();
    expect(screen.getByText(/— · \?\/10.0 GB · MP1/)).toBeInTheDocument();
    expect(screen.getByText('CPUs').closest('tr')).toHaveClass('lg:grid-cols-[7rem_minmax(0,1fr)]');
  });
});
