import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { PBSBackup, PVEBackupsPayload } from '@/types/api';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 390 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));

const guests = () =>
  ['east', 'west'].map((instance) => ({
    ...identityGuest(`guest-${instance}`, instance, 'pve-1'),
    name: 'same-name',
    displayName: 'same-name',
  }));
const emptyPVE: PVEBackupsPayload = { backupTasks: [], storageBackups: [], guestSnapshots: [] };
const archives = (): PVEBackupsPayload => ({
  ...emptyPVE,
  storageBackups: ['east', 'west'].map((instance) =>
    identityArchive({ id: `file-${instance}`, instance }),
  ),
});
const response = (data: unknown) =>
  new Response(JSON.stringify({ data }), { headers: { 'Content-Type': 'application/json' } });
function mount({ workloads = guests(), pve = archives(), pbs = [] as PBSBackup[] } = {}) {
  const [current, setWorkloads] = createSignal(workloads);
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(response(url === '/api/backups/pbs' ? { backups: pbs } : pve)),
  );
  // Visible inventory never establishes canonical protection.
  api.json.mockResolvedValue({ data: [], policy: protectionPolicy });
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={current()} />}
      />
    </Router>
  ));
  return setWorkloads;
}
async function coverageRows() {
  await screen.findAllByText('same-name');
  return Array.from(document.querySelectorAll<HTMLElement>('[data-proxmox-backup-row="coverage"]'));
}
const connectionOf = (row: Element) =>
  row.querySelector('[data-backup-workload-connection]')?.textContent;
const connectionRow = (detail: HTMLElement) =>
  within(detail).getByText('PVE connection', { exact: true }).closest('tr')!;

beforeEach(() => {
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date'] });
  vi.resetAllMocks();
  api.width = 390;
  resetCreateNonSuspendingQueryCacheForTest();
  window.history.replaceState({}, '', '/proxmox/backups/coverage');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('backup workload identity in real inventory consumers', () => {
  it.each([330, 600, 800, 1000, 1200])(
    'distinguishes identical guest names/VMIDs/nodes at %ipx without changing their evidence or owner',
    async (width) => {
      api.width = width;
      mount();
      const rows = await coverageRows();
      expect(rows).toHaveLength(2);
      for (const scope of ['east', 'west']) {
        const row = rows.find((item) => connectionOf(item) === `PVE: ${scope}`)!;
        expect(row).toBeDefined();
        expect(row).toHaveTextContent('same-name');
        expect(row).toHaveTextContent('100');
        const toggle = within(row).getByRole('button', {
          name: `Expand details for same-name (VM 100) · PVE connection: ${scope}`,
        });
        fireEvent.click(toggle);
        const detail = screen.getByTestId('proxmox-coverage-detail');
        expect(connectionRow(detail)).toHaveTextContent(scope);
        expect(within(detail).getByRole('link', { name: 'Open same-name' })).toHaveAttribute(
          'href',
          `/proxmox/overview?resource=guest-${scope}`,
        );
        const sources = detail.querySelectorAll('[data-backup-source-connection]');
        expect(sources).toHaveLength(1);
        expect(sources[0]).toHaveTextContent(`Connection: ${scope}`);
        expect(detail).toHaveTextContent('Backup file');
        expect(detail).not.toHaveTextContent('Unverified');
        fireEvent.click(toggle);
      }
      expect(screen.getByRole('button', { name: '2 unknown' })).toBeInTheDocument();
      expect(api.fetch).toHaveBeenCalledTimes(2);
    },
  );

  it('keeps the PVE workload scope separate from its PBS reader and namespace in both details', async () => {
    mount({
      workloads: guests().slice(0, 1),
      pve: emptyPVE,
      pbs: [identityPBS({ instance: 'pbs-reader', namespace: 'unrelated-namespace' })],
    });
    const [row] = await coverageRows();
    expect(connectionOf(row)).toBe('PVE: east');
    fireEvent.click(within(row).getByRole('button', { name: /expand details/i }));
    let detail = screen.getByTestId('proxmox-coverage-detail');
    expect(connectionRow(detail)).toHaveTextContent('east');
    expect(connectionRow(detail)).not.toHaveTextContent('pbs-reader');
    expect(detail.querySelector('[data-backup-source-connection]')).toHaveTextContent(
      'Connection: pbs-reader',
    );
    await fireEvent.click(screen.getByRole('link', { name: /by date/i }));
    const point = await screen.findByRole('button', {
      name: 'Expand details for same-name (VM 100) · PVE connection: east',
    });
    fireEvent.click(point);
    detail = screen.getByTestId('proxmox-recoverable-detail');
    expect(connectionRow(detail)).toHaveTextContent('east');
    expect(within(detail).getByText('Connection', { exact: true }).closest('tr')).toHaveTextContent(
      'pbs-reader',
    );
    expect(detail).toHaveTextContent('main / unrelated-namespace');
    expect(within(detail).getByRole('link')).toHaveAttribute(
      'href',
      '/proxmox/overview?resource=guest-east',
    );
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });

  it('never borrows a PBS reader or namespace for unmatched guest scope, or invents a host PVE connection', async () => {
    mount({
      workloads: [],
      pve: emptyPVE,
      pbs: [
        identityPBS({ instance: 'pbs-reader', namespace: 'east' }),
        identityPBS({ id: 'host-point', backupType: 'host', vmid: 'nas-box' }),
      ],
    });
    fireEvent.click(await screen.findByRole('button', { name: /1 unmatched backup/i }));
    const vm = await screen.findByRole('button', {
      name: 'Expand details for VM 100 · PVE connection: Not reported',
    });
    expect(vm.closest('tr')!.textContent?.match(/VM 100/g)).toHaveLength(1);
    fireEvent.click(vm);
    let detail = screen.getByTestId('proxmox-coverage-detail');
    expect(connectionRow(detail)).toHaveTextContent('Not reported');
    expect(connectionRow(detail)).not.toHaveTextContent('east');
    expect(detail.querySelector('[data-backup-source-connection]')).toHaveTextContent('pbs-reader');
    expect(within(detail).queryByRole('link')).not.toBeInTheDocument();
    const host = screen.getByRole('button', { name: /expand details for Host nas-box/i });
    expect(host).not.toHaveAccessibleName(/PVE connection/i);
    fireEvent.click(host);
    detail = screen.getByTestId('proxmox-coverage-detail');
    expect(within(detail).queryByText('PVE connection')).not.toBeInTheDocument();
    expect(within(detail).queryByRole('link')).not.toBeInTheDocument();
    await fireEvent.click(screen.getByRole('link', { name: /by date/i }));
    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Expand details for VM 100 · PVE connection: Not reported',
      }),
    );
    expect(connectionRow(screen.getByTestId('proxmox-recoverable-detail'))).toHaveTextContent(
      'Not reported',
    );
  });

  it('keeps a same-numbered LXC distinguishable from a VM in the same PVE connection', async () => {
    const guest = guests()[0];
    mount({
      workloads: [guest, { ...guest, id: 'guest-lxc', type: 'system-container' }],
      pve: {
        ...emptyPVE,
        storageBackups: [identityArchive(), identityArchive({ id: 'lxc-file', type: 'lxc' })],
      },
    });
    const rows = await coverageRows();
    expect(rows).toHaveLength(2);
    const vm = screen.getByRole('button', {
      name: 'Expand details for same-name (VM 100) · PVE connection: east',
    });
    const ct = screen.getByRole('button', {
      name: 'Expand details for same-name (LXC 100) · PVE connection: east',
    });
    expect(vm.closest('tr')).toHaveTextContent('VM 100');
    expect(ct.closest('tr')).toHaveTextContent('LXC 100');
    fireEvent.click(ct);
    const detail = screen.getByTestId('proxmox-coverage-detail');
    expect(connectionRow(detail)).toHaveTextContent('east');
    expect(detail.querySelectorAll('[data-backup-source-connection]')).toHaveLength(1);
    expect(within(detail).getByRole('link')).toHaveAttribute(
      'href',
      '/proxmox/overview?resource=guest-lxc',
    );
  });

  it('renders a missing live PVE connection explicitly rather than borrowing its node', async () => {
    const guest = guests()[0];
    mount({
      workloads: [{ ...guest, proxmox: { ...guest.proxmox!, instance: '   ' } }],
      pve: emptyPVE,
    });
    const [row] = await coverageRows();
    expect(connectionOf(row)).toBe('PVE: Not reported');
    fireEvent.click(within(row).getByRole('button', { name: /expand details/i }));
    expect(connectionRow(screen.getByTestId('proxmox-coverage-detail'))).toHaveTextContent(
      'Not reported',
    );
  });

  it('uses literal connection text and keeps same-key live updates in the open panel without losing disclosure focus', async () => {
    const guest = guests()[0];
    const setWorkloads = mount({ workloads: [guest], pve: emptyPVE });
    const [row] = await coverageRows();
    const toggle = within(row).getByRole('button', { name: /expand details/i });
    fireEvent.click(toggle);
    toggle.focus();
    const detail = screen.getByTestId('proxmox-coverage-detail');
    const origin = row.querySelector('[data-backup-workload-connection]');
    const literal = '<img data-workload-injected src=x>';
    setWorkloads([{ ...guest, proxmox: { ...guest.proxmox!, instance: literal } }]);
    await waitFor(() => expect(connectionOf(row)).toBe(`PVE: ${literal}`));
    expect(row.querySelector('[data-backup-workload-connection]')).toBe(origin);
    expect(screen.getByTestId('proxmox-coverage-detail')).toBe(detail);
    expect(connectionRow(detail)).toHaveTextContent(literal);
    expect(document.querySelector('[data-workload-injected]')).toBeNull();
    expect(toggle).toHaveAccessibleName(
      `Collapse details for same-name (VM 100) · PVE connection: ${literal}`,
    );
    expect(document.activeElement).toBe(toggle);
    expect(within(detail).getByRole('link')).toHaveAttribute(
      'href',
      '/proxmox/overview?resource=guest-east',
    );
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });

  it('finds the displayed workload connection through the existing search without another read', async () => {
    mount();
    await coverageRows();
    fireEvent.input(screen.getByPlaceholderText(/Search backups by workload/), {
      target: { value: 'WEST' },
    });
    await waitFor(() =>
      expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(1),
    );
    expect(connectionOf(document.querySelector('[data-proxmox-backup-row="coverage"]')!)).toBe(
      'PVE: west',
    );
    expect(api.fetch).toHaveBeenCalledTimes(2);
  });
});
