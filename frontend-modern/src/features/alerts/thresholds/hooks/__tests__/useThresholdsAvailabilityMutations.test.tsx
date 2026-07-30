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
      id: 'cluster-a:node-2:100',
      name: 'db-01',
      type: 'guest',
      resourceType: 'VM',
      vmid: 100,
      node: 'node-2',
      instance: 'cluster-a',
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

    result.setOfflineState('cluster-a:node-2:100', 'critical');

    expect(overrideSignal[0]()).toEqual([
      expect.objectContaining({
        id: 'guest:cluster-a:100',
        disableConnectivity: false,
        poweredOffSeverity: 'critical',
      }),
    ]);
    expect(rawOverridesConfig()).toEqual({
      'guest:cluster-a:100': {
        poweredOffSeverity: 'critical',
      },
    });
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
    expect(removeAlerts).not.toHaveBeenCalled();
  });

  it('persists guest filesystem disables under the stable override identity', () => {
    const overrideSignal = createSignal<any[]>([]);
    const { props, rawOverridesConfig, removeAlerts } = buildTableProps(overrideSignal);
    const filesystemResource: TableResource = {
      id: 'cluster-a:node-2:100-disk-boot-dev-vda1',
      overrideIdCandidates: [
        'guest-disk:guest:cluster-a:100/disk:boot-dev-vda1',
        'guest-disk:cluster-a:node-2:100/disk:boot-dev-vda1',
      ],
      overrideStorageId: 'guest-disk:guest:cluster-a:100/disk:boot-dev-vda1',
      name: '/boot',
      type: 'guestDisk',
      resourceType: 'Guest Filesystem',
      thresholds: {},
    };

    const { result } = renderHook(() =>
      useThresholdsAvailabilityMutations({
        props,
        resources: {
          nodesWithOverrides: () => [],
          agentsWithOverrides: () => [],
          agentDisksWithOverrides: () => [],
          guestDisksWithOverrides: () => [filesystemResource],
          dockerHostsWithOverrides: () => [],
          guestsFlat: () => [],
          dockerContainersFlat: () => [],
          pbsServersWithOverrides: () => [],
          storageWithOverrides: () => [],
        },
        removeOverride: vi.fn(),
      }),
    );

    result.toggleDisabled(filesystemResource.id, true);

    expect(overrideSignal[0]()).toEqual([
      expect.objectContaining({
        id: 'guest-disk:guest:cluster-a:100/disk:boot-dev-vda1',
        disabled: true,
        type: 'guestDisk',
      }),
    ]);
    expect(rawOverridesConfig()).toEqual({
      'guest-disk:guest:cluster-a:100/disk:boot-dev-vda1': {
        disabled: true,
      },
    });

    const removePredicate = removeAlerts.mock.calls[0]?.[0] as
      ((alert: { resourceId: string; type: string }) => boolean) | undefined;
    expect(removePredicate).toBeTypeOf('function');
    expect(
      removePredicate?.({
        resourceId: filesystemResource.id,
        type: 'disk',
      }),
    ).toBe(true);
  });
});
