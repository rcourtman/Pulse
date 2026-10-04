import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { useResourceDetailDrawerHistoryState } from '../useResourceDetailDrawerHistoryState';
import type { Resource, ResourceChange } from '@/types/resource';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';

const api = vi.hoisted(() => ({ facets: vi.fn(), intelligence: vi.fn(), actions: vi.fn() }));
vi.mock('@/api/resources', () => ({ ResourceAPI: { getFacetBundle: api.facets } }));
vi.mock('@/api/ai', () => ({ AIAPI: { getResourceIntelligence: api.intelligence } }));
vi.mock('@/api/actionAudit', () => ({ ActionAuditAPI: { listActionAudits: api.actions } }));

const change = (id: string): ResourceChange => ({
  id,
  resourceId: 'pbs-a',
  observedAt: '2026-10-03T03:00:00Z',
  kind: 'restart',
  sourceType: 'platform_event',
  confidence: 'high',
});
const resource: Resource = {
  id: 'pbs-a',
  name: 'PBS A',
  displayName: 'PBS A',
  type: 'pbs',
  platformId: 'pbs-a',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1,
  recentChanges: [change('embedded-event')],
  facetCounts: { recentChanges: 7 },
};
const bundle = (id = 'remote-event') => ({
  recentChanges: [change(id)],
  counts: { recentChanges: 1 },
});
const failure = (status: number) =>
  Object.assign(new Error('untrusted response detail'), { status });

function Probe(props: { remote?: boolean }) {
  const state = useResourceDetailDrawerHistoryState({
    resource,
    enableRemoteHistory: props.remote,
  });
  return (
    <>
      <output data-testid="history-state">
        {JSON.stringify({
          ids: state.historyRecentChanges().map((event) => event.id),
          count: state.resourceTimelineCount(),
          error: state.facetBundleError(),
          label: state.historyLoadingLabel(),
          actionsError: state.actionAuditError(),
          actionsLabel: state.actionAuditLoadingLabel(),
          available: state.actionAuditAvailable(),
        })}
      </output>
      <button onClick={() => void state.refetchHistoryFacets()}>Refresh changes</button>
      <button onClick={() => state.setTimelineKindFilter('restart')}>Filter changes</button>
      <button onClick={() => state.setTimelineKindFilter('')}>Clear changes filter</button>
      <button onClick={() => void state.refetchActionAudits()}>Refresh actions</button>
    </>
  );
}
const read = () => JSON.parse(screen.getByTestId('history-state').textContent!);
beforeEach(() => {
  api.facets.mockReset().mockResolvedValue(bundle());
  api.intelligence.mockReset().mockResolvedValue(null);
  api.actions.mockReset().mockResolvedValue({ audits: [], count: 0, available: false });
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
});

describe('resource evidence access boundaries', () => {
  it.each([401, 403])('does not substitute embedded events after final HTTP %s', async (status) => {
    render(() => <Probe />);
    await waitFor(() => expect(read().ids).toEqual(['remote-event']));
    api.facets.mockRejectedValueOnce(failure(status));
    fireEvent.click(screen.getByText('Refresh changes'));
    await waitFor(() => expect(read().error).not.toBe(''));
    expect(read().ids).toEqual([]);
    expect(read().count).toBe(0);
    expect(read().error).toBe(
      status === 401
        ? 'Sign in again or check your API token.'
        : 'Access denied. Check your permissions and license plan.',
    );
    expect(read().label).toBe('Changes unavailable');
  });

  it('withdraws embedded events when the initial read is denied', async () => {
    api.facets.mockRejectedValueOnce(failure(403));
    render(() => <Probe />);
    await waitFor(() => expect(read().error).not.toBe(''));
    expect(read().ids).toEqual([]);
    expect(read().count).toBe(0);
  });

  it('does not substitute an unfiltered bundle after a filtered access denial', async () => {
    render(() => <Probe />);
    await waitFor(() => expect(read().ids).toEqual(['remote-event']));
    api.facets.mockRejectedValueOnce(failure(403));
    fireEvent.click(screen.getByText('Filter changes'));
    await waitFor(() => expect(read().error).not.toBe(''));
    expect(read().ids).toEqual([]);
    expect(read().count).toBe(0);
    // The unfiltered request retains its own successful authorisation.
    fireEvent.click(screen.getByText('Clear changes filter'));
    await waitFor(() => expect(read().ids).toEqual(['remote-event']));
  });

  it('keeps denied evidence withdrawn during retry, then accepts a fresh success', async () => {
    render(() => <Probe />);
    await waitFor(() => expect(read().ids).toEqual(['remote-event']));
    api.facets.mockRejectedValueOnce(failure(403));
    fireEvent.click(screen.getByText('Refresh changes'));
    await waitFor(() => expect(read().error).not.toBe(''));
    let resolve!: (value: ReturnType<typeof bundle>) => void;
    api.facets.mockImplementationOnce(
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    fireEvent.click(screen.getByText('Refresh changes'));
    await waitFor(() => expect(read().label).toBe('Refreshing changes...'));
    expect(read().ids).toEqual([]);
    resolve(bundle('fresh-event'));
    await waitFor(() => expect(read().ids).toEqual(['fresh-event']));
    expect(read().error).toBe('');
  });

  it.each([503])(
    'exposes an action read failure even without retained actions: %s',
    async (status) => {
      render(() => <Probe />);
      await waitFor(() => expect(read().actionsLabel).toBe('Actions loaded'));
      api.actions.mockRejectedValueOnce(failure(status));
      fireEvent.click(screen.getByText('Refresh actions'));
      await waitFor(() => expect(read().actionsError).not.toBe(''));
      expect(read().actionsLabel).toBe('Actions unavailable');
      if (status === 401 || status === 403) expect(read().actionsError).not.toContain('untrusted');
      expect(read().available).toBe(false);
    },
  );

  it('retains a successful remote snapshot after a transient outage', async () => {
    render(() => <Probe />);
    await waitFor(() => expect(read().ids).toEqual(['remote-event']));
    api.facets.mockRejectedValueOnce(failure(503));
    fireEvent.click(screen.getByText('Refresh changes'));
    await waitFor(() => expect(read().error).not.toBe(''));
    expect(read().ids).toEqual(['remote-event']);
    expect(read().count).toBe(1);
  });

  it('keeps snapshot-only embedded history without making remote reads', async () => {
    render(() => <Probe remote={false} />);
    expect(read().ids).toEqual(['embedded-event']);
    expect(read().count).toBe(7);
    expect(api.facets).not.toHaveBeenCalled();
    expect(api.actions).not.toHaveBeenCalled();
  });
});
