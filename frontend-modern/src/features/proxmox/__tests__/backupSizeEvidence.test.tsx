import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';
import { parsePBSBackupInventory, parsePVEBackupInventory } from '../proxmoxBackupInventory';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 1200 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));

const guest = identityGuest('backup-size-guest', 'east', 'pve-1');
const payloads = (size: number | undefined) => ({
  pbs: { backups: [identityPBS({ size })] },
  pve: {
    backupTasks: [],
    storageBackups: [identityArchive({ size })],
    guestSnapshots: [identitySnapshot({ sizeBytes: size })],
  },
});
const response = (data: unknown) =>
  new Response(JSON.stringify({ data }), { headers: { 'Content-Type': 'application/json' } });

function mount(getPayloads = () => payloads(0)) {
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(response(url === '/api/backups/pbs' ? getPayloads().pbs : getPayloads().pve)),
  );
  // A size, verification result or completed point never invents a posture.
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
const rows = () =>
  Array.from(document.querySelectorAll<HTMLElement>('[data-proxmox-backup-row="recoverable"]'));
const pointRow = (id: string) =>
  screen
    .getByRole('button', { name: new RegExp(`Expand details.*Identifier: ${id}$`) })
    .closest('tr')!;
const sizeValue = (panel: HTMLElement) =>
  within(panel).getByText('Size', { exact: true, selector: 'td' }).closest('tr')!.lastElementChild!;
const sizeNotice =
  'No usable size was reported. Unknown does not mean this restore point is empty.';

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date', 'setInterval', 'clearInterval'] });
  resetCreateNonSuspendingQueryCacheForTest();
  api.width = 1200;
  window.history.replaceState({}, '', '/proxmox/backups/date');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('restore-point size evidence through parsed inventory', () => {
  it.each([0, undefined])('does not turn %s into measured zero bytes for any source', (size) => {
    const input = payloads(size);
    const pbs = parsePBSBackupInventory({ data: input.pbs });
    const pve = parsePVEBackupInventory({ data: input.pve });
    expect(pbs.backups[0].size).toBe(size);
    expect(pve.storageBackups[0].size).toBe(size);
    expect(pve.guestSnapshots[0].sizeBytes).toBe(size);
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [guest],
      pbsBackups: pbs.backups,
      archives: pve.storageBackups,
      snapshots: pve.guestSnapshots,
      tasks: [],
      nowMs: BACKUP_IDENTITY_NOW,
    });
    expect(model.recoverableArtifacts.map((artifact) => artifact.size)).toEqual([
      undefined,
      undefined,
      undefined,
    ]);
    expect(model.coverageSummary.recoverableArtifacts).toBe(3);
    expect(model.coverageSummary.totalBytes).toBe(0); // Known-byte sum, not a measured fleet total.
    expect(model.coverageRows[0].posture).toBe('unknown');
  });

  it('preserves even a one-byte report for all three sources', () => {
    const input = payloads(1);
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [guest],
      pbsBackups: parsePBSBackupInventory({ data: input.pbs }).backups,
      archives: parsePVEBackupInventory({ data: input.pve }).storageBackups,
      snapshots: parsePVEBackupInventory({ data: input.pve }).guestSnapshots,
      tasks: [],
      nowMs: BACKUP_IDENTITY_NOW,
    });
    expect(model.recoverableArtifacts.map((artifact) => artifact.size)).toEqual([1, 1, 1]);
    expect(model.coverageSummary.totalBytes).toBe(3);
  });

  it.each([1200, 320])(
    '[baseline-sensitive] keeps unknown size explicit in every open restore point at %ipx',
    async (width) => {
      api.width = width;
      mount();
      await waitFor(() => expect(rows()).toHaveLength(3));
      for (const row of rows()) {
        if (width === 1200) expect(within(row).getByText('Unknown size')).toBeInTheDocument();
        fireEvent.click(within(row).getByRole('button', { name: /Expand details/ }));
        const panel = screen.getByTestId('proxmox-recoverable-detail');
        expect(sizeValue(panel)).toHaveTextContent('Unknown size');
        expect(panel).toHaveTextContent(sizeNotice);
        expect(panel).not.toHaveTextContent('0 B');
      }
      expect(document.body).toHaveTextContent('3 completed restore points');
      expect(screen.getByRole('button', { name: '1 unknown' })).toBeInTheDocument();
      expect(api.fetch).toHaveBeenCalledTimes(2);
    },
  );

  it('[baseline-sensitive] sorts only measured sizes, keeping zero and absent reports last in both directions', async () => {
    mount(() => ({
      pbs: {
        backups: [
          identityPBS({ id: 'unknown-zero', size: 0 }),
          identityPBS({ id: 'unknown-missing', size: undefined }),
          identityPBS({ id: 'known-small', size: 1 }),
          identityPBS({ id: 'known-large', size: 4096 }),
        ],
      },
      pve: { backupTasks: [], storageBackups: [], guestSnapshots: [] },
    }));
    await waitFor(() => expect(rows()).toHaveLength(4));
    const order = () =>
      rows().map(
        (row) =>
          within(row)
            .getByRole('button', { name: /Expand details/ })
            .getAttribute('aria-label')!
            .split('Identifier: ')[1],
      );
    fireEvent.click(screen.getByRole('button', { name: 'Sort by Size' }));
    expect(order()).toEqual(['known-large', 'known-small', 'unknown-zero', 'unknown-missing']);
    fireEvent.click(screen.getByRole('button', { name: 'Sort by Size ascending' }));
    expect(order()).toEqual(['known-small', 'known-large', 'unknown-zero', 'unknown-missing']);
    expect(within(pointRow('known-small')).getByText(/^1(?:\.00)? B$/)).toBeInTheDocument();
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });

  it.each([1200, 320])(
    'keeps Coverage size evidence readable at %ipx without implying empty points',
    async (width) => {
      api.width = width;
      window.history.replaceState({}, '', '/proxmox/backups/coverage');
      mount();
      const toggle = await screen.findByRole('button', {
        name: /Expand details for backup-size-guest/,
      });
      fireEvent.click(toggle);
      const panel = screen.getByTestId('proxmox-coverage-detail');
      expect(panel).toHaveTextContent(
        'Some restore-point sizes are unknown. Unknown does not mean those points are empty.',
      );
      expect(
        within(panel).getAllByText(width === 1200 ? 'Unknown size' : 'Size: Unknown size'),
      ).toHaveLength(3);
      expect(panel).not.toHaveTextContent('0 B');
      expect(api.fetch).toHaveBeenCalledTimes(2);
    },
  );

  it('updates the same open point from known to unknown and back on ordinary polls without losing focus', async () => {
    api.width = 320;
    let size = 2048;
    mount(() => payloads(size));
    await waitFor(() => expect(rows()).toHaveLength(3));
    const toggle = within(pointRow('pbs-100')).getByRole('button', { name: /Expand details/ });
    fireEvent.click(toggle);
    toggle.focus();
    const panel = screen.getByTestId('proxmox-recoverable-detail');
    expect(sizeValue(panel)).toHaveTextContent('2.00 KB');
    expect(panel).not.toHaveTextContent(sizeNotice);
    size = 0;
    await vi.advanceTimersByTimeAsync(30_000);
    expect(screen.getByTestId('proxmox-recoverable-detail')).toBe(panel);
    expect(sizeValue(panel)).toHaveTextContent('Unknown size');
    expect(panel).toHaveTextContent(sizeNotice);
    expect(document.activeElement).toBe(toggle);
    size = 4096;
    await vi.advanceTimersByTimeAsync(30_000);
    expect(screen.getByTestId('proxmox-recoverable-detail')).toBe(panel);
    expect(sizeValue(panel)).toHaveTextContent('4.00 KB');
    expect(panel).not.toHaveTextContent(sizeNotice);
    expect(document.activeElement).toBe(toggle);
    expect(api.fetch).toHaveBeenCalledTimes(6);
  });
});
