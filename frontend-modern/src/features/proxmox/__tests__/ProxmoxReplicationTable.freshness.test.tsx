import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReplicationJob } from '@/types/api';
import type { Resource } from '@/types/resource';
import { ProxmoxReplicationTable, getReplicationJobStaleNotice } from '../ProxmoxReplicationTable';

type ReplicationJobWithPoll = ReplicationJob & { lastPolled?: number | string };

const makeNode = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'pve:pve-a',
    name: 'pve-a',
    displayName: 'pve-a',
    type: 'agent',
    platformType: 'proxmox-pve',
    platformId: 'site-a',
    sourceType: 'api',
    sources: ['proxmox'],
    status: 'online',
    lastSeen: Date.parse('2026-11-27T11:59:30Z'),
    proxmox: {
      instance: 'site-a',
      nodeName: 'pve-a',
      connectionHealth: 'healthy',
    },
    sourceStatus: {
      proxmox: {
        status: 'online',
        lastSeen: '2026-11-27T11:59:30Z',
      },
    },
    health: {
      verdict: 'ok',
      reasons: [],
    },
    ...overrides,
  }) as Resource;

const makeJob = (overrides: Partial<ReplicationJobWithPoll> = {}): ReplicationJobWithPoll => ({
  id: 'site-a-100-0',
  instance: 'site-a',
  jobId: '100-0',
  guestId: 100,
  guestName: 'billing-db',
  sourceNode: 'pve-a',
  targetNode: 'pve-b',
  schedule: '*/15',
  enabled: true,
  lastSyncStatus: 'ok',
  lastSyncUnix: Date.parse('2026-11-27T11:40:00Z') / 1000,
  nextSyncUnix: Date.parse('2026-11-27T11:55:00Z') / 1000,
  lastSyncDurationSeconds: 42,
  failCount: 0,
  lastPolled: '2026-11-27T11:42:00Z',
  ...overrides,
});

const mount = (jobs: () => ReplicationJob[], nodes: () => Resource[] | undefined) =>
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => (
          <ProxmoxReplicationTable
            jobs={jobs()}
            nodes={nodes()}
            error={undefined}
            onRetry={() => undefined}
            emptyIcon={<span />}
            emptyTitle="No replication jobs"
            emptyDescription="Replication jobs appear here."
          />
        )}
      />
    </Router>
  ));

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('getReplicationJobStaleNotice', () => {
  it('returns null when owning PVE node is online and fresh', () => {
    expect(getReplicationJobStaleNotice(makeJob(), [makeNode()])).toBeNull();
  });

  it('surfaces telemetry_stale age and Proxmox source errors for the source node', () => {
    const staleNode = makeNode({
      status: 'warning',
      sourceStatus: {
        proxmox: {
          status: 'stale',
          lastSeen: '2026-11-27T11:42:00Z',
          error: 'API timeout',
        },
      },
      health: {
        verdict: 'stale',
        reasons: [{ code: 'telemetry_stale', detail: '18m' }],
      },
    });
    const notice = getReplicationJobStaleNotice(makeJob(), [staleNode]);
    expect(notice).toEqual({
      badge: 'Stale data',
      compactBadge: 'Stale',
      detail:
        'No Proxmox report from pve-a for 18m (API timeout) — showing the last values received.',
      error: 'API timeout',
    });
  });

  it('keeps a fresh peer node fresh when another cluster node is stale', () => {
    const staleA = makeNode({
      id: 'pve:pve-a',
      name: 'pve-a',
      proxmox: { instance: 'site-a', nodeName: 'pve-a', connectionHealth: 'stale' },
      sourceStatus: { proxmox: { status: 'stale', lastSeen: '2026-11-27T11:40:00Z' } },
      health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '20m' }] },
    });
    const freshB = makeNode({
      id: 'pve:pve-b',
      name: 'pve-b',
      proxmox: { instance: 'site-a', nodeName: 'pve-b', connectionHealth: 'healthy' },
    });

    expect(
      getReplicationJobStaleNotice(makeJob({ sourceNode: 'pve-a' }), [staleA, freshB]),
    ).not.toBeNull();
    expect(
      getReplicationJobStaleNotice(makeJob({ sourceNode: 'pve-b' }), [staleA, freshB]),
    ).toBeNull();
  });

  it('flags a job when no source node matches and every instance node is stale', () => {
    const staleA = makeNode({
      id: 'pve:pve-a',
      name: 'pve-a',
      proxmox: { instance: 'site-a', nodeName: 'pve-a', connectionHealth: 'error' },
      sourceStatus: {
        proxmox: {
          status: 'error',
          lastSeen: '2026-11-27T11:40:00Z',
          error: '502 Bad Gateway',
        },
      },
    });
    const staleB = makeNode({
      id: 'pve:pve-b',
      name: 'pve-b',
      proxmox: { instance: 'site-a', nodeName: 'pve-b', connectionHealth: 'stale' },
      sourceStatus: { proxmox: { status: 'stale', lastSeen: '2026-11-27T11:40:00Z' } },
    });
    const notice = getReplicationJobStaleNotice(makeJob({ sourceNode: 'pve-missing' }), [
      staleA,
      staleB,
    ]);
    expect(notice?.detail).toContain('site-a');
    expect(notice?.detail).toContain('502 Bad Gateway');
  });

  it('catches stale Proxmox source even when a linked Pulse agent keeps node.status online', () => {
    const linkedOnlineButPveStale = makeNode({
      status: 'online',
      sources: ['agent', 'proxmox'],
      proxmox: { instance: 'site-a', nodeName: 'pve-a', connectionHealth: 'error' },
      sourceStatus: {
        agent: { status: 'online', lastSeen: '2026-11-27T11:59:50Z' },
        proxmox: { status: 'stale', lastSeen: '2026-11-27T11:40:00Z' },
      },
      health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '20m' }] },
    });
    expect(getReplicationJobStaleNotice(makeJob(), [linkedOnlineButPveStale])?.detail).toBe(
      'Proxmox API poll failed on pve-a (last report 20m ago) — showing the last values received.',
    );
  });
});

describe('ProxmoxReplicationTable stale-reading presentation', () => {
  const nowMs = Date.parse('2026-11-27T12:00:00Z');

  it('moves stale healthy jobs to Stale, keeps Failed jobs Failed, and clears on recovery', async () => {
    vi.spyOn(Date, 'now').mockReturnValue(nowMs);
    const staleNodeA = makeNode({
      id: 'pve:pve-a',
      name: 'pve-a',
      status: 'warning',
      proxmox: { instance: 'site-a', nodeName: 'pve-a', connectionHealth: 'stale' },
      sourceStatus: {
        proxmox: { status: 'stale', lastSeen: '2026-11-27T11:42:00Z' },
      },
      health: {
        verdict: 'stale',
        reasons: [{ code: 'telemetry_stale', detail: '18m' }],
      },
    });
    const freshNodeB = makeNode({
      id: 'pve:pve-b',
      name: 'pve-b',
      proxmox: { instance: 'site-a', nodeName: 'pve-b', connectionHealth: 'healthy' },
    });

    const jobs: ReplicationJobWithPoll[] = [
      makeJob({
        id: 'site-a-100-0',
        jobId: '100-0',
        guestId: 100,
        guestName: 'billing-db',
        sourceNode: 'pve-a',
        targetNode: 'pve-b',
        lastSyncStatus: 'ok',
        failCount: 0,
        lastPolled: '2026-11-27T11:42:00Z',
      }),
      makeJob({
        id: 'site-a-101-0',
        jobId: '101-0',
        guestId: 101,
        guestName: 'worker-queue',
        sourceNode: 'pve-a',
        targetNode: 'pve-b',
        lastSyncStatus: 'error',
        failCount: 2,
        error: 'zfs send failed: exit code 255',
        lastPolled: '2026-11-27T11:42:00Z',
      }),
      makeJob({
        id: 'site-a-102-0',
        jobId: '102-0',
        guestId: 102,
        guestName: 'edge-proxy',
        sourceNode: 'pve-b',
        targetNode: 'pve-a',
        lastSyncStatus: 'ok',
        failCount: 0,
        lastPolled: '2026-11-27T11:59:30Z',
      }),
    ];

    const [nodeSignal, setNodeSignal] = createSignal<Resource[]>([staleNodeA, freshNodeB]);
    mount(() => jobs, nodeSignal);

    // Filter chips separate fresh Healthy (1), Stale (1), and Failed (1).
    expect(screen.getByRole('button', { name: 'All, 3' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Healthy, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Stale, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Failed, 1' })).toBeInTheDocument();

    const billingRow = screen.getByText('100 (billing-db)').closest('tr')!;
    expect(within(billingRow).getByText('Stale data')).toBeInTheDocument();
    expect(within(billingRow).queryByText('Healthy')).not.toBeInTheDocument();

    const failedRow = screen.getByText('101 (worker-queue)').closest('tr')!;
    expect(within(failedRow).getByText('Failed')).toBeInTheDocument();
    expect(within(failedRow).getByText('Stale')).toBeInTheDocument();

    const freshRow = screen.getByText('102 (edge-proxy)').closest('tr')!;
    expect(within(freshRow).getByText('Healthy')).toBeInTheDocument();
    expect(within(freshRow).queryByText(/Stale/)).not.toBeInTheDocument();

    // Expand stale healthy job drawer: warning notice, Last reported, Last polled.
    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for replication job 100-0' }),
    );
    const panel = screen.getByTestId('replication-job-detail');
    expect(within(panel).getByRole('img', { name: 'Stale data' })).toBeInTheDocument();
    expect(within(panel).getByTestId('replication-job-stale-notice')).toHaveTextContent(
      'Stale data. No Proxmox report from pve-a for 18m — showing the last values received.',
    );
    expect(within(panel).getByRole('row', { name: /^Last reported / })).toHaveTextContent(
      'Healthy',
    );
    expect(within(panel).getByRole('row', { name: /^Last polled / })).toHaveTextContent('18m ago');

    // Searching "stale" matches both stale jobs (Stale data and Failed + Stale), not fresh peer.
    const searchInput = screen.getByPlaceholderText('Search jobs, guests, nodes');
    fireEvent.input(searchInput, { target: { value: 'stale' } });
    expect(screen.getByRole('button', { name: 'All, 2' })).toBeInTheDocument();
    expect(screen.getAllByText('100 (billing-db)').length).toBeGreaterThan(0);
    expect(screen.getByText('101 (worker-queue)')).toBeInTheDocument();
    expect(screen.queryByText('102 (edge-proxy)')).not.toBeInTheDocument();
    fireEvent.input(searchInput, { target: { value: '' } });

    // Select Stale chip, then recover pve-a live: chip stays mounted with focus and count 0,
    // and open drawer clears its stale banner.
    const staleChip = screen.getByRole('button', { name: 'Stale, 1' });
    staleChip.focus();
    await fireEvent.click(staleChip);
    expect(screen.getAllByText('100 (billing-db)').length).toBeGreaterThan(0);
    expect(screen.queryByText('102 (edge-proxy)')).not.toBeInTheDocument();

    const recoveredNodeA = makeNode({
      id: 'pve:pve-a',
      name: 'pve-a',
      proxmox: { instance: 'site-a', nodeName: 'pve-a', connectionHealth: 'healthy' },
    });
    setNodeSignal([recoveredNodeA, freshNodeB]);

    expect(screen.getByRole('button', { name: 'Stale, 0' })).toBe(staleChip);
    expect(staleChip).toHaveFocus();
    expect(screen.getByRole('button', { name: 'Healthy, 2' })).toBeInTheDocument();
    expect(screen.queryByTestId('replication-job-stale-notice')).not.toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'All, 3' }));
    expect(screen.queryByRole('button', { name: /^Stale,/ })).not.toBeInTheDocument();
    expect(
      within(screen.getAllByText('100 (billing-db)')[0].closest('tr')!).getByText('Healthy'),
    ).toBeInTheDocument();
  });

  it('uses compact Stale label in phone layout without losing Failed visibility', () => {
    vi.spyOn(Date, 'now').mockReturnValue(nowMs);
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(360);
    const staleNodeA = makeNode({
      status: 'warning',
      proxmox: { instance: 'site-a', nodeName: 'pve-a', connectionHealth: 'stale' },
      sourceStatus: { proxmox: { status: 'stale', lastSeen: '2026-11-27T11:42:00Z' } },
      health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '18m' }] },
    });

    mount(
      () => [
        makeJob({ id: 'site-a-100-0', jobId: '100-0', guestId: 100, guestName: 'billing-db' }),
        makeJob({
          id: 'site-a-101-0',
          jobId: '101-0',
          guestId: 101,
          guestName: 'worker-queue',
          lastSyncStatus: 'error',
          failCount: 2,
        }),
      ],
      () => [staleNodeA],
    );

    const billingRow = screen.getByText('100 billing-db').closest('tr')!;
    expect(within(billingRow).getByText('Stale')).toBeInTheDocument();
    expect(within(billingRow).queryByText('Stale data')).not.toBeInTheDocument();

    const failedRow = screen.getByText('101 worker-queue').closest('tr')!;
    expect(within(failedRow).getByText('Failed')).toBeInTheDocument();
    expect(within(failedRow).getByText('Stale')).toBeInTheDocument();
  });
});
