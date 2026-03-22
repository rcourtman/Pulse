import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';

import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { Resource as TableResource } from '@/features/alerts/thresholds/tableTypes';

import { useThresholdsAvailabilityMutations } from '../useThresholdsAvailabilityMutations';

const buildTableProps = (overridesSignal: ReturnType<typeof createSignal<any[]>>) => {
  const [overrides, setOverrides] = overridesSignal;
  const [rawOverridesConfig, setRawOverridesConfig] = createSignal<Record<string, any>>({});
  const setHasUnsavedChanges = vi.fn();
  const removeAlerts = vi.fn();

  const props = {
    overrides,
    setOverrides,
    rawOverridesConfig,
    setRawOverridesConfig,
    guestDisableConnectivity: () => false,
    guestPoweredOffSeverity: () => 'warning' as const,
    dockerDisableConnectivity: () => false,
    dockerPoweredOffSeverity: () => 'warning' as const,
    setHasUnsavedChanges,
    removeAlerts,
  } as unknown as ThresholdsTableProps;

  return {
    props,
    rawOverridesConfig,
    setHasUnsavedChanges,
    removeAlerts,
  };
};

describe('useThresholdsAvailabilityMutations', () => {
  it('owns powered-off severity and disable-connectivity persistence for guest resources', () => {
    const overrideSignal = createSignal<any[]>([]);
    const { props, rawOverridesConfig, setHasUnsavedChanges, removeAlerts } =
      buildTableProps(overrideSignal);
    const guestResource: TableResource = {
      id: 'vm-100',
      name: 'db-01',
      type: 'guest',
      resourceType: 'VM',
      vmid: 100,
      node: 'pve-1',
      instance: 'qemu/100',
      thresholds: {},
    };

    const { result } = renderHook(() =>
      useThresholdsAvailabilityMutations({
        props,
        resources: {
          nodesWithOverrides: () => [],
          agentsWithOverrides: () => [],
          agentDisksWithOverrides: () => [],
          dockerHostsWithOverrides: () => [],
          guestsFlat: () => [guestResource],
          dockerContainersFlat: () => [],
          pbsServersWithOverrides: () => [],
          storageWithOverrides: () => [],
        },
        removeOverride: vi.fn(),
      }),
    );

    result.setOfflineState('vm-100', 'critical');

    expect(overrideSignal[0]()).toEqual([
      expect.objectContaining({
        id: 'vm-100',
        disableConnectivity: false,
        poweredOffSeverity: 'critical',
      }),
    ]);
    expect(rawOverridesConfig()).toEqual({
      'vm-100': {
        poweredOffSeverity: 'critical',
      },
    });
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
    expect(removeAlerts).not.toHaveBeenCalled();
  });
});
