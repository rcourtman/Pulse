import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import type { APITokenRecord } from '@/api/security';
import type { Resource } from '@/types/resource';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { dismissTokenReveal, useTokenRevealState } from '@/stores/tokenReveal';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { useAPITokenManagerState } from '../useAPITokenManagerState';

// The state owner, SecurityAPI, authenticated client, decoding and reveal store
// stay connected. In particular, an unannounced workspace change does not abort
// the controller or change its version: completion must recheck the reader too.
const environment = vi.hoisted(() => ({ markAgents: vi.fn(), markRuntimes: vi.fn() }));
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
    resources: resourceList,
    byType: (type: string) => resourceList().filter((resource) => resource.type === type),
  }),
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), debug: vi.fn(), warn: vi.fn() },
}));

const original: APITokenRecord = {
  id: 'token/collector:one',
  name: 'Saved collector',
  prefix: 'test',
  suffix: '1234',
  createdAt: '2026-11-01T00:00:00Z',
  scopes: ['monitoring:read'],
  ownerUserId: 'synthetic-user',
  expiresAt: '2027-01-01T00:00:00Z',
};
const created: APITokenRecord = {
  ...original,
  id: 'token/created',
  name: 'New collector',
  createdAt: '2026-11-05T00:00:00Z',
};
const southToken: APITokenRecord = {
  ...original,
  id: 'token/south',
  name: 'South collector',
};
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
const [resourceList, setResourceList] = createSignal<Resource[]>([]);
const json = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
};
type Operation = 'create' | 'scope' | 'rename' | 'revoke';
const operations: Operation[] = ['create', 'scope', 'rename', 'revoke'];
const methods = { create: 'POST', scope: 'PATCH', rename: 'PATCH', revoke: 'DELETE' };
const payloads = {
  create: { name: 'New collector', scopes: ['monitoring:read'] },
  scope: { scopes: ['monitoring:read', 'settings:read'] },
  rename: { name: 'Updated collector' },
  revoke: undefined,
};
const savedAfter = (operation: Operation) =>
  operation === 'create'
    ? [created, original]
    : operation === 'revoke'
      ? []
      : [{ ...original, ...payloads[operation] }];
const responseBody = (operation: Operation, accepted = true) => {
  if (!accepted) return JSON.stringify({ error: 'synthetic unconfirmed response' });
  if (operation === 'create')
    return JSON.stringify({ token: 'synthetic-only-issued-value', record: created });
  if (operation === 'revoke') return '';
  return JSON.stringify({ record: savedAfter(operation)[0] });
};
const fetchMock = vi.fn<typeof fetch>();
const changed = vi.fn();
const reveal = useTokenRevealState();
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) =>
    ['POST', 'PATCH', 'DELETE'].includes(init?.method ?? ''),
  );
const inventories = () =>
  fetchMock.mock.calls.filter(([url, init]) => url === '/api/security/tokens' && !init?.method);
const preparations = () => fetchMock.mock.calls.filter(([url]) => url === '/api/health');
let preparation: ReturnType<typeof deferred<Response>>;
let savedNorth: APITokenRecord[];
let pendingBody: ReturnType<typeof deferred<string>> | undefined;
let bodyRead: Response['text'] | undefined;
let accepted: boolean;

const mount = async () => {
  const [identity, setIdentity] = createSignal({ user: 'reader-a', allowed: true, hint: 'first' });
  let state!: ReturnType<typeof useAPITokenManagerState>;
  render(() => {
    state = useAPITokenManagerState({
      get currentUser() {
        return identity().user;
      },
      get canManage() {
        return identity().allowed;
      },
      get currentTokenHint() {
        return identity().hint;
      },
      onTokensChanged: changed,
    });
    return <div />;
  });
  await waitFor(() => expect(state.tokensLoaded()).toBe(true));
  state.setNameInput('  New collector  ');
  state.applyScopePreset(['monitoring:read']);
  return { state, setIdentity };
};
type State = Awaited<ReturnType<typeof mount>>['state'];
const submit = (state: State, operation: Operation, record: APITokenRecord) => {
  if (operation === 'create') return state.handleGenerate();
  if (operation === 'scope')
    return state.handleUpdateScopes(record, ['settings:read', 'monitoring:read']);
  if (operation === 'rename') return state.handleRename(record, '  Updated collector  ');
  return state.handleDelete(record);
};
const finishPreparation = () => {
  document.cookie = 'pulse_csrf=synthetic-prepared-csrf; Path=/';
  preparation.resolve(json({}));
};
const expectNoAcceptedMutation = (state: State) => {
  expect(state.newTokenValue()).toBeNull();
  expect(state.newTokenRecord()).toBeNull();
  expect(reveal()).toBeNull();
  expect(notificationStore.success).not.toHaveBeenCalled();
  expect(changed).not.toHaveBeenCalled();
  expect(environment.markAgents).not.toHaveBeenCalled();
  expect(environment.markRuntimes).not.toHaveBeenCalled();
};
const expectDispatch = (operation: Operation) => {
  expect(preparations()).toHaveLength(1);
  expect(writes()).toHaveLength(1);
  const [url, init] = writes()[0];
  expect(url).toBe(
    operation === 'create'
      ? '/api/security/tokens'
      : `/api/security/tokens/${encodeURIComponent(original.id)}`,
  );
  expect(init).toMatchObject({
    method: methods[operation],
    credentials: 'include',
    signal: expect.any(AbortSignal),
    headers: {
      'Content-Type': 'application/json',
      'X-API-Token': 'synthetic-reader-token',
      'X-Pulse-Org-ID': 'estate/north',
      'X-CSRF-Token': 'synthetic-prepared-csrf',
    },
  });
  expect(init?.signal?.aborted).toBe(false);
  expect(init).not.toHaveProperty('expectedOrgID');
  expect(init).not.toHaveProperty('retry');
  expect(init?.body ? JSON.parse(String(init.body)) : undefined).toEqual(payloads[operation]);
};
const announceSouth = async (state: State) => {
  // The existing context event is what recovers the panel. Scope the fixture's
  // runtime inventory too; old resources must not invent revocations in south.
  setResourceList([]);
  eventBus.emit('org_switched', 'estate/south');
  await waitFor(() => expect(state.tokensLoaded()).toBe(true));
  expect(state.sortedTokens()).toEqual([southToken]);
  expect(state.canManage()).toBe(true);
  expect(getOrgID()).toBe('estate/south');
};

beforeEach(() => {
  vi.clearAllMocks();
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  setApiToken('synthetic-reader-token');
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  // Exercise the actual preparation branch, not its test-mode shortcut.
  vi.stubEnv('MODE', 'production');
  dismissTokenReveal();
  setResourceList(resources);
  preparation = deferred<Response>();
  savedNorth = [original];
  pendingBody = undefined;
  bodyRead = undefined;
  accepted = true;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url, init) => {
    if (url === '/api/health') return preparation.promise;
    if (url === '/api/security/tokens' && !init?.method) {
      const org = (init?.headers as Record<string, string>)['X-Pulse-Org-ID'];
      return json({ tokens: org === 'estate/south' ? [southToken] : savedNorth });
    }
    if (
      ['POST', 'PATCH', 'DELETE'].includes(init?.method ?? '') &&
      ['/api/security/tokens', `/api/security/tokens/${encodeURIComponent(original.id)}`].includes(
        String(url),
      )
    ) {
      const operation: Operation =
        init?.method === 'POST'
          ? 'create'
          : init?.method === 'DELETE'
            ? 'revoke'
            : JSON.parse(String(init?.body)).name
              ? 'rename'
              : 'scope';
      // Even the unconfirmed response can follow a committed write. Never use
      // missing acknowledgement to infer that the server rolled it back.
      savedNorth = savedAfter(operation);
      const body = responseBody(operation, accepted);
      const response = new Response(accepted && operation === 'revoke' ? null : body, {
        status: accepted ? (operation === 'revoke' ? 204 : 200) : 503,
      });
      // Do not let cancellation in the fixture conceal acceptance of old text.
      if (pendingBody) bodyRead = vi.spyOn(response, 'text').mockReturnValue(pendingBody.promise);
      return response;
    }
    throw new Error(`Unexpected synthetic token request: ${init?.method ?? 'GET'} ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  dismissTokenReveal();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  document.cookie = 'pulse_org_id=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('API token reader ownership through real cold preparation and decoding', () => {
  it.each(operations)('keeps one %s pending across an equal reader refresh', async (operation) => {
    const { state, setIdentity } = await mount();
    const record = state.sortedTokens()[0];
    const version = state.contextVersion();
    const saving = submit(state, operation, record);
    await waitFor(() => expect(preparations()).toHaveLength(1));
    // A new backing object really notifies both principal/authority memos;
    // assigning the same primitive values would not exercise their equality.
    setIdentity({ user: 'reader-a', allowed: true, hint: 'second' });
    await Promise.all(operations.map((other) => submit(state, other, record)));
    expect(state.contextVersion()).toBe(version);
    expect(state.isMutating()).toBe(true);
    expect(writes()).toHaveLength(0);
    expect(inventories()).toHaveLength(1);
    expectNoAcceptedMutation(state);
    finishPreparation();
    await saving;
    expectDispatch(operation);
    expect(state.sortedTokens()).toEqual(savedAfter(operation));
    expect(state.unconfirmedChange()).toBeNull();
    expect(state.isMutating()).toBe(false);
    expect(changed).toHaveBeenCalledTimes(1);
    expect(notificationStore.success).toHaveBeenCalledTimes(1);
    expect(notificationStore.error).not.toHaveBeenCalled();
    if (operation === 'create') {
      expect(reveal()).toMatchObject({
        token: 'synthetic-only-issued-value',
        record: created,
        source: 'security',
      });
      expect(state.newTokenValue()).toBe('synthetic-only-issued-value');
    } else expect(reveal()).toBeNull();
    if (operation === 'revoke') {
      expect(environment.markAgents).toHaveBeenCalledWith(original.id, ['agent-one']);
      expect(environment.markRuntimes).toHaveBeenCalledWith(original.id, ['docker-host-one']);
    } else {
      expect(environment.markAgents).not.toHaveBeenCalled();
      expect(environment.markRuntimes).not.toHaveBeenCalled();
    }
    expect(inventories()).toHaveLength(1);
  });

  it.each(operations)(
    'blocks %s after a workspace switch without its UI event during preparation',
    async (operation) => {
      const { state } = await mount();
      const version = state.contextVersion();
      const saving = submit(state, operation, state.sortedTokens()[0]);
      await waitFor(() => expect(preparations()).toHaveLength(1));
      setOrgID('estate/south');
      finishPreparation();
      await saving;
      expect(writes()).toHaveLength(0);
      expect(savedNorth).toEqual([original]);
      expect(state.contextVersion()).toBe(version);
      expect(state.canManage()).toBe(false);
      expectNoAcceptedMutation(state);
      await announceSouth(state);
      expect(writes()).toHaveLength(0);
      expect(inventories()).toHaveLength(2);
    },
  );

  it.each(
    operations.flatMap((operation) => [true, false].map((success) => ({ operation, success }))),
  )(
    'discards $operation response (success: $success) after unannounced workspace drift',
    async ({ operation, success }) => {
      accepted = success;
      pendingBody = deferred<string>();
      const { state } = await mount();
      const version = state.contextVersion();
      const saving = submit(state, operation, state.sortedTokens()[0]);
      await waitFor(() => expect(preparations()).toHaveLength(1));
      finishPreparation();
      await waitFor(() => expect(bodyRead).toHaveBeenCalledTimes(1));
      expectDispatch(operation);
      setOrgID('estate/south');
      expect(state.contextVersion()).toBe(version);
      // No event, no abort: the current-reader check is the only remaining
      // guard. This deliberately retains the old inventory, inert, until the
      // real context event. It is not automatic recovery or server rollback.
      expect(writes()[0][1]?.signal?.aborted).toBe(false);
      pendingBody.resolve(responseBody(operation, success));
      await saving;
      expect(state.sortedTokens()).toEqual([original]);
      expect(savedNorth).toEqual(savedAfter(operation));
      expect(state.canManage()).toBe(false);
      expect(state.isMutating()).toBe(false);
      expect(state.unconfirmedChange()).toBeNull();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expectNoAcceptedMutation(state);
      expect(inventories()).toHaveLength(1);
      await announceSouth(state);
      expect(writes()).toHaveLength(1);
      expect(inventories()).toHaveLength(2);
      expectNoAcceptedMutation(state);
    },
  );
});
