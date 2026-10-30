import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';
import { setKioskMode } from '@/utils/url';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
} from '../__fixtures__/backupIdentity';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
const guest = identityGuest('east-100', 'east', 'pve-1');
const searchLabel = 'Search backups by workload, node, source, or status';
let failed: Set<string>;
let empty: Set<string>;

function mount({ pveEmpty = false, pbsOrphaned = false } = {}) {
  api.fetch.mockImplementation(async (url: string) => {
    const source = url.endsWith('/pbs') ? 'pbs' : 'pve';
    return new Response(
      JSON.stringify({
        data:
          source === 'pbs'
            ? {
                backups: empty.has(source) ? [] : [identityPBS(pbsOrphaned ? { vmid: '999' } : {})],
              }
            : {
                backupTasks: [],
                guestSnapshots: [],
                storageBackups: pveEmpty || empty.has(source) ? [] : [identityArchive()],
              },
      }),
      { status: failed.has(source) ? 403 : 200, headers: { 'Content-Type': 'application/json' } },
    );
  });
  api.json.mockResolvedValue({ data: [], policy: protectionPolicy });
  return render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} />}
      />
    </Router>
  ));
}
async function point(source = 'pbs') {
  return screen.findByRole('button', {
    name: new RegExp(`Expand details.*Identifier: ${source === 'pbs' ? 'pbs-100' : 'file-100'}$`),
  });
}
async function fail(source: string) {
  failed.add(source);
  await vi.advanceTimersByTimeAsync(30_000);
  return screen.findByRole('button', { name: `Retry ${source.toUpperCase()} inventory` });
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date', 'setInterval', 'clearInterval'] });
  resetCreateNonSuspendingQueryCacheForTest();
  failed = new Set();
  empty = new Set();
  window.history.replaceState({}, '', '/proxmox/backups/date');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  setKioskMode(false);
  window.sessionStorage.removeItem('pulse_kiosk_mode');
  vi.useRealTimers();
});

describe('retiring backup evidence retains a usable keyboard position', () => {
  it.each(['pbs', 'pve'])(
    'hands a withdrawn %s disclosure to inventory retry, without leaving its old point',
    async (source) => {
      mount();
      const disclosure = await point(source);
      fireEvent.click(disclosure);
      disclosure.focus();
      const retry = await fail(source);
      await waitFor(() => expect(retry).toHaveFocus());
      expect(disclosure).not.toBeInTheDocument();
      expect(screen.queryByTestId('proxmox-recoverable-detail')).toBeNull();
      expect(document.querySelectorAll('[data-proxmox-backup-row="recoverable"]')).toHaveLength(1);
      expect(api.fetch).toHaveBeenCalledTimes(4);
      failed.clear();
      await fireEvent.click(retry);
      await waitFor(() => expect(retry).not.toBeInTheDocument());
      await waitFor(() => expect(screen.getByRole('link', { name: 'By date' })).toHaveFocus());
      expect(screen.getByTestId('proxmox-recoverable-detail')).toHaveTextContent(
        source === 'pbs' ? 'pbs-100' : 'file-100',
      );
      expect(api.fetch).toHaveBeenCalledTimes(5);
    },
  );

  it.each(['pbs', 'pve'])(
    'hands focus inside a withdrawn %s detail to retry when its controlling row is gone',
    async (source) => {
      mount();
      fireEvent.click(await point(source));
      const detail = screen.getByTestId('proxmox-recoverable-detail');
      within(detail)
        .getByRole('button', { name: /Collapse .* details/ })
        .focus();
      await waitFor(() => expect(detail).toContainElement(document.activeElement as HTMLElement));
      const retry = await fail(source);
      await waitFor(() => expect(retry).toHaveFocus());
      expect(detail).not.toBeInTheDocument();
    },
  );

  it('does not let the cleanup fallback replace normal same-point close focus', async () => {
    mount();
    const disclosure = await point();
    fireEvent.click(disclosure);
    const close = within(screen.getByTestId('proxmox-recoverable-detail')).getByRole('button', {
      name: /Collapse .* details/,
    });
    close.focus();
    await fireEvent.click(close);
    await waitFor(() => expect(disclosure).toHaveFocus());
    expect(disclosure).toHaveAttribute('aria-expanded', 'false');
  });

  it('returns to the active view when an accepted empty poll removes the focused point', async () => {
    mount();
    const disclosure = await point();
    fireEvent.click(disclosure);
    within(screen.getByTestId('proxmox-recoverable-detail'))
      .getByRole('button', { name: /Collapse .* details/ })
      .focus();
    empty.add('pbs');
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(screen.getByRole('link', { name: 'By date' })).toHaveFocus());
    expect(screen.queryByRole('button', { name: 'Retry PBS inventory' })).toBeNull();
    expect(disclosure).not.toBeInTheDocument();
  });

  it('retains focus when lookup removes a Coverage workload as its source fails', async () => {
    window.history.replaceState({}, '', '/proxmox/backups/coverage?q=pbs-100');
    mount();
    const disclosure = await screen.findByRole('button', { name: /Expand details for east-100/ });
    fireEvent.click(disclosure);
    const detail = screen.getByTestId('proxmox-coverage-detail');
    within(detail)
      .getByRole('button', { name: /Collapse .* details/ })
      .focus();
    const retry = await fail('pbs');
    await waitFor(() => expect(retry).toHaveFocus());
    expect(detail).not.toBeInTheDocument();
    expect(document.querySelector('[data-proxmox-backup-row="coverage"]')).toBeNull();
    expect(screen.getByPlaceholderText(searchLabel)).toHaveValue('pbs-100');
  });

  it.each([true, false])(
    'retains focus when a PBS-filtered activity chart is withdrawn (PVE empty: %s)',
    async (pveEmpty) => {
      window.history.replaceState({}, '', '/proxmox/backups/date?source=pbs');
      mount({ pveEmpty });
      await point();
      const day = document.querySelector<HTMLButtonElement>(
        '[data-backup-activity-day="2026-10-04"]',
      )!;
      expect(day).not.toBeNull();
      day.focus();
      const retry = await fail('pbs');
      await waitFor(() => expect(retry).toHaveFocus());
      expect(day).not.toBeInTheDocument();
    },
  );

  it('retains focus when a failed source removes the unmatched-evidence disclosure', async () => {
    window.history.replaceState({}, '', '/proxmox/backups/coverage');
    mount({ pbsOrphaned: true });
    const disclosure = await screen.findByRole('button', { name: /1 unmatched backup/ });
    disclosure.focus();
    const retry = await fail('pbs');
    await waitFor(() => expect(retry).toHaveFocus());
    expect(disclosure).not.toBeInTheDocument();
  });

  it.each(['search', 'peer detail', 'outside'])(
    'does not steal focus from %s when an unrelated source fails',
    async (target) => {
      mount();
      fireEvent.click(await point('pve'));
      const outside = document.createElement('button');
      document.body.append(outside);
      const control =
        target === 'search'
          ? screen.getByPlaceholderText(searchLabel)
          : target === 'peer detail'
            ? within(screen.getByTestId('proxmox-recoverable-detail')).getByRole('button', {
                name: /Collapse .* details/,
              })
            : outside;
      control.focus();
      await fail('pbs');
      await waitFor(() => expect(control).toHaveFocus());
      expect(screen.getByTestId('proxmox-recoverable-detail')).toHaveTextContent('file-100');
      outside.remove();
    },
  );

  it('retains the whole-inspection focus when both inventories fail', async () => {
    mount();
    await point();
    screen.getByPlaceholderText(searchLabel).focus();
    failed.add('pve');
    const retry = await fail('pbs');
    await waitFor(() => expect(screen.queryByPlaceholderText(searchLabel)).toBeNull());
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Retry PVE inventory' })).toHaveFocus(),
    );
    expect(retry).toBeInTheDocument();
  });

  it('uses the named inspection when kiosk mode hides the active view on an empty recovery', async () => {
    setKioskMode(true);
    mount();
    const disclosure = await point();
    disclosure.focus();
    empty.add('pbs');
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() =>
      expect(screen.getByRole('region', { name: 'Backup inspection' })).toHaveFocus(),
    );
    expect(screen.queryByRole('navigation', { name: 'Backup views' })).toBeNull();
  });

  it('does not restore focus from retired evidence after leaving the inspection', async () => {
    const mounted = mount();
    const disclosure = await point();
    fireEvent.click(disclosure);
    within(screen.getByTestId('proxmox-recoverable-detail'))
      .getByRole('button', { name: /Collapse .* details/ })
      .focus();
    mounted.unmount();
    const outside = document.createElement('button');
    document.body.append(outside);
    outside.focus();
    await Promise.resolve();
    await Promise.resolve();
    expect(outside).toHaveFocus();
    outside.remove();
  });
});
