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
    expect(
      within(section).getByText('Task status does not verify a restore or guest responsiveness.'),
    ).toHaveClass('whitespace-normal');
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

describe('uncertain task chronology through the inventory consumer', () => {
  const chronology = (panel: HTMLElement) =>
    within(panel).getByText('Task order', { exact: true, selector: 'th' }).closest('tbody')!;
  const candidate = (panel: HTMLElement, n: number) =>
    within(panel)
      .getByText(`Possible latest task ${n}`, { exact: true, selector: 'th' })
      .closest('tbody')!;
  const setAmbiguous = (startTime = 'invalid') => {
    tasks = [failedTask({ id: 'uncertain-failure', startTime }), identityTask({ id: 'dated-ok' })];
  };

  it.each([
    { start: '', width: 1200 },
    { start: 'invalid', width: 330 },
    { start: '2027-01-01T00:00:00Z', width: 1200 },
    { start: '2026-10-04T11:00:00Z', width: 330 },
  ])(
    'withdraws a false latest OK and exposes both possible tasks: $start / $width',
    async ({ start, width }) => {
      api.width = width;
      setAmbiguous(start);
      mount();
      const panel = await open();
      expect(value(chronology(panel), 'Latest task')).toHaveTextContent('Unknown');
      expect(chronology(panel)).toHaveTextContent(
        'Task start times do not establish which task ran last. Possible latest tasks are listed below without a time order.',
      );
      expect(within(panel).queryByText('Latest task', { exact: true, selector: 'th' })).toBeNull();
      const candidates = [candidate(panel, 1), candidate(panel, 2)];
      const failureSection = candidates.find(
        (section) => value(section, 'Task ID').textContent === 'uncertain-failure',
      )!;
      expect(value(failureSection, 'Status')).toHaveTextContent('Failed');
      expect(value(failureSection, 'Error').textContent).toBe(failure);
      expect(value(failureSection, 'Error').querySelector('svg')).toBeNull();
      expect(value(failureSection, 'Started')).toHaveTextContent(
        start === '2026-10-04T11:00:00Z'
          ? formatPlatformTableDateTimeValue(Date.parse(start))
          : 'Unknown',
      );
      const summary = screen
        .getByRole('button', {
          name: 'Collapse details for same-name (VM 100) · PVE connection: east',
        })
        .closest('tr')!;
      expect(summary).toHaveTextContent('Latest task: Unknown');
      expect(summary).not.toHaveTextContent('OK');
      expect(panel).toHaveTextContent('dated-ok');
      expect(panel).toHaveTextContent('Restore evidence');
      expect(screen.getByRole('button', { name: '2 unknown' })).toBeInTheDocument();
      expect(api.fetch).toHaveBeenCalledTimes(2);

      fireEvent.click(within(panel).getByRole('button', { name: 'Collapse same-name details' }));
      const west = await open('west');
      expect(west).not.toHaveTextContent('Possible latest task');
      expect(west.textContent).not.toContain(failure);
      expect(west).not.toHaveTextContent('Task order');
    },
  );

  it('keeps the task column and all possible tasks when every start is unknown', async () => {
    tasks = [
      failedTask({ id: 'failure-a', startTime: 'invalid' }),
      identityTask({ id: 'ok-b', startTime: '' }),
    ];
    mount();
    const panel = await open();
    expect(screen.getByRole('columnheader', { name: 'Job' })).toBeInTheDocument();
    expect(value(chronology(panel), 'Latest task')).toHaveTextContent('Unknown');
    expect(value(candidate(panel, 1), 'Started')).toHaveTextContent('Unknown');
    expect(value(candidate(panel, 2), 'Started')).toHaveTextContent('Unknown');
    expect(panel).toHaveTextContent('failure-a');
    expect(panel).toHaveTextContent('ok-b');
  });

  it('reconciles uncertain, ordered, uncertain and absent tasks in the open keyed panel on ordinary polls', async () => {
    setAmbiguous();
    mount();
    const panel = await open();
    expect(value(chronology(panel), 'Latest task')).toHaveTextContent('Unknown');
    const toggle = screen.getByRole('button', {
      name: 'Collapse details for same-name (VM 100) · PVE connection: east',
    });
    toggle.focus();
    const summary = toggle.closest('tr');
    tasks = [failedTask({ startTime: '2026-10-03T11:00:00Z' }), identityTask({ id: 'ordered-ok' })];
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(value(taskSection(panel), 'Task ID')).toHaveTextContent('ordered-ok'),
    );
    expect(value(taskSection(panel), 'Status')).toHaveTextContent('OK');
    expect(panel).not.toHaveTextContent('Task order');
    expect(panel).not.toHaveTextContent('Possible latest task');
    expect(panel.textContent).not.toContain(failure);

    setAmbiguous('2027-01-01T00:00:00Z');
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(value(chronology(panel), 'Latest task')).toHaveTextContent('Unknown'),
    );
    expect(panel.textContent).toContain(failure);
    expect(panel).not.toHaveTextContent('ordered-ok');
    expect(within(panel).queryByText('Latest task', { exact: true, selector: 'th' })).toBeNull();

    tasks = [];
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(panel).not.toHaveTextContent('Task order'));
    expect(panel).not.toHaveTextContent('Possible latest task');
    expect(panel.textContent).not.toContain(failure);
    expect(panel).toHaveTextContent('Restore evidence');
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(toggle.closest('tr')).toBe(summary);
    expect(document.activeElement).toBe(toggle);
    expect(api.fetch).toHaveBeenCalledTimes(8);
  });

  it('finds scoped possible-task errors and IDs through the existing saved-view search', async () => {
    vi.useRealTimers();
    vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date'] });
    setAmbiguous();
    mount();
    await open();
    const input = screen.getByPlaceholderText(
      'Search backups by workload, node, source, or status',
    );
    fireEvent.input(input, { target: { value: 'uncertain-failure' } });
    await waitFor(() =>
      expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(1),
    );
    await waitFor(() => expect(window.location.search).toContain('q=uncertain-failure'));
    expect(screen.getByTestId('proxmox-coverage-detail').textContent).toContain(failure);
    fireEvent.input(input, { target: { value: 'backup target unavailable' } });
    await waitFor(() => expect(window.location.search).toContain('backup+target+unavailable'));
    expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(1);
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });
});

// A provider history can contribute 200 tasks per node. If its clock is wrong,
// none of those starts can rule out a task as latest. Keep the complete evidence
// searchable without creating hundreds of detail cards on a single disclosure.
describe('bounded uncertain-task inspection', () => {
  const history = () =>
    Array.from({ length: 200 }, (_, index) =>
      identityTask({
        id: `history-${String(index).padStart(3, '0')}`,
        startTime: index === 199 ? '2026-10-04T11:00:00Z' : 'invalid',
        status:
          index === 198 ? 'failed' : index === 199 ? 'running' : index === 197 ? 'warning' : 'OK',
        error: index === 198 ? failure : undefined,
      }),
    );
  const cards = (panel: HTMLElement) =>
    within(panel).queryAllByText(/^Possible latest task \d+$/, { selector: 'th' });

  it.each([320, 1200])(
    'bounds a 200-task disclosure and preserves outcome witnesses at %ipx',
    async (width) => {
      api.width = width;
      tasks = history();
      mount();
      const panel = await open();
      expect(cards(panel)).toHaveLength(8);
      expect(panel).toHaveTextContent('Showing 8 of 200 possible latest tasks.');
      expect(panel).toHaveTextContent(
        'Failed: 1 · Running: 1 · OK: 197 · Other status: 1 · With error text: 1 · Unknown start: 199',
      );
      for (const id of ['history-197', 'history-198', 'history-199'])
        expect(panel).toHaveTextContent(id);
      expect(panel.textContent).toContain(failure);
      expect(panel.querySelector('svg[onload]')).toBeNull();
      expect(panel).toHaveTextContent('It is not a time order.');
      expect(panel).not.toHaveTextContent('history-150');
      expect(screen.getByRole('button', { name: '2 unknown' })).toBeInTheDocument();
      expect(api.fetch).toHaveBeenCalledTimes(2);
    },
  );

  it('allows deliberate complete inspection without more reads, and keeps the same focused control', async () => {
    tasks = history();
    mount();
    const panel = await open();
    const more = within(panel).getByRole('button', { name: 'Show 8 more possible latest tasks' });
    more.focus();
    for (let expected = 16; expected <= 200; expected += 8) {
      fireEvent.click(more);
      expect(cards(panel)).toHaveLength(expected);
      expect(document.activeElement).toBe(more);
    }
    expect(more).toHaveTextContent('All 200 possible latest tasks shown');
    expect(more).toHaveAttribute('aria-disabled', 'true');
    fireEvent.click(more);
    expect(cards(panel)).toHaveLength(200);
    for (const task of tasks) expect(panel).toHaveTextContent(task.id);
    expect(api.fetch).toHaveBeenCalledTimes(2);
    expect(api.json).toHaveBeenCalledTimes(1);
  });

  it('retains expansion and focus through replacement, shrink, ordered and empty polling', async () => {
    tasks = history();
    mount();
    const panel = await open();
    const more = within(panel).getByRole('button', { name: 'Show 8 more possible latest tasks' });
    more.focus();
    fireEvent.click(more);
    expect(cards(panel)).toHaveLength(16);

    tasks = history().map((task) => ({
      ...task,
      error: task.error ? 'updated literal <b>failure</b>' : undefined,
    }));
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(panel).toHaveTextContent('updated literal <b>failure</b>'));
    expect(cards(panel)).toHaveLength(16);
    expect(document.activeElement).toBe(more);
    tasks = tasks.slice(0, 3);
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(more).toHaveTextContent('All 3 possible latest tasks shown'));
    expect(cards(panel)).toHaveLength(3);
    expect(panel.textContent).not.toContain(failure);
    expect(document.activeElement).toBe(more);
    tasks = [identityTask({ id: 'now-ordered' })];
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(panel).toHaveTextContent('now-ordered'));
    expect(cards(panel)).toHaveLength(0);
    expect(more).toHaveTextContent('All 0 possible latest tasks shown');
    expect(panel).toHaveTextContent('No uncertain tasks remain in this read.');
    expect(panel).not.toHaveTextContent('Task order');
    expect(document.activeElement).toBe(more);
    tasks = [];
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(panel).not.toHaveTextContent('now-ordered'));
    expect(document.activeElement).toBe(more);
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(api.fetch).toHaveBeenCalledTimes(10);
  });

  it('continues to find an undisplayed candidate through saved-view workload search', async () => {
    vi.useRealTimers();
    vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date'] });
    tasks = history();
    mount();
    const panel = await open();
    const search = screen.getByPlaceholderText(
      'Search backups by workload, node, source, or status',
    );
    fireEvent.input(search, { target: { value: 'history-150' } });
    await waitFor(() => expect(window.location.search).toContain('q=history-150'));
    expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(1);
    expect(cards(panel)).toHaveLength(8);
    expect(panel).toHaveTextContent('Showing 8 of 200 possible latest tasks.');
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });
});
