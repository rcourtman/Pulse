import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

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

afterEach(cleanup);

describe('NodeDrawerOverview update evidence', () => {
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

    expect(screen.getByText('Unavailable · Sys.Audit permission required')).toBeInTheDocument();
  });
});
