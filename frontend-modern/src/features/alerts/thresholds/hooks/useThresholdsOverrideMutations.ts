import type { Accessor } from 'solid-js';

import type { RawOverrideConfig, BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';
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
  ThresholdsTableProps,
} from '@/features/alerts/thresholds/types';
import type { Resource as TableResource } from '@/features/alerts/thresholds/tableTypes';
import {
  findOverrideForResource,
  findRawOverrideConfigForResource,
  getOverridePersistenceIdentity,
  stripOverrideCandidates,
  stripRawOverrideCandidates,
} from '@/features/alerts/thresholds/guestThresholdOverrideMutationModel';
import {
  upsertOverride,
  withThresholdEntries,
} from '@/features/alerts/thresholds/thresholdsOverrideMutationModel';

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

  const guestLikeResources = () => [...resources.guestsFlat(), ...resources.dockerContainersFlat()];

  const allThresholdResources = () => [...proxmoxResources(), ...guestLikeResources()];
  const findThresholdResource = (resourceId: string) =>
    allThresholdResources().find((entry) => entry.id === resourceId);

  const removeOverride = (resourceId: string) => {
    const resource = findThresholdResource(resourceId);
    props.setOverrides(stripOverrideCandidates(props.overrides(), resource));
    props.setRawOverridesConfig(stripRawOverrideCandidates(props.rawOverridesConfig(), resource));
    props.setHasUnsavedChanges(true);
  };

  const saveEdit = (resourceId: string) => {
    const resource = findThresholdResource(resourceId);
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

    const { storageId } = getOverridePersistenceIdentity(resource);
    const existingOverride = findOverrideForResource(props.overrides(), resource);
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
      id: storageId,
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

    props.setOverrides(
      upsertOverride(stripOverrideCandidates(props.overrides(), resource), override),
    );

    const previousRaw = findRawOverrideConfigForResource(props.rawOverridesConfig(), resource);
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

    const nextRawConfig = stripRawOverrideCandidates(props.rawOverridesConfig(), resource);
    nextRawConfig[storageId] = hysteresisThresholds;

    props.setRawOverridesConfig(nextRawConfig);
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
        if (previousRaw.disabled !== undefined)
          hysteresisThresholds.disabled = previousRaw.disabled;
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

    const { storageId } = getOverridePersistenceIdentity(resource);
    const existingOverride = findOverrideForResource(props.overrides(), resource);
    const previousRaw = findRawOverrideConfigForResource(props.rawOverridesConfig(), resource);
    const baseConfig = existingOverride?.backup || props.backupDefaults();
    const newBackup = {
      ...baseConfig,
      enabled: forceState !== undefined ? forceState : !baseConfig.enabled,
    };

    const override: Override = {
      ...(existingOverride || {
        id: storageId,
        instance: 'instance' in resource ? resource.instance : undefined,
        name: resource.name,
        node: 'node' in resource ? resource.node : undefined,
        thresholds: {},
        type: resource.type as OverrideType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
      }),
      id: storageId,
      backup: newBackup,
    };

    props.setOverrides(
      upsertOverride(stripOverrideCandidates(props.overrides(), resource), override),
    );
    const nextRawConfig = stripRawOverrideCandidates(props.rawOverridesConfig(), resource);
    nextRawConfig[storageId] = {
      ...(previousRaw || {}),
      backup: newBackup,
    };
    props.setRawOverridesConfig(nextRawConfig);
    props.setHasUnsavedChanges(true);
  };

  const toggleSnapshot = (resourceId: string, forceState?: boolean) => {
    const resource = guestLikeResources().find((entry) => entry.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const { storageId } = getOverridePersistenceIdentity(resource);
    const existingOverride = findOverrideForResource(props.overrides(), resource);
    const previousRaw = findRawOverrideConfigForResource(props.rawOverridesConfig(), resource);
    const baseConfig = existingOverride?.snapshot || props.snapshotDefaults();
    const newSnapshot = {
      ...baseConfig,
      enabled: forceState !== undefined ? forceState : !baseConfig.enabled,
    };

    const override: Override = {
      ...(existingOverride || {
        id: storageId,
        instance: 'instance' in resource ? resource.instance : undefined,
        name: resource.name,
        node: 'node' in resource ? resource.node : undefined,
        thresholds: {},
        type: resource.type as OverrideType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
      }),
      id: storageId,
      snapshot: newSnapshot,
    };

    props.setOverrides(
      upsertOverride(stripOverrideCandidates(props.overrides(), resource), override),
    );
    const nextRawConfig = stripRawOverrideCandidates(props.rawOverridesConfig(), resource);
    nextRawConfig[storageId] = {
      ...(previousRaw || {}),
      snapshot: newSnapshot,
    };
    props.setRawOverridesConfig(nextRawConfig);
    props.setHasUnsavedChanges(true);
  };

  return {
    handleSaveBulkEdit,
    removeOverride,
    saveEdit,
    toggleBackup,
    toggleSnapshot,
  } as const;
}
