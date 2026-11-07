import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { eventBus } from '@/stores/events';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { useSSOProvidersState } from '../useSSOProvidersState';
import type { SSOProvider } from '../ssoProvidersModel';

// Keep the provider owner, payload model and authenticated client connected.
// Unlike the adjacent warm-cookie suites, exercise production's asynchronous
// CSRF preparation. Only the final HTTP boundary and notifications are synthetic.
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

type State = ReturnType<typeof useSSOProvidersState>;
type ProviderType = 'oidc' | 'saml';
type Operation = 'create' | 'edit' | 'toggle' | 'delete';
const operations: Operation[] = ['create', 'edit', 'toggle', 'delete'];
const listURL = '/api/security/sso/providers';
const restrictions = {
  allowedGroups: ['Server Operators', 'Backup Readers'],
  allowedDomains: ['example.test'],
  allowedEmails: ['reader@example.test'],
  groupsClaim: 'access_groups',
  groupRoleMappings: { 'Server Operators': 'operator', 'Backup Readers': 'viewer' },
};
const provider = (type: ProviderType): SSOProvider => ({
  id: `saved/${type}:production`,
  name: 'Saved production provider',
  type,
  enabled: true,
  displayName: 'Company sign-in',
  priority: 7,
  ...structuredClone(restrictions),
});
const details = (type: ProviderType) => ({
  ...provider(type),
  ...(type === 'oidc'
    ? {
        oidc: {
          issuerUrl: 'https://issuer.example.test',
          clientId: 'pulse-client',
          clientSecretSet: true,
          scopes: ['openid', 'profile', 'email'],
        },
      }
    : {
        saml: {
          idpEntityId: 'synthetic-idp',
          idpSsoUrl: 'https://issuer.example.test/sso',
          signRequests: true,
          allowIdpInitiated: false,
          emailAttr: 'mail',
          groupsAttr: 'access_groups',
        },
      }),
});
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const fetchMock = vi.fn<typeof fetch>();
const writes = () =>
  fetchMock.mock.calls.filter(([, options]) =>
    ['POST', 'PUT', 'DELETE'].includes(options?.method ?? 'GET'),
  );
const healthReads = () => fetchMock.mock.calls.filter(([url]) => url === '/api/health');
const inventoryReads = () =>
  fetchMock.mock.calls.filter(
    ([url, options]) => url === listURL && (options?.method ?? 'GET') === 'GET',
  );
let saved: SSOProvider[];
let fixtureType: ProviderType;
let preparation: ReturnType<typeof deferred<Response>>;
let reply: ReturnType<typeof deferred<Response>>;

function mount(type: ProviderType = 'oidc') {
  fixtureType = type;
  saved = [provider(type)];
  const [identity, setIdentity] = createSignal({ user: 'admin-a', allowed: true });
  const changed = vi.fn();
  let state!: State;
  const view = render(() => {
    state = useSSOProvidersState({
      get currentUser() {
        return identity().user;
      },
      get canManage() {
        return identity().allowed;
      },
      onConfigUpdated: changed,
    });
    return <div />;
  });
  return {
    state,
    view,
    changed,
    refresh: () => setIdentity((current) => ({ ...current })),
    changeUser: () => setIdentity((current) => ({ ...current, user: 'admin-b' })),
    withdraw: () => setIdentity((current) => ({ ...current, allowed: false })),
  };
}
async function loaded(state: State) {
  await waitFor(() => expect(state.loading()).toBe(false));
  expect(state.loadError()).toBe(false);
}
async function prepare(state: State, operation: Operation) {
  await loaded(state);
  expect(state.canEdit()).toBe(true);
  const selected = state.providers()[0];
  if (operation === 'create') {
    state.openAddModal(fixtureType);
    state.setForm({
      name: 'New production provider',
      displayName: 'Company sign-in',
      priority: 7,
      allowedGroups: 'Server Operators, Backup Readers',
      allowedDomains: 'example.test',
      allowedEmails: 'reader@example.test',
      groupsClaim: 'access_groups',
      groupRoleMappings: 'Server Operators=operator, Backup Readers=viewer',
      oidcIssuerUrl: 'https://issuer.example.test',
      oidcClientId: 'pulse-client',
      oidcClientSecret: 'SYNTHETIC_NEW_PROVIDER_SECRET',
      samlIdpEntityId: 'synthetic-idp',
      samlIdpSsoUrl: 'https://issuer.example.test/sso',
      samlSignRequests: true,
      samlEmailAttr: 'mail',
    });
  } else if (operation === 'edit') {
    await state.openEditModal(selected);
    expect(state.showModal()).toBe(true);
    expect(state.form.oidcClientSecret).toBe('');
    state.setForm('name', 'Renamed production provider');
  } else if (operation === 'delete') {
    state.setDeleteConfirm(selected.id);
  }
  return selected;
}
function submit(state: State, operation: Operation, selected: SSOProvider) {
  if (operation === 'delete') return state.handleDelete(selected.id);
  if (operation === 'toggle') return state.handleToggleEnabled(selected);
  return state.handleSave();
}
function finishPreparation() {
  document.cookie = 'pulse_csrf=synthetic-prepared-csrf; Path=/';
  preparation.resolve(json({}));
}
function switchOrg(emit = true) {
  setOrgID('south');
  if (emit) eventBus.emit('org_switched', 'south');
}
function expectedPayload(type: ProviderType, operation: Operation) {
  if (operation === 'delete') return undefined;
  const common = {
    ...(operation === 'create' ? {} : { id: provider(type).id }),
    name:
      operation === 'create'
        ? 'New production provider'
        : operation === 'edit'
          ? 'Renamed production provider'
          : provider(type).name,
    type,
    enabled: operation !== 'toggle',
    displayName: 'Company sign-in',
    priority: 7,
    ...restrictions,
  };
  if (operation === 'toggle') return common;
  return {
    ...common,
    ...(type === 'oidc'
      ? {
          oidc: {
            issuerUrl: 'https://issuer.example.test',
            clientId: 'pulse-client',
            ...(operation === 'create' ? { clientSecret: 'SYNTHETIC_NEW_PROVIDER_SECRET' } : {}),
            scopes: ['openid', 'profile', 'email'],
          },
        }
      : {
          saml: {
            idpEntityId: 'synthetic-idp',
            idpSsoUrl: 'https://issuer.example.test/sso',
            signRequests: true,
            allowIdpInitiated: false,
            emailAttr: 'mail',
            groupsAttr: 'access_groups',
          },
        }),
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubEnv('MODE', 'production');
  clearAuth();
  sessionStorage.clear();
  setOrgID('north');
  setApiToken('synthetic-sso-admin-token');
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  preparation = deferred<Response>();
  reply = deferred<Response>();
  fetchMock.mockImplementation(async (input, options = {}) => {
    const url = String(input);
    const method = options.method ?? 'GET';
    if (url === '/api/health' && method === 'GET') return preparation.promise;
    if (method === 'GET') {
      if (url === listURL) {
        const org = (options.headers as Record<string, string>)['X-Pulse-Org-ID'];
        return json({
          providers:
            org === 'south' ? [{ ...provider(fixtureType), name: 'South provider' }] : saved,
        });
      }
      if (url === '/api/security/status') return json({ publicUrl: 'https://pulse.example.test' });
      if (url === `${listURL}/${encodeURIComponent(provider(fixtureType).id)}`)
        return json(details(fixtureType));
    }
    if (
      (url === listURL && method === 'POST') ||
      (url === `${listURL}/${encodeURIComponent(provider(fixtureType).id)}` &&
        ['PUT', 'DELETE'].includes(method))
    ) {
      // Model a server that applied the request before its reply was lost.
      // This is synthetic saved state, not native IdP/secret persistence proof.
      saved =
        method === 'DELETE'
          ? []
          : [
              {
                ...provider(fixtureType),
                ...JSON.parse(String(options.body)),
                id: method === 'POST' ? 'created/provider' : provider(fixtureType).id,
              },
            ];
      // Ignore signal here deliberately. Native fetch cancellation alone must
      // not hide a dispatch made after the confirmation owner was withdrawn.
      return reply.promise;
    }
    throw new Error(`Unexpected synthetic SSO request: ${method} ${url}`);
  });
});
afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe.each(['oidc', 'saml'] as const)('%s provider cold-CSRF composition', (type) => {
  it.each(
    operations.flatMap((operation) =>
      (['confirmed', 'unconfirmed'] as const).map((outcome) => ({ operation, outcome })),
    ),
  )(
    'keeps one $operation and its $outcome outcome across equal-reader refreshes',
    async ({ operation, outcome }) => {
      const { state, changed, refresh } = mount(type);
      const selected = await prepare(state, operation);
      const saving = submit(state, operation, selected);
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      expect(healthReads()[0][1]).toEqual({ method: 'GET', credentials: 'include' });
      expect(writes()).toHaveLength(0);
      expect(state.mutating()).toBe(true);
      refresh();
      await settle();
      await submit(state, operation, selected);
      expect(state.mutating()).toBe(true);
      expect(healthReads()).toHaveLength(1);
      expect(inventoryReads()).toHaveLength(1);
      expect(writes()).toHaveLength(0);

      finishPreparation();
      await waitFor(() => expect(writes()).toHaveLength(1));
      const [url, options] = writes()[0];
      expect(url).toBe(
        operation === 'create' ? listURL : `${listURL}/${encodeURIComponent(selected.id)}`,
      );
      expect(options).toMatchObject({
        method: operation === 'create' ? 'POST' : operation === 'delete' ? 'DELETE' : 'PUT',
        credentials: 'include',
        signal: expect.any(AbortSignal),
        headers: {
          'X-Pulse-Org-ID': 'north',
          'X-API-Token': 'synthetic-sso-admin-token',
          'X-CSRF-Token': 'synthetic-prepared-csrf',
        },
      });
      expect(options).not.toHaveProperty('retry');
      expect(options).not.toHaveProperty('expectedOrgID');
      expect(options?.body ? JSON.parse(String(options.body)) : undefined).toEqual(
        expectedPayload(type, operation),
      );
      refresh();
      await settle();
      await submit(state, operation, selected);
      expect(writes()).toHaveLength(1);
      expect(inventoryReads()).toHaveLength(1);
      expect(options?.signal?.aborted).toBe(false);
      expect(state.mutating()).toBe(true);
      expect(notices.success).not.toHaveBeenCalled();
      expect(changed).not.toHaveBeenCalled();

      reply.resolve(
        json(
          outcome === 'confirmed' ? {} : { error: 'synthetic gateway failure' },
          outcome === 'confirmed' ? 200 : 502,
        ),
      );
      await saving;
      if (outcome === 'confirmed') {
        expect(state.unconfirmedChange()).toBeNull();
        expect(state.showModal()).toBe(false);
        expect(state.deleteConfirm()).toBeNull();
        expect(notices.success).toHaveBeenCalledTimes(1);
        expect(changed).toHaveBeenCalledTimes(1);
      } else {
        expect(state.unconfirmedChange()).toContain('may already have been applied');
        expect(state.canEdit()).toBe(false);
        if (operation === 'create' || operation === 'edit') {
          expect(state.showModal()).toBe(true);
          expect(state.form.name).toBe(expectedPayload(type, operation)?.name);
        }
        refresh();
        await settle();
        await submit(state, operation, selected);
        expect(state.unconfirmedChange()).toContain('may already have been applied');
        expect(inventoryReads()).toHaveLength(1);
        expect(writes()).toHaveLength(1);
        // Only an explicit read reconciles the applied outcome; never replay it.
        await state.loadProviders();
        expect(state.unconfirmedChange()).toBeNull();
        expect(state.showModal()).toBe(false);
        expect(state.form.oidcClientSecret).toBe('');
        expect(notices.success).not.toHaveBeenCalled();
        expect(changed).not.toHaveBeenCalled();
      }
      expect(state.mutating()).toBe(false);
      expect(state.canEdit()).toBe(true);
      expect(state.providers()).toEqual(saved);
      expect(inventoryReads()).toHaveLength(2);
      expect(writes()).toHaveLength(1);
      expect(healthReads()).toHaveLength(1);
      expect(notices.error).not.toHaveBeenCalled();
      expect(getOrgID()).toBe('north');
    },
  );
});

describe('SSO confirmation withdrawal before HTTP dispatch', () => {
  it.each(
    operations.flatMap((operation) =>
      (['organisation', 'user', 'permission', 'unmount'] as const).map((change) => ({
        operation,
        change,
      })),
    ),
  )(
    'never dispatches $operation after $change changes during preparation',
    async ({ operation, change }) => {
      const { state, view, changed, changeUser, withdraw } = mount();
      const selected = await prepare(state, operation);
      const saving = submit(state, operation, selected);
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      if (change === 'organisation') switchOrg();
      if (change === 'user') changeUser();
      if (change === 'permission') withdraw();
      if (change === 'unmount') view.unmount();
      await settle();
      finishPreparation();
      // An erroneous dispatch must also settle so the assertion reports the
      // actual write, not merely a timeout waiting for the deliberately held reply.
      reply.resolve(json({}));
      await saving;
      expect(writes()).toHaveLength(0);
      expect(saved).toEqual([provider('oidc')]);
      expect(state.showModal()).toBe(false);
      expect(state.form.oidcClientSecret).toBe('');
      expect(state.unconfirmedChange()).toBeNull();
      expect(state.mutating()).toBe(false);
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).not.toHaveBeenCalled();
      expect(changed).not.toHaveBeenCalled();
      expect(healthReads()).toHaveLength(1);
      if (change !== 'unmount') {
        expect(state.deleteConfirm()).toBeNull();
        await loaded(state);
        expect(inventoryReads()).toHaveLength(2);
        expect(state.canEdit()).toBe(change !== 'permission');
      } else {
        // Detached signals need not be reset, but can never readmit this owner.
        await submit(state, operation, selected);
        expect(state.canEdit()).toBe(false);
        expect(writes()).toHaveLength(0);
        eventBus.emit('org_switched', 'north');
        await settle();
        expect(inventoryReads()).toHaveLength(1);
        expect(state.providers()).toEqual([]);
      }
      if (change === 'organisation') {
        expect(state.providers()[0].name).toBe('South provider');
        expect(inventoryReads()[1][1]?.headers).toMatchObject({ 'X-Pulse-Org-ID': 'south' });
      }
    },
  );

  it.each(operations)(
    'rechecks the admitted org for %s even before the UI event arrives',
    async (operation) => {
      const { state, changed } = mount();
      const selected = await prepare(state, operation);
      const saving = submit(state, operation, selected);
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      switchOrg(false);
      finishPreparation();
      reply.resolve(json({}));
      await saving;
      expect(writes()).toHaveLength(0);
      expect(saved).toEqual([provider('oidc')]);
      expect(getOrgID()).toBe('south');
      expect(state.canManage()).toBe(false);
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).not.toHaveBeenCalled();
      expect(changed).not.toHaveBeenCalled();
      eventBus.emit('org_switched', 'south');
      await loaded(state);
      expect(state.providers()[0].name).toBe('South provider');
      expect(state.showModal()).toBe(false);
      expect(state.canEdit()).toBe(true);
    },
  );
});
