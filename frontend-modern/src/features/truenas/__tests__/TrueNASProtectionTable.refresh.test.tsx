import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { TrueNASProtectionTable } from '@/features/truenas/TrueNASProtectionTable';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import type { RecoveryPointsTransportResponse } from '@/types/recovery';

const apiFetchJSON = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON }));

const point = (id: string, outcome = 'success') => ({
  id,
  platform: 'truenas',
  kind: 'snapshot',
  mode: 'snapshot',
  outcome,
  completedAt: '2026-10-01T00:00:00Z',
  display: { subjectLabel: `tank/${id}`, subjectType: 'dataset' },
});

const deferred = () => {
  let resolve!: (value: RecoveryPointsTransportResponse) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<RecoveryPointsTransportResponse>((ok, fail) => {
    resolve = ok;
    reject = fail;
  });
  return { promise, resolve, reject };
};

function mountProtection() {
  let recovery!: ReturnType<typeof useRecoveryPoints>;
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => {
          recovery = useRecoveryPoints(() => ({ platform: 'truenas', page: 1, limit: 200 }));
          const [search, setSearch] = createSignal('');
          return (
            <TrueNASProtectionTable
              points={recovery.points()}
              loading={recovery.response.loading}
              error={recovery.response.error}
              onRefresh={() => void recovery.refetch()}
              search={search}
              onSearchChange={setSearch}
              emptyIcon={<span />}
              emptyTitle="No TrueNAS protection activity"
              emptyDescription="No events in this successful response."
            />
          );
        }}
      />
    </Router>
  ));
  return () => recovery;
}

const row = (id: string) => document.querySelector(`[data-truenas-protection-row="${id}"]`);
const notice = () => screen.getByRole('alert', { name: 'Protection refresh failed' });

async function expectLoaded() {
  await vi.waitFor(() => expect(row('apps')).not.toBeNull());
}

afterEach(() => {
  cleanup();
  apiFetchJSON.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
  localStorage.clear();
});

describe('TrueNAS protection refresh evidence', () => {
  it('labels retained events after a background failure and keeps their filters and details usable', async () => {
    apiFetchJSON.mockResolvedValueOnce({ data: [point('apps'), point('archive', 'failed')] });
    const recovery = mountProtection();
    await expectLoaded();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    apiFetchJSON.mockRejectedValueOnce(new Error('private-host/token=do-not-render'));
    await recovery().refetch({ background: true });

    expect(notice()).toHaveTextContent('Could not refresh TrueNAS protection.');
    expect(notice()).toHaveTextContent('Showing previously loaded events.');
    expect(notice()).toHaveTextContent('Outcomes and counts may be out of date;');
    expect(notice()).toHaveTextContent('newer activity may be missing.');
    expect(document.body).not.toHaveTextContent('private-host');
    expect(row('apps')).toHaveAttribute('data-truenas-protection-outcome', 'success');
    expect(row('archive')).toHaveAttribute('data-truenas-protection-outcome', 'failed');
    await fireEvent.click(screen.getByRole('button', { name: 'Attention, 1' }));
    expect(row('apps')).toBeNull();
    expect(row('archive')).not.toBeNull();
    expect(notice()).toBeInTheDocument();
    await fireEvent.click(row('archive')!);
    expect(screen.getByTestId('truenas-protection-detail')).toHaveTextContent('tank/archive');
    expect(notice()).toBeInTheDocument();
    expect(apiFetchJSON.mock.calls).toEqual([
      ['/api/recovery/points?page=1&limit=200&platform=truenas'],
      ['/api/recovery/points?page=1&limit=200&platform=truenas'],
    ]);
  });

  it('keeps the failure visible through an empty search and pending retry, then accepts the new response', async () => {
    apiFetchJSON.mockResolvedValueOnce({ data: [point('apps')] });
    const recovery = mountProtection();
    await expectLoaded();
    apiFetchJSON.mockRejectedValueOnce(new Error('transport unavailable'));
    await recovery().refetch({ background: true });
    await fireEvent.input(screen.getByPlaceholderText('Search snapshots or replication'), {
      target: { value: 'not-a-dataset' },
    });
    expect(screen.getByText('No protection events match current filters')).toBeInTheDocument();
    expect(notice()).toBeInTheDocument();

    const retry = deferred();
    apiFetchJSON.mockReturnValueOnce(retry.promise);
    const button = within(notice()).getByRole('button', { name: 'Refresh TrueNAS protection' });
    await fireEvent.click(button);
    expect(button).toBeDisabled();
    expect(button).toHaveTextContent('Refreshing...');
    expect(notice()).toBeInTheDocument();
    await fireEvent.click(button);
    expect(apiFetchJSON).toHaveBeenCalledTimes(3);
    retry.resolve({ data: [point('current', 'warning')] });
    await vi.waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
    await fireEvent.input(screen.getByPlaceholderText('Search snapshots or replication'), {
      target: { value: '' },
    });
    expect(row('apps')).toBeNull();
    expect(row('current')).toHaveAttribute('data-truenas-protection-outcome', 'warning');
  });

  it('does not clear a retained-data warning after another failed retry, but does after a successful empty read', async () => {
    apiFetchJSON.mockResolvedValueOnce({ data: [point('apps')] });
    const recovery = mountProtection();
    await expectLoaded();
    apiFetchJSON.mockRejectedValueOnce(new Error('transport unavailable'));
    await recovery().refetch({ background: true });
    apiFetchJSON.mockRejectedValueOnce(new Error('still unavailable'));
    await fireEvent.click(within(notice()).getByRole('button', { name: 'Refresh TrueNAS protection' }));
    await vi.waitFor(() =>
      expect(within(notice()).getByRole('button', { name: 'Refresh TrueNAS protection' })).toBeEnabled(),
    );
    expect(row('apps')).not.toBeNull();
    apiFetchJSON.mockResolvedValueOnce({ data: [] });
    await fireEvent.click(within(notice()).getByRole('button', { name: 'Refresh TrueNAS protection' }));
    await vi.waitFor(() => expect(screen.getByText('No TrueNAS protection activity')).toBeInTheDocument());
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(row('apps')).toBeNull();
  });

  it('distinguishes initial loading, failure and an accepted empty response without inventing events', async () => {
    const initial = deferred();
    apiFetchJSON.mockReturnValueOnce(initial.promise);
    mountProtection();
    expect(screen.getByText('Loading TrueNAS protection')).toBeInTheDocument();
    expect(screen.queryByText('No TrueNAS protection activity')).not.toBeInTheDocument();
    initial.reject(new Error('private-provider-detail'));
    await vi.waitFor(() => expect(screen.getByText('Could not load TrueNAS protection')).toBeInTheDocument());
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(document.body).not.toHaveTextContent('previously loaded');
    expect(document.body).not.toHaveTextContent('private-provider-detail');
    apiFetchJSON.mockResolvedValueOnce({ data: [] });
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh', exact: true }));
    await vi.waitFor(() => expect(screen.getByText('No TrueNAS protection activity')).toBeInTheDocument());
    expect(document.querySelector('[data-truenas-protection-row]')).toBeNull();
  });

  it.each([401, 403])('withdraws retained events after HTTP %s instead of offering stale private rows', async (status) => {
    apiFetchJSON.mockResolvedValueOnce({ data: [point('apps')] });
    const recovery = mountProtection();
    await expectLoaded();
    apiFetchJSON.mockRejectedValueOnce(Object.assign(new Error('access denied'), { status }));
    await recovery().refetch({ background: true });
    expect(row('apps')).toBeNull();
    expect(screen.getByText('Could not load TrueNAS protection')).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent('previously loaded');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
