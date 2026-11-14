import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal, Show } from 'solid-js';
import { SecurityAuthPanel } from '../SecurityAuthPanel';
import { useSettingsAccess } from '../useSettingsAccess';
import { canMountSettingsPanel } from '../settingsNavVisibility';
import type { SettingsTab } from '../settingsNavigationModel';
import type { SecurityStatus } from '@/types/config';
import { AlertsAPI } from '@/api/alerts';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { showError, showSuccess } from '@/utils/toast';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';

vi.mock('@/stores/license', () => ({
  hasFeature: () => true,
  runtimeCapabilitiesLoaded: () => true,
  isHostedModeEnabled: () => false,
  isRuntimeCapabilityBlocked: () => false,
}));
vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/toast', () => ({ showError: vi.fn(), showSuccess: vi.fn() }));
const fetchMock = vi.fn();
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const savedStatus = (): SecurityStatus => ({
  hasAuthentication: true,
  requiresAuth: true,
  apiTokenConfigured: true,
  authUsername: 'local-admin',
  currentUsername: 'reader-admin',
  apiTokenHint: 'fixture…hint',
  tokenScopes: ['settings:read', 'settings:write'],
  settingsCapabilities: {
    authenticationRead: true,
    authenticationWrite: true,
  } as SecurityStatus['settingsCapabilities'],
});
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const writes = () =>
  fetchMock.mock.calls.filter(
    ([url, options]) => url === '/api/security/quick-setup' && options.method === 'POST',
  );
let nextStatus: () => Response | Promise<Response>;
let nextWrite: () => Response | Promise<Response>;
let access!: ReturnType<typeof useSettingsAccess>;
let dispose: () => void;
let clipboard: ReturnType<typeof vi.fn>;
let blobs: Blob[];

async function mount(initial = false) {
  if (initial)
    nextStatus = () =>
      json({ ...savedStatus(), hasAuthentication: false, apiTokenConfigured: false });
  const view = render(() => {
    const [activeTab, setActiveTab] = createSignal<SettingsTab>('security-auth');
    const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
    const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(false);
    const [showPasswordModal, setShowPasswordModal] = createSignal(false);
    access = useSettingsAccess({ activeTab, setActiveTab, searchQuery: () => '' });
    return (
      <Show
        when={canMountSettingsPanel('security-auth', access.securityStatus()?.settingsCapabilities)}
      >
        <SecurityAuthPanel
          securityStatus={access.securityStatus}
          securityStatusLoading={access.securityStatusLoading}
          canManage={
            access.securityStatus()?.settingsCapabilities?.authenticationWrite === true &&
            access.securityStatus()?.presentationPolicy?.readOnly !== true &&
            access.securityStatus()?.presentationPolicy?.demoMode !== true
          }
          versionInfo={() => null}
          showQuickSecuritySetup={showQuickSecuritySetup}
          setShowQuickSecuritySetup={setShowQuickSecuritySetup}
          showQuickSecurityWizard={showQuickSecurityWizard}
          setShowQuickSecurityWizard={setShowQuickSecurityWizard}
          showPasswordModal={showPasswordModal}
          setShowPasswordModal={setShowPasswordModal}
          hideLocalLogin={() => false}
          hideLocalLoginLocked={() => false}
          savingHideLocalLogin={() => false}
          handleHideLocalLoginChange={async () => {}}
          loadSecurityStatus={access.loadSecurityStatus}
        />
      </Show>
    );
  });
  dispose = view.unmount;
  await waitFor(() => expect(access.securityStatusLoading()).toBe(false));
  fireEvent.click(screen.getByRole('button', { name: initial ? 'Setup' : 'Rotate credentials' }));
}
const fillCustom = (password = 'Synthetic-only-pass1!', confirmation = password) => {
  fireEvent.click(screen.getByRole('button', { name: 'Custom' }));
  for (const [name, value] of [
    ['Username', 'synthetic-local'],
    ['Password (min 12 characters)', password],
    ['Confirm password', confirmation],
  ]) {
    fireEvent.input(screen.getByLabelText(name, { exact: true }), { target: { value } });
  }
};
const submit = (initial = false) => {
  const buttons = screen.getAllByRole('button', {
    name: initial ? 'Enable Security Now' : 'Rotate credentials',
  });
  fireEvent.click(buttons[buttons.length - 1]);
};
const generated = () =>
  JSON.parse(writes()[0][1].body) as {
    username: string;
    password: string;
    apiToken: string;
    force: boolean;
  };
const download = () =>
  fireEvent.click(screen.getByRole('button', { name: 'Download credentials' }));
const blobText = (blob: Blob) =>
  new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = reject;
    reader.readAsText(blob);
  });
const reveal = () => screen.findByRole('button', { name: 'Download credentials' });

beforeEach(() => {
  vi.clearAllMocks();
  clearAuth();
  sessionStorage.clear();
  setApiToken('synthetic-original-token');
  setOrgID('fixture-estate');
  document.cookie = 'pulse_csrf=fixture-csrf; Path=/';
  syncSessionPresentationPolicy(null);
  nextStatus = () => json(savedStatus());
  nextWrite = () => json({ success: true });
  fetchMock.mockReset().mockImplementation((url, options) => {
    if (url === '/api/security/status') return nextStatus();
    if (url === '/api/security/quick-setup' && options.method === 'POST') return nextWrite();
    throw new Error('Unexpected fixture request');
  });
  vi.stubGlobal('fetch', fetchMock);
  clipboard = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: clipboard },
  });
  blobs = [];
  vi.stubGlobal(
    'URL',
    Object.assign(URL, {
      createObjectURL: vi.fn((blob: Blob) => {
        blobs.push(blob);
        return 'blob:synthetic-only';
      }),
      revokeObjectURL: vi.fn(),
    }),
  );
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  clearAuth();
});

describe('native Settings credential setup and rotation through real API transport', () => {
  it('keeps confirmed once-only credentials available after an old unbound alert401', async () => {
    await mount();
    const held = deferred<Response>();
    fetchMock.mockImplementationOnce(() => held.promise);
    const oldAlert = AlertsAPI.getActive().catch((error: unknown) => error);
    submit();
    await reveal();
    const issued = generated();
    expect(getApiToken()).toBeNull();
    held.resolve(json({ error: 'synthetic-old-token-refused' }, 401));
    expect(await oldAlert).toMatchObject({ status: 401 });
    expect(getOrgID()).toBe('fixture-estate');
    expect(localStorage.getItem('just_logged_out')).toBeNull();
    expect(screen.getByText('Credentials are active now')).toBeVisible();
    expect(document.body).toHaveTextContent(issued.password);
    expect(document.body).toHaveTextContent(issued.apiToken);
    download();
    expect(await blobText(blobs[0])).toContain(issued.apiToken);
    expect(writes()).toHaveLength(1);
  });

  it.each([false, true])(
    'submits one admitted %s initial/rotation write and preserves org while dropping its obsolete token',
    async (initial) => {
      await mount(initial);
      const pending = deferred<Response>();
      nextWrite = () => pending.promise;
      const button = screen
        .getAllByRole('button', { name: initial ? 'Enable Security Now' : 'Rotate credentials' })
        .at(-1)!;
      fireEvent.click(button);
      fireEvent.click(button);
      expect(writes()).toHaveLength(1);
      expect(generated()).toMatchObject({
        force: !initial,
        username: initial ? 'admin' : 'local-admin',
      });
      expect(generated().password).toHaveLength(16);
      expect(generated().apiToken).toMatch(/^[a-f0-9]{48}$/);
      expect(writes()[0][1].headers).toMatchObject({
        'X-Pulse-Org-ID': 'fixture-estate',
        'X-API-Token': 'synthetic-original-token',
        'X-CSRF-Token': 'fixture-csrf',
      });
      expect(writes()[0][1].signal).toBeInstanceOf(AbortSignal);
      expect(screen.getByRole('button', { name: 'Auto-Generate' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Custom' })).toBeDisabled();
      expect(
        screen.getByRole('button', { name: initial ? 'Setup' : 'Change password' }),
      ).toBeDisabled();
      pending.resolve(json({ success: true }));
      await reveal();
      expect(showSuccess).toHaveBeenCalledOnce();
      expect(getApiToken()).toBeNull();
      expect(getOrgID()).toBe('fixture-estate');
      expect(screen.getByText('Credentials are active now')).toBeVisible();
      expect(screen.getByText(/No restart is needed/)).toBeVisible();
      expect(document.body).toHaveTextContent(generated().password);
      download();
      expect(blobs).toHaveLength(1);
      const content = await blobText(blobs[0]);
      expect(content).toContain('Status: Confirmed active');
      expect(content).toContain(generated().apiToken);
    },
  );

  it('explains immediate all-token replacement before any write and freezes a custom draft in flight', async () => {
    await mount();
    fillCustom();
    expect(screen.getByText('Replace all API tokens (full access)')).toBeVisible();
    expect(screen.getByText(/Active immediately/)).toBeVisible();
    const pending = deferred<Response>();
    nextWrite = () => pending.promise;
    submit();
    expect(screen.getByLabelText('Username', { exact: true })).toBeDisabled();
    expect(screen.getByLabelText('Password (min 12 characters)', { exact: true })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Hide local login form' })).toBeDisabled();
    expect(generated().password).toBe('Synthetic-only-pass1!');
    pending.resolve(json({ success: true }));
    await reveal();
  });

  it.each([
    ['too-short', 'too-short', 'Password must be at least 12 characters'],
    ['Synthetic-only-pass1!', 'different', 'Passwords do not match'],
  ])('keeps invalid custom input without a write: %s', async (password, confirmation, message) => {
    await mount();
    fillCustom(password, confirmation);
    submit();
    expect(screen.getByRole('alert')).toHaveTextContent(message);
    expect(writes()).toHaveLength(0);
  });

  it('retains a same-reader draft through verification without permitting dispatch', async () => {
    await mount();
    fillCustom();
    const pending = deferred<Response>();
    nextStatus = () => pending.promise;
    const refresh = access.loadSecurityStatus();
    submit();
    expect(writes()).toHaveLength(0);
    expect(screen.getByLabelText('Username', { exact: true })).toBeDisabled();
    pending.resolve(json({ ...savedStatus(), tokenScopes: ['settings:write', 'settings:read'] }));
    await refresh;
    expect(screen.getByLabelText('Username', { exact: true })).toHaveValue('synthetic-local');
    expect(screen.getByLabelText('Username', { exact: true })).toBeEnabled();
    submit();
    await reveal();
    expect(writes()).toHaveLength(1);
  });

  it.each([400, 401, 403, 409, 429])(
    'keeps a definite %i refusal local and does not replay even a refreshed CSRF response',
    async (code) => {
      await mount();
      fillCustom();
      nextWrite = () =>
        new Response(JSON.stringify({ error: 'Synthetic credential refusal' }), {
          status: code,
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': 'replacement-csrf' },
        });
      submit();
      await screen.findByRole('alert');
      expect(screen.getByRole('alert')).toHaveTextContent('Synthetic credential refusal');
      expect(screen.getByLabelText('Username', { exact: true })).toHaveValue('synthetic-local');
      expect(screen.getByLabelText('Username', { exact: true })).toBeEnabled();
      expect(screen.queryByRole('button', { name: 'Download credentials' })).toBeNull();
      expect(getApiToken()).toBe('synthetic-original-token');
      expect(showSuccess).not.toHaveBeenCalled();
      expect(writes()).toHaveLength(1);
    },
  );

  it.each(['server', 'network', 'malformed', 'false-success', 'empty'])(
    'retains attempted auto-generated credentials without a success claim or blind retry after %s uncertainty',
    async (kind) => {
      await mount();
      nextWrite = () => {
        if (kind === 'network') return Promise.reject(new Error('PRIVATE_RESPONSE_CANARY'));
        if (kind === 'server') return json({ error: 'PRIVATE_RESPONSE_CANARY' }, 500);
        if (kind === 'malformed') return new Response('{PRIVATE_RESPONSE_CANARY', { status: 200 });
        if (kind === 'empty') return new Response('', { status: 200 });
        return json({ success: false });
      };
      const staleSubmit = screen.getAllByRole('button', { name: 'Rotate credentials' }).at(-1)!;
      fireEvent.click(staleSubmit);
      await reveal();
      expect(screen.getByText('Credential change unconfirmed')).toBeVisible();
      expect(screen.getByText(/The password or token may have changed/)).toBeVisible();
      expect(document.body).toHaveTextContent(generated().password);
      expect(document.body).not.toHaveTextContent('PRIVATE_RESPONSE_CANARY');
      expect(screen.queryByText('Credentials are active now')).toBeNull();
      expect(showSuccess).not.toHaveBeenCalled();
      expect(getApiToken()).toBe('synthetic-original-token');
      fireEvent.click(staleSubmit);
      expect(writes()).toHaveLength(1);
      download();
      const content = await blobText(blobs[0]);
      expect(content).toContain('Status: Unconfirmed');
      expect(content).toContain(generated().password);
      fireEvent.click(screen.getAllByRole('button', { name: 'Copy' })[1]);
      await waitFor(() => expect(clipboard).toHaveBeenCalledWith(generated().password));
    },
  );

  it.each(['confirmed', 'unconfirmed'])(
    'keeps every saved %s field and its own copy action in order through status refresh',
    async (outcome) => {
      await mount();
      nextWrite = () =>
        json(outcome === 'confirmed' ? { success: true } : {}, outcome === 'confirmed' ? 200 : 500);
      submit();
      await reveal();
      const values = generated();
      const copyButtons: HTMLButtonElement[] = [];
      for (const [index, [label, value]] of [
        ['Username', values.username],
        ['Password', values.password],
        ['API token', values.apiToken],
      ].entries()) {
        const row = screen.getByText(label, { exact: true }).parentElement!;
        const code = within(row).getByText(value, { selector: 'code', exact: true });
        expect(code.classList.contains('break-all')).toBe(label !== 'Username');
        const button = within(row).getByRole<HTMLButtonElement>('button', {
          name: 'Copy',
        });
        copyButtons.push(button);
        fireEvent.click(button);
        await waitFor(() => expect(clipboard).toHaveBeenNthCalledWith(index + 1, value));
        expect(await within(row).findByRole('button', { name: 'Copied!' })).toBeVisible();
        expect(screen.getAllByRole('button', { name: 'Copied!' })).toHaveLength(1);
      }
      const tokenRow = screen.getByText('API token', { exact: true }).parentElement!;
      expect(
        within(tokenRow).getByText('Use this token with the X-API-Token header for automation.'),
      ).toBeVisible();
      expect(
        within(tokenRow).getByText('This token is only shown once. Save it now.'),
      ).toBeVisible();
      const pending = deferred<Response>();
      nextStatus = () => pending.promise;
      const refresh = access.loadSecurityStatus();
      for (const button of copyButtons) {
        expect(button).toBeDisabled();
        fireEvent.click(button);
      }
      expect(clipboard).toHaveBeenCalledTimes(3);
      expect(screen.getByRole('button', { name: 'Download credentials' })).toBeDisabled();
      pending.resolve(json(savedStatus()));
      await refresh;
      for (const button of copyButtons) expect(button).toBeEnabled();
      expect(document.body).toHaveTextContent(values.password);
      expect(document.body).toHaveTextContent(values.apiToken);
      expect(writes()).toHaveLength(1);
    },
  );

  it('refreshes a skipped setup without revealing invented credentials or clearing auth', async () => {
    await mount(true);
    nextWrite = () => json({ success: true, skipped: true, message: 'PRIVATE_RESPONSE_CANARY' });
    submit(true);
    await waitFor(() =>
      expect(showError).toHaveBeenCalledWith(
        'Security is already configured. No credentials were changed.',
      ),
    );
    expect(screen.queryByRole('button', { name: 'Download credentials' })).toBeNull();
    expect(getApiToken()).toBe('synthetic-original-token');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(writes()).toHaveLength(1);
  });

  it.each([
    'org',
    'same-org-event',
    'reader',
    'local account',
    'credential hint',
    'read-only',
    'demo',
    'read',
    'scope',
    'write',
    'failed read',
    'disposal',
    'credential',
  ])(
    'ignores a late accepted result after %s withdrawal and never clears the later credential',
    async (kind) => {
      await mount();
      fillCustom();
      const pending = deferred<Response>();
      nextWrite = () => pending.promise;
      submit();
      const signal = writes()[0][1].signal as AbortSignal;
      if (kind === 'org' || kind === 'same-org-event') {
        if (kind === 'org') setOrgID('other-estate');
        eventBus.emit('org_switched', kind === 'org' ? 'other-estate' : 'fixture-estate');
      } else if (kind === 'disposal') dispose();
      else if (kind !== 'credential') {
        const change = savedStatus();
        if (kind === 'reader') change.currentUsername = 'another-reader';
        if (kind === 'local account') change.authUsername = 'another-local';
        if (kind === 'scope') change.tokenScopes = ['settings:read'];
        if (kind === 'credential hint') change.apiTokenHint = 'another…hint';
        if (kind === 'read') change.settingsCapabilities!.authenticationRead = false;
        if (kind === 'read-only' || kind === 'demo')
          change.presentationPolicy = {
            readOnly: kind === 'read-only',
            demoMode: kind === 'demo',
            hideCommercial: false,
            hideUpgrade: false,
          };
        if (kind === 'write') change.settingsCapabilities!.authenticationWrite = false;
        nextStatus = () => (kind === 'failed read' ? json({}, 403) : json(change));
        await access.loadSecurityStatus();
      }
      if (kind !== 'credential') expect(signal.aborted).toBe(true);
      setApiToken('synthetic-later-token');
      pending.resolve(json({ success: true }));
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(getApiToken()).toBe('synthetic-later-token');
      expect(showSuccess).not.toHaveBeenCalled();
      expect(screen.queryByRole('button', { name: 'Download credentials' })).toBeNull();
      expect(writes()).toHaveLength(1);
    },
  );

  it.each([401, 500])('ignores late %i failure feedback after an org cycle', async (code) => {
    await mount();
    const pending = deferred<Response>();
    nextWrite = () => pending.promise;
    submit();
    eventBus.emit('org_switched', 'fixture-estate');
    setApiToken('synthetic-later-token');
    pending.resolve(json({ error: 'PRIVATE_RESPONSE_CANARY' }, code));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(getApiToken()).toBe('synthetic-later-token');
    expect(showError).not.toHaveBeenCalled();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('cancels dispatch when its form closes during asynchronous CSRF preparation', async () => {
    await mount();
    document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
    submit();
    expect(screen.getByText('Rotating credentials...')).toBeVisible();
    // Test mode skips the health GET, but the awaited preparation still yields.
    // The native browser proof separately holds the real production-mode GET.
    eventBus.emit('org_switched', 'fixture-estate');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(writes()).toHaveLength(0);
  });

  it('does not reveal a success body completed after withdrawal', async () => {
    await mount();
    const body = deferred<unknown>();
    const response = json({});
    vi.spyOn(response, 'json').mockReturnValue(body.promise);
    nextWrite = () => response;
    submit();
    await waitFor(() => expect(response.json).toHaveBeenCalledOnce());
    eventBus.emit('org_switched', 'fixture-estate');
    setApiToken('synthetic-later-token');
    body.resolve({ success: true });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(showSuccess).not.toHaveBeenCalled();
    expect(getApiToken()).toBe('synthetic-later-token');
    expect(screen.queryByRole('button', { name: 'Download credentials' })).toBeNull();
  });

  it('keeps setup and rotation mutually exclusive for an API-only installation', async () => {
    nextStatus = () => json({ ...savedStatus(), hasAuthentication: false });
    await mount();
    fillCustom();
    fireEvent.click(screen.getByRole('button', { name: 'Setup' }));
    expect(screen.queryByLabelText('Username', { exact: true })).toBeNull();
    expect(screen.queryByRole('heading', { name: 'Generate new admin credentials' })).toBeNull();
    expect(screen.getByRole('heading', { name: 'Quick security setup' })).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Rotate credentials' }));
    expect(screen.queryByRole('heading', { name: 'Quick security setup' })).toBeNull();
    expect(screen.getByRole('heading', { name: 'Generate new admin credentials' })).toBeVisible();
    expect(writes()).toHaveLength(0);
  });

  it('withdraws a reveal and ignores obsolete copy, download and dismissal controls', async () => {
    await mount();
    submit();
    await reveal();
    const copy = deferred<void>();
    clipboard.mockReturnValue(copy.promise);
    const downloadButton = screen.getByRole('button', { name: 'Download credentials' });
    const done = screen.getByRole('button', { name: "Done - I've Saved My Credentials" });
    fireEvent.click(screen.getAllByRole('button', { name: 'Copy' })[1]);
    eventBus.emit('org_switched', 'fixture-estate');
    setApiToken('synthetic-later-token');
    copy.resolve();
    fireEvent.click(downloadButton);
    fireEvent.click(done);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(blobs).toHaveLength(0);
    expect(showError).not.toHaveBeenCalled();
    expect(getApiToken()).toBe('synthetic-later-token');
    expect(screen.queryByText('Copied!')).toBeNull();
  });
});

describe('pending credential changes versus real background authentication failures', () => {
  const redirect = vi.fn();
  const locationDescriptor = Object.getOwnPropertyDescriptor(window, 'location');
  beforeEach(() => {
    localStorage.clear();
    redirect.mockReset();
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        get href() {
          return 'http://localhost/settings/security';
        },
        set href(value: string) {
          redirect(value);
        },
      },
    });
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    if (locationDescriptor) Object.defineProperty(window, 'location', locationDescriptor);
  });
  const rejectBackgroundRead = async () => {
    fetchMock.mockImplementationOnce(() =>
      json({ error: 'Synthetic expired authentication' }, 401),
    );
    expect(await AlertsAPI.getActive().catch((error: unknown) => error)).toMatchObject({
      status: 401,
    });
  };

  it.each(['headers', 'body'])(
    'retains once-only credentials when a background401 arrives before rotation %s',
    async (phase) => {
      await mount();
      const heldHeaders = deferred<Response>();
      const heldBody = deferred<unknown>();
      const response = json({ success: true });
      if (phase === 'body') response.json = () => heldBody.promise;
      nextWrite = () => (phase === 'headers' ? heldHeaders.promise : response);
      submit();
      await rejectBackgroundRead();
      expect(getOrgID()).toBe('fixture-estate');
      expect(getApiToken()).toBe('synthetic-original-token');
      expect(redirect).not.toHaveBeenCalled();
      expect(screen.getByText('Sign-in rejected:')).toBeVisible();
      expect(screen.getByRole('button', { name: 'Rotating credentials...' })).toBeDisabled();
      if (phase === 'headers') heldHeaders.resolve(response);
      else heldBody.resolve({ success: true });
      await reveal();
      expect(getApiToken()).toBeNull();
      expect(getOrgID()).toBe('fixture-estate');
      expect(redirect).not.toHaveBeenCalled();
      expect(screen.getByText('Credentials are active now')).toBeVisible();
      download();
      expect(await blobText(blobs[0])).toContain(generated().apiToken);
      expect(writes()).toHaveLength(1);
      await rejectBackgroundRead();
      expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
    },
  );

  it.each(['server', 'network', 'malformed'])(
    'allows attempted credentials to be saved after an unconfirmed %s and does not hide401 on close',
    async (failure) => {
      await mount();
      const held = deferred<Response>();
      nextWrite = () => held.promise;
      submit();
      await rejectBackgroundRead();
      if (failure === 'network') held.reject(new Error('Synthetic lost response'));
      else if (failure === 'server')
        held.resolve(json({ error: 'Synthetic incomplete change' }, 500));
      else held.resolve(new Response('not-json'));
      await reveal();
      expect(screen.getByText('Credential change unconfirmed')).toBeVisible();
      expect(screen.getByText(/Background sign-in was rejected/)).toBeVisible();
      expect(redirect).not.toHaveBeenCalled();
      expect(getApiToken()).toBe('synthetic-original-token');
      download();
      expect(await blobText(blobs[0])).toContain('Status: Unconfirmed');
      expect(await blobText(blobs[0])).toContain(generated().apiToken);
      fireEvent.click(screen.getByRole('button', { name: "Close - I've Saved These Credentials" }));
      expect(getApiToken()).toBeNull();
      expect(getOrgID()).toBeNull();
      expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
      expect(writes()).toHaveLength(1);
    },
  );

  it('does not keep a background401 deferred after a definite refusal', async () => {
    await mount();
    const held = deferred<Response>();
    nextWrite = () => held.promise;
    submit();
    await rejectBackgroundRead();
    held.resolve(json({ error: 'Synthetic change refused' }, 403));
    await waitFor(() => expect(redirect).toHaveBeenCalledExactlyOnceWith('/'));
    expect(screen.queryByRole('button', { name: 'Download credentials' })).toBeNull();
    expect(writes()).toHaveLength(1);
  });

  it('also retains a rejection first delivered after an uncertain result until save/close', async () => {
    await mount();
    nextWrite = () => json({ error: 'Synthetic incomplete change' }, 500);
    submit();
    await reveal();
    await rejectBackgroundRead();
    expect(screen.getByText(/Background sign-in was rejected/)).toBeVisible();
    expect(redirect).not.toHaveBeenCalled();
    download();
    expect(await blobText(blobs[0])).toContain(generated().apiToken);
    fireEvent.click(screen.getByRole('button', { name: "Close - I've Saved These Credentials" }));
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
    expect(writes()).toHaveLength(1);
  });

  it.each(['headers', 'body'])(
    'bounds a never-completing %s to one unconfirmed attempt, not a new rotation',
    async (phase) => {
      await mount();
      vi.useFakeTimers();
      const never = new Promise<never>(() => {});
      const response = json({ success: true });
      if (phase === 'body') response.json = () => never;
      nextWrite = () => (phase === 'headers' ? never : response);
      submit();
      await rejectBackgroundRead();
      await vi.advanceTimersByTimeAsync(30_000);
      expect(screen.getByText('Credential change unconfirmed')).toBeVisible();
      expect(screen.getByRole('button', { name: 'Download credentials' })).toBeEnabled();
      expect(writes()[0][1].signal.aborted).toBe(true);
      expect(redirect).not.toHaveBeenCalled();
      expect(writes()).toHaveLength(1);
      vi.useRealTimers();
      download();
      expect(await blobText(blobs[0])).toContain(generated().apiToken);
    },
  );

  it('releases an unresolved rejection when its own form unmounts', async () => {
    await mount();
    nextWrite = () => new Promise<Response>(() => {});
    submit();
    await rejectBackgroundRead();
    dispose();
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
    expect(writes()[0][1].signal.aborted).toBe(true);
  });

  it('does not transfer an unresolved rejection or reveal to replacement credentials', async () => {
    await mount();
    const held = deferred<Response>();
    nextWrite = () => held.promise;
    submit();
    await rejectBackgroundRead();
    setApiToken('synthetic-later-token');
    held.resolve(json({ success: true }));
    await new Promise((resolve) => setTimeout(resolve, 0));
    dispose();
    expect(getApiToken()).toBe('synthetic-later-token');
    expect(getOrgID()).toBe('fixture-estate');
    expect(redirect).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: 'Download credentials' })).toBeNull();
  });
});
