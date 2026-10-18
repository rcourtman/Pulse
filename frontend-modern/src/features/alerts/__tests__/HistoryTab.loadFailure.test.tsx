import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import type { Alert } from '@/types/api';
import { HistoryTab } from '../tabs/HistoryTab';

const [search, setSearch] = createSignal('');
let liveAlerts: Record<string, Alert> = {};

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    pathname: '/alerts/history',
    hash: '',
    get search() {
      return search();
    },
  }),
  useNavigate: () => (path: string) =>
    setSearch(path.includes('?') ? path.slice(path.indexOf('?')) : ''),
}));
vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: liveAlerts }),
}));
vi.mock('@/hooks/useBreakpoint', () => ({ useBreakpoint: () => ({ isMobile: () => false }) }));
vi.mock('@/api/alerts', () => ({ AlertsAPI: { getHistory: vi.fn(), clearHistory: vi.fn() } }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

type History = Awaited<ReturnType<typeof AlertsAPI.getHistory>>;
const deferred = () => {
  let resolve!: (value: History) => void;
  const promise = new Promise<History>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
};
const row = (id: string) =>
  ({
    id,
    type: 'cpu',
    level: 'warning',
    resourceId: id,
    resourceName: id,
    startTime: new Date(Date.now() - 60000).toISOString(),
    lastSeen: new Date().toISOString(),
    message: `Recorded ${id}`,
    acknowledged: false,
  }) as Alert;
const mount = () =>
  render(() => <HistoryTab getResource={() => undefined} allResources={() => []} />);

afterEach(cleanup);
beforeEach(() => {
  vi.mocked(AlertsAPI.getHistory).mockReset();
  vi.mocked(AlertsAPI.clearHistory).mockReset();
  liveAlerts = {};
  setSearch('');
  localStorage.clear();
});

describe('existing History page read failure', () => {
  it('announces a safe error, not an empty result; explicit retry is single-flight and read-only', async () => {
    vi.mocked(AlertsAPI.getHistory).mockRejectedValueOnce(new Error('secret-provider-body'));
    const retry = deferred();
    vi.mocked(AlertsAPI.getHistory).mockReturnValueOnce(retry.promise);
    const { container } = mount();
    await screen.findByRole('alert');
    expect(screen.getByText('Could not load alert history')).toBeInTheDocument();
    expect(container).not.toHaveTextContent('secret-provider-body');
    expect(screen.queryByText('No alerts found')).not.toBeInTheDocument();
    expect(screen.queryByText('Alert frequency')).not.toBeInTheDocument();
    expect(
      within(screen.getByRole('group', { name: 'Severity' })).getByRole('button', { name: 'All' }),
    ).toBeInTheDocument();
    const button = screen.getByRole('button', { name: 'Retry history' });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(AlertsAPI.getHistory).toHaveBeenCalledTimes(2);
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');
    expect(button).toHaveTextContent('Retrying…');
    expect(screen.getByRole('status')).toHaveTextContent('Loading alert history');
    retry.resolve([]);
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
    expect(screen.getByText('No alerts found')).toBeInTheDocument();
    expect(screen.getByText('Alert frequency')).toBeInTheDocument();
    expect(AlertsAPI.clearHistory).not.toHaveBeenCalled();
  });

  it('does not present zero totals before the first read has completed', async () => {
    const initial = deferred();
    vi.mocked(AlertsAPI.getHistory).mockReturnValueOnce(initial.promise);
    mount();
    expect(screen.getByRole('status')).toHaveTextContent('Loading alert history');
    expect(screen.queryByText('No alerts found')).not.toBeInTheDocument();
    expect(screen.queryByText('Alert frequency')).not.toBeInTheDocument();
    initial.resolve([]);
    await screen.findByText('No alerts found');
  });

  it('keeps live alerts visible but does not relabel saved rows after a larger-range failure', async () => {
    liveAlerts = { live: row('live') };
    vi.mocked(AlertsAPI.getHistory).mockResolvedValueOnce([row('old-range')]);
    mount();
    await screen.findAllByText('Recorded old-range');
    vi.mocked(AlertsAPI.getHistory).mockRejectedValueOnce(new Error('Unavailable'));
    fireEvent.click(screen.getByRole('button', { name: 'Last 30d' }));
    await screen.findByRole('alert');
    expect(screen.queryByText('Recorded old-range')).not.toBeInTheDocument();
    expect(screen.getAllByText('Recorded live').length).toBeGreaterThan(0);
    expect(screen.queryByText('Alert frequency')).not.toBeInTheDocument();
    expect(search()).toBe('?period=30d');
    vi.mocked(AlertsAPI.getHistory).mockResolvedValueOnce([row('new-range')]);
    fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
    await screen.findAllByText('Recorded new-range');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(search()).toBe('?period=30d');
    expect(AlertsAPI.clearHistory).not.toHaveBeenCalled();
  });
});
