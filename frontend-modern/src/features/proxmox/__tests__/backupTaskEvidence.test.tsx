import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { BackupTask, PVEBackupsPayload } from '@/types/api';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { formatPlatformTableDateTimeValue } from '@/features/platformPage/sharedPlatformPage';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityTask,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 390 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));

const failure =
  'backup target unavailable\n<svg onload="throw 1">' + 'long-provider-detail-'.repeat(8);
const failedTask = (extra: Partial<BackupTask> = {}) =>
  identityTask({ status: 'failed', error: failure, ...extra });
let tasks: BackupTask[];
const guests = ['east', 'west'].map((instance) => ({
  ...identityGuest(`guest-${instance}`, instance, 'pve-1'),
  name: 'same-name',
  displayName: 'same-name',
}));
const response = (data: unknown) =>
  new Response(JSON.stringify({ data }), { headers: { 'Content-Type': 'application/json' } });
function mount() {
  api.fetch.mockImplementation((url: string) => {
    const pve: PVEBackupsPayload = {
      backupTasks: tasks,
      storageBackups: [identityArchive()],
      guestSnapshots: [],
    };
    return Promise.resolve(response(url === '/api/backups/pbs' ? { backups: [] } : pve));
  });
  // A provider task result cannot confer protection, even if it says OK.
  api.json.mockResolvedValue({ data: [], policy: protectionPolicy });
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={guests} />}
      />
    </Router>
  ));
}
async function open(scope = 'east') {
  const toggle = await screen.findByRole('button', {
    name: `Expand details for same-name (VM 100) · PVE connection: ${scope}`,
  });
  fireEvent.click(toggle);
  return screen.findByTestId('proxmox-coverage-detail');
}
const taskSection = (panel: HTMLElement) =>
  within(panel).getByText('Latest task', { exact: true, selector: 'th' }).closest('tbody')!;
const value = (section: HTMLElement, label: string) =>
  within(section).getByText(label, { exact: true, selector: 'td' }).closest('tr')!
    .lastElementChild!;

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date', 'setInterval', 'clearInterval'] });
  api.width = 390;
  tasks = [failedTask()];
  resetCreateNonSuspendingQueryCacheForTest();
  window.history.replaceState({}, '', '/proxmox/backups/coverage');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('latest backup task evidence through the inventory consumer', () => {
  it.each([330, 1200])('exposes scoped, literal failure evidence at %ipx', async (width) => {
    api.width = width;
    mount();
    const panel = await open();
    const section = taskSection(panel);
    expect(value(section, 'Status')).toHaveTextContent('Failed');
    expect(value(section, 'Started')).toHaveTextContent(
      formatPlatformTableDateTimeValue(Date.parse(tasks[0].startTime)),
    );
    expect(value(section, 'Duration')).toHaveTextContent('5m 0s');
    expect(value(section, 'Task ID')).toHaveTextContent('task-100');
    expect(value(section, 'Error').textContent).toBe(failure);
    expect(value(section, 'Error').firstElementChild).toHaveClass(
      'whitespace-pre-wrap',
      'wrap-anywhere',
    );
    expect(value(section, 'Error').querySelector('svg')).toBeNull();
    expect(section).toHaveTextContent(
      'Task status does not verify a restore or guest responsiveness.',
    );
    expect(panel).toHaveTextContent('Restore evidence');
    expect(screen.getByRole('button', { name: '2 unknown' })).toBeInTheDocument();
    expect(api.fetch).toHaveBeenCalledTimes(2);

    fireEvent.click(within(panel).getByRole('button', { name: 'Collapse same-name details' }));
    const other = await open('west');
    expect(within(other).queryByText('Latest task', { exact: true, selector: 'th' })).toBeNull();
    expect(other.textContent).not.toContain(failure);
    expect(other).not.toHaveTextContent('task-100');
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });

  it('keeps missing failure text, invalid start and unordered duration explicit', async () => {
    tasks = [failedTask({ error: '  ', startTime: 'invalid', endTime: 'also-invalid' })];
    mount();
    const section = taskSection(await open());
    expect(value(section, 'Started')).toHaveTextContent('Unknown');
    expect(value(section, 'Duration')).toHaveTextContent('Not reported');
    expect(value(section, 'Error')).toHaveTextContent('Not reported');
    expect(section).not.toHaveTextContent('Invalid Date');
    expect(section).not.toHaveTextContent('also-invalid');
  });

  it.each([
    { status: 'running', endTime: undefined, duration: 'Not reported' },
    { status: 'OK', endTime: '2026-10-04T10:59:00Z', duration: 'Not reported' },
    { status: 'OK', endTime: '2026-10-04T11:00:00.100Z', duration: '0s' },
  ])('does not invent completion or protection for $status / $duration', async (extra) => {
    tasks = [identityTask(extra)];
    mount();
    const section = taskSection(await open());
    expect(value(section, 'Status')).toHaveTextContent(
      extra.status === 'running' ? 'Running' : 'OK',
    );
    expect(value(section, 'Duration')).toHaveTextContent(extra.duration);
    expect(within(section).queryByText('Error', { exact: true, selector: 'td' })).toBeNull();
    expect(screen.getByRole('button', { name: '2 unknown' })).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent('A current verified backup is available.');
  });

  it('updates failure, recovery and task removal in the same open panel on ordinary polling', async () => {
    mount();
    const panel = await open();
    expect(value(taskSection(panel), 'Error').textContent).toBe(failure);
    const toggle = screen.getByRole('button', {
      name: 'Collapse details for same-name (VM 100) · PVE connection: east',
    });
    toggle.focus();
    const summary = toggle.closest('tr');
    tasks = [
      identityTask({
        id: 'task-recovered',
        startTime: '2026-10-04T11:30:00Z',
        endTime: '2026-10-04T11:31:05Z',
      }),
    ];
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(value(taskSection(panel), 'Task ID')).toHaveTextContent('task-recovered'),
    );
    expect(value(taskSection(panel), 'Status')).toHaveTextContent('OK');
    expect(value(taskSection(panel), 'Duration')).toHaveTextContent('1m 5s');
    expect(
      within(taskSection(panel)).queryByText('Error', { exact: true, selector: 'td' }),
    ).toBeNull();
    expect(panel.textContent).not.toContain(failure);
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(toggle.closest('tr')).toBe(summary);
    expect(document.activeElement).toBe(toggle);
    expect(screen.getByRole('button', { name: '2 unknown' })).toBeInTheDocument();
    expect(api.fetch).toHaveBeenCalledTimes(4);

    tasks = [];
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(within(panel).queryByText('Latest task', { exact: true, selector: 'th' })).toBeNull(),
    );
    expect(panel).not.toHaveTextContent('task-recovered');
    expect(panel.textContent).not.toContain(failure);
    expect(panel).toHaveTextContent('Restore evidence');
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(document.activeElement).toBe(toggle);
    expect(api.fetch).toHaveBeenCalledTimes(6);
  });
});
