import { createMemo } from 'solid-js';

import { getAlertResourceDisplayLabel } from '@/features/alerts/helpers';
import { buildKubernetesPageModel } from '@/features/kubernetes/kubernetesPageModel';
import { buildTrueNASPageModel } from '@/features/truenas/truenasPageModel';
import { buildVmwarePageModel } from '@/features/vmware/vmwarePageModel';
import type { Resource } from '@/types/resource';

import type { Resource as TableResource } from '../tableTypes';
import type { Override } from '../types';
import type { ThresholdsDataInputs } from '../thresholdsResourceModel';
import {
  createOverridesMap,
  findOverrideByCandidates,
  hasThresholdDiff,
  readString,
  uniqueIds,
} from '../thresholdsResourceModel';

type AlertPlatformResourceType =
  | 'kubernetesCluster'
  | 'kubernetesNode'
  | 'kubernetesNamespace'
  | 'kubernetesDeployment'
  | 'kubernetesPod'
  | 'truenasSystem'
  | 'truenasPool'
  | 'truenasDataset'
  | 'truenasDisk'
  | 'vmwareHost'
  | 'vmwareVm'
  | 'vmwareDatastore'
  | 'vmwareNetwork';

const resourceAlertIdCandidates = (resource: Resource): string[] =>
  uniqueIds(
    resource.id,
    resource.metricsTarget?.resourceId,
    resource.discoveryTarget?.resourceId,
    resource.platformId,
  );

const resourceAlertActionId = (resource: Resource): string =>
  resourceAlertIdCandidates(resource)[0] || resource.id;

const searchResource = (resource: TableResource, search: string): boolean => {
  if (!search) return true;
  return [
    resource.name,
    resource.displayName,
    resource.rawName,
    resource.node,
    resource.instance,
    resource.resourceType,
  ].some((value) => typeof value === 'string' && value.toLowerCase().includes(search));
};

const metricPercent = (
  resource: Resource,
  metric: 'cpu' | 'memory' | 'disk',
): number | undefined => {
  const value = resource[metric]?.current;
  return typeof value === 'number' ? value : undefined;
};

const kubernetesClusterName = (resource: Resource): string =>
  readString(resource.kubernetes?.clusterName) ||
  readString(resource.kubernetes?.context) ||
  readString(resource.kubernetes?.clusterId) ||
  readString(resource.clusterId) ||
  '';

const kubernetesNamespace = (resource: Resource): string =>
  readString(resource.kubernetes?.namespace) || '';

const trueNASSystemName = (resource: Resource): string =>
  readString(resource.truenas?.hostname) || readString(resource.parentName) || '';

const vmwareConnectionName = (resource: Resource): string =>
  readString(resource.vmware?.connectionName) ||
  readString(resource.vmware?.vcenterHost) ||
  'vSphere';

const vmwareNodeName = (resource: Resource): string =>
  readString(resource.vmware?.runtimeHostName) ||
  readString(resource.parentName) ||
  readString(resource.vmware?.clusterName) ||
  readString(resource.vmware?.datacenterName) ||
  readString(resource.vmware?.vcenterHost) ||
  '';

const toTableResource = ({
  resource,
  type,
  resourceType,
  defaults,
  candidates = resourceAlertIdCandidates(resource),
  overridesMap,
  node,
  instance,
}: {
  resource: Resource;
  type: AlertPlatformResourceType;
  resourceType: string;
  defaults: Record<string, number | undefined>;
  candidates?: string[];
  overridesMap: Map<string, Override>;
  node?: string;
  instance?: string;
}): TableResource => {
  const override = findOverrideByCandidates(overridesMap, candidates);
  const hasCustomThresholds = hasThresholdDiff(override, defaults);
  const note = typeof override?.note === 'string' ? override.note : undefined;

  return {
    id: override?.id || candidates[0] || resource.id,
    name: getAlertResourceDisplayLabel(resource),
    displayName: getAlertResourceDisplayLabel(resource),
    rawName: resource.name,
    type,
    resourceType,
    node,
    instance,
    status: resource.status,
    cpu: metricPercent(resource, 'cpu'),
    memory: metricPercent(resource, 'memory'),
    hasOverride: hasCustomThresholds || Boolean(override?.disabled) || Boolean(note && note.trim()),
    disabled: override?.disabled || false,
    thresholds: override?.thresholds || {},
    defaults,
    clusterName: kubernetesClusterName(resource) || undefined,
    note,
  };
};

export function useThresholdsPlatformData(inputs: ThresholdsDataInputs) {
  const { props, editingId, searchTerm } = inputs;

  const allResources = () => props.allResources ?? [];
  const kubernetesDefaults = () => props.kubernetesDefaults ?? {};
  const trueNASDefaults = () => props.trueNASDefaults ?? {};
  const trueNASDiskDefaults = () => props.trueNASDiskDefaults ?? {};
  const vmwareDefaults = () => props.vmwareDefaults ?? {};

  const kubernetesClustersWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildKubernetesPageModel(allResources())
      .clusters.map((resource) =>
        toTableResource({
          resource,
          type: 'kubernetesCluster',
          resourceType: 'Kubernetes Cluster',
          defaults: kubernetesDefaults(),
          overridesMap,
          node: kubernetesClusterName(resource),
          instance: kubernetesClusterName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const kubernetesNodesWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildKubernetesPageModel(allResources())
      .nodes.map((resource) =>
        toTableResource({
          resource,
          type: 'kubernetesNode',
          resourceType: 'Kubernetes Node',
          defaults: kubernetesDefaults(),
          overridesMap,
          node: readString(resource.kubernetes?.nodeName) || resource.name,
          instance: kubernetesClusterName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const kubernetesNamespacesWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildKubernetesPageModel(allResources())
      .namespaces.map((resource) =>
        toTableResource({
          resource,
          type: 'kubernetesNamespace',
          resourceType: 'Kubernetes Namespace',
          defaults: {},
          overridesMap,
          node: kubernetesClusterName(resource),
          instance: kubernetesClusterName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const kubernetesDeploymentsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildKubernetesPageModel(allResources())
      .deployments.map((resource) =>
        toTableResource({
          resource,
          type: 'kubernetesDeployment',
          resourceType: 'Kubernetes Deployment',
          defaults: kubernetesDefaults(),
          overridesMap,
          node: kubernetesNamespace(resource),
          instance: kubernetesClusterName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const kubernetesPodsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildKubernetesPageModel(allResources())
      .pods.map((resource) =>
        toTableResource({
          resource,
          type: 'kubernetesPod',
          resourceType: 'Kubernetes Pod',
          defaults: kubernetesDefaults(),
          overridesMap,
          node: readString(resource.kubernetes?.nodeName) || kubernetesNamespace(resource),
          instance: kubernetesClusterName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const trueNASSystemsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildTrueNASPageModel(allResources())
      .systems.map((resource) =>
        toTableResource({
          resource,
          type: 'truenasSystem',
          resourceType: 'TrueNAS System',
          defaults: trueNASDefaults(),
          overridesMap,
          node: trueNASSystemName(resource),
          instance: 'TrueNAS',
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const trueNASPoolsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildTrueNASPageModel(allResources())
      .pools.map((resource) =>
        toTableResource({
          resource,
          type: 'truenasPool',
          resourceType: 'TrueNAS Pool',
          defaults: { usage: trueNASDefaults().usage ?? 85 },
          overridesMap,
          node: trueNASSystemName(resource),
          instance: 'TrueNAS',
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const trueNASDatasetsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildTrueNASPageModel(allResources())
      .datasets.map((resource) =>
        toTableResource({
          resource,
          type: 'truenasDataset',
          resourceType: 'TrueNAS Dataset',
          defaults: { usage: trueNASDefaults().usage ?? 85 },
          overridesMap,
          node: trueNASSystemName(resource),
          instance: 'TrueNAS',
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const trueNASDisksWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildTrueNASPageModel(allResources())
      .disks.map((resource) =>
        toTableResource({
          resource,
          type: 'truenasDisk',
          resourceType: 'TrueNAS Disk',
          defaults: trueNASDiskDefaults(),
          overridesMap,
          node: trueNASSystemName(resource),
          instance: readString(resource.physicalDisk?.diskType) || 'TrueNAS',
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const vmwareHostsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildVmwarePageModel(allResources())
      .hosts.map((resource) =>
        toTableResource({
          resource,
          type: 'vmwareHost',
          resourceType: 'vSphere Host',
          defaults: vmwareDefaults(),
          overridesMap,
          node: vmwareNodeName(resource),
          instance: vmwareConnectionName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const vmwareVMsWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildVmwarePageModel(allResources())
      .vms.map((resource) =>
        toTableResource({
          resource,
          type: 'vmwareVm',
          resourceType: 'vSphere VM',
          defaults: vmwareDefaults(),
          overridesMap,
          node: vmwareNodeName(resource),
          instance: vmwareConnectionName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const vmwareDatastoresWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildVmwarePageModel(allResources())
      .datastores.map((resource) =>
        toTableResource({
          resource,
          type: 'vmwareDatastore',
          resourceType: 'vSphere Datastore',
          defaults: { usage: vmwareDefaults().usage ?? 85 },
          overridesMap,
          node: vmwareNodeName(resource),
          instance: vmwareConnectionName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  const vmwareNetworksWithOverrides = createMemo<TableResource[]>((prev = []) => {
    if (editingId()) return prev;
    const search = searchTerm().toLowerCase();
    const overridesMap = createOverridesMap(props.overrides());
    return buildVmwarePageModel(allResources())
      .networks.map((resource) =>
        toTableResource({
          resource,
          type: 'vmwareNetwork',
          resourceType: 'vSphere Network',
          defaults: {},
          overridesMap,
          node: vmwareNodeName(resource),
          instance: vmwareConnectionName(resource),
        }),
      )
      .filter((resource) => searchResource(resource, search));
  }, []);

  return {
    kubernetesClustersWithOverrides,
    kubernetesNodesWithOverrides,
    kubernetesNamespacesWithOverrides,
    kubernetesDeploymentsWithOverrides,
    kubernetesPodsWithOverrides,
    trueNASSystemsWithOverrides,
    trueNASPoolsWithOverrides,
    trueNASDatasetsWithOverrides,
    trueNASDisksWithOverrides,
    vmwareHostsWithOverrides,
    vmwareVMsWithOverrides,
    vmwareDatastoresWithOverrides,
    vmwareNetworksWithOverrides,
  };
}

export { resourceAlertIdCandidates, resourceAlertActionId };
