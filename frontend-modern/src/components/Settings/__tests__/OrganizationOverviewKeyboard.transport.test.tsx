import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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
const requests: { url: string; method: string; options: RequestInit }[] = [];
const writes = () => requests.filter((request) => request.method === 'PUT');
const reads = () =>
  requests.filter(
    (request) => request.method === 'GET' && request.url === '/api/orgs/operations%2Feast',
  );
const organization = (id = orgID) => ({
  id,
  displayName: id === orgID ? savedName : 'Operations West',
  ownerUserId: owner,
  createdAt: '2026-10-01T10:00:00Z',
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
      if (url.endsWith('/members'))
        return response([{ userId: owner, role: 'owner', addedAt: '' }]);
      if (failRead) return response({ error: 'Synthetic read unavailable' }, 503);
      return response(organization(decodeURIComponent(url.split('/').at(-1)!)));
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
      expect(inventory).toHaveAttribute('aria-busy', 'true');
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
        expect(inventory).toHaveAttribute('aria-busy', 'false');
        failRead = false;
        // Simulate Safari's pointer activation, which need not focus a button.
        document.body.focus();
        fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
      }
      const input = await screen.findByRole('textbox', { name: 'Display Name' });
      await settleFocus();
      expect(inventory).toHaveFocus();
      expect(screen.getByRole('region', { name: 'Organization details' })).toBe(inventory);
      expect(inventory).toHaveAttribute('aria-busy', 'false');
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
      const inventory = screen.getByRole('region', { name: 'Organization details' });
      expect(inventory).toHaveAttribute('aria-busy', 'true');
      expect(document.activeElement).toBe(document.body);
      readGate.resolve();
      if (failedRead) await screen.findByRole('alert');
      else await screen.findByRole('textbox', { name: 'Display Name' });
      await settleFocus();
      expect(inventory).toHaveAttribute('aria-busy', 'false');
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
      expect(inventory).toHaveAttribute('aria-busy', 'false');
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
        syncSessionPresentationPolicy({ presentationPolicy: { demoMode: true } });
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
    expect(inventory).toHaveAttribute('aria-busy', 'false');
    expect(screen.getByText('Operations East', { exact: true })).toBeInTheDocument();
    expect(writes()).toHaveLength(0);
  });
});
