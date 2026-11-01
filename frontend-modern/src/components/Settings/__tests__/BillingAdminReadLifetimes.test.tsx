import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { BillingAdminPanel } from '../BillingAdminPanel';
import {
  BILLING_ADMIN_READ_ERROR,
  BILLING_ADMIN_UNCONFIRMED_CHANGE,
  useBillingAdminPanelState,
} from '../useBillingAdminPanelState';
import type { BillingState } from '@/api/billingAdmin';

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  get: vi.fn(),
  put: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  log: vi.fn(),
  hosted: vi.fn(),
  multi: vi.fn(),
  hidden: vi.fn(),
  readOnly: vi.fn(),
}));
vi.mock('@/api/billingAdmin', () => ({
  BillingAdminAPI: {
    listOrganizations: mocks.list,
    getBillingState: mocks.get,
    putBillingState: mocks.put,
  },
}));
vi.mock('@/stores/license', () => ({
  isHostedModeEnabled: mocks.hosted,
  isMultiTenantEnabled: mocks.multi,
}));
vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesOrganizationSurfaces: mocks.hidden,
  presentationPolicyIsReadOnly: mocks.readOnly,
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: mocks.success, error: mocks.error },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: mocks.log } }));

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const org = (id = 'org-a') => ({
  org_id: id,
  display_name: `Organization ${id}`,
  owner_user_id: 'owner',
  created_at: '2026-01-01',
  suspended: false,
  soft_deleted: false,
});
const billing = (name = 'active'): BillingState => ({
  subscription_state: name,
  capabilities: ['multi_tenant'],
  limits: { max_nodes: 20 },
  meters_enabled: [],
});
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));
const signalAt = (mock: typeof mocks.list, call = 0, position = 0) =>
  (mock.mock.calls[call][position] as { signal: AbortSignal }).signal;
const mountState = () => {
  let state!: ReturnType<typeof useBillingAdminPanelState>;
  const view = render(() => {
    state = useBillingAdminPanelState();
    return null;
  });
  return { state, unmount: view.unmount };
};
let setHosted!: (v: boolean) => void;
let setMulti!: (v: boolean) => void;
let setHidden!: (v: boolean) => void;
let setReadOnly!: (v: boolean) => void;

beforeEach(() => {
  Object.values(mocks).forEach((mock) => mock.mockReset());
  const [hosted, hostedSetter] = createSignal(true);
  setHosted = hostedSetter;
  const [multi, multiSetter] = createSignal(true);
  setMulti = multiSetter;
  const [hidden, hiddenSetter] = createSignal(false);
  setHidden = hiddenSetter;
  const [readOnly, readOnlySetter] = createSignal(false);
  setReadOnly = readOnlySetter;
  mocks.hosted.mockImplementation(hosted);
  mocks.multi.mockImplementation(multi);
  mocks.hidden.mockImplementation(hidden);
  mocks.readOnly.mockImplementation(readOnly);
  mocks.list.mockResolvedValue([org()]);
  mocks.get.mockResolvedValue(billing());
  mocks.put.mockImplementation(async (_id, payload) => payload);
});
afterEach(cleanup);

describe('Billing Admin accepted inventory and billing reads', () => {
  it('disables both actions until the actual row read succeeds', async () => {
    const pending = deferred<BillingState>();
    mocks.get.mockReturnValueOnce(pending.promise);
    render(() => <BillingAdminPanel />);
    await waitFor(() => expect(mocks.get).toHaveBeenCalledTimes(1));
    expect(screen.getByRole('button', { name: 'Suspend Org' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Activate Org' })).toBeDisabled();
    expect(screen.queryByText('N/A')).not.toBeInTheDocument();
    pending.resolve(billing());
    await waitFor(() => expect(screen.getByRole('button', { name: 'Suspend Org' })).toBeEnabled());
  });

  it('removes the former inventory on refresh and distinguishes failure from an empty inventory', async () => {
    render(() => <BillingAdminPanel />);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Suspend Org' })).toBeEnabled());
    const pending = deferred<unknown>();
    mocks.list.mockReturnValueOnce(pending.promise);
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(screen.queryByText('Organization org-a')).not.toBeInTheDocument();
    pending.reject(new Error('SYNTHETIC PRIVATE BACKEND DETAIL'));
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('Unable to load the organization list.'),
    );
    expect(screen.queryByText(/No organizations/)).not.toBeInTheDocument();
    expect(screen.queryByText(/SYNTHETIC PRIVATE/)).not.toBeInTheDocument();
    expect(mocks.error).not.toHaveBeenCalled();
    mocks.list.mockResolvedValueOnce([]);
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    await waitFor(() => expect(screen.getByText(/No organizations/)).toBeInTheDocument());
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(1);
  });

  it.each([
    null,
    {},
    [org(), org()],
    [{ ...org(), org_id: ' ' }],
    [{ ...org(), org_id: ' org-a' }],
  ])('does not turn malformed inventory %j into usable rows', async (data) => {
    mocks.list.mockResolvedValueOnce(data);
    const { state } = mountState();
    await settle();
    expect(state.orgsError()).toBeTruthy();
    expect(state.orgs()).toEqual([]);
    expect(mocks.get).not.toHaveBeenCalled();
  });

  it('keeps a failed row unavailable without automatic retries as other rows finish, and offers a read-only reload', async () => {
    const other = deferred<BillingState>();
    mocks.list.mockResolvedValue([org(), org('org-b')]);
    mocks.get
      .mockRejectedValueOnce(new Error('SYNTHETIC PRIVATE BILLING DETAIL'))
      .mockReturnValueOnce(other.promise)
      .mockResolvedValue(billing());
    render(() => <BillingAdminPanel />);
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(BILLING_ADMIN_READ_ERROR),
    );
    other.resolve(billing());
    await settle();
    await settle();
    expect(mocks.get).toHaveBeenCalledTimes(2);
    expect(screen.queryByText(/SYNTHETIC PRIVATE/)).not.toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Organization org-a org-a' }));
    expect(screen.getByRole('status')).toHaveTextContent(BILLING_ADMIN_READ_ERROR);
    expect(screen.queryByText(/"loading": true/)).not.toBeInTheDocument();
    const before = mocks.put.mock.calls.length;
    await fireEvent.click(
      screen.getByRole('button', { name: 'Reload billing for Organization org-a' }),
    );
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
    expect(mocks.get).toHaveBeenCalledTimes(3);
    expect(mocks.put).toHaveBeenCalledTimes(before);
  });

  it.each([
    null,
    {},
    { ...billing(), capabilities: null },
    { ...billing(), limits: [] },
    { ...billing(), limits: { users: NaN } },
    { ...billing(), meters_enabled: 'all' },
    { ...billing(), subscription_state: '' },
  ])('rejects malformed billing %j rather than enabling a write', async (data) => {
    mocks.get.mockResolvedValueOnce(data);
    const { state } = mountState();
    await settle();
    expect(state.billingErrorByOrgID()['org-a']).toBe(BILLING_ADMIN_READ_ERROR);
    expect(state.billingByOrgID()['org-a']).toBeUndefined();
    await state.updateSubscriptionState('org-a', 'suspended');
    expect(mocks.put).not.toHaveBeenCalled();
    expect(mocks.get).toHaveBeenCalledTimes(1);
  });

  it.each(['success', 'failure'])(
    'ignores an obsolete row reload %s without ending its replacement spinner',
    async (outcome) => {
      const old = deferred<BillingState>();
      const current = deferred<BillingState>();
      mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
      const { state } = mountState();
      await settle();
      const reload = state.reloadOrganization('org-a');
      expect(signalAt(mocks.get, 0, 1).aborted).toBe(true);
      if (outcome === 'success') old.resolve(billing('obsolete'));
      else old.reject(new Error('obsolete'));
      await settle();
      expect(state.billingLoadingByOrgID()['org-a']).toBe(true);
      expect(state.billingByOrgID()['org-a']).toBeUndefined();
      expect(state.billingErrorByOrgID()['org-a']).toBeUndefined();
      expect(mocks.log).not.toHaveBeenCalled();
      current.resolve(billing('trialing'));
      await reload;
      expect(state.billingByOrgID()['org-a']?.subscription_state).toBe('trialing');
    },
  );

  it('ignores obsolete list success and failure across repeated refreshes', async () => {
    const first = deferred<ReturnType<typeof org>[]>();
    const second = deferred<ReturnType<typeof org>[]>();
    mocks.list
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)
      .mockResolvedValueOnce([org('current')]);
    const { state } = mountState();
    const middle = state.refreshOrganizations();
    await state.refreshOrganizations();
    expect(signalAt(mocks.list, 0).aborted).toBe(true);
    expect(signalAt(mocks.list, 1).aborted).toBe(true);
    first.resolve([org('obsolete')]);
    second.reject(new Error('obsolete'));
    await middle;
    await settle();
    expect(state.orgs().map((o) => o.org_id)).toEqual(['current']);
    expect(state.orgsError()).toBeNull();
    expect(mocks.log).not.toHaveBeenCalled();
    expect(mocks.get.mock.calls.map((c) => c[0])).toEqual(['current']);
  });

  it('does not let a previous inventory billing completion overwrite a new row with the same ID', async () => {
    const old = deferred<BillingState>();
    const current = deferred<BillingState>();
    mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
    const { state } = mountState();
    await settle();
    const refreshing = state.refreshOrganizations();
    await settle();
    current.resolve(billing('trialing'));
    await refreshing;
    old.resolve(billing('obsolete'));
    await settle();
    expect(state.billingByOrgID()['org-a']?.subscription_state).toBe('trialing');
    expect(signalAt(mocks.get, 0, 1).aborted).toBe(true);
  });

  it.each(['hosted', 'multi', 'hidden'])(
    'clears and invalidates %s withdrawal, then reads afresh on re-entry',
    async (kind) => {
      const old = deferred<ReturnType<typeof org>[]>();
      mocks.list.mockReturnValueOnce(old.promise);
      const { state } = mountState();
      const withdraw = () =>
        kind === 'hosted' ? setHosted(false) : kind === 'multi' ? setMulti(false) : setHidden(true);
      const restore = () =>
        kind === 'hosted' ? setHosted(true) : kind === 'multi' ? setMulti(true) : setHidden(false);
      withdraw();
      await settle();
      expect(state.orgs()).toEqual([]);
      expect(state.hostedEnabled()).toBe(false);
      expect(signalAt(mocks.list).aborted).toBe(true);
      restore();
      await settle();
      old.resolve([org('obsolete')]);
      await settle();
      expect(state.orgs().map((o) => o.org_id)).toEqual(['org-a']);
      expect(mocks.get.mock.calls.map((c) => c[0])).toEqual(['org-a']);
    },
  );

  it('bounds prefetch to six concurrent reads and stops the old pool from admitting queued rows', async () => {
    mocks.list.mockResolvedValue(Array.from({ length: 10 }, (_, i) => org(`org-${i}`)));
    const pending = Array.from({ length: 6 }, () => deferred<BillingState>());
    let n = 0;
    mocks.get.mockImplementation(() => pending[n++].promise);
    const { state } = mountState();
    await settle();
    expect(mocks.get).toHaveBeenCalledTimes(6);
    setHosted(false);
    await settle();
    pending.forEach((p) => p.resolve(billing()));
    await settle();
    await settle();
    expect(mocks.get).toHaveBeenCalledTimes(6);
    expect(state.orgs()).toEqual([]);
    mocks.get.mock.calls.forEach((_c, i) => expect(signalAt(mocks.get, i, 1).aborted).toBe(true));
  });

  it('disposal aborts reads, clears retained snapshots, and ignores later errors without notifications', async () => {
    const pending = deferred<BillingState>();
    mocks.get.mockReturnValueOnce(pending.promise);
    const { state, unmount } = mountState();
    await settle();
    unmount();
    expect(signalAt(mocks.get, 0, 1).aborted).toBe(true);
    pending.reject(new Error('late'));
    await settle();
    expect(state.orgs()).toEqual([]);
    expect(state.billingByOrgID()).toEqual({});
    expect(mocks.log).not.toHaveBeenCalled();
    expect(mocks.error).not.toHaveBeenCalled();
    await state.refreshOrganizations();
    expect(mocks.list).toHaveBeenCalledTimes(1);
  });

  it('disposal while listing cannot launch a billing read', async () => {
    const pending = deferred<ReturnType<typeof org>[]>();
    mocks.list.mockReturnValueOnce(pending.promise);
    const { unmount } = mountState();
    unmount();
    pending.resolve([org()]);
    await settle();
    expect(mocks.get).not.toHaveBeenCalled();
    expect(signalAt(mocks.list).aborted).toBe(true);
  });

  it('keeps opaque organization IDs separate from object prototypes', async () => {
    mocks.list.mockResolvedValue(['__proto__', 'constructor', 'toString'].map(org));
    const { state } = mountState();
    await settle();
    expect(mocks.get).toHaveBeenCalledTimes(3);
    for (const id of ['__proto__', 'constructor', 'toString'])
      expect(state.billingByOrgID()[id]?.subscription_state).toBe('active');
    await state.updateSubscriptionState('__proto__', 'suspended');
    expect(mocks.put.mock.calls[0][0]).toBe('__proto__');
    expect(Object.prototype).not.toHaveProperty('subscription_state');
  });
});

describe('Billing Admin writes require the current accepted snapshot', () => {
  it('does not fetch or write implicitly for an unread or unknown organization', async () => {
    const pending = deferred<BillingState>();
    mocks.get.mockReturnValueOnce(pending.promise);
    const { state } = mountState();
    await settle();
    await state.updateSubscriptionState('org-a', 'suspended');
    await state.updateSubscriptionState('foreign', 'active');
    expect(mocks.get).toHaveBeenCalledTimes(1);
    expect(mocks.put).not.toHaveBeenCalled();
    pending.resolve(billing());
  });

  it('deduplicates writes, preserves all saved billing fields and refuses a row reload while saving', async () => {
    const current = {
      ...billing(),
      plan_version: 'cloud_power',
      trial_started_at: 10,
      trial_ends_at: 20,
      stripe_customer_id: 'cus_fixture',
      stripe_subscription_id: 'sub_fixture',
      stripe_price_id: 'price_fixture',
    };
    mocks.get.mockResolvedValueOnce(current);
    const pending = deferred<BillingState>();
    mocks.put.mockReturnValueOnce(pending.promise);
    const { state } = mountState();
    await settle();
    const writing = state.updateSubscriptionState('org-a', 'suspended');
    await state.updateSubscriptionState('org-a', 'suspended');
    await state.reloadOrganization('org-a');
    expect(mocks.put).toHaveBeenCalledTimes(1);
    expect(mocks.get).toHaveBeenCalledTimes(1);
    expect(mocks.put.mock.calls[0][1]).toEqual({ ...current, subscription_state: 'suspended' });
    pending.resolve({ ...current, subscription_state: 'suspended' });
    await writing;
    expect(state.savingByOrgID()['org-a']).toBe(false);
    expect(mocks.success).toHaveBeenCalledTimes(1);
    await state.updateSubscriptionState('org-a', 'suspended');
    expect(mocks.put).toHaveBeenCalledTimes(1);
  });

  it.each(['rejected', 'malformed', 'mismatched'])(
    'withdraws the snapshot after an unconfirmed %s update without retrying or claiming rollback',
    async (kind) => {
      if (kind === 'rejected')
        mocks.put.mockRejectedValueOnce(new Error('SYNTHETIC PRIVATE UPDATE DETAIL'));
      else mocks.put.mockResolvedValueOnce(kind === 'mismatched' ? billing('active') : {});
      const { state } = mountState();
      await settle();
      await state.updateSubscriptionState('org-a', 'suspended');
      expect(state.billingByOrgID()['org-a']).toBeUndefined();
      expect(state.billingErrorByOrgID()['org-a']).toBe(BILLING_ADMIN_UNCONFIRMED_CHANGE);
      expect(mocks.success).not.toHaveBeenCalled();
      expect(mocks.error).not.toHaveBeenCalled();
      await state.updateSubscriptionState('org-a', 'active');
      expect(mocks.put).toHaveBeenCalledTimes(1);
      expect(mocks.get).toHaveBeenCalledTimes(1);
      mocks.get.mockResolvedValueOnce(billing('suspended'));
      await state.reloadOrganization('org-a');
      expect(state.billingErrorByOrgID()['org-a']).toBeUndefined();
      expect(state.billingByOrgID()['org-a']?.subscription_state).toBe('suspended');
    },
  );

  it.each(['refresh', 'read-only', 'dispose'])(
    'invalidates a pending write on %s without applying its completion to the new snapshot',
    async (kind) => {
      const pending = deferred<BillingState>();
      mocks.put.mockReturnValueOnce(pending.promise);
      const { state, unmount } = mountState();
      await settle();
      const writing = state.updateSubscriptionState('org-a', 'suspended');
      if (kind === 'refresh') {
        mocks.list.mockResolvedValueOnce([org('org-b')]);
        await state.refreshOrganizations();
      } else if (kind === 'read-only') {
        setReadOnly(true);
        await settle();
      } else unmount();
      expect(signalAt(mocks.put, 0, 2).aborted).toBe(true);
      pending.resolve(billing('suspended'));
      await writing;
      expect(mocks.success).not.toHaveBeenCalled();
      expect(mocks.error).not.toHaveBeenCalled();
      if (kind === 'dispose') expect(state.billingByOrgID()['org-a']).toBeUndefined();
      else if (kind === 'refresh') {
        expect(state.billingByOrgID()['org-a']).toBeUndefined();
        await state.updateSubscriptionState('org-a', 'suspended');
      } else {
        expect(state.billingByOrgID()['org-a']?.subscription_state).toBe('active');
        await state.updateSubscriptionState('org-a', 'suspended');
      }
      expect(mocks.put).toHaveBeenCalledTimes(1);
    },
  );
});
