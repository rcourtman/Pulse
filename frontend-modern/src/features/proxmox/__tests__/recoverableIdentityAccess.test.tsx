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
  it.each([320, 720, 1200])(
    'keeps each source’s exact reported date and uncertainty inspectable at %ipx',
    async (width) => {
      api.width = width;
      const exact = '2026-10-04T12:15:34.123456789+02:00';
      const literal = '<img src=x onerror=alert(1)>';
      const dates = [exact, '2026-09-31T10:00:00Z', '2099-10-04T10:00:00Z', '', literal];
      mount({
        guests: workloads().slice(0, 1),
        pve: () => ({
          ...emptyPVE,
          storageBackups: dates.map((time, index) =>
            identityArchive({ id: `file-${index}`, instance: scopes[0], time }),
          ),
          guestSnapshots: dates.map((time, index) =>
            identitySnapshot({ id: `snapshot-${index}`, instance: scopes[0], time }),
          ),
        }),
        pbs: () =>
          dates.map((backupTime, index) => identityPBS({ id: `pbs-${index}`, backupTime })),
      });
      const points = await rows();
      expect(points).toHaveLength(15);
      for (const kind of ['pbs', 'file', 'snapshot']) {
        for (const [index, date] of dates.entries()) {
          const button = screen.getByRole('button', {
            name: new RegExp(`Identifier: ${kind}-${index}$`),
          });
          await fireEvent.click(button);
          const panel = screen.getByTestId('proxmox-recoverable-detail');
          const reported = within(panel).getByText('Reported date').closest('tr')!;
          expect(reported).toHaveTextContent(date || 'Not reported');
          expect(reported.querySelector('td:last-child span')).not.toHaveClass('truncate');
          const created = within(panel).getByText('Created').closest('tr')!;
          if (index === 0) {
            expect(created).toHaveTextContent('2026');
          } else {
            expect(created).toHaveTextContent('Unknown');
            const notice = screen.getByText(
              /Backup entries with unavailable or future dates remain listed/,
            );
            expect(notice).toHaveTextContent('Their ages are unknown.');
            expect(notice).toHaveTextContent('excluded from the activity chart and day filters');
          }
          expect(panel.querySelector('img')).toBeNull();
        }
      }
      expect(screen.getAllByTestId('proxmox-recoverable-detail')).toHaveLength(1);
      expect(api.fetch).toHaveBeenCalledTimes(2);
    },
  );

  it.each([320, 1200])(
    'updates the open reported date and its interpretation without replacing the record at %ipx',
    async (width) => {
      api.width = width;
      vi.useFakeTimers({
        now: BACKUP_IDENTITY_NOW,
        toFake: ['Date', 'setInterval', 'clearInterval'],
      });
      let date = '2026-09-31T10:00:00Z';
      mount({
        guests: workloads().slice(0, 1),
        pve: () => emptyPVE,
        pbs: () => [identityPBS({ backupTime: date })],
      });
      const [point] = await rows();
      const toggle = within(point).getByRole('button', { name: /Expand details/ });
      await fireEvent.click(toggle);
      toggle.focus();
      const panel = screen.getByTestId('proxmox-recoverable-detail');
      expect(
        screen.getByText(/Backup entries with unavailable or future dates remain listed/),
      ).toBeVisible();
      for (const next of ['2026-10-04T12:15:34.123456789+02:00', '']) {
        date = next;
        await vi.advanceTimersByTimeAsync(30_000);
        await waitFor(() =>
          expect(within(panel).getByText('Reported date').closest('tr')).toHaveTextContent(
            next || 'Not reported',
          ),
        );
        expect(screen.getByTestId('proxmox-recoverable-detail')).toBe(panel);
        expect(toggle).toHaveFocus();
        const notice = screen.queryByText(
          /Backup entries with unavailable or future dates remain listed/,
        );
        if (next) expect(notice).toBeNull();
        else expect(notice).toBeVisible();
      }
      expect(api.fetch).toHaveBeenCalledTimes(6);
      expect(panel).not.toHaveTextContent('2026-09-31');
      expect(panel).not.toHaveTextContent('.123456789');
    },
  );

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
        // Rows stay on one line: identity truncates, never wraps.
        for (const element of [
          within(row).getByText(name, { exact: true }),
          row.querySelector('[data-backup-workload-connection]')!,
        ]) {
          expect(element).toHaveClass('truncate');
          expect(element).not.toHaveClass('whitespace-normal');
        }
        if (width < 720) {
          // Narrow rows are workload, age and state; source and location are drawer detail.
          expect(row.children).toHaveLength(3);
          expect(row.querySelector('[data-backup-source-connection]')).toBeNull();
          expect(row).not.toHaveTextContent(`repository-${index}`);
        } else {
          expect(row.querySelector('[data-backup-source-connection]')).toHaveTextContent(scope);
          expect(row).toHaveTextContent(`repository-${index}`);
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
      expect(screen.getByRole('button', { name: '2 status unknown' })).toBeInTheDocument();
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
      // A narrow row holds no reader or location text; the drawer has it whole.
      expect(point.querySelector('[data-backup-workload-connection]')).toBeNull();
      expect(point.querySelector('[data-backup-source-connection]')).toBeNull();
      fireEvent.click(within(point).getByRole('button', { name: /Expand details/ }));
      const panel = screen.getByTestId('proxmox-recoverable-detail');
      expect(
        within(panel).getByText('Connection', { exact: true }).closest('tr'),
      ).toHaveTextContent(kind === 'pbs' ? reader : scopes[0]);
      expect(panel).toHaveTextContent(
        kind === 'pbs' ? `${location} / pve-1` : kind === 'archive' ? location : 'pve-1',
      );
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
    const host = points.find((point) => point !== guest)!;
    // The inventory spans two PVE connections, so the guest row names its own
    // (not reported) rather than borrowing the reader; a host has none.
    expect(guest.querySelector('[data-backup-workload-connection]')).toHaveTextContent(
      'PVE: Not reported',
    );
    expect(host.querySelector('[data-backup-workload-connection]')).toBeNull();
    for (const point of points) {
      expect(point.querySelector('[data-backup-source-connection]')).toBeNull();
    }
    fireEvent.click(within(guest).getByRole('button', { name: /Expand details/ }));
    let panel = screen.getByTestId('proxmox-recoverable-detail');
    expect(within(panel).getByText('PVE connection').closest('tr')).toHaveTextContent(
      'Not reported',
    );
    expect(within(panel).getByText('Connection', { exact: true }).closest('tr')).toHaveTextContent(
      'shared-pbs',
    );
    expect(within(panel).queryByRole('link')).toBeNull();
    fireEvent.click(within(host).getByRole('button', { name: /Expand details/ }));
    panel = screen.getByTestId('proxmox-recoverable-detail');
    expect(within(panel).queryByText('PVE connection')).toBeNull();
    expect(within(panel).getByText('Connection', { exact: true }).closest('tr')).toHaveTextContent(
      'shared-pbs',
    );
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
    await waitFor(() => expect(panel).toHaveTextContent('after / pve-1'));
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
    expect(created).toHaveTextContent('2026');
    expect(within(detail).getByText('Identifier').closest('tr')).toHaveTextContent(id);
  }
  expect(screen.getAllByTestId('proxmox-recoverable-detail')).toHaveLength(1);
  expect(api.fetch).toHaveBeenCalledTimes(2);
});
