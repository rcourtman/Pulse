import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';

import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type { Resource as TableResource } from '@/features/alerts/thresholds/tableTypes';

import { useThresholdsOverrideMutations } from '../useThresholdsOverrideMutations';

const buildTableProps = (overridesSignal: ReturnType<typeof createSignal<any[]>>) => {
  const [overrides, setOverrides] = overridesSignal;
  const [rawOverridesConfig, setRawOverridesConfig] = createSignal<Record<string, any>>({});
  const [backupDefaults] = createSignal({
    enabled: false,
    warningDays: 7,
    criticalDays: 14,
  });
  const [snapshotDefaults] = createSignal({
    enabled: false,
    warningDays: 30,
    criticalDays: 45,
    warningSizeGiB: 0,
    criticalSizeGiB: 0,
  });
  const setHasUnsavedChanges = vi.fn();
  const removeAlerts = vi.fn();

  const props = {
    overrides,
    setOverrides,
    rawOverridesConfig,
    setRawOverridesConfig,
    backupDefaults,
    snapshotDefaults,
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

describe('useThresholdsOverrideMutations', () => {
  it('owns threshold override save persistence outside the table-state shell', () => {
    const overrideSignal = createSignal<any[]>([]);
    const { props, rawOverridesConfig, setHasUnsavedChanges } = buildTableProps(overrideSignal);
    const [editingThresholds] = createSignal<Record<string, number | undefined>>({ cpu: 95 });
    const [editingNote] = createSignal('Investigate host pressure');
    const [bulkEditIds] = createSignal<string[]>([]);
    const cancelEdit = vi.fn();
    const guestResource: TableResource = {
      id: 'vm-100',
      name: 'db-01',
      type: 'guest',
      resourceType: 'VM',
      vmid: 100,
      node: 'pve-1',
      instance: 'qemu/100',
      defaults: { cpu: 80 },
      thresholds: { cpu: 80 },
    };

    const { result } = renderHook(() =>
      useThresholdsOverrideMutations({
        props,
        resources: {
          nodesWithOverrides: () => [],
          agentsWithOverrides: () => [],
          agentDisksWithOverrides: () => [],
          dockerHostsWithOverrides: () => [],
          guestsFlat: () => [guestResource],
          dockerContainersFlat: () => [],
          pbsServersWithOverrides: () => [],
          pmgServersWithOverrides: () => [],
          storageWithOverrides: () => [],
        },
        editingThresholds,
        editingNote,
        bulkEditIds,
        cancelEdit,
        updateBackupDefaults: vi.fn(),
        updateSnapshotDefaults: vi.fn(),
      }),
    );

    result.saveEdit('vm-100');

    expect(overrideSignal[0]()).toEqual([
      expect.objectContaining({
        id: 'vm-100',
        type: 'guest',
        note: 'Investigate host pressure',
        thresholds: { cpu: 95 },
      }),
    ]);
    expect(rawOverridesConfig()).toEqual({
      'vm-100': {
        cpu: {
          clear: 90,
          trigger: 95,
        },
        note: 'Investigate host pressure',
      },
    });
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
    expect(cancelEdit).toHaveBeenCalledTimes(1);
  });

  it('owns powered-off severity and disable-connectivity persistence for guest resources', () => {
    const overrideSignal = createSignal<any[]>([]);
    const { props, rawOverridesConfig, setHasUnsavedChanges, removeAlerts } =
      buildTableProps(overrideSignal);
    const [editingThresholds] = createSignal<Record<string, number | undefined>>({});
    const [editingNote] = createSignal('');
    const [bulkEditIds] = createSignal<string[]>([]);
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
      useThresholdsOverrideMutations({
        props,
        resources: {
          nodesWithOverrides: () => [],
          agentsWithOverrides: () => [],
          agentDisksWithOverrides: () => [],
          dockerHostsWithOverrides: () => [],
          guestsFlat: () => [guestResource],
          dockerContainersFlat: () => [],
          pbsServersWithOverrides: () => [],
          pmgServersWithOverrides: () => [],
          storageWithOverrides: () => [],
        },
        editingThresholds,
        editingNote,
        bulkEditIds,
        cancelEdit: vi.fn(),
        updateBackupDefaults: vi.fn(),
        updateSnapshotDefaults: vi.fn(),
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
