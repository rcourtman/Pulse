import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal, type JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DarkModeContext, WebSocketContext, type WebSocketStore } from '@/contexts/appRuntime';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { Resource } from '@/types/resource';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupJobsCard } from '../ProxmoxBackupJobsCard';
import { buildBackupServerRows } from '../ProxmoxBackupServersTable';
import { ProxmoxBackupsCoverageStrip } from '../ProxmoxBackupsCoverageStrip';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';
import {
  collectStaleBackupSources,
  collectStaleWorkloadNotices,
  describeStaleBackupSources,
  getArtifactStaleNotice,
  getCoverageRowStaleNotice,
  isPBSServerPollStale,
} from '../proxmoxBackupStaleness';
import { buildBackupVerdict } from '../proxmoxBackupVerdict';

const apiFetchMock = vi.hoisted(() => vi.fn());
const apiFetchJSONMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
  apiFetchJSON: apiFetchJSONMock,
}));

const jsonResponse = (payload: unknown) =>
  new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const makePVENode = (name: string, opts?: { stale?: boolean; agentStale?: boolean }): Resource => ({
  id: `pve-node-${name}`,
  type: 'agent',
  name,
  displayName: name,
  platformId: 'prod-pve',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  sources: ['proxmox'],
  status: opts?.stale ? 'degraded' : 'online',
  lastSeen: Date.parse('2026-11-28T04:00:00Z'),
  health: opts?.stale
    ? { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '18m' }] }
    : opts?.agentStale
      ? { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '12m' }] }
      : { verdict: 'ok', reasons: [] },
  sourceStatus: {
    proxmox: opts?.stale
      ? { status: 'stale', error: 'dial tcp 10.0.0.10:8006: i/o timeout' }
      : { status: 'ok' },
    ...(opts?.agentStale ? { agent: { status: 'stale', error: 'agent timeout' } } : {}),
  },
  proxmox: {
    nodeName: name,
    instance: 'prod-pve',
    connectionHealth: opts?.stale ? 'stale' : 'online',
  },
});

const makePBSServer = (
  name: string,
  opts?: { stale?: boolean; agentStale?: boolean },
): Resource => ({
  id: `pbs-${name}`,
  type: 'pbs',
  name,
  displayName: name,
  platformId: name,
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  sources: ['pbs'],
  status: opts?.stale ? 'degraded' : 'online',
  lastSeen: Date.parse('2026-11-28T04:00:00Z'),
  health: opts?.stale
    ? { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '22m' }] }
    : opts?.agentStale
      ? { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '9m' }] }
      : { verdict: 'ok', reasons: [] },
  sourceStatus: {
    pbs: opts?.stale ? { status: 'stale', error: 'connection refused' } : { status: 'ok' },
    ...(opts?.agentStale ? { agent: { status: 'stale', error: 'agent stopped' } } : {}),
  },
  pbs: {
    connectionHealth: opts?.stale ? 'stale' : 'online',
    datastores: [{ name: 'main', used: 100, total: 1000, status: 'available' }],
  },
});

const makeGuest = (id: string, name: string, vmid: number, node: string): Resource => ({
  id,
  type: 'vm',
  name,
  displayName: name,
  platformId: 'prod-pve',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  sources: ['proxmox'],
  status: 'running',
  parentName: node,
  lastSeen: Date.parse('2026-11-28T04:00:00Z'),
  proxmox: {
    vmid,
    nodeName: node,
    instance: 'prod-pve',
  },
});

const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <DarkModeContext.Provider value={() => false}>
      <WebSocketContext.Provider value={{ activeAlerts: {} } as unknown as WebSocketStore}>
        <Router>
          <Route path="/*" component={component} />
        </Router>
      </WebSocketContext.Provider>
    </DarkModeContext.Provider>
  ));

beforeEach(() => {
  resetCreateNonSuspendingQueryCacheForTest();
  apiFetchMock.mockReset();
  apiFetchJSONMock.mockReset();
  window.history.replaceState({}, '', '/proxmox/backups/coverage');
});

afterEach(() => {
  cleanup();
});

describe('proxmoxBackupStaleness pure model', () => {
  it('collects stale PVE and PBS sources while ignoring stale optional agents when provider poll is fresh', () => {
    const stalePVE = makePVENode('pve-stale', { stale: true });
    const freshPVEWithStaleAgent = makePVENode('pve-fresh', { agentStale: true });
    const stalePBS = makePBSServer('pbs-stale', { stale: true });
    const freshPBSWithStaleAgent = makePBSServer('pbs-fresh', { agentStale: true });

    const sources = collectStaleBackupSources([
      stalePVE,
      freshPVEWithStaleAgent,
      stalePBS,
      freshPBSWithStaleAgent,
    ]);
    expect(sources.map((s) => `${s.provider}:${s.name}`)).toEqual([
      'proxmox-pbs:pbs-stale',
      'proxmox-pve:pve-stale',
    ]);
    expect(isPBSServerPollStale(stalePBS)).toBe(true);
    expect(isPBSServerPollStale(freshPBSWithStaleAgent)).toBe(false);
    expect(buildBackupServerRows([stalePBS])[0]?.pollStale).toBe(true);
    expect(buildBackupServerRows([freshPBSWithStaleAgent])[0]?.pollStale).toBe(false);

    const summary = describeStaleBackupSources(sources);
    expect(summary).toContain('pbs-stale (22m)');
    expect(summary).toContain('pve-stale (18m)');
    expect(summary).toContain('last values received before polling stopped');
  });

  it('matches coverage rows and restore points only to affected stale sources', () => {
    const stalePVE = makePVENode('pve-stale', { stale: true });
    const freshPVE = makePVENode('pve-fresh');
    const stalePBS = makePBSServer('pbs-stale', { stale: true });
    const freshPBS = makePBSServer('pbs-fresh');

    const gStalePve = makeGuest('vm-101', 'db-stale-pve', 101, 'pve-stale');
    const gStalePbs = makeGuest('vm-102', 'app-stale-pbs', 102, 'pve-fresh');
    const gFresh = makeGuest('vm-103', 'api-fresh', 103, 'pve-fresh');

    const postures = new Map([
      [
        'vm-101',
        {
          ...protectionPosture('vm-101'),
          state: 'protected' as const,
          providerStates: [
            {
              provider: 'proxmox-pbs',
              source: 'pbs-backup-enumeration',
              scope: 'pbs-fresh',
              jobState: 'success',
              historyCompleteness: 'complete' as const,
              permissions: 'sufficient' as const,
              evidenceIds: ['e1'],
            },
          ],
        },
      ],
      [
        'vm-102',
        {
          ...protectionPosture('vm-102'),
          state: 'protected' as const,
          providerStates: [
            {
              provider: 'proxmox-pbs',
              source: 'pbs-backup-enumeration',
              scope: 'pbs-stale',
              jobState: 'success',
              historyCompleteness: 'complete' as const,
              permissions: 'sufficient' as const,
              evidenceIds: ['e2'],
            },
          ],
        },
      ],
      [
        'vm-103',
        {
          ...protectionPosture('vm-103'),
          state: 'protected' as const,
          providerStates: [
            {
              provider: 'proxmox-pbs',
              source: 'pbs-backup-enumeration',
              scope: 'pbs-fresh',
              jobState: 'success',
              historyCompleteness: 'complete' as const,
              permissions: 'sufficient' as const,
              evidenceIds: ['e3'],
            },
          ],
        },
      ],
    ]);

    const model = buildProxmoxBackupRecoveryModel({
      workloads: [gStalePve, gStalePbs, gFresh],
      pbsBackups: [
        {
          id: 'pbs-stale/main/vm/102/2026-11-28T01:00:00Z',
          instance: 'pbs-stale',
          datastore: 'main',
          namespace: '',
          backupType: 'vm',
          vmid: '102',
          backupTime: '2026-11-28T01:00:00Z',
          size: 4096,
          protected: false,
          verified: true,
          files: ['index.json.blob'],
        },
        {
          id: 'pbs-fresh/main/vm/103/2026-11-28T01:00:00Z',
          instance: 'pbs-fresh',
          datastore: 'main',
          namespace: '',
          backupType: 'vm',
          vmid: '103',
          backupTime: '2026-11-28T01:00:00Z',
          size: 4096,
          protected: false,
          verified: true,
          files: ['index.json.blob'],
        },
      ],
      archives: [
        {
          id: 'archive-101',
          storage: 'local',
          node: 'pve-stale',
          instance: 'prod-pve',
          type: 'vm',
          vmid: 101,
          time: '2026-11-28T01:30:00Z',
          ctime: 1795829400,
          size: 2048,
          format: 'zst',
          protected: false,
          volid: 'local:backup/vzdump-qemu-101.vma.zst',
          isPBS: false,
          verified: true,
        },
      ],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-11-28T04:00:00Z'),
      protectionPostures: postures,
      protectionPosturesResolved: true,
    });

    const staleSources = collectStaleBackupSources(
      [stalePVE, freshPVE, stalePBS, freshPBS],
      [gStalePve, gStalePbs, gFresh],
    );
    const staleWorkloads = collectStaleWorkloadNotices([gStalePve, gStalePbs, gFresh]);

    const byName = new Map(model.coverageRows.map((row) => [row.workload.name, row]));
    expect(
      getCoverageRowStaleNotice(byName.get('db-stale-pve')!, staleSources, staleWorkloads),
    ).toMatchObject({
      badge: 'Stale · 18m',
      error: 'dial tcp 10.0.0.10:8006: i/o timeout',
    });
    expect(
      getCoverageRowStaleNotice(byName.get('app-stale-pbs')!, staleSources, staleWorkloads),
    ).toMatchObject({
      badge: 'Stale · 22m',
      error: 'connection refused',
    });
    expect(getCoverageRowStaleNotice(byName.get('api-fresh')!, staleSources, staleWorkloads)).toBe(
      null,
    );

    const byArtifactId = new Map(model.recoverableArtifacts.map((a) => [a.id, a]));
    expect(
      getArtifactStaleNotice(byArtifactId.get('archive:archive-101')!, staleSources)?.badge,
    ).toBe('Stale · 18m');
    expect(
      getArtifactStaleNotice(
        byArtifactId.get('pbs:pbs-stale/main/vm/102/2026-11-28T01:00:00Z')!,
        staleSources,
      )?.badge,
    ).toBe('Stale · 22m');
    expect(
      getArtifactStaleNotice(
        byArtifactId.get('pbs:pbs-fresh/main/vm/103/2026-11-28T01:00:00Z')!,
        staleSources,
      ),
    ).toBe(null);
  });
});

describe('Proxmox Backups stale telemetry presentation', () => {
  it('downgrades green protected verdict dot to amber with Stale data cue while keeping red unprotected red', () => {
    const protectedRow = {
      key: 'resource:vm-1',
      workload: {
        key: 'resource:vm-1',
        resourceId: 'vm-1',
        type: 'vm' as const,
        typeLabel: 'VM',
        vmid: '100',
        label: 'vm-1',
        name: 'vm-1',
      },
      artifacts: [],
      pbsCount: 1,
      archiveCount: 0,
      snapshotCount: 0,
      posture: 'protected' as const,
      postureRank: 0,
      protectionPosture: {
        ...protectionPosture('vm-1'),
        state: 'protected' as const,
        reason: 'recent-verified' as const,
      },
      isOrphaned: false,
    };
    const unprotectedRow = {
      ...protectedRow,
      key: 'resource:vm-2',
      posture: 'unprotected' as const,
      protectionPosture: {
        ...protectionPosture('vm-2'),
        state: 'unprotected' as const,
        reason: 'no-backup' as const,
      },
    };

    const view = render(() => (
      <ProxmoxBackupsCoverageStrip
        title="Backup health"
        verdict={buildBackupVerdict([protectedRow], protectionPolicy)}
        staleNotice="Backup telemetry is stale for pve-stale (18m)."
      />
    ));
    expect(view.container.querySelector('[data-backup-verdict="attention"]')).toHaveTextContent(
      '1 / 1 protected',
    );
    expect(view.container.querySelector('[data-backup-verdict-stale]')).toHaveTextContent(
      'Stale data',
    );
    cleanup();

    const redView = render(() => (
      <ProxmoxBackupsCoverageStrip
        title="Backup health"
        verdict={buildBackupVerdict([unprotectedRow], protectionPolicy)}
        staleNotice="Backup telemetry is stale for pve-stale (18m)."
      />
    ));
    expect(redView.container.querySelector('[data-backup-verdict="unprotected"]')).not.toBeNull();
    expect(redView.container.querySelector('[data-backup-verdict-stale]')).toHaveTextContent(
      'Stale data',
    );
  });

  it('surfaces stale banner, row cues, drawers, search, and clears them live on recovery', async () => {
    const gStale = makeGuest('vm-101', 'db-primary', 101, 'pve-stale');
    const gFresh = makeGuest('vm-102', 'api-fresh', 102, 'pve-fresh');

    apiFetchMock.mockImplementation((url: string) => {
      if (url === '/api/backups/pbs') {
        return Promise.resolve(
          jsonResponse({
            data: {
              backups: [
                {
                  id: 'pbs-fresh/main/vm/102/2026-11-28T01:00:00Z',
                  instance: 'pbs-fresh',
                  datastore: 'main',
                  backupType: 'vm',
                  vmid: '102',
                  backupTime: '2026-11-28T01:00:00Z',
                  size: 4_000_000,
                  protected: false,
                  verified: true,
                  files: ['index.json.blob'],
                  guestInstance: 'prod-pve',
                },
              ],
            },
          }),
        );
      }
      if (url === '/api/backups/pve') {
        return Promise.resolve(
          jsonResponse({
            data: {
              guestSnapshots: [],
              storageBackups: [
                {
                  id: 'archive-101',
                  storage: 'local',
                  node: 'pve-stale',
                  instance: 'prod-pve',
                  type: 'vm',
                  vmid: 101,
                  time: '2026-11-28T01:30:00Z',
                  ctime: 1795829400,
                  size: 2_000_000,
                  format: 'zst',
                  protected: false,
                  volid: 'local:backup/vzdump-qemu-101.vma.zst',
                  isPBS: false,
                  verified: true,
                },
              ],
              backupTasks: [],
            },
          }),
        );
      }
      return Promise.resolve(jsonResponse({}));
    });

    apiFetchJSONMock.mockResolvedValue({
      data: [
        { ...protectionPosture('vm-101'), state: 'protected', reason: 'recent-verified' },
        { ...protectionPosture('vm-102'), state: 'protected', reason: 'recent-verified' },
      ],
      policy: protectionPolicy,
    });

    const [recovered, setRecovered] = createSignal(false);
    renderInRouter(() => (
      <ProxmoxBackupsTable
        emptyIcon={<span />}
        workloads={[gStale, gFresh]}
        servers={[
          makePVENode('pve-stale', { stale: !recovered() }),
          makePVENode('pve-fresh'),
          makePBSServer('pbs-fresh'),
        ]}
        summaryAside={(days) => (
          <ProxmoxBackupJobsCard
            jobs={{
              timezone: 'UTC',
              historyState: 'complete',
              last24h: {
                historyState: 'complete',
                counts: { success: 2, failed: 0, warning: 0, running: 0, unknown: 0 },
                failures: [],
              },
              days: [
                {
                  date: '2026-11-28',
                  historyState: 'complete',
                  counts: { success: 2, failed: 0, warning: 0, running: 0, unknown: 0 },
                  failures: [],
                },
              ],
            }}
            selectedDay={days.selectedDay()}
            staleNotice={days.staleNotice?.()}
          />
        )}
      />
    ));

    await waitFor(() => {
      expect(document.querySelector('[data-stale-backup-sources]')).not.toBeNull();
    });
    expect(document.querySelector('[data-stale-backup-sources]')).toHaveTextContent(
      'pve-stale (18m)',
    );
    expect(document.querySelector('[data-backup-verdict="attention"]')).toHaveTextContent(
      '2 / 2 protected',
    );
    expect(document.querySelector('[data-backup-verdict-stale]')).toHaveTextContent('Stale data');
    expect(document.querySelector('[data-backup-jobs-stale]')).toHaveTextContent('Stale data');

    const rows = document.querySelectorAll('[data-proxmox-backup-row="coverage"]');
    expect(rows).toHaveLength(2);
    const staleRow = [...rows].find((r) => r.textContent?.includes('db-primary'))!;
    const freshRow = [...rows].find((r) => r.textContent?.includes('api-fresh'))!;
    expect(staleRow.querySelector('[data-coverage-stale-cue]')).toHaveTextContent('Stale · 18m');
    expect(staleRow).toHaveTextContent('Protected');
    expect(freshRow.querySelector('[data-coverage-stale-cue]')).toBeNull();

    // Open stale workload drawer and verify inline notice.
    fireEvent.click(within(staleRow as HTMLElement).getByRole('button'));
    await waitFor(() => {
      expect(document.querySelector('[data-coverage-stale-notice]')).not.toBeNull();
    });
    expect(document.querySelector('[data-coverage-stale-notice]')).toHaveTextContent(
      'dial tcp 10.0.0.10:8006: i/o timeout',
    );

    // Searching by stale error narrows Coverage to the affected workload.
    const searchInput = screen.getByPlaceholderText(
      'Search backups by workload, node, source, or status',
    );
    fireEvent.input(searchInput, { target: { value: 'i/o timeout' } });
    await waitFor(() => {
      expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(1);
    });
    expect(document.querySelector('[data-proxmox-backup-row="coverage"]')).toHaveTextContent(
      'db-primary',
    );
    fireEvent.input(searchInput, { target: { value: '' } });
    await waitFor(() => {
      expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(2);
    });

    // Switch to By date view and verify stale cue + drawer notice on affected restore point.
    fireEvent.click(screen.getByRole('link', { name: 'By date' }));
    await waitFor(() => {
      expect(document.querySelectorAll('[data-proxmox-backup-row="recoverable"]')).toHaveLength(2);
    });
    const recRows = [...document.querySelectorAll('[data-proxmox-backup-row="recoverable"]')];
    const staleRecRow = recRows.find((r) => r.textContent?.includes('db-primary'))!;
    const freshRecRow = recRows.find((r) => r.textContent?.includes('api-fresh'))!;
    expect(staleRecRow.querySelector('[data-recoverable-stale-cue]')).toHaveTextContent(
      'Stale · 18m',
    );
    expect(freshRecRow.querySelector('[data-recoverable-stale-cue]')).toBeNull();
    fireEvent.click(within(staleRecRow as HTMLElement).getByRole('button'));
    await waitFor(() => {
      expect(document.querySelector('[data-recoverable-stale-notice]')).not.toBeNull();
    });
    expect(document.querySelector('[data-recoverable-stale-notice]')).toHaveTextContent(
      'Source telemetry is stale.',
    );

    // Live recovery clears banner, verdict stale cue, jobs stale cue, row cue, and drawer notice.
    setRecovered(true);
    await waitFor(() => {
      expect(document.querySelector('[data-stale-backup-sources]')).toBeNull();
    });
    expect(document.querySelector('[data-backup-verdict="protected"]')).not.toBeNull();
    expect(document.querySelector('[data-backup-verdict-stale]')).toBeNull();
    expect(document.querySelector('[data-backup-jobs-stale]')).toBeNull();
    expect(document.querySelector('[data-recoverable-stale-cue]')).toBeNull();
    expect(document.querySelector('[data-recoverable-stale-notice]')).toBeNull();
  });
});
