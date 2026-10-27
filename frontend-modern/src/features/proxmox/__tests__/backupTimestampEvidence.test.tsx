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
  identityTask,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 390 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));
const guest = identityGuest('east-100', 'east', 'pve-1');
let repaired: boolean;
const response = (data: unknown) =>
  new Response(JSON.stringify({ data }), { headers: { 'Content-Type': 'application/json' } });
function mount() {
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(
      response(
        url === '/api/backups/pbs'
          ? {
              backups: [
                identityPBS({
                  backupTime: repaired ? '2026-10-04T10:00:00Z' : '2026-09-31T10:00:00Z',
                }),
              ],
            }
          : {
              backupTasks: [
                identityTask({
                  id: 'malformed-failure',
                  status: 'failed',
                  error: 'Retained failure',
                  startTime: repaired ? '2026-10-04T09:00:00Z' : '2026-10-04T09:00:00',
                }),
                identityTask({ id: 'dated-ok' }),
              ],
              storageBackups: [
                identityArchive({ time: repaired ? '2026-10-04T11:00:00+02:00' : '2026-10-04' }),
              ],
              guestSnapshots: [
                identitySnapshot({
                  time: repaired ? '2026-10-04T11:30:00.123456789Z' : '2026-09-31T11:30:00Z',
                }),
              ],
            },
      ),
    ),
  );
  api.json.mockResolvedValue({ data: [], policy: protectionPolicy });
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} />}
      />
    </Router>
  ));
}
const section = (panel: HTMLElement, label: string) =>
  within(panel).getByText(label, { exact: true, selector: 'th' }).closest('tbody')!;
const value = (parent: HTMLElement, label: string) =>
  within(parent).getByText(label, { exact: true, selector: 'td' }).closest('tr')!.lastElementChild!;
const evidenceTable = (panel: HTMLElement) =>
  within(
    within(panel).getByText('Restore evidence', { exact: true }).closest('div')!.parentElement!,
  ).getByRole('table');
async function open() {
  const toggle = await screen.findByRole('button', { name: /Expand details for east-100/ });
  fireEvent.click(toggle);
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

describe('malformed timestamp evidence through the parsed HTTP consumer', () => {
  it.each([1200, 390, 320])('does not guess coverage ages or latest OK at %ipx', async (width) => {
    api.width = width;
    mount();
    const panel = await open();
    expect(value(section(panel, 'Task order'), 'Latest task')).toHaveTextContent('Unknown');
    expect(value(section(panel, 'Possible latest task 1'), 'Started')).toHaveTextContent('Unknown');
    expect(value(section(panel, 'Possible latest task 1'), 'Duration')).toHaveTextContent(
      'Not reported',
    );
    expect(value(section(panel, 'Possible latest task 1'), 'Error')).toHaveTextContent(
      'Retained failure',
    );
    expect(value(section(panel, 'Possible latest task 2'), 'Status')).toHaveTextContent('OK');
    const evidence = evidenceTable(panel);
    expect(within(evidence).getAllByText('Unknown', { exact: true })).toHaveLength(3);
    expect(document.body).toHaveTextContent('3 completed restore points');
    expect(screen.getByRole('button', { name: '1 unknown' })).toBeInTheDocument();
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });

  it.each([1200, 390, 320])(
    'retains every undated restore point without chart activity at %ipx',
    async (width) => {
      window.history.replaceState({}, '', '/proxmox/backups/date');
      api.width = width;
      mount();
      await waitFor(() =>
        expect(document.querySelectorAll('[data-proxmox-backup-row="recoverable"]')).toHaveLength(
          3,
        ),
      );
      const rows = Array.from(
        document.querySelectorAll<HTMLElement>('[data-proxmox-backup-row="recoverable"]'),
      );
      for (const row of rows)
        expect(within(row).getByText('Unknown', { exact: true })).toBeInTheDocument();
      expect(document.body).toHaveTextContent('No completed restore points with known dates');
      expect(document.body).toHaveTextContent('Their ages are unknown.');
      fireEvent.click(within(rows[0]).getByRole('button', { name: /Expand details/ }));
      const panel = await screen.findByTestId('proxmox-recoverable-detail');
      expect(value(section(panel, 'Restore point'), 'Created')).toHaveTextContent('Unknown');
      expect(panel).not.toHaveTextContent('Invalid Date');
      expect(api.fetch).toHaveBeenCalledTimes(2);
    },
  );

  it('restores valid dates and ordered tasks on the ordinary poll without replacing the open panel', async () => {
    mount();
    const panel = await open();
    const toggle = screen.getByRole('button', { name: /Collapse details for east-100/ });
    toggle.focus();
    expect(value(section(panel, 'Task order'), 'Latest task')).toHaveTextContent('Unknown');
    repaired = true;
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(value(section(panel, 'Latest task'), 'Task ID')).toHaveTextContent('dated-ok'),
    );
    expect(within(panel).queryByText('Task order', { exact: true, selector: 'th' })).toBeNull();
    expect(panel).not.toHaveTextContent('Retained failure');
    const evidence = evidenceTable(panel);
    expect(within(evidence).queryByText('Unknown', { exact: true })).toBeNull();
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(document.activeElement).toBe(toggle);
    expect(screen.getByRole('button', { name: '1 unknown' })).toBeInTheDocument();
    expect(api.fetch).toHaveBeenCalledTimes(4);
  });
});
