import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
  identityTask,
} from '../__fixtures__/backupIdentity';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn(), width: 1200 }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => api.width }),
}));

const guest = identityGuest('checkout-100', 'east', 'pve-1');
const pbs = identityPBS();
const archive = identityArchive();
const snapshot = identitySnapshot(); // Newer than both independent backups.
const task = identityTask();
const complete = protectionPosture(guest.id);
const partial = {
  ...complete,
  providerStates: [{ ...complete.providerStates[0], historyCompleteness: 'partial' as const }],
};
const response = (data: unknown) => new Response(JSON.stringify({ data }));

function mount({
  readPBS = () => response({ backups: [pbs] }),
  assessment = complete,
}: {
  readPBS?: () => Response;
  assessment?: typeof complete;
} = {}) {
  api.fetch.mockImplementation(async (url: string) =>
    url === '/api/backups/pbs'
      ? readPBS()
      : response({ storageBackups: [archive], guestSnapshots: [snapshot], backupTasks: [task] }),
  );
  api.json.mockResolvedValue({ data: [assessment], policy: protectionPolicy });
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} />}
      />
    </Router>
  ));
}

async function expand() {
  const button = await screen.findByRole('button', { name: /Expand details for checkout-100/ });
  const row = button.closest('tr')!;
  await within(row).findByText('Protected', { exact: true });
  fireEvent.click(button);
  const panel = await screen.findByTestId('proxmox-coverage-detail');
  return { row, panel };
}

const summary = (panel: HTMLElement) => panel.querySelector('[data-coverage-newest-backup]')!;
const pointCount = (panel: HTMLElement) =>
  panel.querySelectorAll('[data-coverage-evidence-point]').length;
const datedAge = (row: HTMLElement, when: string) => within(row).getByTitle(new RegExp(when));

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW, toFake: ['Date', 'setInterval', 'clearInterval'] });
  resetCreateNonSuspendingQueryCacheForTest();
  api.width = 1200;
  window.history.replaceState({}, '', '/proxmox/backups/coverage');
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe('Coverage age describes the available backup list', () => {
  it.each([1200, 320])(
    'qualifies a dated backup when provider history is partial at %ipx',
    async (width) => {
      api.width = width;
      mount({ assessment: partial });
      const { row, panel } = await expand();
      expect(within(row).getByText('Listed only')).toBeInTheDocument();
      expect(within(row).getByText('from the available list only')).toHaveClass('sr-only');
      expect(datedAge(row, archive.time)).toBeInTheDocument();
      expect(summary(panel)).toHaveTextContent('Newest listed backup');
      expect(summary(panel)).toHaveTextContent(archive.storage);
      expect(pointCount(panel)).toBe(3);
    },
  );

  it('keeps the older location-scoped age distinct from the full read and restores it on clear', async () => {
    mount();
    const { row, panel } = await expand();
    expect(within(row).queryByText('Listed only')).toBeNull();
    expect(datedAge(row, archive.time)).toBeInTheDocument();
    expect(summary(panel)).toHaveTextContent('Newest backup');
    const filter = screen.getByRole('combobox', { name: 'Filter' });
    const location = screen.getByRole<HTMLOptionElement>('option', {
      name: 'Backup location: PBS · pbs-server / main',
    });
    fireEvent.change(filter, { target: { value: location.value } });
    await screen.findByRole('note', { name: 'Restore point scope' });
    expect(within(row).getByText('Listed only')).toBeInTheDocument();
    expect(datedAge(row, pbs.backupTime)).toBeInTheDocument();
    expect(summary(panel)).toHaveTextContent('Newest listed backup');
    expect(summary(panel)).toHaveTextContent('main / pve-1');
    expect(pointCount(panel)).toBe(1);
    fireEvent.click(screen.getByRole('button', { name: 'Remove Backup location filter' }));
    await waitFor(() => expect(within(row).queryByText('Listed only')).toBeNull());
    expect(datedAge(row, archive.time)).toBeInTheDocument();
    expect(summary(panel)).toHaveTextContent('Newest backup');
    expect(pointCount(panel)).toBe(3);
  });

  it('qualifies the still-readable backup during a failed inventory read until recovery', async () => {
    let readable = false;
    mount({
      readPBS: () =>
        readable ? response({ backups: [pbs] }) : new Response(null, { status: 503 }),
    });
    const { row, panel } = await expand();
    const retry = await screen.findByRole('button', { name: /Retry PBS/ });
    expect(within(row).getByText('Listed only')).toBeInTheDocument();
    expect(datedAge(row, archive.time)).toBeInTheDocument();
    expect(summary(panel)).toHaveTextContent('Newest listed backup');
    expect(pointCount(panel)).toBe(2);
    readable = true;
    fireEvent.click(retry);
    await waitFor(() => expect(pointCount(panel)).toBe(3));
    expect(within(row).queryByText('Listed only')).toBeNull();
    expect(datedAge(row, archive.time)).toBeInTheDocument();
    expect(summary(panel)).toHaveTextContent('Newest backup');
  });
});
