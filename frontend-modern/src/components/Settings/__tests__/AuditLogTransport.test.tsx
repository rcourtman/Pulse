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
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn(), info: vi.fn() },
}));

const orgID = 'estate/east';
const token = 'synthetic-audit-read-token';
const firstID = 'login/first';
const selectedID = 'login/selected?revision=2%';
const completedNotice =
  'Signature verification completed. Verified 2, failed 0, errors 0, unavailable 0.';
const requestLog: { url: string; init: RequestInit }[] = [];
let read: (url: string) => Response | Promise<Response>;
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
  expect(
    row(`${estate} selected sign-in`).getByRole('button', { name: /^Verify signature: / }),
  ).toBeEnabled();
};
const expectNoInventory = () => {
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  expect(screen.queryByText('No audit events found')).not.toBeInTheDocument();
  expect(screen.queryByText('No events match Verification on this page')).not.toBeInTheDocument();
  expect(
    screen.queryByRole('status', { name: 'Verification filter results' }),
  ).not.toBeInTheDocument();
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
        return json({ capabilities: ['audit_logging'], limits: [] });
      if (url.startsWith('/api/audit?')) return read(url);
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
  it('keeps an empty Verification result honest and finds a matching signature on the next page', async () => {
    window.history.replaceState({}, '', '/settings/security-audit?event=login&verification=failed');
    window.localStorage.setItem('pulse-audit-page-size', '25');
    const firstPage = Array.from({ length: 25 }, (_, index) => ({
      ...events()[0],
      id: `first-page-${index}`,
      signature: index === 0 ? 'synthetic-signature' : undefined,
    }));
    const laterPage = events('Later').map((event, index) => ({
      ...event,
      id: `later-page-${index}`,
      signature: index === 0 ? 'synthetic-signature' : undefined,
    }));
    read = (url) =>
      json({
        events: url.includes('offset=25') ? laterPage : firstPage,
        total: 27,
        persistentLogging: true,
      });
    verify = () => json(verificationResult(false));
    mount();

    const result = () => screen.getByRole('status', { name: 'Verification filter results' });
    expect(
      await screen.findByRole('heading', { name: 'No events match Verification on this page' }),
    ).toBeInTheDocument();
    expect(result()).toHaveTextContent(
      'Verification filters this page only: 0 of 25 events shown.',
    );
    expect(screen.getByText('Loaded 1-25 of 27 • Page 1 of 2')).toBeInTheDocument();
    expect(screen.getByText('Unchecked: 1')).toBeInTheDocument();
    expect(screen.getByText('Check another page or change the Verification filter.')).toBeVisible();
    expect(screen.queryByText('No audit events found')).not.toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Verify All' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Clear all filters' })).toBeEnabled();
    expectBoundRead(auditRequests()[0], '/api/audit?limit=25&offset=0&event=login');
    expect(verificationRequests()).toHaveLength(0);

    fireEvent.click(screen.getByRole('button', { name: 'Next' }));
    expect(await screen.findByText('Loaded 26-27 of 27 • Page 2 of 2')).toBeVisible();
    expect(result()).toHaveTextContent('Verification filters this page only: 0 of 2 events shown.');
    expectBoundRead(auditRequests()[1], '/api/audit?limit=25&offset=25&event=login');
    expect(verificationRequests()).toHaveLength(0);

    fireEvent.click(screen.getByRole('button', { name: 'Verify All' }));
    expect(await screen.findByText('Later first sign-in')).toBeInTheDocument();
    expect(row('Later first sign-in').getByText('Failed')).toBeInTheDocument();
    expect(screen.queryByText('Later selected sign-in')).not.toBeInTheDocument();
    expect(result()).toHaveTextContent('Verification filters this page only: 1 of 2 events shown.');
    expect(screen.getByText('Failed: 1')).toBeInTheDocument();
    expect(verificationRequests()).toHaveLength(1);
    expectBoundRead(verificationRequests()[0], '/api/audit/later-page-0/verify');

    fireEvent.click(screen.getByRole('button', { name: 'Remove Verification filter' }));
    expect(await screen.findByText('Later selected sign-in')).toBeInTheDocument();
    expect(screen.getByText('Showing 26-27 of 27 • Page 2 of 2')).toBeInTheDocument();
    expect(
      screen.queryByRole('status', { name: 'Verification filter results' }),
    ).not.toBeInTheDocument();
    expect(window.location.search).toBe('?event=login');
    // No delayed user-filter request should appear after the router change.
    await new Promise((resolve) => setTimeout(resolve, 350));
    expect(auditRequests()).toHaveLength(3);
  });

  it('does not suggest turning to another page when all available events fit on one page', async () => {
    window.history.replaceState({}, '', '/settings/security-audit?verification=failed');
    mount();
    expect(
      await screen.findByRole('heading', { name: 'No events match Verification on this page' }),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Change the Verification filter to see other events on this page.'),
    ).toBeVisible();
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
    expect(auditRequests()).toHaveLength(1);
  });

  it.each([
    {
      filters: 'verification=failed',
      query: '',
      description: 'Audit logging is active, but no events have been recorded yet.',
    },
    {
      filters: 'event=startup&verification=failed',
      query: '&event=startup',
      description:
        'No events match the Event, User or Outcome filters. Try adjusting or clearing them.',
    },
  ])(
    'describes the confirmed log result with $filters instead of a local no-match',
    async ({ filters, query, description }) => {
      window.history.replaceState({}, '', `/settings/security-audit?${filters}`);
      read = () => json({ events: [], total: 0, persistentLogging: true });
      mount();
      expect(await screen.findByRole('heading', { name: 'No audit events found' })).toBeVisible();
      expect(screen.getByText(description)).toBeVisible();
      expect(
        screen.queryByRole('status', { name: 'Verification filter results' }),
      ).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
      expect(auditRequests()).toHaveLength(1);
      expectBoundRead(auditRequests()[0], `/api/audit?limit=100&offset=0${query}`);
    },
  );

  it('clears multiple log filters with one fresh query, not one request per filter', async () => {
    window.history.replaceState(
      {},
      '',
      '/settings/security-audit?event=startup&user=reviewer&success=failed&verification=failed',
    );
    read = (url) =>
      url.includes('event=startup')
        ? json({ events: [], total: 0, persistentLogging: true })
        : json(inventory());
    mount();
    expect(await screen.findByRole('heading', { name: 'No audit events found' })).toBeVisible();
    expectBoundRead(
      auditRequests()[0],
      '/api/audit?limit=100&offset=0&event=startup&user=reviewer&success=false',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Clear all filters' }));
    await loaded();
    await new Promise((resolve) => setTimeout(resolve, 350));
    expect(auditRequests()).toHaveLength(2);
    expectBoundRead(auditRequests()[1], '/api/audit?limit=100&offset=0');
    expect(verificationRequests()).toHaveLength(0);
  });

  it('keeps the same focused user filter through a pending and failed read', async () => {
    const response = deferred<Response>();
    read = (url) => (url.includes('user=') ? response.promise : json(inventory()));
    mount();
    await loaded();
    const filter = screen.getByPlaceholderText('Filter by user...');
    filter.focus();
    fireEvent.input(filter, { target: { value: 'sam' } });
    await waitFor(() => expect(auditRequests()).toHaveLength(2));
    expectBoundRead(auditRequests()[1], '/api/audit?limit=100&offset=0&user=sam');
    expect(screen.getByPlaceholderText('Filter by user...')).toBe(filter);
    expect(filter).toHaveFocus();
    expectNoInventory();

    response.resolve(json({ code: 'audit_store_busy' }, 503));
    expect(await screen.findByRole('alert')).toHaveTextContent('Audit log storage is busy.');
    expect(filter).toHaveFocus();
    expectNoInventory();
    expect(auditRequests()).toHaveLength(2);

    fireEvent.click(screen.getByRole('button', { name: /^Clear filters$/ }));
    await loaded();
    expect(filter).toHaveValue('');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(auditRequests()).toHaveLength(3);
    expectBoundRead(auditRequests()[2], '/api/audit?limit=100&offset=0');
    expect(verificationRequests()).toHaveLength(0);
  });

  it('retires a superseded response body as soon as the user edits the query again', async () => {
    const old = pendingBody();
    const replacement = deferred<Response>();
    read = (url) =>
      url.includes('user=')
        ? url.endsWith('user=sam')
          ? old.response
          : replacement.promise
        : json(inventory());
    mount();
    await loaded();
    const filter = screen.getByPlaceholderText('Filter by user...');
    filter.focus();
    fireEvent.input(filter, { target: { value: 'sam' } });
    await waitFor(() => expect(old.response.bodyUsed).toBe(true));
    const oldRead = auditRequests()[1];
    expectBoundRead(oldRead, '/api/audit?limit=100&offset=0&user=sam');

    fireEvent.input(filter, { target: { value: 'sam-reader' } });
    await waitFor(() => expect(oldRead.init.signal?.aborted).toBe(true));
    old.finish(inventory('Obsolete'));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expectNoInventory();
    expect(screen.queryByText('Obsolete selected sign-in')).not.toBeInTheDocument();
    expect(filter).toHaveFocus();

    await waitFor(() => expect(auditRequests()).toHaveLength(3));
    expectBoundRead(auditRequests()[2], '/api/audit?limit=100&offset=0&user=sam-reader');
    replacement.resolve(json(inventory('Current')));
    await loaded('Current');
    expect(filter).toHaveFocus();
    expect(verificationRequests()).toHaveLength(0);
  });

  it('withdraws the filter bar on an organization switch before accepting new data', async () => {
    const old = deferred<Response>();
    const next = deferred<Response>();
    read = (url) => (url.includes('user=') ? old.promise : json(inventory()));
    mount();
    await loaded();
    fireEvent.input(screen.getByPlaceholderText('Filter by user...'), {
      target: { value: 'previous-estate-reviewer' },
    });
    await waitFor(() => expect(auditRequests()).toHaveLength(2));
    read = () => next.promise;
    setOrgID('estate/west');
    eventBus.emit('org_switched', 'estate/west');
    expect(auditRequests()[1].init.signal?.aborted).toBe(true);
    expect(screen.queryByRole('group', { name: 'Audit log filters' })).not.toBeInTheDocument();
    expectNoInventory();
    await waitFor(() => expect(auditRequests()).toHaveLength(3));
    expectBoundRead(auditRequests()[2], '/api/audit?limit=100&offset=0', 'estate/west');

    old.resolve(json(inventory('Obsolete')));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByRole('group', { name: 'Audit log filters' })).not.toBeInTheDocument();
    expectNoInventory();
    next.resolve(json(inventory('West')));
    await loaded('West');
    expect(screen.getByPlaceholderText('Filter by user...')).toHaveValue('');
    await new Promise((resolve) => setTimeout(resolve, 350));
    expect(auditRequests()).toHaveLength(3);
    expect(verificationRequests()).toHaveLength(0);
  });

  it('withdraws the page and its verification counts while a read is pending or refused', async () => {
    window.history.replaceState({}, '', '/settings/security-audit?verification=failed');
    mount();
    expect(await screen.findByText('Loaded 1-2 of 2 • Page 1 of 1')).toBeVisible();
    expect(screen.getByText('Unchecked: 2')).toBeVisible();

    const refresh = deferred<Response>();
    read = () => refresh.promise;
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(screen.getByRole('group', { name: 'Audit log filters' })).toBeVisible();
    expectNoInventory();
    expect(screen.queryByText('Unchecked: 2')).not.toBeInTheDocument();
    expect(screen.queryByText('Loaded 1-2 of 2 • Page 1 of 1')).not.toBeInTheDocument();
    refresh.resolve(json({ error: 'access_denied' }, 403));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Audit log access could not be confirmed.',
    );
    expect(screen.queryByRole('group', { name: 'Audit log filters' })).not.toBeInTheDocument();
    expectNoInventory();
    expect(screen.queryByText('Unchecked: 2')).not.toBeInTheDocument();
    expect(auditRequests()).toHaveLength(2);
    expect(auditRequests().every(({ url }) => url === '/api/audit?limit=100&offset=0')).toBe(true);
  });

  it('names same-user signature actions by their exact saved event ID', async () => {
    const rows = events();
    rows[1].timestamp = rows[0].timestamp;
    read = () => json({ events: rows, total: 2, persistentLogging: true });
    mount();
    await loaded();
    for (const event of rows) {
      const action = screen.getByRole('button', {
        name: `Verify signature: ${event.event} by ${event.user} at ${new Date(event.timestamp).toLocaleString()} (Event ID: ${event.id})`,
      });
      expect(action).toHaveTextContent('Verify');
      expect(action).toBeEnabled();
    }
    expect(verificationRequests()).toHaveLength(0);
  });

  it.each([
    {
      trigger: 'Legend',
      title: 'Audit signature verification',
      description: 'Unsigned: no signature stored for this event',
    },
    {
      trigger: 'Help: Auto-verify limit',
      title: 'Auto-verify limit',
      description: 'Large verification batches can add load. Use a smaller limit on busy systems.',
    },
  ])(
    'opens $title as named help without verifying an event',
    async ({ trigger, title, description }) => {
      mount();
      await loaded();
      const action = screen.getByRole('button', { name: trigger });
      expect(action).toHaveAttribute('type', 'button');
      expect(action).toHaveAttribute('aria-expanded', 'false');
      fireEvent.click(action);
      const help = screen.getByRole('dialog', { name: title });
      expect(action).toHaveAttribute('aria-expanded', 'true');
      expect(help).toHaveAttribute('id', action.getAttribute('aria-controls'));
      expect(help).toHaveTextContent(description);
      if (trigger === 'Legend') {
        for (const definition of [
          'Unavailable: verification not supported by the logger',
          'Not checked: signature exists but has not been verified',
          'Failed: signature mismatch or tampering detected',
          'Error: verification request failed',
        ]) {
          expect(help).toHaveTextContent(definition);
        }
      }
      fireEvent.click(within(help).getByRole('button', { name: 'Close help' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(action).toHaveFocus();
      expect(action).toHaveAttribute('aria-expanded', 'false');
      expect(verificationRequests()).toHaveLength(0);
      expect(auditRequests()).toHaveLength(1);
    },
  );
  it('withdraws open event help with the audit reader', async () => {
    const { reader, setReader } = mount();
    await loaded();
    fireEvent.click(screen.getByRole('button', { name: 'Legend' }));
    expect(
      screen.getByRole('dialog', { name: 'Audit signature verification' }),
    ).toBeInTheDocument();
    setReader({ ...reader(), canRead: false });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Legend' })).not.toBeInTheDocument();
    expectNoInventory();
    expect(verificationRequests()).toHaveLength(0);
  });

  it('verifies only the chosen grid row, with its complete opaque ID and saved reader', async () => {
    const completion = deferred<Response>();
    verify = () => completion.promise;
    mount();
    await loaded();
    expectBoundRead(auditRequests()[0], '/api/audit?limit=100&offset=0');

    fireEvent.click(row().getByRole('button', { name: /^Verify signature: / }));
    await waitFor(() => expect(verificationRequests()).toHaveLength(1));
    expectBoundRead(
      verificationRequests()[0],
      `/api/audit/${encodeURIComponent(selectedID)}/verify`,
    );
    expect(row().getByText('Verifying…')).toBeInTheDocument();
    expect(row().getByRole('button', { name: /^Verify signature: / })).toBeDisabled();
    expect(row('East first sign-in').getByText('Not checked')).toBeInTheDocument();
    fireEvent.click(row().getByRole('button', { name: /^Verify signature: / }));
    expect(verificationRequests()).toHaveLength(1);

    completion.resolve(json(verificationResult()));
    await waitFor(() => expect(row().getByText('Verified')).toBeInTheDocument());
    expect(row('East first sign-in').getByText('Not checked')).toBeInTheDocument();
    expect(screen.getByText('Verified: 1')).toBeInTheDocument();
    expect(screen.getByText('Unchecked: 1')).toBeInTheDocument();
    expect(row().getByRole('button', { name: /^Verify signature: / })).toBeEnabled();
    expect(auditRequests()).toHaveLength(2);
  });

  it('does not attach an old verification body to a refreshed grid row with the same ID', async () => {
    const old = pendingBody();
    const current = deferred<Response>();
    verify = () => old.response;
    mount();
    await loaded();
    fireEvent.click(row().getByRole('button', { name: /^Verify signature: / }));
    await waitFor(() => expect(old.response.bodyUsed).toBe(true));
    const signal = verificationRequests()[0].init.signal;

    read = () => json(inventory('Refreshed'));
    verify = () => current.promise;
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(signal?.aborted).toBe(true);
    await loaded('Refreshed');
    fireEvent.click(
      row('Refreshed selected sign-in').getByRole('button', { name: /^Verify signature: / }),
    );
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
      expect(screen.queryByRole('group', { name: 'Audit log filters' })).not.toBeInTheDocument();
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
    fireEvent.click(row().getByRole('button', { name: /^Verify signature: / }));
    await waitFor(() => expect(row().getByText('Error')).toBeInTheDocument());
    expect(row('East first sign-in').getByText('Not checked')).toBeInTheDocument();
    expect(verificationRequests()).toHaveLength(1);
    expect(getApiToken()).toBe(token);
    expect(getOrgID()).toBe(orgID);

    verify = () => json(verificationResult());
    fireEvent.click(row().getByRole('button', { name: /^Verify signature: / }));
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
    fireEvent.click(
      row('West selected sign-in').getByRole('button', { name: /^Verify signature: / }),
    );
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
