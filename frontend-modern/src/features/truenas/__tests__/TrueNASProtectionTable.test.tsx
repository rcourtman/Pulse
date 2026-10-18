import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { createSignal } from 'solid-js';

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
  it('keeps a failed-read notice visible even when filters hide every retained record', async () => {
    render(() => (
      <TrueNASProtectionTable
        points={[makeRecoveryPoint({ id: 'retained', kind: 'snapshot', mode: 'snapshot' })]}
        error={new Error('private diagnostic')}
        emptyIcon={<span />}
        emptyTitle="No protection"
        emptyDescription="No protection"
      />
    ));
    expect(screen.getByText(/Could not refresh TrueNAS protection\./)).toHaveTextContent(
      'last loaded records',
    );
    await fireEvent.click(screen.getByRole('button', { name: 'Attention, 0' }));
    expect(screen.getByText('No protection events match current filters')).toBeInTheDocument();
    expect(screen.getByText(/Could not refresh TrueNAS protection\./)).toHaveTextContent(
      'do not establish current protection',
    );
    expect(screen.queryByText('private diagnostic')).not.toBeInTheDocument();
  });

  it('keeps the issue count in the filters and filters directly to attention outcomes', async () => {
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
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await fireEvent.click(row!);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    expect(detail.getByText('Protection detail')).toBeInTheDocument();
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
});

describe('TrueNAS protection activity evidence', () => {
  const replication = (overrides: Partial<RecoveryPoint> = {}): RecoveryPoint =>
    makeRecoveryPoint({
      id: 'replication-evidence',
      kind: 'backup',
      mode: 'remote',
      outcome: 'running',
      display: { itemLabel: 'tank/photos' },
      details: { lastState: 'RUNNING' },
      ...overrides,
    });

  const mount = (point: RecoveryPoint) =>
    render(() => (
      <TrueNASProtectionTable
        points={[point]}
        emptyIcon={<span />}
        emptyTitle="No activity"
        emptyDescription="No activity"
      />
    ));

  const expand = async () => {
    await fireEvent.click(screen.getByRole('button', { name: 'Expand details for tank/photos' }));
    return screen.getByTestId('truenas-protection-detail');
  };

  it('labels and sorts reported activity without claiming it completed', async () => {
    const points = [
      replication({ id: 'early', completedAt: '2026-05-18T01:00:00Z' }),
      replication({ id: 'late', completedAt: '2026-05-20T01:00:00Z' }),
      replication({ id: 'missing', completedAt: null, startedAt: null }),
    ];
    render(() => (
      <TrueNASProtectionTable
        points={points}
        emptyIcon={<span />}
        emptyTitle="No activity"
        emptyDescription="No activity"
      />
    ));
    const sortHeader = screen.getByRole('columnheader', { name: /Reported time/ });
    expect(screen.queryByRole('columnheader', { name: /Completed/ })).not.toBeInTheDocument();
    const rowIDs = () =>
      [...document.querySelectorAll('[data-truenas-protection-row]')].map((row) =>
        row.getAttribute('data-truenas-protection-row'),
      );
    await fireEvent.click(sortHeader);
    expect(rowIDs()).toEqual(['late', 'early', 'missing']);
    await fireEvent.click(sortHeader);
    expect(rowIDs()).toEqual(['early', 'late', 'missing']);
  });

  it.each(['running', 'failed', 'success', 'unknown'])(
    'uses the reported last run, not invented start/end times, for %s replication',
    async (outcome) => {
      mount(replication({ outcome }));
      const detail = within(await expand());
      expect(detail.getByText('Last run')).toBeInTheDocument();
      expect(detail.queryByText('Started', { exact: true })).not.toBeInTheDocument();
      expect(detail.queryByText('Completed', { exact: true })).not.toBeInTheDocument();
      expect(document.querySelector('[data-truenas-protection-row]')).toHaveAttribute(
        'data-truenas-protection-outcome',
        outcome,
      );
    },
  );

  it('renders a long multiline reported error as escaped, wrapping text', async () => {
    const reported = `Permission denied\n<img src=x onerror="window.fixtureUnsafe=true">\n${'dataset-'.repeat(60)}`;
    mount(replication({ outcome: 'failed', details: { lastError: reported } }));
    const detail = await expand();
    const row = within(detail).getByText('Reported error').closest('tr')!;
    const value = row.querySelector('span.whitespace-pre-wrap')!;
    expect(value.textContent).toBe(reported);
    expect(value).toHaveClass('wrap-anywhere');
    expect(detail.querySelector('img, script, a')).toBeNull();
  });

  it.each([undefined, null, '', '   ', 17, false, { message: 'not a string' }])(
    'does not invent a clean result or render a non-text reported error: %s',
    async (lastError) => {
      mount(replication({ details: { lastError } }));
      const detail = within(await expand());
      expect(detail.queryByText('Reported error')).not.toBeInTheDocument();
      expect(detail.queryByText(/No errors/)).not.toBeInTheDocument();
    },
  );

  it('preserves the reported error even when the supplied outcome differs', async () => {
    mount(replication({ outcome: 'success', details: { lastError: 'Previous reported error' } }));
    const detail = within(await expand());
    expect(detail.getByText('Previous reported error')).toBeInTheDocument();
    expect(detail.getByText('Reported error')).toBeInTheDocument();
    expect(document.querySelector('[data-truenas-protection-row]')).toHaveAttribute(
      'data-truenas-protection-outcome',
      'success',
    );
  });

  it.each([null, 'not a timestamp'])(
    'keeps an unreadable last-run time unknown: %s',
    async (at) => {
      mount(replication({ completedAt: at, startedAt: null }));
      const detail = within(await expand());
      const row = detail.getByText('Last run').closest('tr')!;
      expect(within(row).getByText('Unknown')).toBeInTheDocument();
      expect(detail.queryByText('not a timestamp')).not.toBeInTheDocument();
    },
  );

  it('uses the existing start-only spelling as a last-run time, not a completion', async () => {
    mount(replication({ completedAt: null }));
    const detail = within(await expand());
    expect(detail.getByText('Last run').closest('tr')).not.toHaveTextContent('Unknown');
    expect(detail.queryByText('Completed', { exact: true })).not.toBeInTheDocument();
  });

  it.each([
    { startedAt: '2026-05-20T00:00:00Z', label: 'Created' },
    { startedAt: null, label: 'Observed' },
  ])('distinguishes a snapshot creation from the collection fallback: $label', async (testCase) => {
    mount(
      makeRecoveryPoint({
        id: 'snapshot-evidence',
        kind: 'snapshot',
        mode: 'snapshot',
        display: { itemLabel: 'tank/photos' },
        startedAt: testCase.startedAt,
      }),
    );
    const detail = within(await expand());
    expect(detail.getByText(testCase.label)).toBeInTheDocument();
    expect(detail.queryByText('Started', { exact: true })).not.toBeInTheDocument();
    expect(detail.queryByText('Completed', { exact: true })).not.toBeInTheDocument();
  });

  it('replaces and removes error evidence in the same open task on accepted refresh', async () => {
    const [points, setPoints] = createSignal([
      replication({ outcome: 'failed', details: { lastError: 'Original failure reason' } }),
    ]);
    render(() => (
      <TrueNASProtectionTable
        points={points()}
        emptyIcon={<span />}
        emptyTitle="No activity"
        emptyDescription="No activity"
      />
    ));
    await expand();
    expect(screen.getByText('Original failure reason')).toBeInTheDocument();
    setPoints([
      replication({
        outcome: 'failed',
        completedAt: '2026-05-21T01:00:00Z',
        details: { lastError: 'Replacement failure reason' },
      }),
    ]);
    expect(screen.getByText('Replacement failure reason')).toBeInTheDocument();
    expect(screen.queryByText('Original failure reason')).not.toBeInTheDocument();
    setPoints([replication({ outcome: 'success', completedAt: '2026-05-22T02:00:00Z' })]);
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    expect(detail.queryByText('Reported error')).not.toBeInTheDocument();
    expect(detail.queryByText('Replacement failure reason')).not.toBeInTheDocument();
    expect(detail.getByText('Last run').closest('tr')).toHaveTextContent('2026');
  });
});
