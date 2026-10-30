import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
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
  identitySnapshot,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 320 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));

const name = 'accounting-database-production-primary';
const scopes = [
  'pve.cluster.production.london.example.internal',
  'pve.cluster.production.bristol.example.internal',
];
const workloads = () =>
  scopes.map((scope, index) => ({
    ...identityGuest(`guest-${index}`, scope, 'pve-1'),
    name,
    displayName: name,
  }));
const emptyPVE: PVEBackupsPayload = { storageBackups: [], guestSnapshots: [], backupTasks: [] };
const response = (data: unknown) =>
  new Response(JSON.stringify({ data }), { headers: { 'Content-Type': 'application/json' } });

function mount({
  guests = workloads(),
  pve = () => ({
    ...emptyPVE,
    storageBackups: scopes.map((scope, index) =>
      identityArchive({ id: `file-${index}`, instance: scope, storage: `repository-${index}` }),
    ),
  }),
  pbs = () => [] as PBSBackup[],
} = {}) {
  api.fetch.mockImplementation((url: string) =>
    Promise.resolve(response(url === '/api/backups/pbs' ? { backups: pbs() } : pve())),
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
async function rows() {
  await waitFor(() =>
    expect(document.querySelector('[data-proxmox-backup-row="recoverable"]')).not.toBeNull(),
  );
  return Array.from(
    document.querySelectorAll<HTMLElement>('[data-proxmox-backup-row="recoverable"]'),
  );
}

beforeEach(() => {
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date'] });
  vi.resetAllMocks();
  api.width = 320;
  resetCreateNonSuspendingQueryCacheForTest();
  window.history.replaceState({}, '', '/proxmox/backups/date');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('restore-point identity without hover', () => {
  it.each([320, 479, 480, 719, 720, 1200])(
    'keeps namesake workload and reader scope distinct at %ipx',
    async (width) => {
      api.width = width;
      mount();
      const points = await rows();
      expect(points).toHaveLength(2);
      for (const [index, scope] of scopes.entries()) {
        const row = points.find((point) =>
          point.querySelector('[data-backup-workload-connection]')?.textContent?.includes(scope),
        )!;
        expect(row).toBeDefined();
        expect(row).toHaveTextContent(name);
        expect(row).toHaveTextContent('100');
        expect(row.querySelector('[data-backup-source-connection]')).toHaveTextContent(scope);
        expect(row).toHaveTextContent(`repository-${index}`);
        if (width < 720) {
          for (const element of [
            within(row).getByText(name, { exact: true }),
            row.querySelector('[data-backup-workload-connection]')!,
            row.querySelector('[data-backup-source-connection]')!,
            row.querySelector('[data-backup-location]')!,
          ]) {
            expect(element).toHaveClass('whitespace-normal', 'wrap-anywhere');
            expect(element).not.toHaveClass('truncate');
          }
          expect(row.children).toHaveLength(3);
          expect(row.closest('table')?.querySelector('thead th')).toHaveTextContent(
            'Restore point',
          );
        }
        fireEvent.click(within(row).getByRole('button', { name: /Expand details/ }));
        const panel = screen.getByTestId('proxmox-recoverable-detail');
        expect(panel).toHaveTextContent(scope);
        expect(panel).toHaveTextContent(`repository-${index}`);
        expect(panel).not.toHaveTextContent(`repository-${1 - index}`);
        expect(within(panel).getByRole('link', { name: `Open ${name}` })).toHaveAttribute(
          'href',
          `/proxmox/overview?resource=guest-${index}`,
        );
      }
      expect(api.fetch).toHaveBeenCalledTimes(2);
      expect(screen.getByRole('button', { name: '2 unknown' })).toBeInTheDocument();
    },
  );

  it.each(['pbs', 'archive', 'snapshot'] as const)(
    'exposes the complete %s reader/location without changing its source kind',
    async (kind) => {
      const reader = 'backup.connection.long-name.example.internal';
      const location = 'repository-with-a-long-preserved-name';
      mount({
        guests: workloads().slice(0, 1),
        pve: () => ({
          ...emptyPVE,
          storageBackups:
            kind === 'archive' ? [identityArchive({ instance: scopes[0], storage: location })] : [],
          guestSnapshots: kind === 'snapshot' ? [identitySnapshot({ instance: scopes[0] })] : [],
        }),
        pbs: () =>
          kind === 'pbs'
            ? [identityPBS({ instance: reader, namespace: 'pve-1', datastore: location })]
            : [],
      });
      const [point] = await rows();
      expect(point.querySelector('[data-backup-workload-connection]')).toHaveTextContent(scopes[0]);
      expect(point.querySelector('[data-backup-source-connection]')).toHaveTextContent(
        kind === 'pbs' ? reader : scopes[0],
      );
      expect(point.querySelector('[data-backup-location]')).toHaveTextContent(
        kind === 'pbs' ? `${location} / pve-1` : kind === 'archive' ? location : 'pve-1',
      );
      expect(
        within(point.children[0] as HTMLElement).getByText(
          kind === 'pbs' ? 'PBS' : kind === 'archive' ? 'PVE file' : 'Snapshot',
        ),
      ).toBeInTheDocument();
      expect(point).toHaveTextContent(
        kind === 'pbs' ? 'Verified' : kind === 'snapshot' ? 'Snapshot' : 'Backup file',
      );
    },
  );

  it('does not borrow a reader connection for an ambiguous guest or host backup', async () => {
    mount({
      pve: () => emptyPVE,
      pbs: () => [
        identityPBS({ instance: 'shared-pbs', namespace: 'pve-1' }),
        identityPBS({ id: 'host', instance: 'shared-pbs', backupType: 'host', vmid: '0' }),
      ],
    });
    const points = await rows();
    expect(points).toHaveLength(2);
    const guest = points.find((point) => point.textContent?.includes('VM 100'))!;
    expect(guest.querySelector('[data-backup-workload-connection]')).toHaveTextContent(
      'PVE: Not reported',
    );
    expect(guest.querySelector('[data-backup-source-connection]')).toHaveTextContent('shared-pbs');
    const host = points.find((point) => point !== guest)!;
    expect(host.querySelector('[data-backup-workload-connection]')).toBeNull();
    expect(host.querySelector('[data-backup-source-connection]')).toHaveTextContent('shared-pbs');
    fireEvent.click(within(guest).getByRole('button', { name: /Expand details/ }));
    expect(within(screen.getByTestId('proxmox-recoverable-detail')).queryByRole('link')).toBeNull();
  });

  it('updates an open point in place without dropping focus or borrowing its new reader', async () => {
    vi.useFakeTimers({
      now: BACKUP_IDENTITY_NOW,
      toFake: ['Date', 'setInterval', 'clearInterval'],
    });
    let reader = 'pbs-before';
    let datastore = 'before';
    mount({
      guests: workloads().slice(0, 1),
      pve: () => emptyPVE,
      pbs: () => [identityPBS({ instance: reader, datastore })],
    });
    const [point] = await rows();
    const toggle = within(point).getByRole('button', { name: /Expand details/ });
    fireEvent.click(toggle);
    toggle.focus();
    const panel = screen.getByTestId('proxmox-recoverable-detail');
    reader = 'pbs-after';
    datastore = 'after';
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(point.querySelector('[data-backup-source-connection]')).toHaveTextContent(reader),
    );
    expect(point.querySelector('[data-backup-workload-connection]')).toHaveTextContent(scopes[0]);
    expect(point.querySelector('[data-backup-location]')).toHaveTextContent('after / pve-1');
    expect(screen.getByTestId('proxmox-recoverable-detail')).toBe(panel);
    expect(panel).toHaveTextContent(reader);
    expect(panel).not.toHaveTextContent('pbs-before');
    expect(toggle).toHaveAccessibleName(/Backup connection: pbs-after/);
    expect(toggle).toHaveAccessibleName(/Location: after \/ pve-1/);
    expect(toggle).not.toHaveAccessibleName(/pbs-before/);
    expect(document.activeElement).toBe(toggle);
    expect(api.fetch).toHaveBeenCalledTimes(4);
  });
});

it('names multiple dates, readers and same-time snapshots independently in the raw inventory consumer', async () => {
  const dated = [
    identityPBS({ id: 'newer', backupTime: '2026-10-04T10:00:01Z' }),
    identityPBS({ id: 'older', backupTime: '2026-10-03T10:00:01Z' }),
    identityPBS({ id: 'offsite', instance: 'offsite-reader', backupTime: '2026-10-04T10:00:01Z' }),
  ];
  mount({
    guests: workloads().slice(0, 1),
    pve: () => ({
      ...emptyPVE,
      guestSnapshots: [
        identitySnapshot({ id: 'before-upgrade', instance: scopes[0] }),
        identitySnapshot({ id: 'before-packages', instance: scopes[0], name: 'before-packages' }),
      ],
    }),
    pbs: () => dated,
  });
  const points = await rows();
  expect(points).toHaveLength(5);
  const controls = points.map((row) => within(row).getByRole('button', { name: /Expand details/ }));
  expect(new Set(controls.map((button) => button.getAttribute('aria-label'))).size).toBe(5);
  for (const id of ['newer', 'older', 'offsite', 'before-upgrade', 'before-packages']) {
    const button = screen.getByRole('button', { name: new RegExp(`Identifier: ${id}$`) });
    expect(button).not.toHaveClass('sr-only');
    expect(button).toHaveAccessibleName(/Created:/);
    expect(button).toHaveAccessibleName(/PVE connection: pve.cluster.production.london/);
    fireEvent.click(button);
    const detail = screen.getByTestId('proxmox-recoverable-detail');
    expect(detail).toHaveAttribute(
      'data-inline-detail-for',
      `${dated.some((p) => p.id === id) ? 'pbs' : 'snapshot'}:${id}`,
    );
    const created = within(detail)
      .getByText('Created')
      .closest('tr')!
      .querySelector('td:last-child')!;
    expect(button.getAttribute('aria-label')).toContain(`Created: ${created.textContent}`);
  }
  expect(screen.getAllByTestId('proxmox-recoverable-detail')).toHaveLength(1);
  expect(api.fetch).toHaveBeenCalledTimes(2);
});
