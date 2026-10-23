import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { Suspense } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { eventBus } from '@/stores/events';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { FindingsPanel } from '../FindingsPanel';

// The Patrol findings list is re-read every 30s by the app shell and every
// 60s by the Patrol page. These tests drive those reads through the real
// store, FindingsPanel and review sections, under a Suspense boundary like the
// app's route boundary, whose fallback replaces the whole page.

const getPatrolFindingsMock = vi.hoisted(() => vi.fn());
const getInvestigationMock = vi.hoisted(() => vi.fn());
const getInvestigationMessagesMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/patrol', async (importOriginal) => ({
  ...((await importOriginal()) as object),
  getPatrolFindings: getPatrolFindingsMock,
  getInvestigation: getInvestigationMock,
  getInvestigationMessages: getInvestigationMessagesMock,
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
  },
}));

const SEEN_AT = '2026-10-23T09:40:00Z';

type FindingRecord = Record<string, unknown> & { id: string };

// Patrol stamps the same time on every finding a run sees, so these tie on
// severity and recency, as real findings do.
const findingRecords = (): FindingRecord[] => [
  {
    id: 'finding-nginx',
    key: 'finding-nginx',
    severity: 'warning',
    category: 'reliability',
    resource_id: 'instance:node:102',
    resource_name: 'web-02',
    resource_type: 'vm',
    title: 'nginx on web-02 restarts every few hours',
    description: 'systemd restarted nginx 7 times in the last 24 hours.',
    recommendation: 'Check the nginx error log for the crash cause.',
    detected_at: '2026-10-22T07:00:00Z',
    last_seen_at: SEEN_AT,
    investigation_session_id: 'session-nginx',
    investigation_status: 'completed',
    investigation_outcome: 'needs_attention',
    investigation_attempts: 1,
  },
  {
    id: 'finding-disk',
    key: 'finding-disk',
    severity: 'warning',
    category: 'capacity',
    resource_id: 'instance:node:105',
    resource_name: 'db-01',
    resource_type: 'vm',
    title: 'db-01 data disk is 91% full',
    description: 'The PostgreSQL volume grew 4% in two days.',
    detected_at: '2026-10-22T07:00:00Z',
    last_seen_at: SEEN_AT,
  },
  {
    id: 'finding-backup',
    key: 'finding-backup',
    severity: 'warning',
    category: 'backup',
    resource_id: 'instance:node:201',
    resource_name: 'pbs-01',
    resource_type: 'host',
    title: 'Nightly backup on pbs-01 takes three times longer than usual',
    description: 'The last three backup jobs each ran for about 3 hours.',
    detected_at: '2026-10-22T07:00:00Z',
    last_seen_at: SEEN_AT,
  },
];

// Each read parses new objects, and the API returns tied findings in no
// fixed order.
const respondWith = (records: FindingRecord[]) => {
  getPatrolFindingsMock.mockImplementation(async () =>
    records.map((record) => JSON.parse(JSON.stringify(record))),
  );
};

const renderPanel = () => {
  const fallbackShown = { count: 0 };
  const observer = new MutationObserver((records) => {
    for (const record of records) {
      for (const node of record.addedNodes) {
        if (node.textContent?.includes('Loading view...')) fallbackShown.count += 1;
      }
    }
  });
  observer.observe(document.body, { childList: true, subtree: true });
  render(() => (
    <Router>
      <Route
        path="/"
        component={() => (
          <Suspense fallback={<div>Loading view...</div>}>
            <FindingsPanel findingsSource="patrol" />
          </Suspense>
        )}
      />
    </Router>
  ));
  return { fallbackShown, stop: () => observer.disconnect() };
};

const rowIds = () =>
  [...document.querySelectorAll('div[id^="finding-"]:not([id$="-details"])')].map((row) => row.id);
const reviewContent = () =>
  document.querySelector('aside#finding-finding-nginx-details .overflow-y-auto > div');

// Open the nginx finding's review and its investigation thread, as someone
// reading the finding would.
const openReview = async () => {
  await aiIntelligenceStore.loadPatrolFindings();
  fireEvent.click(
    await screen.findByRole('button', {
      name: 'Review issue for nginx on web-02 restarts every few hours',
    }),
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Show investigation thread' }));
  await screen.findByText('Checked memory history on web-02.');
};

describe('FindingsPanel across periodic refreshes', () => {
  beforeEach(() => {
    eventBus.emit('org_switched', 'default');
    resetCreateNonSuspendingQueryCacheForTest();
    getPatrolFindingsMock.mockReset();
    getInvestigationMock.mockReset();
    getInvestigationMessagesMock.mockReset();
    respondWith(findingRecords());
    // Every read parses a new object, as the API client does.
    getInvestigationMock.mockImplementation(async () => ({
      id: 'inv-nginx',
      finding_id: 'finding-nginx',
      session_id: 'session-nginx',
      status: 'completed',
      outcome: 'needs_attention',
      started_at: '2026-10-23T09:00:00Z',
      turn_count: 3,
      summary: 'Patrol traced the restarts to memory pressure on web-02.',
      tools_used: ['metrics.history'],
    }));
    getInvestigationMessagesMock.mockResolvedValue({
      investigation_id: 'inv-nginx',
      session_id: 'session-nginx',
      messages: [
        {
          id: 'message-1',
          role: 'assistant',
          content: 'Checked memory history on web-02.',
          timestamp: '2026-10-23T09:01:00Z',
        },
      ],
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps the open review, its rows and the page when a refresh returns the same findings', async () => {
    const { fallbackShown, stop } = renderPanel();
    await openReview();

    const row = document.getElementById('finding-finding-nginx');
    const content = reviewContent();
    const order = rowIds();
    expect(row).not.toBeNull();
    expect(content).not.toBeNull();
    expect(order).toHaveLength(3);

    // Same findings, new objects, in the opposite order.
    respondWith(findingRecords().reverse());
    await aiIntelligenceStore.loadPatrolFindings();
    await aiIntelligenceStore.loadPatrolFindings({ includeResolved: true });

    expect(document.getElementById('finding-finding-nginx')).toBe(row);
    expect(reviewContent()).toBe(content);
    expect(rowIds()).toEqual(order);
    expect(screen.getByRole('button', { name: 'Hide investigation thread' })).toBeInTheDocument();
    expect(getInvestigationMock).toHaveBeenCalledTimes(1);
    expect(getInvestigationMessagesMock).toHaveBeenCalledTimes(1);
    // Neither opening the review nor the refreshes suspended the route.
    expect(fallbackShown.count).toBe(0);
    stop();
  });

  it('keeps the open review when a refresh changes another finding', async () => {
    const { fallbackShown, stop } = renderPanel();
    await openReview();
    const row = document.getElementById('finding-finding-nginx');
    const content = reviewContent();

    respondWith(
      findingRecords().map((record) =>
        record.id === 'finding-disk' ? { ...record, title: 'db-01 data disk is 93% full' } : record,
      ),
    );
    await aiIntelligenceStore.loadPatrolFindings();

    expect(await screen.findByText('db-01 data disk is 93% full')).toBeInTheDocument();
    expect(document.getElementById('finding-finding-nginx')).toBe(row);
    expect(reviewContent()).toBe(content);
    expect(screen.getByRole('button', { name: 'Hide investigation thread' })).toBeInTheDocument();
    expect(getInvestigationMock).toHaveBeenCalledTimes(1);
    expect(fallbackShown.count).toBe(0);
    stop();
  });

  it('keeps the open review when a related finding in its group changes', async () => {
    const memory: FindingRecord = {
      id: 'finding-memory',
      key: 'finding-memory',
      severity: 'warning',
      category: 'performance',
      resource_id: 'instance:node:102',
      resource_name: 'web-02',
      resource_type: 'vm',
      title: 'Memory on web-02 peaks at 97%',
      description: 'Memory peaked at 97% before each restart.',
      detected_at: '2026-10-22T07:00:00Z',
      last_seen_at: SEEN_AT,
    };
    respondWith([...findingRecords(), memory]);
    const { fallbackShown, stop } = renderPanel();
    await openReview();
    const content = reviewContent();
    expect(screen.getByText('Memory on web-02 peaks at 97%')).toBeInTheDocument();

    respondWith([...findingRecords(), { ...memory, title: 'Memory on web-02 peaks at 99%' }]);
    await aiIntelligenceStore.loadPatrolFindings();

    // The related issues list updates inside the review, which stays as it was.
    expect(
      await screen.findByText('Memory on web-02 peaks at 99%', {
        selector: 'aside#finding-finding-nginx-details span',
      }),
    ).toBeInTheDocument();
    expect(reviewContent()).toBe(content);
    expect(screen.getByRole('button', { name: 'Hide investigation thread' })).toBeInTheDocument();
    expect(getInvestigationMock).toHaveBeenCalledTimes(1);
    expect(fallbackShown.count).toBe(0);
    stop();
  });

  it('keeps ages moving in the rows and review that stay mounted', async () => {
    // Rows used to be rebuilt by every refresh, which also refreshed their
    // ages. Mounted rows read the shared clock instead.
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-10-23T10:00:00Z'),
    });
    try {
      const { fallbackShown, stop } = renderPanel();
      await openReview();
      const row = document.getElementById('finding-finding-nginx');
      const review = document.querySelector('aside#finding-finding-nginx-details');
      const content = reviewContent();
      expect(row).toHaveTextContent('last seen 20m ago');
      expect(review).toHaveTextContent('last seen 20m ago');
      expect(review).toHaveTextContent('started 1h ago');
      expect(review).toHaveTextContent('59m ago');

      // No findings re-read: only the shared clock moves.
      const reads = getPatrolFindingsMock.mock.calls.length;
      vi.setSystemTime(Date.parse('2026-10-23T11:05:00Z'));
      vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);

      expect(document.getElementById('finding-finding-nginx')).toBe(row);
      expect(row).toHaveTextContent('last seen 1h ago');
      expect(review).toHaveTextContent('last seen 1h ago');
      expect(review).toHaveTextContent('started 2h ago');
      expect(review).not.toHaveTextContent('59m ago');
      expect(getPatrolFindingsMock.mock.calls.length).toBe(reads);

      // The same 30s also re-read the finished investigation, as a new
      // object. The thread belongs to the same run, so it is not read again,
      // and the review is not redrawn.
      await waitFor(() => expect(getInvestigationMock).toHaveBeenCalledTimes(2));
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(getInvestigationMessagesMock).toHaveBeenCalledTimes(1);
      expect(reviewContent()).toBe(content);
      expect(screen.getByRole('button', { name: 'Hide investigation thread' })).toBeInTheDocument();
      expect(fallbackShown.count).toBe(0);
      stop();
    } finally {
      vi.useRealTimers();
    }
  });

  it('shows a change to the open finding without suspending the route or reloading its investigation', async () => {
    const { fallbackShown, stop } = renderPanel();
    await openReview();

    respondWith(
      findingRecords().map((record) =>
        record.id === 'finding-nginx'
          ? { ...record, description: 'systemd restarted nginx 9 times in the last 24 hours.' }
          : record,
      ),
    );
    await aiIntelligenceStore.loadPatrolFindings();

    // The review shows the new description at once, and the investigation
    // it already read stays on screen while it is read again.
    expect(
      screen.getAllByText('systemd restarted nginx 9 times in the last 24 hours.').length,
    ).toBeGreaterThan(0);
    expect(
      screen.getByText('Patrol traced the restarts to memory pressure on web-02.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Loading investigation...')).not.toBeInTheDocument();
    await waitFor(() => expect(getInvestigationMock).toHaveBeenCalledTimes(2));
    expect(fallbackShown.count).toBe(0);
    stop();
  });
});
