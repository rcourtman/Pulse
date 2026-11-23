import { createSignal } from 'solid-js';
import { useOrganizationAccessPanelState } from '../useOrganizationAccessPanelState';
import type { OrganizationMember } from '@/api/orgs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { OrganizationAccessPanel } from '../OrganizationAccessPanel';

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
let setHidden: (hidden: boolean) => void;
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
  const [hidden, updateHidden] = createSignal({ value: false });
  setHidden = (value) => updateHidden({ value });
  hideOrganizationsMock.mockImplementation(() => hidden().value);
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
  it('distinguishes a pending invitation from the proposed role in the draft', async () => {
    const userId = 'new-user';
    listPendingInvitationsMock.mockResolvedValue([
      { userId, role: 'editor', invitedAt: '2026-11-03T00:00:00Z' },
    ]);
    renderPanel();

    const input = await screen.findByRole('textbox', { name: 'Member user ID' });
    input.focus();
    fireEvent.input(input, { target: { value: `  ${userId}  ` } });
    const form = screen.getByRole('form', { name: 'Update pending invitation' });
    const role = within(form).getByRole('combobox', { name: 'Member role' });
    expect(input).toHaveFocus();
    expect(role).toHaveValue('viewer');
    expect(role).toHaveAccessibleDescription(
      'Last read: Editor, pending acceptance. If already accepted, the role changes immediately.',
    );
    expect(within(form).getByRole('button', { name: 'Save role' })).toBeEnabled();
    const pending = within(screen.getByRole('group', { name: `Pending invitation for ${userId}` }));
    expect(pending.getByText('editor', { exact: true })).toBeVisible();
    expect(pending.getByRole('button', { name: `Revoke invitation for ${userId}` })).toBeEnabled();
    expect(inviteMemberMock).not.toHaveBeenCalled();
  });

  it.each(['org-a', 'default'])(
    'uses native required-field recovery before submitting an empty %s member draft',
    async (orgID) => {
      getOrgIDMock.mockReturnValue(orgID);
      orgGetMock.mockResolvedValue({ ...baseOrg, id: orgID, canManageMembers: true });
      renderPanel();

      const input = (await screen.findByRole('textbox', {
        name: orgID === 'default' ? 'User ID to add' : 'Member user ID',
      })) as HTMLInputElement;
      const form = screen.getByRole('form', {
        name: orgID === 'default' ? 'Add member' : 'Invite member',
      }) as HTMLFormElement;
      const invalid = vi.fn();
      input.addEventListener('invalid', invalid);
      expect(input).toBeRequired();
      expect(input.validity.valueMissing).toBe(true);

      form.requestSubmit();
      expect(invalid).toHaveBeenCalledTimes(1);
      expect(inviteMemberMock).not.toHaveBeenCalled();
      expect(notificationErrorMock).not.toHaveBeenCalled();
      expect(listMembersMock).toHaveBeenCalledTimes(1);
      expect(screen.getByRole('combobox', { name: 'Member role' })).toHaveValue('viewer');

      fireEvent.input(input, { target: { value: '  new-user  ' } });
      expect(input.validity.valid).toBe(true);
      form.requestSubmit();
      await waitFor(() => {
        expect(inviteMemberMock).toHaveBeenCalledExactlyOnceWith(
          orgID,
          { userId: 'new-user', role: 'viewer' },
          expect.objectContaining({ expectedOrgID: orgID, retry: false }),
        );
      });
      expect(invalid).toHaveBeenCalledTimes(1);
    },
  );

  const pendingInvitations = () =>
    [
      'provisioningserviceaccountfortheoperationsdepartmentanddisasterrecoveryteam@example.test',
      'analyst@example.test',
    ].map((userId) => ({
      userId,
      role: 'editor' as const,
      invitedBy: 'centralidentitymanagementadministratorfortheoperationsdepartment@example.test',
      invitedAt: '2026-11-01T12:00:00Z',
    }));

  it('identifies each pending invitation and its revocation action without shortening identities', async () => {
    listPendingInvitationsMock.mockResolvedValue(pendingInvitations());
    renderPanel();

    for (const invitation of pendingInvitations()) {
      const group = await screen.findByRole('group', {
        name: `Pending invitation for ${invitation.userId}`,
      });
      expect(within(group).getByText(invitation.userId, { exact: true })).toBeVisible();
      expect(group).toHaveTextContent(`Invited by ${invitation.invitedBy}`);
      expect(within(group).getByText('editor', { exact: true })).toBeVisible();
      expect(
        within(group).getByRole('button', {
          name: `Revoke invitation for ${invitation.userId}`,
        }),
      ).toBeEnabled();
    }
    expectNoWrites();
  });

  it.each([0, 1])(
    'revokes only the invitation named by action %s and rereads saved access',
    async (index) => {
      let invitations = pendingInvitations();
      const target = invitations[index].userId;
      const retained = invitations[1 - index].userId;
      listPendingInvitationsMock.mockImplementation(async () => invitations);
      revokeInvitationMock.mockImplementation(async (_orgId, userId) => {
        invitations = invitations.filter((invitation) => invitation.userId !== userId);
      });
      renderPanel();

      fireEvent.click(
        await screen.findByRole('button', {
          name: `Revoke invitation for ${target}`,
        }),
      );

      await waitFor(() =>
        expect(
          screen.queryByRole('group', {
            name: `Pending invitation for ${target}`,
          }),
        ).not.toBeInTheDocument(),
      );
      expect(revokeInvitationMock).toHaveBeenCalledExactlyOnceWith(
        'org-a',
        target,
        expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
      );
      expect(
        await screen.findByRole('group', {
          name: `Pending invitation for ${retained}`,
        }),
      ).toBeVisible();
      expect(await screen.findByText('user-2', { exact: true })).toBeVisible();
      expect(removeMemberMock).not.toHaveBeenCalled();
      expect(updateMemberRoleMock).not.toHaveBeenCalled();
      expect(inviteMemberMock).not.toHaveBeenCalled();
      expect(acceptMyInvitationMock).not.toHaveBeenCalled();
      expect(declineMyInvitationMock).not.toHaveBeenCalled();
    },
  );

  it.each(['user-2', 'user-3'])(
    'names member removal for %s and confirms through an accessible dialog',
    async (target) => {
      listMembersMock.mockResolvedValue([
        ...baseMembers,
        { userId: 'user-3', role: 'viewer', addedAt: '2026-11-01T12:00:00Z' },
      ]);
      renderPanel();

      for (const userId of ['user-2', 'user-3']) {
        expect(
          await screen.findByRole('button', {
            name: `Remove ${userId} from organization`,
          }),
        ).toBeEnabled();
      }
      expect(
        screen.queryByRole('button', {
          name: 'Remove user-1 from organization',
        }),
      ).not.toBeInTheDocument();
      fireEvent.click(
        screen.getByRole('button', {
          name: `Remove ${target} from organization`,
        }),
      );

      const dialog = screen.getByRole('dialog', { name: 'Remove organization member' });
      expect(dialog).toHaveTextContent(target);
      expect(dialog).toHaveTextContent('Organization A');
      fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expectNoWrites();
    },
  );

  it('does not expose named invitation or member administration in a read-only session', async () => {
    listPendingInvitationsMock.mockResolvedValue(pendingInvitations());
    setReadOnly(true);
    renderPanel();

    await screen.findByText('user-2', { exact: true });
    expect(
      screen.queryByRole('button', { name: /^Revoke invitation for / }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /^Remove .* from organization$/ }),
    ).not.toBeInTheDocument();
    expectNoWrites();
  });

  const sameNamedInvitations = () =>
    ['client-east', 'client-west'].map((orgId) => ({
      orgId,
      orgDisplayName: 'Operations',
      userId: 'user-1',
      role: 'viewer',
      invitedBy: 'admin@example.test',
      invitedAt: '2026-11-01T10:00:00Z',
    }));

  it('distinguishes same-named invitations with visible organization IDs and named actions', async () => {
    listMyInvitationsMock.mockResolvedValue(sameNamedInvitations());
    renderPanel();

    for (const orgId of ['client-east', 'client-west']) {
      const invitation = await screen.findByRole('group', {
        name: `Invitation to Operations (${orgId})`,
      });
      expect(within(invitation).getByText(`Operations (${orgId})`)).toBeVisible();
      expect(invitation).toHaveTextContent('admin@example.test');
      expect(invitation).toHaveTextContent('viewer');
      for (const action of ['Accept', 'Decline']) {
        expect(
          within(invitation).getByRole('button', {
            name: `${action} invitation to Operations (${orgId})`,
          }),
        ).toBeEnabled();
      }
    }
    expectNoWrites();
  });

  it.each(['Accept', 'Decline'])(
    '%s sends only the named invitation ID and refreshes without changing the active organization',
    async (action) => {
      let invitations = sameNamedInvitations();
      listMyInvitationsMock.mockImplementation(async () => invitations);
      const request = action === 'Accept' ? acceptMyInvitationMock : declineMyInvitationMock;
      request.mockImplementation(async (orgId) => {
        invitations = invitations.filter((invitation) => invitation.orgId !== orgId);
        return { kind: 'member' };
      });
      renderPanel();

      fireEvent.click(
        await screen.findByRole('button', {
          name: `${action} invitation to Operations (client-west)`,
        }),
      );
      await waitFor(() => expect(request).toHaveBeenCalledTimes(1));
      expect(request).toHaveBeenCalledWith(
        'client-west',
        expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
      );
      expect(
        action === 'Accept' ? declineMyInvitationMock : acceptMyInvitationMock,
      ).not.toHaveBeenCalled();
      expect(
        await screen.findByRole('group', {
          name: 'Invitation to Operations (client-east)',
        }),
      ).toBeVisible();
      expect(
        screen.queryByRole('group', {
          name: 'Invitation to Operations (client-west)',
        }),
      ).not.toBeInTheDocument();
      expect(getOrgIDMock()).toBe('org-a');
      expect(inviteMemberMock).not.toHaveBeenCalled();
      expect(updateMemberRoleMock).not.toHaveBeenCalled();
    },
  );

  it.each(['', 'client-east'])(
    'shows the ID once when the display name is %j',
    async (orgDisplayName) => {
      listMyInvitationsMock.mockResolvedValue([{ ...sameNamedInvitations()[0], orgDisplayName }]);
      renderPanel();
      const invitation = await screen.findByRole('group', {
        name: 'Invitation to client-east',
      });
      expect(within(invitation).getByText('client-east')).toBeVisible();
      expect(
        within(invitation).getByRole('button', {
          name: 'Accept invitation to client-east',
        }),
      ).toBeEnabled();
      expect(invitation).not.toHaveTextContent('client-east (client-east)');
      expectNoWrites();
    },
  );

  it('renders loading skeleton first, then loaded content', async () => {
    const orgDeferred = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(orgDeferred.promise);

    renderPanel();

    expect(screen.getByLabelText('Loading organization access')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Restart access check' }),
    ).toHaveAccessibleDescription(
      'Checking saved members and invitations for org-a. Restarting does not repeat a change.',
    );
    expect(screen.queryByText('Invite Member', { selector: 'h4' })).not.toBeInTheDocument();

    orgDeferred.resolve(baseOrg);

    const heading = await screen.findByText('Invite Member', {
      selector: 'h4',
    });
    expect(heading).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Invite Member' })).toBe(heading);
    expect(screen.queryByLabelText('Loading organization access')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Restart access check' })).not.toBeInTheDocument();
  });

  it('reviews an ownership change without displaying it as saved, and can cancel', async () => {
    renderPanel();
    const role = await screen.findByRole('combobox', {
      name: 'Role for user-2',
    });
    fireEvent.change(role, { target: { value: 'owner' } });
    expect(
      screen.getByRole('dialog', { name: 'Transfer organization ownership' }),
    ).toHaveTextContent('user-2');
    expect(role).toHaveValue('viewer');
    expectNoWrites();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(role).toHaveValue('viewer');
    expectNoWrites();
  });

  it('displays the saved Owner to a non-owner admin without offering another ownership grant', async () => {
    orgGetMock.mockResolvedValue({
      ...baseOrg,
      members: baseMembers.map((member) => ({
        ...member,
        role: member.userId === 'user-2' ? 'admin' : 'owner',
      })),
    });
    render(() => <OrganizationAccessPanel currentUser="user-2" />);
    const ownerRow = (await screen.findByText('user-1', { exact: true })).closest('tr')!;
    expect(within(ownerRow).getByText('owner', { exact: true })).toBeVisible();
    expect(within(ownerRow).queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Role for user-2' })).not.toHaveTextContent(
      'Owner',
    );
    expect(screen.getByRole('combobox', { name: 'Member role' })).not.toHaveTextContent('Owner');
    expectNoWrites();
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
        fireEvent.click(
          screen.getByRole('button', {
            name: 'Remove user-2 from organization',
          }),
        );
        const dialog = screen.getByRole('dialog', { name: 'Remove organization member' });
        fireEvent.click(within(dialog).getByRole('button', { name: 'Remove member' }));
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

  it.each([
    { raw: undefined, scope: 'default' },
    { raw: null, scope: 'default' },
    { raw: '', scope: 'default' },
    { raw: ' \t ', scope: 'default' },
    { raw: 'default', scope: 'default' },
    { raw: ' org-a \t', scope: 'org-a' },
    { raw: 'org-a', scope: 'org-a' },
  ])(
    'binds access reads and a member change to normalized scope $scope from "$raw"',
    async ({ raw, scope }) => {
      getOrgIDMock.mockReturnValue(raw);
      orgGetMock.mockResolvedValue({
        ...baseOrg,
        id: scope,
        canManageMembers: true,
        ownerUserId: scope === 'default' ? undefined : baseOrg.ownerUserId,
      });
      renderPanel();

      const input = await screen.findByRole('textbox', {
        name: scope === 'default' ? 'User ID to add' : 'Member user ID',
      });
      expect(screen.getByText('user-2')).toBeInTheDocument();
      const binding = {
        expectedOrgID: scope,
        retry: false,
        signal: expect.any(AbortSignal),
      };
      for (const read of [orgGetMock, listMembersMock, listPendingInvitationsMock]) {
        expect(read).toHaveBeenCalledExactlyOnceWith(scope, binding);
      }
      expect(listMyInvitationsMock).toHaveBeenCalledExactlyOnceWith(binding);

      const roles = screen.getByRole('combobox', {
        name: 'Member role',
      }) as HTMLSelectElement;
      if (scope === 'default') {
        expect(Array.from(roles.options, (option) => option.value)).toEqual(['viewer']);
      }
      fireEvent.input(input, { target: { value: 'new-user' } });
      fireEvent.click(
        screen.getByRole('button', {
          name: scope === 'default' ? 'Add' : 'Invite',
        }),
      );
      await waitFor(() =>
        expect(inviteMemberMock).toHaveBeenCalledExactlyOnceWith(
          scope,
          { userId: 'new-user', role: 'viewer' },
          binding,
        ),
      );
      await waitFor(() => expect(orgGetMock).toHaveBeenCalledTimes(2));
      await screen.findByRole('textbox', {
        name: scope === 'default' ? 'User ID to add' : 'Member user ID',
      });
      expect(loggerErrorMock).not.toHaveBeenCalled();
    },
  );

  it.each([false, undefined])(
    'does not infer default member administration from an owner ID when canManageMembers is %s',
    async (canManageMembers) => {
      getOrgIDMock.mockReturnValue(' \t ');
      orgGetMock.mockResolvedValue({
        ...baseOrg,
        id: 'default',
        canManageMembers,
      });
      renderPanel();

      await screen.findByText('user-2');
      expect(screen.queryByRole('textbox', { name: 'Member user ID' })).not.toBeInTheDocument();
      expect(screen.queryByRole('combobox', { name: 'Role for user-2' })).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: /^Remove .* from organization$/ }),
      ).not.toBeInTheDocument();
      expect(listPendingInvitationsMock).not.toHaveBeenCalled();
      expectNoWrites();
    },
  );
});

describe('Organization Access keyboard recovery', () => {
  const warning = () => screen.getByRole('alert', { name: 'Organization access unavailable' });
  const summary = () => screen.getByText('Access for', { exact: false });
  const settleFocus = () => new Promise((resolve) => setTimeout(resolve, 10));

  it.each(['invite', 'role', 'remove'])(
    'focuses the warning after an uncertain %s without repeating the write',
    async (operation) => {
      const request = deferred<never>();
      const change = {
        invite: inviteMemberMock,
        role: updateMemberRoleMock,
        remove: removeMemberMock,
      }[operation]!;
      change.mockReturnValueOnce(request.promise);
      renderPanel();
      const input = await screen.findByRole('textbox', {
        name: 'Member user ID',
      });
      fireEvent.input(input, { target: { value: 'new-user' } });
      if (operation === 'role') {
        const control = screen.getByRole('combobox', {
          name: 'Role for user-2',
        });
        control.focus();
        fireEvent.change(control, { target: { value: 'editor' } });
      } else {
        const control = screen.getByRole('button', {
          name: operation === 'invite' ? 'Invite' : 'Remove user-2 from organization',
        });
        control.focus();
        fireEvent.click(control);
        if (operation === 'remove') {
          const dialog = screen.getByRole('dialog', { name: 'Remove organization member' });
          fireEvent.click(within(dialog).getByRole('button', { name: 'Remove member' }));
        }
      }
      request.reject(new Error('response lost after applying change'));
      await waitFor(() => expect(warning()).toHaveFocus());
      expect(warning()).toHaveTextContent('last access change could not be confirmed');
      expect(warning()).toHaveAttribute('tabindex', '-1');
      expect(warning()).not.toHaveAttribute('aria-describedby');
      expect(screen.getByRole('button', { name: 'Reload access' })).toBeEnabled();
      expect(screen.queryByText('user-2', { exact: true })).not.toBeInTheDocument();
      expect(change).toHaveBeenCalledTimes(1);
      expect(writes().reduce((sum, write) => sum + write.mock.calls.length, 0)).toBe(1);
      expect(orgGetMock).toHaveBeenCalledTimes(1);
    },
  );

  it('does not steal outside focus when an uncertain change finishes', async () => {
    const request = deferred<never>();
    updateMemberRoleMock.mockReturnValueOnce(request.promise);
    render(() => (
      <>
        <button>Outside access panel</button>
        <OrganizationAccessPanel currentUser="user-1" />
      </>
    ));
    const control = await screen.findByRole('combobox', {
      name: 'Role for user-2',
    });
    control.focus();
    fireEvent.change(control, { target: { value: 'editor' } });
    const outside = screen.getByRole('button', {
      name: 'Outside access panel',
    });
    outside.focus();
    request.reject(new Error('response unavailable'));
    await screen.findByRole('alert');
    await settleFocus();
    expect(outside).toHaveFocus();
    expect(updateMemberRoleMock).toHaveBeenCalledTimes(1);
  });

  it('does not take focus on an initial read failure or an unrequested background read', async () => {
    orgGetMock.mockRejectedValueOnce(new Error('Unavailable'));
    renderPanel();
    await screen.findByRole('alert');
    await settleFocus();
    expect(document.body).toHaveFocus();
    eventBusEmitMock('organizations_changed');
    await screen.findByText('user-2');
    await settleFocus();
    expect(document.body).toHaveFocus();
    expect(summary()).toHaveAttribute('tabindex', '-1');
    expect(summary()).not.toHaveAttribute('aria-label');
    expectNoWrites();
  });

  it.each(['keyboard', 'pointer without button focus'])(
    'focuses the saved summary after an explicit %s reload, without a write',
    async (activation) => {
      orgGetMock.mockRejectedValueOnce(new Error('Unavailable'));
      renderPanel();
      const reload = await screen.findByRole('button', {
        name: 'Reload access',
      });
      if (activation === 'keyboard') reload.focus();
      fireEvent.click(reload);
      await waitFor(() => expect(summary()).toHaveFocus());
      expect(summary()).toHaveTextContent('Access for Organization A (org-a)');
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(orgGetMock).toHaveBeenCalledTimes(2);
      expectNoWrites();
    },
  );

  it('returns focus to a fresh warning when an explicit saved-access reload fails', async () => {
    orgGetMock.mockRejectedValue(new Error('Unavailable'));
    renderPanel();
    const reload = await screen.findByRole('button', { name: 'Reload access' });
    const previous = warning();
    reload.focus();
    fireEvent.click(reload);
    await waitFor(() => expect(warning()).toHaveFocus());
    expect(warning()).not.toBe(previous);
    expect(orgGetMock).toHaveBeenCalledTimes(2);
    expectNoWrites();
  });

  it('preserves focus deliberately moved elsewhere during an explicit reload', async () => {
    const read = deferred<typeof baseOrg>();
    orgGetMock.mockRejectedValueOnce(new Error('Unavailable')).mockReturnValueOnce(read.promise);
    render(() => (
      <>
        <button>Outside access panel</button>
        <OrganizationAccessPanel currentUser="user-1" />
      </>
    ));
    const reload = await screen.findByRole('button', { name: 'Reload access' });
    reload.focus();
    fireEvent.click(reload);
    const outside = screen.getByRole('button', {
      name: 'Outside access panel',
    });
    outside.focus();
    read.resolve(baseOrg);
    await screen.findByText('user-2');
    await settleFocus();
    expect(outside).toHaveFocus();
    expectNoWrites();
  });

  it.each(['organization', 'user', 'hidden surface'])(
    'cancels explicit reload focus when the %s changes',
    async (context) => {
      const read = deferred<typeof baseOrg>();
      orgGetMock.mockRejectedValueOnce(new Error('Unavailable')).mockReturnValueOnce(read.promise);
      const [user, setUser] = createSignal('user-1');
      render(() => <OrganizationAccessPanel currentUser={user()} />);
      const reload = await screen.findByRole('button', {
        name: 'Reload access',
      });
      reload.focus();
      fireEvent.click(reload);
      if (context === 'organization') {
        getOrgIDMock.mockReturnValue('org-b');
        orgGetMock.mockResolvedValue({ ...baseOrg, id: 'org-b' });
        eventBusEmitMock('org_switched');
      } else if (context === 'user') {
        setUser('different-user');
      } else {
        setHidden(true);
      }
      read.resolve(baseOrg);
      if (context !== 'hidden surface') await screen.findByText('user-2');
      await settleFocus();
      expect(document.body).toHaveFocus();
      expectNoWrites();
    },
  );

  it('restores the same saved role control after a successful refresh without repeating its write', async () => {
    renderPanel();
    const control = await screen.findByRole('combobox', {
      name: 'Role for user-2',
    });
    control.focus();
    fireEvent.change(control, { target: { value: 'editor' } });
    await waitFor(() => expect(orgGetMock).toHaveBeenCalledTimes(2));
    await screen.findByText('user-2');
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: 'Role for user-2' })).toHaveFocus(),
    );
    expect(screen.getByRole('combobox', { name: 'Role for user-2' })).not.toBe(control);
    expect(updateMemberRoleMock).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it.each(['invite-user', 'invite-role', 'invite-submit'])(
    'returns to the same draft control after a successful %s submission',
    async (origin) => {
      renderPanel();
      const input = await screen.findByRole('textbox', {
        name: 'Member user ID',
      });
      fireEvent.input(input, { target: { value: 'new-user' } });
      const control = {
        'invite-user': input,
        'invite-role': screen.getByRole('combobox', { name: 'Member role' }),
        'invite-submit': screen.getByRole('button', { name: 'Invite' }),
      }[origin]!;
      control.focus();
      fireEvent.submit(screen.getByRole('form', { name: 'Invite member' }));
      await waitFor(() => expect(orgGetMock).toHaveBeenCalledTimes(2));
      const fresh = await screen.findByRole('textbox', {
        name: 'Member user ID',
      });
      await waitFor(() =>
        expect(document.activeElement).toHaveAttribute('data-org-access-action', origin),
      );
      expect(fresh).toHaveValue('');
      expect(inviteMemberMock).toHaveBeenCalledTimes(1);
    },
  );

  it.each([
    ['first', 'middle'],
    ['middle', 'last'],
    ['last', 'middle'],
  ])(
    'continues removed member %s at %s, not the same displayed row index',
    async (target, next) => {
      let members = [
        baseMembers[0],
        ...['first', 'middle', 'last'].map((userId) => ({
          ...baseMembers[1],
          userId,
        })),
      ];
      listMembersMock.mockImplementation(async () => members);
      removeMemberMock.mockImplementation(async (_orgID, userId) => {
        // The new snapshot changes order; continuation follows the old identities.
        members = members.filter((member) => member.userId !== userId).reverse();
      });
      renderPanel();
      const trigger = await screen.findByRole('button', {
        name: `Remove ${target} from organization`,
      });
      trigger.focus();
      fireEvent.click(trigger);
      const dialog = screen.getByRole('dialog', { name: 'Remove organization member' });
      fireEvent.click(within(dialog).getByRole('button', { name: 'Remove member' }));
      await waitFor(() =>
        expect(
          screen.getByRole('button', {
            name: `Remove ${next} from organization`,
          }),
        ).toHaveFocus(),
      );
      expect(removeMemberMock).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['revoke', 'accept', 'decline'])(
    'continues %s invitations by exact identity, falling back to the saved summary when none remain',
    async (action) => {
      let invitations = ['east/1?realm=saved#exact', 'west/2?realm=saved#exact'].map((id) => ({
        userId: id,
        orgId: id,
        orgDisplayName: 'Operations',
        role: 'viewer',
        invitedBy: 'owner',
        invitedAt: '2026-11-01T00:00:00Z',
      }));
      const request =
        action === 'revoke'
          ? revokeInvitationMock
          : action === 'accept'
            ? acceptMyInvitationMock
            : declineMyInvitationMock;
      if (action === 'revoke')
        listPendingInvitationsMock.mockImplementation(async () => invitations);
      else listMyInvitationsMock.mockImplementation(async () => invitations);
      request.mockImplementation(async (orgID, userID) => {
        const id = action === 'revoke' ? userID : orgID;
        invitations = invitations.filter((invitation) => invitation.orgId !== id);
        return { kind: 'member' };
      });
      renderPanel();
      for (const id of invitations.map((invitation) => invitation.orgId)) {
        const name =
          action === 'revoke'
            ? `Revoke invitation for ${id}`
            : `${action === 'accept' ? 'Accept' : 'Decline'} invitation to Operations (${id})`;
        const trigger = await screen.findByRole('button', { name });
        trigger.focus();
        fireEvent.click(trigger);
        await waitFor(() => expect(request).toHaveBeenCalledTimes(id.startsWith('east') ? 1 : 2));
        if (id.startsWith('east')) {
          await waitFor(() =>
            expect(document.activeElement).toHaveAttribute(
              'data-org-access-target',
              'west/2?realm=saved#exact',
            ),
          );
          expect(document.activeElement).toHaveAttribute('data-org-access-action', action);
        } else await waitFor(() => expect(summary()).toHaveFocus());
      }
    },
  );

  it.each(['pending write', 'saved reread'])(
    'does not steal deliberate outside focus during the %s of a successful change',
    async (phase) => {
      const request = deferred<undefined>();
      const read = deferred<typeof baseOrg>();
      updateMemberRoleMock.mockReturnValueOnce(request.promise);
      orgGetMock.mockReturnValueOnce(Promise.resolve(baseOrg)).mockReturnValueOnce(read.promise);
      render(() => (
        <>
          <button>Outside access panel</button>
          <OrganizationAccessPanel currentUser="user-1" />
        </>
      ));
      const trigger = await screen.findByRole('combobox', {
        name: 'Role for user-2',
      });
      trigger.focus();
      fireEvent.change(trigger, { target: { value: 'editor' } });
      if (phase === 'saved reread') {
        request.resolve(undefined);
        await screen.findByLabelText('Loading organization access');
      }
      const outside = screen.getByRole('button', {
        name: 'Outside access panel',
      });
      outside.focus();
      request.resolve(undefined);
      read.resolve(baseOrg);
      await screen.findByText('user-2');
      await settleFocus();
      expect(outside).toHaveFocus();
      expect(updateMemberRoleMock).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['organization round trip', 'user round trip', 'read-only', 'hidden surface'])(
    'retires saved-control focus on %s while a change is pending',
    async (context) => {
      const request = deferred<undefined>();
      updateMemberRoleMock.mockReturnValueOnce(request.promise);
      const [user, setUser] = createSignal('user-1');
      render(() => <OrganizationAccessPanel currentUser={user()} />);
      const trigger = await screen.findByRole('combobox', {
        name: 'Role for user-2',
      });
      trigger.focus();
      fireEvent.change(trigger, { target: { value: 'editor' } });
      if (context === 'organization round trip') {
        getOrgIDMock.mockReturnValue('org-b');
        orgGetMock.mockResolvedValueOnce({ ...baseOrg, id: 'org-b' });
        eventBusEmitMock('org_switched');
        await screen.findByText('user-2');
        getOrgIDMock.mockReturnValue('org-a');
        eventBusEmitMock('org_switched');
      } else if (context === 'user round trip') {
        setUser('different-user');
        await screen.findByText('user-2');
        setUser('user-1');
      } else if (context === 'read-only') setReadOnly(true);
      else setHidden(true);
      request.resolve(undefined);
      if (context !== 'hidden surface') await screen.findByText('user-2');
      await settleFocus();
      expect(document.body).toHaveFocus();
      expect(updateMemberRoleMock).toHaveBeenCalledTimes(1);
    },
  );

  it('cancels a pending focus handoff when the panel unmounts', async () => {
    const request = deferred<never>();
    updateMemberRoleMock.mockReturnValueOnce(request.promise);
    const view = renderPanel();
    const control = await screen.findByRole('combobox', {
      name: 'Role for user-2',
    });
    control.focus();
    fireEvent.change(control, { target: { value: 'editor' } });
    request.reject(new Error('response unavailable'));
    await Promise.resolve();
    expect(warning()).toBeInTheDocument();
    const focus = vi.spyOn(warning(), 'focus');
    view.unmount();
    await settleFocus();
    expect(focus).not.toHaveBeenCalled();
    focus.mockRestore();
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
    orgGetMock.mockResolvedValueOnce({
      ...baseOrg,
      id: 'org-b',
      displayName: 'Organization B',
    });
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
  const [user, setUser] = createSignal<string | undefined>('user-1');
  const view = render(() => {
    state = useOrganizationAccessPanelState({
      get currentUser() {
        return user();
      },
    });
    return <div />;
  });
  return { state, setUser, ...view };
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
  const noPendingInvitations = 'No pending invitations for this organization.';
  const noPersonalInvitations = 'No invitations are waiting for you.';
  const noInvitations = 'No pending invitations. None for you.';
  const emptyResults = [noPendingInvitations, noPersonalInvitations, noInvitations];

  it.each([
    { failedList: 'neither', expected: noInvitations },
    { failedList: 'pending', expected: noPersonalInvitations },
    { failedList: 'personal', expected: noPendingInvitations },
    { failedList: 'both', expected: undefined },
  ])(
    'reports only the verified empty result when $failedList failed',
    async ({ failedList, expected }) => {
      const unavailable = () =>
        Object.assign(new Error('Temporarily unavailable'), { status: 503 });
      if (failedList === 'pending' || failedList === 'both') {
        listPendingInvitationsMock.mockRejectedValueOnce(unavailable());
      }
      if (failedList === 'personal' || failedList === 'both') {
        listMyInvitationsMock.mockRejectedValueOnce(unavailable());
      }
      renderPanel();

      const summary = within(await screen.findByText(/Access for/));
      for (const result of emptyResults) {
        if (result === expected) expect(summary.getByText(result)).toBeVisible();
        else expect(summary.queryByText(result)).not.toBeInTheDocument();
      }
      if (failedList !== 'neither') {
        expect(
          screen.getByRole('alert', { name: 'Organization invitations incomplete' }),
        ).toBeVisible();
        expect(screen.getByRole('button', { name: /^Invite$/ })).toBeDisabled();
      }
      expectNoWrites();
    },
  );

  it.each(['viewer', 'read-only owner'])(
    'does not describe the manager-only list as empty for a %s',
    async (reader) => {
      setReadOnly(reader === 'read-only owner');
      render(() => (
        <OrganizationAccessPanel currentUser={reader === 'viewer' ? 'user-2' : 'user-1'} />
      ));

      const summary = within(await screen.findByText(/Access for/));
      expect(summary.getByText(noPersonalInvitations)).toBeVisible();
      expect(summary.queryByText(noPendingInvitations)).not.toBeInTheDocument();
      expect(summary.queryByText(noInvitations)).not.toBeInTheDocument();
      if (reader === 'viewer') expect(listPendingInvitationsMock).not.toHaveBeenCalled();
      expectNoWrites();
    },
  );

  it('allows a read-only reload when invitations fail before a sibling read settles', async () => {
    const oldMembers = deferred<typeof baseMembers>();
    listMembersMock.mockReturnValueOnce(oldMembers.promise);
    listPendingInvitationsMock.mockRejectedValueOnce(
      Object.assign(new Error('Temporary invitation failure'), { status: 503 }),
    );
    renderPanel();
    const warning = await screen.findByRole('alert', {
      name: 'Organization invitations incomplete',
    });
    expect(warning).toHaveTextContent('Pending Invitations');
    expect(screen.queryByText('user-2')).not.toBeInTheDocument();
    for (const result of emptyResults) expect(screen.queryByText(result)).not.toBeInTheDocument();
    const oldSignal = listMembersMock.mock.calls[0][1].signal;
    expect(oldSignal.aborted).toBe(false);
    const reload = within(warning).getByRole('button', { name: 'Reload access' });
    expect(reload).toBeEnabled();
    expect(screen.queryByRole('button', { name: 'Restart access check' })).not.toBeInTheDocument();

    listMembersMock.mockResolvedValueOnce([{ ...baseMembers[1], userId: 'fresh-member' }]);
    fireEvent.click(reload);
    expect(oldSignal.aborted).toBe(true);
    expect(await screen.findByText('fresh-member')).toBeInTheDocument();
    oldMembers.resolve(baseMembers);
    await waitFor(() => expect(screen.getByText(/Access for/)).toHaveFocus());
    expect(screen.getByText(noInvitations)).toBeVisible();
    expect(screen.queryByText('user-2')).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(orgGetMock).toHaveBeenCalledTimes(2);
    expectNoWrites();
  });

  it('keeps first-read failure distinct from accepted empty membership, and retries only reads', async () => {
    orgGetMock.mockRejectedValueOnce(new Error('Unavailable'));
    renderPanel();
    await screen.findByRole('alert');
    expect(screen.queryByText('No organization members found.')).not.toBeInTheDocument();
    for (const result of emptyResults) expect(screen.queryByText(result)).not.toBeInTheDocument();
    orgGetMock.mockResolvedValue(baseOrg);
    listMembersMock.mockResolvedValue([]);
    fireEvent.click(screen.getByRole('button', { name: 'Reload access' }));
    await screen.findByText('No organization members found.');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expectNoWrites();
  });

  it('preserves only the same-selection invitation draft through a failed refresh and reload', async () => {
    renderPanel();
    const input = await screen.findByRole('textbox', {
      name: 'Member user ID',
    });
    fireEvent.input(input, { target: { value: 'unsent-user' } });
    orgGetMock.mockRejectedValueOnce(new Error('Unavailable'));
    eventBusEmitMock('organizations_changed');
    await screen.findByRole('alert');
    expect(screen.queryByText('user-2')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Reload access' }));
    expect(await screen.findByRole('textbox', { name: 'Member user ID' })).toHaveValue(
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

  it('does not surface a temporary invitation error from a retired workspace read', async () => {
    const oldInvitations = deferred<unknown[]>();
    listPendingInvitationsMock.mockReturnValueOnce(oldInvitations.promise);
    const { state } = renderState();
    await waitFor(() => expect(listPendingInvitationsMock).toHaveBeenCalledTimes(1));
    const oldSignal = listPendingInvitationsMock.mock.calls[0][1].signal;
    getOrgIDMock.mockReturnValue('org-b');
    orgGetMock.mockResolvedValueOnce({ ...baseOrg, id: 'org-b' });
    eventBusEmitMock('org_switched');
    await ready(state);
    expect(oldSignal.aborted).toBe(true);

    oldInvitations.reject(Object.assign(new Error('Obsolete failure'), { status: 503 }));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(state.org()?.id).toBe('org-b');
    expect(state.invitationReadIssue()).toBeNull();
    expect(state.canChangeAccess()).toBe(true);
  });

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
    orgGetMock.mockResolvedValue({
      ...baseOrg,
      id: 'default',
      ownerUserId: undefined,
    });
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

  it('rejects every mutation handler even on current rows if invitations are incomplete', async () => {
    listMyInvitationsMock.mockRejectedValueOnce(
      Object.assign(new Error('Temporary invitation failure'), { status: 503 }),
    );
    const { state } = renderState();
    await ready(state);
    expect(state.invitationReadIssue()).toBe('personal');
    expect(state.canManageCurrentOrg()).toBe(true);
    await exerciseAllChanges(state, state.members()[1]);
    expectNoWrites();
    expect(confirm).not.toHaveBeenCalled();
  });

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
        remove: () => {
          state.removeMember(member);
          const removal = state.memberRemoval();
          return removal ? state.confirmMemberRemoval(removal) : Promise.resolve();
        },
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

  const refreshActions = [
    {
      name: 'invite',
      mock: inviteMemberMock,
      run: (state: ReturnType<typeof useOrganizationAccessPanelState>) => state.inviteMember(),
    },
    {
      name: 'role',
      mock: updateMemberRoleMock,
      run: (state: ReturnType<typeof useOrganizationAccessPanelState>) =>
        state.updateRole(state.members()[1], 'editor'),
    },
    {
      name: 'remove',
      mock: removeMemberMock,
      run: (state: ReturnType<typeof useOrganizationAccessPanelState>) => {
        state.removeMember(state.members()[1]);
        const removal = state.memberRemoval();
        return removal ? state.confirmMemberRemoval(removal) : Promise.resolve();
      },
    },
    {
      name: 'revoke',
      mock: revokeInvitationMock,
      run: (state: ReturnType<typeof useOrganizationAccessPanelState>) =>
        state.revokeInvitation('invited-user'),
    },
    {
      name: 'accept',
      mock: acceptMyInvitationMock,
      run: (state: ReturnType<typeof useOrganizationAccessPanelState>) =>
        state.acceptInvitation('inviting-org'),
    },
    {
      name: 'decline',
      mock: declineMyInvitationMock,
      run: (state: ReturnType<typeof useOrganizationAccessPanelState>) =>
        state.declineInvitation('inviting-org'),
    },
  ];

  it.each(
    refreshActions.flatMap((action) =>
      ['success', 'uncertain'].map((outcome) => ({ ...action, outcome })),
    ),
  )(
    'preserves pending $name through equal-reader refreshes and handles $outcome once',
    async ({ mock, run, outcome }) => {
      const { state } = renderState();
      await ready(state);
      const member = state.members()[1];
      state.setInviteUserID('new-user');
      const result = deferred<unknown>();
      mock.mockReturnValueOnce(result.promise);
      const change = run(state);
      const signal = mock.mock.calls[0].at(-1).signal;
      eventBusEmitMock('organizations_changed');
      eventBusEmitMock('organizations_changed');
      await state.loadOrganizationAccess();
      setHidden(false); // A new policy object with unchanged effective authority.
      expect(signal.aborted).toBe(false);
      expect(state.saving()).toBe(true);
      expect(orgGetMock).toHaveBeenCalledTimes(1);
      await exerciseAllChanges(state, state.members()[1]);
      expect(writes().reduce((sum, write) => sum + write.mock.calls.length, 0)).toBe(1);

      if (outcome === 'success') {
        result.resolve({ kind: 'member', member });
        await change;
        await ready(state);
        expect(notificationSuccessMock).toHaveBeenCalledTimes(1);
        expect(orgGetMock).toHaveBeenCalledTimes(2);
      } else {
        result.reject(new Error('response lost after the server applied the change'));
        await change;
        expect(state.loadError()).toContain('could not be confirmed');
        expect(state.org()).toBeNull();
        expect(state.members()).toEqual([]);
        expect(state.pendingInvitations()).toEqual([]);
        expect(state.myInvitations()).toEqual([]);
        expect(notificationSuccessMock).not.toHaveBeenCalled();
        expect(orgGetMock).toHaveBeenCalledTimes(1);
        await exerciseAllChanges(state, member);
        expect(writes().reduce((sum, write) => sum + write.mock.calls.length, 0)).toBe(1);
        await state.loadOrganizationAccess();
        await ready(state);
        expect(orgGetMock).toHaveBeenCalledTimes(2);
      }
      expect(mock).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['organization', 'read-only', 'entitlement', 'disposal'])(
    'still cancels a pending access change on %s loss after an equal-reader refresh',
    async (kind) => {
      const { state, unmount } = renderState();
      await ready(state);
      const result = deferred<unknown>();
      inviteMemberMock.mockReturnValueOnce(result.promise);
      state.setInviteUserID('new-user');
      const change = state.inviteMember();
      const signal = inviteMemberMock.mock.calls[0].at(-1).signal;
      eventBusEmitMock('organizations_changed');
      if (kind === 'organization') {
        getOrgIDMock.mockReturnValue('org-b');
        orgGetMock.mockResolvedValue({ ...baseOrg, id: 'org-b' });
        eventBusEmitMock('org_switched');
      }
      if (kind === 'read-only') setReadOnly(true);
      if (kind === 'entitlement') setEnabled(false);
      if (kind === 'disposal') unmount();
      expect(signal.aborted).toBe(true);
      result.resolve({ kind: 'member' });
      await change;
      expect(notificationSuccessMock).not.toHaveBeenCalled();
      expect(notificationErrorMock).not.toHaveBeenCalled();
      expect(inviteMemberMock).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['user', 'anonymous', 'hidden'])(
    'invalidates a pending access change on %s withdrawal after same-reader refresh',
    async (kind) => {
      const { state, setUser } = renderState();
      await ready(state);
      const result = deferred<unknown>();
      inviteMemberMock.mockReturnValueOnce(result.promise);
      state.setInviteUserID('new-user');
      const change = state.inviteMember();
      const signal = inviteMemberMock.mock.calls[0].at(-1).signal;
      eventBusEmitMock('organizations_changed');
      expect(signal.aborted).toBe(false);
      if (kind === 'user') setUser('other-user');
      if (kind === 'anonymous') setUser(undefined);
      if (kind === 'hidden') setHidden(true);
      expect(signal.aborted).toBe(true);
      expect(state.saving()).toBe(false);
      result.reject(new Error('Late old-reader response'));
      await change;
      expect(notificationSuccessMock).not.toHaveBeenCalled();
      expect(notificationErrorMock).not.toHaveBeenCalled();
      expect(inviteMemberMock).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['success', 'uncertain', 'mismatched confirmation'])(
    'retains a confirmed ownership transfer through equal-reader refreshes: %s',
    async (outcome) => {
      const { state } = renderState();
      await ready(state);
      const member = state.members()[1];
      await state.updateRole(member, 'owner');
      const transfer = state.ownershipTransfer()!;
      expect(transfer.member).toBe(member);
      const result = deferred<unknown>();
      updateMemberRoleMock.mockReturnValueOnce(result.promise);
      const change = state.confirmOwnershipTransfer(transfer);
      const signal = updateMemberRoleMock.mock.calls[0].at(-1).signal;
      eventBusEmitMock('organizations_changed');
      await state.loadOrganizationAccess();
      setHidden(false);
      expect(state.saving()).toBe(true);
      expect(signal.aborted).toBe(false);
      expect(orgGetMock).toHaveBeenCalledTimes(1);
      await state.confirmOwnershipTransfer(transfer);
      await exerciseAllChanges(state, member);
      expect(writes().reduce((sum, write) => sum + write.mock.calls.length, 0)).toBe(1);
      orgGetMock.mockResolvedValue({ ...baseOrg, ownerUserId: member.userId });
      listMembersMock.mockResolvedValue([
        { ...baseMembers[0], role: 'admin' },
        { ...member, role: 'owner' },
      ]);
      if (outcome === 'uncertain') result.reject(new Error('Response lost after apply'));
      else
        result.resolve({
          kind: 'member',
          member: {
            ...member,
            role: outcome === 'success' ? 'owner' : 'viewer',
          },
        });
      await change;
      if (outcome === 'success') {
        await ready(state);
        expect(notificationSuccessMock).toHaveBeenCalledTimes(1);
      } else {
        expect(state.loadError()).toContain('could not be confirmed');
        expect(notificationSuccessMock).not.toHaveBeenCalled();
        expect(orgGetMock).toHaveBeenCalledTimes(1);
        await state.loadOrganizationAccess();
        await ready(state);
      }
      expect(state.org()?.ownerUserId).toBe(member.userId);
      expect(state.members()[1].role).toBe('owner');
      expect(updateMemberRoleMock).toHaveBeenCalledExactlyOnceWith(
        'org-a',
        { userId: member.userId, role: 'owner' },
        expect.objectContaining({
          expectedOrgID: 'org-a',
          retry: false,
          signal,
        }),
      );
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
    result.resolve({
      kind: 'invitation',
      invitation: { ...pendingInvitation, userId: 'first-user' },
    });
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
    state.removeMember(state.members()[1]);
    const removal = state.memberRemoval()!;
    expect(removal).not.toBeNull();
    getOrgIDMock.mockReturnValue('org-b');
    await state.confirmMemberRemoval(removal);
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
    expect(
      screen.getByRole('button', {
        name: 'Accept invitation to Inviting Organization (inviting-org)',
      }),
    ).toBeDisabled();
    expect(
      screen.getByRole('button', {
        name: 'Decline invitation to Inviting Organization (inviting-org)',
      }),
    ).toBeDisabled();
    expect(screen.queryByRole('textbox', { name: 'Member user ID' })).not.toBeInTheDocument();
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
    fireEvent.input(await screen.findByRole('textbox', { name: 'Member user ID' }), {
      target: { value: 'private-draft' },
    });
    const pending = deferred<typeof baseMembers>();
    listMembersMock.mockReturnValueOnce(pending.promise);
    eventBusEmitMock('organizations_changed');
    await waitFor(() => expect(listMembersMock).toHaveBeenCalledTimes(2));
    orgGetMock.mockResolvedValue({ ...baseOrg, ownerUserId: 'user-b' });
    setCurrentUser('user-b');
    expect(await screen.findByRole('textbox', { name: 'Member user ID' })).toHaveValue('');
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

describe('Default organization member form', () => {
  it('names the immediate viewer-only Add flow without implying an invitation', async () => {
    getOrgIDMock.mockReturnValue('default');
    orgGetMock.mockResolvedValue({
      ...baseOrg,
      id: 'default',
      canManageMembers: true,
    });
    inviteMemberMock.mockResolvedValue({
      kind: 'member',
      member: { userId: 'new-user', role: 'viewer' },
    });
    renderPanel();
    const form = await screen.findByRole('form', { name: 'Add member' });
    const input = within(form).getByRole('textbox', { name: 'User ID to add' }) as HTMLInputElement;
    const role = within(form).getByRole('combobox', { name: 'Member role' });
    expect(input).toHaveAccessibleDescription(
      /Adding grants access immediately.*instance roles determine permissions/,
    );
    expect(form).toHaveAccessibleDescription(/Viewer is the organization role/);
    expect(Array.from(input.labels ?? [], (label) => label.textContent?.trim())).toContain(
      'User ID',
    );
    expect(within(role).getAllByRole('option')).toHaveLength(1);
    expect(role).toHaveValue('viewer');
    expect(within(form).getByRole('button', { name: 'Add' })).toHaveAttribute('type', 'submit');
    fireEvent.input(input, { target: { value: 'new-user' } });
    fireEvent.submit(form);
    await waitFor(() => expect(inviteMemberMock).toHaveBeenCalledTimes(1));
    expect(inviteMemberMock).toHaveBeenCalledWith(
      'default',
      { userId: 'new-user', role: 'viewer' },
      expect.objectContaining({ expectedOrgID: 'default', retry: false }),
    );
    expect(notificationSuccessMock).toHaveBeenCalledExactlyOnceWith('Added new-user as viewer.');
  });
});
