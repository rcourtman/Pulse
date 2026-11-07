import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type {
  OrganizationInvitation,
  OrganizationMember,
  UserOrganizationInvitation,
} from '@/api/orgs';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { OrganizationAccessPanel } from '../OrganizationAccessPanel';

// Keep actual controls, grid reconciliation, ownership dialog, state owner,
// OrgsAPI and session transport connected. Hook-only and API-mocked suites
// cannot detect a rendered control submitting a different admitted identity.
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const orgID = 'estate/north?realm=lab#saved';
const displayName = 'North estate';
const owner = 'owner/maintainer?realm=lab#saved';
const unrelatedToken = 'synthetic-unrelated-organization-token';
const csrf = 'synthetic-organization-selection-csrf';
const people = ['first', 'second', 'third'].map((part) => `member/${part}?realm=lab#saved`);
const originalMembers: OrganizationMember[] = [
  { userId: owner, role: 'owner', addedAt: '2026-11-01T00:00:00Z' },
  ...people.map<OrganizationMember>((userId) => ({
    userId,
    role: 'viewer',
    addedAt: '2026-11-02T00:00:00Z',
  })),
];
const originalPending: OrganizationInvitation[] = ['first', 'second', 'third'].map((part) => ({
  userId: `pending/${part}?realm=lab#saved`,
  role: 'viewer',
  invitedAt: '2026-11-03T00:00:00Z',
  invitedBy: owner,
}));
const originalPersonal: UserOrganizationInvitation[] = ['first', 'second', 'third'].map((part) => ({
  orgId: `inviting/${part}?realm=lab#saved`,
  orgDisplayName: 'Client estate',
  userId: owner,
  role: 'viewer',
  invitedAt: '2026-11-03T00:00:00Z',
  invitedBy: `inviter/${part}`,
}));
const orgPath = `/api/orgs/${encodeURIComponent(orgID)}`;
const personalPath = (id: string) => `/api/org-invitations/${encodeURIComponent(id)}`;
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const fetchMock = vi.fn<typeof fetch>();
const confirmMock = vi.fn(() => true);
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => ['POST', 'DELETE'].includes(init?.method ?? 'GET'));
const orgReads = () =>
  fetchMock.mock.calls.filter(([url, init]) => url === orgPath && !init?.method);
let members: OrganizationMember[];
let pending: OrganizationInvitation[];
let personal: UserOrganizationInvitation[];
let ownerID: string;
let nextWriteStatus: number;
let unexpected: string[];

function roleFor(userId: string) {
  return screen.getByRole('combobox', { name: `Role for ${userId}` });
}
function pendingGroup(userId: string) {
  return screen.getByRole('group', { name: `Pending invitation for ${userId}` });
}
function personalGroup(invitation: UserOrganizationInvitation) {
  return screen.getByRole('group', {
    name: `Invitation to Client estate (${invitation.orgId})`,
  });
}
function expectBoundRequests() {
  for (const [url, init] of fetchMock.mock.calls) {
    if (!String(url).startsWith('/api/org')) continue;
    expect(init).toMatchObject({ credentials: 'include', signal: expect.any(AbortSignal) });
    const headers = new Headers(init?.headers);
    // These are session-only organisation operations, even with a saved API
    // token. Personal invitation routes also retain the current reader context.
    expect(headers.get('X-Pulse-Org-ID')).toBe(orgID);
    expect(headers.has('X-API-Token')).toBe(false);
    if (init?.method) expect(headers.get('X-CSRF-Token')).toBe(csrf);
    for (const internal of ['expectedOrgID', 'retry', 'preferSessionAuth', 'skipOrgContext'])
      expect(init).not.toHaveProperty(internal);
  }
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(unrelatedToken);
}
function expectWrite(path: string, method: string, body?: unknown) {
  expect(writes()).toHaveLength(1);
  const [url, init] = writes()[0];
  expect(url).toBe(path);
  expect(init?.method).toBe(method);
  expect(init?.body === undefined ? undefined : JSON.parse(String(init.body))).toEqual(body);
  expectBoundRequests();
}
async function expectSavedMembers() {
  await waitFor(() => {
    for (const member of members) expect(roleFor(member.userId)).toHaveValue(member.role);
  });
  expect(screen.getAllByRole('combobox', { name: /^Role for / })).toHaveLength(members.length);
}
async function openPanel() {
  render(() => <OrganizationAccessPanel currentUser={owner} />);
  await expectSavedMembers();
  for (const invitation of pending) expect(pendingGroup(invitation.userId)).toBeInTheDocument();
  for (const invitation of personal) expect(personalGroup(invitation)).toBeInTheDocument();
  expect(orgReads()).toHaveLength(1);
  expect(writes()).toHaveLength(0);
}
async function expectSavedReread() {
  await waitFor(() => expect(orgReads()).toHaveLength(2));
  await expectSavedMembers();
}

beforeEach(async () => {
  clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  setOrgID(orgID);
  setApiToken(unrelatedToken);
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: false,
      hideUpgrade: true,
    },
  });
  vi.stubEnv('MODE', 'production');
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
  members = structuredClone(originalMembers);
  pending = structuredClone(originalPending);
  personal = structuredClone(originalPersonal);
  ownerID = owner;
  nextWriteStatus = 200;
  unexpected = [];
  confirmMock.mockReset().mockReturnValue(true);
  vi.stubGlobal('confirm', confirmMock);
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET') {
      if (path === '/api/license/runtime-capabilities')
        return json({ capabilities: ['multi_tenant'], limits: [] });
      if (path === orgPath) return json({ id: orgID, displayName, ownerUserId: ownerID, members });
      // Deliberately return a different order from the fixture's identity list.
      if (path === `${orgPath}/members`)
        return json([...members.slice(-1), ...members.slice(0, -1)]);
      if (path === `${orgPath}/invitations`) return json(pending);
      if (path === '/api/org-invitations') return json(personal);
    } else {
      // Derive all saved fixture changes from the actual path/body, never the
      // identity the test intended. A lost reply may still have applied a write.
      const reply = (body: unknown, empty = false) =>
        nextWriteStatus === 200
          ? empty
            ? new Response(null, { status: 204 })
            : json(body)
          : json({ error: 'Synthetic acknowledgement unavailable' }, nextWriteStatus);
      if (path === `${orgPath}/members` && method === 'POST') {
        const body = JSON.parse(String(init.body));
        const member = members.find((candidate) => candidate.userId === body.userId);
        if (member) {
          members = members.map((candidate) => ({
            ...candidate,
            role:
              candidate.userId === body.userId
                ? body.role
                : body.role === 'owner' && candidate.userId === ownerID
                  ? 'admin'
                  : candidate.role,
          }));
          if (body.role === 'owner') ownerID = body.userId;
          return reply({ kind: 'member', member: members.find((m) => m.userId === body.userId) });
        }
        const invitation: OrganizationInvitation = {
          userId: body.userId,
          role: body.role,
          invitedAt: '2026-11-07T00:00:00Z',
          invitedBy: owner,
        };
        pending = [...pending, invitation];
        return reply({ kind: 'invitation', invitation });
      }
      const member = members.find(
        (candidate) => path === `${orgPath}/members/${encodeURIComponent(candidate.userId)}`,
      );
      if (member && method === 'DELETE') {
        members = members.filter((candidate) => candidate.userId !== member.userId);
        return reply(undefined, true);
      }
      const invitation = pending.find(
        (candidate) => path === `${orgPath}/invitations/${encodeURIComponent(candidate.userId)}`,
      );
      if (invitation && method === 'DELETE') {
        pending = pending.filter((candidate) => candidate.userId !== invitation.userId);
        return reply(undefined, true);
      }
      const incoming = personal.find((candidate) =>
        [personalPath(candidate.orgId), `${personalPath(candidate.orgId)}/accept`].includes(path),
      );
      if (
        incoming &&
        ((method === 'DELETE' && path === personalPath(incoming.orgId)) ||
          (method === 'POST' && path === `${personalPath(incoming.orgId)}/accept`))
      ) {
        personal = personal.filter((candidate) => candidate.orgId !== incoming.orgId);
        return reply(
          { kind: 'member', member: { userId: owner, role: incoming.role, addedAt: '' } },
          method === 'DELETE',
        );
      }
    }
    unexpected.push(`${method} ${path}`);
    throw new Error(`Unexpected synthetic organization selection request: ${method} ${path}`);
  });
  vi.stubGlobal('fetch', fetchMock);
  await loadRuntimeCapabilities(true);
});

afterEach(() => {
  cleanup();
  syncSessionPresentationPolicy(null);
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  expect(unexpected).toEqual([]);
});

describe('organization access selections through rendered controls and real transport', () => {
  it('updates the selected grid member and then the same reconciled row without changing siblings', async () => {
    await openPanel();
    const selected = people[1];
    fireEvent.change(roleFor(selected), { target: { value: 'editor' } });
    expect(roleFor(selected)).toHaveValue('viewer');
    await expectSavedReread();
    expectWrite(`${orgPath}/members`, 'POST', { userId: selected, role: 'editor' });
    expect(members.filter((member) => member.userId !== selected)).toEqual(
      originalMembers.filter((member) => member.userId !== selected),
    );
    expect(roleFor(selected)).toHaveValue('editor');
    // PulseDataGrid now holds proxies for a new read's objects. The selected
    // row must remain actionable and still target its identity after reload.
    fireEvent.change(roleFor(selected), { target: { value: 'admin' } });
    await waitFor(() => expect(orgReads()).toHaveLength(3));
    await expectSavedMembers();
    expect(writes()).toHaveLength(2);
    expect(JSON.parse(String(writes()[1][1]?.body))).toEqual({ userId: selected, role: 'admin' });
    expect(roleFor(selected)).toHaveValue('admin');
    for (const sibling of people.filter((person) => person !== selected))
      expect(roleFor(sibling)).toHaveValue('viewer');
    expectBoundRequests();
  });

  it('cancels then confirms removal of only the selected member, never the owner or sibling', async () => {
    await openPanel();
    const selected = people[1];
    const remove = () =>
      screen.getByRole('button', { name: `Remove ${selected} from organization` });
    confirmMock.mockReturnValueOnce(false);
    fireEvent.click(remove());
    expect(confirmMock).toHaveBeenLastCalledWith(`Remove ${selected} from ${displayName}?`);
    expect(writes()).toHaveLength(0);
    expect(members).toEqual(originalMembers);
    expect(roleFor(selected)).toHaveValue('viewer');
    fireEvent.click(remove());
    await expectSavedReread();
    expectWrite(`${orgPath}/members/${encodeURIComponent(selected)}`, 'DELETE');
    expect(members).toEqual(originalMembers.filter((member) => member.userId !== selected));
    expect(screen.queryByRole('combobox', { name: `Role for ${selected}` })).toBeNull();
    expect(screen.queryByRole('button', { name: `Remove ${owner} from organization` })).toBeNull();
    for (const sibling of people.filter((person) => person !== selected))
      expect(roleFor(sibling)).toHaveValue('viewer');
  });

  it('binds cancelled and confirmed ownership transfer to the selected member and reconciles both owners', async () => {
    await openPanel();
    const selected = people[1];
    const requestTransfer = () =>
      fireEvent.change(roleFor(selected), { target: { value: 'owner' } });
    requestTransfer();
    const dialog = within(screen.getByRole('dialog', { name: 'Transfer organization ownership' }));
    expect(dialog.getByText(selected, { selector: 'strong' })).toBeInTheDocument();
    expect(dialog.getByText(displayName, { selector: 'strong' })).toBeInTheDocument();
    expect(roleFor(selected)).toHaveValue('viewer');
    expect(writes()).toHaveLength(0);
    fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(members).toEqual(originalMembers);
    expect(writes()).toHaveLength(0);
    requestTransfer();
    fireEvent.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: 'Transfer ownership' }),
    );
    await expectSavedReread();
    expectWrite(`${orgPath}/members`, 'POST', { userId: selected, role: 'owner' });
    expect(ownerID).toBe(selected);
    expect(roleFor(selected)).toHaveValue('owner');
    expect(roleFor(selected)).toBeDisabled();
    expect(roleFor(owner)).toHaveValue('admin');
    for (const sibling of people.filter((person) => person !== selected))
      expect(roleFor(sibling)).toHaveValue('viewer');
    expect(
      screen.queryByRole('button', { name: `Remove ${selected} from organization` }),
    ).toBeNull();
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('submits the trimmed invitation draft and selected role while preserving existing access', async () => {
    await openPanel();
    const userId = 'new/member?realm=lab#saved';
    fireEvent.input(screen.getByRole('textbox', { name: 'Invite user ID' }), {
      target: { value: `  ${userId}  ` },
    });
    fireEvent.change(screen.getByRole('combobox', { name: 'Invite role' }), {
      target: { value: 'editor' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Invite', exact: true }));
    await expectSavedReread();
    expectWrite(`${orgPath}/members`, 'POST', { userId, role: 'editor' });
    expect(members).toEqual(originalMembers);
    expect(pending.filter((invitation) => invitation.userId !== userId)).toEqual(originalPending);
    expect(within(pendingGroup(userId)).getByText('editor')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Invite user ID' })).toHaveValue('');
    expect(screen.getByRole('combobox', { name: 'Invite role' })).toHaveValue('viewer');
  });

  it('revokes only the selected pending invitation and keeps the other recipients available', async () => {
    await openPanel();
    const selected = originalPending[1];
    fireEvent.click(
      within(pendingGroup(selected.userId)).getByRole('button', {
        name: `Revoke invitation for ${selected.userId}`,
      }),
    );
    await expectSavedReread();
    expectWrite(`${orgPath}/invitations/${encodeURIComponent(selected.userId)}`, 'DELETE');
    expect(pending).toEqual(originalPending.filter((invitation) => invitation !== selected));
    expect(
      screen.queryByRole('group', { name: `Pending invitation for ${selected.userId}` }),
    ).toBeNull();
    for (const sibling of pending) expect(pendingGroup(sibling.userId)).toBeInTheDocument();
    expect(members).toEqual(originalMembers);
    expect(personal).toEqual(originalPersonal);
  });

  it.each(['accept', 'decline'] as const)(
    '%ss only the selected namesake organization invitation without switching the reader',
    async (action) => {
      await openPanel();
      const selected = originalPersonal[1];
      fireEvent.click(
        within(personalGroup(selected)).getByRole('button', {
          name: `${action === 'accept' ? 'Accept' : 'Decline'} invitation to Client estate (${selected.orgId})`,
        }),
      );
      await expectSavedReread();
      expectWrite(
        personalPath(selected.orgId) + (action === 'accept' ? '/accept' : ''),
        action === 'accept' ? 'POST' : 'DELETE',
      );
      expect(personal).toEqual(originalPersonal.filter((invitation) => invitation !== selected));
      expect(
        screen.queryByRole('group', { name: `Invitation to Client estate (${selected.orgId})` }),
      ).toBeNull();
      for (const sibling of personal) expect(personalGroup(sibling)).toBeInTheDocument();
      expect(members).toEqual(originalMembers);
      expect(pending).toEqual(originalPending);
    },
  );

  it('recovers an applied removal with a lost reply using Reload access, never another write', async () => {
    await openPanel();
    const selected = people[1];
    nextWriteStatus = 502;
    fireEvent.click(screen.getByRole('button', { name: `Remove ${selected} from organization` }));
    const warning = await screen.findByRole('alert');
    expect(
      within(warning).getByText(/last access change could not be confirmed/),
    ).toBeInTheDocument();
    expect(orgReads()).toHaveLength(1);
    expect(screen.queryAllByRole('combobox', { name: /^Role for / })).toHaveLength(0);
    expect(screen.queryByRole('button', { name: 'Invite', exact: true })).toBeNull();
    expectWrite(`${orgPath}/members/${encodeURIComponent(selected)}`, 'DELETE');
    fireEvent.click(within(warning).getByRole('button', { name: 'Reload access' }));
    await expectSavedReread();
    expect(writes()).toHaveLength(1);
    expect(members).toEqual(originalMembers.filter((member) => member.userId !== selected));
    expect(screen.queryByRole('combobox', { name: `Role for ${selected}` })).toBeNull();
    for (const sibling of people.filter((person) => person !== selected))
      expect(roleFor(sibling)).toHaveValue('viewer');
    expectBoundRequests();
  });
});
