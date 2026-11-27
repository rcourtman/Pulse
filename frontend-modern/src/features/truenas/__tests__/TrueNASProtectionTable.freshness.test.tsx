import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal, type JSX } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import { TrueNASProtectionTable } from '@/features/truenas/TrueNASProtectionTable';
import {
  buildTrueNASProtectionPosture,
  buildTrueNASProtectionStaleNoticeMap,
  filterTrueNASProtectionPoints,
  getTrueNASProtectionStaleNotice,
  sortTrueNASProtectionPoints,
} from '@/features/truenas/truenasPageModel';
import type { RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';

const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

const makeSystem = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'agent',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformId: 'truenas-primary',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    sourceType: 'agent',
    sources: ['agent', 'truenas'],
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

const makePoint = (
  point: Partial<RecoveryPoint> & Pick<RecoveryPoint, 'id' | 'kind' | 'mode'>,
): RecoveryPoint => ({
  outcome: 'success',
  platform: 'truenas',
  startedAt: '2026-11-27T12:00:00Z',
  completedAt: '2026-11-27T12:05:00Z',
  ...point,
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe('TrueNAS protection freshness model', () => {
  it('isolates DR clones sharing a hostname by connectionId so healthy peers never turn stale', () => {
    const stalePrimary = makeSystem({
      id: 'truenas:agent:system:conn-primary',
      name: 'truenas',
      displayName: 'truenas-primary',
      platformId: 'conn-primary',
      agent: { agentId: 'conn-primary', hostname: 'truenas' },
      truenas: { hostname: 'truenas' },
      sourceStatus: {
        truenas: { status: 'error', error: 'websocket read timeout' },
      },
      health: {
        verdict: 'stale',
        reasons: [{ code: 'telemetry_stale', detail: '18m' }],
      },
    });
    const healthyDr = makeSystem({
      id: 'truenas:agent:system:conn-dr',
      name: 'truenas',
      displayName: 'truenas-dr',
      platformId: 'conn-dr',
      agent: { agentId: 'conn-dr', hostname: 'truenas' },
      truenas: { hostname: 'truenas' },
      sourceStatus: {
        truenas: { status: 'online' },
      },
    });

    const primarySnap = makePoint({
      id: 'snap-primary',
      kind: 'snapshot',
      mode: 'snapshot',
      details: { connectionId: 'conn-primary', hostname: 'truenas', dataset: 'tank/apps' },
    });
    const drSnap = makePoint({
      id: 'snap-dr',
      kind: 'snapshot',
      mode: 'snapshot',
      details: { connectionId: 'conn-dr', hostname: 'truenas', dataset: 'vault/apps' },
    });
    const unknownConnSnap = makePoint({
      id: 'snap-unknown',
      kind: 'snapshot',
      mode: 'snapshot',
      details: { connectionId: 'conn-other', hostname: 'truenas', dataset: 'other/apps' },
    });

    const primaryNotice = getTrueNASProtectionStaleNotice(primarySnap, [stalePrimary, healthyDr]);
    expect(primaryNotice).toEqual(
      expect.objectContaining({
        badge: 'Stale data',
        systemName: 'truenas-primary',
        age: '18m',
        detail: 'TrueNAS collection error: websocket read timeout (last report 18m ago)',
      }),
    );
    expect(getTrueNASProtectionStaleNotice(drSnap, [stalePrimary, healthyDr])).toBeNull();
    expect(getTrueNASProtectionStaleNotice(unknownConnSnap, [stalePrimary, healthyDr])).toBeNull();
  });

  it('does not treat a live degraded ZFS pool as stale when TrueNAS polling is healthy', () => {
    const degradedLiveSystem = makeSystem({
      id: 'truenas:agent:system:conn-live',
      name: 'nas-live',
      status: 'degraded',
      agent: { agentId: 'conn-live', hostname: 'nas-live' },
      truenas: { hostname: 'nas-live' },
      sourceStatus: { truenas: { status: 'online' } },
    });
    const snap = makePoint({
      id: 'snap-live',
      kind: 'snapshot',
      mode: 'snapshot',
      details: { connectionId: 'conn-live', hostname: 'nas-live', dataset: 'tank/media' },
    });

    expect(getTrueNASProtectionStaleNotice(snap, [degradedLiveSystem])).toBeNull();
  });

  it('detects backend telemetry_stale and offline systems even when sourceStatus is absent', () => {
    const telemetryStale = makeSystem({
      id: 'agent:nas-telemetry',
      name: 'nas-telemetry',
      truenas: { hostname: 'nas-telemetry' },
      health: {
        verdict: 'stale',
        reasons: [{ code: 'telemetry_stale', detail: '24m' }],
      },
    });
    const offlineSystem = makeSystem({
      id: 'agent:nas-offline',
      name: 'nas-offline',
      status: 'offline',
      truenas: { hostname: 'nas-offline' },
    });

    const telemetryPoint = makePoint({
      id: 'pt-telemetry',
      kind: 'snapshot',
      mode: 'snapshot',
      display: { nodeHostLabel: 'nas-telemetry', itemLabel: 'tank/db' },
    });
    const offlinePoint = makePoint({
      id: 'pt-offline',
      kind: 'snapshot',
      mode: 'snapshot',
      details: { hostname: 'nas-offline', dataset: 'tank/backups' },
    });

    expect(getTrueNASProtectionStaleNotice(telemetryPoint, [telemetryStale, offlineSystem])).toEqual(
      expect.objectContaining({
        badge: 'Stale data',
        systemName: 'nas-telemetry',
        age: '24m',
        detail: 'No TrueNAS report from nas-telemetry for 24m',
      }),
    );
    expect(getTrueNASProtectionStaleNotice(offlinePoint, [telemetryStale, offlineSystem])).toEqual(
      expect.objectContaining({
        badge: 'Stale data',
        systemName: 'nas-offline',
        detail: 'TrueNAS system nas-offline is offline in Pulse',
      }),
    );
  });

  it('excludes stale success and running points from Healthy and Running posture, ranks stale above fresh, and searches stale details', () => {
    const staleNas = makeSystem({
      id: 'truenas:agent:system:conn-stale',
      name: 'nas-east',
      agent: { agentId: 'conn-stale', hostname: 'nas-east' },
      truenas: { hostname: 'nas-east' },
      sourceStatus: { truenas: { status: 'error', error: 'dial tcp timeout' } },
    });
    const freshNas = makeSystem({
      id: 'truenas:agent:system:conn-fresh',
      name: 'nas-west',
      agent: { agentId: 'conn-fresh', hostname: 'nas-west' },
      truenas: { hostname: 'nas-west' },
      sourceStatus: { truenas: { status: 'online' } },
    });

    const freshOk = makePoint({
      id: 'fresh-ok',
      kind: 'snapshot',
      mode: 'snapshot',
      completedAt: '2026-11-27T13:00:00Z',
      details: { connectionId: 'conn-fresh', hostname: 'nas-west', dataset: 'west/ok' },
    });
    const staleOk = makePoint({
      id: 'stale-ok',
      kind: 'snapshot',
      mode: 'snapshot',
      completedAt: '2026-11-27T11:00:00Z',
      details: { connectionId: 'conn-stale', hostname: 'nas-east', dataset: 'east/ok' },
    });
    const staleRunning = makePoint({
      id: 'stale-running',
      kind: 'backup',
      mode: 'remote',
      outcome: 'running',
      completedAt: undefined,
      details: { connectionId: 'conn-stale', hostname: 'nas-east', taskName: 'rep-east' },
    });
    const staleFailed = makePoint({
      id: 'stale-failed',
      kind: 'backup',
      mode: 'remote',
      outcome: 'failed',
      completedAt: '2026-11-27T10:00:00Z',
      details: { connectionId: 'conn-stale', hostname: 'nas-east', taskName: 'rep-fail' },
    });

    const points = [freshOk, staleOk, staleRunning, staleFailed];
    const staleMap = buildTrueNASProtectionStaleNoticeMap(points, [staleNas, freshNas]);
    const posture = buildTrueNASProtectionPosture(points, staleMap);

    expect(posture).toEqual({
      healthy: 1,
      stale: 3,
      warning: 0,
      failed: 1,
      running: 0,
      unknown: 0,
      attention: 3,
    });

    const sorted = sortTrueNASProtectionPoints(points, staleMap);
    expect(sorted.map((p) => p.id)).toEqual([
      'stale-failed',
      'stale-ok',
      'stale-running',
      'fresh-ok',
    ]);

    expect(filterTrueNASProtectionPoints(points, '', 'success', staleMap).map((p) => p.id)).toEqual([
      'fresh-ok',
    ]);
    expect(filterTrueNASProtectionPoints(points, '', 'running', staleMap)).toEqual([]);
    expect(filterTrueNASProtectionPoints(points, '', 'stale', staleMap).map((p) => p.id)).toEqual([
      'stale-ok',
      'stale-running',
      'stale-failed',
    ]);
    expect(
      filterTrueNASProtectionPoints(points, '', 'attention', staleMap).map((p) => p.id),
    ).toEqual(['stale-ok', 'stale-running', 'stale-failed']);
    expect(filterTrueNASProtectionPoints(points, '', 'failed', staleMap).map((p) => p.id)).toEqual([
      'stale-failed',
    ]);
    expect(
      filterTrueNASProtectionPoints(points, 'dial tcp timeout', 'all', staleMap).map((p) => p.id),
    ).toEqual(['stale-ok', 'stale-running', 'stale-failed']);
  });
});

describe('TrueNASProtectionTable stale-reading UI', () => {
  it('surfaces stale healthy, running, and failed events truthfully and clears them reactively on live recovery', async () => {
    const staleNas = makeSystem({
      id: 'truenas:agent:system:conn-east',
      name: 'nas-east',
      displayName: 'nas-east',
      agent: { agentId: 'conn-east', hostname: 'nas-east' },
      truenas: { hostname: 'nas-east' },
      sourceStatus: { truenas: { status: 'error', error: 'API connection refused' } },
      health: {
        verdict: 'stale',
        reasons: [{ code: 'telemetry_stale', detail: '14m' }],
      },
    });
    const freshNas = makeSystem({
      id: 'truenas:agent:system:conn-west',
      name: 'nas-west',
      displayName: 'nas-west',
      agent: { agentId: 'conn-west', hostname: 'nas-west' },
      truenas: { hostname: 'nas-west' },
      sourceStatus: { truenas: { status: 'online' } },
    });

    const points: RecoveryPoint[] = [
      makePoint({
        id: 'snap-east-ok',
        kind: 'snapshot',
        mode: 'snapshot',
        outcome: 'success',
        display: { itemLabel: 'tank/billing', nodeHostLabel: 'nas-east' },
        details: {
          connectionId: 'conn-east',
          hostname: 'nas-east',
          dataset: 'tank/billing',
          snapshot: 'auto-20261127-1200',
        },
      }),
      makePoint({
        id: 'rep-east-running',
        kind: 'backup',
        mode: 'remote',
        outcome: 'running',
        completedAt: undefined,
        display: { itemLabel: 'tank/media', nodeHostLabel: 'nas-east' },
        details: {
          connectionId: 'conn-east',
          hostname: 'nas-east',
          taskName: 'replicate-media',
          lastState: 'RUNNING',
        },
      }),
      makePoint({
        id: 'rep-east-failed',
        kind: 'backup',
        mode: 'remote',
        outcome: 'failed',
        display: { itemLabel: 'tank/archive', nodeHostLabel: 'nas-east' },
        details: {
          connectionId: 'conn-east',
          hostname: 'nas-east',
          taskName: 'replicate-archive',
          lastState: 'FAILED',
        },
      }),
      makePoint({
        id: 'snap-west-ok',
        kind: 'snapshot',
        mode: 'snapshot',
        outcome: 'success',
        display: { itemLabel: 'vault/docs', nodeHostLabel: 'nas-west' },
        details: {
          connectionId: 'conn-west',
          hostname: 'nas-west',
          dataset: 'vault/docs',
          snapshot: 'auto-20261127-1205',
        },
      }),
    ];

    const [systems, setSystems] = createSignal<Resource[]>([staleNas, freshNas]);

    renderInRouter(() => (
      <TrueNASProtectionTable
        points={points}
        systems={systems()}
        emptyIcon={<span />}
        emptyTitle="No protection"
        emptyDescription="No protection"
      />
    ));

    // Filter chips: 1 fresh Healthy, 3 Stale, 3 Attention, 1 Failed, 0 Running
    expect(screen.getByRole('button', { name: 'All, 4' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Attention, 3' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Stale, 3' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Healthy, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Failed, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Running, 0' })).toBeInTheDocument();

    // Stale healthy row shows amber Stale data badge in Outcome and no redundant cue under Dataset
    const staleOkRow = document.querySelector('[data-truenas-protection-row="snap-east-ok"]')!;
    expect(staleOkRow).toHaveAttribute('data-truenas-protection-stale', 'true');
    expect(staleOkRow.querySelector('[data-truenas-protection-stale-badge="true"]')).toHaveTextContent(
      'Stale data',
    );
    expect(staleOkRow.querySelector('[data-truenas-protection-stale-cue="true"]')).toBeNull();

    // Stale failed row preserves red Failed badge AND shows amber Stale data cue under Dataset
    const staleFailedRow = document.querySelector(
      '[data-truenas-protection-row="rep-east-failed"]',
    )!;
    expect(staleFailedRow).toHaveAttribute('data-truenas-protection-stale', 'true');
    expect(staleFailedRow.querySelector('[data-truenas-protection-stale-cue="true"]')).toHaveTextContent(
      'Stale data',
    );
    expect(staleFailedRow.querySelectorAll('td')[5]).toHaveTextContent('Failed');

    // Stale running row preserves Running text without pulsing dot and shows amber Stale data cue
    const staleRunningRow = document.querySelector(
      '[data-truenas-protection-row="rep-east-running"]',
    )!;
    expect(staleRunningRow).toHaveAttribute('data-truenas-protection-stale', 'true');
    expect(staleRunningRow.querySelector('[data-truenas-protection-stale-cue="true"]')).toHaveTextContent(
      'Stale data',
    );
    expect(staleRunningRow.querySelector('.animate-pulse')).toBeNull();

    // Fresh row stays Healthy without stale cues
    const freshRow = document.querySelector('[data-truenas-protection-row="snap-west-ok"]')!;
    expect(freshRow).not.toHaveAttribute('data-truenas-protection-stale');
    expect(freshRow.querySelectorAll('td')[5]).toHaveTextContent('Healthy');

    // Open stale healthy detail drawer: shows amber InlineNotice, Status: Stale data, Last reported: Healthy
    await fireEvent.click(staleOkRow);
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    const staleNotice = detail.getByTestId('truenas-protection-stale-notice');
    expect(staleNotice).toHaveTextContent('Readings from nas-east may be out of date.');
    expect(staleNotice).toHaveTextContent(
      'TrueNAS collection error: API connection refused (last report 14m ago)',
    );
    expect(detail.getByText('Status', { exact: true }).closest('tr')).toHaveTextContent('Stale data');
    expect(detail.getByText('Last reported', { exact: true }).closest('tr')).toHaveTextContent(
      'Healthy',
    );

    // Filter to Stale then recover live over WebSocket: Stale chip stays visible at 0 while active
    await fireEvent.click(screen.getByRole('button', { name: 'Stale, 3' }));
    expect(document.querySelectorAll('[data-truenas-protection-row]')).toHaveLength(3);

    setSystems([
      makeSystem({
        ...staleNas,
        sourceStatus: { truenas: { status: 'online' } },
        health: { verdict: 'ok', reasons: [] },
      }),
      freshNas,
    ]);

    expect(screen.getByRole('button', { name: 'Stale, 0' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Healthy, 2' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Running, 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Attention, 1' })).toBeInTheDocument();

    // Switching back to All removes the Stale chip and clears the open drawer stale banner
    await fireEvent.click(screen.getByRole('button', { name: 'All, 4' }));
    expect(screen.queryByRole('button', { name: /^Stale,/ })).not.toBeInTheDocument();
    expect(screen.queryByTestId('truenas-protection-stale-notice')).not.toBeInTheDocument();
    expect(
      within(screen.getByTestId('truenas-protection-detail'))
        .getByText('Outcome', { exact: true })
        .closest('tr'),
    ).toHaveTextContent('Healthy');
  });
});
