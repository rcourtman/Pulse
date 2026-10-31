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

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 320 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));

const guests = [
  identityGuest('east-100', 'east', 'pve-1'),
  identityGuest('west-100', 'west', 'pve-1'),
];
const literal = '<img src=x onerror=alert(1)>';
const volume = `local:backup/very-long-production-archive-${'preserved-'.repeat(8)}.vma.zst`;
let updated: boolean;
function inventories() {
  return {
    backups: [
      identityPBS({ id: 'same-time-pbs-a', namespace: 'east' }),
      identityPBS({
        id: 'same-time-pbs-b',
        namespace: 'east',
        files: updated ? ['index.json.blob', 'disk.fidx'] : ['index.json.blob'],
        inProgress: true,
        writeActivityObserved: true,
        writeActive: false,
      }),
    ],
    storageBackups: [
      identityArchive({ id: 'same-time-archive-a', volid: updated ? `${volume}-updated` : volume }),
      identityArchive({ id: `same-time-archive-b ${literal}`, volid: `local:backup/${literal}` }),
    ],
    guestSnapshots: [
      identitySnapshot({ id: 'same-time-snapshot-a', name: 'before-update' }),
      identitySnapshot({
        id: 'same-time-snapshot-b',
        name: 'after-update',
        description: updated ? `updated ${literal}` : 'second snapshot',
      }),
    ],
    backupTasks: [],
  };
}
function mount() {
  api.fetch.mockImplementation((url: string) => {
    const inventory = inventories();
    return Promise.resolve(
      new Response(
        JSON.stringify({
          data: url === '/api/backups/pbs' ? { backups: inventory.backups } : inventory,
        }),
        { headers: { 'Content-Type': 'application/json' } },
      ),
    );
  });
  // Point identities must not invent an assessment or change workload protection.
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
async function open() {
  const toggle = await screen.findByRole('button', { name: /Expand details for east-100/ });
  await fireEvent.click(toggle);
  return screen.findByTestId('proxmox-coverage-detail');
}
function evidenceRows(panel: HTMLElement) {
  return Array.from(panel.querySelectorAll<HTMLElement>('[data-coverage-evidence-detail]')).map(
    (details) => details.closest('tr')!,
  );
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date', 'setInterval', 'clearInterval'] });
  resetCreateNonSuspendingQueryCacheForTest();
  api.width = 320;
  updated = false;
  window.history.replaceState({}, '', '/proxmox/backups/coverage');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('Coverage restore-point identity through parsed inventory', () => {
  it.each([320, 479, 480, 719, 720, 960, 1200])(
    'distinguishes same-time points with full identifiers and metadata at %ipx',
    async (width) => {
      api.width = width;
      mount();
      const panel = await open();
      const rows = evidenceRows(panel);
      expect(rows).toHaveLength(6);
      const expected = [
        ['same-time-pbs-a', '1 PBS file'],
        ['same-time-pbs-b', '1 PBS file'],
        ['same-time-archive-a', volume],
        [`same-time-archive-b ${literal}`, `local:backup/${literal}`],
        ['same-time-snapshot-a', 'before-update'],
        ['same-time-snapshot-b', 'second snapshot'],
      ];
      for (const [id, metadata] of expected) {
        const row = rows.find((candidate) =>
          candidate.querySelector('[data-coverage-evidence-detail]')?.textContent?.includes(id),
        )!;
        expect(row).toBeDefined();
        expect(row.querySelector('[data-coverage-evidence-detail]')).toHaveTextContent(
          `Identifier: ${id}`,
        );
        expect(row.querySelector('[data-coverage-evidence-detail]')).toHaveTextContent(
          `Metadata: ${metadata}`,
        );
        expect(row.querySelector('[data-coverage-evidence-detail]')).toHaveClass(
          'whitespace-pre-line',
          'wrap-anywhere',
        );
        expect(row.querySelector('[data-coverage-evidence-detail]')).not.toHaveClass('truncate');
      }
      expect(panel.querySelector('img')).toBeNull();
      expect(within(panel).getByText('Failed', { exact: true })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: '2 unknown' })).toBeInTheDocument();
      expect(screen.getByText(/5 completed restore points/)).toBeInTheDocument();
      expect(api.fetch).toHaveBeenCalledTimes(2);
      await fireEvent.click(screen.getByRole('button', { name: /Expand details for west-100/ }));
      expect(screen.getByTestId('proxmox-coverage-detail')).not.toHaveTextContent(volume);
      expect(screen.getByTestId('proxmox-coverage-detail')).not.toHaveTextContent(
        'same-time-pbs-a',
      );
    },
  );

  it.each([320, 1200])('refreshes the same open evidence and focus at %ipx', async (width) => {
    api.width = width;
    mount();
    const panel = await open();
    const toggle = screen.getByRole('button', { name: /Collapse details for east-100/ });
    toggle.focus();
    const before = evidenceRows(panel);
    expect(before).toHaveLength(6);
    updated = true;
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(panel).toHaveTextContent(`${volume}-updated`));
    expect(panel).not.toHaveTextContent('second snapshot');
    expect(panel).toHaveTextContent(`updated ${literal}`);
    expect(panel).toHaveTextContent('2 PBS files');
    expect(panel.querySelector('img')).toBeNull();
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(panel);
    expect(evidenceRows(panel)).toEqual(before);
    expect(document.activeElement).toBe(toggle);
    expect(api.fetch).toHaveBeenCalledTimes(4);
    expect(screen.getByText(/5 completed restore points/)).toBeInTheDocument();
  });
});
