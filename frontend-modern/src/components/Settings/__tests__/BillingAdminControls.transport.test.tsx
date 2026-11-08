import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { BillingAdminPanel } from '../BillingAdminPanel';
import {
  BILLING_ADMIN_READ_ERROR,
  BILLING_ADMIN_UNCONFIRMED_CHANGE,
} from '../useBillingAdminPanelState';
import type { BillingState, HostedOrganizationSummary } from '@/api/billingAdmin';
import { loadRuntimeCapabilities } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import {
  apiClient,
  clearAuth,
  getApiToken,
  getOrgID,
  setApiToken,
  setOrgID,
} from '@/utils/apiClient';

// Exercise actual row actions, state, policy/capability stores, API and client.
// All HTTP is intercepted; these fixtures never contact a billing service.
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn(), info: vi.fn() },
}));

const firstID = 'org/first?name=shared';
const selectedID = 'org/second%opaque?#';
const ambientOrg = 'unrelated/workspace';
const storedToken = 'synthetic-stored-billing-token';
const csrf = 'synthetic-billing-csrf';
const initialWidth = window.innerWidth;
const listURL = '/api/hosted/organizations';
const billingURL = (id: string) => `/api/admin/orgs/${encodeURIComponent(id)}/billing-state`;
const org = (id: string): HostedOrganizationSummary => ({
  org_id: id,
  display_name: 'Same organization name',
  owner_user_id: `synthetic-owner-${id}`,
  created_at: '2026-01-01T00:00:00Z',
  suspended: false,
  soft_deleted: false,
});
// Include synthetic server-returned extension fields absent from the TS interface.
// This checks faithful cached-payload forwarding, NOT concurrent server preservation.
const billing = (state = 'active'): BillingState & Record<string, unknown> => ({
  subscription_state: state,
  capabilities: ['multi_tenant', 'audit_logging'],
  limits: { max_nodes: 29, max_users: 7 },
  meters_enabled: ['synthetic-usage'],
  plan_version: 'saved-selected-plan',
  trial_started_at: 1700000000,
  trial_ends_at: null,
  stripe_customer_id: 'synthetic-selected-customer',
  stripe_subscription_id: 'synthetic-selected-subscription',
  stripe_price_id: 'synthetic-selected-price',
  entitlement_jwt: 'synthetic-entitlement-not-a-jwt',
  entitlement_refresh_token: 'synthetic-entitlement-refresh',
  commercial_migration: {
    source: 'v5_license',
    state: 'failed',
    reason: 'exchange_unavailable',
    recommended_action: 'retry_activation',
    first_failed_at: 1700000400,
  },
  trial_extended_at: 1700000100,
  overflow_granted_at: 1700000200,
  quickstart_credits_granted: true,
  quickstart_credits_used: 3,
  quickstart_credits_granted_at: 1700000300,
  integrity: 'synthetic-billing-integrity',
});
const siblingBilling = (): BillingState => ({
  subscription_state: 'trial',
  capabilities: ['sso'],
  limits: { max_nodes: 3 },
  stripe_customer_id: 'synthetic-sibling-customer',
});
const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
const pendingBody = () => {
  let finish!: (body: unknown) => void;
  const response = new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        finish = (body) => {
          controller.enqueue(new TextEncoder().encode(JSON.stringify(body)));
          controller.close();
        };
      },
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  return { response, finish };
};
const settle = () => new Promise((done) => setTimeout(done, 0));
const primeCSRF = () => {
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
};
const policy = (readOnly = false, demoMode = false) =>
  syncSessionPresentationPolicy({
    presentationPolicy: { readOnly, demoMode, hideCommercial: false, hideUpgrade: true },
  });

const requests: { url: string; init: RequestInit }[] = [];
let inventory: HostedOrganizationSummary[];
let states: Record<string, BillingState>;
let hosted: boolean;
let multi: boolean;
let listReply: () => Response | Promise<Response>;
let readReply: (id: string) => Response | Promise<Response>;
let writeReply: (id: string, init: RequestInit) => Response | Promise<Response>;
let healthReply: () => Response | Promise<Response>;
let success: MockInstance<typeof notificationStore.success>;
let prepareRequest: MockInstance<typeof apiClient.fetch>;

const lists = () => requests.filter(({ url }) => url === listURL);
const reads = (id: string) =>
  requests.filter(({ url, init }) => url === billingURL(id) && !init.method);
const writes = () => requests.filter(({ init }) => init.method === 'PUT');
const healthReads = () => requests.filter(({ url }) => url === '/api/health');
const row = (id = selectedID) => screen.getByText(id, { exact: true }).closest('tr')!;
const action = (name: 'Suspend Org' | 'Activate Org', id = selectedID) =>
  within(row(id)).getByRole('button', { name });
const loaded = async () => {
  await waitFor(() => expect(action('Suspend Org')).toBeEnabled());
  expect(action('Suspend Org', firstID)).toBeEnabled();
};
const acceptWrite = (id: string, init: RequestInit) => {
  states[id] = JSON.parse(String(init.body));
  return json(states[id]);
};
const expectWire = (request: (typeof requests)[number], method: 'GET' | 'PUT', url: string) => {
  expect(request.url).toBe(url);
  expect(request.init.method ?? 'GET').toBe(method);
  expect(request.init.credentials).toBe('include');
  expect(request.init.signal).toBeInstanceOf(AbortSignal);
  for (const option of ['preferSessionAuth', 'skipOrgContext', 'retry'])
    expect(request.init).not.toHaveProperty(option);
  const headers = new Headers(request.init.headers);
  expect(headers.get('X-Pulse-Org-ID')).toBe('default');
  expect(headers.get('X-API-Token')).toBeNull();
  expect(headers.get('X-Requested-With')).toBe('XMLHttpRequest');
  expect(headers.get('Content-Type')).toBe('application/json');
  if (method === 'PUT') expect(headers.get('X-CSRF-Token')).toBe(csrf);
  else expect(request.init.body).toBeUndefined();
};
const expectAuthRetained = () => {
  expect(getOrgID()).toBe(ambientOrg);
  expect(getApiToken()).toBe(storedToken);
  expect(localStorage.getItem('just_logged_out')).toBeNull();
};

beforeEach(async () => {
  vi.clearAllMocks();
  vi.stubEnv('MODE', 'production');
  sessionStorage.clear();
  localStorage.clear();
  clearAuth();
  setOrgID(ambientOrg);
  setApiToken(storedToken);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1440 });
  policy();
  success = vi.spyOn(notificationStore, 'success');
  // Observe the public request promise without replacing transport or private helpers.
  prepareRequest = vi.spyOn(apiClient, 'fetch');
  inventory = [org(firstID), org(selectedID)];
  states = { [firstID]: siblingBilling(), [selectedID]: billing() };
  hosted = multi = true;
  listReply = () => json(inventory);
  readReply = (id) => json(states[id]);
  writeReply = acceptWrite;
  healthReply = () => {
    primeCSRF();
    return json({});
  };
  requests.length = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (request: string | URL | Request, init: RequestInit = {}) => {
      const url = String(request);
      requests.push({ url, init });
      if (url === '/api/license/runtime-capabilities')
        return json({
          capabilities: multi ? ['multi_tenant'] : [],
          hosted_mode: hosted,
          limits: [],
          max_history_days: 7,
        });
      if (url === listURL && !init.method) return listReply();
      if (url === '/api/health' && init.method === 'GET') return healthReply();
      const id = inventory.find((item) => billingURL(item.org_id) === url)?.org_id;
      if (id && !init.method) return readReply(id);
      if (id && init.method === 'PUT') return writeReply(id, init);
      throw new Error(`Unexpected synthetic request: ${init.method ?? 'GET'} ${url}`);
    }),
  );
  await loadRuntimeCapabilities(true);
});

afterEach(() => {
  cleanup();
  clearAuth();
  syncSessionPresentationPolicy(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: initialWidth });
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('Billing Admin rendered controls through real HTTP', () => {
  it.each([1440, 390])(
    'suspends and reactivates only the non-first namesake organization at width %i',
    async (width) => {
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: width });
      render(() => <BillingAdminPanel />);
      await loaded();
      expectWire(lists()[0], 'GET', listURL);
      expectWire(reads(selectedID)[0], 'GET', billingURL(selectedID));
      const original = structuredClone(states[selectedID]);
      const sibling = structuredClone(states[firstID]);
      const health = deferred<Response>();
      const save = deferred<Response>();
      healthReply = () => health.promise;
      writeReply = () => save.promise;
      fireEvent.click(action('Suspend Org'));
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      expect(writes()).toHaveLength(0);
      expect(action('Suspend Org')).toBeDisabled();
      expect(action('Activate Org')).toBeDisabled();
      expect(action('Suspend Org', firstID)).toBeEnabled();
      expect(success).not.toHaveBeenCalled();
      primeCSRF();
      health.resolve(json({}));
      await waitFor(() => expect(writes()).toHaveLength(1));
      expectWire(writes()[0], 'PUT', billingURL(selectedID));
      expect(JSON.parse(String(writes()[0].init.body))).toEqual({
        ...original,
        subscription_state: 'suspended',
      });
      expect(within(row()).getByText('Active', { exact: true })).toBeInTheDocument();
      expect(success).not.toHaveBeenCalled();
      // The real producer normalizes suspended entitlements to empty sets.
      // Reactivation must use the accepted reply, not resurrect the pre-suspend cache.
      states[selectedID] = {
        ...original,
        subscription_state: 'suspended',
        capabilities: [],
        limits: {},
        meters_enabled: [],
      };
      const accepted = structuredClone(states[selectedID]);
      save.resolve(json(states[selectedID]));
      await waitFor(() => expect(action('Activate Org')).toBeEnabled());
      expect(action('Suspend Org')).toBeDisabled();
      expect(success).toHaveBeenCalledTimes(1);
      expect(within(row(firstID)).getByText('Trial', { exact: true })).toBeInTheDocument();
      expect(states[firstID]).toEqual(sibling);
      expect(reads(selectedID)).toHaveLength(1);
      expect(reads(firstID)).toHaveLength(1);
      expect(lists()).toHaveLength(1);
      // Inventory order can change. Keep targeting identity, not the former row index.
      inventory = [org(selectedID), org(firstID)];
      fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
      await waitFor(() => expect(action('Activate Org')).toBeEnabled());
      writeReply = acceptWrite;
      fireEvent.click(action('Activate Org'));
      await waitFor(() => expect(action('Suspend Org')).toBeEnabled());
      expect(writes()).toHaveLength(2);
      expectWire(writes()[1], 'PUT', billingURL(selectedID));
      expect(JSON.parse(String(writes()[1].init.body))).toEqual({
        ...accepted,
        subscription_state: 'active',
      });
      expect(states[firstID]).toEqual(sibling);
      expect(states[selectedID]).toEqual({ ...accepted, subscription_state: 'active' });
      expect(success).toHaveBeenCalledTimes(2);
      expect(reads(selectedID)).toHaveLength(2);
      expect(reads(firstID)).toHaveLength(2);
      expect(lists()).toHaveLength(2);
      expectAuthRetained();
    },
  );

  it('does not invent a plan, limits or Stripe fields absent from the selected saved state', async () => {
    states[selectedID] = { subscription_state: 'active', capabilities: [], limits: {} };
    primeCSRF();
    render(() => <BillingAdminPanel />);
    await loaded();
    fireEvent.click(action('Suspend Org'));
    await waitFor(() => expect(action('Activate Org')).toBeEnabled());
    expect(writes()).toHaveLength(1);
    expectWire(writes()[0], 'PUT', billingURL(selectedID));
    expect(JSON.parse(String(writes()[0].init.body))).toEqual({
      subscription_state: 'suspended',
      capabilities: [],
      limits: {},
    });
    expect(states[firstID]).toEqual(siblingBilling());
  });

  it.each(['403', '429', '503', 'malformed', 'empty', 'network', 'mismatched'])(
    'withdraws an unconfirmed %s change until an explicit selected-row reload, without replay or sibling loss',
    async (kind) => {
      primeCSRF();
      writeReply = () => {
        if (kind === 'network')
          return Promise.reject(new TypeError('synthetic-private-billing-network'));
        if (kind === 'empty') return new Response(null, { status: 200 });
        return kind === 'malformed'
          ? new Response('synthetic-private-billing-body', { status: 200 })
          : kind === 'mismatched'
            ? json(billing('active'))
            : json({ error: 'synthetic-private-billing-error' }, Number(kind), {
                'X-CSRF-Token': 'synthetic-refreshed-token',
                'Retry-After': '0.01',
              });
      };
      render(() => <BillingAdminPanel />);
      await loaded();
      fireEvent.click(action('Suspend Org'));
      await waitFor(() =>
        expect(within(row()).getByRole('alert')).toHaveTextContent(
          BILLING_ADMIN_UNCONFIRMED_CHANGE,
        ),
      );
      expect(action('Suspend Org')).toBeDisabled();
      expect(action('Activate Org')).toBeDisabled();
      expect(
        within(within(row()).getByRole('alert').closest('td')!).getByText('Unavailable', {
          exact: true,
        }),
      ).toBeInTheDocument();
      expect(action('Suspend Org', firstID)).toBeEnabled();
      expect(document.body).not.toHaveTextContent('synthetic-private-billing');
      expect(success).not.toHaveBeenCalled();
      expect(writes()).toHaveLength(1);
      expect(reads(selectedID)).toHaveLength(1);
      expectAuthRetained();

      // The server may have committed despite the lost/invalid response.
      // Recovery reads the actual row; it must not replay or presume rollback.
      states[selectedID] = {
        ...billing('suspended'),
        capabilities: ['sso'],
        limits: { max_nodes: 43 },
        plan_version: 'newly-saved-selected-plan',
      };
      fireEvent.click(within(row()).getByRole('button', { name: /Reload billing for/ }));
      await waitFor(() => expect(action('Activate Org')).toBeEnabled());
      expect(reads(selectedID)).toHaveLength(2);
      expect(reads(firstID)).toHaveLength(1);
      expect(writes()).toHaveLength(1);
      expect(success).not.toHaveBeenCalled();
      const recovered = structuredClone(states[selectedID]);
      writeReply = acceptWrite;
      fireEvent.click(action('Activate Org'));
      await waitFor(() => expect(action('Suspend Org')).toBeEnabled());
      expect(writes()).toHaveLength(2);
      expectWire(writes()[1], 'PUT', billingURL(selectedID));
      expect(JSON.parse(String(writes()[1].init.body))).toEqual({
        ...recovered,
        subscription_state: 'active',
      });
      expect(states[firstID]).toEqual(siblingBilling());
      expect(success).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['refresh', 'read-only', 'hosted', 'multi', 'demo', 'dispose'])(
    'never dispatches a selected-row change after %s invalidation during cold CSRF preparation',
    async (kind) => {
      const health = deferred<Response>();
      healthReply = () => health.promise;
      const view = render(() => <BillingAdminPanel />);
      await loaded();
      fireEvent.click(action('Suspend Org'));
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      expect(writes()).toHaveLength(0);
      if (kind === 'refresh') fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
      else if (kind === 'read-only') policy(true);
      else if (kind === 'demo') policy(false, true);
      else if (kind === 'dispose') view.unmount();
      else {
        if (kind === 'hosted') hosted = false;
        else multi = false;
        await loadRuntimeCapabilities(true);
      }
      await settle();
      if (kind !== 'dispose' && kind !== 'refresh' && kind !== 'read-only') {
        expect(screen.queryByText(selectedID, { exact: true })).not.toBeInTheDocument();
        hosted = multi = true;
        policy();
        await loadRuntimeCapabilities(true);
      }
      primeCSRF();
      const requestIndex = prepareRequest.mock.calls.findIndex(
        ([url, options]) => url === billingURL(selectedID) && options?.method === 'PUT',
      );
      expect(requestIndex).toBeGreaterThanOrEqual(0);
      const preparation = prepareRequest.mock.results[requestIndex];
      expect(preparation.type).toBe('return');
      const completion = Promise.resolve(preparation.value).then(
        () => 'dispatched',
        (error: unknown) => {
          expect(error).toMatchObject({ name: 'AbortError' });
          return 'aborted';
        },
      );
      health.resolve(json({}));
      // Await the actual public request, including post-health preparation,
      // before checking absence even when there will be no subsequent action.
      expect(await completion).toBe('aborted');
      await settle();
      expect(writes()).toHaveLength(0);
      expect(success).not.toHaveBeenCalled();
      expect(states[selectedID]).toEqual(billing());
      expect(states[firstID]).toEqual(siblingBilling());
      expectAuthRetained();
      if (kind === 'read-only') {
        expect(action('Suspend Org')).toBeDisabled();
        expect(action('Activate Org')).toBeDisabled();
        policy();
      }
      if (kind !== 'dispose') {
        await loaded();
        // A new explicit action still works, using a fresh admitted snapshot.
        fireEvent.click(action('Suspend Org'));
        await waitFor(() => expect(action('Activate Org')).toBeEnabled());
        expect(writes()).toHaveLength(1);
        expectWire(writes()[0], 'PUT', billingURL(selectedID));
      }
    },
  );

  it.each(['success', 'invalid'])(
    'guards the current client snapshot from a late streamed write %s after read-only withdrawal and return',
    async (outcome) => {
      primeCSRF();
      const old = pendingBody();
      writeReply = () => old.response;
      render(() => <BillingAdminPanel />);
      await loaded();
      fireEvent.click(action('Suspend Org'));
      await waitFor(() => expect(old.response.bodyUsed).toBe(true));
      // A synthetic Response ignores abort. This exercises the hook's post-body
      // generation guard, not browser transport cancellation or server rollback.
      const oldSignal = writes()[0].init.signal;
      policy(true);
      await settle();
      states[selectedID] = { ...billing('trial'), plan_version: 'current-after-withdrawal' };
      policy();
      await loaded();
      expect(oldSignal?.aborted).toBe(true);
      old.finish(outcome === 'success' ? billing('suspended') : {});
      await settle();
      expect(within(row()).getByText('Trial', { exact: true })).toBeInTheDocument();
      expect(within(row()).queryByRole('alert')).not.toBeInTheDocument();
      expect(action('Suspend Org')).toBeEnabled();
      expect(success).not.toHaveBeenCalled();
      expect(writes()).toHaveLength(1);
      expect(reads(selectedID)).toHaveLength(3);
      writeReply = acceptWrite;
      fireEvent.click(action('Suspend Org'));
      await waitFor(() => expect(action('Activate Org')).toBeEnabled());
      expect(JSON.parse(String(writes()[1].init.body))).toEqual({
        ...billing('suspended'),
        plan_version: 'current-after-withdrawal',
      });
      expect(success).toHaveBeenCalledTimes(1);
    },
  );

  it('keeps a failed selected-row read unavailable and reloads only that namesake while read-only', async () => {
    readReply = (id) =>
      id === selectedID ? json({ error: 'synthetic-private-row-read' }, 503) : json(states[id]);
    policy(true);
    render(() => <BillingAdminPanel />);
    await waitFor(() =>
      expect(within(row()).getByRole('alert')).toHaveTextContent(BILLING_ADMIN_READ_ERROR),
    );
    expect(within(row(firstID)).getByText('Trial', { exact: true })).toBeInTheDocument();
    expect(action('Suspend Org')).toBeDisabled();
    expect(action('Activate Org')).toBeDisabled();
    expect(document.body).not.toHaveTextContent('synthetic-private-row-read');
    readReply = (id) => json(states[id]);
    fireEvent.click(within(row()).getByRole('button', { name: /Reload billing for/ }));
    await within(row()).findByText('Active', { exact: true });
    expect(within(row()).queryByRole('alert')).not.toBeInTheDocument();
    expect(action('Suspend Org')).toBeDisabled();
    expect(action('Activate Org')).toBeDisabled();
    expect(reads(selectedID)).toHaveLength(2);
    expect(reads(firstID)).toHaveLength(1);
    expect(writes()).toHaveLength(0);
    expect(success).not.toHaveBeenCalled();
    expectAuthRetained();
  });

  it('ignores an old streamed inventory after policy withdrawal and re-entry', async () => {
    const old = pendingBody();
    listReply = () => old.response;
    render(() => <BillingAdminPanel />);
    await waitFor(() => expect(old.response.bodyUsed).toBe(true));
    expect(reads(selectedID)).toHaveLength(0);
    // Loading disables Refresh. Withdraw/re-enter through the actual policy store.
    policy(false, true);
    await settle();
    listReply = () => json(inventory);
    policy();
    await loaded();
    expect(lists()[0].init.signal?.aborted).toBe(true);
    old.finish([org('obsolete/organization')]);
    await settle();
    expect(screen.queryByText('obsolete/organization')).not.toBeInTheDocument();
    expect(action('Suspend Org')).toBeEnabled();
    expect(reads(firstID)).toHaveLength(1);
    expect(reads(selectedID)).toHaveLength(1);
    expect(writes()).toHaveLength(0);
  });
});
