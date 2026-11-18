import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { NoOrganizationAccess } from '@/components/NoOrganizationAccess';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';

vi.mock('@/utils/logger', () => ({ logger: { warn: vi.fn(), debug: vi.fn() } }));

const fetchMock = vi.fn<typeof fetch>();
const invitations = [
  { orgId: 'client/east', orgDisplayName: 'Operations', role: 'viewer' },
  { orgId: 'client/west', orgDisplayName: 'Operations', role: 'editor' },
];
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const deferred = () => {
  let resolve!: (value: Response) => void;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
const outcomes = ['saved invitations', 'confirmed empty', 'unavailable'] as const;
type Outcome = (typeof outcomes)[number];
const response = (outcome: Outcome) =>
  json(outcome === 'saved invitations' ? invitations : outcome === 'confirmed empty' ? [] : null);
const waitForOutcome = async (outcome: Outcome) => {
  if (outcome === 'saved invitations') {
    await screen.findByRole('group', { name: 'Invitation to Operations (client/east)' });
    expect(
      screen.getByRole('group', { name: 'Invitation to Operations (client/west)' }),
    ).toBeVisible();
  } else if (outcome === 'confirmed empty') {
    await screen.findByText('Ask a Pulse administrator to add you to an organization.');
  } else {
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not check your invitations.');
  }
};
const mount = () => {
  const onReload = vi.fn();
  const onSignOut = vi.fn();
  render(() => <NoOrganizationAccess onReload={onReload} onSignOut={onSignOut} />);
  return { onReload, onSignOut };
};

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockRejectedValue(new Error('Unexpected request'));
  vi.stubGlobal('fetch', fetchMock);
  window.sessionStorage.clear();
  window.localStorage.clear();
  clearAuth();
  setOrgID('unusable-organization');
  setApiToken('synthetic-independent-token');
});
afterEach(() => {
  cleanup();
  clearAuth();
  vi.unstubAllGlobals();
});

describe('personal invitation read keyboard recovery', () => {
  it.each(outcomes)('keeps the explicit retry local through %s', async (outcome) => {
    const read = deferred();
    fetchMock.mockResolvedValueOnce(json(null, 503)).mockReturnValueOnce(read.promise);
    const exits = mount();
    await screen.findByRole('alert');
    const retry = screen.getByRole('button', { name: 'Retry invitations' });
    retry.focus();
    fireEvent.click(retry);

    const region = screen.getByRole('region', { name: 'Organization invitations' });
    expect(region).toHaveFocus();
    expect(region).toHaveAttribute('aria-busy', 'true');
    expect(screen.getByRole('status')).toHaveTextContent('Checking invitations');
    expect(screen.queryByRole('button', { name: 'Retry invitations' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Check again' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeEnabled();

    read.resolve(response(outcome));
    await waitForOutcome(outcome);
    expect(region).toHaveFocus();
    expect(region).toHaveAttribute('aria-busy', 'false');
    expect(exits.onReload).not.toHaveBeenCalled();
    expect(exits.onSignOut).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    for (const [url, options] of fetchMock.mock.calls) {
      expect(url).toBe('/api/org-invitations');
      expect(options?.method ?? 'GET').toBe('GET');
      expect(options?.credentials).toBe('include');
      expect(options?.headers).toMatchObject({ 'X-Pulse-Org-ID': 'default' });
      expect(options?.headers).not.toHaveProperty('X-API-Token');
    }
  });

  it.each(outcomes)(
    'does not take focus back after a deliberate departure and %s',
    async (outcome) => {
      const read = deferred();
      fetchMock.mockResolvedValueOnce(json(null, 503)).mockReturnValueOnce(read.promise);
      mount();
      await screen.findByRole('alert');
      const retry = screen.getByRole('button', { name: 'Retry invitations' });
      retry.focus();
      fireEvent.click(retry);
      const exit = screen.getByRole('button', { name: 'Sign out' });
      exit.focus();

      read.resolve(response(outcome));
      await waitForOutcome(outcome);
      expect(exit).toHaveFocus();
      expect(fetchMock).toHaveBeenCalledTimes(2);
    },
  );

  it('does not autofocus when the initial background read finishes', async () => {
    const read = deferred();
    fetchMock.mockReturnValueOnce(read.promise);
    mount();
    const exit = screen.getByRole('button', { name: 'Sign out' });
    exit.focus();
    read.resolve(json(invitations));
    await screen.findByRole('group', { name: 'Invitation to Operations (client/east)' });
    expect(exit).toHaveFocus();
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it('reuses the stable region on another explicit read without starting a mutation', async () => {
    const read = deferred();
    fetchMock
      .mockResolvedValueOnce(json(null, 503))
      .mockResolvedValueOnce(json(null, 503))
      .mockReturnValueOnce(read.promise);
    mount();
    await screen.findByRole('alert');
    fireEvent.click(screen.getByRole('button', { name: 'Retry invitations' }));
    const region = screen.getByRole('region', { name: 'Organization invitations' });
    await waitFor(() => expect(region).toHaveAttribute('aria-busy', 'false'));
    expect(region).toHaveFocus();
    const retry = screen.getByRole('button', { name: 'Retry invitations' });
    retry.focus();
    fireEvent.click(retry);
    expect(region).toHaveFocus();
    expect(region).toHaveAttribute('aria-busy', 'true');

    read.resolve(json([]));
    await waitForOutcome('confirmed empty');
    expect(screen.getByRole('region', { name: 'Organization invitations' })).toBe(region);
    expect(region).toHaveFocus();
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(
      fetchMock.mock.calls.every(([, options]) => !options?.method || options.method === 'GET'),
    ).toBe(true);
  });
});
