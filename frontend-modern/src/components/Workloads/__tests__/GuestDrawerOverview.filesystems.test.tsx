import { cleanup, render, screen } from '@solidjs/testing-library';
import type { ComponentProps } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { GuestDrawerOverview } from '../GuestDrawerOverview';

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

  it('shows shared-prefix paths above usage, including unknown usage, without relying on hover', () => {
    render(() => <GuestDrawerOverview {...props} />);
    for (const path of paths) {
      const label = screen.getByText(path);
      expect(label.tagName).toBe('SPAN');
      expect(label).toHaveClass('whitespace-normal', '[overflow-wrap:anywhere]');
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
    expect(screen.getByText('Values').closest('tr')).toHaveClass(
      'lg:grid-cols-[7rem_minmax(0,1fr)]',
    );
  });
});
