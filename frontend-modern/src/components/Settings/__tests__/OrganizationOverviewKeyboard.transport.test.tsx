import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { OrganizationOverviewPanel } from '../OrganizationOverviewPanel';
import { clearApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), debug: vi.fn() } }));

const orgID = 'operations/east';
const owner = 'owner@example.test';
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
};
let completion: ReturnType<typeof deferred<Response>>;
let savedName: string;
let readGate: ReturnType<typeof deferred<void>> | undefined;
let failRead: boolean;
let detailResponse: ((id: string) => Response | Promise<Response>) | undefined;
let memberResponse: (() => Response | Promise<Response>) | undefined;
const requests: { url: string; method: string; options: RequestInit }[] = [];
const writes = () => requests.filter((request) => request.method === 'PUT');
const reads = () =>
  requests.filter(
    (request) => request.method === 'GET' && request.url === '/api/orgs/operations%2Feast',
  );
const memberReads = () => requests.filter((request) => request.url.endsWith('/members'));
const organization = (id = orgID) => ({
  id,
  displayName: id === orgID ? savedName : 'Operations West',
  ownerUserId: owner,
  createdAt: '2026-10-01T10:00:00Z',
  members: [
    { userId: owner, role: 'owner', addedAt: '2026-10-01T10:00:00Z' },
    { userId: `${id}-viewer`, role: 'viewer', addedAt: '2026-10-02T10:00:00Z' },
  ],
});
const policy = (readOnly = false) =>
  syncSessionPresentationPolicy({
    presentationPolicy: { demoMode: false, readOnly, hideCommercial: false, hideUpgrade: true },
  });

beforeEach(async () => {
  savedName = 'Operations East';
  completion = deferred<Response>();
  readGate = undefined;
  failRead = false;
  detailResponse = undefined;
  memberResponse = undefined;
  requests.length = 0;
  clearApiToken();
  setOrgID(orgID);
  document.cookie = 'pulse_csrf=synthetic-session-csrf; Path=/';
  policy();
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, options: RequestInit) => {
      const method = options.method || 'GET';
      requests.push({ url, method, options });
      if (url === '/api/license/runtime-capabilities')
        return response({ capabilities: ['multi_tenant'], limits: [] });
      if (method === 'PUT') return completion.promise;
      if (readGate) await readGate.promise;
      if (url.endsWith('/members')) return memberResponse?.() ?? response(organization().members);
      if (failRead) return response({ error: 'Synthetic read unavailable' }, 503);
      const id = decodeURIComponent(url.split('/').at(-1)!);
      return detailResponse?.(id) ?? response(organization(id));
    }),
  );
  await loadRuntimeCapabilities(true);
});

afterEach(() => {
  cleanup();
  clearApiToken();
  setOrgID(null);
  vi.unstubAllGlobals();
});

const open = async (initialUser = owner) => {
  const [user, setUser] = createSignal(initialUser);
  const view = render(() => (
    <>
      <button type="button">Outside overview</button>
      <OrganizationOverviewPanel currentUser={user()} />
    </>
  ));
  const input = await screen.findByRole('textbox', { name: 'Display Name' });
  expect(input).toHaveValue('Operations East');
  return { ...view, input, setUser };
};
const submit = () => {
  const form = screen.getByRole('form', { name: 'Organization display name' });
  expect(screen.getByRole('button', { name: 'Save' })).toHaveAttribute('type', 'submit');
  expect(fireEvent.submit(form)).toBe(false);
};
const rename = async () => {
  const view = await open();
  view.input.focus();
  fireEvent.input(view.input, { target: { value: ' Operations North ' } });
  submit();
  await waitFor(() => expect(writes()).toHaveLength(1));
  return view;
};
const settleFocus = () => new Promise((resolve) => window.setTimeout(resolve, 15));
const results = () => screen.getByRole('group', { name: 'Saved organization details' });
const restartButton = () => screen.getByRole('button', { name: 'Restart details check' });

describe('Organization Overview selected membership snapshot', () => {
  it('shows the saved name, member count and roles from one authorized detail request', async () => {
    memberResponse = () => response({ code: 'org_load_failed' }, 500);
    const { input } = await open();
    const table = screen.getByRole('table');
    expect(within(table).getByRole('row', { name: /owner@example\.test.*owner/i })).toBeVisible();
    expect(
      within(table).getByRole('row', { name: /operations\/east-viewer.*viewer/i }),
    ).toBeVisible();
    expect(screen.getByText('Members', { exact: true }).nextElementSibling).toHaveTextContent(
      /^2$/,
    );
    expect(input).toBeEnabled();
    expect(reads()).toHaveLength(1);
    expect(memberReads()).toHaveLength(0);
    expect(writes()).toHaveLength(0);
  });

  it.each([
    { members: null },
    { members: {} },
    { members: [null] },
    { members: [{ userId: 'viewer', role: 'owner', addedAt: null }] },
    { members: [{ userId: 'viewer', role: 'unknown', addedAt: '' }] },
  ])(
    'withdraws the complete panel for present malformed membership $members',
    async ({ members }) => {
      detailResponse = (id) => response({ ...organization(id), members });
      render(() => <OrganizationOverviewPanel currentUser={owner} />);
      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Unable to load organization details.',
      );
      expect(screen.queryByRole('table')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
      expect(screen.queryByText('No members found.')).not.toBeInTheDocument();
      expect(memberReads()).toHaveLength(0);
      expect(writes()).toHaveLength(0);
    },
  );

  it('verifies an omitted count with the bound list, keeping unread distinct from empty', async () => {
    setOrgID('default');
    detailResponse = (id) => response({ id, displayName: 'Default organization' });
    memberResponse = () => response({ code: 'org_load_failed' }, 500);
    render(() => <OrganizationOverviewPanel currentUser={owner} />);
    expect(await screen.findByRole('alert')).toHaveTextContent('Unable to load organization');
    expect(screen.queryByText('No members found.')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
    expect(memberReads()).toHaveLength(1);
    const request = memberReads()[0]!;
    expect(request.url).toBe('/api/orgs/default/members');
    expect(new Headers(request.options.headers).get('X-Pulse-Org-ID')).toBe('default');
    expect(request.options.signal).toBe(
      requests.find((read) => read.url === '/api/orgs/default')?.options.signal,
    );
    memberResponse = () => response([]);
    fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
    expect(await screen.findByText('No members found.')).toBeVisible();
    expect(screen.getByText('Members', { exact: true }).nextElementSibling).toHaveTextContent(
      /^0$/,
    );
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    expect(memberReads()).toHaveLength(2);
    expect(writes()).toHaveLength(0);
  });

  it('discards a late list for an omitted selection after switching workspaces', async () => {
    const previousMembers = deferred<Response>();
    detailResponse = (id) =>
      response(id === orgID ? { ...organization(id), members: undefined } : organization(id));
    memberResponse = () => previousMembers.promise;
    render(() => <OrganizationOverviewPanel currentUser={owner} />);
    await waitFor(() => expect(memberReads()).toHaveLength(1));
    const oldSignal = memberReads()[0]!.options.signal;
    expect(oldSignal).toBe(reads()[0]!.options.signal);
    setOrgID('operations/west');
    eventBus.emit('org_switched');
    expect(oldSignal?.aborted).toBe(true);
    expect(await screen.findByLabelText('Display Name')).toHaveValue('Operations West');
    previousMembers.resolve(response(organization().members));
    await settleFocus();
    expect(screen.getByText('operations/west-viewer')).toBeVisible();
    expect(screen.queryByText('operations/east-viewer')).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(memberReads()).toHaveLength(1);
    expect(writes()).toHaveLength(0);
  });
});

describe('Organization Overview native submission and keyboard continuation', () => {
  it('submits the named form once with trimmed text and exact org/CSRF, retaining pending controls', async () => {
    const { input } = await rename();
    expect(input).toBeDisabled();
    expect(input).toHaveValue(' Operations North ');
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled();
    fireEvent.submit(screen.getByRole('form', { name: 'Organization display name' }));
    expect(writes()).toHaveLength(1);
    const write = writes()[0];
    expect(write.url).toBe('/api/orgs/operations%2Feast');
    expect(JSON.parse(write.options.body as string)).toEqual({ displayName: 'Operations North' });
    const headers = new Headers(write.options.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(orgID);
    expect(headers.get('X-CSRF-Token')).toBe('synthetic-session-csrf');
    expect(headers.has('X-API-Token')).toBe(false);
  });

  it.each(['', '   ', 'Operations East', ' Operations East '])(
    'does not write an empty or unchanged name %j',
    async (draft) => {
      const { input } = await open();
      fireEvent.input(input, { target: { value: draft } });
      submit();
      await settleFocus();
      expect(writes()).toHaveLength(0);
    },
  );

  it.each(['read-only', 'nonmanager'])(
    'does not turn native submission into %s authority',
    async (kind) => {
      if (kind === 'read-only') policy(true);
      const { input } = await open(kind === 'nonmanager' ? 'viewer@example.test' : owner);
      expect(input).toBeDisabled();
      fireEvent.submit(screen.getByRole('form', { name: 'Organization display name' }));
      await settleFocus();
      expect(writes()).toHaveLength(0);
    },
  );

  it('continues in the saved name after success and an equal-reader refresh without another write', async () => {
    const { input } = await rename();
    eventBus.emit('organizations_changed');
    expect(input).toBeInTheDocument();
    expect(reads()).toHaveLength(1);
    savedName = 'Operations North';
    readGate = deferred<void>();
    completion.resolve(response(organization()));
    await waitFor(() => expect(input.isConnected).toBe(false));
    expect(document.activeElement).toBe(document.body);
    readGate.resolve();
    const saved = await screen.findByRole('textbox', { name: 'Display Name' });
    await waitFor(() => expect(saved).toHaveFocus());
    expect(saved).toHaveValue(savedName);
    expect(reads()).toHaveLength(2);
    expect(writes()).toHaveLength(1);
  });

  it.each([false, true])(
    'focuses uncertain feedback, then keeps explicit %s recovery local without replay',
    async (failedRecovery) => {
      await rename();
      savedName = 'Operations North';
      completion.resolve(response({ error: 'Synthetic unavailable response' }, 503));
      const warning = await screen.findByRole('alert');
      await waitFor(() => expect(warning).toHaveFocus());
      expect(warning).toHaveAccessibleName('Organization details unavailable');
      expect(warning).toHaveTextContent('The last name change could not be confirmed.');
      failRead = failedRecovery;
      readGate = deferred<void>();
      const inventory = screen.getByRole('region', { name: 'Organization details' });
      const reload = screen.getByRole('button', { name: 'Reload organization' });
      reload.focus();
      fireEvent.click(reload);
      expect(inventory).toHaveFocus();
      expect(results()).toHaveAttribute('aria-busy', 'true');
      expect(inventory).toHaveAttribute('tabindex', '-1');
      expect(reload.isConnected).toBe(false);
      expect(reads()).toHaveLength(2);
      expect(writes()).toHaveLength(1);
      readGate.resolve();
      if (failedRecovery) {
        await waitFor(() =>
          expect(screen.getByRole('alert')).toHaveTextContent(
            'Unable to load organization details.',
          ),
        );
        expect(inventory).toHaveFocus();
        expect(results()).toHaveAttribute('aria-busy', 'false');
        failRead = false;
        // Simulate Safari's pointer activation, which need not focus a button.
        document.body.focus();
        fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
      }
      const input = await screen.findByRole('textbox', { name: 'Display Name' });
      await settleFocus();
      expect(inventory).toHaveFocus();
      expect(screen.getByRole('region', { name: 'Organization details' })).toBe(inventory);
      expect(results()).toHaveAttribute('aria-busy', 'false');
      expect(input).toHaveValue(savedName);
      expect(writes()).toHaveLength(1);
      expect(reads()).toHaveLength(failedRecovery ? 3 : 2);
    },
  );

  it.each(['success', 'uncertain', 'reload'])(
    'does not steal deliberate external focus after %s',
    async (outcome) => {
      await rename();
      if (outcome === 'reload') {
        completion.resolve(response({}, 503));
        await screen.findByRole('alert');
        readGate = deferred<void>();
        fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
      }
      screen.getByRole('button', { name: 'Outside overview' }).focus();
      if (outcome === 'reload') readGate!.resolve();
      else completion.resolve(response(organization(), outcome === 'success' ? 200 : 503));
      if (outcome === 'uncertain') await screen.findByRole('alert');
      else await screen.findByRole('textbox', { name: 'Display Name' });
      await settleFocus();
      expect(screen.getByRole('button', { name: 'Outside overview' })).toHaveFocus();
      expect(writes()).toHaveLength(1);
    },
  );

  it.each(['organization', 'principal', 'read-only round trip'])(
    'retires old focus intent on %s',
    async (kind) => {
      const { input, setUser } = await open();
      input.focus();
      readGate = deferred<void>();
      eventBus.emit('organizations_changed');
      expect(input.isConnected).toBe(false);
      if (kind === 'organization') {
        setOrgID('operations/west');
        eventBus.emit('org_switched');
      }
      if (kind === 'principal') setUser('replacement@example.test');
      if (kind === 'read-only round trip') {
        policy(true);
        policy(false);
      }
      readGate.resolve();
      await screen.findByRole('textbox', { name: 'Display Name' });
      await settleFocus();
      expect(document.activeElement).toBe(document.body);
      expect(writes()).toHaveLength(0);
    },
  );

  it.each([false, true])(
    'keeps the initial pending read and its failure %s passive',
    async (failedRead) => {
      failRead = failedRead;
      readGate = deferred<void>();
      render(() => <OrganizationOverviewPanel currentUser={owner} />);
      expect(results()).toHaveAttribute('aria-busy', 'true');
      expect(document.activeElement).toBe(document.body);
      readGate.resolve();
      if (failedRead) await screen.findByRole('alert');
      else await screen.findByRole('textbox', { name: 'Display Name' });
      await settleFocus();
      expect(results()).toHaveAttribute('aria-busy', 'false');
      expect(document.activeElement).toBe(document.body);
      expect(writes()).toHaveLength(0);
    },
  );

  it.each([false, true])(
    'does not revive abandoned manual focus after recovery failure %s',
    async (failedRead) => {
      await rename();
      completion.resolve(response({}, 503));
      await screen.findByRole('alert');
      readGate = deferred<void>();
      fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
      const inventory = screen.getByRole('region', { name: 'Organization details' });
      expect(inventory).toHaveFocus();
      const outside = screen.getByRole('button', { name: 'Outside overview' });
      outside.focus();
      outside.blur();
      expect(document.activeElement).toBe(document.body);
      failRead = failedRead;
      readGate.resolve();
      if (failedRead) await screen.findByRole('alert');
      else await screen.findByRole('textbox', { name: 'Display Name' });
      await settleFocus();
      expect(results()).toHaveAttribute('aria-busy', 'false');
      expect(document.activeElement).toBe(document.body);
      expect(writes()).toHaveLength(1);
    },
  );

  it.each(['organization', 'principal', 'read-only', 'hidden surface', 'capability', 'disposal'])(
    'never revives a manual read after %s retirement',
    async (kind) => {
      const { setUser, unmount } = await rename();
      completion.resolve(response({}, 503));
      await screen.findByRole('alert');
      readGate = deferred<void>();
      fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
      expect(screen.getByRole('region', { name: 'Organization details' })).toHaveFocus();
      const outside = screen.getByRole('button', { name: 'Outside overview' });
      outside.focus();
      outside.blur();
      if (kind === 'organization') {
        setOrgID('operations/west');
        eventBus.emit('org_switched');
        setOrgID(orgID);
        eventBus.emit('org_switched');
      } else if (kind === 'principal') {
        setUser('replacement@example.test');
        setUser(owner);
      } else if (kind === 'read-only') {
        policy(true);
        policy(false);
      } else if (kind === 'hidden surface') {
        syncSessionPresentationPolicy({
          presentationPolicy: {
            demoMode: true,
            readOnly: true,
            hideCommercial: false,
            hideUpgrade: true,
          },
        });
        policy();
      } else if (kind === 'capability') {
        const fetchMock = vi.mocked(fetch);
        fetchMock.mockResolvedValueOnce(response({ capabilities: [], limits: [] }));
        await loadRuntimeCapabilities(true);
        fetchMock.mockResolvedValueOnce(response({ capabilities: ['multi_tenant'], limits: [] }));
        await loadRuntimeCapabilities(true);
      } else unmount();
      readGate.resolve();
      if (kind !== 'disposal') await screen.findByRole('textbox', { name: 'Display Name' });
      await settleFocus();
      expect(document.activeElement).toBe(document.body);
      expect(writes()).toHaveLength(1);
    },
  );

  it('keeps an initial read failure passive, but makes explicit nonmanager recovery readable', async () => {
    failRead = true;
    render(() => <OrganizationOverviewPanel currentUser="viewer@example.test" />);
    await screen.findByRole('alert');
    await settleFocus();
    expect(document.activeElement).toBe(document.body);
    failRead = false;
    fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
    const input = await screen.findByRole('textbox', { name: 'Display Name' });
    expect(input).toBeDisabled();
    const inventory = screen.getByRole('region', { name: 'Organization details' });
    await settleFocus();
    expect(inventory).toHaveFocus();
    expect(results()).toHaveAttribute('aria-busy', 'false');
    expect(screen.getByText('Operations East', { exact: true })).toBeInTheDocument();
    expect(writes()).toHaveLength(0);
  });
});

describe('Organization Overview stalled read recovery', () => {
  it.each(['details', 'members'])(
    'replaces a stalled %s read without exposing incomplete or obsolete data',
    async (stage) => {
      const abandoned = deferred<Response>();
      const replacement = deferred<Response>();
      const secondRead = stage === 'members';
      detailResponse = secondRead
        ? (id) => response({ ...organization(id), members: undefined })
        : () => abandoned.promise;
      if (secondRead) memberResponse = () => abandoned.promise;
      render(() => <OrganizationOverviewPanel currentUser={owner} />);

      const pending = secondRead ? memberReads : reads;
      await waitFor(() => expect(pending()).toHaveLength(1));
      const oldSignal = pending()[0]?.options.signal;
      expect(oldSignal).toBe(reads()[0]?.options.signal);
      expect(oldSignal?.aborted).toBe(false);
      const restart = restartButton();
      expect(restart).toHaveAccessibleDescription(
        'Checking saved details for operations/east. Restarting does not repeat a name change.',
      );
      expect(results()).not.toContainElement(restart);
      expect(screen.queryByRole('table')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();

      if (secondRead) {
        detailResponse = (id) =>
          response({ ...organization(id), displayName: 'Current East', members: undefined });
        memberResponse = () => replacement.promise;
      } else detailResponse = () => replacement.promise;
      fireEvent.click(restart);
      expect(restart).toHaveFocus();
      expect(oldSignal?.aborted).toBe(true);
      await waitFor(() => expect(pending()).toHaveLength(2));
      const current = pending()[1]!;
      expect(current.options.signal).toBe(reads()[1]?.options.signal);
      expect(current.options.signal).not.toBe(oldSignal);
      expect(current.options.signal?.aborted).toBe(false);
      expect(new Headers(current.options.headers).get('X-Pulse-Org-ID')).toBe(orgID);
      abandoned.resolve(response(secondRead ? organization().members : organization()));
      await settleFocus();
      expect(results()).toHaveAttribute('aria-busy', 'true');
      expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();

      const members = organization().members.slice(0, 1);
      replacement.resolve(
        response(
          secondRead ? members : { ...organization(), displayName: 'Current East', members },
        ),
      );
      const name = await screen.findByRole('textbox', { name: 'Display Name' });
      expect(name).toHaveValue('Current East');
      await waitFor(() => expect(name).toHaveFocus());
      expect(screen.getByText('Members', { exact: true }).nextElementSibling).toHaveTextContent(
        /^1$/,
      );
      expect(screen.queryByText('operations/east-viewer')).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Restart details check' }),
      ).not.toBeInTheDocument();
      expect(reads()).toHaveLength(2);
      expect(memberReads()).toHaveLength(secondRead ? 2 : 0);
      expect(writes()).toHaveLength(0);
    },
  );

  it('restarts an unconfirmed name change readback, not the rename', async () => {
    await rename();
    completion.resolve(response({}, 503));
    expect(await screen.findByRole('alert')).toHaveTextContent('could not be confirmed');
    const stalled = deferred<void>();
    readGate = stalled;
    fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
    const oldSignal = reads()[1]?.options.signal;
    readGate = undefined;
    failRead = true;
    fireEvent.click(restartButton());
    expect(oldSignal?.aborted).toBe(true);
    const warning = await screen.findByRole('alert');
    expect(warning).toHaveTextContent('Unable to load organization details.');
    await waitFor(() => expect(warning).toHaveFocus());
    failRead = false;
    stalled.resolve();
    await settleFocus();
    expect(warning).toHaveFocus();
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
    expect(await screen.findByLabelText('Display Name')).toHaveValue('Operations East');
    expect(reads()).toHaveLength(4);
    expect(writes()).toHaveLength(1);
  });

  it('does not accept a click from the old workspace before the switch notification', async () => {
    const stalled = deferred<void>();
    readGate = stalled;
    render(() => <OrganizationOverviewPanel currentUser={owner} />);
    const old = restartButton();
    const oldSignal = reads()[0]?.options.signal;
    setOrgID('operations/west');
    fireEvent.click(old);
    expect(reads()).toHaveLength(1);
    expect(oldSignal?.aborted).toBe(false);
    expect(old).not.toHaveFocus();
    eventBus.emit('org_switched');
    expect(oldSignal?.aborted).toBe(true);
    expect(
      await screen.findByRole('button', { name: 'Restart details check' }),
    ).toHaveAccessibleDescription(/operations\/west/);
    stalled.resolve();
    expect(await screen.findByLabelText('Display Name')).toHaveValue('Operations West');
    expect(requests.filter(({ url }) => url === '/api/orgs/operations%2Fwest')).toHaveLength(1);
    expect(writes()).toHaveLength(0);
  });

  it('keeps deliberate outside focus even after a restarted check completes', async () => {
    const stalled = deferred<void>();
    readGate = stalled;
    render(() => (
      <>
        <button type="button">Outside overview</button>
        <OrganizationOverviewPanel currentUser={owner} />
      </>
    ));
    fireEvent.click(restartButton());
    const outside = screen.getByRole('button', { name: 'Outside overview' });
    outside.focus();
    outside.blur();
    stalled.resolve();
    expect(await screen.findByLabelText('Display Name')).toHaveValue('Operations East');
    await settleFocus();
    expect(document.activeElement).toBe(document.body);
    expect(reads()).toHaveLength(2);
    expect(writes()).toHaveLength(0);
  });
});
