import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 390 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));
const east = identityGuest('east-100', 'east', 'pve-1');
const west = identityGuest('west-100', 'west', 'pve-1');
const literal = 'undated-a <img src=x onerror=alert(1)>';
let repaired: boolean;
const time = (index: number) => new Date(BACKUP_IDENTITY_NOW - (index + 1) * 60_000).toISOString();
const unknownTime = (index: number) =>
  repaired ? time(index) : index === 0 ? '2026-09-31T10:00:00Z' : '2026-10-04T10:00:00';
function inventories() {
  return {
    backups: [
      ...Array.from({ length: 12 }, (_, index) =>
        identityPBS({ id: `pbs-${index}`, namespace: 'east', backupTime: time(index) }),
      ),
      ...[0, 1].map((index) =>
        identityPBS({
          id: `pbs-unknown-${index}`,
          namespace: 'east',
          backupTime: unknownTime(index),
          datastore: index === 0 ? literal : 'undated-b',
        }),
      ),
      ...(['running', 'failed'] as const).map((state, index) =>
        identityPBS({
          id: `pbs-${state}`,
          namespace: 'east',
          backupTime: unknownTime(index),
          datastore: state,
          inProgress: true,
          writeActivityObserved: true,
          writeActive: state === 'running',
        }),
      ),
    ],
    storageBackups: [
      ...Array.from({ length: 12 }, (_, index) =>
        identityArchive({ id: `archive-${index}`, time: time(index) }),
      ),
      ...[0, 1].map((index) =>
        identityArchive({
          id: `archive-unknown-${index}`,
          time: unknownTime(index),
          storage: `archive-undated-${index}`,
        }),
      ),
    ],
    guestSnapshots: [
      ...Array.from({ length: 12 }, (_, index) =>
        identitySnapshot({ id: `snapshot-${index}`, time: time(index) }),
      ),
      ...[0, 1].map((index) =>
        identitySnapshot({ id: `snapshot-unknown-${index}`, time: unknownTime(index) }),
      ),
    ],
    backupTasks: [],
  };
}
function mount() {
  api.fetch.mockImplementation((url: string) => {
    const points = inventories();
    return Promise.resolve(
      new Response(
        JSON.stringify({ data: url === '/api/backups/pbs' ? { backups: points.backups } : points }),
        {
          headers: { 'Content-Type': 'application/json' },
        },
      ),
    );
  });
  api.json.mockResolvedValue({ data: [], policy: protectionPolicy });
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={[east, west]} />}
      />
    </Router>
  ));
}
const evidenceTable = (panel: HTMLElement) =>
  within(
    within(panel).getByText('Restore evidence', { exact: true }).closest('div')!.parentElement!,
  ).getByRole('table');
const entries = (panel: HTMLElement) => evidenceTable(panel).querySelectorAll('tbody > tr');
async function open(name = 'east-100') {
  fireEvent.click(
    await screen.findByRole('button', { name: new RegExp(`Expand details for ${name}`) }),
  );
  return screen.findByTestId('proxmox-coverage-detail');
}
beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date', 'setInterval', 'clearInterval'] });
  resetCreateNonSuspendingQueryCacheForTest();
  api.width = 390;
  repaired = false;
  window.history.replaceState({}, '', '/proxmox/backups/coverage');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('Coverage uncertain evidence inspection through parsed inventory', () => {
  it.each([1200, 390, 320])(
    'keeps uncertainty inspectable and progressively exposes every entry at %ipx',
    async (width) => {
      api.width = width;
      mount();
      const panel = await open();
      expect(panel).toHaveTextContent('8 entries have an unavailable or future date');
      expect(panel).toHaveTextContent('sample from each affected source and state');
      expect(entries(panel)).toHaveLength(8);
      const evidence = evidenceTable(panel);
      expect(within(evidence).getAllByText('Unknown', { exact: true })).toHaveLength(5);
      expect(evidence).toHaveTextContent('Running');
      expect(evidence).toHaveTextContent('Failed');
      expect(evidence).toHaveTextContent(literal);
      expect(evidence.querySelector('img')).toBeNull();
      expect(evidence).toHaveTextContent('archive-undated-0');
      expect(evidence).not.toHaveTextContent('undated-b');
      const more = within(panel).getByRole('button', { name: 'Show 8 more evidence entries' });
      more.focus();
      for (const count of [16, 24, 32, 40, 44]) {
        fireEvent.click(more);
        expect(entries(panel)).toHaveLength(count);
        expect(document.activeElement).toBe(more);
      }
      expect(more).toHaveTextContent('All 44 evidence entries shown');
      expect(more).toHaveAttribute('aria-disabled', 'true');
      expect(panel).not.toHaveTextContent('sample from each affected source and state');
      expect(evidenceTable(panel)).toHaveTextContent('undated-b');
      expect(within(evidenceTable(panel)).getAllByText('Unknown', { exact: true })).toHaveLength(8);
      fireEvent.click(more);
      expect(entries(panel)).toHaveLength(44);
      expect(api.fetch).toHaveBeenCalledTimes(2);
      fireEvent.click(screen.getByRole('button', { name: /Collapse details for east-100/ }));
      const peer = await open('west-100');
      expect(peer).not.toHaveTextContent('Restore evidence');
      expect(peer).not.toHaveTextContent(literal);
      fireEvent.click(screen.getByRole('button', { name: /Expand details for east-100/ }));
      expect(entries(await screen.findByTestId('proxmox-coverage-detail'))).toHaveLength(8);
    },
  );

  it('preserves expanded evidence and focus while polling removes and restores uncertainty', async () => {
    mount();
    const panel = await open();
    const more = within(panel).getByRole('button', { name: 'Show 8 more evidence entries' });
    more.focus();
    fireEvent.click(more);
    expect(entries(panel)).toHaveLength(16);
    repaired = true;
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(panel).not.toHaveTextContent('unavailable or future date'));
    expect(entries(panel)).toHaveLength(16);
    expect(within(evidenceTable(panel)).queryByText('Unknown', { exact: true })).toBeNull();
    expect(document.activeElement).toBe(more);
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    repaired = false;
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(panel).toHaveTextContent('8 entries have an unavailable or future date'),
    );
    expect(entries(panel)).toHaveLength(16);
    expect(evidenceTable(panel)).toHaveTextContent(literal);
    expect(document.activeElement).toBe(more);
    expect(api.fetch).toHaveBeenCalledTimes(6);
  });
});
