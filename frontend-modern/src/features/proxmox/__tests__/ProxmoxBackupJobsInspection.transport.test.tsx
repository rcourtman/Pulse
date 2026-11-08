import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { ProxmoxBackupJobsSection } from '../ProxmoxBackupJobsCard';
import type { BackupJobsPayload } from '../proxmoxBackupJobs';
import { identityArchive, identityGuest, identityPBS } from '../__fixtures__/backupIdentity';

// Exercise the production summaryAside composition, not substitute day buttons
// or callback spies. The history reader/parser, card, table, router, query owner
// and apiClient stay real. This is DOM/HTTP proof, not browser/guest recovery.
vi.mock('@/api/license', () => ({
  LicenseAPI: {
    getRuntimeCapabilities: async () => ({
      capabilities: [],
      limits: [],
      blocked_capabilities: [],
      hosted_mode: false,
      max_history_days: 7,
      runtime: { build: 'community', label: 'Synthetic Community runtime' },
    }),
  },
}));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => 1200 }),
}));
const pointer = vi.hoisted(() => ({ hover: true }));
vi.mock('@/components/shared/hoverCapability', () => ({
  supportsHoverTooltips: () => pointer.hover,
}));
vi.mock('@/utils/logger', () => ({
  logger: { warn: vi.fn(), debug: vi.fn(), error: vi.fn() },
}));

const org = 'synthetic/backup-estate';
const token = 'synthetic-monitoring-read-token';
const now = new Date(2026, 9, 4, 12).valueOf();
const beforeMidnight = new Date(2026, 9, 3, 23, 59).toISOString();
const afterMidnight = new Date(2026, 9, 4, 0, 1).toISOString();
const guest = identityGuest('synthetic-guest', 'east', 'pve-1');
const counts = (success = 0, failed = 0) => ({
  success,
  failed,
  warning: 0,
  running: 0,
  unknown: 0,
});
const failure = (name: string) => ({
  workloadId: `synthetic-${name}`,
  workloadName: name,
  instance: 'east',
  at: new Date(2026, 9, 2, 10).toISOString(),
  outcome: 'failed',
});
const json = (data: unknown) =>
  new Response(JSON.stringify(data), { headers: { 'Content-Type': 'application/json' } });
const fetchMock = vi.fn<typeof fetch>();
const scrollIntoView = vi.fn();
let jobs: BackupJobsPayload;
let archiveTime: string;

const card = () => document.querySelector<HTMLElement>('[data-backup-jobs]')!;
const chosenDay = () => card().querySelector('[data-backup-jobs-chosen-day]');
const rows = () =>
  Array.from(document.querySelectorAll<HTMLElement>('[data-proxmox-backup-row="recoverable"]'));
const rowIDs = () => rows().map((row) => row.querySelector('button')?.getAttribute('aria-label'));
const dayParam = () => new URLSearchParams(window.location.search).get('day');
const day = (label: RegExp) => within(card()).getByRole('button', { name: label });
const settle = () => vi.advanceTimersByTimeAsync(0);
const pills = () =>
  Array.from(card().querySelectorAll('[data-backup-jobs-pill]'), (pill) => pill.textContent);

function expectReadOnlyContext() {
  for (const [url, options] of fetchMock.mock.calls) {
    const parsed = new URL(String(url), 'https://pulse.invalid');
    expect(options?.method ?? 'GET').toBe('GET');
    expect(options?.body).toBeUndefined();
    expect(options?.signal).toBeInstanceOf(AbortSignal);
    expect(options?.credentials).toBe('include');
    const headers = new Headers(options?.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(org);
    expect(headers.get('X-API-Token')).toBe(token);
    expect([
      '/api/backups/pve',
      '/api/backups/pbs',
      '/api/backups/jobs',
      '/api/recovery/postures',
    ]).toContain(parsed.pathname);
    if (parsed.pathname === '/api/backups/jobs') {
      expect(Array.from(parsed.searchParams)).toEqual([
        ['timezone', Intl.DateTimeFormat().resolvedOptions().timeZone],
      ]);
    }
  }
  expect(getOrgID()).toBe(org);
}

async function mount(path = '/proxmox/backups/date') {
  window.history.replaceState({}, '', path);
  const view = render(() => (
    <Router>
      <Route
        path="/*"
        component={() => (
          <ProxmoxBackupsTable
            emptyIcon={<span />}
            workloads={[guest]}
            summaryAside={(days) => <ProxmoxBackupJobsSection {...days} />}
          />
        )}
      />
    </Router>
  ));
  await settle();
  expect(card()).toBeInTheDocument();
  const restorePoints = document.querySelector<HTMLElement>('[data-backup-restore-points]')!;
  // Deliberately below the fold: assert navigation intent, not jsdom layout.
  vi.spyOn(restorePoints, 'getBoundingClientRect').mockReturnValue({
    top: window.innerHeight + 100,
  } as DOMRect);
  return { ...view, restorePoints };
}

beforeEach(() => {
  vi.useFakeTimers({ now });
  vi.clearAllMocks();
  pointer.hover = true;
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('scrollTo', vi.fn());
  vi.spyOn(HTMLElement.prototype, 'scrollIntoView').mockImplementation(scrollIntoView);
  sessionStorage.clear();
  clearAuth();
  setApiToken(token);
  setOrgID(org);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  resetCreateNonSuspendingQueryCacheForTest();
  archiveTime = beforeMidnight;
  jobs = {
    historyState: 'complete',
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    // Deliberately unlike the two inventory rows and the selected day's
    // counts. Jobs, restore points and protection are independent evidence.
    last24h: { historyState: 'complete', counts: counts(83, 2), failures: [] },
    days: [
      {
        date: '2026-10-02',
        historyState: 'complete',
        counts: counts(0, 2),
        failures: [failure('database'), failure('mail')],
      },
      { date: '2026-10-03', historyState: 'complete', counts: counts(17), failures: [] },
      { date: '2026-10-04', historyState: 'partial', counts: counts(3), failures: [] },
    ],
  };
  fetchMock.mockImplementation(async (input) => {
    switch (new URL(String(input), 'https://pulse.invalid').pathname) {
      case '/api/backups/jobs':
        return json({ data: jobs });
      case '/api/backups/pve':
        return json({
          data: {
            storageBackups: [
              identityArchive({
                id: 'archive-before-midnight',
                time: archiveTime,
                ctime: Date.parse(archiveTime) / 1000,
              }),
            ],
            guestSnapshots: [],
            backupTasks: [],
          },
        });
      case '/api/backups/pbs':
        return json({
          data: { backups: [identityPBS({ id: 'pbs-after-midnight', backupTime: afterMidnight })] },
        });
      case '/api/recovery/postures':
        // A successful job cannot turn unknown restore protection green.
        return json({ data: [], policy: protectionPolicy });
      default:
        throw new Error(`Unexpected synthetic backup request: ${String(input)}`);
    }
  });
});

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  clearAuth();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('Backup jobs through the real restore-point inspection', () => {
  it('joins the local calendar request and real bars to the matching restore points', async () => {
    await mount('/proxmox/backups/coverage?reason=source-failed');
    expect(fetchMock).toHaveBeenCalledTimes(4);
    expect(pills()).toEqual(['83 ok', '2 failed']);
    expect(document.querySelector('[data-backup-verdict]')).toHaveAttribute(
      'data-backup-verdict',
      'unknown',
    );

    await fireEvent.click(day(/Oct 3: 17 ok/));
    await settle();
    expect(window.location.pathname).toBe('/proxmox/backups/date');
    expect(dayParam()).toBe('2026-10-03');
    expect(new URLSearchParams(window.location.search).get('reason')).toBeNull();
    expect(day(/Oct 3: 17 ok/)).toHaveAttribute('aria-pressed', 'true');
    expect(rowIDs()).toEqual([expect.stringContaining('Identifier: archive-before-midnight')]);
    expect(pills()).toEqual(['83 ok', '2 failed']);

    await fireEvent.click(day(/Oct 4: 3 ok \(not fully observed\)/));
    await settle();
    expect(dayParam()).toBe('2026-10-04');
    expect(rowIDs()).toEqual([expect.stringContaining('Identifier: pbs-after-midnight')]);
    expect(day(/Oct 3: 17 ok/)).toHaveAttribute('aria-pressed', 'false');
    expect(chosenDay()).toHaveTextContent('not fully observed');

    await fireEvent.click(day(/Oct 4: 3 ok/));
    await settle();
    expect(dayParam()).toBeNull();
    expect(rows()).toHaveLength(2);
    expect(chosenDay()).toBeNull();
    expect(document.querySelector('[data-backup-verdict]')).toHaveAttribute(
      'data-backup-verdict',
      'unknown',
    );
    expect(fetchMock).toHaveBeenCalledTimes(4);
    expectReadOnlyContext();
  });

  it.each([
    { device: 'touch', hover: false, automaticReveals: 0 },
    { device: 'pointer', hover: true, automaticReveals: 1 },
  ])('keeps failed-day names inspectable on $device before explicit reveal', async ({
    hover,
    automaticReveals,
  }) => {
    pointer.hover = hover;
    const { restorePoints } = await mount();
    await fireEvent.click(day(/Oct 2: 2 failed/));
    await settle();
    expect(dayParam()).toBe('2026-10-02');
    expect(chosenDay()).toHaveTextContent('2 failed: database, mail');
    expect(rows()).toHaveLength(0);
    expect(screen.getByText('No restore points from Oct 2, 2026')).toBeInTheDocument();
    expect(screen.getByText(/A failed backup usually leaves none/)).toBeInTheDocument();
    expect(scrollIntoView).toHaveBeenCalledTimes(automaticReveals);

    await fireEvent.click(within(card()).getByRole('button', { name: 'View restore points' }));
    await settle();
    expect(scrollIntoView).toHaveBeenCalledTimes(automaticReveals + 1);
    expect(scrollIntoView.mock.contexts.at(-1)).toBe(restorePoints);
    expect(dayParam()).toBe('2026-10-02');
    expect(chosenDay()).toHaveTextContent('database, mail');
    expect(fetchMock).toHaveBeenCalledTimes(4);
    expectReadOnlyContext();
  });

  it('preserves saved filters and does not blame an observed day for their empty result', async () => {
    await mount('/proxmox/backups/date?q=no-such-guest&source=pbs');
    await fireEvent.click(day(/Oct 4: 3 ok/));
    await settle();
    const params = new URLSearchParams(window.location.search);
    expect(params.get('q')).toBe('no-such-guest');
    expect(params.get('source')).toBe('pbs');
    expect(params.get('day')).toBe('2026-10-04');
    expect(screen.getByText('No recoverable artifacts match current filters')).toBeInTheDocument();
    expect(screen.queryByText(/^No restore points from/)).toBeNull();
    expect(chosenDay()).toHaveTextContent('not fully observed');
    expect(fetchMock).toHaveBeenCalledTimes(4);
    expectReadOnlyContext();
  });

  it('keeps the selected date, not the old bar index, through accepted independent polls', async () => {
    await mount('/proxmox/backups/date?day=2026-10-03');
    expect(day(/Oct 3: 17 ok/)).toHaveAttribute('aria-pressed', 'true');
    expect(rowIDs()).toEqual([expect.stringContaining('archive-before-midnight')]);
    jobs = {
      ...jobs,
      last24h: { historyState: 'partial', counts: counts(91, 4), failures: [] },
      days: [
        { ...jobs.days[1], counts: counts(19) },
        jobs.days[0],
        jobs.days[2],
      ],
    };
    archiveTime = afterMidnight;
    await vi.advanceTimersByTimeAsync(60_000);
    expect(dayParam()).toBe('2026-10-03');
    expect(day(/Oct 3: 19 ok/)).toHaveAttribute('aria-pressed', 'true');
    expect(day(/Oct 2: 2 failed/)).toHaveAttribute('aria-pressed', 'false');
    expect(chosenDay()).toHaveTextContent('19 ok');
    expect(pills()).toEqual(['91 ok', '4 failed']);
    expect(card().querySelector('[data-backup-jobs-incomplete]')).toHaveTextContent('incomplete data');
    expect(rows()).toHaveLength(0);
    expect(screen.getByText('No restore points from Oct 3, 2026')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(11);
    expectReadOnlyContext();
  });
});
