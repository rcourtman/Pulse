import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { OrganizationBillingPanel } from '../OrganizationBillingPanel';
import { OrganizationOverviewPanel } from '../OrganizationOverviewPanel';
import { eventBus } from '@/stores/events';

const mocks = vi.hoisted(() => ({
  status: vi.fn(),
  list: vi.fn(),
  members: vi.fn(),
  get: vi.fn(),
  update: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  log: vi.fn(),
  enabled: vi.fn(),
  hidden: vi.fn(),
  readOnly: vi.fn(),
  orgID: vi.fn(),
}));
vi.mock('@/api/license', () => ({ LicenseAPI: { getStatus: mocks.status } }));
vi.mock('@/api/orgs', () => ({
  OrgsAPI: {
    list: mocks.list,
    listMembers: mocks.members,
    get: mocks.get,
    update: mocks.update,
  },
}));
vi.mock('@/utils/apiClient', () => ({ getOrgID: mocks.orgID }));
vi.mock('@/stores/license', () => ({ isMultiTenantEnabled: mocks.enabled }));
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
  let reject!: (error?: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const org = (id: string) => ({ id, displayName: `Organization ${id}`, ownerUserId: 'owner' });
const members = (id: string) => [{ userId: `${id}-member`, role: 'viewer', addedAt: '' }];
const status = (id: string) => ({
  valid: true,
  tier: 'cloud',
  is_lifetime: false,
  days_remaining: 30,
  features: [],
  email: `${id}@example.test`,
});
let selected = 'org-a';
let setEnabled!: (value: boolean) => void;
let setHidden!: (value: boolean) => void;
let setReadOnly!: (value: boolean) => void;
const switchOrg = (id: string) => {
  selected = id;
  eventBus.emit('org_switched', id);
};
const renderBilling = () => render(() => <OrganizationBillingPanel nodeUsage={5} guestUsage={2} />);
const renderOverview = () => render(() => <OrganizationOverviewPanel currentUser="owner" />);
const settle = async () => {
  await new Promise((resolve) => setTimeout(resolve, 0));
};

beforeEach(() => {
  Object.values(mocks).forEach((mock) => mock.mockReset());
  const [enabled, updateEnabled] = createSignal(true);
  const [hidden, updateHidden] = createSignal(false);
  const [readOnly, updateReadOnly] = createSignal(false);
  setEnabled = updateEnabled;
  setHidden = updateHidden;
  setReadOnly = updateReadOnly;
  mocks.enabled.mockImplementation(enabled);
  mocks.hidden.mockImplementation(hidden);
  mocks.readOnly.mockImplementation(readOnly);
  selected = 'org-a';
  mocks.orgID.mockImplementation(() => selected);
  mocks.status.mockImplementation(() => Promise.resolve(status(selected)));
  mocks.list.mockResolvedValue([org('org-a'), org('org-b')]);
  mocks.members.mockImplementation((id) => Promise.resolve(members(id)));
  mocks.get.mockImplementation((id) => Promise.resolve(org(id)));
  mocks.update.mockImplementation((id, payload) => Promise.resolve({ ...org(id), ...payload }));
});
afterEach(cleanup);

describe('billing read lifetime', () => {
  it('binds all three reads to one abortable selection and session preference', async () => {
    renderBilling();
    await screen.findByText('org-a@example.test');
    const options = mocks.status.mock.calls[0][0];
    expect(options).toEqual({
      expectedOrgID: 'org-a',
      signal: expect.any(AbortSignal),
      retry: false,
      preferSessionAuth: true,
    });
    expect(mocks.list).toHaveBeenCalledWith(options);
    expect(mocks.members).toHaveBeenCalledWith('org-a', options);
  });

  it.each(['status', 'list', 'members'] as const)(
    'shows unread, not free/zero, if %s fails',
    async (key) => {
      mocks[key].mockRejectedValueOnce(new Error('private upstream credential'));
      renderBilling();
      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Unable to load billing and plan details.',
      );
      expect(screen.queryByText('Plan Tier')).not.toBeInTheDocument();
      expect(screen.queryByText('Not configured')).not.toBeInTheDocument();
      expect(screen.queryByText(/private upstream/)).not.toBeInTheDocument();
      expect(mocks.status.mock.calls[0][0].signal.aborted).toBe(true);
    },
  );

  it.each([null, {}, { valid: true }])('rejects malformed status %j', async (payload) => {
    mocks.status.mockResolvedValueOnce(payload);
    renderBilling();
    await screen.findByRole('alert');
    expect(screen.queryByText('Plan Tier')).not.toBeInTheDocument();
  });

  it('cannot finish a newer spinner or overwrite B with a late A success', async () => {
    const a = deferred<ReturnType<typeof status>>();
    const b = deferred<ReturnType<typeof status>>();
    mocks.status.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise);
    const { container } = renderBilling();
    const signalA = mocks.status.mock.calls[0][0].signal;
    switchOrg('org-b');
    expect(signalA.aborted).toBe(true);
    a.resolve(status('org-a'));
    await settle();
    expect(
      container.querySelector('[aria-label="Loading organization billing"]'),
    ).toBeInTheDocument();
    expect(screen.queryByText('org-a@example.test')).not.toBeInTheDocument();
    b.resolve(status('org-b'));
    await screen.findByText('org-b@example.test');
  });

  it('ignores A failure after B has loaded', async () => {
    const a = deferred<ReturnType<typeof status>>();
    mocks.status.mockReturnValueOnce(a.promise);
    renderBilling();
    switchOrg('org-b');
    await screen.findByText('org-b@example.test');
    a.reject(new Error('old failure'));
    await settle();
    expect(screen.getByText('org-b@example.test')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(mocks.log).not.toHaveBeenCalled();
  });

  it('rejects first A even after selecting A again', async () => {
    const first = deferred<ReturnType<typeof status>>();
    mocks.status.mockReturnValueOnce(first.promise);
    renderBilling();
    switchOrg('org-b');
    switchOrg('org-a');
    await screen.findByText('org-a@example.test');
    first.resolve(status('obsolete'));
    await settle();
    expect(screen.queryByText('obsolete@example.test')).not.toBeInTheDocument();
    expect(screen.getByText('org-a@example.test')).toBeInTheDocument();
  });

  it('clears old billing on failed refresh and explicitly reloads accepted empty counts', async () => {
    renderBilling();
    await screen.findByText('org-a@example.test');
    mocks.members.mockRejectedValueOnce(new Error('denied'));
    eventBus.emit('organizations_changed');
    await screen.findByRole('alert');
    expect(screen.queryByText('org-a@example.test')).not.toBeInTheDocument();
    const next = deferred<unknown[]>();
    mocks.members.mockReturnValueOnce(next.promise);
    const retry = screen.getByRole('button', { name: 'Reload billing' });
    fireEvent.click(retry);
    fireEvent.click(retry);
    expect(mocks.members).toHaveBeenCalledTimes(3);
    next.resolve([]);
    await screen.findByText('org-a@example.test');
    const label = screen.getByText('Members (Current Org)');
    expect(label.parentElement).toHaveTextContent('0');
  });

  it.each(['capability', 'demo'] as const)(
    'invalidates pending data on %s withdrawal and reads afresh on return',
    async (kind) => {
      const old = deferred<ReturnType<typeof status>>();
      mocks.status.mockReturnValueOnce(old.promise);
      renderBilling();
      const signal = mocks.status.mock.calls[0][0].signal;
      if (kind === 'capability') setEnabled(false);
      else setHidden(true);
      await screen.findByText('Organization settings are not available on this server.');
      expect(signal.aborted).toBe(true);
      old.resolve(status('obsolete'));
      await settle();
      if (kind === 'capability') setEnabled(true);
      else setHidden(false);
      await screen.findByText('org-a@example.test');
      expect(screen.queryByText('obsolete@example.test')).not.toBeInTheDocument();
      expect(mocks.status).toHaveBeenCalledTimes(2);
    },
  );

  it('aborts and detaches listeners on disposal without late feedback', async () => {
    const old = deferred<ReturnType<typeof status>>();
    mocks.status.mockReturnValueOnce(old.promise);
    const view = renderBilling();
    const signal = mocks.status.mock.calls[0][0].signal;
    view.unmount();
    expect(signal.aborted).toBe(true);
    eventBus.emit('organizations_changed');
    switchOrg('org-b');
    old.reject(new Error('late failure'));
    await settle();
    expect(mocks.status).toHaveBeenCalledTimes(1);
    expect(mocks.log).not.toHaveBeenCalled();
  });
});

describe('overview read and rename lifetime', () => {
  it('binds both reads and renders only the matching organization', async () => {
    renderOverview();
    await screen.findByLabelText('Display Name');
    const options = mocks.get.mock.calls[0][1];
    expect(options).toEqual({
      expectedOrgID: 'org-a',
      signal: expect.any(AbortSignal),
      retry: false,
    });
    expect(mocks.members).toHaveBeenCalledWith('org-a', options);
  });

  it.each(['get', 'members'] as const)(
    'shows an explicit error and no retained members when %s fails',
    async (key) => {
      renderOverview();
      await screen.findByText('org-a-member');
      mocks[key].mockRejectedValueOnce(new Error('private credential'));
      eventBus.emit('organizations_changed');
      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Unable to load organization details.',
      );
      expect(screen.queryByText('org-a-member')).not.toBeInTheDocument();
      expect(screen.queryByText('No members found.')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
      expect(screen.queryByText(/private credential/)).not.toBeInTheDocument();
    },
  );

  it('rejects another organization and malformed member lists', async () => {
    mocks.get.mockResolvedValueOnce(org('org-b'));
    renderOverview();
    await screen.findByRole('alert');
    mocks.members.mockResolvedValueOnce(null);
    fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
    await screen.findByRole('alert');
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
    await screen.findByLabelText('Display Name');
  });

  it('cannot finish B loading with late A data', async () => {
    const a = deferred<ReturnType<typeof org>>();
    const b = deferred<ReturnType<typeof org>>();
    mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise);
    const { container } = renderOverview();
    const signal = mocks.get.mock.calls[0][1].signal;
    switchOrg('org-b');
    a.resolve(org('org-a'));
    await settle();
    expect(signal.aborted).toBe(true);
    expect(
      container.querySelector('[aria-label="Loading organization overview"]'),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
    b.resolve(org('org-b'));
    await screen.findByText('org-b-member');
    expect(screen.getByLabelText('Display Name')).toHaveValue('Organization org-b');
  });

  it('ignores a late failure and rejects first A after A/B/A', async () => {
    const first = deferred<ReturnType<typeof org>>();
    mocks.get.mockReturnValueOnce(first.promise);
    renderOverview();
    switchOrg('org-b');
    switchOrg('org-a');
    await screen.findByText('org-a-member');
    first.reject(new Error('obsolete'));
    await settle();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(mocks.log).not.toHaveBeenCalled();
  });

  it('clears data and draft when the current user changes', async () => {
    const [user, setUser] = createSignal('owner');
    render(() => <OrganizationOverviewPanel currentUser={user()} />);
    await screen.findByLabelText('Display Name');
    fireEvent.input(screen.getByLabelText('Display Name'), { target: { value: 'Unsaved' } });
    const next = deferred<ReturnType<typeof org>>();
    mocks.get.mockReturnValueOnce(next.promise);
    setUser('viewer');
    await settle();
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
    next.resolve(org('org-a'));
    await screen.findByLabelText('Display Name');
    expect(screen.getByLabelText('Display Name')).toBeDisabled();
    expect(screen.getByLabelText('Display Name')).toHaveValue('Organization org-a');
  });

  it('keeps read-only sessions from renaming', async () => {
    setReadOnly(true);
    renderOverview();
    await screen.findByLabelText('Display Name');
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(mocks.update).not.toHaveBeenCalled();
  });

  it('dispatches one exact rename, then refreshes saved details', async () => {
    const pending = deferred<ReturnType<typeof org>>();
    mocks.update.mockReturnValueOnce(pending.promise);
    renderOverview();
    await screen.findByLabelText('Display Name');
    fireEvent.input(screen.getByLabelText('Display Name'), { target: { value: ' Renamed A ' } });
    const save = screen.getByRole('button', { name: 'Save' });
    fireEvent.click(save);
    fireEvent.click(save);
    expect(mocks.update).toHaveBeenCalledTimes(1);
    expect(mocks.update).toHaveBeenCalledWith(
      'org-a',
      { displayName: 'Renamed A' },
      {
        expectedOrgID: 'org-a',
        signal: expect.any(AbortSignal),
        retry: false,
      },
    );
    mocks.get.mockResolvedValueOnce({ ...org('org-a'), displayName: 'Renamed A' });
    pending.resolve({ ...org('org-a'), displayName: 'Renamed A' });
    await waitFor(() => expect(screen.getByLabelText('Display Name')).toHaveValue('Renamed A'));
    expect(mocks.get).toHaveBeenCalledTimes(2);
    expect(mocks.success).toHaveBeenCalledTimes(1);
  });

  it('requires a saved read after an unconfirmed write, not an automatic write retry', async () => {
    mocks.update.mockRejectedValueOnce(new Error('private upstream failure'));
    renderOverview();
    await screen.findByLabelText('Display Name');
    fireEvent.input(screen.getByLabelText('Display Name'), { target: { value: 'Renamed' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The last name change could not be confirmed.',
    );
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
    expect(mocks.error).toHaveBeenCalledWith('Unable to update the organization name.');
    fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
    await screen.findByLabelText('Display Name');
    expect(mocks.update).toHaveBeenCalledTimes(1);
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it.each(['success', 'failure'] as const)(
    'ignores late rename %s after a switch',
    async (result) => {
      const pending = deferred<ReturnType<typeof org>>();
      mocks.update.mockReturnValueOnce(pending.promise);
      renderOverview();
      await screen.findByLabelText('Display Name');
      fireEvent.input(screen.getByLabelText('Display Name'), { target: { value: 'Renamed A' } });
      fireEvent.click(screen.getByRole('button', { name: 'Save' }));
      const signal = mocks.update.mock.calls[0][2].signal;
      switchOrg('org-b');
      await screen.findByText('org-b-member');
      expect(signal.aborted).toBe(true);
      if (result === 'success') pending.resolve({ ...org('org-a'), displayName: 'Renamed A' });
      else pending.reject(new Error('old write failure'));
      await settle();
      expect(screen.getByLabelText('Display Name')).toHaveValue('Organization org-b');
      expect(mocks.success).not.toHaveBeenCalled();
      expect(mocks.error).not.toHaveBeenCalled();
      expect(mocks.get).toHaveBeenCalledTimes(2);
    },
  );

  it('aborts pending reads on withdrawal and disposal, detaching refreshes', async () => {
    const pending = deferred<ReturnType<typeof org>>();
    mocks.get.mockReturnValueOnce(pending.promise);
    const view = renderOverview();
    const signal = mocks.get.mock.calls[0][1].signal;
    setEnabled(false);
    await screen.findByText('Organization settings are not available on this server.');
    expect(signal.aborted).toBe(true);
    pending.resolve(org('org-a'));
    await settle();
    setEnabled(true);
    await screen.findByLabelText('Display Name');
    view.unmount();
    eventBus.emit('organizations_changed');
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });
});
