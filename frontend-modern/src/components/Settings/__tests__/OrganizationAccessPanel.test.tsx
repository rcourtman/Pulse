import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
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

  isMultiTenantEnabledMock.mockReturnValue(true);
  getOrgIDMock.mockReturnValue('org-a');
  eventBusOnMock.mockReturnValue(() => undefined);

  orgGetMock.mockResolvedValue(baseOrg);
  listMembersMock.mockResolvedValue(baseMembers);
  listPendingInvitationsMock.mockResolvedValue([]);
  listMyInvitationsMock.mockResolvedValue([]);
  updateMemberRoleMock.mockResolvedValue(undefined);
  inviteMemberMock.mockResolvedValue({ kind: 'invitation' });
  acceptMyInvitationMock.mockResolvedValue({ kind: 'member' });
  declineMyInvitationMock.mockResolvedValue(undefined);
  revokeInvitationMock.mockResolvedValue(undefined);
  removeMemberMock.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
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

  it('normalizes org scope through the shared helper', () => {
    expect(organizationAccessStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationAccessStateSource).not.toContain("getOrgID() || 'default'");
  });
});
