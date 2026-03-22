import type { Accessor } from 'solid-js';

import type {
  RawOverrideConfig,
  BackupAlertConfig,
  SnapshotAlertConfig,
} from '@/types/alerts';
import { matchesAlertIdentifier } from '@/features/alerts/identity';
import {
  DEFAULT_SNAPSHOT_WARNING,
  DEFAULT_SNAPSHOT_CRITICAL,
  DEFAULT_SNAPSHOT_WARNING_SIZE,
  DEFAULT_SNAPSHOT_CRITICAL_SIZE,
  DEFAULT_BACKUP_WARNING,
  DEFAULT_BACKUP_CRITICAL,
} from '@/features/alerts/thresholds/constants';
import type {
  Override,
  OverrideType,
  OfflineState,
  ThresholdsTableProps,
} from '@/features/alerts/thresholds/types';
import type { Resource as TableResource } from '@/features/alerts/thresholds/tableTypes';

interface ThresholdsOverrideMutationResources {
  nodesWithOverrides: Accessor<TableResource[]>;
  agentsWithOverrides: Accessor<TableResource[]>;
  agentDisksWithOverrides: Accessor<TableResource[]>;
  dockerHostsWithOverrides: Accessor<TableResource[]>;
  guestsFlat: Accessor<TableResource[]>;
  dockerContainersFlat: Accessor<TableResource[]>;
  pbsServersWithOverrides: Accessor<TableResource[]>;
  pmgServersWithOverrides: Accessor<TableResource[]>;
  storageWithOverrides: Accessor<TableResource[]>;
}

interface ThresholdsOverrideMutationProps {
  props: ThresholdsTableProps;
  resources: ThresholdsOverrideMutationResources;
  editingThresholds: Accessor<Record<string, number | undefined>>;
  editingNote: Accessor<string>;
  bulkEditIds: Accessor<string[]>;
  cancelEdit: () => void;
  updateBackupDefaults: (
    updater: BackupAlertConfig | ((prev: BackupAlertConfig) => BackupAlertConfig),
  ) => void;
  updateSnapshotDefaults: (
    updater: SnapshotAlertConfig | ((prev: SnapshotAlertConfig) => SnapshotAlertConfig),
  ) => void;
}

const upsertOverride = (overrides: Override[], override: Override): Override[] => {
  const existingIndex = overrides.findIndex((entry) => entry.id === override.id);
  if (existingIndex >= 0) {
    const nextOverrides = [...overrides];
    nextOverrides[existingIndex] = override;
    return nextOverrides;
  }

  return [...overrides, override];
};

const withThresholdEntries = (
  rawConfig: RawOverrideConfig,
  thresholds: Record<string, number | undefined>,
): RawOverrideConfig => {
  const next = { ...rawConfig };

  Object.entries(thresholds).forEach(([metric, value]) => {
    if (value !== undefined && value !== null) {
      next[metric] = {
        clear: Math.max(0, value - 5),
        trigger: value,
      };
    }
  });

  return next;
};

const stripStateKeys = (thresholds: Record<string, number>): Record<string, number> => {
  const next = { ...thresholds };
  delete (next as Record<string, unknown>).disabled;
  delete (next as Record<string, unknown>).disableConnectivity;
  delete (next as Record<string, unknown>).poweredOffSeverity;
  return next;
};

export function useThresholdsOverrideMutations({
  props,
  resources,
  editingThresholds,
  editingNote,
  bulkEditIds,
  cancelEdit,
  updateBackupDefaults,
  updateSnapshotDefaults,
}: ThresholdsOverrideMutationProps) {
  const proxmoxResources = () => [
    ...resources.nodesWithOverrides(),
    ...resources.agentsWithOverrides(),
    ...resources.agentDisksWithOverrides(),
    ...resources.dockerHostsWithOverrides(),
    ...resources.pbsServersWithOverrides(),
    ...resources.pmgServersWithOverrides(),
    ...resources.storageWithOverrides(),
  ];

  const guestLikeResources = () => [
    ...resources.guestsFlat(),
    ...resources.dockerContainersFlat(),
  ];

  const allThresholdResources = () => [...proxmoxResources(), ...guestLikeResources()];

  const removeOverride = (resourceId: string) => {
    props.setOverrides(props.overrides().filter((override) => override.id !== resourceId));
    const newRawConfig = { ...props.rawOverridesConfig() };
    delete newRawConfig[resourceId];
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);
  };

  const saveEdit = (resourceId: string) => {
    const resource = allThresholdResources().find((entry) => entry.id === resourceId);
    if (!resource) return;

    const editedThresholdMap = editingThresholds();
    const trimmedNote = editingNote().trim();
    const noteForOverride = trimmedNote.length > 0 ? trimmedNote : undefined;

    if (resource.editScope === 'backup') {
      const currentBackupDefaults = props.backupDefaults();
      updateBackupDefaults({
        criticalDays:
          editedThresholdMap['critical days'] ??
          currentBackupDefaults.criticalDays ??
          DEFAULT_BACKUP_CRITICAL,
        enabled: currentBackupDefaults.enabled,
        warningDays:
          editedThresholdMap['warning days'] ??
          currentBackupDefaults.warningDays ??
          DEFAULT_BACKUP_WARNING,
      });
      cancelEdit();
      return;
    }

    if (resource.editScope === 'snapshot') {
      const currentSnapshotDefaults = props.snapshotDefaults();
      updateSnapshotDefaults({
        criticalDays:
          editedThresholdMap['critical days'] ??
          currentSnapshotDefaults.criticalDays ??
          DEFAULT_SNAPSHOT_CRITICAL,
        criticalSizeGiB:
          editedThresholdMap['critical size (gib)'] ??
          currentSnapshotDefaults.criticalSizeGiB ??
          DEFAULT_SNAPSHOT_CRITICAL_SIZE,
        enabled: currentSnapshotDefaults.enabled,
        warningDays:
          editedThresholdMap['warning days'] ??
          currentSnapshotDefaults.warningDays ??
          DEFAULT_SNAPSHOT_WARNING,
        warningSizeGiB:
          editedThresholdMap['warning size (gib)'] ??
          currentSnapshotDefaults.warningSizeGiB ??
          DEFAULT_SNAPSHOT_WARNING_SIZE,
      });
      cancelEdit();
      return;
    }

    const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;
    const overrideThresholds: Record<string, number> = {};

    Object.keys(editedThresholdMap).forEach((key) => {
      const editedValue = editedThresholdMap[key];
      const defaultValue = defaultThresholds[key];
      if (editedValue !== undefined && editedValue !== defaultValue) {
        overrideThresholds[key] = editedValue;
      }
    });

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const hasStateOnlyOverride = Boolean(
      resource.disabled ||
        resource.disableConnectivity ||
        resource.poweredOffSeverity !== undefined ||
        noteForOverride !== undefined ||
        existingOverride?.backup ||
        existingOverride?.snapshot,
    );

    if (Object.keys(overrideThresholds).length === 0 && !hasStateOnlyOverride) {
      if (resource.hasOverride) {
        removeOverride(resourceId);
      }
      cancelEdit();
      return;
    }

    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type as OverrideType,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: resource.disabled,
      disableConnectivity: resource.disableConnectivity,
      poweredOffSeverity: resource.poweredOffSeverity,
      note: noteForOverride,
      backup: existingOverride?.backup,
      snapshot: existingOverride?.snapshot,
      thresholds: overrideThresholds,
    };

    props.setOverrides(upsertOverride(props.overrides(), override));

    const previousRaw = props.rawOverridesConfig()[resourceId];
    let hysteresisThresholds: RawOverrideConfig = {};

    if (previousRaw) {
      if (previousRaw.disabled !== undefined) hysteresisThresholds.disabled = previousRaw.disabled;
      if (previousRaw.disableConnectivity !== undefined) {
        hysteresisThresholds.disableConnectivity = previousRaw.disableConnectivity;
      }
      if (previousRaw.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = previousRaw.poweredOffSeverity;
      }
      if (previousRaw.backup) hysteresisThresholds.backup = previousRaw.backup;
      if (previousRaw.snapshot) hysteresisThresholds.snapshot = previousRaw.snapshot;
    }

    hysteresisThresholds = withThresholdEntries(hysteresisThresholds, overrideThresholds);

    if (resource.disabled) {
      hysteresisThresholds.disabled = true;
    } else {
      delete hysteresisThresholds.disabled;
    }

    if (resource.disableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
      delete hysteresisThresholds.poweredOffSeverity;
    } else {
      if (
        (resource.type === 'guest' || resource.type === 'dockerContainer') &&
        props.guestDisableConnectivity()
      ) {
        hysteresisThresholds.disableConnectivity = false;
      } else {
        delete hysteresisThresholds.disableConnectivity;
      }
      if (resource.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = resource.poweredOffSeverity;
      } else {
        delete hysteresisThresholds.poweredOffSeverity;
      }
    }

    if (noteForOverride) {
      hysteresisThresholds.note = noteForOverride;
    } else {
      delete hysteresisThresholds.note;
    }

    props.setRawOverridesConfig({
      ...props.rawOverridesConfig(),
      [resourceId]: hysteresisThresholds,
    });
    props.setHasUnsavedChanges(true);
    cancelEdit();
  };

  const handleSaveBulkEdit = (thresholds: Record<string, number | undefined>) => {
    const nextOverrides = [...props.overrides()];
    const nextRawConfig = { ...props.rawOverridesConfig() };

    for (const id of bulkEditIds()) {
      const resource = proxmoxResources().find((entry) => entry.id === id);
      if (!resource) continue;

      const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;
      const existingOverride = nextOverrides.find((override) => override.id === id);
      const previousRaw = nextRawConfig[id];
      const newThresholds: Record<string, number | undefined> = {
        ...(existingOverride?.thresholds ?? {}),
      };

      Object.keys(thresholds).forEach((key) => {
        if (thresholds[key] !== undefined) {
          const value = thresholds[key];
          if (value === defaultThresholds[key]) {
            delete newThresholds[key];
          } else {
            newThresholds[key] = value as number;
          }
        }
      });

      const hasStateOnlyOverride = Boolean(
        resource.disabled ||
          resource.disableConnectivity ||
          resource.poweredOffSeverity !== undefined ||
          existingOverride?.note !== undefined ||
          existingOverride?.backup ||
          existingOverride?.snapshot,
      );

      if (Object.keys(newThresholds).length === 0 && !hasStateOnlyOverride) {
        const existingIndex = nextOverrides.findIndex((override) => override.id === id);
        if (existingIndex !== -1) nextOverrides.splice(existingIndex, 1);
        delete nextRawConfig[id];
        continue;
      }

      const override: Override = {
        id,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? resource.instance : undefined,
        disabled: resource.disabled,
        disableConnectivity: resource.disableConnectivity,
        poweredOffSeverity: resource.poweredOffSeverity,
        note: existingOverride?.note,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: newThresholds,
      };

      const existingIndex = nextOverrides.findIndex((entry) => entry.id === id);
      if (existingIndex >= 0) {
        nextOverrides[existingIndex] = override;
      } else {
        nextOverrides.push(override);
      }

      let hysteresisThresholds: RawOverrideConfig = {};
      if (previousRaw) {
        if (previousRaw.disabled !== undefined) hysteresisThresholds.disabled = previousRaw.disabled;
        if (previousRaw.disableConnectivity !== undefined) {
          hysteresisThresholds.disableConnectivity = previousRaw.disableConnectivity;
        }
        if (previousRaw.poweredOffSeverity !== undefined) {
          hysteresisThresholds.poweredOffSeverity = previousRaw.poweredOffSeverity;
        }
        if (previousRaw.note !== undefined) hysteresisThresholds.note = previousRaw.note;
        if (previousRaw.backup !== undefined) hysteresisThresholds.backup = previousRaw.backup;
        if (previousRaw.snapshot !== undefined) {
          hysteresisThresholds.snapshot = previousRaw.snapshot;
        }
      }

      nextRawConfig[id] = withThresholdEntries(hysteresisThresholds, newThresholds);
    }

    props.setOverrides(nextOverrides);
    props.setRawOverridesConfig(nextRawConfig);
    props.setHasUnsavedChanges(true);
  };

  const toggleBackup = (resourceId: string, forceState?: boolean) => {
    const resource = guestLikeResources().find((entry) => entry.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const baseConfig = existingOverride?.backup || props.backupDefaults();
    const newBackup = {
      ...baseConfig,
      enabled: forceState !== undefined ? forceState : !baseConfig.enabled,
    };

    const override: Override = {
      ...(existingOverride || {
        id: resourceId,
        instance: 'instance' in resource ? resource.instance : undefined,
        name: resource.name,
        node: 'node' in resource ? resource.node : undefined,
        thresholds: {},
        type: resource.type as OverrideType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
      }),
      backup: newBackup,
    };

    props.setOverrides(upsertOverride(props.overrides(), override));
    props.setRawOverridesConfig({
      ...props.rawOverridesConfig(),
      [resourceId]: {
        ...(props.rawOverridesConfig()[resourceId] || {}),
        backup: newBackup,
      },
    });
    props.setHasUnsavedChanges(true);
  };

  const toggleSnapshot = (resourceId: string, forceState?: boolean) => {
    const resource = guestLikeResources().find((entry) => entry.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const baseConfig = existingOverride?.snapshot || props.snapshotDefaults();
    const newSnapshot = {
      ...baseConfig,
      enabled: forceState !== undefined ? forceState : !baseConfig.enabled,
    };

    const override: Override = {
      ...(existingOverride || {
        id: resourceId,
        instance: 'instance' in resource ? resource.instance : undefined,
        name: resource.name,
        node: 'node' in resource ? resource.node : undefined,
        thresholds: {},
        type: resource.type as OverrideType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
      }),
      snapshot: newSnapshot,
    };

    props.setOverrides(upsertOverride(props.overrides(), override));
    props.setRawOverridesConfig({
      ...props.rawOverridesConfig(),
      [resourceId]: {
        ...(props.rawOverridesConfig()[resourceId] || {}),
        snapshot: newSnapshot,
      },
    });
    props.setHasUnsavedChanges(true);
  };

  const toggleDisabled = (resourceId: string, forceState?: boolean) => {
    const resource = [
      ...guestLikeResources(),
      ...resources.storageWithOverrides(),
      ...resources.pbsServersWithOverrides(),
      ...resources.agentsWithOverrides(),
      ...resources.agentDisksWithOverrides(),
    ].find((entry) => entry.id === resourceId);

    if (
      !resource ||
      (resource.type !== 'guest' &&
        resource.type !== 'storage' &&
        resource.type !== 'pbs' &&
        resource.type !== 'dockerContainer' &&
        resource.type !== 'agent' &&
        resource.type !== 'agentDisk')
    ) {
      return;
    }

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const currentDisabledState = resource.disabled;
    const newDisabledState = forceState !== undefined ? forceState : !currentDisabledState;
    const cleanThresholds = stripStateKeys({ ...(existingOverride?.thresholds || {}) });

    if (!newDisabledState && (!existingOverride || Object.keys(cleanThresholds).length === 0)) {
      removeOverride(resourceId);
    } else {
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? resource.instance : undefined,
        disabled: newDisabledState,
        disableConnectivity: existingOverride?.disableConnectivity,
        poweredOffSeverity: existingOverride?.poweredOffSeverity,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: cleanThresholds,
      };

      props.setOverrides(upsertOverride(props.overrides(), override));

      let hysteresisThresholds: RawOverrideConfig = withThresholdEntries({}, override.thresholds);
      if (newDisabledState) hysteresisThresholds.disabled = true;
      if (override.backup) hysteresisThresholds.backup = override.backup;
      if (override.snapshot) hysteresisThresholds.snapshot = override.snapshot;
      if (override.disableConnectivity) hysteresisThresholds.disableConnectivity = true;
      if (override.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = override.poweredOffSeverity;
      }

      const nextRawConfig = { ...props.rawOverridesConfig() };
      if (Object.keys(hysteresisThresholds).length === 0) {
        delete nextRawConfig[resourceId];
      } else {
        nextRawConfig[resourceId] = hysteresisThresholds;
      }
      props.setRawOverridesConfig(nextRawConfig);
    }

    if (newDisabledState && props.removeAlerts) {
      if (resource.type === 'guest') {
        props.removeAlerts(
          (alert) => alert.resourceId === resourceId && alert.type === 'powered-off',
        );
      } else if (resource.type === 'pbs') {
        const offlineId = `pbs-offline-${resourceId}`;
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (matchesAlertIdentifier(alert, offlineId) || alert.type === 'offline'),
        );
      } else if (resource.type === 'dockerContainer') {
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.type === 'docker-container-state' || alert.type === 'docker-container-health'),
        );
      }
    }

    props.setHasUnsavedChanges(true);
  };

  const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {
    const resource = [
      ...resources.nodesWithOverrides(),
      ...resources.pbsServersWithOverrides(),
      ...resources.guestsFlat(),
      ...resources.agentsWithOverrides(),
      ...resources.dockerHostsWithOverrides(),
    ].find((entry) => entry.id === resourceId);

    if (
      !resource ||
      (resource.type !== 'agent' &&
        resource.type !== 'pbs' &&
        resource.type !== 'guest' &&
        resource.type !== 'dockerHost')
    ) {
      return;
    }

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const currentDisableConnectivity = resource.disableConnectivity;
    const newDisableConnectivity =
      forceState !== undefined ? forceState : !currentDisableConnectivity;
    const cleanThresholds = stripStateKeys({ ...(existingOverride?.thresholds || {}) });

    if (!newDisableConnectivity && Object.keys(cleanThresholds).length === 0) {
      removeOverride(resourceId);
    } else {
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        disableConnectivity: newDisableConnectivity,
        disabled: existingOverride?.disabled,
        poweredOffSeverity: existingOverride?.poweredOffSeverity,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: cleanThresholds,
      };

      props.setOverrides(upsertOverride(props.overrides(), override));

      let hysteresisThresholds: RawOverrideConfig = withThresholdEntries({}, cleanThresholds);
      if (newDisableConnectivity) {
        hysteresisThresholds.disableConnectivity = true;
      }
      if (override.backup) hysteresisThresholds.backup = override.backup;
      if (override.snapshot) hysteresisThresholds.snapshot = override.snapshot;
      if (override.disabled) hysteresisThresholds.disabled = true;
      if (override.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = override.poweredOffSeverity;
      }

      const nextRawConfig = { ...props.rawOverridesConfig() };
      if (Object.keys(hysteresisThresholds).length === 0) {
        delete nextRawConfig[resourceId];
      } else {
        nextRawConfig[resourceId] = hysteresisThresholds;
      }
      props.setRawOverridesConfig(nextRawConfig);
    }

    props.setHasUnsavedChanges(true);

    if (props.removeAlerts && resource.type === 'dockerHost') {
      const offlineId = `docker-host-offline-${resourceId}`;
      const resourceKey = `docker:${resourceId}`;
      props.removeAlerts(
        (alert) => matchesAlertIdentifier(alert, offlineId) || alert.resourceId === resourceKey,
      );
    }
  };

  const setOfflineState = (resourceId: string, state: OfflineState) => {
    const resource = guestLikeResources().find((entry) => entry.id === resourceId);
    if (!resource) return;

    const isDockerContainer = resource.type === 'dockerContainer';
    const defaultDisabled = isDockerContainer
      ? props.dockerDisableConnectivity()
      : props.guestDisableConnectivity();
    const defaultSeverity = isDockerContainer
      ? props.dockerPoweredOffSeverity()
      : props.guestPoweredOffSeverity();

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const cleanThresholds = stripStateKeys({ ...(existingOverride?.thresholds || {}) });

    const newDisableConnectivity = state === 'off';
    const newSeverity: 'warning' | 'critical' | undefined =
      state === 'off' ? undefined : state === 'critical' ? 'critical' : 'warning';

    const overrideDisabled = existingOverride?.disabled || false;
    const hasThresholds = Object.keys(cleanThresholds).length > 0;
    const differsFromDefaults =
      newDisableConnectivity !== defaultDisabled ||
      (!newDisableConnectivity && newSeverity !== defaultSeverity);

    if (
      !differsFromDefaults &&
      !hasThresholds &&
      !overrideDisabled &&
      !existingOverride?.disableConnectivity
    ) {
      if (existingOverride) {
        removeOverride(resourceId);
      }
      return;
    }

    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type as OverrideType,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: overrideDisabled,
      disableConnectivity: newDisableConnectivity,
      poweredOffSeverity: newDisableConnectivity ? undefined : newSeverity,
      backup: existingOverride?.backup,
      snapshot: existingOverride?.snapshot,
      thresholds: cleanThresholds,
    };

    props.setOverrides(upsertOverride(props.overrides(), override));

    let hysteresisThresholds: RawOverrideConfig = withThresholdEntries({}, cleanThresholds);
    if (overrideDisabled) hysteresisThresholds.disabled = true;
    if (newDisableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
    } else {
      if (defaultDisabled) hysteresisThresholds.disableConnectivity = false;
      if (newSeverity) hysteresisThresholds.poweredOffSeverity = newSeverity;
    }
    if (override.backup) hysteresisThresholds.backup = override.backup;
    if (override.snapshot) hysteresisThresholds.snapshot = override.snapshot;

    const nextRawConfig = { ...props.rawOverridesConfig() };
    if (Object.keys(hysteresisThresholds).length > 0) {
      nextRawConfig[resourceId] = hysteresisThresholds;
    } else {
      delete nextRawConfig[resourceId];
    }

    props.setRawOverridesConfig(nextRawConfig);
    props.setHasUnsavedChanges(true);

    if (props.removeAlerts && newDisableConnectivity) {
      if (resource.type === 'guest') {
        props.removeAlerts(
          (alert) => alert.resourceId === resourceId && alert.type === 'powered-off',
        );
      } else if (resource.type === 'dockerContainer') {
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.type === 'docker-container-state' || alert.type === 'docker-container-health'),
        );
      }
    }
  };

  return {
    handleSaveBulkEdit,
    removeOverride,
    saveEdit,
    setOfflineState,
    toggleBackup,
    toggleDisabled,
    toggleNodeConnectivity,
    toggleSnapshot,
  } as const;
}
