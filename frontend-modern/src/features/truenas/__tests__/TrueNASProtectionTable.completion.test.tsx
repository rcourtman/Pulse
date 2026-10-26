import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { TrueNASProtectionTable } from '@/features/truenas/TrueNASProtectionTable';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import type { RecoveryPointTransport } from '@/types/recovery';

const apiFetchJSON = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON }));

const startedAt = '2026-10-26T01:00:00Z';
const completedAt = '2026-10-26T01:15:00Z';
const point = (id: string, overrides: Partial<RecoveryPointTransport> = {}) => ({
  id,
  platform: 'truenas',
  kind: 'backup',
  mode: 'remote',
  outcome: 'success',
  startedAt,
  display: { subjectLabel: `tank/${id}`, subjectType: 'dataset' },
  ...overrides,
});

function mountProtection() {
  let recovery!: ReturnType<typeof useRecoveryPoints>;
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => {
          recovery = useRecoveryPoints(() => ({ platform: 'truenas', page: 1, limit: 200 }));
          return (
            <TrueNASProtectionTable
              points={recovery.points()}
              loading={recovery.response.loading}
              error={recovery.response.error}
              emptyIcon={<span />}
              emptyTitle="No protection"
              emptyDescription="No events in this successful response."
            />
          );
        }}
      />
    </Router>
  ));
  return () => recovery;
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

describe('TrueNAS protection completion evidence', () => {
  it('pairs phone column priorities with their cells and preserves the hidden fields in details', async () => {
    apiFetchJSON.mockResolvedValueOnce({
      data: [
        point('phone', {
          completedAt,
          display: {
            subjectLabel: 'tank/departmental-archives',
            subjectType: 'dataset',
            nodeHostLabel: 'nas-east',
          },
          details: {
            taskName: 'replicate-departmental-archives-to-offsite',
            targetDataset: 'vault/offsite/departmental-archives',
          },
        }),
      ],
    });
    mountProtection();
    await vi.waitFor(() => expect(row('phone')).not.toBeNull());
    const cells = row('phone').querySelectorAll('td');
    for (const [label, index] of [
      ['Artifact', 1],
      ['Target', 2],
    ] as const) {
      expect(screen.getByRole('columnheader', { name: label })).toHaveClass(
        'platform-table-phone-hidden',
      );
      expect(cells[index]).toHaveClass('platform-table-phone-hidden');
    }
    for (const [label, index] of [
      ['Dataset', 0],
      ['Completed', 3],
      ['Outcome', 5],
    ] as const) {
      expect(screen.getByRole('columnheader', { name: new RegExp(label) })).not.toHaveClass(
        'platform-table-phone-hidden',
      );
      expect(cells[index]).not.toHaveClass('platform-table-phone-hidden');
    }
    expect(screen.getByRole('columnheader', { name: /Completed/ })).toHaveClass(
      'platform-table-mobile-w-30',
    );
    expect(screen.getByRole('columnheader', { name: 'Outcome' })).toHaveClass(
      'platform-table-mobile-w-30',
    );
    await fireEvent.click(
      within(row('phone')).getByRole('button', { name: /details.*tank\/departmental-archives/i }),
    );
    const detail = within(screen.getByTestId('truenas-protection-detail'));
    expect(detail.getByText('Artifact', { exact: true }).closest('tr')).toHaveTextContent(
      'replicate-departmental-archives-to-offsite',
    );
    expect(
      detail.getByText('Target', { selector: 'td', exact: true }).closest('tr'),
    ).toHaveTextContent('vault/offsite/departmental-archives');
    expect(detail.getByText('Host', { exact: true }).closest('tr')).toHaveTextContent('nas-east');
    expect(detail.getByText('Completed', { exact: true }).closest('tr')).toHaveTextContent('2026');
    expect(detail.getByText('Outcome', { exact: true }).closest('tr')).toHaveTextContent('Healthy');
  });

  it('never substitutes a start or a running last-run date for completion in rows or details', async () => {
    apiFetchJSON.mockResolvedValueOnce({
      data: [
        point('running', { outcome: 'running', completedAt }),
        point('started-only'),
        point('failed-started-only', { outcome: 'failed' }),
        point('unknown', { outcome: 'unknown', startedAt: undefined }),
        point('invalid', { completedAt: 'not-a-date' }),
        point('completed', { completedAt }),
        point('failed-completed', { outcome: 'failed', completedAt }),
      ],
    });
    mountProtection();
    await vi.waitFor(() => expect(row('completed')).not.toBeNull());

    for (const id of ['running', 'started-only', 'failed-started-only', 'unknown', 'invalid']) {
      expect(completion(id)).toHaveTextContent(/^\s*-\s*$/);
    }
    for (const id of ['completed', 'failed-completed']) {
      expect(completion(id)).not.toHaveTextContent(/^\s*-\s*$/);
    }
    expect(row('running')).toHaveAttribute('data-truenas-protection-outcome', 'running');
    expect(row('failed-started-only')).toHaveAttribute('data-truenas-protection-outcome', 'failed');

    for (const id of ['running', 'started-only', 'failed-started-only']) {
      await fireEvent.click(row(id));
      const detail = within(screen.getByTestId('truenas-protection-detail'));
      expect(detail.getByText('Started', { exact: true }).closest('tr')).toHaveTextContent('2026');
      expect(detail.queryByText('Completed', { exact: true })).not.toBeInTheDocument();
      await fireEvent.click(detail.getByRole('button', { name: /^Collapse .* details$/ }));
    }
    await fireEvent.click(row('completed'));
    expect(
      within(screen.getByTestId('truenas-protection-detail'))
        .getByText('Completed', { exact: true })
        .closest('tr'),
    ).toHaveTextContent('2026');
  });

  it('sorts completion by actual finished dates with absent completions last in either direction', async () => {
    apiFetchJSON.mockResolvedValueOnce({
      data: [
        point('running', { outcome: 'running', completedAt: '2026-10-26T04:00:00Z' }),
        point('started-only', { startedAt: '2026-10-26T05:00:00Z' }),
        point('older', { completedAt }),
        point('newer', { completedAt: '2026-10-26T02:00:00Z' }),
      ],
    });
    mountProtection();
    await vi.waitFor(() => expect(row('newer')).not.toBeNull());
    const sort = screen.getByRole('columnheader', { name: /Completed/ });
    await fireEvent.click(sort);
    expect(rowIDs().slice(0, 2)).toEqual(['newer', 'older']);
    expect(rowIDs().slice(2).sort()).toEqual(['running', 'started-only']);
    await fireEvent.click(sort);
    expect(rowIDs().slice(0, 2)).toEqual(['older', 'newer']);
    expect(rowIDs().slice(2).sort()).toEqual(['running', 'started-only']);
  });

  it('accepts a finished read and updates the already-open running details without a remount', async () => {
    apiFetchJSON.mockResolvedValueOnce({ data: [point('active', { outcome: 'running' })] });
    const recovery = mountProtection();
    await vi.waitFor(() => expect(row('active')).not.toBeNull());
    await fireEvent.click(row('active'));
    expect(completion('active')).toHaveTextContent(/^\s*-\s*$/);
    expect(
      within(screen.getByTestId('truenas-protection-detail')).queryByText('Completed', {
        exact: true,
      }),
    ).not.toBeInTheDocument();

    apiFetchJSON.mockResolvedValueOnce({ data: [point('active', { completedAt })] });
    await recovery().refetch({ background: true });
    expect(row('active')).toHaveAttribute('data-truenas-protection-outcome', 'success');
    expect(completion('active')).not.toHaveTextContent(/^\s*-\s*$/);
    expect(
      within(screen.getByTestId('truenas-protection-detail')).getByText('Completed', {
        exact: true,
      }),
    ).toBeInTheDocument();
    expect(apiFetchJSON.mock.calls).toEqual([
      ['/api/recovery/points?page=1&limit=200&platform=truenas'],
      ['/api/recovery/points?page=1&limit=200&platform=truenas'],
    ]);
  });

  it('keeps the completion sort control labelled through direction changes, filters and an empty result', async () => {
    apiFetchJSON.mockResolvedValueOnce({
      data: [
        point('running', { outcome: 'running', completedAt }),
        point('completed', { completedAt }),
      ],
    });
    mountProtection();
    await vi.waitFor(() => expect(row('completed')).not.toBeNull());
    const heading = () => screen.getByRole('columnheader', { name: /Completed/ });
    const button = () => within(heading()).getByRole('button', { name: /Completed/ });
    for (let i = 0; i < 3; i++) {
      button().focus();
      await fireEvent.click(button());
      expect(button()).toHaveFocus();
      expect(heading()).toHaveAttribute('scope', 'col');
    }

    await fireEvent.click(screen.getByRole('button', { name: 'Running, 1' }));
    expect(button()).toBeInTheDocument();
    expect(rowIDs()).toEqual(['running']);
    expect(completion('running')).toHaveTextContent(/^\s*-\s*$/);
    const search = screen.getByPlaceholderText('Search snapshots or replication');
    await fireEvent.input(search, { target: { value: 'not-a-dataset' } });
    expect(screen.getByText('No protection events match current filters')).toBeInTheDocument();
    await fireEvent.input(search, { target: { value: '' } });
    expect(button()).toBeInTheDocument();
    await fireEvent.click(button());
    expect(heading()).toHaveAttribute('aria-sort', 'descending');
    expect(apiFetchJSON).toHaveBeenCalledTimes(1);
  });
});
