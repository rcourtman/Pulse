import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { SecurityAPI } from '@/api/security';
import type { ActionAuditRecord, ActionDetailResponse } from '@/types/actionAudit';
import Actions from '../../../pages/Actions';

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: { listActions: vi.fn(), getAction: vi.fn() },
}));
vi.mock('@/api/security', () => ({ SecurityAPI: { getStatus: vi.fn() } }));
vi.mock('@/api/patrol', () => ({ getPatrolAutonomySettings: vi.fn().mockResolvedValue({}) }));

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};

const audit = (id: string, state: ActionAuditRecord['state'] = 'executing'): ActionAuditRecord => ({
  id,
  createdAt: '2026-09-29T18:00:00Z',
  updatedAt: '2026-09-29T18:01:00Z',
  state,
  decisionRevision: 1,
  request: {
    requestId: `request-${id}`,
    resourceId: `docker:container:${id}`,
    capabilityName: 'update',
    reason: `Update ${id}`,
    requestedBy: 'operator',
  },
  plan: {
    actionId: id,
    requestId: `request-${id}`,
    allowed: true,
    requiresApproval: false,
    approvalPolicy: 'none',
    rollbackAvailable: false,
    expiresAt: '2026-09-29T21:00:00Z',
    planHash: `sha256:plan-${id}`,
  },
  verificationOutcome: { status: 'unknown' },
});

const detail = (id: string): ActionDetailResponse => ({
  audit: audit(id),
  events: [],
  readiness: {
    ready: false,
    code: 'receipt_pending',
    message: 'Awaiting receipt',
    refreshable: false,
    checkedAt: '2026-09-29T18:01:00Z',
  },
  attempt: {
    id: `attempt-${id}`,
    actionId: id,
    state: 'receipt_pending',
    createdAt: '2026-09-29T18:00:00Z',
    updatedAt: '2026-09-29T18:01:00Z',
    dispatchCount: 1,
  },
});

const renderActions = () =>
  render(() => (
    <Router>
      <Route path="*" component={Actions} />
    </Router>
  ));

beforeEach(() => {
  window.history.replaceState(null, '', '/');
  vi.mocked(ResourceActionsAPI.listActions).mockResolvedValue({
    actions: [audit('edge-a'), audit('edge-b')],
    count: 2,
    view: 'pending',
    readOnly: false,
  });
  vi.mocked(SecurityAPI.getStatus).mockResolvedValue({
    hasAuthentication: true,
    requiresAuth: true,
    settingsCapabilities: { authenticationWrite: false },
  } as Awaited<ReturnType<typeof SecurityAPI.getStatus>>);
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  window.history.replaceState(null, '', '/');
});

describe('Actions request ownership', () => {
  it('keeps the newest selected action when an older detail response arrives last', async () => {
    const first = deferred<ActionDetailResponse>();
    vi.mocked(ResourceActionsAPI.getAction).mockImplementation((id) =>
      id === 'edge-a' ? first.promise : Promise.resolve(detail('edge-b')),
    );
    renderActions();
    fireEvent.click(
      await screen.findByRole('button', { name: /Review Update on docker:container:edge-a/ }),
    );
    fireEvent.click(
      screen.getByRole('button', { name: /Review Update on docker:container:edge-b/ }),
    );
    expect(await screen.findByText('docker:container:edge-b')).toBeVisible();

    first.resolve(detail('edge-a'));
    await waitFor(() => expect(ResourceActionsAPI.getAction).toHaveBeenCalledTimes(2));
    expect(screen.getByRole('dialog')).toHaveTextContent('docker:container:edge-b');
    expect(screen.getByRole('dialog')).not.toHaveTextContent('docker:container:edge-a');
    expect(window.location.search).toBe('?action=edge-b');
  });

  it('does not reopen a closed review when a receipt re-read finishes late', async () => {
    const read = deferred<ActionDetailResponse>();
    vi.mocked(ResourceActionsAPI.getAction)
      .mockResolvedValueOnce(detail('edge-a'))
      .mockReturnValueOnce(read.promise);
    renderActions();
    fireEvent.click(
      await screen.findByRole('button', { name: /Review Update on docker:container:edge-a/ }),
    );
    fireEvent.click(await screen.findByRole('button', { name: 'Check for receipt' }));
    fireEvent.click(screen.getByRole('button', { name: 'Close action review' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    await waitFor(() => expect(window.location.search).toBe(''));

    read.resolve({ ...detail('edge-a'), audit: audit('edge-a', 'completed') });
    await waitFor(() => expect(ResourceActionsAPI.getAction).toHaveBeenCalledTimes(2));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(window.location.search).toBe('');
  });

  it('refuses details returned for a different action id', async () => {
    vi.mocked(ResourceActionsAPI.getAction).mockResolvedValue(detail('edge-b'));
    renderActions();
    fireEvent.click(
      await screen.findByRole('button', { name: /Review Update on docker:container:edge-a/ }),
    );
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Action details did not match the selected action.',
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('does not show an old Open-list response under History', async () => {
    const open = deferred<Awaited<ReturnType<typeof ResourceActionsAPI.listActions>>>();
    vi.mocked(ResourceActionsAPI.listActions).mockImplementation((requestedView) =>
      requestedView === 'pending'
        ? open.promise
        : Promise.resolve({
            actions: [audit('finished', 'completed')],
            count: 1,
            view: 'settled',
            readOnly: false,
          }),
    );
    renderActions();
    fireEvent.click(screen.getByRole('tab', { name: 'History' }));
    expect(await screen.findByRole('button', { name: /docker:container:finished/ })).toBeVisible();

    open.resolve({ actions: [audit('edge-a')], count: 1, view: 'pending', readOnly: false });
    await waitFor(() => expect(ResourceActionsAPI.listActions).toHaveBeenCalledTimes(2));
    expect(screen.getByRole('button', { name: /docker:container:finished/ })).toBeVisible();
    expect(
      screen.queryByRole('button', { name: /docker:container:edge-a/ }),
    ).not.toBeInTheDocument();
  });

  it('opens History from its route and returns to Open through the tab', async () => {
    window.history.replaceState(null, '', '/actions/history');
    renderActions();

    expect(await screen.findByRole('tab', { name: 'History' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await waitFor(() => expect(ResourceActionsAPI.listActions).toHaveBeenLastCalledWith('settled'));

    fireEvent.click(screen.getByRole('tab', { name: 'Open' }));
    await waitFor(() => expect(window.location.pathname).toBe('/actions'));
    await waitFor(() => expect(ResourceActionsAPI.listActions).toHaveBeenLastCalledWith('pending'));
  });
});
