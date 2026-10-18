import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { TrueNASProtectionTable } from '@/features/truenas/TrueNASProtectionTable';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';

const apiFetchJSONMock = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: apiFetchJSONMock }));

const point = (id = 'snapshot-a') => ({
  id,
  provider: 'truenas',
  kind: 'snapshot',
  mode: 'snapshot',
  outcome: 'success',
  completedAt: '2026-10-18T08:00:00Z',
  display: { subjectLabel: 'tank/apps' },
});

const mountProtection = () => {
  let state!: ReturnType<typeof useRecoveryPoints>;
  const view = render(() => {
    state = useRecoveryPoints(() => ({ platform: 'truenas' }));
    return (
      <TrueNASProtectionTable
        points={state.points()}
        loading={state.response.loading}
        error={state.response.error}
        onRefresh={() => void state.refetch()}
        emptyIcon={<span />}
        emptyTitle="No protection activity"
        emptyDescription="No records were returned."
      />
    );
  });
  return { ...view, read: () => state };
};

afterEach(() => {
  cleanup();
  apiFetchJSONMock.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
});

describe('TrueNAS protection read availability', () => {
  it('shows unreadable data as unavailable, never as an empty inventory', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ data: 'not-an-inventory' });
    mountProtection();
    expect(await screen.findByText('Could not load TrueNAS protection')).toBeInTheDocument();
    expect(screen.queryByText('No protection activity')).not.toBeInTheDocument();
  });

  it('retains records with a visible qualification after an unreadable refresh', async () => {
    apiFetchJSONMock
      .mockResolvedValueOnce({ data: [point()] })
      .mockResolvedValueOnce({ data: 'not-an-inventory' });
    const view = mountProtection();
    await screen.findByText('tank/apps');
    await view.read().refetch();
    expect(document.querySelector('[data-truenas-protection-row="snapshot-a"]')).not.toBeNull();
    expect(screen.getByText(/Could not refresh TrueNAS protection\./)).toHaveTextContent(
      'last loaded records',
    );
    expect(screen.queryByText('No protection activity')).not.toBeInTheDocument();
  });

  it('exposes a failed refresh through retries and cached remounts, then clears it on success', async () => {
    apiFetchJSONMock
      .mockResolvedValueOnce({ data: [point()] })
      .mockRejectedValueOnce(new Error('private transport diagnostic'));
    const first = mountProtection();
    await screen.findByText('tank/apps');
    await first.read().refetch();
    expect(screen.getByText(/Could not refresh TrueNAS protection\./)).toHaveTextContent(
      'last loaded records',
    );
    expect(screen.queryByText('private transport diagnostic')).not.toBeInTheDocument();
    first.unmount();

    let resolve!: (value: unknown) => void;
    apiFetchJSONMock.mockImplementationOnce(
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    mountProtection();
    await vi.waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalledTimes(3));
    expect(screen.getByText(/Could not refresh TrueNAS protection\./)).toHaveTextContent(
      'last loaded records',
    );
    expect(screen.getByRole('button', { name: 'Refreshing protection activity…' })).toBeDisabled();
    resolve({ data: [point('snapshot-b')] });
    await vi.waitFor(() =>
      expect(screen.queryByText(/Could not refresh TrueNAS protection\./)).not.toBeInTheDocument(),
    );
    expect(document.querySelector('[data-truenas-protection-row="snapshot-b"]')).not.toBeNull();
  });

  it('allows a readable empty inventory and recovers from a first-read failure using Refresh', async () => {
    apiFetchJSONMock
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({ data: [] });
    mountProtection();
    await screen.findByText('Could not load TrueNAS protection');
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(await screen.findByText('No protection activity')).toBeInTheDocument();
    expect(screen.queryByText('Could not load TrueNAS protection')).not.toBeInTheDocument();
  });
});

describe('TrueNAS protection empty and access boundaries', () => {
  it('keeps a failed refresh of an empty page unavailable throughout a pending retry', async () => {
    apiFetchJSONMock
      .mockResolvedValueOnce({ data: [] })
      .mockRejectedValueOnce(new Error('offline'));
    const view = mountProtection();
    await screen.findByText('No protection activity');
    await view.read().refetch();
    expect(screen.getByText('Could not load TrueNAS protection')).toBeInTheDocument();
    expect(screen.queryByText('No protection activity')).not.toBeInTheDocument();
    let resolve!: (value: unknown) => void;
    apiFetchJSONMock.mockImplementationOnce(
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(screen.getByText('Could not load TrueNAS protection')).toBeInTheDocument();
    expect(screen.queryByText('Loading TrueNAS protection')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeDisabled();
    resolve({ data: [] });
    await screen.findByText('No protection activity');
  });

  it('rejects an incomplete page atomically, instead of replacing the inventory with its usable rows', async () => {
    apiFetchJSONMock
      .mockResolvedValueOnce({ data: [point()] })
      .mockResolvedValueOnce({ data: [point('partial'), null] });
    const view = mountProtection();
    await screen.findByText('tank/apps');
    await view.read().refetch();
    expect(document.querySelector('[data-truenas-protection-row="snapshot-a"]')).not.toBeNull();
    expect(document.querySelector('[data-truenas-protection-row="partial"]')).toBeNull();
    expect(screen.getByText(/Could not refresh TrueNAS protection\./)).toHaveTextContent(
      'last loaded records',
    );
  });

  it('withdraws the retained page when the server actually denies access', async () => {
    apiFetchJSONMock
      .mockResolvedValueOnce({ data: [point()] })
      .mockRejectedValueOnce({ status: 403, message: 'private access diagnostic' });
    const view = mountProtection();
    await screen.findByText('tank/apps');
    await view.read().refetch();
    expect(screen.getByText('Could not load TrueNAS protection')).toBeInTheDocument();
    expect(document.querySelector('[data-truenas-protection-row]')).toBeNull();
    expect(screen.queryByText('private access diagnostic')).not.toBeInTheDocument();
    expect(screen.queryByText('No protection activity')).not.toBeInTheDocument();
  });
});
