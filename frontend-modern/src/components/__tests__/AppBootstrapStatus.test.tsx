import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { APP_BOOTSTRAP_SLOW_DELAY_MS, AppBootstrapStatus } from '@/components/AppBootstrapStatus';

const orgsApi = vi.hoisted(() => ({
  listMyInvitations: vi.fn(),
  acceptMyInvitation: vi.fn(),
  declineMyInvitation: vi.fn(),
}));
const client = vi.hoisted(() => ({ apiFetch: vi.fn(), clearAuth: vi.fn() }));

vi.mock('@/api/orgs', () => ({ OrgsAPI: orgsApi }));
vi.mock('@/utils/apiClient', () => client);

const jsonResponse = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

const refuseEveryOrganization = () =>
  client.apiFetch.mockImplementation(async (url: string) => {
    if (url === '/api/state/summary') {
      return jsonResponse(403, {
        error: 'default_org_access_denied',
        message: 'User is not a member of the default organization',
      });
    }
    if (url === '/api/security/status') return jsonResponse(200, {});
    return jsonResponse(200, { success: true });
  });

describe('AppBootstrapStatus', () => {
  beforeEach(() => {
    client.apiFetch.mockResolvedValue(new Response('unavailable', { status: 503 }));
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('offers pending invitations to a signed-in user with no organization', async () => {
    refuseEveryOrganization();
    orgsApi.listMyInvitations.mockResolvedValue([
      {
        orgId: 'acme',
        orgDisplayName: 'Acme',
        userId: 'bob',
        role: 'viewer',
        invitedAt: '2026-11-03T00:00:00Z',
        invitedBy: 'alice',
      },
    ]);
    render(() => <AppBootstrapStatus failed />);

    expect(await screen.findByText('Acme')).toBeVisible();
    expect(screen.getByRole('main')).toHaveAccessibleName('No organization access yet');
    expect(screen.queryByRole('button', { name: 'Retry connection' })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    await waitFor(() =>
      expect(client.apiFetch).toHaveBeenCalledWith(
        '/api/logout',
        expect.objectContaining({ method: 'POST' }),
      ),
    );
  });

  it('lets its fallback probe recover stale default before claiming no organization access', async () => {
    client.apiFetch.mockImplementation(
      async (url: string, options?: { recoverDefaultOrg?: boolean }) => {
        if (url === '/api/state/summary' && options?.recoverDefaultOrg)
          return jsonResponse(200, {});
        return jsonResponse(403, { error: 'default_org_access_denied' });
      },
    );
    render(() => <AppBootstrapStatus failed />);
    await waitFor(() =>
      expect(client.apiFetch).toHaveBeenCalledWith(
        '/api/state/summary',
        expect.objectContaining({ recoverDefaultOrg: true }),
      ),
    );
    expect(screen.getByRole('main')).toHaveAccessibleName('Could not check your session');
    expect(screen.queryByText('No organization access yet')).toBeNull();
    expect(orgsApi.listMyInvitations).not.toHaveBeenCalled();
    expect(client.clearAuth).not.toHaveBeenCalled();
  });

  it('tells a user without invitations who can grant access', async () => {
    refuseEveryOrganization();
    orgsApi.listMyInvitations.mockResolvedValue([]);
    render(() => <AppBootstrapStatus failed />);

    expect(
      await screen.findByText('Ask a Pulse administrator to add you to an organization.'),
    ).toBeVisible();
  });

  it('does not probe organization access while the bootstrap is still running', () => {
    render(() => <AppBootstrapStatus />);
    expect(client.apiFetch).not.toHaveBeenCalled();
  });

  it('names and announces the initial connection work', () => {
    render(() => <AppBootstrapStatus />);

    expect(screen.getByRole('main')).toHaveAccessibleName('Connecting to Pulse');
    expect(screen.getByRole('status')).toHaveTextContent(
      'Checking your session and preparing the workspace.',
    );
    expect(screen.queryByRole('button', { name: 'Retry connection' })).toBeNull();
  });

  it('keeps waiting while making a slow bootstrap understandable and retryable', async () => {
    vi.useFakeTimers();
    const onRetry = vi.fn();
    render(() => <AppBootstrapStatus onRetry={onRetry} />);

    await vi.advanceTimersByTimeAsync(APP_BOOTSTRAP_SLOW_DELAY_MS);

    expect(screen.getByRole('status')).toHaveTextContent(
      'Pulse is taking longer than expected to respond.',
    );
    expect(screen.getByText('You can keep waiting, or retry the connection.')).toBeVisible();

    const retry = screen.getByRole('button', { name: 'Retry connection' });
    fireEvent.click(retry);
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('announces a failed session check immediately without suggesting it is still running', () => {
    const onRetry = vi.fn();
    render(() => <AppBootstrapStatus failed onRetry={onRetry} />);

    expect(screen.getByRole('main')).toHaveAccessibleName('Could not check your session');
    expect(screen.getByRole('status')).toHaveTextContent(
      'Your saved sign-in details have not been cleared.',
    );
    expect(screen.queryByText(/You can keep waiting/)).toBeNull();
    expect(screen.queryByText(/Checking your session and preparing/)).toBeNull();
    const retry = screen.getByRole('button', { name: 'Retry connection' });
    expect(retry).toBeVisible();
    fireEvent.click(retry);
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('cancels a completed bootstrap timer and gives a fresh attempt its full waiting period', async () => {
    vi.useFakeTimers();
    const first = render(() => <AppBootstrapStatus />);
    await vi.advanceTimersByTimeAsync(APP_BOOTSTRAP_SLOW_DELAY_MS - 1);
    expect(screen.queryByRole('button', { name: 'Retry connection' })).toBeNull();

    first.unmount();
    expect(vi.getTimerCount()).toBe(0);

    render(() => <AppBootstrapStatus />);
    await vi.advanceTimersByTimeAsync(1);
    expect(screen.getByRole('status')).toHaveTextContent(
      'Checking your session and preparing the workspace.',
    );
    expect(screen.queryByRole('button', { name: 'Retry connection' })).toBeNull();

    await vi.advanceTimersByTimeAsync(APP_BOOTSTRAP_SLOW_DELAY_MS - 1);
    expect(screen.getByRole('button', { name: 'Retry connection' })).toBeVisible();
  });
});
