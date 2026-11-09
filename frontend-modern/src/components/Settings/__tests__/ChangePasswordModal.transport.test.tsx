import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { SettingsDialogs } from '../SettingsDialogs';
import { useSettingsAccess } from '../useSettingsAccess';
import type { SettingsTab } from '../settingsNavigationModel';
import type { SecurityStatus } from '@/types/config';
import { clearAuth, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
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
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));
const fetchMock = vi.fn();
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const status = (): SecurityStatus => ({
  hasAuthentication: true,
  requiresAuth: true,
  authUsername: 'configured-local',
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
const writes = () => fetchMock.mock.calls.filter(([, options]) => options.method === 'POST');
let nextStatus: () => Response | Promise<Response>;
let nextWrite: () => Response | Promise<Response>;
let access!: ReturnType<typeof useSettingsAccess>;
let setOpen!: (value: boolean) => void;

async function mount() {
  render(() => {
    const [activeTab, setActiveTab] = createSignal<SettingsTab>('security-auth');
    const [open, show] = createSignal(false);
    setOpen = show;
    access = useSettingsAccess({ activeTab, setActiveTab, searchQuery: () => '' });
    return (
      <SettingsDialogs
        securityStatus={access.securityStatus}
        securityStatusLoading={access.securityStatusLoading}
        showPasswordModal={open}
        closePasswordModal={() => {
          show(false);
          void access.loadSecurityStatus();
        }}
        onPasswordChanged={() => show(false)}
        showUpdateConfirmation={() => false}
        closeUpdateConfirmation={() => {}}
        handleConfirmUpdate={() => {}}
        versionInfo={() => null}
        updateInfo={() => null}
        updatePlan={() => null}
        isInstallingUpdate={() => false}
        exportPassphrase={() => ''}
        setExportPassphrase={() => {}}
        exportPassphraseConfirmation={() => ''}
        setExportPassphraseConfirmation={() => {}}
        importPassphrase={() => ''}
        setImportPassphrase={() => {}}
        importFile={() => null}
        setImportFile={() => {}}
        showExportDialog={() => false}
        showImportDialog={() => false}
        showApiTokenModal={() => false}
        transferOrgID={() => 'fixture-estate'}
        isTransferring={() => false}
        transferError={() => ''}
        transferNotice={() => ''}
        apiTokenInput={() => ''}
        apiTokenError={() => ''}
        setApiTokenInput={() => {}}
        handleExport={() => {}}
        handleImport={() => {}}
        closeExportDialog={() => {}}
        closeImportDialog={() => {}}
        closeApiTokenModal={() => {}}
        handleApiTokenAuthenticate={() => {}}
      />
    );
  });
  await waitFor(() => expect(access.securityStatusLoading()).toBe(false));
  setOpen(true);
}
const input = (label: string, value: string) =>
  fireEvent.input(screen.getByLabelText(label), { target: { value } });
const fill = (current = 'fixture-current-only') => {
  input('Current Password', current);
  input('New Password', 'fixture-new-password-only');
  input('Confirm New Password', 'fixture-new-password-only');
};
const submit = () =>
  fireEvent.submit(screen.getByRole('dialog', { name: 'Change password' }).querySelector('form')!);
const expectBlank = () => {
  for (const name of ['Current Password', 'New Password', 'Confirm New Password']) {
    expect(screen.getByLabelText(name)).toHaveValue('');
  }
};

beforeEach(() => {
  vi.clearAllMocks();
  clearAuth();
  sessionStorage.clear();
  sessionStorage.setItem('pulse_auth_user', 'obsolete-other-reader');
  setOrgID('fixture-estate');
  document.cookie = 'pulse_csrf=fixture-csrf; Path=/';
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: false,
      hideUpgrade: false,
    },
  });
  nextStatus = () => json(status());
  nextWrite = () => json({});
  fetchMock.mockReset().mockImplementation((url, options) => {
    if (url === '/api/security/status' && options.method !== 'POST') return nextStatus();
    if (url === '/api/security/change-password' && options.method === 'POST') return nextWrite();
    throw new Error('Unexpected fixture request');
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  clearAuth();
});

describe('password dialog through the native settings status reader and dialog binding', () => {
  it('preserves the same reader draft, disables it during refresh and resumes after equal facts', async () => {
    await mount();
    fill();
    const read = deferred<Response>();
    nextStatus = () => read.promise;
    const refresh = access.loadSecurityStatus();
    expect(screen.getByLabelText('Current Password')).toBeDisabled();
    expect(screen.getByRole('status')).toHaveTextContent('Checking settings access');
    submit();
    expect(writes()).toHaveLength(0);
    read.resolve(json({ ...status(), tokenScopes: ['settings:write', 'settings:read'] }));
    await refresh;
    expect(screen.getByLabelText('Current Password')).toHaveValue('fixture-current-only');
    expect(screen.getByRole('button', { name: 'Change Password' })).toBeEnabled();
  });

  it.each([
    'failed read',
    'reader',
    'local account',
    'credential hint',
    'scopes',
    'write',
    'read',
    'read-only',
    'demo',
  ])('withdraws on %s and never revives the draft', async (kind) => {
    await mount();
    fill();
    const changed = status();
    if (kind === 'reader') changed.currentUsername = 'another-reader';
    if (kind === 'local account') changed.authUsername = 'another-local';
    if (kind === 'credential hint') changed.apiTokenHint = 'another…hint';
    if (kind === 'scopes') changed.tokenScopes = ['settings:read'];
    if (kind === 'write') changed.settingsCapabilities!.authenticationWrite = false;
    if (kind === 'read') changed.settingsCapabilities!.authenticationRead = false;
    if (kind === 'read-only' || kind === 'demo')
      changed.presentationPolicy = {
        readOnly: kind === 'read-only',
        demoMode: kind === 'demo',
        hideCommercial: false,
        hideUpgrade: false,
      };
    nextStatus = () => (kind === 'failed read' ? json({}, 403) : json(changed));
    await access.loadSecurityStatus();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    nextStatus = () => json(status());
    await access.loadSecurityStatus();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    setOpen(true);
    expectBlank();
  });

  it('cancels an idle draft on an organisation event, including a return to the same ID', async () => {
    await mount();
    fill();
    eventBus.emit('org_switched', 'fixture-estate');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    setOpen(true);
    expectBlank();
  });

  it('uses the saved local account and original org, UTF-8 credentials, one write and owned success', async () => {
    await mount();
    fill('fixture-currént-only');
    const pending = deferred<Response>();
    nextWrite = () => pending.promise;
    const schedule = vi.spyOn(globalThis, 'setTimeout');
    const cancel = vi.spyOn(globalThis, 'clearTimeout');
    submit();
    submit();
    expect(writes()).toHaveLength(1);
    const [, options] = writes()[0];
    expect(options.headers['X-Pulse-Org-ID']).toBe('fixture-estate');
    expect(options.headers.Authorization).toBe(
      `Basic ${btoa(String.fromCharCode(...new TextEncoder().encode('configured-local:fixture-currént-only')))}`,
    );
    expect(JSON.parse(options.body)).toEqual({
      currentPassword: 'fixture-currént-only',
      newPassword: 'fixture-new-password-only',
    });
    pending.resolve(json({}));
    await waitFor(() => expect(notificationStore.success).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(fetchMock.mock.calls.filter(([url]) => url === '/api/security/status')).toHaveLength(1);
    const index = schedule.mock.calls.findIndex(([, delay]) => delay === 2000);
    expect(index).toBeGreaterThanOrEqual(0);
    const timer = schedule.mock.results[index].value;
    eventBus.emit('org_switched', 'fixture-estate');
    expect(cancel).toHaveBeenCalledWith(timer);
    setOpen(true);
    expectBlank();
  });

  it('does not invent an admin account when status omits a local username', async () => {
    nextStatus = () => json({ ...status(), authUsername: undefined });
    await mount();
    fill();
    nextWrite = () => json({ error: 'Current password is incorrect' }, 401);
    submit();
    await screen.findByRole('alert');
    expect(writes()[0][1].headers.Authorization).toBeUndefined();
  });

  it.each(['org', 'reader', 'close', 'dispose', 'late 401'])(
    'aborts pending work on %s and ignores a late success or failure',
    async (kind) => {
      await mount();
      fill();
      const pending = deferred<Response>();
      nextWrite = () => pending.promise;
      const schedule = vi.spyOn(globalThis, 'setTimeout');
      submit();
      const signal = writes()[0][1].signal;
      if (kind === 'org' || kind === 'late 401') {
        setOrgID('fixture-other');
        eventBus.emit('org_switched', 'fixture-other');
      }
      if (kind === 'reader') {
        nextStatus = () => json({ ...status(), currentUsername: 'other-reader' });
        await access.loadSecurityStatus();
      }
      if (kind === 'close') setOpen(false);
      if (kind === 'dispose') cleanup();
      expect(signal.aborted).toBe(true);
      pending.resolve(kind === 'late 401' ? json({}, 401) : json({}));
      await Promise.resolve();
      await Promise.resolve();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(schedule.mock.calls.filter(([, delay]) => delay === 2000)).toHaveLength(0);
      expect(writes()).toHaveLength(1);
    },
  );

  it('ignores a rejection body that finishes after reader withdrawal', async () => {
    await mount();
    fill();
    const body = deferred<string>();
    const response = json({}, 400);
    vi.spyOn(response, 'text').mockReturnValue(body.promise);
    nextWrite = () => response;
    submit();
    await waitFor(() => expect(response.text).toHaveBeenCalled());
    eventBus.emit('org_switched', 'fixture-estate');
    body.resolve('{"message":"Old reader error"}');
    await Promise.resolve();
    await Promise.resolve();
    expect(notificationStore.error).not.toHaveBeenCalled();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it.each([401, 403, 429])(
    'does not automatically retry a confirmed HTTP %s rejection',
    async (code) => {
      await mount();
      fill();
      nextWrite = () => json({ error: 'Fixture refusal' }, code);
      submit();
      await screen.findByRole('alert');
      expect(writes()).toHaveLength(1);
      expect(screen.getByRole('button', { name: 'Change Password' })).toBeEnabled();
      expect(notificationStore.success).not.toHaveBeenCalled();
    },
  );

  it.each(['network', 'server'])(
    'retains an unknown %s outcome without enabling another blind write',
    async (kind) => {
      await mount();
      fill();
      nextWrite = () =>
        kind === 'network' ? Promise.reject(new Error('network unavailable')) : json({}, 500);
      submit();
      expect(await screen.findByRole('alert')).toHaveTextContent('could not be confirmed');
      expect(screen.getByRole('button', { name: 'Change Password' })).toBeDisabled();
      submit();
      expect(writes()).toHaveLength(1);
      expect(notificationStore.success).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    },
  );

  it('clears a parent-closed draft, leaves validation local and never sends mismatched passwords', async () => {
    await mount();
    fill();
    input('Confirm New Password', 'not-matching');
    submit();
    expect(screen.getByRole('alert')).toHaveTextContent('do not match');
    expect(writes()).toHaveLength(0);
    setOpen(false);
    setOpen(true);
    expectBlank();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
