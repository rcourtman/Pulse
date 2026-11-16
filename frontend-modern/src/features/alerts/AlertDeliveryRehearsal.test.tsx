import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { NotificationsAPI, type DeliveryRehearsal } from '@/api/notifications';
import { AlertDeliveryRehearsal } from './AlertDeliveryRehearsal';

let org = 'org-a';
vi.mock('@/utils/apiClient', () => ({ getOrgID: () => org }));
vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    listRehearsals: vi.fn(),
    getRehearsal: vi.fn(),
    startRehearsal: vi.fn(),
    recoverRehearsal: vi.fn(),
  },
}));

const fixture = (): DeliveryRehearsal => ({
  id: 'run-a',
  createdAt: '2026-11-16T12:00:00Z',
  level: 'warning',
  tags: [],
  policy: { status: 'passed', reason: 'admitted', message: 'Delivery is enabled.', attempts: 0 },
  destinations: [
    {
      id: 'ops',
      name: 'Ops',
      firing: {
        status: 'failed',
        reason: 'rejected',
        message: 'Receiver rejected the message.',
        attempts: 1,
      },
      recovery: {
        status: 'not_started',
        reason: 'not_started',
        message: 'Send recovery after inspecting firing.',
        attempts: 0,
      },
    },
  ],
});
function open() {
  fireEvent.click(screen.getByText('Rehearse alert delivery'));
}

describe('AlertDeliveryRehearsal', () => {
  beforeEach(() => {
    org = 'org-a';
    vi.resetAllMocks();
    vi.mocked(NotificationsAPI.listRehearsals).mockResolvedValue([]);
  });
  afterEach(cleanup);

  it('retains separate history identities when starting and selecting another run', async () => {
    const older = { ...fixture(), tags: ['retained-old'], recoveredAt: '2026-11-16T12:02:00Z' };
    const newer = {
      ...fixture(),
      id: 'run-b',
      createdAt: '2026-11-16T12:03:00Z',
      tags: ['current-new'],
    };
    vi.mocked(NotificationsAPI.listRehearsals).mockResolvedValue([older]);
    vi.mocked(NotificationsAPI.startRehearsal).mockResolvedValue(newer);
    render(() => <AlertDeliveryRehearsal />);
    open();
    expect(await screen.findByText('warning · retained-old')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Send rehearsal alert' }));
    expect(await screen.findByText('warning · current-new')).toBeInTheDocument();
    const select = screen.getByRole('combobox', { name: /Recent rehearsals/ }) as HTMLSelectElement;
    expect(Array.from(select.options, (option) => option.value)).toEqual(['run-b', 'run-a']);
    fireEvent.change(select, { target: { value: 'run-a' } });
    expect(await screen.findByText('warning · retained-old')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Resume current rehearsal' })).toBeInTheDocument();
  });

  it('opens an exact linked report outside the recent list and follows a new link', async () => {
    const old = {
      ...fixture(),
      id: 'older-run',
      tags: ['linked-older'],
      recoveredAt: '2026-11-16T12:01:00Z',
    };
    const recent = {
      ...fixture(),
      id: 'recent-run',
      tags: ['recent'],
      recoveredAt: '2026-11-16T12:02:00Z',
    };
    vi.mocked(NotificationsAPI.listRehearsals).mockResolvedValue([recent]);
    vi.mocked(NotificationsAPI.getRehearsal).mockResolvedValue(old);
    const [requested, setRequested] = createSignal<string | undefined>('older-run');
    render(() => <AlertDeliveryRehearsal requestedRehearsalID={requested} />);
    expect(await screen.findByText('warning · linked-older')).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: /Rehearsal started/ })).toHaveFocus(),
    );
    expect(screen.getByTestId('delivery-rehearsal')).toHaveAttribute('open');
    expect(NotificationsAPI.getRehearsal).toHaveBeenCalledWith('older-run', {
      expectedOrgID: 'org-a',
      retry: false,
    });
    setRequested('recent-run');
    expect(await screen.findByText('warning · recent')).toBeInTheDocument();
  });

  it('returns to recent evidence when a linked report cannot be read', async () => {
    const recent = { ...fixture(), tags: ['recent'], recoveredAt: '2026-11-16T12:02:00Z' };
    vi.mocked(NotificationsAPI.listRehearsals).mockResolvedValue([recent]);
    vi.mocked(NotificationsAPI.getRehearsal).mockRejectedValue(new Error('missing report'));
    render(() => <AlertDeliveryRehearsal requestedRehearsalID={() => 'expired-link'} />);
    expect(await screen.findByRole('alert')).toHaveTextContent('evidence is unavailable');
    fireEvent.click(screen.getByRole('button', { name: 'Show recent rehearsals' }));
    expect(await screen.findByText('warning · recent')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('distinguishes queue failures, provider acceptance and suppressed recovery', async () => {
    const run = fixture();
    run.destinations[0].firing.diagnostic = 'Provider HTTP 400, response body withheld.';
    vi.mocked(NotificationsAPI.startRehearsal).mockResolvedValue(run);
    vi.mocked(NotificationsAPI.recoverRehearsal).mockResolvedValue({
      ...run,
      recoveredAt: '2026-11-16T12:05:00Z',
      destinations: [
        {
          ...run.destinations[0],
          recovery: {
            status: 'suppressed',
            reason: 'firing_not_delivered',
            message: 'No successful firing receipt.',
            attempts: 0,
          },
        },
      ],
    });
    render(() => <AlertDeliveryRehearsal />);
    open();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Send rehearsal alert' })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Send rehearsal alert' }));
    expect(await screen.findByText('Failed')).toBeInTheDocument();
    expect(screen.getByText('Receiver rejected the message.')).toBeInTheDocument();
    expect(screen.getByText('Provider HTTP 400, response body withheld.')).not.toBeVisible();
    fireEvent.click(screen.getByText('Delivery details', { exact: true }));
    expect(screen.getByText('Provider HTTP 400, response body withheld.')).toBeVisible();
    const detailsSummary = screen.getByText('Delivery details', { exact: true });
    const refreshed = structuredClone(run);
    refreshed.policy.message = 'Fresh admission evidence.';
    vi.mocked(NotificationsAPI.listRehearsals).mockResolvedValue([refreshed]);
    fireEvent.click(screen.getByRole('button', { name: 'Refresh rehearsal' }));
    expect(await screen.findByText('Fresh admission evidence.')).toBeInTheDocument();
    expect(screen.getByText('Delivery details', { exact: true })).toBe(detailsSummary);
    expect(screen.getByText('Provider HTTP 400, response body withheld.')).toBeVisible();
    expect(
      screen.getByText(/A passed delivery means the destination accepted it/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Send recovery and finish rehearsal' }));
    expect(await screen.findByText('Suppressed')).toBeInTheDocument();
    expect(screen.getByText('No successful firing receipt.')).toBeInTheDocument();
    expect(NotificationsAPI.recoverRehearsal).toHaveBeenCalledWith('run-a', {
      expectedOrgID: 'org-a',
      retry: false,
    });
  });

  it('blocks unsaved settings and unreadable history, and validates tags before any send', async () => {
    vi.mocked(NotificationsAPI.listRehearsals).mockRejectedValueOnce(new Error('unreadable'));
    render(() => <AlertDeliveryRehearsal hasUnsavedChanges={() => true} />);
    open();
    expect(await screen.findByRole('alert')).toHaveTextContent('unavailable');
    expect(screen.getByRole('button', { name: 'Send rehearsal alert' })).toBeDisabled();
    expect(screen.getByText(/Save or discard/)).toBeInTheDocument();
    expect(NotificationsAPI.startRehearsal).not.toHaveBeenCalled();
    cleanup();
    vi.mocked(NotificationsAPI.listRehearsals).mockResolvedValue([]);
    render(() => <AlertDeliveryRehearsal />);
    open();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Send rehearsal alert' })).toBeEnabled(),
    );
    fireEvent.input(screen.getByLabelText(/Routing tags/), { target: { value: 'x'.repeat(65) } });
    fireEvent.click(screen.getByRole('button', { name: 'Send rehearsal alert' }));
    expect(screen.getByRole('alert')).toHaveTextContent('at most 32 routing tags');
    expect(NotificationsAPI.startRehearsal).not.toHaveBeenCalled();
  });

  it('retries an uncertain start with the same ID and reconciles a committed request on Refresh', async () => {
    vi.mocked(NotificationsAPI.startRehearsal).mockRejectedValue(new Error('response lost'));
    render(() => <AlertDeliveryRehearsal />);
    open();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Send rehearsal alert' })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Send rehearsal alert' }));
    const retry = await screen.findByRole('button', { name: 'Retry same rehearsal request' });
    await waitFor(() => expect(retry).toBeEnabled());
    const firstID = vi.mocked(NotificationsAPI.startRehearsal).mock.calls[0][0].id;
    fireEvent.click(retry);
    await waitFor(() => expect(NotificationsAPI.startRehearsal).toHaveBeenCalledTimes(2));
    expect(vi.mocked(NotificationsAPI.startRehearsal).mock.calls[1][0].id).toBe(firstID);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Refresh rehearsal' })).toBeEnabled(),
    );
    vi.mocked(NotificationsAPI.listRehearsals).mockResolvedValue([{ ...fixture(), id: firstID }]);
    fireEvent.click(screen.getByRole('button', { name: 'Refresh rehearsal' }));
    expect(await screen.findByText('Receiver rejected the message.')).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Retry same rehearsal request' }),
    ).not.toBeInTheDocument();
  });

  it('never applies an old workspace reply after the context changes', async () => {
    let resolve!: (run: DeliveryRehearsal) => void;
    vi.mocked(NotificationsAPI.startRehearsal).mockReturnValue(
      new Promise((done) => {
        resolve = done;
      }),
    );
    render(() => <AlertDeliveryRehearsal />);
    open();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Send rehearsal alert' })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Send rehearsal alert' }));
    org = 'org-b';
    resolve(fixture());
    await Promise.resolve();
    expect(screen.queryByText('Receiver rejected the message.')).not.toBeInTheDocument();
  });
});
