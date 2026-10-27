import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { BackupTask } from '@/types/api';
import type { Resource } from '@/types/resource';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { BACKUP_IDENTITY_NOW, identityGuest, identityTask } from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 390 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));
let tasks: BackupTask[];
let guests: Resource[];
const response = (data: unknown) =>
  new Response(JSON.stringify({ data }), { headers: { 'Content-Type': 'application/json' } });
function mount() {
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(
      response(
        url === '/api/backups/pbs'
          ? { backups: [] }
          : { backupTasks: tasks, storageBackups: [], guestSnapshots: [] },
      ),
    ),
  );
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
  fireEvent.click(
    await screen.findByRole('button', {
      name: `Expand details for same-name (VM 100) · PVE connection: ${scope}`,
    }),
  );
  return screen.findByTestId('proxmox-coverage-detail');
}
const section = (panel: HTMLElement, label = 'Latest task') =>
  within(panel).getByText(label, { exact: true, selector: 'th' }).closest('tbody')!;
const value = (parent: HTMLElement, label: string) =>
  within(parent).getByText(label, { exact: true, selector: 'td' }).closest('tr')!.lastElementChild!;

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date', 'setInterval', 'clearInterval'] });
  api.width = 390;
  guests = ['east', 'west'].map((instance) => ({
    ...identityGuest(`guest-${instance}`, instance, 'current-node'),
    name: 'same-name',
    displayName: 'same-name',
  }));
  tasks = [identityTask({ node: 'historical-node', status: 'failed', error: 'retained failure' })];
  resetCreateNonSuspendingQueryCacheForTest();
  window.history.replaceState({}, '', '/proxmox/backups/coverage');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('task-reported origin through the inventory consumer', () => {
  it.each([1200, 390, 320])(
    'keeps historical task origin separate from current placement at %ipx',
    async (width) => {
      api.width = width;
      mount();
      const panel = await open();
      expect(value(section(panel, 'Workload'), 'Node')).toHaveTextContent('current-node');
      expect(value(section(panel), 'Task node')).toHaveTextContent('historical-node');
      expect(value(section(panel), 'Task connection')).toHaveTextContent('east');
      expect(value(section(panel), 'Status')).toHaveTextContent('Failed');
      expect(section(panel)).toHaveTextContent('not the workload’s current placement');
      expect(section(panel)).toHaveTextContent(
        'Task status does not verify a restore or guest responsiveness.',
      );
      expect(api.fetch).toHaveBeenCalledTimes(2);
      fireEvent.click(within(panel).getByRole('button', { name: 'Collapse same-name details' }));
      const west = await open('west');
      expect(west).not.toHaveTextContent('historical-node');
      expect(within(west).queryByText('Latest task', { exact: true, selector: 'th' })).toBeNull();
    },
  );

  it.each([
    { instance: 'east', node: '  ' },
    { instance: '', node: 'current-node' },
    { instance: undefined, node: undefined },
  ])('does not borrow a missing origin from the workload: $instance / $node', async (origin) => {
    guests = [guests[0]];
    tasks = [identityTask({ ...origin, type: 'vm' })];
    mount();
    const panel = await open();
    expect(value(section(panel), 'Task connection').textContent).toBe(
      origin.instance?.trim() || 'Not reported',
    );
    expect(value(section(panel), 'Task node').textContent).toBe(
      origin.node?.trim() || 'Not reported',
    );
    expect(value(section(panel, 'Workload'), 'Node')).toHaveTextContent('current-node');
    expect(value(section(panel, 'Workload'), 'PVE connection')).toHaveTextContent('east');
  });

  it('renders long provider node text literally without interpreting markup', async () => {
    const node = '<img src=x onerror="throw 1">' + 'case-Sensitive-long-node-'.repeat(8);
    tasks = [identityTask({ node })];
    mount();
    const cell = value(section(await open()), 'Task node');
    expect(cell.textContent).toBe(node);
    expect(cell.querySelector('img')).toBeNull();
    expect(cell.firstElementChild).toHaveClass('whitespace-normal', 'wrap-break-word');
  });

  it('keeps the independent origins of every possible latest task', async () => {
    tasks = [
      tasks[0],
      identityTask({ id: 'other-task', node: 'other-historical-node', startTime: 'invalid' }),
    ];
    mount();
    const panel = await open();
    expect(value(section(panel, 'Task order'), 'Latest task')).toHaveTextContent('Unknown');
    const candidates = [
      section(panel, 'Possible latest task 1'),
      section(panel, 'Possible latest task 2'),
    ];
    expect(candidates.map((candidate) => value(candidate, 'Task node').textContent)).toEqual([
      'other-historical-node',
      'historical-node',
    ]);
    expect(
      candidates.every((candidate) => value(candidate, 'Task connection').textContent === 'east'),
    ).toBe(true);
  });

  it.each(['historical-node', 'task-100'])(
    'searches exposed task evidence without leaking into a namesake: %s',
    async (term) => {
      // Match the existing saved-view consumer tests: real timers for input
      // and URL navigation, with only the observation date fixed.
      vi.useRealTimers();
      vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date'] });
      mount();
      await screen.findByRole('button', { name: /Expand details for same-name.*east/ });
      fireEvent.input(
        screen.getByPlaceholderText('Search backups by workload, node, source, or status'),
        { target: { value: term } },
      );
      await waitFor(() => expect(window.location.search).toContain(term));
      expect(
        screen.getByRole('button', { name: /Expand details for same-name.*east/ }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: /Expand details for same-name.*west/ }),
      ).toBeNull();
    },
  );

  it('updates and clears reported origins on ordinary polling with panel and focus intact', async () => {
    mount();
    const panel = await open();
    const toggle = screen.getByRole('button', { name: /Collapse details for same-name.*east/ });
    toggle.focus();
    tasks = [identityTask({ node: 'new-task-node' })];
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(value(section(panel), 'Task node')).toHaveTextContent('new-task-node'),
    );
    expect(panel).not.toHaveTextContent('historical-node');
    expect(value(section(panel), 'Status')).toHaveTextContent('OK');
    tasks = [identityTask({ node: '', instance: '', type: 'vm' })];
    // A single live candidate keeps the same workload key; its known current
    // placement must not fill either missing task field.
    guests.splice(1);
    // Workloads are not reactive here, but the fresh model reads this array on
    // the inventory poll, just as the production resources query replaces it.
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(value(section(panel), 'Task node')).toHaveTextContent('Not reported'),
    );
    expect(value(section(panel), 'Task connection')).toHaveTextContent('Not reported');
    expect(value(section(panel, 'Workload'), 'Node')).toHaveTextContent('current-node');
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(document.activeElement).toBe(toggle);
    expect(api.fetch).toHaveBeenCalledTimes(6);
  });
});
