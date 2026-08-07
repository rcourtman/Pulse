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
  shouldBlockSettingsRouteItem: (...args: unknown[]) => shouldBlockSettingsRouteItemMock(...args),
  shouldHideSettingsNavItem: (...args: unknown[]) => shouldHideSettingsNavItemMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    error: vi.fn(),
  },
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

  it('falls back to the default tab when the current route is no longer allowed', async () => {
    const setActiveTabSpy = vi.fn();
    hasFeatureMock.mockReturnValue(false);
    shouldBlockSettingsRouteItemMock.mockImplementation(
      (tab: string) => tab === 'organization-access',
    );

    renderHarness(setActiveTabSpy);

    await waitFor(() => {
      expect(setActiveTabSpy).toHaveBeenCalledWith('infrastructure-systems');
    });
  });

  it('falls back to General rather than Plans when the admin-only tabs are blocked', async () => {
    // A non-admin session: Infrastructure, Availability checks, the three Pulse
    // Intelligence tabs and the Support tabs are all gated on capabilities the
    // session does not hold. Plain catalog order resolves that to
    // system-billing, so every non-admin landed on an upgrade page it cannot
    // act on. General is user-scoped (appearance, language, units), so it is
    // the tab a non-admin can actually use.
    const setActiveTabSpy = vi.fn();
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
      'api',
      'security-auth',
      'security-sso',
      'security-roles',
      'security-users',
      'security-audit',
      'security-webhooks',
      'system-relay',
    ]);
    hasFeatureMock.mockReturnValue(false);
    shouldBlockSettingsRouteItemMock.mockImplementation((tab: string) =>
      blockedForNonAdmin.has(tab),
    );

    // Starts on a tab with no requiredCapability so the effect is not waiting
    // on the security status; the gates being exercised are the fallback's, not
    // the entry route's.
    renderHarness(setActiveTabSpy, 'organization-access');

    await waitFor(() => {
      expect(setActiveTabSpy).toHaveBeenCalledWith('system-general');
    });
    expect(setActiveTabSpy).not.toHaveBeenCalledWith('system-billing');
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
