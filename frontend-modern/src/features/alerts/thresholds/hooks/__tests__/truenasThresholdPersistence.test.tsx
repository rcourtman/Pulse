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
const secondCanonicalID = 'agent-4266ee45469c27f1';
const secondLegacyCanonicalID = 'agent-0d4080c143654b4f';
const secondConnectionID = 'truenas-connection-2';

const connectionBackedTrueNASResource = ({
  canonicalId = canonicalID,
  legacyId = legacyCanonicalID,
  connectionId = connectionID,
  machineId,
  memory = 87,
}: {
  canonicalId?: string;
  legacyId?: string;
  connectionId?: string;
  machineId: string;
  memory?: number;
}): Resource =>
  ({
    id: canonicalId,
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
      resourceId: connectionId,
    },
    canonicalIdentity: {
      primaryId: `agent:${connectionId}`,
      aliases: [canonicalId, connectionId, 'strawberrynas'],
      supersededIds: [legacyId],
    },
    memory: {
      current: memory,
    },
  }) as Resource;

const projectOverrides = (
  rawConfig: Record<string, RawOverrideConfig>,
  resources: Resource[],
): Override[] =>
  buildProjectedOverrides({
    rawConfig,
    nodeResources: [],
    vmResources: [],
    containerResources: [],
    storageResources: [],
    agentResourceList: resources,
    containerRuntimeResources: resources,
    getChildren: () => [],
    pbsInstanceById: new Map(),
    allResources: resources,
  });

describe('TrueNAS threshold persistence identity', () => {
  it('re-homes a connection-target override onto the canonical ID and survives reload/refetch', () => {
    const [resource, setResource] = createSignal(
      connectionBackedTrueNASResource({ machineId: 'serial-visible-on-first-poll' }),
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
      projectOverrides(initialRawConfig, [resource()]),
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
    setResource(connectionBackedTrueNASResource({ machineId: 'serial-missing-on-next-poll' }));
    setOverrides(projectOverrides(persisted, [resource()]));

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

  it('keeps the edited connection stable through a websocket repoll/reorder and the blur save handler', () => {
    const firstPoll = connectionBackedTrueNASResource({
      machineId: 'shared-dr-serial',
      memory: 87,
    });
    const secondSystem = connectionBackedTrueNASResource({
      canonicalId: secondCanonicalID,
      legacyId: secondLegacyCanonicalID,
      connectionId: secondConnectionID,
      machineId: 'shared-dr-serial',
      memory: 68,
    });
    const [resources, setResources] = createSignal<Resource[]>([firstPoll, secondSystem]);
    const [rawOverridesConfig, setRawOverridesConfig] = createSignal<
      Record<string, RawOverrideConfig>
    >({});
    const [overrides, setOverrides] = createSignal<Override[]>([]);
    const [editingId, setEditingId] = createSignal<string | null>(null);
    const [editingThresholds, setEditingThresholds] = createSignal<
      Record<string, number | undefined>
    >({});
    const [editingNote, setEditingNote] = createSignal('');
    const [bulkEditIds] = createSignal<string[]>([]);
    const setHasUnsavedChanges = vi.fn();
    const cancelEdit = () => {
      setEditingId(null);
      setEditingThresholds({});
      setEditingNote('');
    };

    const props = {
      get allResources() {
        return resources();
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

    expect(platform.result.trueNASSystemsWithOverrides().map((resource) => resource.id)).toEqual([
      canonicalID,
      secondCanonicalID,
    ]);

    // AlertResourceTableRow wires number-input blur directly to saveEdit. Seed
    // the exact table-state payload that callback receives so this regression
    // can focus on identity stability while the shared row blur behavior stays
    // covered by ResourceTable.test.tsx.
    setEditingId(canonicalID);
    setEditingThresholds({ memory: 95 });
    expect(editingId()).toBe(canonicalID);
    expect(editingThresholds().memory).toBe(95);

    // Model a WebSocket state refresh while the input is active: resource
    // order changes and the reported DMI serial disappears. Both systems keep
    // the same display hostname, so only the configured connection can own
    // the edit.
    const repolledFirst = connectionBackedTrueNASResource({
      machineId: '',
      memory: 88,
    });
    setResources([secondSystem, repolledFirst]);
    setOverrides(projectOverrides(rawOverridesConfig(), resources()));

    expect(platform.result.trueNASSystemsWithOverrides().map((resource) => resource.id)).toEqual([
      canonicalID,
      secondCanonicalID,
    ]);
    expect(editingThresholds().memory).toBe(95);

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
    expect(rawOverridesConfig()).not.toHaveProperty(legacyCanonicalID);
    expect(rawOverridesConfig()).not.toHaveProperty(secondCanonicalID);
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
    expect(editingId()).toBeNull();

    const refreshedRows = platform.result.trueNASSystemsWithOverrides();
    expect(refreshedRows.map((resource) => resource.id)).toEqual([secondCanonicalID, canonicalID]);
    expect(refreshedRows.find((resource) => resource.id === canonicalID)).toEqual(
      expect.objectContaining({
        hasOverride: true,
        thresholds: {
          memory: 95,
        },
      }),
    );
  });
});
