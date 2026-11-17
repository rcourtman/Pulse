import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { APITokenRecord } from '@/api/security';
import type { Resource } from '@/types/resource';
import { eventBus } from '@/stores/events';
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
  displayName: `${type} fixture`,
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
  const card = await screen.findByRole('article', {
    name: `API token ${original.name} (${original.prefix}…${original.suffix})`,
  });
  fireEvent.input(screen.getByLabelText('Token name'), { target: { value: 'Unsent draft' } });
  fireEvent.click(screen.getByRole('button', { name: 'Kiosk / Monitoring' }));
  return card;
};
const deferredRead = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((finish) => {
    resolve = finish;
  });
  return { promise, resolve };
};
type Operation = 'create' | 'scope' | 'rename' | 'revoke';
const confirm = async (operation: Operation, card: HTMLElement) => {
  if (operation === 'create') {
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));
    return;
  }
  const action = { scope: 'Edit scopes', rename: 'Rename', revoke: 'Revoke' }[operation];
  fireEvent.click(within(card).getByRole('button', { name: new RegExp(`^${action} `) }));
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
      const recoveryRead = deferredRead();
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
          // Hold the first explicit recovery read so its pending focus is proved.
          // Its eventual partial list is not a confirmed empty inventory.
          return reads === 2 ? recoveryRead.promise : json({ tokens: inventory });
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

      check.focus();
      fireEvent.click(check);
      const inventoryRegion = screen.getByRole('region', { name: 'API token inventory' });
      expect(inventoryRegion).toHaveFocus();
      expect(inventoryRegion).toHaveAttribute('aria-busy', 'true');
      recoveryRead.resolve(json({ tokens: [original, null] }));
      await screen.findByRole('button', { name: 'Check token inventory' });
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
      expectUnknown();
      expect(inventoryRegion).toHaveFocus();
      expect(inventoryRegion).toHaveAttribute('aria-busy', 'false');
      expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled();
      const nextCheck = screen.getByRole('button', { name: 'Check token inventory' });
      nextCheck.focus();
      fireEvent.click(nextCheck);
      // Loading also replaces Unknown. Wait for the completed recovery, not
      // merely for the absence of the earlier unknown-state text.
      await waitFor(() =>
        expect(screen.getByRole('button', { name: 'Generate' })).not.toBeDisabled(),
      );
      expect(screen.queryAllByText('Unknown')).toHaveLength(0);
      expect(inventoryRegion).toHaveFocus();
      expect(inventoryRegion).toHaveAttribute('aria-busy', 'false');
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
          const currentCard = screen.getByRole('article', {
            name: `API token ${record.name} (${record.prefix}…${record.suffix})`,
          });
          expect(currentCard).toHaveAttribute('data-api-token-card', record.id);
          expect(within(currentCard).getByRole('button', { name: /^Revoke / })).not.toBeDisabled();
        }
        if (applied && operation === 'scope') {
          expect(
            within(
              screen.getByRole('article', {
                name: `API token ${original.name} (${original.prefix}…${original.suffix})`,
              }),
            ).getByText('Settings (read)'),
          ).toBeInTheDocument();
        }
      }
    },
  );

  it('keeps a refused write refused when a CSRF response would otherwise invite replay', async () => {
    fetchMock.mockResolvedValueOnce(json({ tokens: [original] })).mockResolvedValueOnce(
      new Response('{"error":"permission_denied"}', {
        status: 403,
        headers: { 'X-CSRF-Token': 'synthetic-refreshed-csrf' },
      }),
    );
    await confirm('rename', await ready());
    expect(await within(screen.getByRole('dialog')).findByRole('alert')).toHaveTextContent(
      'Unable to rename the API token.',
    );
    expect(notificationStore.error).not.toHaveBeenCalled();
    expect(requests()).toEqual([
      ['GET', '/api/security/tokens'],
      ['PATCH', '/api/security/tokens/token%2Fone'],
    ]);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.queryAllByText('Unknown')).toHaveLength(0);
    expect(
      screen.getByRole('article', {
        name: `API token ${original.name} (${original.prefix}…${original.suffix})`,
      }),
    ).toBeInTheDocument();
    expect(notificationStore.success).not.toHaveBeenCalled();
    expectNoRevocations();
  });

  it('does not replace a refused organisation with the server-selected default inventory', async () => {
    fetchMock
      .mockResolvedValueOnce(json({ error: 'invalid_org' }, 400))
      .mockResolvedValueOnce(json({ tokens: [original] }));
    render(() => <APITokenManager canManage />);
    const retry = await screen.findByRole('button', { name: 'Retry token inventory' });
    expectUnknown();
    expect(getOrgID()).toBe('org-a');
    expect(requests()).toEqual([['GET', '/api/security/tokens']]);
    fireEvent.click(retry);
    await screen.findByRole('article', {
      name: `API token ${original.name} (${original.prefix}…${original.suffix})`,
    });
    expect(requests()).toEqual([
      ['GET', '/api/security/tokens'],
      ['GET', '/api/security/tokens'],
    ]);
    for (const [, options] of fetchMock.mock.calls) {
      expect(options?.headers).toMatchObject({ 'X-Pulse-Org-ID': 'org-a' });
    }
    expect(getOrgID()).toBe('org-a');
    expectNoRevocations();
  });

  it('ignores a late HTTP inventory after the organisation has changed', async () => {
    let finishOldRead!: (response: Response) => void;
    const oldRead = new Promise<Response>((resolve) => {
      finishOldRead = resolve;
    });
    const oldResponse = json({ tokens: [original] });
    const oldText = vi.spyOn(oldResponse, 'text');
    const other = { ...original, id: 'token/other', name: 'Org B collector' };
    fetchMock.mockReturnValueOnce(oldRead).mockResolvedValueOnce(json({ tokens: [other] }));
    render(() => <APITokenManager canManage />);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    setOrgID('org-b');
    eventBus.emit('org_switched', 'org-b');
    await screen.findByRole('article', {
      name: `API token ${other.name} (${other.prefix}…${other.suffix})`,
    });
    expect(fetchMock.mock.calls[0][1]?.signal?.aborted).toBe(true);
    // Deliberately let the old fixture ignore abort: fetch cancellation alone
    // is not proof that a late decoded response cannot replace the new list.
    finishOldRead(oldResponse);
    await waitFor(() => expect(oldText).toHaveBeenCalledTimes(1));
    await oldText.mock.results[0].value;
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
    expect(
      screen.getByRole('article', {
        name: `API token ${other.name} (${other.prefix}…${other.suffix})`,
      }),
    ).toHaveAttribute('data-api-token-card', other.id);
    expect(
      screen.queryByRole('article', {
        name: `API token ${original.name} (${original.prefix}…${original.suffix})`,
      }),
    ).not.toBeInTheDocument();
    expect(requests()).toEqual([
      ['GET', '/api/security/tokens'],
      ['GET', '/api/security/tokens'],
    ]);
    expect(fetchMock.mock.calls[0][1]?.headers).toMatchObject({ 'X-Pulse-Org-ID': 'org-a' });
    expect(fetchMock.mock.calls[1][1]?.headers).toMatchObject({ 'X-Pulse-Org-ID': 'org-b' });
    expect(getOrgID()).toBe('org-b');
    expect(onChanged).not.toHaveBeenCalled();
    expect(reveal()).toBeNull();
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
    await screen.findByRole('article', {
      name: `API token ${original.name} (${original.prefix}…${original.suffix})`,
    });
    expect(requests()).toEqual([
      ['GET', '/api/security/tokens'],
      ['GET', '/api/security/tokens'],
    ]);
    expectNoRevocations();
  });
});

describe('API token inventory keyboard recovery', () => {
  const startRetry = async () => {
    const retry = await screen.findByRole('button', { name: 'Retry token inventory' });
    retry.focus();
    fireEvent.click(retry);
    const region = screen.getByRole('region', { name: 'API token inventory' });
    expect(region).toHaveFocus();
    expect(region).toHaveAttribute('aria-busy', 'true');
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    return region;
  };
  const expectScopedReads = () => {
    expect(requests()).toEqual([
      ['GET', '/api/security/tokens'],
      ['GET', '/api/security/tokens'],
    ]);
    for (const [, options] of fetchMock.mock.calls) {
      expect(options).toMatchObject({
        credentials: 'include',
        headers: { 'X-Pulse-Org-ID': 'org-a', 'X-API-Token': 'synthetic-only-auth' },
      });
    }
    expectNoRevocations();
    expect(onChanged).not.toHaveBeenCalled();
    expect(reveal()).toBeNull();
  };

  it.each(['saved', 'empty', 'failed'] as const)(
    'retains a named, non-tab-stop inventory while Retry settles as %s',
    async (result) => {
      const pending = deferredRead();
      fetchMock
        .mockResolvedValueOnce(json({ error: 'synthetic upstream detail' }, 503))
        .mockReturnValueOnce(pending.promise);
      render(() => <APITokenManager currentUser="synthetic-user" canManage />);
      const region = await startRetry();
      expect(region).toHaveAttribute('tabindex', '-1');
      pending.resolve(
        result === 'failed'
          ? json({ error: 'synthetic upstream detail' }, 503)
          : json({ tokens: result === 'empty' ? [] : [original] }),
      );
      await waitFor(() => expect(region).toHaveAttribute('aria-busy', 'false'));
      expect(region).toHaveFocus();
      if (result === 'saved')
        expect(
          screen.getByRole('article', {
            name: `API token ${original.name} (${original.prefix}…${original.suffix})`,
          }),
        ).toBeInTheDocument();
      else if (result === 'empty') expect(screen.getByText(/No tokens yet/)).toBeInTheDocument();
      else {
        expect(screen.getByRole('button', { name: 'Retry token inventory' })).not.toBeDisabled();
        expectUnknown();
      }
      expectScopedReads();
    },
  );

  it('does not move focus when the initial read fails', async () => {
    const pending = deferredRead();
    fetchMock.mockReturnValueOnce(pending.promise);
    render(() => (
      <>
        <button type="button">Outside task</button>
        <APITokenManager canManage />
      </>
    ));
    const outside = screen.getByRole('button', { name: 'Outside task' });
    outside.focus();
    pending.resolve(json({ error: 'synthetic upstream detail' }, 503));
    await screen.findByRole('button', { name: 'Retry token inventory' });
    expect(outside).toHaveFocus();
    expect(requests()).toEqual([['GET', '/api/security/tokens']]);
  });

  it('keeps read-only inventory recovery usable without granting management', async () => {
    const pending = deferredRead();
    fetchMock.mockResolvedValueOnce(json({}, 503)).mockReturnValueOnce(pending.promise);
    render(() => <APITokenManager canManage={false} />);
    const region = await startRetry();
    pending.resolve(json({ tokens: [original] }));
    await waitFor(() => expect(region).toHaveAttribute('aria-busy', 'false'));
    expect(region).toHaveFocus();
    expect(screen.getByRole('button', { name: 'New token' })).toBeDisabled();
    for (const revoke of screen.getAllByRole('button', { name: /^Revoke / }))
      expect(revoke).toBeDisabled();
    expectScopedReads();
  });

  it.each([true, false])(
    'never takes focus back after a deliberate move (success: %s)',
    async (success) => {
      const pending = deferredRead();
      fetchMock.mockResolvedValueOnce(json({}, 503)).mockReturnValueOnce(pending.promise);
      render(() => (
        <>
          <button type="button">Outside task</button>
          <APITokenManager canManage />
        </>
      ));
      const region = await startRetry();
      const outside = screen.getByRole('button', { name: 'Outside task' });
      outside.focus();
      pending.resolve(success ? json({ tokens: [original] }) : json({}, 503));
      await waitFor(() => expect(region).toHaveAttribute('aria-busy', 'false'));
      expect(outside).toHaveFocus();
      expectScopedReads();
    },
  );

  it.each(['organisation', 'principal', 'management', 'unmount'] as const)(
    'does not refocus a retired reader after %s changes',
    async (change) => {
      const pending = deferredRead();
      const lateResponse = json({ tokens: [original] });
      const readLateBody = vi.spyOn(lateResponse, 'text');
      const other = { ...original, id: 'other-token', name: 'Other reader credential' };
      fetchMock
        .mockResolvedValueOnce(json({}, 503))
        .mockReturnValueOnce(pending.promise)
        .mockResolvedValue(json({ tokens: [other] }));
      const [user, setUser] = createSignal('synthetic-user');
      const [manage, setManage] = createSignal(true);
      const view = render(() => (
        <>
          <button type="button">Outside task</button>
          <APITokenManager currentUser={user()} canManage={manage()} />
        </>
      ));
      await startRetry();
      const outside = screen.getByRole('button', { name: 'Outside task' });
      outside.focus();
      if (change === 'organisation') {
        setOrgID('org-b');
        eventBus.emit('org_switched', 'org-b');
      } else if (change === 'principal') setUser('another-user');
      else if (change === 'management') setManage(false);
      else view.unmount();
      if (change !== 'unmount')
        await screen.findByRole('article', {
          name: `API token ${other.name} (${other.prefix}…${other.suffix})`,
        });
      expect(fetchMock.mock.calls[1][1]?.signal?.aborted).toBe(true);
      const stableFocus = document.activeElement;
      // The fixture intentionally ignores abort; decode the old response fully
      // before asserting it cannot replace the new reader or move its focus.
      pending.resolve(lateResponse);
      await waitFor(() => expect(readLateBody).toHaveBeenCalledTimes(1));
      await readLateBody.mock.results[0].value;
      await new Promise<void>((resolve) => setTimeout(resolve, 0));
      expect(document.activeElement).toBe(stableFocus);
      expect(
        screen.queryByRole('article', {
          name: `API token ${original.name} (${original.prefix}…${original.suffix})`,
        }),
      ).not.toBeInTheDocument();
      if (change !== 'unmount') expect(outside).toHaveFocus();
      expect(requests().every(([method]) => method === 'GET')).toBe(true);
      expectNoRevocations();
      expect(onChanged).not.toHaveBeenCalled();
      expect(reveal()).toBeNull();
    },
  );
});
