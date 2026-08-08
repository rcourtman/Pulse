import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { useSettingsAccess } from '../useSettingsAccess';

const hasFeatureMock = vi.fn();
const runtimeCapabilitiesLoadedMock = vi.fn();
const isHostedModeEnabledMock = vi.fn();
const presentationPolicyHidesCommercialSurfacesMock = vi.fn();
const presentationPolicyHidesOrganizationSurfacesMock = vi.fn();
const presentationPolicyIsDemoModeMock = vi.fn();
const presentationPolicyIsReadOnlyMock = vi.fn();
const sessionPresentationPolicyResolvedMock = vi.fn();
const shouldHideSettingsNavItemMock = vi.fn();
const shouldBlockSettingsRouteItemMock = vi.fn();
const apiFetchMock = vi.fn();

vi.mock('@/stores/license', () => ({
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  isHostedModeEnabled: (...args: unknown[]) => isHostedModeEnabledMock(...args),
  runtimeCapabilitiesLoaded: (...args: unknown[]) => runtimeCapabilitiesLoadedMock(...args),
  getRuntimeCapabilityBlock: () => undefined,
  isRuntimeCapabilityBlocked: () => false,
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesCommercialSurfaces: (...args: unknown[]) =>
    presentationPolicyHidesCommercialSurfacesMock(...args),
  presentationPolicyHidesOrganizationSurfaces: (...args: unknown[]) =>
    presentationPolicyHidesOrganizationSurfacesMock(...args),
  presentationPolicyIsDemoMode: (...args: unknown[]) => presentationPolicyIsDemoModeMock(...args),
  presentationPolicyIsReadOnly: (...args: unknown[]) => presentationPolicyIsReadOnlyMock(...args),
  sessionPresentationPolicyResolved: (...args: unknown[]) =>
    sessionPresentationPolicyResolvedMock(...args),
  syncSessionPresentationPolicy: vi.fn(),
}));

vi.mock('../settingsNavVisibility', () => ({
  canMountSettingsPanel: (tab: string) =>
    ![
      'infrastructure-systems',
      'monitoring-availability',
      'system-ai',
      'system-ai-patrol',
      'system-ai-assistant',
      'support-diagnostics',
      'support-reporting',
      'support-logs',
      'system-network',
      'system-updates',
      'system-recovery',
      'api',
      'security-auth',
      'security-sso',
      'security-roles',
      'security-users',
      'security-audit',
      'security-webhooks',
      'system-relay',
    ].includes(tab),
  shouldBlockSettingsRouteItem: (...args: unknown[]) => shouldBlockSettingsRouteItemMock(...args),
  shouldHideSettingsNavItem: (...args: unknown[]) => shouldHideSettingsNavItemMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetch: (...args: unknown[]) => apiFetchMock(...args),
}));

function renderHarness(setActiveTabSpy: (tab: string) => void, initialTab = 'organization-access') {
  return render(() => {
    const [activeTab, setActiveTab] = createSignal(initialTab as never);
    useSettingsAccess({
      activeTab,
      setActiveTab: (tab) => {
        setActiveTabSpy(tab);
        setActiveTab(tab as never);
      },
      searchQuery: () => '',
    });
    return null;
  });
}

describe('useSettingsAccess', () => {
  beforeEach(() => {
    hasFeatureMock.mockReset();
    runtimeCapabilitiesLoadedMock.mockReset();
    isHostedModeEnabledMock.mockReset();
    presentationPolicyHidesCommercialSurfacesMock.mockReset();
    presentationPolicyHidesOrganizationSurfacesMock.mockReset();
    presentationPolicyIsDemoModeMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReset();
    sessionPresentationPolicyResolvedMock.mockReset();
    shouldHideSettingsNavItemMock.mockReset();
    shouldBlockSettingsRouteItemMock.mockReset();
    apiFetchMock.mockReset();

    hasFeatureMock.mockImplementation((feature: string) => feature === 'multi_tenant');
    runtimeCapabilitiesLoadedMock.mockReturnValue(true);
    isHostedModeEnabledMock.mockReturnValue(false);
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(false);
    presentationPolicyHidesOrganizationSurfacesMock.mockReturnValue(false);
    presentationPolicyIsDemoModeMock.mockReturnValue(false);
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    sessionPresentationPolicyResolvedMock.mockReturnValue(true);
    shouldHideSettingsNavItemMock.mockImplementation(
      (tab: string) => tab === 'organization-access',
    );
    shouldBlockSettingsRouteItemMock.mockReturnValue(false);
    apiFetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ settingsCapabilities: {} }),
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps an explicit organization route active when the tab contract still allows it', async () => {
    const setActiveTabSpy = vi.fn();

    renderHarness(setActiveTabSpy);

    await waitFor(() => {
      expect(setActiveTabSpy).not.toHaveBeenCalled();
    });
  });

  it('falls back to General while the default capability is unresolved', async () => {
    const setActiveTabSpy = vi.fn();
    hasFeatureMock.mockReturnValue(false);
    shouldBlockSettingsRouteItemMock.mockImplementation(
      (tab: string) => tab === 'organization-access',
    );

    renderHarness(setActiveTabSpy);

    await waitFor(() => {
      expect(setActiveTabSpy).toHaveBeenCalledWith('system-general');
    });
  });

  it('prefers General over catalog order when admin-only routes are blocked', async () => {
    const setActiveTabSpy = vi.fn();
    hasFeatureMock.mockReturnValue(false);
    const blockedForNonAdmin = new Set([
      'organization-access',
      'infrastructure-systems',
      'monitoring-availability',
      'system-ai',
      'system-ai-patrol',
      'system-ai-assistant',
      'support-diagnostics',
      'support-reporting',
      'support-logs',
      'system-network',
      'system-updates',
      'system-recovery',
      'api',
      'security-auth',
      'security-sso',
      'security-roles',
      'security-users',
      'security-audit',
      'security-webhooks',
      'system-relay',
    ]);
    shouldBlockSettingsRouteItemMock.mockImplementation((tab: string) =>
      blockedForNonAdmin.has(tab),
    );

    renderHarness(setActiveTabSpy);

    await waitFor(() => {
      expect(setActiveTabSpy).toHaveBeenCalledWith('system-general');
    });
    expect(setActiveTabSpy).not.toHaveBeenCalledWith('system-billing');
  });

  it('treats a failed security-status request as a resolved denial', async () => {
    const setActiveTabSpy = vi.fn();
    apiFetchMock.mockResolvedValue({ ok: false, status: 503 });
    shouldBlockSettingsRouteItemMock.mockImplementation(
      (tab: string, context: { settingsCapabilitiesResolved?: boolean }) =>
        tab === 'infrastructure-systems' && context.settingsCapabilitiesResolved === true,
    );

    renderHarness(setActiveTabSpy, 'infrastructure-systems');

    await waitFor(() => {
      expect(setActiveTabSpy).toHaveBeenCalledWith('system-general');
    });
  });

  it('deduplicates concurrent security-status loads', async () => {
    let resolveFetch!: (value: { ok: boolean; json: () => Promise<unknown> }) => void;
    apiFetchMock.mockReturnValue(
      new Promise((resolve) => {
        resolveFetch = resolve;
      }),
    );
    let access!: ReturnType<typeof useSettingsAccess>;
    render(() => {
      const [activeTab, setActiveTab] = createSignal('system-general' as never);
      access = useSettingsAccess({
        activeTab,
        setActiveTab: setActiveTab as never,
        searchQuery: () => '',
      });
      return null;
    });

    await waitFor(() => expect(apiFetchMock).toHaveBeenCalledTimes(1));
    const first = access.loadSecurityStatus();
    const second = access.loadSecurityStatus();
    expect(apiFetchMock).toHaveBeenCalledTimes(1);

    resolveFetch({ ok: true, json: async () => ({ settingsCapabilities: {} }) });
    await Promise.all([first, second]);
  });

  it('keeps direct feature-gated routes active when the panel owns the locked state', async () => {
    const setActiveTabSpy = vi.fn();
    hasFeatureMock.mockReturnValue(false);
    shouldHideSettingsNavItemMock.mockImplementation((tab: string) => tab === 'support-reporting');
    shouldBlockSettingsRouteItemMock.mockReturnValue(false);

    renderHarness(setActiveTabSpy, 'support-reporting');

    await waitFor(() => {
      expect(setActiveTabSpy).not.toHaveBeenCalled();
    });
  });

  it.each(['mcp', 'claude', 'opencode', 'connector', 'external agent'])(
    'sidebar search %j surfaces the Assistant page hosting the external-agent connector setup',
    async (query) => {
      shouldHideSettingsNavItemMock.mockReturnValue(false);
      let access!: ReturnType<typeof useSettingsAccess>;
      render(() => {
        const [activeTab, setActiveTab] = createSignal('system-ai-assistant' as never);
        access = useSettingsAccess({
          activeTab,
          setActiveTab: setActiveTab as never,
          searchQuery: () => query,
        });
        return null;
      });

      await waitFor(() => {
        const matchedIds = access
          .filteredTabGroups()
          .flatMap((group) => group.items.map((item) => item.id));
        expect(matchedIds).toContain('system-ai-assistant');
      });
    },
  );
});
