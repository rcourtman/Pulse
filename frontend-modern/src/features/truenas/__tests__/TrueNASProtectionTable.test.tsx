import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { TrueNASProtectionTable } from '@/features/truenas/TrueNASProtectionTable';
import type { RecoveryPoint } from '@/types/recovery';

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
  it('summarizes protection issues and filters directly to attention outcomes', async () => {
    render(() => (
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

    expect(screen.getByRole('region', { name: 'Protection posture' })).toHaveTextContent(
      '1 protection issue needs review',
    );
    await fireEvent.click(screen.getByRole('button', { name: 'Show issues' }));

    expect(document.querySelectorAll('[data-truenas-protection-row]')).toHaveLength(1);
    expect(document.querySelector('[data-truenas-protection-row="failed"]')).not.toBeNull();
    expect(screen.getByRole('button', { name: 'Show all events' })).toBeInTheDocument();
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

    render(() => (
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
    expect(row).toHaveAttribute('aria-expanded', 'false');

    await fireEvent.click(row!);

    expect(row).toHaveAttribute('aria-expanded', 'true');
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    expect(detail.getByText('Protection detail')).toBeInTheDocument();
    expect(detail.getByText('Protection')).toBeInTheDocument();
    expect(detail.getAllByText('Replication').length).toBeGreaterThan(1);
    expect(detail.getAllByText('Target').length).toBeGreaterThan(1);
    expect(detail.getByText('rep-task-tank-apps')).toBeInTheDocument();
    expect(detail.getAllByText('vault/compliance/tank_apps').length).toBeGreaterThan(1);
    expect(detail.getByText('tank/apps@auto-20260331-0600')).toBeInTheDocument();

    await fireEvent.click(detail.getByRole('button', { name: 'Close' }));

    expect(screen.queryByTestId('truenas-protection-detail')).not.toBeInTheDocument();
    expect(row).toHaveAttribute('aria-expanded', 'false');
  });
});
