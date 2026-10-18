import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
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

    fireEvent.click(screen.getByTestId('node-technical-details').querySelector('summary')!);
    const status = screen.getByText('Unavailable · Update check access denied');
    expect(status).toHaveClass('whitespace-normal');
    expect(status).not.toHaveClass('truncate');
    expect(screen.getByText(/Other monitoring can still work/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Why update checks are unavailable' }));
    const help = screen.getByRole('dialog', { name: 'Proxmox update-check access' });
    expect(help).toHaveTextContent('Sys.Modify');
    expect(help).toHaveTextContent('broader than read-only monitoring');
    expect(help).toHaveTextContent('does not identify the exact missing privilege');
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
    fireEvent.click(screen.getByTestId('node-technical-details').querySelector('summary')!);
    fireEvent.click(screen.getByRole('button', { name: 'Why update checks are unavailable' }));
    const help = screen.getByRole('dialog', { name: 'Proxmox update-check access' });
    setNode({ ...node(), cpu: 0.2 });
    vi.advanceTimersByTime(60_000);
    expect(screen.getByRole('dialog', { name: 'Proxmox update-check access' })).toBe(help);
    expect(screen.getByText(/7 pending.*stale.*checked 2 mins ago/)).toBeInTheDocument();
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
