import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { SSOProvidersPanel } from '../SSOProvidersPanel';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';

// Exercise the actual panel, state owner and authenticated client. Only the
// synthetic HTTP boundary and unrelated toast/logger sinks are substituted.
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));
const fetchMock = vi.fn<typeof fetch>();
const listURL = '/api/security/sso/providers';
const org = 'sso/recovery?east#opaque';
const provider = {
  id: 'saved/provider?realm=lab#opaque',
  name: 'Saved sign-in',
  type: 'oidc' as const,
  enabled: true,
  priority: 1,
  oidcIssuerUrl: 'https://idp.example.test/realms/operations',
  oidc: { issuerUrl: 'https://idp.example.test/realms/operations', clientId: 'pulse' },
};
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
let read: () => Promise<Response>;
let saved: (typeof provider)[];
const reads = () => fetchMock.mock.calls.filter(([url, init]) => url === listURL && !init?.method);
const writes = () => fetchMock.mock.calls.filter(([, init]) => (init?.method ?? 'GET') !== 'GET');
const panel = () => screen.getByRole('region', { name: 'Single Sign-On Providers' });
const retry = () => screen.getByRole('button', { name: 'Retry' });
const add = () => screen.getByRole('button', { name: 'Add OIDC' });
function expectBoundReads() {
  for (const [, init] of reads()) {
    const headers = new Headers(init?.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(org);
    expect(headers.get('X-API-Token')).toBe('synthetic-sso-recovery-token');
    expect(init).toMatchObject({ credentials: 'include', signal: expect.any(AbortSignal) });
  }
  expect(fetchMock.mock.calls.filter(([url]) => url === '/api/health')).toHaveLength(0);
}

beforeEach(() => {
  vi.clearAllMocks();
  setOrgID(org);
  setApiToken('synthetic-sso-recovery-token');
  document.cookie = 'pulse_csrf=synthetic-sso-recovery-csrf';
  saved = [{ ...provider }];
  read = async () => json({}, 503);
  fetchMock.mockImplementation(async (url, init) => {
    if ((init?.method ?? 'GET') !== 'GET') {
      if (init?.method === 'DELETE') saved = [];
      else saved = saved.map((p) => ({ ...p, ...JSON.parse(String(init?.body)) }));
      // The change was applied, but its response cannot confirm that fact.
      return json({ error: 'Synthetic unavailable' }, 502);
    }
    if (url === listURL) return read();
    if (url === '/api/security/status') return json({ publicUrl: 'https://pulse.example.test' });
    if (url === `${listURL}/${encodeURIComponent(provider.id)}`) return json(provider);
    throw new Error('Unexpected fixture request');
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  clearAuth();
  setOrgID(null);
  vi.unstubAllGlobals();
});

describe('explicit SSO provider read recovery', () => {
  for (const canManage of [true, false]) {
    it.each(['saved', 'empty', 'failure'] as const)(
      `keeps the stable inventory focused after %s recovery (canManage=${canManage})`,
      async (outcome) => {
        render(() => <SSOProvidersPanel currentUser="fixture-reader" canManage={canManage} />);
        const button = await screen.findByRole('button', { name: 'Retry' });
        const pending = deferred<Response>();
        read = () => pending.promise;
        button.focus();
        fireEvent.click(button);
        expect(panel()).toHaveFocus();
        expect(panel()).toHaveAttribute('aria-busy', 'true');
        expect(button).not.toBeInTheDocument();
        expect(add()).toBeDisabled();
        expect(reads()).toHaveLength(2);
        pending.resolve(
          outcome === 'failure'
            ? json({}, 503)
            : json({ providers: outcome === 'empty' ? [] : saved }),
        );
        await waitFor(() => expect(panel()).toHaveAttribute('aria-busy', 'false'));
        expect(panel()).toHaveFocus();
        if (outcome === 'saved') expect(screen.getByText(provider.oidcIssuerUrl)).toBeVisible();
        if (outcome === 'empty')
          expect(screen.getByText('No SSO providers configured')).toBeVisible();
        if (outcome === 'failure') {
          expect(retry()).toBeEnabled();
          expect(screen.queryByText('No SSO providers configured')).toBeNull();
        }
        if (!canManage || outcome === 'failure') expect(add()).toBeDisabled();
        else expect(add()).toBeEnabled();
        expect(writes()).toHaveLength(0);
        expectBoundReads();
      },
    );
  }

  it.each(['saved', 'failure'] as const)(
    'does not reclaim deliberate focus when the %s response arrives',
    async (outcome) => {
      render(() => (
        <>
          <SSOProvidersPanel />
          <button type="button">Next setting</button>
        </>
      ));
      const button = await screen.findByRole('button', { name: 'Retry' });
      const pending = deferred<Response>();
      read = () => pending.promise;
      button.focus();
      fireEvent.click(button);
      expect(panel()).toHaveFocus();
      const next = screen.getByRole('button', { name: 'Next setting' });
      next.focus();
      pending.resolve(outcome === 'failure' ? json({}, 503) : json({ providers: saved }));
      await waitFor(() => expect(panel()).toHaveAttribute('aria-busy', 'false'));
      await new Promise((done) => setTimeout(done, 10));
      expect(next).toHaveFocus();
      expect(writes()).toHaveLength(0);
    },
  );

  it.each(['toggle', 'edit', 'delete'] as const)(
    'keeps %s uncertainty checks local through dialog release, failed read and saved read without replay',
    async (operation) => {
      read = async () => json({ providers: saved });
      render(() => <SSOProvidersPanel />);
      await waitFor(() => expect(add()).toBeEnabled());
      if (operation === 'toggle') {
        fireEvent.click(screen.getByRole('button', { name: 'Disable Saved sign-in' }));
      } else {
        fireEvent.click(
          screen.getByRole('button', {
            name:
              operation === 'edit'
                ? 'Edit provider Saved sign-in'
                : 'Delete provider Saved sign-in',
          }),
        );
        const dialog = within(await screen.findByRole('dialog'));
        if (operation === 'edit') {
          fireEvent.input(dialog.getByLabelText('Provider Name'), {
            target: { value: 'Changed sign-in' },
          });
          fireEvent.submit(dialog.getByRole('button', { name: 'Save Changes' }).closest('form')!);
        } else fireEvent.click(dialog.getByRole('button', { name: 'Delete Provider' }));
      }
      const check = await screen.findByRole('button', {
        name: operation === 'edit' ? 'Close draft and check providers' : 'Check saved providers',
      });
      await waitFor(() => expect(check).toBeEnabled());
      const pending = deferred<Response>();
      read = () => pending.promise;
      check.focus();
      fireEvent.click(check);
      expect(screen.queryByRole('dialog')).toBeNull();
      expect(panel().closest('[inert]')).toBeNull();
      expect(panel()).toHaveFocus();
      expect(panel()).toHaveAttribute('aria-busy', 'true');
      // The former warning timer must not redirect the explicit check later.
      await new Promise((done) => setTimeout(done, 10));
      expect(panel()).toHaveFocus();
      pending.resolve(json({}, 503));
      await screen.findByText('Unable to load SSO providers. Configured providers are unknown.');
      expect(panel()).toHaveFocus();
      expect(add()).toBeDisabled();
      expect(screen.getByText(/may already have been applied/)).toBeVisible();
      const nextRead = deferred<Response>();
      read = () => nextRead.promise;
      const recheck = screen.getByRole('button', { name: 'Check saved providers' });
      recheck.focus();
      fireEvent.click(recheck);
      expect(panel()).toHaveFocus();
      nextRead.resolve(json({ providers: saved }));
      await waitFor(() => expect(add()).toBeEnabled());
      expect(panel()).toHaveFocus();
      expect(document.body).not.toHaveTextContent('could not be confirmed');
      if (operation === 'edit') expect(screen.getByText('Changed sign-in')).toBeVisible();
      if (operation === 'delete')
        expect(screen.getByText('No SSO providers configured')).toBeVisible();
      if (operation === 'toggle')
        expect(screen.getByRole('button', { name: 'Enable Saved sign-in' })).toBeVisible();
      expect(writes()).toHaveLength(1);
      expect(reads()).toHaveLength(3);
      expectBoundReads();
    },
  );

  it('ignores a prior reader completion after an organization round trip without restoring focus', async () => {
    const [user] = createSignal('fixture-reader');
    render(() => (
      <>
        <SSOProvidersPanel currentUser={user()} />
        <button type="button">Next setting</button>
      </>
    ));
    const button = await screen.findByRole('button', { name: 'Retry' });
    const pending = deferred<Response>();
    read = () => pending.promise;
    button.focus();
    fireEvent.click(button);
    const oldRead = reads()[1][1]!;
    read = async () => json({ providers: [] });
    setOrgID('fixture-west');
    eventBus.emit('org_switched', 'fixture-west');
    setOrgID(org);
    eventBus.emit('org_switched', org);
    await screen.findByText('No SSO providers configured');
    const next = screen.getByRole('button', { name: 'Next setting' });
    next.focus();
    pending.resolve(json({ providers: [provider] }));
    await new Promise((done) => setTimeout(done, 10));
    expect(oldRead.signal?.aborted).toBe(true);
    expect(next).toHaveFocus();
    expect(screen.queryByText(provider.oidcIssuerUrl)).toBeNull();
    expect(writes()).toHaveLength(0);
  });
});
