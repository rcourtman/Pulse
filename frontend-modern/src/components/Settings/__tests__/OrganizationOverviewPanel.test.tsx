import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { OrganizationOverviewPanel } from '../OrganizationOverviewPanel';

const orgGetMock = vi.fn();
const listMembersMock = vi.fn();
const updateOrgMock = vi.fn();
const isMultiTenantEnabledMock = vi.fn();
const presentationPolicyHidesOrganizationSurfacesMock = vi.fn();
const getOrgIDMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const eventBusOnMock = vi.fn();
const eventBusEmitMock = vi.fn();
const loggerErrorMock = vi.fn();
const listeners = new Map<string, Set<() => void>>();

vi.mock('@/api/orgs', () => ({
  OrgsAPI: {
    get: (...args: unknown[]) => orgGetMock(...args),
    listMembers: (...args: unknown[]) => listMembersMock(...args),
    update: (...args: unknown[]) => updateOrgMock(...args),
  },
}));

vi.mock('@/stores/license', () => ({
  isMultiTenantEnabled: (...args: unknown[]) => isMultiTenantEnabledMock(...args),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => false,
  presentationPolicyHidesOrganizationSurfaces: (...args: unknown[]) =>
    presentationPolicyHidesOrganizationSurfacesMock(...args),
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
  createdAt: '2024-01-10T00:00:00Z',
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

const renderPanel = () => render(() => <OrganizationOverviewPanel currentUser="user-1" />);

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
  updateOrgMock.mockReset();
  isMultiTenantEnabledMock.mockReset();
  presentationPolicyHidesOrganizationSurfacesMock.mockReset();
  getOrgIDMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  eventBusOnMock.mockReset();
  eventBusEmitMock.mockReset();
  loggerErrorMock.mockReset();
  listeners.clear();

  isMultiTenantEnabledMock.mockReturnValue(true);
  presentationPolicyHidesOrganizationSurfacesMock.mockReturnValue(false);
  getOrgIDMock.mockReturnValue('org-a');
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
  updateOrgMock.mockResolvedValue({ ...baseOrg, displayName: 'Updated Org' });
});

afterEach(() => {
  cleanup();
});

describe('OrganizationOverviewPanel', () => {
  it('keeps each full saved member ID paired with its own role and added date', async () => {
    const members = [
      {
        userId: 'oidc:operations-north:platform-owner@example.test',
        role: 'owner',
        addedAt: '2026-10-01T10:00:00Z',
      },
      {
        userId: 'oidc:operations-north:alex-administrator@example.test',
        role: 'admin',
        addedAt: '2026-10-03T10:00:00Z',
      },
      {
        userId: 'oidc:operations-north:alex-observer@example.test',
        role: 'viewer',
        addedAt: '2026-10-05T10:00:00Z',
      },
      {
        userId: 'local:automation-support:night-shift-editor@example.test',
        role: 'editor',
        addedAt: '2026-10-07T10:00:00Z',
      },
    ];
    listMembersMock.mockResolvedValue(members);
    renderPanel();
    const table = await screen.findByRole('table');

    const expectSavedMembership = async (expected: typeof members) => {
      await waitFor(() => {
        const rows = within(screen.getByRole('table')).getAllByRole('row').slice(1);
        expect(rows).toHaveLength(expected.length);
        expected.forEach((member, index) => {
          const cells = within(rows[index]).getAllByRole('cell');
          expect(cells).toHaveLength(3);
          expect(cells[0]).toHaveTextContent(member.userId);
          expect(cells[1]).toHaveTextContent(member.role);
          expect(cells[2]).toHaveTextContent(new Date(member.addedAt).toLocaleDateString());
        });
      });
    };

    expect(
      within(table)
        .getAllByRole('columnheader')
        .map((header) => header.textContent),
    ).toEqual(['User', 'Role', 'Added']);
    await expectSavedMembership(members);

    // Keyed rows must keep the returned identity/role pairing after refresh,
    // rather than attaching a new role to a previous row's long display text.
    const refreshed = [...members].reverse().map((member) => ({
      ...member,
      role: member.role === 'viewer' ? 'editor' : member.role,
    }));
    listMembersMock.mockResolvedValue(refreshed);
    eventBusEmitMock('organizations_changed');
    await expectSavedMembership(refreshed);
    expect(listMembersMock).toHaveBeenCalledTimes(2);
    expect(updateOrgMock).not.toHaveBeenCalled();
  });

  it('renders loading skeleton first, then loaded content', async () => {
    const orgDeferred = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(orgDeferred.promise);

    const { container } = renderPanel();

    expect(container.querySelector('.animate-pulse')).toBeTruthy();
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();

    orgDeferred.resolve(baseOrg);

    await waitFor(() => {
      expect(screen.getByLabelText('Display Name')).toBeInTheDocument();
    });
    const options = orgGetMock.mock.calls[0][1];
    expect(options).toEqual({
      expectedOrgID: 'org-a',
      signal: expect.any(AbortSignal),
      retry: false,
    });
    expect(listMembersMock).toHaveBeenCalledWith('org-a', options);
  });

  it.each([
    { raw: undefined, scope: 'default' },
    { raw: null, scope: 'default' },
    { raw: '', scope: 'default' },
    { raw: ' \t ', scope: 'default' },
    { raw: 'default', scope: 'default' },
    { raw: ' org-a \t', scope: 'org-a' },
    { raw: 'org-a', scope: 'org-a' },
  ])(
    'binds overview reads and a rename to normalized scope $scope from "$raw"',
    async ({ raw, scope }) => {
      getOrgIDMock.mockReturnValue(raw);
      orgGetMock.mockResolvedValue({ ...baseOrg, id: scope });
      const savedOrg = { ...baseOrg, id: scope, displayName: 'Selected workspace' };
      const pending = deferred<typeof savedOrg>();
      updateOrgMock.mockReturnValueOnce(pending.promise);
      renderPanel();

      const input = await screen.findByRole('textbox', { name: 'Display Name' });
      expect(input).toHaveValue(baseOrg.displayName);
      expect(screen.getByText('user-2')).toBeInTheDocument();
      const binding = {
        expectedOrgID: scope,
        retry: false,
        signal: expect.any(AbortSignal),
      };
      expect(orgGetMock).toHaveBeenCalledExactlyOnceWith(scope, binding);
      expect(listMembersMock).toHaveBeenCalledExactlyOnceWith(scope, binding);

      fireEvent.input(input, { target: { value: 'Selected workspace' } });
      fireEvent.click(screen.getByRole('button', { name: 'Save' }));
      await waitFor(() =>
        expect(updateOrgMock).toHaveBeenCalledExactlyOnceWith(
          scope,
          { displayName: 'Selected workspace' },
          binding,
        ),
      );
      // Equal-reader refreshes must not discard this normalized-scope rename.
      eventBusEmitMock('organizations_changed');
      eventBusEmitMock('organizations_changed');
      expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled();
      expect(input).toHaveValue('Selected workspace');
      expect(input).toBeDisabled();
      expect(updateOrgMock.mock.calls[0][2].signal.aborted).toBe(false);
      expect(orgGetMock).toHaveBeenCalledTimes(1);
      orgGetMock.mockResolvedValue(savedOrg);
      pending.resolve(savedOrg);
      await waitFor(() => expect(orgGetMock).toHaveBeenCalledTimes(2));
      expect(await screen.findByRole('textbox', { name: 'Display Name' })).toHaveValue(
        savedOrg.displayName,
      );
      expect(loggerErrorMock).not.toHaveBeenCalled();
    },
  );

  it('stays unavailable in demo mode without loading organization data', () => {
    presentationPolicyHidesOrganizationSurfacesMock.mockReturnValue(true);

    renderPanel();

    expect(
      screen.getByText('Organization settings are not available on this server.'),
    ).toBeInTheDocument();
    expect(orgGetMock).not.toHaveBeenCalled();
    expect(listMembersMock).not.toHaveBeenCalled();
  });
});
