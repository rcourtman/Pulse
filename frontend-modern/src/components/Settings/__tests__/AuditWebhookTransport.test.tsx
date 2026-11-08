import { createSignal } from 'solid-js';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AuditWebhookPanel } from '../AuditWebhookPanel';
import type { AuditWebhookPanelProps } from '../useAuditWebhookPanelState';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';

// Keep the panel, confirmation dialog, capability store and HTTP client real.
// Mocking apiFetchJSON cannot witness dispatch after cold CSRF preparation or
// retries of a full-list replacement. No synthetic URL is ever contacted.
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const endpoint = '/api/admin/webhooks/audit';
const orgID = 'estate/east';
const token = 'synthetic-audit-webhook-token';
const csrf = 'synthetic-cold-csrf';
const firstURL = 'https://audit.example.test/events?receiver=first';
const selectedURL = 'https://audit.example.test/events?receiver=second%2Fopaque';
const addedURL = 'https://audit.example.test/events?receiver=third';
const otherURL = 'https://other.example.test/events';
const readError = 'Audit webhook configuration could not be loaded.';
const saveError = 'Failed to save webhook configuration';
const empty = 'No audit webhooks configured yet.';
const success = 'Audit webhooks updated';
const requests: { url: string; init: RequestInit }[] = [];
let inventory: string[];
let capabilities: string[];
let readReply: () => Response | Promise<Response>;
let writeReply: (init: RequestInit) => Response | Promise<Response>;
let healthReply: () => Response | Promise<Response>;

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
const primeCSRF = () => {
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
};
const acceptWrite = (init: RequestInit) => {
  inventory = JSON.parse(String(init.body)).urls;
  return new Response(null, { status: 204 });
};
const reads = () => requests.filter(({ url, init }) => url === endpoint && !init.method);
const writes = () => requests.filter(({ init }) => init.method === 'POST');
const healthReads = () => requests.filter(({ url }) => url === '/api/health');
const input = () => screen.getByRole('textbox', { name: 'Audit webhook URL' });
const add = (draft = addedURL) => {
  fireEvent.input(input(), { target: { value: draft } });
  fireEvent.keyDown(input(), { key: 'Enter' });
};
const openRemove = (url = selectedURL) => {
  // Locate the actual endpoint card, not the first matching remove button.
  const card = screen.getByTitle(url).parentElement!.parentElement!;
  fireEvent.click(within(card).getByRole('button', { name: 'Remove webhook endpoint' }));
  return screen.getByRole('dialog', { name: 'Remove audit webhook' });
};
const confirmRemove = () =>
  fireEvent.click(
    within(screen.getByRole('dialog', { name: 'Remove audit webhook' })).getByRole('button', {
      name: 'Remove webhook',
    }),
  );
const mount = (props: AuditWebhookPanelProps = {}) => {
  const [reader, setReader] = createSignal<AuditWebhookPanelProps>({
    currentUser: 'same-admin',
    canRead: true,
    canManage: true,
    ...props,
  });
  return { ...render(() => <AuditWebhookPanel {...reader()} />), reader, setReader };
};
const loaded = async (urls = [firstURL, selectedURL]) => {
  for (const url of urls) await screen.findByTitle(url);
  await waitFor(() =>
    expect(screen.getAllByRole('button', { name: 'Remove webhook endpoint' })).toHaveLength(
      urls.length,
    ),
  );
};
const expectWire = (
  request: (typeof requests)[number],
  method: 'GET' | 'POST',
  expectedOrg = orgID,
) => {
  expect(request.url).toBe(endpoint);
  expect(request.init.method ?? 'GET').toBe(method);
  expect(request.init.credentials).toBe('include');
  expect(request.init.signal).toBeInstanceOf(AbortSignal);
  expect(request.init).not.toHaveProperty('expectedOrgID');
  expect(request.init).not.toHaveProperty('retry');
  const headers = new Headers(request.init.headers);
  expect(headers.get('X-Pulse-Org-ID')).toBe(expectedOrg);
  expect(headers.get('X-API-Token')).toBe(token);
  expect(headers.get('X-Requested-With')).toBe('XMLHttpRequest');
  expect(headers.get('Content-Type')).toBe('application/json');
  if (method === 'POST') expect(headers.get('X-CSRF-Token')).toBe(csrf);
  else expect(request.init.body).toBeUndefined();
};
const expectAuthRetained = (expectedOrg = orgID) => {
  expect(getOrgID()).toBe(expectedOrg);
  expect(getApiToken()).toBe(token);
  expect(window.localStorage.getItem('just_logged_out')).toBeNull();
};

beforeEach(async () => {
  vi.clearAllMocks();
  vi.stubEnv('MODE', 'production');
  sessionStorage.clear();
  localStorage.clear();
  clearAuth();
  setOrgID(orgID);
  setApiToken(token);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: true,
      hideUpgrade: true,
    },
  });
  inventory = [firstURL, selectedURL];
  capabilities = ['audit_logging'];
  readReply = () => json({ urls: inventory });
  writeReply = acceptWrite;
  healthReply = () => {
    primeCSRF();
    return json({});
  };
  requests.length = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (request: string | URL | Request, init: RequestInit = {}) => {
      const url = String(request);
      requests.push({ url, init });
      if (url === '/api/license/runtime-capabilities')
        return json({ capabilities, limits: [], max_history_days: 7 });
      if (url === '/api/health' && init.method === 'GET') return healthReply();
      if (url === endpoint && !init.method) return readReply();
      if (url === endpoint && init.method === 'POST') return writeReply(init);
      throw new Error(`Unexpected synthetic request: ${init.method ?? 'GET'} ${url}`);
    }),
  );
  await loadRuntimeCapabilities(true);
});

afterEach(() => {
  cleanup();
  clearAuth();
  syncSessionPresentationPolicy(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('Audit Webhook rendered controls through the real HTTP client', () => {
  it('adds to the complete saved list only after cold CSRF preparation, without erasing a newer draft', async () => {
    const health = deferred<Response>();
    const save = deferred<Response>();
    healthReply = () => health.promise;
    writeReply = () => save.promise;
    mount();
    await loaded();
    expectWire(reads()[0], 'GET');
    add(`  ${addedURL}  `);
    await waitFor(() => expect(healthReads()).toHaveLength(1));
    expect(healthReads()[0].init).toEqual({ method: 'GET', credentials: 'include' });
    expect(writes()).toHaveLength(0);
    expect(screen.getByTitle(firstURL)).toBeInTheDocument();
    expect(screen.getByTitle(selectedURL)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
    for (const button of screen.getAllByRole('button', { name: 'Remove webhook endpoint' }))
      expect(button).toBeDisabled();
    fireEvent.input(input(), { target: { value: otherURL } });
    fireEvent.keyDown(input(), { key: 'Enter' });
    primeCSRF();
    health.resolve(json({}));
    await waitFor(() => expect(writes()).toHaveLength(1));
    expectWire(writes()[0], 'POST');
    expect(JSON.parse(String(writes()[0].init.body))).toEqual({
      urls: [firstURL, selectedURL, addedURL],
    });
    expect(screen.queryByTitle(addedURL)).not.toBeInTheDocument();
    save.resolve(new Response(null, { status: 204 }));
    await screen.findByTitle(addedURL);
    expect(input()).toHaveValue(otherURL);
    expect(screen.getByText(success)).toBeInTheDocument();
    expect(reads()).toHaveLength(1);
    expect(writes()).toHaveLength(1);
    expectAuthRetained();
  });

  it('cancels without preparation and then removes only the chosen non-first namesake endpoint', async () => {
    primeCSRF();
    mount();
    await loaded();
    fireEvent.input(input(), { target: { value: addedURL } });
    const dialog = openRemove();
    expect(within(dialog).getByText(selectedURL)).toBeInTheDocument();
    expect(within(dialog).queryByText(firstURL)).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(writes()).toHaveLength(0);
    expect(healthReads()).toHaveLength(0);
    expect(input()).toHaveValue(addedURL);
    openRemove();
    confirmRemove();
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expectWire(writes()[0], 'POST');
    expect(JSON.parse(String(writes()[0].init.body))).toEqual({ urls: [firstURL] });
    expect(screen.getByTitle(firstURL)).toBeInTheDocument();
    expect(screen.queryByTitle(selectedURL)).not.toBeInTheDocument();
    expect(input()).toHaveValue(addedURL);
    expect(writes()).toHaveLength(1);
    expect(reads()).toHaveLength(1);
  });

  const refusals: { status: number; error: string; headers: Record<string, string> }[] = [
    { status: 400, error: 'invalid_org', headers: {} },
    { status: 403, error: 'not_org_member', headers: { 'X-CSRF-Token': 'synthetic-refreshed' } },
    { status: 429, error: 'rate_limited', headers: { 'Retry-After': '0.01' } },
  ];

  it.each(refusals)(
    'keeps a refused $status removal and draft without replay, retargeting or credential loss',
    async ({ status, error, headers }) => {
      primeCSRF();
      writeReply = () => json({ error, detail: 'synthetic-private-save-detail' }, status, headers);
      mount();
      await loaded();
      fireEvent.input(input(), { target: { value: addedURL } });
      openRemove();
      confirmRemove();
      const dialog = await screen.findByRole('dialog', { name: 'Remove audit webhook' });
      expect(await within(dialog).findByRole('alert')).toHaveTextContent(saveError);
      expect(document.body).not.toHaveTextContent('synthetic-private-save-detail');
      expect(screen.queryByText(success)).not.toBeInTheDocument();
      expect(screen.getByTitle(firstURL)).toBeInTheDocument();
      expect(screen.getByTitle(selectedURL)).toBeInTheDocument();
      expect(input()).toHaveValue(addedURL);
      expect(writes()).toHaveLength(1);
      expectWire(writes()[0], 'POST');
      expect(JSON.parse(String(writes()[0].init.body))).toEqual({ urls: [firstURL] });
      expect(reads()).toHaveLength(1);
      expect(healthReads()).toHaveLength(0);
      expectAuthRetained();

      writeReply = acceptWrite;
      confirmRemove();
      await screen.findByText(success);
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByTitle(selectedURL)).not.toBeInTheDocument();
      expect(screen.getByTitle(firstURL)).toBeInTheDocument();
      expect(input()).toHaveValue(addedURL);
      expect(writes()).toHaveLength(2);
      expectWire(writes()[1], 'POST');
      expect(writes()[1].init.body).toBe(writes()[0].init.body);
      expect(reads()).toHaveLength(1);
    },
  );

  it.each(refusals)(
    'keeps a $status read unknown until explicit recovery, without a default-org or automatic retry',
    async ({ status, error, headers }) => {
      primeCSRF();
      readReply = () => json({ error, detail: 'synthetic-private-read-detail' }, status, headers);
      mount();
      await screen.findByText(readError);
      expect(screen.getByText(/Existing destinations are unknown/)).toBeInTheDocument();
      expect(screen.queryByText(empty)).not.toBeInTheDocument();
      expect(document.body).not.toHaveTextContent('synthetic-private-read-detail');
      add();
      expect(input()).toHaveValue(addedURL);
      expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
      expect(reads()).toHaveLength(1);
      expectWire(reads()[0], 'GET');
      expect(writes()).toHaveLength(0);
      expectAuthRetained();

      readReply = () => json({ urls: inventory });
      fireEvent.click(screen.getByRole('button', { name: 'Retry loading' }));
      await loaded();
      expect(input()).toHaveValue(addedURL);
      expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeEnabled();
      expect(reads()).toHaveLength(2);
      expectWire(reads()[1], 'GET');
      expect(writes()).toHaveLength(0);
      expect(screen.queryByText(success)).not.toBeInTheDocument();
    },
  );

  it.each([
    'organization',
    'silent organization',
    'principal',
    'write',
    'read',
    'feature',
    'unmount',
  ])(
    'never dispatches a confirmed full-list save after %s changes during cold CSRF preparation',
    async (change) => {
      const health = deferred<Response>();
      healthReply = () => health.promise;
      const view = mount();
      await loaded();
      openRemove();
      confirmRemove();
      await waitFor(() => expect(healthReads()).toHaveLength(1));
      expect(writes()).toHaveLength(0);
      readReply = () => json({ urls: [otherURL] });
      if (change.includes('organization')) {
        setOrgID('estate/west');
        if (change === 'organization') eventBus.emit('org_switched', 'estate/west');
      }
      if (change === 'principal') view.setReader({ ...view.reader(), currentUser: 'other-admin' });
      if (change === 'write') view.setReader({ ...view.reader(), canManage: false });
      if (change === 'read') view.setReader({ ...view.reader(), canRead: false });
      if (change === 'feature') {
        capabilities = [];
        await loadRuntimeCapabilities(true);
      }
      if (change === 'unmount') view.unmount();

      // Drain the real client's 10ms CSRF await and any erroneous retry timer.
      // Changing context must prevent dispatch, not merely ignore its result.
      vi.useFakeTimers();
      primeCSRF();
      health.resolve(json({}));
      await vi.advanceTimersByTimeAsync(120_000);
      vi.useRealTimers();
      expect(writes()).toHaveLength(0);
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByText(success)).not.toBeInTheDocument();
      expect(screen.queryByTitle(firstURL)).not.toBeInTheDocument();
      expect(screen.queryByTitle(selectedURL)).not.toBeInTheDocument();
      if (change === 'silent organization') {
        await screen.findByText(readError);
        expect(reads()).toHaveLength(1);
        expect(input()).toHaveValue('');
        expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
      }
      expectAuthRetained(change.includes('organization') ? 'estate/west' : orgID);
    },
  );

  it('does not admit an old parsed read body after the same principal returns', async () => {
    const old = pendingBody();
    const current = pendingBody();
    readReply = () => old.response;
    const view = mount();
    await waitFor(() => expect(old.response.bodyUsed).toBe(true));
    const signal = reads()[0].init.signal;
    readReply = () => json({ urls: [otherURL] });
    view.setReader({ ...view.reader(), currentUser: 'other-admin' });
    await loaded([otherURL]);
    readReply = () => current.response;
    view.setReader({ ...view.reader(), currentUser: 'same-admin' });
    await waitFor(() => expect(current.response.bodyUsed).toBe(true));
    expect(signal?.aborted).toBe(true);
    old.finish({ urls: [firstURL, selectedURL] });
    // Let native body parsing and the obsolete completion reach the panel.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByTitle(firstURL)).not.toBeInTheDocument();
    expect(screen.queryByTitle(otherURL)).not.toBeInTheDocument();
    expect(screen.getByText('Loading audit webhooks…')).toBeInTheDocument();
    expect(screen.queryByText(empty)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
    current.finish({ urls: [otherURL] });
    await loaded([otherURL]);
    expect(reads()).toHaveLength(3);
    expect(writes()).toHaveLength(0);
  });

  it('does not let an old parsed save body overwrite a returning principal or erase their matching draft', async () => {
    primeCSRF();
    const old = pendingBody();
    writeReply = () => old.response;
    const view = mount();
    await loaded();
    add();
    await waitFor(() => expect(old.response.bodyUsed).toBe(true));
    const signal = writes()[0].init.signal;
    readReply = () => json({ urls: [otherURL] });
    view.setReader({ ...view.reader(), currentUser: 'other-admin' });
    await loaded([otherURL]);
    view.setReader({ ...view.reader(), currentUser: 'same-admin' });
    await loaded([otherURL]);
    fireEvent.input(input(), { target: { value: addedURL } });
    old.finish({});
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(signal?.aborted).toBe(true);
    expect(screen.getByTitle(otherURL)).toBeInTheDocument();
    expect(screen.queryByTitle(firstURL)).not.toBeInTheDocument();
    expect(screen.queryByTitle(addedURL)).not.toBeInTheDocument();
    expect(screen.queryByText(success)).not.toBeInTheDocument();
    expect(input()).toHaveValue(addedURL);
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeEnabled();
    expect(writes()).toHaveLength(1);
    expect(reads()).toHaveLength(3);
    expectWire(writes()[0], 'POST');
  });

  it('recovers a withheld reader as read-only and requires a fresh read before write access returns', async () => {
    const view = mount({ canRead: false, canManage: false });
    expect(
      screen.getByText('Audit webhook access is unavailable for this account.'),
    ).toBeInTheDocument();
    expect(reads()).toHaveLength(0);
    view.setReader({ ...view.reader(), canRead: true });
    await loaded();
    expect(input()).toBeDisabled();
    fireEvent.keyDown(input(), { key: 'Enter' });
    for (const button of screen.getAllByRole('button', { name: 'Remove webhook endpoint' }))
      expect(button).toBeDisabled();
    expect(writes()).toHaveLength(0);

    const current = deferred<Response>();
    readReply = () => current.promise;
    view.setReader({ ...view.reader(), canManage: true });
    await waitFor(() => expect(reads()).toHaveLength(2));
    expect(screen.queryByTitle(selectedURL)).not.toBeInTheDocument();
    add();
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
    expect(writes()).toHaveLength(0);
    current.resolve(json({ urls: [firstURL, selectedURL] }));
    await loaded();
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeEnabled();
    expect(writes()).toHaveLength(0);
    expect(healthReads()).toHaveLength(0);
    for (const request of reads()) expectWire(request, 'GET');
  });

  it('preserves a pending confirmation and draft across an equal real capability refresh', async () => {
    primeCSRF();
    const save = deferred<Response>();
    writeReply = () => save.promise;
    mount();
    await loaded();
    fireEvent.input(input(), { target: { value: addedURL } });
    openRemove();
    confirmRemove();
    await waitFor(() => expect(writes()).toHaveLength(1));
    await loadRuntimeCapabilities(true);
    expect(writes()[0].init.signal?.aborted).toBe(false);
    expect(screen.getByRole('dialog', { name: 'Remove audit webhook' })).toBeInTheDocument();
    expect(input()).toHaveValue(addedURL);
    expect(reads()).toHaveLength(1);
    save.resolve(new Response(null, { status: 204 }));
    await screen.findByText(success);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByTitle(firstURL)).toBeInTheDocument();
    expect(screen.queryByTitle(selectedURL)).not.toBeInTheDocument();
    expect(input()).toHaveValue(addedURL);
    expect(writes()).toHaveLength(1);
  });
});
