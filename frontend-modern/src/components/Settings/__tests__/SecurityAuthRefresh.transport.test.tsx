import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { batch, createSignal, Show } from 'solid-js';
import { SecurityAuthPanel } from '../SecurityAuthPanel';
import { useSettingsAccess } from '../useSettingsAccess';
import { useSystemSettingsState } from '../useSystemSettingsState';
import { canMountSettingsPanel } from '../settingsNavVisibility';
import type { SettingsTab } from '../settingsNavigationModel';
import type { SecurityStatus } from '@/types/config';
import type { VersionInfo } from '@/api/updates';
import { eventBus } from '@/stores/events';
import { setOrgID } from '@/utils/apiClient';
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
  notificationStore: { success: vi.fn(), info: vi.fn(), error: vi.fn() },
}));

const fetchMock = vi.fn();
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const savedStatus = (): SecurityStatus => ({
  hasAuthentication: true,
  requiresAuth: true,
  authUsername: 'local-admin',
  currentUsername: 'reader-admin',
  apiTokenHint: 'fixture…hint',
  tokenScopes: ['settings:read', 'settings:write'],
  settingsCapabilities: {
    authenticationRead: true,
    authenticationWrite: true,
  } as SecurityStatus['settingsCapabilities'],
});
const deferred = () => {
  let resolve!: (value: Response) => void;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
const writes = () =>
  fetchMock.mock.calls.filter(([, options]) => options.method && options.method !== 'GET');

function formProps() {
  const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
  const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(false);
  const [showPasswordModal, setShowPasswordModal] = createSignal(false);
  return {
    showQuickSecuritySetup,
    setShowQuickSecuritySetup,
    showQuickSecurityWizard,
    setShowQuickSecurityWizard,
    showPasswordModal,
    setShowPasswordModal,
    versionInfo: () => ({ deploymentType: 'systemd' }) as VersionInfo,
    hideLocalLogin: () => false,
    hideLocalLoginLocked: () => false,
    savingHideLocalLogin: () => false,
    handleHideLocalLoginChange: vi.fn(),
    loadSecurityStatus: vi.fn(),
  };
}

function mountPanel(initial: SecurityStatus | null = savedStatus(), initiallyLoading = false) {
  let controls!: {
    setStatus: (status: SecurityStatus | null) => void;
    setLoading: (loading: boolean) => void;
    setCanManage: (allowed: boolean) => void;
  };
  render(() => {
    const [status, setStatus] = createSignal(initial);
    const [loading, setLoading] = createSignal(initiallyLoading);
    const [canManage, setCanManage] = createSignal(true);
    controls = { setStatus, setLoading, setCanManage };
    return (
      <SecurityAuthPanel
        {...formProps()}
        securityStatus={status}
        securityStatusLoading={loading}
        canManage={canManage()}
      />
    );
  });
  return controls;
}

function mountTransport() {
  let access!: ReturnType<typeof useSettingsAccess>;
  render(() => {
    const [activeTab, setActiveTab] = createSignal<SettingsTab>('security-auth');
    access = useSettingsAccess({ activeTab, setActiveTab, searchQuery: () => '' });
    const system = useSystemSettingsState({
      activeTab,
      loadSecurityStatus: access.loadSecurityStatus,
      setDiscoveryEnabled: () => false,
      applySavedDiscoverySubnet: () => {},
    });
    return (
      <Show
        when={canMountSettingsPanel('security-auth', access.securityStatus()?.settingsCapabilities)}
      >
        <SecurityAuthPanel
          {...formProps()}
          securityStatus={access.securityStatus}
          securityStatusLoading={access.securityStatusLoading}
          canManage={access.securityStatus()?.settingsCapabilities?.authenticationWrite === true}
          hideLocalLogin={system.hideLocalLogin}
          savingHideLocalLogin={system.savingHideLocalLogin}
          handleHideLocalLoginChange={system.handleHideLocalLoginChange}
          loadSecurityStatus={access.loadSecurityStatus}
        />
      </Show>
    );
  });
  return { access: () => access };
}

const openDraft = async (setup = false) => {
  fireEvent.click(
    await screen.findByRole('button', {
      name: setup ? 'Setup' : 'Rotate credentials',
    }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Custom' }));
  fireEvent.input(screen.getByLabelText('Username', { exact: true }), {
    target: { value: 'unsaved-draft-user' },
  });
  fireEvent.input(screen.getByLabelText('Password (min 12 characters)', { exact: true }), {
    target: { value: 'synthetic-only-password' },
  });
  fireEvent.input(screen.getByLabelText('Confirm password', { exact: true }), {
    target: { value: 'synthetic-only-password' },
  });
};
const expectDraft = () => {
  expect(screen.getByLabelText('Username', { exact: true })).toHaveValue('unsaved-draft-user');
  expect(screen.getByLabelText('Password (min 12 characters)', { exact: true })).toHaveValue(
    'synthetic-only-password',
  );
  expect(screen.getByLabelText('Confirm password', { exact: true })).toHaveValue(
    'synthetic-only-password',
  );
};

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  document.cookie = 'pulse_csrf=fixture-csrf';
  setOrgID('authentication-a');
  syncSessionPresentationPolicy(null);
  fetchMock.mockImplementation(async (url: string) =>
    url === '/api/security/status' ? json(savedStatus()) : json({}),
  );
});
afterEach(() => {
  cleanup();
  setOrgID(null);
  vi.unstubAllGlobals();
});

describe('Authentication status and credential draft refresh', () => {
  it('does not infer disabled authentication from an unresolved or unavailable status', () => {
    const state = mountPanel(null, true);
    expect(screen.getByRole('status')).toHaveTextContent('Checking authentication status…');
    expect(screen.queryByText('Authentication disabled')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Setup' })).toBeNull();
    state.setLoading(false);
    expect(screen.getByRole('status')).toHaveTextContent('Authentication status is unavailable.');
    expect(screen.queryByText('Authentication disabled')).toBeNull();
    expect(writes()).toHaveLength(0);
  });

  it('keeps a same-reader custom rotation draft disabled through the real toggle POST and pending status GET', async () => {
    const refresh = deferred();
    let reads = 0;
    fetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/security/status')
        return ++reads === 1 ? json(savedStatus()) : refresh.promise;
      if (url === '/api/system/settings/update') return json({});
      throw new Error(`Unexpected path: ${url}`);
    });
    mountTransport();
    await openDraft();
    const username = screen.getByLabelText('Username', { exact: true });
    const toggle = screen.getByRole('button', { name: 'Hide local login form' });
    fireEvent.click(toggle);
    await waitFor(() => expect(reads).toBe(2));
    expect(screen.getByRole('status')).toHaveTextContent('Refreshing authentication status…');
    expectDraft();
    expect(username).toBeDisabled();
    expect(toggle).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Change password' })).toBeDisabled();
    expect(screen.getAllByRole('button', { name: 'Rotate credentials' })).toHaveLength(2);
    for (const button of screen.getAllByRole('button', { name: 'Rotate credentials' }))
      expect(button).toBeDisabled();
    refresh.resolve(json({ ...savedStatus(), tokenScopes: ['settings:write', 'settings:read'] }));
    await waitFor(() => expect(username).toBeEnabled());
    expect(screen.getByLabelText('Username', { exact: true })).toBe(username);
    expect(screen.getByRole('button', { name: 'Hide local login form' })).toBe(toggle);
    expectDraft();
    expect(toggle).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByRole('status')).toBeNull();
    expect(writes()).toHaveLength(1);
    expect(writes()[0][0]).toBe('/api/system/settings/update');
    expect(JSON.parse(writes()[0][1].body)).toEqual({ hideLocalLogin: true });
  });

  it.each([401, 403, 503])(
    'still withdraws the panel and discards the draft after a %i refresh',
    async (code) => {
      const reader = mountTransport();
      await openDraft();
      fetchMock.mockResolvedValue(json({ error: 'PRIVATE_STATUS_CANARY' }, code));
      await reader.access().loadSecurityStatus();
      expect(reader.access().securityStatus()).toBeNull();
      expect(screen.queryByLabelText('Username', { exact: true })).toBeNull();
      expect(screen.queryByText('Authentication disabled')).toBeNull();
      expect(document.body).not.toHaveTextContent('PRIVATE_STATUS_CANARY');
      fetchMock.mockResolvedValue(json(savedStatus()));
      await reader.access().loadSecurityStatus();
      await screen.findByRole('button', { name: 'Change password' });
      expect(screen.queryByLabelText('Username', { exact: true })).toBeNull();
      expect(writes()).toHaveLength(0);
    },
  );

  it.each([
    { currentUsername: 'another-admin' },
    { authUsername: 'another-local-admin' },
    { apiTokenHint: 'another…hint' },
    { tokenScopes: ['settings:read'] },
  ])('does not transfer a draft to a changed reader/credential: %j', async (change) => {
    const state = mountPanel();
    await openDraft();
    batch(() => {
      state.setLoading(true);
      state.setStatus({ ...savedStatus(), ...change });
    });
    expect(screen.queryByLabelText('Username', { exact: true })).toBeNull();
    state.setLoading(false);
    fireEvent.click(screen.getByRole('button', { name: 'Rotate credentials' }));
    fireEvent.click(screen.getByRole('button', { name: 'Custom' }));
    expect(screen.getByLabelText('Username', { exact: true })).not.toHaveValue(
      'unsaved-draft-user',
    );
    expect(screen.getByLabelText('Password (min 12 characters)', { exact: true })).toHaveValue('');
    expect(writes()).toHaveLength(0);
  });

  it('does not revive a draft after management is withdrawn and restored', async () => {
    const state = mountPanel();
    await openDraft();
    state.setCanManage(false);
    expect(screen.queryByLabelText('Username', { exact: true })).toBeNull();
    expect(screen.getByRole('button', { name: 'Change password' })).toBeDisabled();
    expect(
      screen.getByText('Authentication settings are read-only for this account.'),
    ).toBeVisible();
    state.setCanManage(true);
    expect(screen.queryByLabelText('Username', { exact: true })).toBeNull();
    expect(writes()).toHaveLength(0);
  });

  it('discards credential drafts on organisation switches, including a return to the first organisation', async () => {
    mountPanel();
    await openDraft();
    setOrgID('authentication-b');
    eventBus.emit('org_switched', 'authentication-b');
    expect(screen.queryByLabelText('Username', { exact: true })).toBeNull();
    setOrgID('authentication-a');
    eventBus.emit('org_switched', 'authentication-a');
    expect(screen.queryByLabelText('Username', { exact: true })).toBeNull();
    expect(writes()).toHaveLength(0);
  });

  it('preserves a known disabled setup draft during refresh without enabling any controls', async () => {
    const status = { ...savedStatus(), hasAuthentication: false, apiTokenConfigured: false };
    const state = mountPanel(status);
    await openDraft(true);
    state.setLoading(true);
    expect(screen.getByText('Authentication disabled')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Setup' })).toBeDisabled();
    expectDraft();
    expect(screen.getByLabelText('Username', { exact: true })).toBeDisabled();
    state.setStatus({ ...status });
    state.setLoading(false);
    expectDraft();
    expect(screen.getByLabelText('Username', { exact: true })).toBeEnabled();
    expect(writes()).toHaveLength(0);
  });

  it('keeps configured restart guidance intact during a pending refresh', () => {
    const state = mountPanel({ ...savedStatus(), configuredButPendingRestart: true });
    expect(screen.getByText('sudo systemctl restart pulse')).toBeVisible();
    state.setLoading(true);
    expect(screen.getByText('sudo systemctl restart pulse')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Change password' })).toBeDisabled();
    expect(writes()).toHaveLength(0);
  });
});
