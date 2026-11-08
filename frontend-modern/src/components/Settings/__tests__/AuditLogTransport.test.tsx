import { createSignal } from 'solid-js';
import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AuditLogPanel from '../AuditLogPanel';
import type { AuditEvent, AuditLogPanelProps } from '../useAuditLogPanelState';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';

vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const orgID = 'estate/east';
const token = 'synthetic-audit-read-token';
const firstID = 'login/first';
const selectedID = 'login/selected?revision=2%';
const completedNotice =
  'Signature verification completed. Verified 2, failed 0, errors 0, unavailable 0.';
const requestLog: { url: string; init: RequestInit }[] = [];
let read: () => Response | Promise<Response>;
let verify: (url: string) => Response | Promise<Response>;

const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
// Exercise the real Response.json boundary, not an apiFetch/json mock that
// could hide transport-side retries or a body admitted after its reader left.
const pendingBody = () => {
  let finish!: (body: unknown) => void;
  const response = new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        finish = (body) => {
          controller.enqueue(new TextEncoder().encode(JSON.stringify(body)));
          controller.close();
        };
      },
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  return { response, finish };
};
const events = (estate = 'East'): AuditEvent[] => [
  {
    id: firstID,
    timestamp: '2026-11-08T04:00:00Z',
    event: 'login',
    user: 'same-reader',
    ip: '192.0.2.1',
    success: true,
    details: `${estate} first sign-in`,
    signature: 'synthetic-signature-first',
  },
  {
    id: selectedID,
    timestamp: '2026-11-08T04:01:00Z',
    event: 'login',
    user: 'same-reader',
    ip: '192.0.2.2',
    success: true,
    details: `${estate} selected sign-in`,
    signature: 'synthetic-signature-selected',
  },
];
const inventory = (estate = 'East') => ({
  events: events(estate),
  total: 2,
  persistentLogging: true,
});
const verificationResult = (verified = true) => ({ available: true, verified });
const row = (details = 'East selected sign-in') => {
  const element = screen.getByText(details).closest('tr');
  expect(element).not.toBeNull();
  return within(element!);
};
const auditRequests = () => requestLog.filter(({ url }) => url.startsWith('/api/audit'));
const verificationRequests = () => auditRequests().filter(({ url }) => url.endsWith('/verify'));
const expectBoundRead = (
  request: (typeof requestLog)[number],
  url: string,
  expectedOrg = orgID,
) => {
  expect(request.url).toBe(url);
  expect(request.init.method ?? 'GET').toBe('GET');
  expect(request.init.credentials).toBe('include');
  expect(new Headers(request.init.headers).get('X-Pulse-Org-ID')).toBe(expectedOrg);
  expect(new Headers(request.init.headers).get('X-API-Token')).toBe(token);
  expect(new Headers(request.init.headers).get('X-Requested-With')).toBe('XMLHttpRequest');
  expect(request.init.body).toBeUndefined();
  expect(request.init).not.toHaveProperty('retry');
  expect(request.init).not.toHaveProperty('expectedOrgID');
  expect(request.init.signal).toBeInstanceOf(AbortSignal);
};
const mount = () => {
  const [reader, setReader] = createSignal<AuditLogPanelProps>({
    currentUser: 'same-reader',
    accessKey: 'synthetic-audit-scope',
    canRead: true,
  });
  const view = render(() => (
    <Router>
      <Route path="/*" component={() => <AuditLogPanel {...reader()} />} />
    </Router>
  ));
  return { ...view, reader, setReader };
};
const loaded = async (estate = 'East') => {
  await screen.findByText(`${estate} selected sign-in`);
  expect(row(`${estate} selected sign-in`).getByRole('button', { name: 'Verify' })).toBeEnabled();
};
const expectNoInventory = () => {
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  expect(screen.queryByText('No audit events found')).not.toBeInTheDocument();
  expect(screen.queryByText('Verified: 2')).not.toBeInTheDocument();
  expect(screen.queryByText(completedNotice)).not.toBeInTheDocument();
};

beforeEach(async () => {
  window.sessionStorage.clear();
  window.localStorage.clear();
  clearAuth();
  setOrgID(orgID);
  setApiToken(token);
  // A retry-enabled 403 would otherwise be replayed by the actual HTTP client.
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
  window.localStorage.setItem('pulse-audit-auto-verify', 'false');
  window.history.replaceState({}, '', '/settings/security-audit');
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: false,
      hideUpgrade: true,
    },
  });
  requestLog.length = 0;
  read = () => json(inventory());
  verify = () => json(verificationResult());
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: string | URL | Request, init: RequestInit = {}) => {
      const url = String(input);
      requestLog.push({ url, init });
      if (url === '/api/license/runtime-capabilities')
        return json({ capabilities: ['audit_logging'], limits: [], max_history_days: 7 });
      if (url.startsWith('/api/audit?')) return read();
      if (url.startsWith('/api/audit/') && url.endsWith('/verify')) return verify(url);
      throw new Error(`Unexpected synthetic request: ${url}`);
    }),
  );
  await loadRuntimeCapabilities(true);
});

afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  window.history.replaceState({}, '', '/');
  vi.unstubAllGlobals();
});

describe('Audit Log rendered controls through the real HTTP client', () => {
  it('verifies only the chosen grid row, with its complete opaque ID and saved reader', async () => {
    const completion = deferred<Response>();
    verify = () => completion.promise;
    mount();
    await loaded();
    expectBoundRead(auditRequests()[0], '/api/audit?limit=100&offset=0');

    fireEvent.click(row().getByRole('button', { name: 'Verify' }));
    await waitFor(() => expect(verificationRequests()).toHaveLength(1));
    expectBoundRead(
      verificationRequests()[0],
      `/api/audit/${encodeURIComponent(selectedID)}/verify`,
    );
    expect(row().getByText('Verifying…')).toBeInTheDocument();
    expect(row().getByRole('button', { name: 'Verify' })).toBeDisabled();
    expect(row('East first sign-in').getByText('Not checked')).toBeInTheDocument();
    fireEvent.click(row().getByRole('button', { name: 'Verify' }));
    expect(verificationRequests()).toHaveLength(1);

    completion.resolve(json(verificationResult()));
    await waitFor(() => expect(row().getByText('Verified')).toBeInTheDocument());
    expect(row('East first sign-in').getByText('Not checked')).toBeInTheDocument();
    expect(screen.getByText('Verified: 1')).toBeInTheDocument();
    expect(screen.getByText('Unchecked: 1')).toBeInTheDocument();
    expect(row().getByRole('button', { name: 'Verify' })).toBeEnabled();
    expect(auditRequests()).toHaveLength(2);
  });

  it('does not attach an old verification body to a refreshed grid row with the same ID', async () => {
    const old = pendingBody();
    const current = deferred<Response>();
    verify = () => old.response;
    mount();
    await loaded();
    fireEvent.click(row().getByRole('button', { name: 'Verify' }));
    await waitFor(() => expect(old.response.bodyUsed).toBe(true));
    const signal = verificationRequests()[0].init.signal;

    read = () => json(inventory('Refreshed'));
    verify = () => current.promise;
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(signal?.aborted).toBe(true);
    await loaded('Refreshed');
    fireEvent.click(row('Refreshed selected sign-in').getByRole('button', { name: 'Verify' }));
    await waitFor(() => expect(verificationRequests()).toHaveLength(2));

    old.finish(verificationResult(false));
    // A task boundary lets the native body parser and obsolete continuation run.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(row('Refreshed selected sign-in').getByText('Verifying…')).toBeInTheDocument();
    expect(screen.queryByText('Failed: 1')).not.toBeInTheDocument();
    current.resolve(json(verificationResult()));
    await waitFor(() =>
      expect(row('Refreshed selected sign-in').getByText('Verified')).toBeInTheDocument(),
    );
    expect(row('Refreshed first sign-in').getByText('Not checked')).toBeInTheDocument();
    expect(auditRequests()).toHaveLength(4);
  });

  it.each([
    { selectedOrg: orgID, error: 'access_denied' },
    { selectedOrg: 'default', error: 'default_org_access_denied' },
  ])(
    'withdraws the whole batch on $error without replay, retargeting or credential loss',
    async ({ selectedOrg, error }) => {
      setOrgID(selectedOrg);
      verify = () => json({ error, detail: 'synthetic-private-detail' }, 403);
      mount();
      await loaded();
      fireEvent.click(screen.getByRole('button', { name: 'Verify All' }));

      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Audit log access could not be confirmed. Refresh after access is restored.',
      );
      expectNoInventory();
      expect(screen.queryByText(/synthetic-private-detail/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Verified 2/)).not.toBeInTheDocument();
      expect(verificationRequests()).toHaveLength(1);
      expectBoundRead(
        verificationRequests()[0],
        `/api/audit/${encodeURIComponent(firstID)}/verify`,
        selectedOrg,
      );
      expect(getApiToken()).toBe(token);
      expect(getOrgID()).toBe(selectedOrg);
      expect(window.localStorage.getItem('just_logged_out')).toBeNull();
      expect(screen.getByRole('button', { name: 'Verify All' })).toBeDisabled();

      // Recovery is an explicit read, not an automatically retried verification.
      fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
      await loaded();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(row().getByText('Not checked')).toBeInTheDocument();
      expect(row('East first sign-in').getByText('Not checked')).toBeInTheDocument();
      expect(verificationRequests()).toHaveLength(1);
      expect(auditRequests()).toHaveLength(3);
      expectBoundRead(auditRequests()[2], '/api/audit?limit=100&offset=0', selectedOrg);
    },
  );

  it('keeps throttled verification unknown until another explicit row click', async () => {
    verify = () => json({ error: 'rate_limited' }, 429, { 'Retry-After': '1' });
    mount();
    await loaded();
    fireEvent.click(row().getByRole('button', { name: 'Verify' }));
    await waitFor(() => expect(row().getByText('Error')).toBeInTheDocument());
    expect(row('East first sign-in').getByText('Not checked')).toBeInTheDocument();
    expect(verificationRequests()).toHaveLength(1);
    expect(getApiToken()).toBe(token);
    expect(getOrgID()).toBe(orgID);

    verify = () => json(verificationResult());
    fireEvent.click(row().getByRole('button', { name: 'Verify' }));
    await waitFor(() => expect(row().getByText('Verified')).toBeInTheDocument());
    expect(verificationRequests()).toHaveLength(2);
    for (const request of verificationRequests())
      expectBoundRead(request, `/api/audit/${encodeURIComponent(selectedID)}/verify`);
    expect(auditRequests()).toHaveLength(3);
  });

  it('cancels a rendered batch immediately and verifies again only through explicit Resume', async () => {
    const old = pendingBody();
    verify = () => old.response;
    mount();
    await loaded();
    fireEvent.click(screen.getByRole('button', { name: 'Verify All' }));
    await waitFor(() => expect(old.response.bodyUsed).toBe(true));
    const signal = verificationRequests()[0].init.signal;
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(signal?.aborted).toBe(true);
    expect(screen.getByText('Verification canceled')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Verify All' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Resume (2)' })).toBeEnabled();

    old.finish(verificationResult());
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(verificationRequests()).toHaveLength(1);
    expect(row().getByText('Not checked')).toBeInTheDocument();
    expect(row('East first sign-in').getByText('Not checked')).toBeInTheDocument();
    expect(screen.queryByText(completedNotice)).not.toBeInTheDocument();
    verify = () => json(verificationResult());
    fireEvent.click(screen.getByRole('button', { name: 'Resume (2)' }));
    expect(await screen.findByRole('status')).toHaveTextContent(completedNotice);
    expect(verificationRequests()).toHaveLength(3);
    expectBoundRead(verificationRequests()[1], `/api/audit/${encodeURIComponent(firstID)}/verify`);
    expectBoundRead(
      verificationRequests()[2],
      `/api/audit/${encodeURIComponent(selectedID)}/verify`,
    );
    expect(row().getByText('Verified')).toBeInTheDocument();
    expect(row('East first sign-in').getByText('Verified')).toBeInTheDocument();
    expect(screen.queryByText('Verification canceled')).not.toBeInTheDocument();
    expect(auditRequests()).toHaveLength(4);
  });

  it('withdraws an in-progress inventory body across an organization switch before accepting namesake rows', async () => {
    const old = pendingBody();
    read = () => old.response;
    mount();
    await waitFor(() => expect(old.response.bodyUsed).toBe(true));
    const firstRead = auditRequests()[0];
    read = () => json(inventory('West'));
    setOrgID('estate/west');
    eventBus.emit('org_switched', 'estate/west');
    expect(firstRead.init.signal?.aborted).toBe(true);
    await loaded('West');

    old.finish(inventory());
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText('East selected sign-in')).not.toBeInTheDocument();
    fireEvent.click(row('West selected sign-in').getByRole('button', { name: 'Verify' }));
    await waitFor(() =>
      expect(row('West selected sign-in').getByText('Verified')).toBeInTheDocument(),
    );
    expect(verificationRequests()).toHaveLength(1);
    expectBoundRead(
      verificationRequests()[0],
      `/api/audit/${encodeURIComponent(selectedID)}/verify`,
      'estate/west',
    );
    expect(auditRequests()).toHaveLength(3);
  });

  it.each(['principal', 'credential', 'read access'] as const)(
    'keeps a completed batch notice and rows out of a withdrawn %s',
    async (withdrawal) => {
      const view = mount();
      await loaded();
      fireEvent.click(screen.getByRole('button', { name: 'Verify All' }));
      expect(await screen.findByRole('status')).toHaveTextContent(completedNotice);
      expect(verificationRequests()).toHaveLength(2);
      const recovery = deferred<Response>();
      read = () => recovery.promise;
      if (withdrawal === 'principal') view.setReader({ ...view.reader(), currentUser: 'reader-b' });
      if (withdrawal === 'credential') {
        setApiToken('synthetic-replacement-token');
        view.setReader({ ...view.reader(), accessKey: 'synthetic-new-scope' });
      }
      if (withdrawal === 'read access') view.setReader({ ...view.reader(), canRead: false });

      expect(screen.queryByText(completedNotice)).not.toBeInTheDocument();
      expectNoInventory();
      fireEvent.click(screen.getByRole('button', { name: 'Verify All' }));
      expect(verificationRequests()).toHaveLength(2);
      if (withdrawal === 'read access') {
        expect(screen.getByRole('status')).toHaveTextContent(
          'Audit log is unavailable for this session.',
        );
        expect(auditRequests()).toHaveLength(3);
        view.setReader({ ...view.reader(), canRead: true });
      }
      await waitFor(() => expect(auditRequests()).toHaveLength(4));
      recovery.resolve(json(inventory('Current')));
      await loaded('Current');
      expect(row('Current selected sign-in').getByText('Not checked')).toBeInTheDocument();
      expect(screen.queryByText(completedNotice)).not.toBeInTheDocument();
      expect(verificationRequests()).toHaveLength(2);
      const headers = new Headers(auditRequests()[3].init.headers);
      expect(headers.get('X-Pulse-Org-ID')).toBe(orgID);
      expect(headers.get('X-API-Token')).toBe(
        withdrawal === 'credential' ? 'synthetic-replacement-token' : token,
      );
    },
  );
});
