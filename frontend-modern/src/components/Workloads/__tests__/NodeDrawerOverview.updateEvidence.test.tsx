import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { NodeDrawerOverview } from '@/components/Workloads/NodeDrawerOverview';
import type { Node } from '@/types/api';

const makeNode = (overrides: Partial<Node>): Node =>
  ({
    id: 'lab-pve1',
    name: 'pve1',
    instance: 'lab',
    status: 'online',
    type: 'node',
    cpu: 0,
    memory: { total: 1024, used: 256, free: 768, usage: 25 },
    disk: { total: 1024, used: 256, free: 768, usage: 25 },
    uptime: 3600,
    loadAverage: [],
    kernelVersion: '6.8.12',
    pveVersion: 'pve-manager/9.0.1',
    cpuInfo: { model: 'CPU', cores: 4, sockets: 1, mhz: '2400' },
    lastSeen: '2026-08-29T12:00:00Z',
    connectionHealth: 'healthy',
    ...overrides,
  }) as Node;

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('NodeDrawerOverview hardware', () => {
  it('shows physical port names, speeds and link states while Details is closed', () => {
    render(() => (
      <NodeDrawerOverview
        node={makeNode({
          cpuInfo: { model: 'Intel Core i5-10600', cores: 12, sockets: 1, mhz: '3300' },
          memory: { total: 15.3 * 1024 ** 3, used: 0, free: 0, usage: 0 },
          sensors: { fanRpm: { fan1: 1400 } } as Node['sensors'],
          networkInterfaces: [
            { name: 'eno1', addresses: [], speedMbps: 2500, linkUp: true },
            { name: 'eno2', addresses: [], speedMbps: 1000, linkUp: false },
            { name: 'enp3s0', addresses: [], linkUp: true },
            { name: 'enp4s0', addresses: [], speedMbps: 100 },
            { name: 'wlp2s0', addresses: [] },
            { name: 'vmbr0', addresses: ['192.168.0.10/24'] },
          ],
        })}
      />
    ));

    expect(screen.getByTestId('node-technical-details')).not.toHaveAttribute('open');
    const inventory = within(screen.getByTestId('node-drawer-hardware'));
    expect(inventory.getByText('Intel Core i5-10600 · 12 threads')).toBeVisible();
    expect(inventory.getByText('16 GB')).toBeVisible();
    expect(inventory.getByRole('row', { name: 'Fans reported 1' })).toBeVisible();
    for (const [port, details] of [
      ['eno1', 'Link up · 2.5 Gb/s'],
      ['eno2', 'No link'],
      ['enp3s0', 'Link up · Speed unavailable'],
      ['enp4s0', 'Link status unknown · 100 Mb/s'],
      ['wlp2s0 (Wi-Fi)', 'Link status unknown · Speed unavailable'],
    ]) {
      const label = inventory.getByText(port, { exact: true });
      expect(label).toBeVisible();
      expect(label.closest('tr')).toHaveTextContent(details);
    }
    expect(inventory.queryByText('vmbr0')).not.toBeInTheDocument();
  });

  it('qualifies retained link readings when the agent stops reporting', () => {
    render(() => (
      <NodeDrawerOverview
        node={makeNode({
          sensorsStale: true,
          networkInterfaces: [
            { name: 'eno1', addresses: [], speedMbps: 2500, linkUp: true },
            { name: 'eno2', addresses: [], linkUp: false },
            { name: 'enp3s0', addresses: [] },
          ],
        })}
      />
    ));

    const inventory = within(screen.getByTestId('node-drawer-hardware'));
    expect(inventory.getByText('Last known · Link up · 2.5 Gb/s')).toBeVisible();
    expect(inventory.getByText('Last known · No link')).toBeVisible();
    expect(inventory.getByText('Link status unknown · Speed unavailable')).toBeVisible();
  });

  it('omits hardware inventory when the node only reports virtual interfaces', () => {
    render(() => (
      <NodeDrawerOverview
        node={makeNode({
          cpuInfo: { model: 'Unknown', cores: 0, sockets: 0, mhz: '0' },
          memory: { total: 0, used: 0, free: 0, usage: 0 },
          networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.0.10/24'] }],
        })}
      />
    ));

    expect(screen.queryByTestId('node-drawer-hardware')).not.toBeInTheDocument();
  });
});

describe('NodeDrawerOverview update evidence', () => {
  it.each(['checked', 'stale'] as const)(
    'ages %s evidence without inventing a new check or changing its verdict',
    (status) => {
      const start = Date.parse('2026-10-10T12:00:00Z');
      vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });
      const node = makeNode({
        pendingUpdates: 0,
        pendingUpdatesStatus: status,
        pendingUpdatesCheckedAt: new Date(start - 60_000).toISOString(),
        ...(status === 'stale' ? { pendingUpdatesReason: 'check_failed' as const } : {}),
      });
      render(() => <NodeDrawerOverview node={node} />);
      // The alert-first Overview demotes healthy inventory, not warning rows.
      (screen.getByTestId('node-technical-details') as HTMLDetailsElement).open = true;
      expect(screen.getByText(/No pending updates.*checked 1 min ago/)).toBeInTheDocument();
      vi.advanceTimersByTime(60_000);
      expect(screen.getByText(/No pending updates.*checked 2 mins ago/)).toHaveAttribute(
        'title',
        status === 'checked'
          ? 'Last successful update check 2 mins ago'
          : 'Latest check failed · last success 2 mins ago',
      );
      expect(node.pendingUpdatesStatus).toBe(status);
      expect(node.pendingUpdatesCheckedAt).toBe(new Date(start - 60_000).toISOString());
    },
  );
  it('shows confirmed zero evidence instead of omitting the update row', () => {
    render(() => (
      <NodeDrawerOverview
        node={makeNode({
          pendingUpdates: 0,
          pendingUpdatesStatus: 'checked',
          pendingUpdatesCheckedAt: new Date().toISOString(),
        })}
      />
    ));

    (screen.getByTestId('node-technical-details') as HTMLDetailsElement).open = true;
    expect(screen.getByText('Updates')).toBeInTheDocument();
    expect(screen.getByText(/No pending updates · checked/)).toBeInTheDocument();
  });

  it('explains why update evidence is unavailable', () => {
    render(() => (
      <NodeDrawerOverview
        node={makeNode({
          pendingUpdatesStatus: 'unavailable',
          pendingUpdatesReason: 'permission_denied',
        })}
      />
    ));

    const status = screen.getByText('Unavailable · Update check access denied');
    expect(screen.getByTestId('node-technical-details')).not.toHaveAttribute('open');
    const details = screen.getByTestId('node-technical-details') as HTMLDetailsElement;
    expect(status.closest('details')).toBe(details);
    expect(status).not.toBeVisible();
    details.open = true;
    expect(status).toBeVisible();
    details.open = false;
    expect(status).toHaveClass('whitespace-normal');
    expect(status).not.toHaveClass('truncate');
    expect(screen.getByText(/Other monitoring can still work/)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Why update checks are unavailable' }).closest('details'),
    ).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Why update checks are unavailable' }));
    const help = screen.getByRole('dialog', { name: 'Proxmox update-check access' });
    expect(help).toHaveTextContent('Sys.Modify');
    expect(help).toHaveTextContent('broader than read-only monitoring');
    expect(help).toHaveTextContent('does not identify the exact missing privilege');
    expect(screen.getByRole('link', { name: 'Learn more' })).toHaveAttribute(
      'href',
      '/docs/TROUBLESHOOTING#proxmox-pending-update-access',
    );
  });

  it('keeps guidance open while node readings and evidence age update', () => {
    const start = Date.parse('2026-10-10T12:00:00Z');
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });
    const [node, setNode] = createSignal(
      makeNode({
        pendingUpdates: 7,
        pendingUpdatesStatus: 'stale',
        pendingUpdatesReason: 'permission_denied',
        pendingUpdatesCheckedAt: new Date(start - 60_000).toISOString(),
      }),
    );
    render(() => <NodeDrawerOverview node={node()} />);
    fireEvent.click(screen.getByRole('button', { name: 'Why update checks are unavailable' }));
    const help = screen.getByRole('dialog', { name: 'Proxmox update-check access' });
    const trigger = screen.getByRole('button', { name: 'Why update checks are unavailable' });
    const details = screen.getByTestId('node-technical-details') as HTMLDetailsElement;
    details.open = true;
    details.open = false;
    setNode({ ...node(), cpu: 0.2 });
    vi.advanceTimersByTime(60_000);
    expect(screen.getByRole('dialog', { name: 'Proxmox update-check access' })).toBe(help);
    expect(screen.getByRole('button', { name: 'Why update checks are unavailable' })).toBe(trigger);
    expect(screen.getByText(/7 pending.*stale.*checked 2 mins ago/)).toBeInTheDocument();
    setNode({ ...node(), pendingUpdatesStatus: 'unavailable' });
    expect(screen.getByRole('dialog', { name: 'Proxmox update-check access' })).toBe(help);
    expect(screen.getByTestId('node-technical-details')).toBe(details);
    expect(screen.queryByTestId('node-drawer-exceptions')).toBeNull();
    const status = screen.getByText('Unavailable · Update check access denied');
    expect(status).not.toBeVisible();
    details.open = true;
    expect(status).toBeVisible();
    expect(screen.getByRole('dialog', { name: 'Proxmox update-check access' })).toBe(help);
  });

  it('keeps a denied update check in Details, not under Worth checking', () => {
    render(() => (
      <NodeDrawerOverview
        node={makeNode({
          pendingUpdatesStatus: 'unavailable',
          pendingUpdatesReason: 'permission_denied',
        })}
      />
    ));
    expect(screen.queryByTestId('node-drawer-exceptions')).toBeNull();
    // Details shows the whole reason, wrapped, not cut off behind an ellipsis.
    expect(screen.getByText('Unavailable · Update check access denied')).toHaveClass(
      'whitespace-normal',
    );
    expect(screen.getByTestId('node-technical-details').textContent).toContain(
      'Unavailable · Update check access denied',
    );
  });

  it('lists an update check that failed under Worth checking', () => {
    render(() => (
      <NodeDrawerOverview
        node={makeNode({
          pendingUpdatesStatus: 'unavailable',
          pendingUpdatesReason: 'source_unavailable',
        })}
      />
    ));
    expect(screen.getByTestId('node-drawer-exceptions').textContent).toContain(
      'Unavailable · Proxmox source unavailable',
    );
  });
});
