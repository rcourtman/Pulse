import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { eventBus } from '@/stores/events';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { ProxmoxBackupJobsSection } from '../ProxmoxBackupJobsCard';

// The query, parser, apiClient and consuming card remain real. All responses
// and identities are synthetic; this does not exercise a guest or restore.
const fetchMock = vi.fn<typeof fetch>();
const counts = (success = 0, failed = 0) => ({
  success,
  failed,
  warning: 0,
  running: 0,
  unknown: 0,
});
const history = () => ({
  data: {
    historyState: 'complete',
    timezone: 'UTC',
    last24h: { historyState: 'complete', counts: counts(7, 2), failures: [] },
    days: [
      {
        date: '2026-11-17',
        historyState: 'complete',
        counts: counts(7, 2),
        failures: [
          {
            workloadId: 'synthetic-prior-id',
            workloadName: 'synthetic-prior-guest',
            instance: 'synthetic-prior-connection',
            at: '2026-11-17T01:00:00Z',
            outcome: 'failed',
          },
        ],
      },
    ],
  },
});
const reply = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const settle = () => vi.advanceTimersByTimeAsync(0);
const card = () => document.querySelector<HTMLElement>('[data-backup-jobs]');
const mount = async (onRevealRestorePoints = vi.fn()) => {
  const view = render(() => (
    <div role="region" aria-label="Backup inspection" tabIndex={-1}>
      <button type="button">Outside history</button>
      <ProxmoxBackupJobsSection
        selectedDay={() => '2026-11-17'}
        onSelectDay={vi.fn()}
        onRevealRestorePoints={onRevealRestorePoints}
      />
    </div>
  ));
  await settle();
  return view;
};
const assertReadOnlyContext = (org = 'synthetic-org-a') => {
  for (const [input, init] of fetchMock.mock.calls) {
    const url = new URL(String(input), 'https://pulse.invalid');
    expect(url.pathname).toBe('/api/backups/jobs');
    expect(Array.from(url.searchParams)).toEqual([
      ['timezone', Intl.DateTimeFormat().resolvedOptions().timeZone],
    ]);
    expect(init?.method ?? 'GET').toBe('GET');
    expect(init?.body).toBeUndefined();
    expect(init?.credentials).toBe('include');
    expect(init?.signal).toBeInstanceOf(AbortSignal);
    expect(new Headers(init?.headers).get('X-API-Token')).toBe('synthetic-read-token');
  }
  expect(getOrgID()).toBe(org);
};

beforeEach(() => {
  vi.useFakeTimers({ now: Date.parse('2026-11-17T12:00:00Z') });
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  sessionStorage.clear();
  clearAuth();
  setApiToken('synthetic-read-token');
  setOrgID('synthetic-org-a');
  resetCreateNonSuspendingQueryCacheForTest();
  fetchMock.mockImplementation(async () => reply(history()));
});

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  clearAuth();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('Backup jobs unavailable reads through the consuming card', () => {
  it.each([503, 403])('shows an initial %s as unknown, not absent history', async (status) => {
    fetchMock.mockImplementation(async () => reply({}, status));
    await mount();
    expect(card()).toHaveAttribute('data-backup-jobs', 'unavailable');
    expect(card()).toHaveTextContent('Backup job counts are unknown');
    expect(card()).not.toHaveTextContent(/no backups|0 ok|Invalid Date/i);
    expect(card()?.querySelector('[data-backup-jobs-pill]')).toBeNull();
    expect(screen.getByRole('button', { name: 'Retry backup jobs' })).toBeEnabled();
    if (status === 403) expect(card()).toHaveTextContent('Access denied');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    assertReadOnlyContext();
  });

  it.each([{}, { data: { days: [] } }, { data: { days: [null] } }])(
    'keeps a malformed initial response visible as unavailable: %j',
    async (body) => {
      fetchMock.mockImplementation(async () => reply(body));
      await mount();
      expect(card()).toHaveAttribute('data-backup-jobs', 'unavailable');
      expect(card()).not.toHaveTextContent(/no backups|0 ok|Invalid Date/i);
      assertReadOnlyContext();
    },
  );

  it('still omits valid no-job history and does not introduce a loading error', async () => {
    const body = history();
    body.data.last24h.counts = counts();
    body.data.days[0].counts = counts();
    body.data.days[0].failures = [];
    let release!: (value: Response) => void;
    fetchMock.mockImplementation(() => new Promise((resolve) => (release = resolve)));
    await mount();
    expect(card()).toBeNull();
    release(reply(body));
    await settle();
    expect(card()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it.each(['network', 'json'])(
    'contains an initial %s failure without hiding it',
    async (failure) => {
      fetchMock.mockImplementation(async () => {
        if (failure === 'network') throw new TypeError('Synthetic offline read');
        return new Response('{', { headers: { 'Content-Type': 'application/json' } });
      });
      await mount();
      expect(card()).toHaveAttribute('data-backup-jobs', 'unavailable');
      expect(card()).toHaveTextContent('Backup job counts are unknown');
      expect(card()).not.toHaveTextContent(/no backups|0 ok|synthetic offline/i);
    },
  );

  it('distinguishes an unreadable 24h window and day from observed zero', async () => {
    const body = history();
    Object.assign(body.data.last24h.counts, { success: 'not-a-count' });
    Object.assign(body.data.days[0].counts, { success: -1 });
    fetchMock.mockImplementation(async () => reply(body));
    await mount();
    expect(card()).toBeInTheDocument();
    expect(card()).toHaveTextContent('Last 24h unavailable');
    expect(card()).toHaveTextContent('counts unavailable');
    expect(card()).not.toHaveTextContent(/synthetic-prior-guest|no backups|0 ok/i);
    expect(card()?.querySelector('[data-backup-jobs-pill]')).toBeNull();
  });

  it('does not manufacture zeroes or stale failed guests after a transient poll error', async () => {
    await mount();
    expect(card()).toHaveTextContent('synthetic-prior-guest');
    fetchMock.mockImplementation(async () => reply({}, 503));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(card()).toHaveAttribute('data-backup-jobs', 'unavailable');
    expect(card()).toHaveTextContent('counts unavailable');
    expect(card()).not.toHaveTextContent(/synthetic-prior-guest|no backups|0 failed/i);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    assertReadOnlyContext();
  });

  it.each([403, 503])('keeps the selected restore-point shortcut after %s', async (status) => {
    const reveal = vi.fn();
    await mount(reveal);
    fetchMock.mockImplementation(async () => reply({}, status));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(card()).not.toHaveTextContent('synthetic-prior-guest');
    if (status === 403) {
      expect(card()).toHaveTextContent('Check your monitoring permissions.');
      expect(card()).not.toHaveTextContent('license plan');
    }
    await fireEvent.click(within(card()!).getByRole('button', { name: 'View restore points' }));
    expect(reveal).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    assertReadOnlyContext();
  });

  it('retains readable same-context observations while a poll is still pending', async () => {
    await mount();
    let release!: (value: Response) => void;
    fetchMock.mockImplementation(() => new Promise((resolve) => (release = resolve)));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(card()).toHaveTextContent('7 ok');
    expect(card()).toHaveTextContent('synthetic-prior-guest');
    release(reply({}, 503));
    await settle();
    expect(card()).toHaveAttribute('data-backup-jobs', 'unavailable');
    expect(card()).not.toHaveTextContent('synthetic-prior-guest');
  });

  it.each(['switch', 'round-trip', 'denied'])(
    'does not revive withdrawn history on a failed read after %s',
    async (change) => {
      await mount();
      expect(card()).toHaveTextContent('synthetic-prior-guest');
      fetchMock.mockImplementation(async () => reply({}, change === 'denied' ? 403 : 503));
      if (change === 'denied') await vi.advanceTimersByTimeAsync(60_000);
      else {
        setOrgID('synthetic-org-b');
        eventBus.emit('org_switched', 'synthetic-org-b');
        await settle();
        if (change === 'round-trip') {
          setOrgID('synthetic-org-a');
          eventBus.emit('org_switched', 'synthetic-org-a');
          await settle();
        }
      }
      expect(card()).toHaveAttribute('data-backup-jobs', 'unavailable');
      expect(card()?.querySelectorAll('[data-backup-jobs-day]')).toHaveLength(0);
      expect(card()).not.toHaveTextContent(/synthetic-prior|Nov 17|0 failed|no backups/i);
      const headers = fetchMock.mock.calls.map(([, init]) =>
        new Headers(init?.headers).get('X-Pulse-Org-ID'),
      );
      expect(headers).toEqual(
        change === 'denied'
          ? ['synthetic-org-a', 'synthetic-org-a']
          : change === 'switch'
            ? ['synthetic-org-a', 'synthetic-org-b']
            : ['synthetic-org-a', 'synthetic-org-b', 'synthetic-org-a'],
      );
      assertReadOnlyContext(change === 'switch' ? 'synthetic-org-b' : 'synthetic-org-a');
    },
  );

  it.each([false, true])(
    'bounds explicit retry and preserves focus ownership (departed=%s)',
    async (departed) => {
      fetchMock.mockImplementation(async () => reply({}, 503));
      await mount();
      const retry = screen.getByRole('button', { name: 'Retry backup jobs' });
      let release!: (value: Response) => void;
      fetchMock.mockImplementation(() => new Promise((resolve) => (release = resolve)));
      retry.focus();
      await fireEvent.click(retry);
      await fireEvent.click(retry);
      expect(retry).toHaveAttribute('aria-disabled', 'true');
      expect(fetchMock).toHaveBeenCalledTimes(2);
      if (departed) screen.getByRole('button', { name: 'Outside history' }).focus();
      release(reply(history()));
      await settle();
      expect(card()).toHaveTextContent('7 ok');
      expect(within(card()!).queryByRole('button', { name: 'Retry backup jobs' })).toBeNull();
      expect(document.activeElement).toBe(
        departed ? screen.getByRole('button', { name: 'Outside history' }) : card(),
      );
      assertReadOnlyContext();
    },
  );

  it('ignores the old organization poll after an organization switch', async () => {
    await mount();
    let release!: (value: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise((resolve) => (release = resolve)));
    await vi.advanceTimersByTimeAsync(60_000);
    const oldSignal = fetchMock.mock.calls.at(-1)?.[1]?.signal;
    fetchMock.mockImplementation(async () => reply({}, 503));
    setOrgID('synthetic-org-b');
    eventBus.emit('org_switched', 'synthetic-org-b');
    await settle();
    expect(oldSignal?.aborted).toBe(true);
    release(reply(history()));
    await settle();
    expect(card()).toHaveAttribute('data-backup-jobs', 'unavailable');
    expect(card()?.querySelectorAll('[data-backup-jobs-day]')).toHaveLength(0);
    expect(card()).not.toHaveTextContent('synthetic-prior-guest');
    assertReadOnlyContext('synthetic-org-b');
  });

  it.each([false, true])(
    'keeps focus in the inspection when a retry confirms no jobs (departed=%s)',
    async (departed) => {
      fetchMock.mockImplementation(async () => reply({}, 503));
      await mount();
      let release!: (value: Response) => void;
      fetchMock.mockImplementation(() => new Promise((resolve) => (release = resolve)));
      const retry = screen.getByRole('button', { name: 'Retry backup jobs' });
      retry.focus();
      await fireEvent.click(retry);
      const outside = screen.getByRole('button', { name: 'Outside history' });
      if (departed) outside.focus();
      const empty = history();
      empty.data.last24h.counts = counts();
      empty.data.days[0].counts = counts();
      empty.data.days[0].failures = [];
      release(reply(empty));
      await settle();
      expect(card()).toBeNull();
      expect(document.activeElement).toBe(
        departed ? outside : screen.getByRole('region', { name: 'Backup inspection' }),
      );
      expect(fetchMock).toHaveBeenCalledTimes(2);
    },
  );

  it('aborts a retry on unmount and ignores its late response', async () => {
    fetchMock.mockImplementation(async () => reply({}, 503));
    const view = await mount();
    let release!: (value: Response) => void;
    fetchMock.mockImplementation(() => new Promise((resolve) => (release = resolve)));
    await fireEvent.click(screen.getByRole('button', { name: 'Retry backup jobs' }));
    const signal = fetchMock.mock.calls.at(-1)?.[1]?.signal;
    view.unmount();
    expect(signal?.aborted).toBe(true);
    release(reply(history()));
    await settle();
    expect(card()).toBeNull();
    expect(document.body).not.toHaveTextContent('synthetic-prior-guest');
  });
});
