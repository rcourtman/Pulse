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

  it('keeps direct feature-gated routes active when the panel owns the locked state', async () => {
    const setActiveTabSpy = vi.fn();
    hasFeatureMock.mockReturnValue(false);
    shouldHideSettingsNavItemMock.mockImplementation(
      (tab: string) => tab === 'support-reporting',
    );
    shouldBlockSettingsRouteItemMock.mockReturnValue(false);

    renderHarness(setActiveTabSpy, 'support-reporting');

    await waitFor(() => {
      expect(setActiveTabSpy).not.toHaveBeenCalled();
    });
  });
});
