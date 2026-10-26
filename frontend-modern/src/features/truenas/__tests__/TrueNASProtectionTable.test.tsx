import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import { TrueNASProtectionTable } from '@/features/truenas/TrueNASProtectionTable';
import type { RecoveryPoint } from '@/types/recovery';

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
    expect(detail.getByText('FAILED')).toBeInTheDocument();
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
