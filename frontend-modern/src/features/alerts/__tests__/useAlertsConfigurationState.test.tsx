import { renderHook, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const mockGetConfig = vi.fn();
const mockLoadDestinations = vi.fn();
const mockReplaceRawOverridesConfig = vi.fn();
const containerRuntimeResources = () =>
  [
    {
      id: 'truenas-main',
      type: 'truenas',
    },
  ] as any[];

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getConfig: (...args: unknown[]) => mockGetConfig(...args),
    updateConfig: vi.fn(),
  },
}));

vi.mock('../useAlertDestinationsState', () => ({
  useAlertDestinationsState: () => ({
    isLoadingDestinations: () => false,
    destConfigLoadError: () => '',
    emailConfig: () => null,
    setEmailConfig: vi.fn(),
    appriseConfig: () => null,
    setAppriseConfig: vi.fn(),
    resetDestinations: vi.fn(),
    loadDestinations: (...args: unknown[]) => mockLoadDestinations(...args),
    saveDestinations: vi.fn(),
  }),
}));

vi.mock('../useAlertOverridesState', () => ({
  useAlertOverridesState: () => ({
    overrides: () => [],
    setOverrides: vi.fn(),
    rawOverridesConfig: () => ({}),
    setRawOverridesConfig: vi.fn(),
    replaceRawOverridesConfig: (...args: unknown[]) => mockReplaceRawOverridesConfig(...args),
    allGuests: () => [],
    agentResources: () => [],
    containerRuntimeResources,
    pbsInstances: () => [],
    pmgInstances: () => [],
  }),
}));

import { useAlertsConfigurationState } from '../useAlertsConfigurationState';

describe('useAlertsConfigurationState', () => {
  beforeEach(() => {
    mockGetConfig.mockReset();
    mockLoadDestinations.mockReset();
    mockReplaceRawOverridesConfig.mockReset();
    mockGetConfig.mockResolvedValue({ overrides: {} });
    mockLoadDestinations.mockResolvedValue(undefined);
  });

  it('surfaces canonical container runtime resources from the overrides owner', async () => {
    const { result } = renderHook(() =>
      useAlertsConfigurationState({
        activeTab: () => 'thresholds',
        allResources: () => [],
        byType: () => [],
        children: () => [],
        activeAlerts: {},
        removeAlerts: vi.fn(),
        setOverviewOverrides: vi.fn(),
        hasUnsavedChanges: () => false,
        setHasUnsavedChanges: vi.fn(),
        alertsActivationState: () => null,
        alertsActivationConfig: () => null,
      }),
    );

    await waitFor(() => expect(mockGetConfig).toHaveBeenCalledTimes(1));
    expect(result.containerRuntimeResources).toBe(containerRuntimeResources);
    expect(result.containerRuntimeResources()).toEqual([
      expect.objectContaining({
        id: 'truenas-main',
        type: 'truenas',
      }),
    ]);
  });
});
