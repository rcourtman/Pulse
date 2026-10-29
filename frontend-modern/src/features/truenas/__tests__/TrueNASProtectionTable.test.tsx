import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { TrueNASProtectionTable } from '@/features/truenas/TrueNASProtectionTable';
import type { RecoveryPoint, RecoveryPointTransport } from '@/types/recovery';
import { sortTrueNASProtectionPoints } from '@/features/truenas/truenasPageModel';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';

// Record drawers link to the object they are about, which needs a router.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

const makeRecoveryPoint = (
  point: Partial<RecoveryPoint> & Pick<RecoveryPoint, 'id' | 'kind' | 'mode'>,
): RecoveryPoint => ({
  outcome: 'success',
  platform: 'truenas',
  startedAt: '2026-05-20T00:00:00Z',
  completedAt: '2026-05-20T00:00:00Z',
  ...point,
});

afterEach(() => {
  cleanup();
});

describe('TrueNASProtectionTable', () => {
  it('keeps the issue count in the filters and filters directly to attention outcomes', async () => {
    renderInRouter(() => (
      <TrueNASProtectionTable
        points={[
          makeRecoveryPoint({ id: 'healthy', kind: 'snapshot', mode: 'snapshot' }),
          makeRecoveryPoint({
            id: 'failed',
            kind: 'backup',
            mode: 'remote',
            outcome: 'failed',
          }),
          makeRecoveryPoint({
            id: 'running',
            kind: 'backup',
            mode: 'remote',
            outcome: 'running',
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No protection"
        emptyDescription="No protection"
      />
    ));

    expect(screen.queryByRole('region', { name: 'Protection posture' })).not.toBeInTheDocument();
    expect(document.querySelector('[data-truenas-protection-row]')).toHaveAttribute(
      'data-truenas-protection-row',
      'failed',
    );

    const attentionFilter = screen.getByRole('button', {
      name: 'Attention, 1',
    });
    expect(attentionFilter).toHaveTextContent('Attention');
    expect(attentionFilter).toHaveTextContent('1');
    await fireEvent.click(attentionFilter);

    expect(document.querySelectorAll('[data-truenas-protection-row]')).toHaveLength(1);
    expect(document.querySelector('[data-truenas-protection-row="failed"]')).not.toBeNull();

    await fireEvent.click(screen.getByRole('button', { name: 'All, 3' }));
    expect(document.querySelectorAll('[data-truenas-protection-row]')).toHaveLength(3);
  });

  it('keeps undated replication problems in Attention without inventing completion dates', async () => {
    renderInRouter(() => (
      <TrueNASProtectionTable
        points={(['failed', 'warning', 'running', 'unknown'] as const).map((outcome) =>
          makeRecoveryPoint({
            id: outcome,
            kind: 'backup',
            mode: 'remote',
            outcome,
            startedAt: undefined,
            completedAt: undefined,
            details: {
              taskId: `task-${outcome}`,
              taskName: `replicate-${outcome}`,
              sourceDatasets: [`tank/${outcome}`],
              targetDataset: `vault/${outcome}`,
              lastState: outcome.toUpperCase(),
            },
          }),
        )}
        emptyIcon={<span />}
        emptyTitle="No protection"
        emptyDescription="No protection"
      />
    ));

    await fireEvent.click(screen.getByRole('button', { name: 'Attention, 2' }));
    const rows = document.querySelectorAll('[data-truenas-protection-row]');
    expect(rows).toHaveLength(2);
    for (const outcome of ['failed', 'warning']) {
      const row = document.querySelector(`[data-truenas-protection-row="${outcome}"]`)!;
      expect(row).toHaveAttribute('data-truenas-protection-outcome', outcome);
      expect(row.querySelectorAll('td')[3]).toHaveTextContent('-');
    }

    await fireEvent.click(document.querySelector('[data-truenas-protection-row="failed"]')!);
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    expect(detail.getByText('task-failed')).toBeInTheDocument();
    expect(detail.getByText('Last state').closest('tr')).toHaveTextContent('FAILED');
    expect(detail.queryByText('Started', { exact: true })).not.toBeInTheDocument();
    expect(detail.queryByText('Completed', { exact: true })).not.toBeInTheDocument();
    await fireEvent.click(detail.getByRole('button', { name: /^Collapse .* details$/ }));

    await fireEvent.click(screen.getByRole('button', { name: 'Running, 1' }));
    expect(document.querySelectorAll('[data-truenas-protection-row]')).toHaveLength(1);
    expect(document.querySelector('[data-truenas-protection-row="running"]')).not.toBeNull();
    await fireEvent.click(screen.getByRole('button', { name: 'Unknown, 1' }));
    expect(document.querySelectorAll('[data-truenas-protection-row]')).toHaveLength(1);
    expect(document.querySelector('[data-truenas-protection-row="unknown"]')).not.toBeNull();
  });

  it('opens inline table details for a TrueNAS replication recovery point', async () => {
    const replication = makeRecoveryPoint({
      id: 'replicate-tank-apps',
      kind: 'backup',
      mode: 'remote',
      outcome: 'running',
      display: {
        itemLabel: 'tank/apps',
        repositoryLabel: 'vault/compliance/tank_apps',
        detailsSummary: 'replicate-tank-apps (tank/apps@auto-20260331-0600)',
      },
      repositoryRef: {
        type: 'truenas-dataset',
        name: 'vault/compliance/tank_apps',
        id: 'vault/compliance/tank_apps',
      },
      details: {
        direction: 'PUSH',
        lastSnapshot: 'tank/apps@auto-20260331-0600',
        lastState: 'RUNNING',
        sourceDatasets: ['tank/apps'],
        targetDataset: 'vault/compliance/tank_apps',
        taskId: 'rep-task-tank-apps',
        taskName: 'replicate-tank-apps',
      },
    });

    renderInRouter(() => (
      <TrueNASProtectionTable
        points={[replication]}
        emptyIcon={<span />}
        emptyTitle="No protection"
        emptyDescription="No protection"
        showToolbar={false}
      />
    ));

    const row = screen.getByText('tank/apps').closest('tr');
    expect(row).toBeTruthy();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await fireEvent.click(row!);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    expect(detail.getByRole('heading', { level: 2 })).toHaveTextContent('tank/apps');
    expect(detail.getByText('Protection')).toBeInTheDocument();
    expect(detail.getAllByText('Replication').length).toBeGreaterThan(1);
    expect(detail.getAllByText('Target').length).toBeGreaterThan(1);
    expect(detail.getByText('rep-task-tank-apps')).toBeInTheDocument();
    expect(detail.getAllByText('vault/compliance/tank_apps').length).toBeGreaterThan(1);
    expect(detail.getByText('tank/apps@auto-20260331-0600')).toBeInTheDocument();

    await fireEvent.click(detail.getByRole('button', { name: /^Collapse .* details$/ }));

    expect(screen.queryByTestId('truenas-protection-detail')).not.toBeInTheDocument();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });

  it('keeps undated snapshots unknown and dateless without hiding their artifact details', async () => {
    renderInRouter(() => (
      <TrueNASProtectionTable
        points={[
          makeRecoveryPoint({
            id: 'undated',
            kind: 'snapshot',
            mode: 'snapshot',
            outcome: 'unknown',
            startedAt: undefined,
            completedAt: undefined,
            details: { dataset: 'tank/undated', fullName: 'tank/undated@daily', snapshot: 'daily' },
          }),
          makeRecoveryPoint({ id: 'known', kind: 'snapshot', mode: 'snapshot' }),
          makeRecoveryPoint({ id: 'running', kind: 'backup', mode: 'remote', outcome: 'running' }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No protection"
        emptyDescription="No protection"
      />
    ));
    await fireEvent.click(screen.getByRole('button', { name: 'Unknown, 1' }));
    const rows = document.querySelectorAll('[data-truenas-protection-row]');
    expect(rows).toHaveLength(1);
    const row = rows[0];
    expect(row).toHaveAttribute('data-truenas-protection-outcome', 'unknown');
    expect(within(row as HTMLElement).getByText('Unknown')).toBeInTheDocument();
    expect(row.querySelectorAll('td')[3]).toHaveTextContent('-');
    await fireEvent.click(row);
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    expect(detail.getByText('tank/undated@daily')).toBeInTheDocument();
    expect(detail.queryByText('Started', { exact: true })).not.toBeInTheDocument();
    expect(detail.queryByText('Completed', { exact: true })).not.toBeInTheDocument();
    await fireEvent.click(detail.getByRole('button', { name: /^Collapse .* details$/ }));
    await fireEvent.click(screen.getByRole('button', { name: 'Healthy, 1' }));
    expect(document.querySelector('[data-truenas-protection-row]')).toHaveAttribute(
      'data-truenas-protection-row',
      'known',
    );
    await fireEvent.click(screen.getByRole('button', { name: 'Running, 1' }));
    expect(document.querySelector('[data-truenas-protection-row]')).toHaveAttribute(
      'data-truenas-protection-row',
      'running',
    );
  });
});

const apiFetchJSON = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON }));

const point = (
  id: string,
  overrides: Partial<RecoveryPointTransport> = {},
): RecoveryPointTransport => ({
  id,
  platform: 'truenas',
  kind: 'backup',
  mode: 'remote',
  outcome: 'success',
  startedAt: '2026-09-29T01:00:00Z',
  completedAt: '2026-09-30T01:15:00Z',
  display: { subjectLabel: `tank/${id}`, subjectType: 'dataset' },
  ...overrides,
});

function mount(points: RecoveryPointTransport[]) {
  apiFetchJSON.mockResolvedValue({ data: points });
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => {
          const recovery = useRecoveryPoints(() => ({ platform: 'truenas', page: 1, limit: 200 }));
          return (
            <TrueNASProtectionTable
              points={recovery.points()}
              emptyIcon={<span />}
              emptyTitle="No protection"
              emptyDescription="No events in this response."
            />
          );
        }}
      />
    </Router>
  ));
}

const row = (id: string) =>
  document.querySelector<HTMLElement>(`[data-truenas-protection-row="${id}"]`)!;
const completion = (id: string) => row(id).querySelectorAll('td')[3]!;
const rowIDs = () =>
  [...document.querySelectorAll('[data-truenas-protection-row]')].map((item) =>
    item.getAttribute('data-truenas-protection-row'),
  );

afterEach(() => {
  cleanup();
  apiFetchJSON.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
  localStorage.clear();
});

describe('TrueNAS protection timestamp admission', () => {
  it.each([
    '2026-09-31T01:15:00Z',
    '2026-02-29T01:15:00Z',
    '2026-09-30T01:15:00',
    '2026-09-30',
    '2026-09-30T24:00:00Z',
    'not-a-date',
  ])(
    'keeps a malformed reported date unknown in the row and both detail fields: %s',
    async (raw) => {
      mount([point('malformed', { startedAt: raw, completedAt: raw, outcome: 'failed' })]);
      await vi.waitFor(() => expect(row('malformed')).not.toBeNull());
      expect(completion('malformed')).toHaveTextContent(/^Unknown$/);
      expect(row('malformed')).toHaveAttribute('data-truenas-protection-outcome', 'failed');
      await fireEvent.click(within(row('malformed')).getByRole('button', { name: /details/i }));
      const detail = within(screen.getByTestId('truenas-protection-detail'));
      for (const label of ['Started', 'Completed']) {
        expect(detail.getByText(label, { exact: true }).closest('tr')).toHaveTextContent(
          `Unknown date (${raw})`,
        );
      }
      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/recovery/points?page=1&limit=200&platform=truenas',
      );
    },
  );

  it.each([
    '2024-02-29T01:15:00.123456789+01:30',
    '2026-09-30T01:15:00Z',
    '2026-09-29T23:15:00-02:00',
  ])('preserves valid explicit instants through the real transport normalizer: %s', async (raw) => {
    mount([point('valid', { startedAt: raw, completedAt: raw })]);
    await vi.waitFor(() => expect(row('valid')).not.toBeNull());
    expect(completion('valid')).not.toHaveTextContent(/Unknown|^\s*-\s*$/);
    await fireEvent.click(within(row('valid')).getByRole('button', { name: /details/i }));
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    const expected = new Date(raw).toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
    for (const label of ['Started', 'Completed']) {
      expect(detail.getByText(label, { exact: true }).closest('tr')).toHaveTextContent(expected);
    }
  });

  it('keeps malformed completions last in either completion sort direction without borrowing starts', async () => {
    mount([
      point('malformed', {
        completedAt: '2026-09-31T01:15:00Z',
        startedAt: '2026-10-02T00:00:00Z',
      }),
      point('missing', { completedAt: undefined }),
      point('older', { completedAt: '2026-09-29T01:15:00Z' }),
      point('newer'),
    ]);
    await vi.waitFor(() => expect(row('malformed')).not.toBeNull());
    const sort = within(screen.getByRole('columnheader', { name: /Completed/ })).getByRole(
      'button',
    );
    await fireEvent.click(sort);
    expect(rowIDs().slice(0, 2)).toEqual(['newer', 'older']);
    expect(rowIDs().slice(2).sort()).toEqual(['malformed', 'missing']);
    await fireEvent.click(sort);
    expect(rowIDs().slice(0, 2)).toEqual(['older', 'newer']);
    expect(rowIDs().slice(2).sort()).toEqual(['malformed', 'missing']);
  });

  it('does not give malformed dates a default chronological rank, and preserves attention priority', () => {
    const valid = point('valid', { completedAt: '1960-01-01T00:00:00Z' });
    const malformed = point('malformed', { completedAt: '2026-09-31T01:15:00Z' });
    const absent = point('absent', { completedAt: undefined, startedAt: undefined });
    expect(sortTrueNASProtectionPoints([absent, malformed, valid]).map(({ id }) => id)).toEqual([
      'valid',
      'absent',
      'malformed',
    ]);
    const failed = point('failed', { outcome: 'failed', completedAt: '2026-09-31T01:15:00Z' });
    expect(sortTrueNASProtectionPoints([valid, failed]).map(({ id }) => id)).toEqual([
      'failed',
      'valid',
    ]);
  });
});
