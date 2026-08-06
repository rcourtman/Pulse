import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import type { PBSInstance, PMGInstance } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource, ResourceType } from '@/types/resource';
import { resolveProxmoxPlatformScope } from '@/features/proxmox/proxmoxPageModel';
import { isAgentFacetInfrastructureResource } from '@/utils/agentResources';
import { pbsInstanceFromResource, pmgInstanceFromResource } from '@/utils/resourceStateAdapters';
import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';

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

const isVirtualizationHostResource = (resource: Resource): boolean => {
  const platformType = resolveResourcePlatformType(resource);
  if (platformType) return platformType === 'proxmox-pve';
  return resolveProxmoxPlatformScope(resource) === 'proxmox-pve';
};

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

  // Virtualization Hosts is the Proxmox PVE threshold scope. Other canonical
  // `agent` resources have their own identity-aware sections: Pulse agents use
  // Systems, while TrueNAS and vSphere use their platform tabs. Letting any of
  // those resources into this list duplicates the same resource ID with node
  // defaults, so the shared save path can normalize a platform override against
  // the wrong defaults (for example, TrueNAS memory 95% against PVE's 95%).
  const virtualizationHostResources = createMemo(() =>
    props.byType('agent').filter(isVirtualizationHostResource),
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
    const storageResources = props
      .allResources()
      .filter((resource) => resource.type === 'storage' || resource.type === 'datastore');
    const normalizedRawConfig = normalizeRawOverridesConfig(rawConfig, storageResources);
    if (JSON.stringify(normalizedRawConfig) !== JSON.stringify(rawConfig)) {
      setRawOverridesConfig(normalizedRawConfig);
      return;
    }

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
      allResources: props.allResources(),
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
          JSON.stringify(newOverride.snapshot ?? null) !== JSON.stringify(existing.snapshot ?? null)
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
    const storageResources = props
      .allResources()
      .filter((resource) => resource.type === 'storage' || resource.type === 'datastore');
    setRawOverridesConfig(normalizeRawOverridesConfig(value, storageResources));
  };

  return {
    overrides,
    setOverrides,
    rawOverridesConfig,
    setRawOverridesConfig,
    replaceRawOverridesConfig,
    allGuests,
    agentResources,
    virtualizationHostResources,
    containerRuntimeResources,
    pbsInstances,
    pmgInstances,
  };
}
