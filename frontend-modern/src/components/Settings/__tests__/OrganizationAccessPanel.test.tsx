import { createSignal } from 'solid-js';
import { useOrganizationAccessPanelState } from '../useOrganizationAccessPanelState';
import type { OrganizationMember } from '@/api/orgs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { OrganizationAccessPanel } from '../OrganizationAccessPanel';
import organizationAccessStateSource from '../useOrganizationAccessPanelState.ts?raw';

const orgGetMock = vi.fn();
const listMembersMock = vi.fn();
const listPendingInvitationsMock = vi.fn();
const listMyInvitationsMock = vi.fn();
const updateMemberRoleMock = vi.fn();
const inviteMemberMock = vi.fn();
const acceptMyInvitationMock = vi.fn();
const declineMyInvitationMock = vi.fn();
const revokeInvitationMock = vi.fn();
const removeMemberMock = vi.fn();
const isMultiTenantEnabledMock = vi.fn();
const getOrgIDMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const eventBusOnMock = vi.fn();
const eventBusEmitMock = vi.fn();
const loggerErrorMock = vi.fn();
const readOnlyMock = vi.fn();
const hideOrganizationsMock = vi.fn();
let setEnabled: (enabled: boolean) => void;
let setReadOnly: (readOnly: boolean) => void;
const listeners = new Map<string, Set<() => void>>();

vi.mock('@/api/orgs', () => ({
  OrgsAPI: {
    get: (...args: unknown[]) => orgGetMock(...args),
    listMembers: (...args: unknown[]) => listMembersMock(...args),
    listPendingInvitations: (...args: unknown[]) => listPendingInvitationsMock(...args),
    listMyInvitations: (...args: unknown[]) => listMyInvitationsMock(...args),
    updateMemberRole: (...args: unknown[]) => updateMemberRoleMock(...args),
    inviteMember: (...args: unknown[]) => inviteMemberMock(...args),
    acceptMyInvitation: (...args: unknown[]) => acceptMyInvitationMock(...args),
    declineMyInvitation: (...args: unknown[]) => declineMyInvitationMock(...args),
    revokeInvitation: (...args: unknown[]) => revokeInvitationMock(...args),
    removeMember: (...args: unknown[]) => removeMemberMock(...args),
  },
}));

vi.mock('@/stores/license', () => ({
  isMultiTenantEnabled: (...args: unknown[]) => isMultiTenantEnabledMock(...args),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesOrganizationSurfaces: () => hideOrganizationsMock(),
  presentationPolicyIsReadOnly: () => readOnlyMock(),
}));

vi.mock('@/utils/apiClient', () => ({
  getOrgID: (...args: unknown[]) => getOrgIDMock(...args),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: (...args: unknown[]) => eventBusOnMock(...args),
    emit: (...args: unknown[]) => eventBusEmitMock(...args),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
  },
}));

const baseOrg = {
  id: 'org-a',
  displayName: 'Organization A',
  ownerUserId: 'user-1',
};

const baseMembers = [
  {
    userId: 'user-1',
    role: 'owner',
    addedAt: '2024-01-10T00:00:00Z',
  },
  {
    userId: 'user-2',
    role: 'viewer',
    addedAt: '2024-01-11T00:00:00Z',
  },
];

const renderPanel = () => render(() => <OrganizationAccessPanel currentUser="user-1" />);

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

beforeEach(() => {
  orgGetMock.mockReset();
  listMembersMock.mockReset();
  listPendingInvitationsMock.mockReset();
  listMyInvitationsMock.mockReset();
  updateMemberRoleMock.mockReset();
  inviteMemberMock.mockReset();
  acceptMyInvitationMock.mockReset();
  declineMyInvitationMock.mockReset();
  revokeInvitationMock.mockReset();
  removeMemberMock.mockReset();
  isMultiTenantEnabledMock.mockReset();
  getOrgIDMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  eventBusOnMock.mockReset();
  eventBusEmitMock.mockReset();
  loggerErrorMock.mockReset();

  const [enabled, updateEnabled] = createSignal(true);
  const [readOnly, updateReadOnly] = createSignal(false);
  setEnabled = updateEnabled;
  setReadOnly = updateReadOnly;
  isMultiTenantEnabledMock.mockImplementation(enabled);
  readOnlyMock.mockImplementation(readOnly);
  hideOrganizationsMock.mockReturnValue(false);
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );
  getOrgIDMock.mockReturnValue('org-a');
  listeners.clear();
  eventBusOnMock.mockImplementation((event, callback) => {
    if (!listeners.has(event)) listeners.set(event, new Set());
    listeners.get(event)!.add(callback);
    return () => listeners.get(event)?.delete(callback);
  });
  eventBusEmitMock.mockImplementation((event) => {
    listeners.get(event)?.forEach((callback) => callback());
  });

  orgGetMock.mockResolvedValue(baseOrg);
  listMembersMock.mockResolvedValue(baseMembers);
  listPendingInvitationsMock.mockResolvedValue([]);
  listMyInvitationsMock.mockResolvedValue([]);
  updateMemberRoleMock.mockResolvedValue(undefined);
  inviteMemberMock.mockResolvedValue({
    kind: 'invitation',
    invitation: { userId: 'new-user', role: 'viewer' },
  });
  acceptMyInvitationMock.mockResolvedValue({ kind: 'member' });
  declineMyInvitationMock.mockResolvedValue(undefined);
  revokeInvitationMock.mockResolvedValue(undefined);
  removeMemberMock.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('OrganizationAccessPanel', () => {
  it('renders loading skeleton first, then loaded content', async () => {
    const orgDeferred = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(orgDeferred.promise);

    const { container } = renderPanel();

    expect(container.querySelector('.animate-pulse')).toBeTruthy();
    expect(screen.queryByRole('heading', { name: 'Invite Member' })).not.toBeInTheDocument();

    orgDeferred.resolve(baseOrg);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Invite Member' })).toBeInTheDocument();
    });
  });

  it.each(['role', 'remove'] as const)(
    'admits the real datagrid proxy for a current %s change',
    async (operation) => {
      renderPanel();
      await screen.findByText('user-2');
      if (operation === 'role') {
        fireEvent.change(screen.getByRole('combobox', { name: 'Role for user-2' }), {
          target: { value: 'editor' },
        });
        await waitFor(() =>
          expect(updateMemberRoleMock).toHaveBeenCalledWith(
            'org-a',
            { userId: 'user-2', role: 'editor' },
            expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
          ),
        );
      } else {
        fireEvent.click(screen.getByRole('button', { name: 'Remove' }));
        await waitFor(() =>
          expect(removeMemberMock).toHaveBeenCalledWith(
            'org-a',
            'user-2',
            expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
          ),
        );
      }
    },
  );

  it('normalizes org scope through the shared helper', () => {
    expect(organizationAccessStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationAccessStateSource).not.toContain("getOrgID() || 'default'");
  });
});

describe('Organization Access selection safety', () => {
  it('does not restore the previous organization after a failed switch, and offers a read-only retry', async () => {
    renderPanel();
    await screen.findByText('user-2');
    getOrgIDMock.mockReturnValue('org-b');
    orgGetMock.mockRejectedValueOnce(new Error('Forbidden'));
    eventBusEmitMock('org_switched');
    await waitFor(() => expect(loggerErrorMock).toHaveBeenCalled());
    expect(screen.queryByText('user-2')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Invite' })).not.toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('org-b');
    expect(screen.getByRole('button', { name: 'Reload access' })).toBeInTheDocument();
  });

  it('does not let a late old read replace the current organization', async () => {
    const oldMembers = deferred<typeof baseMembers>();
    listMembersMock.mockReturnValueOnce(oldMembers.promise);
    renderPanel();
    await waitFor(() => expect(listMembersMock).toHaveBeenCalledTimes(1));
    getOrgIDMock.mockReturnValue('org-b');
    orgGetMock.mockResolvedValueOnce({ ...baseOrg, id: 'org-b', displayName: 'Organization B' });
    listMembersMock.mockResolvedValueOnce([{ ...baseMembers[1], userId: 'member-b' }]);
    eventBusEmitMock('org_switched');
    await screen.findByText('member-b');
    oldMembers.resolve(baseMembers);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.getByText('member-b')).toBeInTheDocument();
    expect(screen.queryByText('user-2')).not.toBeInTheDocument();
  });
});

const pendingInvitation = {
  userId: 'invited-user',
  role: 'viewer',
  invitedAt: '2026-10-01T00:00:00Z',
  invitedBy: 'user-1',
};
const personalInvitation = {
  ...pendingInvitation,
  userId: 'user-1',
  orgId: 'inviting-org',
  orgDisplayName: 'Inviting Organization',
};
const renderState = () => {
  let state!: ReturnType<typeof useOrganizationAccessPanelState>;
  const view = render(() => {
    state = useOrganizationAccessPanelState({ currentUser: 'user-1' });
    return <div />;
  });
  return { state, ...view };
};
const ready = async (state: ReturnType<typeof useOrganizationAccessPanelState>) => {
  await waitFor(() => expect(state.loading()).toBe(false));
  expect(state.loadError()).toBeNull();
};
const writes = () => [
  updateMemberRoleMock,
  inviteMemberMock,
  acceptMyInvitationMock,
  declineMyInvitationMock,
  revokeInvitationMock,
  removeMemberMock,
];
const expectNoWrites = () => writes().forEach((write) => expect(write).not.toHaveBeenCalled());

describe('Organization Access read admission', () => {
  it('keeps first-read failure distinct from accepted empty membership, and retries only reads', async () => {
    orgGetMock.mockRejectedValueOnce(new Error('Unavailable'));
    renderPanel();
    await screen.findByRole('alert');
    expect(screen.queryByText('No organization members found.')).not.toBeInTheDocument();
    orgGetMock.mockResolvedValue(baseOrg);
    listMembersMock.mockResolvedValue([]);
    fireEvent.click(screen.getByRole('button', { name: 'Reload access' }));
    await screen.findByText('No organization members found.');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expectNoWrites();
  });

  it('preserves only the same-selection invitation draft through a failed refresh and reload', async () => {
    renderPanel();
    const input = await screen.findByRole('textbox', { name: 'Invite user ID' });
    fireEvent.input(input, { target: { value: 'unsent-user' } });
    orgGetMock.mockRejectedValueOnce(new Error('Unavailable'));
    eventBusEmitMock('organizations_changed');
    await screen.findByRole('alert');
    expect(screen.queryByText('user-2')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Reload access' }));
    expect(await screen.findByRole('textbox', { name: 'Invite user ID' })).toHaveValue(
      'unsent-user',
    );
    expectNoWrites();
  });

  it('does not follow an obsolete organization lookup with membership reads', async () => {
    const oldOrg = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(oldOrg.promise);
    const { state } = renderState();
    getOrgIDMock.mockReturnValue('org-b');
    orgGetMock.mockResolvedValueOnce({ ...baseOrg, id: 'org-b' });
    eventBusEmitMock('org_switched');
    await ready(state);
    oldOrg.resolve(baseOrg);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(listMembersMock.mock.calls.map(([orgId]) => orgId)).toEqual(['org-b']);
    expect(state.org()?.id).toBe('org-b');
  });

  it.each(['late success', 'late failure'])(
    'ignores %s from the first read after A → B → A',
    async (result) => {
      const first = deferred<typeof baseMembers>();
      listMembersMock.mockReturnValueOnce(first.promise);
      const { state } = renderState();
      await waitFor(() => expect(listMembersMock).toHaveBeenCalledTimes(1));
      getOrgIDMock.mockReturnValue('org-b');
      orgGetMock.mockResolvedValueOnce({ ...baseOrg, id: 'org-b' });
      eventBusEmitMock('org_switched');
      await ready(state);
      getOrgIDMock.mockReturnValue('org-a');
      listMembersMock.mockResolvedValueOnce([{ ...baseMembers[1], userId: 'new-snapshot' }]);
      eventBusEmitMock('org_switched');
      await ready(state);
      if (result === 'late success') first.resolve(baseMembers);
      else first.reject(new Error('Old selection failed'));
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(state.members().map((member) => member.userId)).toEqual(['new-snapshot']);
      expect(state.loadError()).toBeNull();
      expect(loggerErrorMock).not.toHaveBeenCalled();
    },
  );

  it('rejects a different organization returned by the server without reading its members', async () => {
    orgGetMock.mockResolvedValueOnce({ ...baseOrg, id: 'different-org' });
    const { state } = renderState();
    await waitFor(() => expect(state.loadError()).not.toBeNull());
    expect(state.org()).toBeNull();
    expect(listMembersMock).not.toHaveBeenCalled();
    expectNoWrites();
  });

  it.each(['members', 'pending', 'personal'] as const)(
    'does not admit a malformed %s list as empty',
    async (list) => {
      const request = {
        members: listMembersMock,
        pending: listPendingInvitationsMock,
        personal: listMyInvitationsMock,
      }[list];
      request.mockResolvedValueOnce(null);
      const { state } = renderState();
      await waitFor(() => expect(state.loadError()).not.toBeNull());
      expect(state.org()).toBeNull();
      expect(state.canManageCurrentOrg()).toBe(false);
    },
  );

  it('binds every admitted read to the exact selected organization with one abort signal', async () => {
    const { state } = renderState();
    await ready(state);
    for (const request of [orgGetMock, listMembersMock, listPendingInvitationsMock]) {
      expect(request).toHaveBeenCalledWith(
        'org-a',
        expect.objectContaining({
          expectedOrgID: 'org-a',
          retry: false,
          signal: expect.any(AbortSignal),
        }),
      );
    }
    expect(listMyInvitationsMock).toHaveBeenCalledWith(
      expect.objectContaining({
        expectedOrgID: 'org-a',
        retry: false,
        signal: expect.any(AbortSignal),
      }),
    );
  });

  it('normalizes deliberate default selection without accepting another payload identity', async () => {
    getOrgIDMock.mockReturnValue(null);
    orgGetMock.mockResolvedValue({ ...baseOrg, id: 'default', ownerUserId: undefined });
    const { state } = renderState();
    await ready(state);
    expect(state.org()?.id).toBe('default');
    expect(orgGetMock.mock.calls[0][1].expectedOrgID).toBe('default');
    expect(state.canManageCurrentOrg()).toBe(false);
  });
});

describe('Organization Access change safety', () => {
  beforeEach(() => {
    listPendingInvitationsMock.mockResolvedValue([pendingInvitation]);
    listMyInvitationsMock.mockResolvedValue([personalInvitation]);
  });

  const exerciseAllChanges = async (
    state: ReturnType<typeof useOrganizationAccessPanelState>,
    member: OrganizationMember,
  ) => {
    state.setInviteUserID('new-user');
    await state.inviteMember();
    await state.updateRole(member, 'editor');
    await state.removeMember(member);
    await state.revokeInvitation('invited-user');
    await state.acceptInvitation('inviting-org');
    await state.declineInvitation('inviting-org');
  };

  it.each(['pending', 'failed', 'switched without event', 'read-only', 'disabled'])(
    'rejects every retained mutation handler when %s',
    async (reason) => {
      const { state } = renderState();
      await ready(state);
      const member = state.members()[1];
      if (reason === 'switched without event') getOrgIDMock.mockReturnValue('org-b');
      if (reason === 'read-only') setReadOnly(true);
      if (reason === 'disabled') setEnabled(false);
      if (reason === 'pending' || reason === 'failed') {
        const lookup = deferred<typeof baseOrg>();
        orgGetMock.mockReturnValueOnce(lookup.promise);
        eventBusEmitMock('organizations_changed');
        if (reason === 'failed') {
          lookup.reject(new Error('Unavailable'));
          await waitFor(() => expect(state.loadError()).not.toBeNull());
        }
      }
      await exerciseAllChanges(state, member);
      expectNoWrites();
      expect(confirm).not.toHaveBeenCalled();
    },
  );

  it.each(['invite', 'role', 'remove', 'revoke', 'accept', 'decline'] as const)(
    'uses exactly one snapshot-bound %s mutation and blocks concurrent changes',
    async (operation) => {
      const { state } = renderState();
      await ready(state);
      const member = state.members()[1];
      const result = deferred<any>();
      const request = {
        invite: inviteMemberMock,
        role: updateMemberRoleMock,
        remove: removeMemberMock,
        revoke: revokeInvitationMock,
        accept: acceptMyInvitationMock,
        decline: declineMyInvitationMock,
      }[operation];
      request.mockReturnValueOnce(result.promise);
      state.setInviteUserID('new-user');
      const action = {
        invite: () => state.inviteMember(),
        role: () => state.updateRole(member, 'editor'),
        remove: () => state.removeMember(member),
        revoke: () => state.revokeInvitation('invited-user'),
        accept: () => state.acceptInvitation('inviting-org'),
        decline: () => state.declineInvitation('inviting-org'),
      }[operation];
      const pending = action();
      expect(state.saving()).toBe(true);
      await exerciseAllChanges(state, member);
      expect(writes().reduce((sum, write) => sum + write.mock.calls.length, 0)).toBe(1);
      expect(request.mock.calls[0].at(-1)).toMatchObject({
        expectedOrgID: 'org-a',
        retry: false,
        signal: expect.any(AbortSignal),
      });
      expect(request.mock.calls[0][0]).toBe(
        operation === 'accept' || operation === 'decline' ? 'inviting-org' : 'org-a',
      );
      result.resolve({ kind: 'member', member });
      await pending;
      await ready(state);
      expect(notificationSuccessMock).toHaveBeenCalledTimes(1);
      expect(orgGetMock).toHaveBeenCalledTimes(2);
    },
  );

  it('requires a read after a failed mutation, retaining the draft without replaying the change', async () => {
    const { state } = renderState();
    await ready(state);
    state.setInviteUserID('new-user');
    inviteMemberMock.mockRejectedValueOnce(new Error('Connection lost'));
    await state.inviteMember();
    expect(state.loadError()).toMatch(/could not be confirmed/);
    expect(state.inviteUserID()).toBe('new-user');
    await state.inviteMember();
    expect(inviteMemberMock).toHaveBeenCalledTimes(1);
    expect(orgGetMock).toHaveBeenCalledTimes(1);
    await state.loadOrganizationAccess();
    await ready(state);
    expect(inviteMemberMock).toHaveBeenCalledTimes(1);
    expect(state.inviteUserID()).toBe('new-user');
  });

  it('does not erase a newer draft on successful invite', async () => {
    const result = deferred<any>();
    inviteMemberMock.mockReturnValueOnce(result.promise);
    const { state } = renderState();
    await ready(state);
    state.setInviteUserID('first-user');
    const pending = state.inviteMember();
    state.setInviteUserID('next-user');
    result.resolve({ kind: 'member' });
    await pending;
    await ready(state);
    expect(state.inviteUserID()).toBe('next-user');
  });

  it.each(['success', 'failure'])(
    'ignores old mutation %s after switching, without clearing the new draft or pending read',
    async (outcome) => {
      const result = deferred<any>();
      inviteMemberMock.mockReturnValueOnce(result.promise);
      const { state } = renderState();
      await ready(state);
      state.setInviteUserID('old-user');
      const pending = state.inviteMember();
      const signal = inviteMemberMock.mock.calls[0][2].signal;
      const lookup = deferred<typeof baseOrg>();
      orgGetMock.mockReturnValueOnce(lookup.promise);
      getOrgIDMock.mockReturnValue('org-b');
      eventBusEmitMock('org_switched');
      expect(signal.aborted).toBe(true);
      expect(state.inviteUserID()).toBe('');
      state.setInviteUserID('new-draft');
      if (outcome === 'success') result.resolve({ kind: 'member' });
      else result.reject(new Error('Old change failed'));
      await pending;
      expect(state.loading()).toBe(true);
      expect(state.inviteUserID()).toBe('new-draft');
      expect(notificationSuccessMock).not.toHaveBeenCalled();
      expect(notificationErrorMock).not.toHaveBeenCalled();
      lookup.resolve({ ...baseOrg, id: 'org-b' });
      await ready(state);
      expect(inviteMemberMock).toHaveBeenCalledTimes(1);
    },
  );

  it('does not accept a stale member object or absent invitation after a same-organization reload', async () => {
    const { state } = renderState();
    await ready(state);
    const staleMember = state.members()[1];
    listMembersMock.mockResolvedValue(baseMembers.map((member) => ({ ...member })));
    listPendingInvitationsMock.mockResolvedValue([]);
    listMyInvitationsMock.mockResolvedValue([]);
    await state.loadOrganizationAccess();
    await state.updateRole(staleMember, 'editor');
    await state.removeMember(staleMember);
    await state.revokeInvitation('invited-user');
    await state.acceptInvitation('inviting-org');
    await state.declineInvitation('inviting-org');
    expectNoWrites();
  });

  it('rechecks a remove confirmation against the selection that supplied it', async () => {
    const { state } = renderState();
    await ready(state);
    vi.mocked(confirm).mockImplementationOnce(() => {
      getOrgIDMock.mockReturnValue('org-b');
      return true;
    });
    await state.removeMember(state.members()[1]);
    expect(removeMemberMock).not.toHaveBeenCalled();
  });

  it('preserves the owner and existing-member ownership transfer rules', async () => {
    const { state } = renderState();
    await ready(state);
    await state.removeMember(state.members()[0]);
    await state.updateRole(state.members()[0], 'viewer');
    state.setInviteUserID('not-a-member');
    state.setInviteRole('owner');
    await state.inviteMember();
    expectNoWrites();
    expect(notificationErrorMock).toHaveBeenCalledWith(
      'Ownership can only be transferred to an existing member.',
    );
  });

  it('allows a viewer to answer their own invitation but not manage members or pending invitations', async () => {
    orgGetMock.mockResolvedValue({
      ...baseOrg,
      ownerUserId: 'someone-else',
      members: baseMembers.map((member) => ({ ...member, role: 'viewer' })),
    });
    const { state } = renderState();
    await ready(state);
    expect(state.canManageCurrentOrg()).toBe(false);
    expect(listPendingInvitationsMock).not.toHaveBeenCalled();
    state.setInviteUserID('new-user');
    await state.inviteMember();
    await state.updateRole(state.members()[1], 'admin');
    await state.removeMember(state.members()[1]);
    expect(inviteMemberMock).not.toHaveBeenCalled();
    expect(updateMemberRoleMock).not.toHaveBeenCalled();
    expect(removeMemberMock).not.toHaveBeenCalled();
    await state.acceptInvitation('inviting-org');
    expect(acceptMyInvitationMock).toHaveBeenCalledTimes(1);
  });

  it('shows a read-only session notice instead of presenting a false role requirement', async () => {
    setReadOnly(true);
    renderPanel();
    await screen.findByText('Organization access is read-only for this session.');
    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Decline' })).toBeDisabled();
    expect(screen.queryByRole('textbox', { name: 'Invite user ID' })).not.toBeInTheDocument();
    expect(
      screen.queryByText('Admin or owner role required to manage organization access.'),
    ).not.toBeInTheDocument();
  });
});

describe('Organization Access lifetime', () => {
  it('invalidates reads on capability loss and reads afresh when the capability returns', async () => {
    const oldMembers = deferred<typeof baseMembers>();
    listMembersMock.mockReturnValueOnce(oldMembers.promise);
    const { state } = renderState();
    await waitFor(() => expect(listMembersMock).toHaveBeenCalledTimes(1));
    const signal = listMembersMock.mock.calls[0][1].signal;
    setEnabled(false);
    await waitFor(() => expect(state.loading()).toBe(false));
    expect(signal.aborted).toBe(true);
    oldMembers.resolve(baseMembers);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(state.org()).toBeNull();
    setEnabled(true);
    await ready(state);
    expect(orgGetMock).toHaveBeenCalledTimes(2);
    expect(state.members()).toEqual(baseMembers);
  });

  it('invalidates an older identity and clears its draft even in the same organization', async () => {
    const [currentUser, setCurrentUser] = createSignal('user-1');
    render(() => <OrganizationAccessPanel currentUser={currentUser()} />);
    fireEvent.input(await screen.findByRole('textbox', { name: 'Invite user ID' }), {
      target: { value: 'private-draft' },
    });
    const pending = deferred<typeof baseMembers>();
    listMembersMock.mockReturnValueOnce(pending.promise);
    eventBusEmitMock('organizations_changed');
    await waitFor(() => expect(listMembersMock).toHaveBeenCalledTimes(2));
    orgGetMock.mockResolvedValue({ ...baseOrg, ownerUserId: 'user-b' });
    setCurrentUser('user-b');
    expect(await screen.findByRole('textbox', { name: 'Invite user ID' })).toHaveValue('');
    pending.reject(new Error('Old identity failed'));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('aborts reads and unsubscribes on disposal without emitting late errors', async () => {
    const pending = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(pending.promise);
    const { unmount } = renderState();
    const signal = orgGetMock.mock.calls[0][1].signal;
    unmount();
    expect(signal.aborted).toBe(true);
    pending.reject(new Error('Disposed read'));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(loggerErrorMock).not.toHaveBeenCalled();
    eventBusEmitMock('org_switched');
    expect(orgGetMock).toHaveBeenCalledTimes(1);
  });
});
