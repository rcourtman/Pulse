import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { APITokenRecord } from '@/api/security';
import type { Resource } from '@/types/resource';
import { notificationStore } from '@/stores/notifications';
import { dismissTokenReveal, useTokenRevealState } from '@/stores/tokenReveal';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { APITokenManager } from '../APITokenManager';

// Keep SecurityAPI, apiClient, the state model and the component real. These
// tests cover their composition; the adjacent unit tests isolate each layer.
const environment = vi.hoisted(() => ({
  markAgents: vi.fn(),
  markRuntimes: vi.fn(),
}));
vi.mock('@/api/agentCapabilities', () => ({
  fetchAgentCapabilitiesManifest: () => Promise.resolve({ requiredScopes: [] }),
}));
vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    markAgentsTokenRevoked: environment.markAgents,
    markDockerRuntimesTokenRevoked: environment.markRuntimes,
  }),
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => resources,
    byType: (type: string) => resources.filter((resource) => resource.type === type),
  }),
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), debug: vi.fn(), warn: vi.fn() },
}));

const original: APITokenRecord = {
  id: 'token/one',
  name: 'Saved collector',
  prefix: 'test',
  suffix: '1234',
  createdAt: '2026-11-01T00:00:00Z',
  scopes: ['monitoring:read'],
  ownerUserId: 'synthetic-user',
  expiresAt: '2027-01-01T00:00:00Z',
};
const created: APITokenRecord = { ...original, id: 'token/two', name: 'Unsent draft' };
const resources: Resource[] = (['agent', 'docker-host'] as const).map((type) => ({
  id: `${type}-one`,
  type,
  name: `${type} fixture`,
  platformId: `${type}-one`,
  platformType: type === 'agent' ? 'agent' : 'docker',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 1,
  tags: [],
  platformData: { tokenId: original.id },
}));
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const fetchMock = vi.fn<typeof fetch>();
const reveal = useTokenRevealState();
const onChanged = vi.fn();
const requests = () =>
  fetchMock.mock.calls.map(([url, options]) => [options?.method ?? 'GET', url]);
const expectNoRevocations = () => {
  expect(environment.markAgents).not.toHaveBeenCalled();
  expect(environment.markRuntimes).not.toHaveBeenCalled();
};
const expectUnknown = () => {
  expect(screen.getAllByText('Unknown')).toHaveLength(3);
  expect(screen.queryByText(/No tokens yet/)).not.toBeInTheDocument();
  expectNoRevocations();
};
const ready = async () => {
  render(() => (
    <APITokenManager canManage currentUser="synthetic-user" onTokensChanged={onChanged} />
  ));
  const card = await screen.findByRole('article', { name: `API token ${original.name}` });
  fireEvent.input(screen.getByLabelText('Token name'), { target: { value: 'Unsent draft' } });
  fireEvent.click(screen.getByRole('button', { name: 'Kiosk / Monitoring' }));
  return card;
};
type Operation = 'create' | 'scope' | 'rename' | 'revoke';
const confirm = async (operation: Operation, card: HTMLElement) => {
  if (operation === 'create') {
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));
    return;
  }
  const action = { scope: 'Edit scopes', rename: 'Rename', revoke: 'Revoke' }[operation];
  fireEvent.click(within(card).getByRole('button', { name: action }));
  const dialog = await screen.findByRole('dialog');
  if (operation === 'scope') {
    fireEvent.click(within(dialog).getByRole('checkbox', { name: /Settings \(read\)/ }));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save scopes' }));
  } else if (operation === 'rename') {
    fireEvent.input(within(dialog).getByLabelText('Token name'), {
      target: { value: 'Current name' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }));
  } else {
    fireEvent.click(within(dialog).getByRole('button', { name: 'Revoke token' }));
  }
};
beforeEach(() => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  clearAuth();
  sessionStorage.clear();
  setOrgID('org-a');
  setApiToken('synthetic-only-auth');
  document.cookie = 'pulse_csrf=synthetic-only-csrf; Path=/';
  dismissTokenReveal();
});
afterEach(() => {
  cleanup();
  dismissTokenReveal();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
});

describe('API token recovery through the real HTTP client', () => {
  it.each(
    (['create', 'scope', 'rename', 'revoke'] as const).flatMap((operation) =>
      [true, false].map((applied) => ({ operation, applied })),
    ),
  )(
    'reconciles $operation (server applied: $applied) by explicit read, never replay',
    async ({ operation, applied }) => {
      let inventory = [original];
      let reads = 0;
      const writePath =
        operation === 'create' ? '/api/security/tokens' : '/api/security/tokens/token%2Fone';
      const method = { create: 'POST', scope: 'PATCH', rename: 'PATCH', revoke: 'DELETE' }[
        operation
      ];
      const body = {
        create: { name: 'Unsent draft', scopes: ['monitoring:read'] },
        scope: { scopes: ['monitoring:read', 'settings:read'] },
        rename: { name: 'Current name' },
        revoke: undefined,
      }[operation];
      const expectedInventory = !applied
        ? [original]
        : operation === 'create'
          ? [original, created]
          : operation === 'revoke'
            ? []
            : [{ ...original, ...(body as Partial<APITokenRecord>) }];
      fetchMock.mockImplementation(async (_url, options) => {
        if ((options?.method ?? 'GET') === 'GET') {
          reads += 1;
          // The first recovery read is incomplete, not a confirmed empty list.
          return json({ tokens: reads === 2 ? [original, null] : inventory });
        }
        inventory = expectedInventory;
        // Exercise distinct real parsing/transport outcomes, not a rejected mock
        // of SecurityAPI. None of them establishes server-side rollback.
        if (operation === 'create')
          return new Response('{"token":"synthetic-only-value","record":', { status: 201 });
        if (operation === 'scope') {
          const response = json({ record: inventory[0] });
          vi.spyOn(response, 'text').mockRejectedValueOnce(
            new TypeError('Interrupted response body'),
          );
          return response;
        }
        if (operation === 'rename') throw new TypeError('Failed to fetch');
        return json({ error: 'synthetic upstream failure' }, 503);
      });

      const card = await ready();
      await confirm(operation, card);
      const check = await screen.findByRole('button', { name: 'Check token inventory' });
      expect(screen.getByRole('alert')).toHaveTextContent(
        'The change may already have been saved.',
      );
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expectUnknown();
      expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled();
      expect(requests()).toEqual([
        ['GET', '/api/security/tokens'],
        [method, writePath],
      ]);
      const write = fetchMock.mock.calls[1][1]!;
      expect(write.headers).toMatchObject({ 'X-CSRF-Token': 'synthetic-only-csrf' });
      expect(write.body ? JSON.parse(String(write.body)) : undefined).toEqual(body);

      fireEvent.click(check);
      await screen.findByRole('button', { name: 'Check token inventory' });
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
      expectUnknown();
      expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled();
      fireEvent.click(screen.getByRole('button', { name: 'Check token inventory' }));
      await waitFor(() => expect(screen.queryAllByText('Unknown')).toHaveLength(0));
      expect(requests()).toEqual([
        ['GET', '/api/security/tokens'],
        [method, writePath],
        ['GET', '/api/security/tokens'],
        ['GET', '/api/security/tokens'],
      ]);
      // Assert outside the fetch fixture: an assertion thrown inside fetch would
      // be caught by the model as another unconfirmed network response.
      for (const [, options] of fetchMock.mock.calls) {
        expect(options).toMatchObject({
          credentials: 'include',
          headers: { 'X-Pulse-Org-ID': 'org-a', 'X-API-Token': 'synthetic-only-auth' },
        });
        expect(options).not.toHaveProperty('expectedOrgID');
      }
      expect(screen.getByRole('button', { name: 'Generate' })).not.toBeDisabled();
      expect(screen.getByLabelText('Token name')).toHaveValue('Unsent draft');
      expect(screen.getByRole('button', { name: 'Kiosk / Monitoring' })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      expect(
        screen.queryByRole('button', { name: 'Check token inventory' }),
      ).not.toBeInTheDocument();
      expect(reveal()).toBeNull();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(onChanged).not.toHaveBeenCalled();
      expect(getOrgID()).toBe('org-a');
      if (applied && operation === 'revoke') {
        expect(screen.getByText(/No tokens yet/)).toBeInTheDocument();
        // A positively confirmed withdrawal must still reach linked consumers.
        expect(environment.markAgents).toHaveBeenCalledExactlyOnceWith(original.id, ['agent-one']);
        expect(environment.markRuntimes).toHaveBeenCalledExactlyOnceWith(original.id, [
          'docker-host-one',
        ]);
      } else {
        expectNoRevocations();
        expect(
          screen.getAllByRole('article').map((element) => element.dataset.apiTokenCard),
        ).toEqual(expectedInventory.map((record) => record.id));
        for (const record of expectedInventory) {
          const currentCard = screen.getByRole('article', { name: `API token ${record.name}` });
          expect(currentCard).toHaveAttribute('data-api-token-card', record.id);
          expect(within(currentCard).getByRole('button', { name: 'Revoke' })).not.toBeDisabled();
        }
        if (applied && operation === 'scope') {
          expect(
            within(screen.getByRole('article', { name: `API token ${original.name}` })).getByText(
              'Settings (read)',
            ),
          ).toBeInTheDocument();
        }
      }
    },
  );

  it('keeps a refused write refused when a CSRF response would otherwise invite replay', async () => {
    fetchMock
      .mockResolvedValueOnce(json({ tokens: [original] }))
      .mockResolvedValueOnce(
        new Response('{"error":"permission_denied"}', {
          status: 403,
          headers: { 'X-CSRF-Token': 'synthetic-refreshed-csrf' },
        }),
      );
    await confirm('rename', await ready());
    await waitFor(() => expect(notificationStore.error).toHaveBeenCalledTimes(1));
    expect(requests()).toEqual([
      ['GET', '/api/security/tokens'],
      ['PATCH', '/api/security/tokens/token%2Fone'],
    ]);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.queryAllByText('Unknown')).toHaveLength(0);
    expect(screen.getByRole('article', { name: `API token ${original.name}` })).toBeInTheDocument();
    expect(notificationStore.success).not.toHaveBeenCalled();
    expectNoRevocations();
  });

  it.each([
    { kind: 'missing list', response: {} },
    { kind: 'partial list', response: { tokens: [original, null] } },
    { kind: 'duplicate identity', response: { tokens: [original, original] } },
  ])('does not withdraw linked credentials after a $kind HTTP success', async ({ response }) => {
    fetchMock
      .mockResolvedValueOnce(json(response))
      .mockResolvedValueOnce(json({ tokens: [original] }));
    render(() => <APITokenManager canManage />);
    const retry = await screen.findByRole('button', { name: 'Retry token inventory' });
    expectUnknown();
    expect(requests()).toEqual([['GET', '/api/security/tokens']]);
    fireEvent.click(retry);
    await screen.findByRole('article', { name: `API token ${original.name}` });
    expect(requests()).toEqual([
      ['GET', '/api/security/tokens'],
      ['GET', '/api/security/tokens'],
    ]);
    expectNoRevocations();
  });
});
