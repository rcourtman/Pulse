import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { recoveryDateKeyFromTimestamp } from '@/utils/recoveryDatePresentation';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
} from '../__fixtures__/backupIdentity';

const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({
  apiFetch,
  apiFetchJSON: vi.fn().mockResolvedValue({
    data: [],
    policy: {
      freshnessWindowSeconds: 604800,
      verificationWindowSeconds: 604800,
      requireVerification: true,
    },
  }),
}));

const now = Date.parse('2026-10-19T12:00:00Z');
const today = '2026-10-19T10:00:00Z';
const yesterday = '2026-10-18T10:00:00Z';
const earlier = '2026-10-17T10:00:00Z';
let replacement = false;

beforeEach(() => {
  vi.spyOn(Date, 'now').mockReturnValue(now);
  vi.stubGlobal('scrollTo', vi.fn());
  replacement = false;
  apiFetch.mockImplementation((url: string) =>
    Promise.resolve(
      new Response(
        JSON.stringify({
          data:
            url === '/api/backups/pve'
              ? {
                  backupTasks: [],
                  storageBackups: [identityArchive({ time: today })],
                  guestSnapshots: [identitySnapshot({ time: yesterday })],
                }
              : {
                  backups: [
                    identityPBS({
                      id: 'east-main',
                      backupTime: today,
                      size: replacement ? 4096 : 2048,
                    }),
                    identityPBS({
                      id: 'east-offsite',
                      instance: 'pbs-offsite',
                      backupTime: earlier,
                      size: 4096,
                    }),
                    identityPBS({
                      id: 'west-offsite',
                      instance: 'pbs-offsite',
                      namespace: 'pve-2',
                      vmid: '200',
                      backupTime: yesterday,
                      size: 8192,
                      verified: false,
                    }),
                    identityPBS({
                      id: 'west-undated',
                      instance: 'pbs-other',
                      namespace: 'pve-2',
                      vmid: '200',
                      backupTime: '',
                      size: 0,
                    }),
                    identityPBS({
                      id: 'west-running',
                      instance: 'pbs-offsite',
                      namespace: 'pve-2',
                      vmid: '200',
                      backupTime: today,
                      inProgress: true,
                      size: 16384,
                      verified: false,
                    }),
                  ],
                },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    ),
  );
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  apiFetch.mockReset();
  window.history.replaceState({}, '', '/');
  resetCreateNonSuspendingQueryCacheForTest();
});

const mount = (query = '') => {
  window.history.replaceState({}, '', '/proxmox/backups/date' + query);
  return render(() => (
    <Router>
      <Route
        path="/*"
        component={() => (
          <ProxmoxBackupsTable
            emptyIcon={<span />}
            workloads={[
              identityGuest('east-workload', 'east', 'pve-1'),
              identityGuest('west-workload', 'west', 'pve-2', { vmid: 200 }),
            ]}
          />
        )}
      />
    </Router>
  ));
};
const dayButtons = () => screen.queryAllByRole('button', { name: /: \d+ backups?(, selected)?$/ });
const activityCount = () =>
  dayButtons().reduce(
    (sum, button) => sum + Number(button.getAttribute('aria-label')!.match(/: (\d+) backups?/)![1]),
    0,
  );
const recordRows = () =>
  document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length;

describe('By date activity scope', () => {
  it.each([
    ['', 5, 7, true],
    ['?node=pve-1', 4, 4, false],
    ['?type=vm&q=west-workload', 1, 3, true],
    ['?q=east-workload', 4, 4, false],
    ['?source=pbs', 3, 5, true],
    ['?source=archive', 1, 1, false],
    ['?source=snapshot', 1, 1, false],
    ['?source=verified', 2, 3, true],
    ['?source=unverified', 1, 1, false],
    ['?location=pbs%3Apbs-offsite%3Amain', 2, 3, false],
    [
      '?node=pve-1&type=vm&source=pbs&location=pbs%3Apbs-offsite%3Amain&q=east-workload',
      1,
      1,
      false,
    ],
    ['?type=ct', 0, 0, false],
  ] as const)(
    'uses the list scope, excluding incomplete/undated activity (%s)',
    async (query, count, rows, undated) => {
      mount(query);
      await screen.findByRole('button', { name: 'Refresh backups' });
      await waitFor(() => expect(recordRows()).toBe(rows));
      expect(activityCount()).toBe(count);
      expect(
        Boolean(
          screen.queryByText(/Backup entries with unavailable or future dates remain listed/),
        ),
      ).toBe(undated);
      if (!count)
        expect(screen.getByText(/No completed restore-point activity/)).toBeInTheDocument();
      expect(apiFetch).toHaveBeenCalledTimes(2);
    },
  );

  it('retains the complete scoped timeline when selecting or clearing a day', async () => {
    mount('?q=east-workload');
    await waitFor(() => expect(activityCount()).toBe(4));
    const timeline = dayButtons().map((el) =>
      el.getAttribute('aria-label')!.replace(/, selected$/, ''),
    );
    await fireEvent.click(screen.getByRole('button', { name: /Sort by Created/ }));
    expect(dayButtons().map((el) => el.getAttribute('aria-label'))).toEqual(timeline);
    await fireEvent.click(screen.getByRole('button', { name: /: 2 backups$/ }));
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get('day')).toBe(
        recoveryDateKeyFromTimestamp(Date.parse(today)),
      ),
    );
    expect(recordRows()).toBe(2);
    expect(
      dayButtons().map((el) => el.getAttribute('aria-label')!.replace(/, selected$/, '')),
    ).toEqual(timeline);
    await fireEvent.click(screen.getByRole('button', { name: 'Clear date filter' }));
    await waitFor(() => expect(recordRows()).toBe(4));
    expect(activityCount()).toBe(4);
    expect(apiFetch).toHaveBeenCalledTimes(2);
  });

  it('updates count, missing-size evidence and volume on filters, refresh and clear without extra filter reads', async () => {
    mount();
    await waitFor(() => expect(activityCount()).toBe(5));
    const search = screen.getByPlaceholderText(/Search backups by workload/);
    await fireEvent.input(search, { target: { value: 'east-workload' } });
    await waitFor(() => expect(activityCount()).toBe(4));
    await fireEvent.click(screen.getByRole('button', { name: 'Volume' }));
    expect(screen.getByRole('button', { name: /: 3.00 KB$/ })).toBeInTheDocument();
    expect(
      screen.getByText(/1 completed restore point in this window has no positive size reported/),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /: 8.00 KB$/ })).not.toBeInTheDocument();
    expect(apiFetch).toHaveBeenCalledTimes(2);
    replacement = true;
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh backups' }));
    await screen.findByRole('button', { name: /: 5.00 KB$/ });
    expect(search).toHaveValue('east-workload');
    expect(screen.getByRole('button', { name: 'Volume' })).toHaveAttribute('aria-pressed', 'true');
    expect(apiFetch).toHaveBeenCalledTimes(4);
    await fireEvent.input(search, { target: { value: 'west-workload' } });
    await screen.findByRole('button', { name: /: 8.00 KB$/ });
    expect(screen.queryByText(/no positive size reported/)).not.toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: /Clear filters/ }));
    expect(screen.getByRole('button', { name: /: 5.00 KB$/ })).toBeInTheDocument();
    expect(
      screen.getByRole('button', {
        name: /: 8.00 KB reported\. 1 backup without a positive size$/,
      }),
    ).toBeInTheDocument();
    expect(apiFetch).toHaveBeenCalledTimes(4);
  });
});
