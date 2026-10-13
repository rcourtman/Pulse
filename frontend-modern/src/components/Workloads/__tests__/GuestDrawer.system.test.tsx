import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { GuestDrawer } from '../GuestDrawer';
import type { WorkloadGuest } from '@/types/workloads';

const guest = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest => ({
  id: 'lab-pve-a-101',
  name: 'system-reading-fixture',
  vmid: 101,
  instance: 'lab',
  node: 'pve-a',
  type: 'qemu',
  status: 'running',
  cpu: 0.1,
  cpus: 2,
  memory: { total: 1024, used: 256, free: 768, usage: 25 },
  disk: { total: 1024, used: 256, free: 768, usage: 25 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  tags: [],
  lastBackup: 0,
  lock: '',
  lastSeen: '2026-10-12T20:00:00Z',
  ...overrides,
});

// Upstream keeps the full record in a collapsed, mounted Details section.
// Exercise its native open state before judging the retained System row.
const systemRow = () => {
  const details = screen.getByTestId('guest-technical-details') as HTMLDetailsElement;
  details.open = true;
  return within(details).getByText('System', { selector: 'td' }).closest('tr')!;
};

afterEach(cleanup);

describe('readable guest System evidence', () => {
  it.each(['qemu', 'lxc'] as const)(
    'wraps the full %s name and version in the real drawer',
    (type) => {
      render(() => (
        <GuestDrawer
          guest={guest({
            type,
            osName: 'Microsoft Windows Server 2025 Datacenter Evaluation',
            osVersion: '10.0.26100.1742 (64-bit)',
          })}
          onClose={() => {}}
        />
      ));
      const value = within(systemRow()).getByText(
        'Microsoft Windows Server 2025 Datacenter Evaluation • 10.0.26100.1742 (64-bit)',
      );
      expect(value).toBeVisible();
      expect(value).toHaveClass('whitespace-normal', 'wrap-break-word');
      expect(value).not.toHaveClass('truncate');
      expect(systemRow().querySelector('button, [tabindex]')).toBeNull();
    },
  );

  it.each([
    ['Debian GNU/Linux', '', 'Debian GNU/Linux'],
    ['', '13.7 (trixie)', '13.7 (trixie)'],
  ])(
    'keeps an observed name or version without inventing the missing part',
    (osName, osVersion, summary) => {
      render(() => <GuestDrawer guest={guest({ osName, osVersion })} onClose={() => {}} />);
      expect(within(systemRow()).getByText(summary)).toBeVisible();
    },
  );

  it('adds, replaces and removes System evidence while preserving the mounted drawer', () => {
    const [value, setValue] = createSignal(guest());
    render(() => <GuestDrawer guest={value()} onClose={() => {}} />);
    const drawer = screen.getByTestId('guest-technical-details');
    expect(screen.queryByText('System', { selector: 'td' })).not.toBeInTheDocument();
    setValue({ ...value(), osName: 'Debian GNU/Linux', osVersion: '13.7 (trixie)' });
    expect(within(systemRow()).getByText('Debian GNU/Linux • 13.7 (trixie)')).toBeVisible();
    setValue({ ...value(), osName: 'Ubuntu', osVersion: '24.04.3 LTS' });
    expect(within(systemRow()).getByText('Ubuntu • 24.04.3 LTS')).toBeVisible();
    expect(screen.queryByText('Debian GNU/Linux • 13.7 (trixie)')).not.toBeInTheDocument();
    setValue({ ...value(), osName: '', osVersion: '' });
    expect(screen.queryByText('System', { selector: 'td' })).not.toBeInTheDocument();
    expect(screen.getByTestId('guest-technical-details')).toBe(drawer);
  });

  it('does not turn a retained OS string into current guest-agent availability', () => {
    render(() => (
      <GuestDrawer
        guest={guest({ osName: 'Windows', osVersion: '2025', guestAgentStatus: 'not-running' })}
        onClose={() => {}}
      />
    ));
    expect(within(systemRow()).getByText('Windows • 2025')).toBeVisible();
    expect(screen.getByText('Not running')).toBeVisible();
    expect(screen.queryByText('Reported available')).not.toBeInTheDocument();
  });
});
