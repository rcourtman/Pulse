import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { OrganizationAccessPanel } from '../OrganizationAccessPanel';
import { useOrganizationAccessPanelState } from '../useOrganizationAccessPanelState';
import { apiClient, clearApiToken, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';

const mocks = vi.hoisted(() => ({
  enabled: vi.fn(),
  readOnly: vi.fn(),
  hidden: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));
vi.mock('@/stores/license', () => ({ isMultiTenantEnabled: mocks.enabled }));
vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: mocks.readOnly,
  presentationPolicyHidesOrganizationSurfaces: mocks.hidden,
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: mocks.success, error: mocks.error },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
};
const fetchMock = vi.fn();
let owner: string;
const uncertainResults = ['unconfirmed', 'malformed', 'wrong-member', 'invalid-json'] as const;
let result: 'success' | (typeof uncertainResults)[number];
let finishWrite: ReturnType<typeof deferred<Response>> | undefined;
let setReadOnly: (value: boolean) => void;
let setEnabled: (value: boolean) => void;
let setHidden: (value: boolean) => void;
let targetPresent: boolean;
const members = () => [
  { userId: 'owner-a', role: owner === 'owner-a' ? 'owner' : 'admin', addedAt: '' },
  ...(targetPresent
    ? [{ userId: 'member-b', role: owner === 'member-b' ? 'owner' : 'viewer', addedAt: '' }]
    : []),
];
const writes = () => fetchMock.mock.calls.filter(([, init]) => init.method === 'POST');
const openPanel = (user = 'owner-a') =>
  render(() => <OrganizationAccessPanel currentUser={user} />);
const choose = async (origin: 'row' | 'form') => {
  await screen.findByRole('combobox', { name: 'Role for member-b' });
  if (origin === 'row')
    fireEvent.change(screen.getByRole('combobox', { name: 'Role for member-b' }), {
      target: { value: 'owner' },
    });
  else {
    fireEvent.input(screen.getByRole('textbox', { name: 'Invite user ID' }), {
      target: { value: 'member-b' },
    });
    fireEvent.change(screen.getByRole('combobox', { name: 'Invite role' }), {
      target: { value: 'owner' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Invite' }));
  }
};
const dialog = () => screen.getByRole('dialog', { name: 'Transfer organization ownership' });
const confirmTransfer = () =>
  fireEvent.click(within(dialog()).getByRole('button', { name: 'Transfer ownership' }));
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

beforeEach(() => {
  Object.values(mocks).forEach((mock) => mock.mockReset());
  const [readOnly, updateReadOnly] = createSignal(false);
  const [enabled, updateEnabled] = createSignal(true);
  const [hidden, updateHidden] = createSignal(false);
  setReadOnly = updateReadOnly;
  setEnabled = updateEnabled;
  setHidden = updateHidden;
  mocks.readOnly.mockImplementation(readOnly);
  mocks.enabled.mockImplementation(enabled);
  mocks.hidden.mockImplementation(hidden);
  owner = 'owner-a';
  result = 'success';
  finishWrite = undefined;
  targetPresent = true;
  window.sessionStorage.clear();
  window.localStorage.clear();
  clearApiToken();
  setOrgID('estate/east');
  setApiToken('synthetic-unrelated-token');
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url: string, init: RequestInit) => {
    const id = decodeURIComponent(url.split('/')[3] ?? '');
    if (url === '/api/org-invitations' || url.endsWith('/invitations')) return response([]);
    if (url.endsWith('/members')) {
      if (init.method !== 'POST') return response(members());
      if (finishWrite) return finishWrite.promise;
      owner = 'member-b';
      if (result === 'unconfirmed') return response({ error: 'Response lost' }, 500);
      if (result === 'malformed') return response({});
      if (result === 'wrong-member')
        return response({
          kind: 'member',
          member: { ...members()[1], userId: 'different-member' },
        });
      if (result === 'invalid-json') return new Response('not json');
      return response({ kind: 'member', member: members()[1] });
    }
    if (url.startsWith('/api/orgs/'))
      return response({ id, displayName: 'East estate', ownerUserId: owner, members: members() });
    throw new Error(`Unexpected synthetic request: ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  clearApiToken();
  setOrgID(null);
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('ownership transfer through the real organization client', () => {
  it.each(['row', 'form'] as const)(
    'requires deliberate review from the %s path, then sends one bound write and reads saved roles',
    async (origin) => {
      openPanel();
      await choose(origin);
      expect(writes()).toHaveLength(0);
      expect(dialog()).toHaveTextContent('East estate');
      expect(dialog()).toHaveTextContent('estate/east');
      expect(dialog()).toHaveTextContent('member-b');
      expect(dialog()).toHaveTextContent('owner-a');
      expect(dialog()).toHaveTextContent('Admin');
      expect(screen.getByRole('combobox', { name: 'Role for member-b', hidden: true })).toHaveValue(
        'viewer',
      );
      confirmTransfer();
      await waitFor(() => expect(writes()).toHaveLength(1));
      const [url, init] = writes()[0];
      expect(url).toBe('/api/orgs/estate%2Feast/members');
      expect(JSON.parse(init.body)).toEqual({ userId: 'member-b', role: 'owner' });
      expect(init).toMatchObject({
        credentials: 'include',
        signal: expect.any(AbortSignal),
        headers: { 'X-Pulse-Org-ID': 'estate/east', 'X-CSRF-Token': 'synthetic-csrf' },
      });
      expect(init.headers['X-API-Token']).toBeUndefined();
      await waitFor(() =>
        expect(screen.getByRole('combobox', { name: 'Role for member-b' })).toBeDisabled(),
      );
      expect(screen.getByRole('combobox', { name: 'Role for member-b' })).toHaveValue('owner');
      expect(screen.getByRole('combobox', { name: 'Role for owner-a' })).toHaveValue('admin');
      expect(
        within(screen.getByRole('combobox', { name: 'Invite role' })).queryByRole('option', {
          name: 'Owner',
        }),
      ).not.toBeInTheDocument();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    },
  );

  it.each(['row', 'form'] as const)(
    'cancels the %s path without a write or false saved role',
    async (origin) => {
      openPanel();
      await choose(origin);
      fireEvent.click(within(dialog()).getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.getByRole('combobox', { name: 'Role for member-b' })).toHaveValue('viewer');
      if (origin === 'form')
        expect(screen.getByRole('textbox', { name: 'Invite user ID' })).toHaveValue('member-b');
      expect(writes()).toHaveLength(0);
      expect(mocks.success).not.toHaveBeenCalled();
    },
  );

  it.each(
    (['row', 'form'] as const).flatMap((origin) =>
      uncertainResults.map((outcome) => ({ origin, outcome })),
    ),
  )(
    'checks saved access after a $outcome $origin transfer without replaying it',
    async ({ origin, outcome }) => {
      result = outcome;
      openPanel();
      await choose(origin);
      confirmTransfer();
      await screen.findByRole('alert');
      expect(screen.getByRole('alert')).toHaveTextContent('could not be confirmed');
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByRole('combobox', { name: 'Role for member-b' })).not.toBeInTheDocument();
      expect(writes()).toHaveLength(1);
      expect(mocks.success).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole('button', { name: 'Reload access' }));
      await screen.findByRole('combobox', { name: 'Role for member-b' });
      expect(screen.getByRole('combobox', { name: 'Role for member-b' })).toHaveValue('owner');
      expect(writes()).toHaveLength(1);
      if (origin === 'form') {
        expect(screen.getByRole('textbox', { name: 'Invite user ID' })).toHaveValue('member-b');
        expect(screen.getByRole('combobox', { name: 'Invite role' })).toHaveValue('owner');
        expect(screen.getByRole('option', { name: 'Owner (current owner only)' })).toBeDisabled();
        expect(screen.getByRole('button', { name: 'Invite' })).toBeDisabled();
      }
    },
  );

  it.each([
    'switch',
    'read-only',
    'capability',
    'hidden',
    'refresh',
    'principal',
    'dispose',
  ] as const)(
    'withdraws confirmation on %s, even if a retained confirm handler is called',
    async (withdrawal) => {
      const [user, setUser] = createSignal('owner-a');
      let state!: ReturnType<typeof useOrganizationAccessPanelState>;
      const { unmount } = render(() => {
        state = useOrganizationAccessPanelState({
          get currentUser() {
            return user();
          },
        });
        return null;
      });
      await waitFor(() => expect(state.loading()).toBe(false));
      await state.updateRole(state.members()[1], 'owner');
      expect(state.ownershipTransfer()).not.toBeNull();
      const transfer = state.ownershipTransfer()!;
      const retainedConfirm = () => state.confirmOwnershipTransfer(transfer);
      if (withdrawal === 'switch') {
        setOrgID('estate/west');
        eventBus.emit('org_switched', 'estate/west');
      }
      if (withdrawal === 'read-only') setReadOnly(true);
      if (withdrawal === 'capability') setEnabled(false);
      if (withdrawal === 'hidden') setHidden(true);
      if (withdrawal === 'refresh') eventBus.emit('organizations_changed');
      if (withdrawal === 'principal') setUser('other-owner');
      if (withdrawal === 'dispose') unmount();
      await settle();
      await retainedConfirm();
      expect(state.ownershipTransfer()).toBeNull();
      expect(writes()).toHaveLength(0);
    },
  );

  it('does not dispatch a confirmed transfer after selection changes during CSRF preparation', async () => {
    openPanel();
    await choose('row');
    const csrf = deferred<string>();
    vi.spyOn(apiClient as any, 'loadCSRFToken').mockReturnValueOnce(null);
    vi.spyOn(apiClient as any, 'ensureCSRFToken').mockReturnValueOnce(csrf.promise);
    confirmTransfer();
    setOrgID('estate/west');
    eventBus.emit('org_switched', 'estate/west');
    csrf.resolve('synthetic-csrf');
    await settle();
    expect(writes()).toHaveLength(0);
    expect(mocks.success).not.toHaveBeenCalled();
  });

  it('cannot reuse a confirmation for a different invite draft or a refreshed member', async () => {
    let state!: ReturnType<typeof useOrganizationAccessPanelState>;
    render(() => {
      state = useOrganizationAccessPanelState({ currentUser: 'owner-a' });
      return null;
    });
    await waitFor(() => expect(state.loading()).toBe(false));
    state.setInviteUserID('member-b');
    state.setInviteRole('owner');
    await state.inviteMember();
    const draftTransfer = state.ownershipTransfer()!;
    state.setInviteUserID('another-user');
    await state.confirmOwnershipTransfer(draftTransfer);
    expect(writes()).toHaveLength(0);
    const stale = state.members()[1];
    await state.updateRole(stale, 'owner');
    const staleTransfer = state.ownershipTransfer()!;
    targetPresent = false;
    await state.loadOrganizationAccess();
    await state.confirmOwnershipTransfer(staleTransfer);
    await state.updateRole(stale, 'owner');
    expect(writes()).toHaveLength(0);
  });

  it('does not let a stale cancel or confirmation consume a newer intent', async () => {
    let state!: ReturnType<typeof useOrganizationAccessPanelState>;
    render(() => {
      state = useOrganizationAccessPanelState({ currentUser: 'owner-a' });
      return null;
    });
    await waitFor(() => expect(state.loading()).toBe(false));
    await state.updateRole(state.members()[1], 'owner');
    const old = state.ownershipTransfer()!;
    state.cancelOwnershipTransfer(old);
    await state.updateRole(state.members()[1], 'owner');
    const current = state.ownershipTransfer()!;
    state.cancelOwnershipTransfer(old);
    await state.confirmOwnershipTransfer(old);
    expect(state.ownershipTransfer()).toBe(current);
    expect(writes()).toHaveLength(0);
    await state.confirmOwnershipTransfer(current);
    expect(writes()).toHaveLength(1);
  });

  it('serializes a confirmed transfer and suppresses a late completion after withdrawal', async () => {
    finishWrite = deferred<Response>();
    openPanel();
    await choose('row');
    confirmTransfer();
    await waitFor(() => expect(writes()).toHaveLength(1));
    const signal = writes()[0][1].signal;
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Role for member-b' })).toBeDisabled();
    setReadOnly(true);
    await settle();
    expect(signal.aborted).toBe(true);
    finishWrite.resolve(response({ kind: 'member', member: members()[1] }));
    await settle();
    expect(writes()).toHaveLength(1);
    expect(mocks.success).not.toHaveBeenCalled();
  });

  it('does not offer or dispatch a transfer to a non-member, to self, or from a non-owner', async () => {
    let state!: ReturnType<typeof useOrganizationAccessPanelState>;
    render(() => {
      state = useOrganizationAccessPanelState({ currentUser: 'owner-a' });
      return null;
    });
    await waitFor(() => expect(state.loading()).toBe(false));
    state.setInviteRole('owner');
    state.setInviteUserID('absent-user');
    await state.inviteMember();
    expect(mocks.error).toHaveBeenCalledWith(
      'Ownership can only be transferred to an existing member.',
    );
    state.setInviteUserID('owner-a');
    await state.inviteMember();
    expect(state.ownershipTransfer()).toBeNull();
    expect(writes()).toHaveLength(0);
    owner = 'member-b';
    await state.loadOrganizationAccess();
    state.setInviteUserID('owner-a');
    state.setInviteRole('owner');
    await state.inviteMember();
    await state.updateRole(state.members()[0], 'owner');
    expect(state.ownershipTransfer()).toBeNull();
    expect(writes()).toHaveLength(0);
  });
});
