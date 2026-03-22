import { matchesAlertIdentifier } from '@/features/alerts/identity';

import type {
  OfflineState,
  Override,
  OverrideType,
  ThresholdsTableProps,
} from '@/features/alerts/thresholds/types';
import type { Resource as TableResource } from '@/features/alerts/thresholds/tableTypes';
import {
  stripStateKeys,
  upsertOverride,
  withThresholdEntries,
} from '@/features/alerts/thresholds/thresholdsOverrideMutationModel';

interface ThresholdsAvailabilityMutationResources {
  nodesWithOverrides: () => TableResource[];
  agentsWithOverrides: () => TableResource[];
  agentDisksWithOverrides: () => TableResource[];
  dockerHostsWithOverrides: () => TableResource[];
  guestsFlat: () => TableResource[];
  dockerContainersFlat: () => TableResource[];
  pbsServersWithOverrides: () => TableResource[];
  storageWithOverrides: () => TableResource[];
}

interface ThresholdsAvailabilityMutationProps {
  props: ThresholdsTableProps;
  resources: ThresholdsAvailabilityMutationResources;
  removeOverride: (resourceId: string) => void;
}

export function useThresholdsAvailabilityMutations({
  props,
  resources,
  removeOverride,
}: ThresholdsAvailabilityMutationProps) {
  const guestLikeResources = () => [
    ...resources.guestsFlat(),
    ...resources.dockerContainersFlat(),
  ];

  const clearDockerHostConnectivityAlerts = (resourceId: string) => {
    if (!props.removeAlerts) return;

    const offlineId = `docker-host-offline-${resourceId}`;
    const resourceKey = `docker:${resourceId}`;
    props.removeAlerts(
      (alert) => matchesAlertIdentifier(alert, offlineId) || alert.resourceId === resourceKey,
    );
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
      disabled: newDisabledState,
      disableConnectivity: existingOverride?.disableConnectivity,
      poweredOffSeverity: existingOverride?.poweredOffSeverity,
      backup: existingOverride?.backup,
      snapshot: existingOverride?.snapshot,
      thresholds: cleanThresholds,
    };

    props.setOverrides(upsertOverride(props.overrides(), override));

    const hysteresisThresholds = withThresholdEntries({}, override.thresholds);
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
      if (resource.type === 'dockerHost') {
        clearDockerHostConnectivityAlerts(resourceId);
      }
      return;
    }

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

    const hysteresisThresholds = withThresholdEntries({}, cleanThresholds);
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
    props.setHasUnsavedChanges(true);

    if (resource.type === 'dockerHost') {
      clearDockerHostConnectivityAlerts(resourceId);
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

    const hysteresisThresholds = withThresholdEntries({}, cleanThresholds);
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
    setOfflineState,
    toggleDisabled,
    toggleNodeConnectivity,
  } as const;
}
