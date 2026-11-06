import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { OrganizationOverviewPanel } from '../OrganizationOverviewPanel';
import { clearApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';

const feedback = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: feedback }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), debug: vi.fn() } }));

const orgID = 'client-operations';
const owner = 'owner@example.test';
const savedName = 'Operations North';
const response = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
let completion: ReturnType<typeof deferred<Response>>;
let applied = false;
let failCapabilities = false;
let failRecovery = false;
let mutationSignal: AbortSignal | undefined;
const requests: { url: string; method: string; options: RequestInit }[] = [];
const writes = () => requests.filter((request) => request.method === 'PUT');
const reads = () =>
  requests.filter((request) => request.url === `/api/orgs/${orgID}` && request.method === 'GET');
const policy = (readOnly = false, demoMode = false) =>
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode,
      readOnly,
      hideCommercial: demoMode,
      hideUpgrade: true,
    },
  });
const organization = (id = orgID) => ({
  id,
  displayName: id === orgID ? (applied ? savedName : 'Operations') : 'Peer Client',
  ownerUserId: id === orgID ? owner : 'peer-owner@example.test',
});

beforeEach(async () => {
  completion = deferred<Response>();
  applied = false;
  failCapabilities = false;
  failRecovery = false;
  mutationSignal = undefined;
  requests.length = 0;
  feedback.success.mockReset();
  feedback.error.mockReset();
  clearApiToken();
  setOrgID(orgID);
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
  policy();
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, options: RequestInit) => {
      const method = options.method || 'GET';
      requests.push({ url, method, options });
      if (url === '/api/license/runtime-capabilities') {
        if (failCapabilities) throw new Error('Synthetic unavailable capabilities');
        return response({ capabilities: ['multi_tenant'], limits: [], max_history_days: 7 });
      }
      if (method === 'PUT') {
        mutationSignal = options.signal as AbortSignal;
        return completion.promise;
      }
      if (url === `/api/orgs/${orgID}`) {
        if (failRecovery) return response({ error: 'Synthetic unavailable saved read' }, 503);
        return response(organization());
      }
      if (url === '/api/orgs/peer-client') return response(organization('peer-client'));
      if (url.endsWith('/members'))
        return response([{ userId: owner, role: 'owner', addedAt: '' }]);
      throw new Error(`Unexpected fixture request: ${method} ${url}`);
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

const open = async () => {
  const [user, setUser] = createSignal(owner);
  const view = render(() => <OrganizationOverviewPanel currentUser={user()} />);
  const input = await screen.findByRole('textbox', { name: 'Display Name' });
  expect(input).toHaveValue('Operations');
  const submit = () => {
    fireEvent.input(screen.getByLabelText('Display Name'), { target: { value: ` ${savedName} ` } });
    fireEvent.click(screen.getByRole('button', { name: 'Save', exact: true }));
  };
  submit();
  await waitFor(() => expect(writes()).toHaveLength(1));
  return { ...view, setUser };
};
const unchangedRefresh = async (kind = 'combined') => {
  if (kind === 'directory' || kind === 'combined') {
    eventBus.emit('organizations_changed');
    eventBus.emit('organizations_changed');
  }
  if (kind === 'policy' || kind === 'combined') policy();
  if (kind === 'capability' || kind === 'combined') await loadRuntimeCapabilities(true);
};

// Use the production panel, capability/presentation stores, OrgsAPI and HTTP
// client so a same-ID capability reset cannot be mistaken for an equal reader.
describe('Overview pending renames with the real store and HTTP client', () => {
  it.each(
    ['directory', 'policy', 'capability'].flatMap((refresh) =>
      ['success', 'uncertain', 'mismatched response'].map((outcome) => ({ refresh, outcome })),
    ),
  )(
    'preserves a pending $outcome through equal $refresh refreshes',
    async ({ refresh, outcome }) => {
      await open();
      await unchangedRefresh(refresh);
      expect(mutationSignal?.aborted).toBe(false);
      expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled();
      expect(screen.getByLabelText('Display Name')).toBeDisabled();
      expect(screen.getByLabelText('Display Name')).toHaveValue(` ${savedName} `);
      expect(reads()).toHaveLength(1);
      fireEvent.click(screen.getByRole('button', { name: 'Saving...' }));
      expect(writes()).toHaveLength(1);
      const request = writes()[0];
      expect(request.url).toBe(`/api/orgs/${orgID}`);
      expect(JSON.parse(request.options.body as string)).toEqual({ displayName: savedName });
      expect(new Headers(request.options.headers).get('X-Pulse-Org-ID')).toBe(orgID);
      expect(new Headers(request.options.headers).get('X-CSRF-Token')).toBe('synthetic-csrf');
      applied = true;
      if (outcome === 'uncertain') completion.reject(new Error('Response lost after apply'));
      else
        completion.resolve(response(organization(outcome === 'success' ? orgID : 'peer-client')));
      if (outcome !== 'success') {
        expect(await screen.findByRole('alert')).toHaveTextContent(
          'The last name change could not be confirmed.',
        );
        expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
        expect(reads()).toHaveLength(1);
        expect(feedback.success).not.toHaveBeenCalled();
        expect(feedback.error).toHaveBeenCalledExactlyOnceWith(
          'Unable to update the organization name.',
        );
        fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
      } else {
        await waitFor(() => expect(feedback.success).toHaveBeenCalledTimes(1));
        expect(feedback.error).not.toHaveBeenCalled();
      }
      await waitFor(() => expect(screen.getByLabelText('Display Name')).toHaveValue(savedName));
      expect(reads()).toHaveLength(2);
      expect(writes()).toHaveLength(1);
    },
  );

  it('keeps failed recovery unknown without replaying the rename', async () => {
    await open();
    await unchangedRefresh();
    applied = true;
    completion.reject(new Error('Response lost after apply'));
    await screen.findByRole('alert');
    failRecovery = true;
    const reload = screen.getByRole('button', { name: 'Reload organization' });
    fireEvent.click(reload);
    fireEvent.click(reload);
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Unable to load organization details.',
    );
    expect(screen.queryByText('No members found.')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();
    expect(reads()).toHaveLength(2);
    failRecovery = false;
    fireEvent.click(screen.getByRole('button', { name: 'Reload organization' }));
    await waitFor(() => expect(screen.getByLabelText('Display Name')).toHaveValue(savedName));
    expect(reads()).toHaveLength(3);
    expect(writes()).toHaveLength(1);
  });

  it.each(
    (
      [
        'organization',
        'principal',
        'read-only',
        'hidden',
        'capability failure',
        'same-ID rebind',
        'disposal',
      ] as const
    ).flatMap((withdrawal) =>
      (['success', 'failure'] as const).map((outcome) => ({ withdrawal, outcome })),
    ),
  )('withdraws $withdrawal before a late $outcome', async ({ withdrawal, outcome }) => {
    const view = await open();
    await unchangedRefresh();
    expect(mutationSignal?.aborted).toBe(false);
    switch (withdrawal) {
      case 'organization':
        setOrgID('peer-client');
        eventBus.emit('org_switched', 'peer-client');
        await waitFor(() =>
          expect(screen.getByLabelText('Display Name')).toHaveValue('Peer Client'),
        );
        break;
      case 'principal':
        view.setUser('viewer@example.test');
        await waitFor(() =>
          expect(screen.getByRole('button', { name: 'Save', exact: true })).toBeDisabled(),
        );
        break;
      case 'read-only':
        policy(true);
        await waitFor(() =>
          expect(screen.getByRole('button', { name: 'Save', exact: true })).toBeDisabled(),
        );
        break;
      case 'hidden':
        policy(true, true);
        await screen.findByText('Organization settings are not available on this server.');
        break;
      case 'capability failure':
        failCapabilities = true;
        await loadRuntimeCapabilities(true);
        await screen.findByText('Organization settings are not available on this server.');
        break;
      case 'same-ID rebind':
        eventBus.emit('org_switched', orgID);
        await waitFor(() =>
          expect(screen.getByRole('button', { name: 'Save', exact: true })).toBeEnabled(),
        );
        break;
      case 'disposal':
        view.unmount();
    }
    expect(mutationSignal?.aborted).toBe(true);
    const readsAfterWithdrawal = reads().length;
    applied = true;
    if (outcome === 'success') completion.resolve(response(organization()));
    else completion.reject(new Error('Late failure after withdrawal'));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(feedback.success).not.toHaveBeenCalled();
    expect(feedback.error).not.toHaveBeenCalled();
    expect(screen.queryByText(/could not be confirmed/)).not.toBeInTheDocument();
    expect(reads()).toHaveLength(readsAfterWithdrawal);
    expect(writes()).toHaveLength(1);
  });
});
