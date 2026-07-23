import { renderHook, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const mockGetConfig = vi.fn();
const mockUpdateConfig = vi.fn();
const mockLoadDestinations = vi.fn();
const mockSaveDestinations = vi.fn();
const mockReplaceRawOverridesConfig = vi.fn();
const mockRawOverridesConfig = vi.fn();
const containerRuntimeResources = () =>
  [
    {
      id: 'truenas-main',
      type: 'agent',
      platformType: 'truenas',
    },
  ] as any[];

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getConfig: (...args: unknown[]) => mockGetConfig(...args),
    updateConfig: (...args: unknown[]) => mockUpdateConfig(...args),
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
    saveDestinations: (...args: unknown[]) => mockSaveDestinations(...args),
  }),
}));

vi.mock('../useAlertOverridesState', () => ({
  useAlertOverridesState: () => ({
    overrides: () => [],
    setOverrides: vi.fn(),
    rawOverridesConfig: (...args: unknown[]) => mockRawOverridesConfig(...args),
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
    mockUpdateConfig.mockReset();
    mockLoadDestinations.mockReset();
    mockSaveDestinations.mockReset();
    mockReplaceRawOverridesConfig.mockReset();
    mockRawOverridesConfig.mockReset();
    mockGetConfig.mockResolvedValue({ overrides: {} });
    mockUpdateConfig.mockResolvedValue({ success: true });
    mockLoadDestinations.mockResolvedValue(undefined);
    mockSaveDestinations.mockResolvedValue(undefined);
    mockRawOverridesConfig.mockReturnValue({});
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
        type: 'agent',
        platformType: 'truenas',
      }),
    ]);
  });

  it('sends the canonical TrueNAS override through the global configuration save', async () => {
    const setHasUnsavedChanges = vi.fn();
    mockRawOverridesConfig.mockReturnValue({
      'agent-b9ed6d0e20e94eaf': {
        memory: {
          trigger: 95,
          clear: 90,
        },
      },
    });

    const { result } = renderHook(() =>
      useAlertsConfigurationState({
        activeTab: () => 'thresholds',
        allResources: () => [],
        byType: () => [],
        children: () => [],
        activeAlerts: {},
        removeAlerts: vi.fn(),
        setOverviewOverrides: vi.fn(),
        hasUnsavedChanges: () => true,
        setHasUnsavedChanges,
        alertsActivationState: () => 'active',
        alertsActivationConfig: () => ({ enabled: true }),
      }),
    );

    await waitFor(() => expect(mockGetConfig).toHaveBeenCalledTimes(1));
    await result.saveAlertConfiguration();

    expect(mockUpdateConfig).toHaveBeenCalledTimes(1);
    expect(mockUpdateConfig.mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({
        overrides: {
          'agent-b9ed6d0e20e94eaf': {
            memory: {
              trigger: 95,
              clear: 90,
            },
          },
        },
      }),
    );
    expect(mockSaveDestinations).toHaveBeenCalledTimes(1);
    expect(setHasUnsavedChanges).toHaveBeenLastCalledWith(false);
  });
});
