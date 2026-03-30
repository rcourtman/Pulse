import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import type { PBSInstance, PMGInstance } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource, ResourceType } from '@/types/resource';
import { isAgentFacetInfrastructureResource } from '@/utils/agentResources';
import { pbsInstanceFromResource, pmgInstanceFromResource } from '@/utils/resourceStateAdapters';

import {
  buildContainerRuntimeResources,
  buildProjectedOverrides,
  normalizeRawOverridesConfig,
} from './alertOverridesModel';
import type { Override } from './types';

export interface AlertOverridesStateProps {
  allResources: Accessor<Resource[]>;
  byType: (resourceType: ResourceType) => Resource[];
  children: (resourceId: string) => Resource[];
  hasUnsavedChanges: Accessor<boolean>;
  setOverviewOverrides: (value: Override[]) => void;
}

export function useAlertOverridesState(props: AlertOverridesStateProps) {
  const [overrides, setOverrides] = createSignal<Override[]>([]);
  const [rawOverridesConfig, setRawOverridesConfig] = createSignal<
    Record<string, RawOverrideConfig>
  >({});

  const allGuests = createMemo(
    () => [
      ...props.byType('vm'),
      ...props.byType('system-container'),
      ...props.byType('oci-container'),
    ],
    [],
    {
      equals: (prev, next) => {
        if (prev.length !== next.length) return false;
        return prev.every(
          (current, index) => current.id === next[index].id && current.name === next[index].name,
        );
      },
    },
  );

  const agentResources = createMemo(() =>
    props.allResources().filter((resource) => isAgentFacetInfrastructureResource(resource)),
  );

  const pbsInstances = createMemo<PBSInstance[]>(() =>
    props
      .allResources()
      .filter((resource) => resource.type === 'pbs')
      .map(pbsInstanceFromResource)
      .filter((resource): resource is PBSInstance => Boolean(resource)),
  );

  const pbsInstanceById = createMemo(
    () => new Map(pbsInstances().map((instance) => [instance.id, instance])),
  );

  const pmgInstances = createMemo<PMGInstance[]>(() =>
    props
      .allResources()
      .filter((resource) => resource.type === 'pmg')
      .map(pmgInstanceFromResource)
      .filter((resource): resource is PMGInstance => Boolean(resource)),
  );

  const containerRuntimeResources = createMemo(() =>
    buildContainerRuntimeResources({
      allResources: props.allResources(),
      dockerHostResources: props.byType('docker-host'),
    }),
  );

  createEffect(() => {
    if (props.hasUnsavedChanges()) {
      return;
    }

    const rawConfig = rawOverridesConfig();
    if (Object.keys(rawConfig).length === 0) {
      if (overrides().length > 0) {
        setOverrides([]);
      }
      return;
    }

    const nodeResources = props.byType('agent');
    const vmResources = props.byType('vm');
    const containerResources = [
      ...props.byType('system-container'),
      ...props.byType('oci-container'),
    ];
    const storageResources = props
      .allResources()
      .filter((resource) => resource.type === 'storage' || resource.type === 'datastore');
    const agentResourceList = agentResources();
    const overridesList = buildProjectedOverrides({
      rawConfig,
      nodeResources,
      vmResources,
      containerResources,
      storageResources,
      agentResourceList,
      containerRuntimeResources: containerRuntimeResources(),
      getChildren: props.children,
      pbsInstanceById: pbsInstanceById(),
    });

    const currentOverrides = overrides();
    const hasChanged =
      overridesList.length !== currentOverrides.length ||
      overridesList.some((newOverride) => {
        const existing = currentOverrides.find((override) => override.id === newOverride.id);
        if (!existing) return true;
        return (
          JSON.stringify(newOverride.thresholds) !== JSON.stringify(existing.thresholds) ||
          Boolean(newOverride.disableConnectivity) !== Boolean(existing.disableConnectivity) ||
          Boolean(newOverride.disabled) !== Boolean(existing.disabled) ||
          (newOverride.poweredOffSeverity ?? null) !== (existing.poweredOffSeverity ?? null) ||
          JSON.stringify(newOverride.backup ?? null) !== JSON.stringify(existing.backup ?? null) ||
          JSON.stringify(newOverride.snapshot ?? null) !==
            JSON.stringify(existing.snapshot ?? null)
        );
      });

    if (hasChanged) {
      setOverrides(overridesList);
    }
  });

  createEffect(() => {
    props.setOverviewOverrides(overrides());
  });

  const replaceRawOverridesConfig = (value: Record<string, RawOverrideConfig>) => {
    setRawOverridesConfig(normalizeRawOverridesConfig(value));
  };

  return {
    overrides,
    setOverrides,
    rawOverridesConfig,
    setRawOverridesConfig,
    replaceRawOverridesConfig,
    allGuests,
    agentResources,
    containerRuntimeResources,
    pbsInstances,
    pmgInstances,
  };
}
