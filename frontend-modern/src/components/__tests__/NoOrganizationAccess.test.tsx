import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { NoOrganizationAccess } from '@/components/NoOrganizationAccess';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';

vi.mock('@/utils/logger', () => ({ logger: { warn: vi.fn(), debug: vi.fn() } }));

const fetchMock = vi.fn<typeof fetch>();
const invitations = [
  {
    orgId: 'client/east',
    orgDisplayName: 'East estate',
    userId: 'invited-user',
    role: 'viewer',
    invitedAt: '2026-11-03T00:00:00Z',
    invitedBy: 'administrator',
  },
  {
    orgId: 'client/west',
    orgDisplayName: 'West estate',
    userId: 'invited-user',
    role: 'editor',
    invitedAt: '2026-11-03T00:00:00Z',
    invitedBy: 'administrator',
  },
];
const jsonResponse = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const row = (name: string) => within(screen.getByText(name).closest('li')!);
const renderRecovery = () => {
  const onReload = vi.fn();
  const onSignOut = vi.fn();
  render(() => <NoOrganizationAccess onReload={onReload} onSignOut={onSignOut} />);
  return { onReload, onSignOut };
};
const expectRequest = (index: number, path: string, method: string) => {
  const [url, options] = fetchMock.mock.calls[index];
  expect(url).toBe(path);
  expect(options?.method ?? 'GET').toBe(method);
  expect(options?.credentials).toBe('include');
  expect(options?.headers).toMatchObject({
    'X-Pulse-Org-ID': 'default',
    'X-Requested-With': 'XMLHttpRequest',
  });
  expect(options?.headers).not.toHaveProperty('X-API-Token');
  if (method !== 'GET') {
    expect(options?.headers).toHaveProperty('X-CSRF-Token', 'synthetic-csrf');
  }
};

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockRejectedValue(new Error('Unexpected request'));
  vi.stubGlobal('fetch', fetchMock);
  window.sessionStorage.clear();
  window.localStorage.clear();
  clearAuth();
  // Neither this unusable selection nor an independent saved API token should
  // replace the signed-in user's chosen personal invitation target.
  setOrgID('unusable-organization');
  setApiToken('synthetic-independent-token');
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
});

afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
});

describe('no-organization invitation recovery', () => {
  it('keeps recovery exits available while reading without inventing an empty list or changing access', async () => {
    const read = deferred<Response>();
    fetchMock.mockReturnValueOnce(read.promise);
    const { onReload, onSignOut } = renderRecovery();

    expect(screen.getByRole('main')).toHaveAccessibleName('No organization access yet');
    expect(screen.getByRole('status')).toHaveTextContent('Checking invitations');
    expect(screen.getByRole('button', { name: 'Check again' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeEnabled();
    expect(
      screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
    ).toBeNull();
    expect(screen.queryByRole('button', { name: 'Accept' })).toBeNull();
    expectRequest(0, '/api/org-invitations', 'GET');

    read.resolve(jsonResponse(invitations));
    expect(await screen.findByText('East estate')).toBeVisible();
    expect(screen.getByText('West estate')).toBeVisible();
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(onReload).not.toHaveBeenCalled();
    expect(onSignOut).not.toHaveBeenCalled();
  });

  it('accepts only the chosen invitation and waits for server acknowledgement before reloading', async () => {
    const accept = deferred<Response>();
    fetchMock.mockResolvedValueOnce(jsonResponse(invitations)).mockReturnValueOnce(accept.promise);
    const { onReload } = renderRecovery();
    await screen.findByText('East estate');

    fireEvent.click(row('East estate').getByRole('button', { name: 'Accept' }));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expectRequest(1, '/api/org-invitations/client%2Feast/accept', 'POST');
    expect(onReload).not.toHaveBeenCalled();
    for (const button of screen.getAllByRole('button', { name: /^(Accept|Decline)$/ })) {
      expect(button).toBeDisabled();
    }
    fireEvent.click(row('West estate').getByRole('button', { name: 'Accept' }));
    fireEvent.click(row('East estate').getByRole('button', { name: 'Decline' }));
    expect(fetchMock).toHaveBeenCalledTimes(2);

    accept.resolve(jsonResponse({ kind: 'member' }));
    await waitFor(() => expect(onReload).toHaveBeenCalledOnce());
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it.each(['HTTP failure', 'lost response'] as const)(
    'retains the invitation after an unconfirmed accept (%s), with no success reload or automatic replay',
    async (failure) => {
      fetchMock.mockResolvedValueOnce(jsonResponse(invitations));
      if (failure === 'HTTP failure') {
        fetchMock.mockResolvedValueOnce(jsonResponse({ error: 'synthetic private detail' }, 500));
      } else {
        fetchMock.mockRejectedValueOnce(new Error('synthetic private detail'));
      }
      const { onReload } = renderRecovery();
      await screen.findByText('East estate');

      fireEvent.click(row('East estate').getByRole('button', { name: 'Accept' }));
      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Could not accept the invitation.',
      );
      expect(screen.getByText('East estate')).toBeVisible();
      expect(screen.queryByText(/synthetic private detail/)).toBeNull();
      expect(row('East estate').getByRole('button', { name: 'Accept' })).toBeEnabled();
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(onReload).not.toHaveBeenCalled();
    },
  );

  it('retries an unconfirmed accept only on another explicit click and clears the old error', async () => {
    const retry = deferred<Response>();
    fetchMock
      .mockResolvedValueOnce(jsonResponse(invitations))
      .mockResolvedValueOnce(jsonResponse({ error: 'unavailable' }, 500))
      .mockReturnValueOnce(retry.promise);
    const { onReload } = renderRecovery();
    await screen.findByText('East estate');

    fireEvent.click(row('East estate').getByRole('button', { name: 'Accept' }));
    await screen.findByRole('alert');
    expect(fetchMock).toHaveBeenCalledTimes(2);
    fireEvent.click(row('East estate').getByRole('button', { name: 'Accept' }));
    expect(screen.queryByRole('alert')).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expectRequest(2, '/api/org-invitations/client%2Feast/accept', 'POST');
    expect(onReload).not.toHaveBeenCalled();

    retry.resolve(jsonResponse({ kind: 'member' }));
    await waitFor(() => expect(onReload).toHaveBeenCalledOnce());
  });

  it('declines only the chosen invitation and waits for a fresh list without reloading the app', async () => {
    const decline = deferred<Response>();
    const refresh = deferred<Response>();
    fetchMock
      .mockResolvedValueOnce(jsonResponse(invitations))
      .mockReturnValueOnce(decline.promise)
      .mockReturnValueOnce(refresh.promise);
    const { onReload } = renderRecovery();
    await screen.findByText('West estate');

    fireEvent.click(row('West estate').getByRole('button', { name: 'Decline' }));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expectRequest(1, '/api/org-invitations/client%2Fwest', 'DELETE');
    decline.resolve(new Response(null, { status: 204 }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    expectRequest(2, '/api/org-invitations', 'GET');
    expect(onReload).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: 'Accept' })).toBeNull();
    expect(
      screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
    ).toBeNull();

    refresh.resolve(jsonResponse([invitations[0]]));
    expect(await screen.findByText('East estate')).toBeVisible();
    expect(screen.queryByText('West estate')).toBeNull();
    expect(row('East estate').getByRole('button', { name: 'Accept' })).toBeEnabled();
    expect(onReload).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it('keeps a failed decline distinct from success without replaying, refetching or exposing raw errors', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(invitations))
      .mockResolvedValueOnce(jsonResponse({ error: 'synthetic private detail' }, 500));
    const { onReload } = renderRecovery();
    await screen.findByText('West estate');

    fireEvent.click(row('West estate').getByRole('button', { name: 'Decline' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not decline the invitation.');
    expect(screen.getByText('West estate')).toBeVisible();
    expect(screen.queryByText(/synthetic private detail/)).toBeNull();
    expect(row('West estate').getByRole('button', { name: 'Decline' })).toBeEnabled();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expectRequest(1, '/api/org-invitations/client%2Fwest', 'DELETE');
    expect(onReload).not.toHaveBeenCalled();
  });

  it('keeps explicit check-again and sign-out exits separate from invitation changes', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([]));
    const { onReload, onSignOut } = renderRecovery();
    await screen.findByText('Ask a Pulse administrator to add you to an organization.');

    fireEvent.click(screen.getByRole('button', { name: 'Check again' }));
    expect(onReload).toHaveBeenCalledOnce();
    expect(onSignOut).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    expect(onSignOut).toHaveBeenCalledOnce();
    expect(onReload).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it.each(['HTTP failure', 'lost response', 'invalid list'] as const)(
    'keeps an unread invitation list distinct from no invitations (%s)',
    async (failure) => {
      if (failure === 'HTTP failure') {
        fetchMock.mockResolvedValueOnce(jsonResponse({ error: 'synthetic private detail' }, 500));
      } else if (failure === 'lost response') {
        fetchMock.mockRejectedValueOnce(new Error('synthetic private detail'));
      } else {
        fetchMock.mockResolvedValueOnce(jsonResponse({ invitations }));
      }
      const { onReload, onSignOut } = renderRecovery();

      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Could not check your invitations.',
      );
      expect(screen.queryByText(/synthetic private detail/)).toBeNull();
      expect(
        screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
      ).toBeNull();
      expect(screen.queryByRole('button', { name: 'Accept' })).toBeNull();
      expect(screen.getByRole('button', { name: 'Retry invitations' })).toBeEnabled();
      expect(screen.getByRole('button', { name: 'Check again' })).toBeEnabled();
      expect(screen.getByRole('button', { name: 'Sign out' })).toBeEnabled();
      expectRequest(0, '/api/org-invitations', 'GET');
      expect(fetchMock).toHaveBeenCalledOnce();
      expect(onReload).not.toHaveBeenCalled();
      expect(onSignOut).not.toHaveBeenCalled();
      expect(getOrgID()).toBe('unusable-organization');
      expect(getApiToken()).toBe('synthetic-independent-token');
    },
  );

  it.each(['invitations', 'confirmed empty'] as const)(
    'recovers from an unread list to %s only through an explicit read-only retry',
    async (result) => {
      const read = deferred<Response>();
      fetchMock
        .mockResolvedValueOnce(jsonResponse({ error: 'unavailable' }, 500))
        .mockReturnValueOnce(read.promise);
      const { onReload, onSignOut } = renderRecovery();
      await screen.findByRole('alert');
      expect(fetchMock).toHaveBeenCalledOnce();

      fireEvent.click(screen.getByRole('button', { name: 'Retry invitations' }));
      expect(screen.getByRole('status')).toHaveTextContent('Checking invitations');
      expect(screen.queryByRole('alert')).toBeNull();
      expect(screen.queryByRole('button', { name: 'Retry invitations' })).toBeNull();
      expect(screen.queryByRole('button', { name: 'Accept' })).toBeNull();
      expect(
        screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
      ).toBeNull();
      expect(screen.getByRole('button', { name: 'Check again' })).toBeEnabled();
      expect(screen.getByRole('button', { name: 'Sign out' })).toBeEnabled();
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expectRequest(1, '/api/org-invitations', 'GET');

      read.resolve(jsonResponse(result === 'invitations' ? invitations : []));
      if (result === 'invitations') {
        expect(await screen.findByText('East estate')).toBeVisible();
        expect(screen.getByText('West estate')).toBeVisible();
        expect(row('East estate').getByRole('button', { name: 'Accept' })).toBeEnabled();
        expect(row('West estate').getByRole('button', { name: 'Decline' })).toBeEnabled();
        expect(
          screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
        ).toBeNull();
      } else {
        expect(
          await screen.findByText('Ask a Pulse administrator to add you to an organization.'),
        ).toBeVisible();
        expect(screen.queryByRole('button', { name: 'Accept' })).toBeNull();
      }
      expect(screen.queryByRole('alert')).toBeNull();
      expect(screen.queryByRole('status')).toBeNull();
      expect(screen.queryByRole('button', { name: 'Retry invitations' })).toBeNull();
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(onReload).not.toHaveBeenCalled();
      expect(onSignOut).not.toHaveBeenCalled();
      expect(getOrgID()).toBe('unusable-organization');
      expect(getApiToken()).toBe('synthetic-independent-token');
    },
  );

  it('keeps a failed retry unavailable without automatically repeating reads or invitation changes', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'unavailable' }, 500))
      .mockResolvedValueOnce(jsonResponse(null));
    const { onReload } = renderRecovery();
    await screen.findByRole('alert');

    fireEvent.click(screen.getByRole('button', { name: 'Retry invitations' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not check your invitations.');
    expect(screen.getByRole('button', { name: 'Retry invitations' })).toBeEnabled();
    expect(
      screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
    ).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expectRequest(0, '/api/org-invitations', 'GET');
    expectRequest(1, '/api/org-invitations', 'GET');
    expect(onReload).not.toHaveBeenCalled();
  });

  it('distinguishes a confirmed decline from its unread refreshed list and never replays the decline on retry', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(invitations))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(jsonResponse({ error: 'synthetic private detail' }, 500))
      .mockResolvedValueOnce(jsonResponse([invitations[0]]));
    const { onReload } = renderRecovery();
    await screen.findByText('West estate');

    fireEvent.click(row('West estate').getByRole('button', { name: 'Decline' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not check your invitations.');
    expect(screen.queryByText('Could not decline the invitation.')).toBeNull();
    expect(
      screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
    ).toBeNull();
    expect(screen.queryByRole('button', { name: 'Accept' })).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expectRequest(1, '/api/org-invitations/client%2Fwest', 'DELETE');
    expectRequest(2, '/api/org-invitations', 'GET');

    fireEvent.click(screen.getByRole('button', { name: 'Retry invitations' }));
    expect(await screen.findByText('East estate')).toBeVisible();
    expect(screen.queryByText('West estate')).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(4);
    expectRequest(3, '/api/org-invitations', 'GET');
    expect(onReload).not.toHaveBeenCalled();
  });
});
