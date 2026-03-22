import { createMemo } from 'solid-js';

import type { PMGThresholdDefaults } from '@/types/alerts';
import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';

import { PMG_KEY_TO_NORMALIZED, PMG_NORMALIZED_TO_KEY, PMG_THRESHOLD_COLUMNS } from '../constants';
import type { Resource as TableResource } from '../tableTypes';
import { ThresholdsDataInputs } from '../thresholdsResourceModel';
import {
  createOverridesMap,
  hasThresholdDiff,
  normalizeStorageStatus,
  storageCoords,
} from '../thresholdsResourceModel';

export function useThresholdsInfrastructureData(inputs: ThresholdsDataInputs) {
  const { props, editingId, searchTerm } = inputs;

  const pbsServersWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    const pbsServers = (props.pbsInstances || []).map((pbs) => {
      const override = overridesMap.get(pbs.id);
      const hasCustomThresholds = hasThresholdDiff(override, {
        cpu: props.pbsDefaults?.cpu ?? 80,
        memory: props.pbsDefaults?.memory ?? 85,
      });
      const disableConnectivity = override?.disableConnectivity || false;

      return {
        id: pbs.id,
        name: pbs.name,
        type: 'pbs' as const,
        resourceType: 'PBS',
        host: pbs.host,
        status: pbs.status,
        cpu: pbs.cpu,
        memory: pbs.memory,
        memoryUsed: pbs.memoryUsed,
        memoryTotal: pbs.memoryTotal,
        uptime: pbs.uptime,
        hasOverride: hasCustomThresholds || disableConnectivity,
        disabled: false,
        disableConnectivity,
        thresholds: override?.thresholds || {},
        defaults: {
          cpu: props.pbsDefaults?.cpu ?? 80,
          memory: props.pbsDefaults?.memory ?? 85,
        },
      };
    });

    return search
      ? pbsServers.filter(
          (pbs) => pbs.name.toLowerCase().includes(search) || pbs.host?.toLowerCase().includes(search),
        )
      : pbsServers;
  }, []);

  const pmgGlobalDefaults = createMemo<Record<string, number>>(() => {
    const defaults = props.pmgThresholds();
    const record: Record<string, number> = {};
    PMG_THRESHOLD_COLUMNS.forEach(({ key, normalized }) => {
      const value = defaults[key as keyof PMGThresholdDefaults];
      record[normalized] = typeof value === 'number' && Number.isFinite(value) ? value : 0;
    });
    return record;
  });

  const pmgServersWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    const defaultThresholds = pmgGlobalDefaults();

    const pmgServers = (props.pmgInstances || []).map((pmg) => {
      const override = overridesMap.get(pmg.id);
      const thresholdOverrides: Record<string, number> = {};
      const overrideThresholds = (override?.thresholds ?? {}) as Record<string, unknown>;

      Object.entries(overrideThresholds).forEach(([rawKey, rawValue]) => {
        if (typeof rawValue !== 'number' || Number.isNaN(rawValue)) return;
        const normalizedKey =
          PMG_KEY_TO_NORMALIZED.get(rawKey as keyof PMGThresholdDefaults) ||
          (PMG_NORMALIZED_TO_KEY.has(rawKey) ? rawKey : undefined);
        if (!normalizedKey) return;
        thresholdOverrides[normalizedKey] = rawValue;
      });

      const hasOverride =
        Boolean(override?.disableConnectivity) ||
        Boolean(override?.disabled) ||
        Object.keys(thresholdOverrides).length > 0;

      return {
        id: pmg.id,
        name: pmg.name,
        type: 'pmg' as const,
        resourceType: 'PMG',
        host: pmg.host,
        status: pmg.status,
        hasOverride,
        disabled: override?.disabled || false,
        disableConnectivity: override?.disableConnectivity || false,
        thresholds: thresholdOverrides,
        defaults: { ...defaultThresholds },
      };
    });

    return search
      ? pmgServers.filter(
          (pmg) => pmg.name.toLowerCase().includes(search) || pmg.host?.toLowerCase().includes(search),
        )
      : pmgServers;
  }, []);

  const storageWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) {
      return prev;
    }

    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());

    const storageDevices = (props.storage ?? []).map((storage) => {
      const override = overridesMap.get(storage.id);
      const coords = storageCoords(storage);
      const hasCustomThresholds = hasThresholdDiff(override, { usage: props.storageDefault() });

      return {
        id: storage.id,
        name: getAlertResourceDisplayLabel(storage),
        displayName: getAlertResourceDisplayLabel(storage),
        rawName: storage.name,
        type: 'storage' as const,
        resourceType: 'Storage',
        node: coords.node,
        instance: coords.instance,
        status: normalizeStorageStatus(storage.status),
        hasOverride: hasCustomThresholds || Boolean(override?.disabled),
        disabled: override?.disabled || false,
        thresholds: override?.thresholds || {},
        defaults: { usage: props.storageDefault() },
      };
    });

    return search
      ? storageDevices.filter(
          (storage) =>
            storage.name.toLowerCase().includes(search) || storage.node?.toLowerCase().includes(search),
        )
      : storageDevices;
  }, []);

  const storageGroupedByNode = createMemo<Record<string, TableResource[]>>(() => {
    const grouped: Record<string, TableResource[]> = {};
    storageWithOverrides().forEach((storage) => {
      const key = storage.node?.trim() || 'Unassigned';
      if (!grouped[key]) {
        grouped[key] = [];
      }
      grouped[key].push(storage);
    });

    Object.values(grouped).forEach((resources) => {
      resources.sort((a, b) => a.name.localeCompare(b.name));
    });

    return grouped;
  });

  return {
    pbsServersWithOverrides,
    pmgGlobalDefaults,
    pmgServersWithOverrides,
    storageWithOverrides,
    storageGroupedByNode,
  };
}
