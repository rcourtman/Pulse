import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';

import { buildProjectedOverrides } from '@/features/alerts/alertOverridesModel';
import type { Override } from '@/features/alerts/types';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource } from '@/types/resource';

import { useThresholdsOverrideMutations } from '../useThresholdsOverrideMutations';
import { useThresholdsPlatformData } from '../useThresholdsPlatformData';

const canonicalID = 'agent-b9ed6d0e20e94eaf';
const legacyCanonicalID = 'agent-535886018cb53055';
const connectionID = 'truenas-connection-1';

const connectionBackedTrueNASResource = (machineId: string): Resource =>
  ({
    id: canonicalID,
    type: 'agent',
    name: 'strawberrynas',
    displayName: 'Strawberry NAS',
    status: 'online',
    platformType: 'truenas',
    sources: ['truenas'],
    identity: {
      hostname: 'strawberrynas',
      machineId,
    },
    truenas: {
      hostname: 'strawberrynas',
    },
    metricsTarget: {
      resourceType: 'agent',
      resourceId: connectionID,
    },
    canonicalIdentity: {
      primaryId: `agent:${connectionID}`,
      aliases: [canonicalID, connectionID, 'strawberrynas'],
      supersededIds: [legacyCanonicalID],
    },
    memory: {
      current: 87,
    },
  }) as Resource;

const projectOverrides = (
  rawConfig: Record<string, RawOverrideConfig>,
  resource: Resource,
): Override[] =>
  buildProjectedOverrides({
    rawConfig,
    nodeResources: [],
    vmResources: [],
    containerResources: [],
    storageResources: [],
    agentResourceList: [resource],
    containerRuntimeResources: [resource],
    getChildren: () => [],
    pbsInstanceById: new Map(),
    allResources: [resource],
  });

describe('TrueNAS threshold persistence identity', () => {
  it('re-homes a connection-target override onto the canonical ID and survives reload/refetch', () => {
    const [resource, setResource] = createSignal(
      connectionBackedTrueNASResource('serial-visible-on-first-poll'),
    );
    const initialRawConfig: Record<string, RawOverrideConfig> = {
      [connectionID]: {
        memory: {
          trigger: 95,
          clear: 90,
        },
      },
    };
    const [rawOverridesConfig, setRawOverridesConfig] = createSignal(initialRawConfig);
    const [overrides, setOverrides] = createSignal<Override[]>(
      projectOverrides(initialRawConfig, resource()),
    );
    const [editingId] = createSignal<string | null>(null);
    const [editingThresholds] = createSignal<Record<string, number | undefined>>({
      memory: 95,
    });
    const [editingNote] = createSignal('');
    const [bulkEditIds] = createSignal<string[]>([]);
    const setHasUnsavedChanges = vi.fn();
    const cancelEdit = vi.fn();

    const props = {
      get allResources() {
        return [resource()];
      },
      overrides,
      setOverrides,
      rawOverridesConfig,
      setRawOverridesConfig,
      trueNASDefaults: {
        memory: 85,
      },
      trueNASDiskDefaults: {},
      backupDefaults: () => ({ enabled: false, warningDays: 7, criticalDays: 14 }),
      snapshotDefaults: () => ({
        enabled: false,
        warningDays: 30,
        criticalDays: 45,
        warningSizeGiB: 0,
        criticalSizeGiB: 0,
      }),
      guestDisableConnectivity: () => false,
      guestPoweredOffSeverity: () => 'warning' as const,
      dockerDisableConnectivity: () => false,
      dockerPoweredOffSeverity: () => 'warning' as const,
      setHasUnsavedChanges,
    } as unknown as ThresholdsTableProps;

    const platform = renderHook(() =>
      useThresholdsPlatformData({
        props,
        editingId,
        searchTerm: () => '',
      }),
    );
    const mutations = renderHook(() =>
      useThresholdsOverrideMutations({
        props,
        resources: {
          nodesWithOverrides: () => [],
          agentsWithOverrides: () => [],
          agentDisksWithOverrides: () => [],
          dockerHostsWithOverrides: () => [],
          guestsFlat: () => [],
          dockerContainersFlat: () => [],
          pbsServersWithOverrides: () => [],
          pmgServersWithOverrides: () => [],
          storageWithOverrides: () => [],
          trueNASSystemsWithOverrides: platform.result.trueNASSystemsWithOverrides,
        },
        editingThresholds,
        editingNote,
        bulkEditIds,
        cancelEdit,
        updateBackupDefaults: vi.fn(),
        updateSnapshotDefaults: vi.fn(),
      }),
    );

    expect(platform.result.trueNASSystemsWithOverrides()).toEqual([
      expect.objectContaining({
        id: canonicalID,
        hasOverride: true,
        thresholds: {
          memory: 95,
        },
      }),
    ]);

    mutations.result.saveEdit(canonicalID);

    expect(rawOverridesConfig()).toEqual({
      [canonicalID]: {
        memory: {
          trigger: 95,
          clear: 90,
        },
      },
    });
    expect(rawOverridesConfig()).not.toHaveProperty(connectionID);

    // Model the alerts API JSON round-trip, followed by a resource refetch where
    // TrueNAS changes the reported machine serial. The configured connection
    // remains the durable identity, so the canonical resource ID must not move.
    const persisted = JSON.parse(JSON.stringify(rawOverridesConfig())) as Record<
      string,
      RawOverrideConfig
    >;
    setRawOverridesConfig(persisted);
    setResource(connectionBackedTrueNASResource('serial-missing-on-next-poll'));
    setOverrides(projectOverrides(persisted, resource()));

    expect(platform.result.trueNASSystemsWithOverrides()).toEqual([
      expect.objectContaining({
        id: canonicalID,
        hasOverride: true,
        thresholds: {
          memory: 95,
        },
      }),
    ]);
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
  });
});
