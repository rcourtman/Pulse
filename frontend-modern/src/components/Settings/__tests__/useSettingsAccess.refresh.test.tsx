import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal, Show } from 'solid-js';
import { useSettingsAccess } from '../useSettingsAccess';
import { canMountSettingsPanel } from '../settingsNavVisibility';
import {
  syncSessionPresentationPolicy,
  presentationPolicyIsReadOnly,
} from '@/stores/sessionPresentationPolicy';
import type { SecurityStatus } from '@/types/config';
import type { SettingsTab } from '../settingsNavigationModel';
import { sessionCanReadInfrastructureSettings } from '@/stores/sessionSettingsCapabilities';

const { apiFetchMock } = vi.hoisted(() => ({ apiFetchMock: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: apiFetchMock }));
vi.mock('@/utils/logger', () => ({ logger: { debug: vi.fn(), error: vi.fn() } }));
vi.mock('@/stores/license', () => ({
  hasFeature: () => true,
  runtimeCapabilitiesLoaded: () => true,
  isHostedModeEnabled: () => false,
  isRuntimeCapabilityBlocked: () => false,
}));

const grantedStatus = {
  currentUsername: 'test-admin',
  requiresAuth: true,
  settingsCapabilities: { infrastructureRead: true, authenticationRead: true },
};
const response = (status: Partial<SecurityStatus> = grantedStatus) => ({
  ok: true,
  json: async () => status,
});
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};

function mount() {
  let access!: ReturnType<typeof useSettingsAccess>;
  const view = render(() => {
    const [activeTab, setActiveTab] = createSignal<SettingsTab>('security-auth');
    access = useSettingsAccess({
      activeTab,
      setActiveTab,
      searchQuery: () => '',
    });
    return (
      <Show
        when={canMountSettingsPanel('security-auth', access.securityStatus()?.settingsCapabilities)}
      >
        <p>Privileged panel</p>
      </Show>
    );
  });
  return { access, view };
}

describe('Settings capability refresh withdrawal', () => {
  beforeEach(() => {
    apiFetchMock.mockReset();
    apiFetchMock.mockResolvedValue(response());
    syncSessionPresentationPolicy(null);
  });
  afterEach(cleanup);

  it.each([401, 403, 503])(
    'withdraws previously granted panels after a %i refresh',
    async (status) => {
      const { access, view } = mount();
      await waitFor(() => expect(view.queryByText('Privileged panel')).not.toBeNull());
      expect(sessionCanReadInfrastructureSettings()).toBe(true);
      apiFetchMock.mockResolvedValue({ ok: false, status });
      await access.loadSecurityStatus();
      expect(access.securityStatus()).toBeNull();
      expect(view.queryByText('Privileged panel')).toBeNull();
      expect(sessionCanReadInfrastructureSettings()).toBe(false);
      expect(access.securityStatusError()).toBe(true);
      expect(apiFetchMock.mock.calls[1][1].retry).toBe(false);
    },
  );

  it.each(['network', 'json'])(
    'withdraws previously granted panels after a %s failure',
    async (failure) => {
      const { access, view } = mount();
      await waitFor(() => expect(view.queryByText('Privileged panel')).not.toBeNull());
      if (failure === 'network')
        apiFetchMock.mockRejectedValue(new Error('synthetic transport failure'));
      else
        apiFetchMock.mockResolvedValue({
          ok: true,
          json: async () => {
            throw new SyntaxError('synthetic invalid JSON');
          },
        });
      await access.loadSecurityStatus();
      expect(access.securityStatus()).toBeNull();
      expect(view.queryByText('Privileged panel')).toBeNull();
    },
  );

  it('does not publish an in-flight capability response after disposal', async () => {
    const pending = deferred<ReturnType<typeof response>>();
    apiFetchMock.mockReturnValue(pending.promise);
    const { access, view } = mount();
    await waitFor(() => expect(apiFetchMock).toHaveBeenCalledTimes(1));
    const request = access.loadSecurityStatus();
    view.unmount();
    pending.resolve(response());
    await request;
    expect(access.securityStatus()).toBeNull();
    expect(sessionCanReadInfrastructureSettings()).toBe(false);
    expect(apiFetchMock.mock.calls[0][1].signal.aborted).toBe(true);
    await access.loadSecurityStatus();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
  });

  it('keeps the same reader mounted while checking, then recovers only from a new successful check', async () => {
    const { access, view } = mount();
    await waitFor(() => expect(view.queryByText('Privileged panel')).not.toBeNull());
    const pending = deferred<{ ok: boolean; status: number }>();
    apiFetchMock.mockReturnValue(pending.promise);
    const first = access.loadSecurityStatus();
    const sameRequest = access.loadSecurityStatus();
    expect(first).toBe(sameRequest);
    expect(view.queryByText('Privileged panel')).not.toBeNull();
    expect(access.securityStatusLoading()).toBe(true);
    pending.resolve({ ok: false, status: 503 });
    await first;
    expect(view.queryByText('Privileged panel')).toBeNull();
    expect(access.securityStatusError()).toBe(true);
    apiFetchMock.mockResolvedValue(response());
    await access.loadSecurityStatus();
    expect(access.securityStatusError()).toBe(false);
    expect(view.queryByText('Privileged panel')).not.toBeNull();
    expect(sessionCanReadInfrastructureSettings()).toBe(true);
  });

  it('withdraws access without making a read-only session writable', async () => {
    apiFetchMock.mockResolvedValue(
      response({
        ...grantedStatus,
        presentationPolicy: { readOnly: true },
      }),
    );
    const { access } = mount();
    await waitFor(() => expect(access.securityStatusLoading()).toBe(false));
    expect(presentationPolicyIsReadOnly()).toBe(true);
    apiFetchMock.mockResolvedValue({ ok: false, status: 503 });
    await access.loadSecurityStatus();
    expect(presentationPolicyIsReadOnly()).toBe(true);
    expect(access.securityStatus()).toBeNull();
  });

  it.each([null, [], 'unexpected'])(
    'does not accept malformed status %j as a fresh check',
    async (body) => {
      const { access } = mount();
      await waitFor(() => expect(access.securityStatusLoading()).toBe(false));
      apiFetchMock.mockResolvedValue({ ok: true, json: async () => body });
      await access.loadSecurityStatus();
      expect(access.securityStatus()).toBeNull();
      expect(access.securityStatusError()).toBe(true);
    },
  );
});
