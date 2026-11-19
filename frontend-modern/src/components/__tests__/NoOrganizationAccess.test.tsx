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
  it('distinguishes same-named invitations and sends only the chosen exact ID', async () => {
    const sameNames = invitations.map((invitation) => ({
      ...invitation,
      orgDisplayName: 'Operations',
    }));
    fetchMock
      .mockResolvedValueOnce(jsonResponse(sameNames))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(jsonResponse([sameNames[0]]))
      .mockResolvedValueOnce(jsonResponse({ kind: 'member' }));
    const { onReload } = renderRecovery();
    await screen.findAllByText('Operations');

    for (const invitation of sameNames) {
      const group = screen.getByRole('group', {
        name: `Invitation to Operations (${invitation.orgId})`,
      });
      expect(within(group).getByText('Operations')).toBeVisible();
      expect(within(group).getByText(invitation.orgId)).toBeVisible();
      expect(within(group).getByText(invitation.role)).toBeVisible();
      expect(
        within(group).getByRole('button', {
          name: `Accept invitation to Operations (${invitation.orgId})`,
        }),
      ).toBeEnabled();
      expect(
        within(group).getByRole('button', {
          name: `Decline invitation to Operations (${invitation.orgId})`,
        }),
      ).toBeEnabled();
    }
    expect(fetchMock).toHaveBeenCalledOnce();
    fireEvent.click(
      screen.getByRole('button', { name: 'Decline invitation to Operations (client/west)' }),
    );
    await waitFor(() => expect(screen.getAllByRole('group')).toHaveLength(1));
    expectRequest(1, '/api/org-invitations/client%2Fwest', 'DELETE');
    expectRequest(2, '/api/org-invitations', 'GET');
    expect(screen.getByText('client/east')).toBeVisible();
    expect(screen.queryByText('client/west')).toBeNull();
    expect(onReload).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole('button', { name: 'Accept invitation to Operations (client/east)' }),
    );
    await waitFor(() => expect(onReload).toHaveBeenCalledOnce());
    expectRequest(3, '/api/org-invitations/client%2Feast/accept', 'POST');
    expect(fetchMock).toHaveBeenCalledTimes(4);
    expect(getOrgID()).toBe('unusable-organization');
    expect(getApiToken()).toBe('synthetic-independent-token');
  });

  it.each([undefined, '', 'client/east'])(
    'shows an unnamed or ID-named invitation once without an invented party (%j)',
    async (orgDisplayName) => {
      fetchMock.mockResolvedValueOnce(jsonResponse([{ ...invitations[0], orgDisplayName }]));
      renderRecovery();
      const group = await screen.findByRole('group', { name: 'Invitation to client/east' });
      expect(within(group).getAllByText('client/east')).toHaveLength(1);
      expect(within(group).getByText('viewer')).toBeVisible();
      expect(
        within(group).getByRole('button', { name: 'Accept invitation to client/east' }),
      ).toBeEnabled();
      expect(
        within(group).getByRole('button', { name: 'Decline invitation to client/east' }),
      ).toBeEnabled();
      expect(fetchMock).toHaveBeenCalledOnce();
    },
  );

  it('keeps long, reserved and markup-looking identities literal and sends their exact encoded key', async () => {
    const orgId = 'client/research-network?workspace=shared-services#west';
    const orgDisplayName = '<img src=x onerror=alert(1)> Operations / مركز الأبحاث';
    fetchMock
      .mockResolvedValueOnce(jsonResponse([{ ...invitations[0], orgId, orgDisplayName }]))
      .mockResolvedValueOnce(jsonResponse({ kind: 'member' }));
    const { onReload } = renderRecovery();
    const group = await screen.findByRole('group', {
      name: `Invitation to ${orgDisplayName} (${orgId})`,
    });
    expect(within(group).getByText(orgDisplayName).tagName).toBe('BDI');
    expect(within(group).getByText(orgId).tagName).toBe('BDI');
    expect(group.querySelector('img')).toBeNull();
    expect(group.closest('li')).not.toBeNull();
    fireEvent.click(
      within(group).getByRole('button', {
        name: `Accept invitation to ${orgDisplayName} (${orgId})`,
      }),
    );
    await waitFor(() => expect(onReload).toHaveBeenCalledOnce());
    expectRequest(1, `/api/org-invitations/${encodeURIComponent(orgId)}/accept`, 'POST');
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

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
    expect(screen.queryByRole('button', { name: /^Accept invitation to / })).toBeNull();
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

    fireEvent.click(row('East estate').getByRole('button', { name: /^Accept invitation to / }));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expectRequest(1, '/api/org-invitations/client%2Feast/accept', 'POST');
    expect(onReload).not.toHaveBeenCalled();
    for (const button of screen.getAllByRole('button', {
      name: /^(Accept|Decline) invitation to /,
    })) {
      expect(button).toBeDisabled();
    }
    fireEvent.click(row('West estate').getByRole('button', { name: /^Accept invitation to / }));
    fireEvent.click(row('East estate').getByRole('button', { name: /^Decline invitation to / }));
    expect(fetchMock).toHaveBeenCalledTimes(2);

    accept.resolve(jsonResponse({ kind: 'member' }));
    await waitFor(() => expect(onReload).toHaveBeenCalledOnce());
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it.each([
    ['Accept', 'HTTP failure'],
    ['Accept', 'lost response'],
    ['Decline', 'HTTP failure'],
    ['Decline', 'lost response'],
  ] as const)(
    'checks access instead of repeating an unconfirmed %s (%s)',
    async (action, failure) => {
      const accept = action === 'Accept';
      const sameNames = invitations.map((invitation) => ({
        ...invitation,
        orgDisplayName: 'Operations',
      }));
      const target = sameNames[accept ? 0 : 1];
      fetchMock.mockResolvedValueOnce(jsonResponse(sameNames));
      if (failure === 'HTTP failure') {
        fetchMock.mockResolvedValueOnce(jsonResponse({ error: 'synthetic private detail' }, 500));
      } else {
        fetchMock.mockRejectedValueOnce(new Error('synthetic private detail'));
      }
      const { onReload, onSignOut } = renderRecovery();
      const decision = await screen.findByRole('button', {
        name: `${action} invitation to Operations (${target.orgId})`,
      });

      decision.focus();
      fireEvent.click(decision);
      expect(await screen.findByRole('alert')).toHaveTextContent(
        `Pulse could not confirm your decision to ${action.toLowerCase()} the invitation to Operations (${target.orgId}).`,
      );
      expect(screen.getByRole('main')).toHaveAccessibleName('Check organization access');
      expect(screen.getByRole('alert')).toHaveTextContent(
        'The list below is from before your decision.',
      );
      expect(screen.getByRole('region', { name: 'Organization invitations' })).toHaveFocus();
      expect(screen.queryByText(/Ask a Pulse administrator/)).toBeNull();
      expect(screen.queryByText(/synthetic private detail/)).toBeNull();
      for (const invitation of sameNames) {
        const group = screen.getByRole('group', {
          name: `Invitation to Operations (${invitation.orgId})`,
        });
        expect(within(group).getByText(invitation.orgId)).toBeVisible();
        expect(within(group).getByText(invitation.role)).toBeVisible();
        for (const button of within(group).getAllByRole('button')) {
          expect(button).toBeDisabled();
          fireEvent.click(button);
        }
      }
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expectRequest(
        1,
        `/api/org-invitations/${encodeURIComponent(target.orgId)}${accept ? '/accept' : ''}`,
        accept ? 'POST' : 'DELETE',
      );
      expect(onReload).not.toHaveBeenCalled();
      expect(screen.getByRole('button', { name: 'Sign out' })).toBeEnabled();
      const recovery = screen.getByRole('button', { name: 'Check again' });
      expect(recovery).toBeEnabled();
      fireEvent.click(recovery);
      expect(onReload).toHaveBeenCalledOnce();
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(onSignOut).not.toHaveBeenCalled();
      expect(getOrgID()).toBe('unusable-organization');
      expect(getApiToken()).toBe('synthetic-independent-token');
    },
  );

  it('does not take focus back from Sign out on a late unconfirmed decision', async () => {
    const decision = deferred<Response>();
    fetchMock
      .mockResolvedValueOnce(jsonResponse(invitations))
      .mockReturnValueOnce(decision.promise);
    const { onReload } = renderRecovery();
    await screen.findByText('East estate');

    const accept = row('East estate').getByRole('button', { name: /^Accept invitation to / });
    accept.focus();
    fireEvent.click(accept);
    const exit = screen.getByRole('button', { name: 'Sign out' });
    exit.focus();

    decision.reject(new Error('Response not received'));
    await screen.findByRole('alert');
    expect(exit).toHaveFocus();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(onReload).not.toHaveBeenCalled();
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

    fireEvent.click(row('West estate').getByRole('button', { name: /^Decline invitation to / }));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expectRequest(1, '/api/org-invitations/client%2Fwest', 'DELETE');
    decline.resolve(new Response(null, { status: 204 }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    expectRequest(2, '/api/org-invitations', 'GET');
    expect(onReload).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /^Accept invitation to / })).toBeNull();
    expect(
      screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
    ).toBeNull();

    refresh.resolve(jsonResponse([invitations[0]]));
    expect(await screen.findByText('East estate')).toBeVisible();
    expect(screen.queryByText('West estate')).toBeNull();
    expect(
      row('East estate').getByRole('button', { name: /^Accept invitation to / }),
    ).toBeEnabled();
    expect(onReload).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(3);
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
      expect(screen.queryByRole('button', { name: /^Accept invitation to / })).toBeNull();
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
      expect(screen.queryByRole('button', { name: /^Accept invitation to / })).toBeNull();
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
        expect(
          row('East estate').getByRole('button', { name: /^Accept invitation to / }),
        ).toBeEnabled();
        expect(
          row('West estate').getByRole('button', { name: /^Decline invitation to / }),
        ).toBeEnabled();
        expect(
          screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
        ).toBeNull();
      } else {
        expect(
          await screen.findByText('Ask a Pulse administrator to add you to an organization.'),
        ).toBeVisible();
        expect(screen.queryByRole('button', { name: /^Accept invitation to / })).toBeNull();
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

    fireEvent.click(row('West estate').getByRole('button', { name: /^Decline invitation to / }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not check your invitations.');
    expect(screen.getByRole('main')).toHaveAccessibleName('No organization access yet');
    expect(
      screen.queryByText('Ask a Pulse administrator to add you to an organization.'),
    ).toBeNull();
    expect(screen.queryByRole('button', { name: /^Accept invitation to / })).toBeNull();
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
