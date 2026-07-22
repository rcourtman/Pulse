import type { Node, PBSInstance } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource } from '@/types/resource';
import { getActionableAgentIdFromResource, isTrueNASSystemResource } from '@/utils/agentResources';
import { isAppContainerDiscoveryResourceType } from '@/utils/discoveryTarget';

import {
  extractTriggerValues,
  getAlertResourceDisplayLabel,
  guessNumericId,
  platformData,
} from './helpers';
import {
  getGuestOverrideIdentity,
  guestOverrideIdCandidates,
  guestOverrideStorageId,
  normalizeGuestOverrideKey,
} from './guestOverrideIdentity';
import type { Override } from './types';

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const uniqueIds = (...values: unknown[]): string[] => {
  const ids: string[] = [];
  const seen = new Set<string>();

  values.forEach((value) => {
    const normalized = asString(value);
    if (!normalized || seen.has(normalized)) return;
    seen.add(normalized);
    ids.push(normalized);
  });

  return ids;
};

const buildSharedStorageLegacyKeyMap = (storageResources: Resource[]): Map<string, string> => {
  const legacyToCanonical = new Map<string, string>();

  storageResources.forEach((resource) => {
    const storageMeta = resource.storage;
    const resourceName = asString(resource.name);
    const proxmox = resource.proxmox;
    const instance = asString(proxmox?.instance) || asString(resource.platformId);
    const canonicalID = storageOverrideActionId(resource);

    if (!storageMeta?.shared || !resourceName || !canonicalID) {
      return;
    }

    const clusterNodes = Array.isArray(storageMeta.nodes) ? storageMeta.nodes : [];
    clusterNodes.forEach((node) => {
      const normalizedNode = asString(node);
      if (!normalizedNode) {
        return;
      }

      const prefix =
        instance && !normalizedNode.toLowerCase().startsWith(`${instance.toLowerCase()}-`)
          ? `${instance}-${normalizedNode}`
          : normalizedNode;
      legacyToCanonical.set(`${prefix}-${resourceName}`, canonicalID);
    });
  });

  return legacyToCanonical;
};

// PBS datastore overrides were historically stored under the poller's
// "<instance-id>-<name>" storage ID while the canonical resource ID is
// "<instance-id>/<name>". Datastore names cannot contain "/", so mapping the
// last slash to a dash reconstructs the legacy key exactly; carrying it as a
// trailing candidate keeps pre-existing overrides bound to the single card
// and lets the next save re-home them onto the canonical key (#1591).
const legacyPBSDatastoreOverrideId = (
  resource: Resource,
  canonicalId?: string,
): string | undefined => {
  if (resource.storage?.platform !== 'pbs' || !canonicalId) return undefined;
  const slash = canonicalId.lastIndexOf('/');
  if (slash <= 0) return undefined;
  return `${canonicalId.slice(0, slash)}-${canonicalId.slice(slash + 1)}`;
};

export const storageOverrideIdCandidates = (resource: Resource): string[] => {
  const canonicalId =
    resource.metricsTarget?.resourceType === 'storage'
      ? resource.metricsTarget.resourceId
      : undefined;
  return uniqueIds(canonicalId, resource.id, legacyPBSDatastoreOverrideId(resource, canonicalId));
};

export const storageOverrideActionId = (resource: Resource): string =>
  storageOverrideIdCandidates(resource)[0] || resource.id;

const buildStorageOverrideKeyMap = (storageResources: Resource[]): Map<string, string> => {
  const keyMap = new Map<string, string>();

  storageResources.forEach((resource) => {
    const canonicalID = storageOverrideActionId(resource);
    if (!canonicalID) return;

    storageOverrideIdCandidates(resource).forEach((candidate) => {
      if (candidate !== canonicalID) {
        keyMap.set(candidate, canonicalID);
      }
    });
  });

  buildSharedStorageLegacyKeyMap(storageResources).forEach((canonicalID, legacyID) => {
    keyMap.set(legacyID, canonicalID);
  });

  return keyMap;
};

export const normalizeRawOverridesConfig = (
  rawOverrides: Record<string, RawOverrideConfig>,
  storageResources: Resource[] = [],
): Record<string, RawOverrideConfig> => {
  const cleanedOverrides: Record<string, RawOverrideConfig> = {};
  const priorityByKey = new Map<string, number>();
  const storageOverrideKeyMap = buildStorageOverrideKeyMap(storageResources);

  for (const [key, value] of Object.entries(rawOverrides)) {
    const normalizedGuestKey = normalizeGuestOverrideKey(key);
    const diskMatch = normalizedGuestKey.match(/^(agent:.+\/disk:)(.+)$/);
    let normalizedKey = normalizedGuestKey;
    if (diskMatch) {
      const normalized =
        diskMatch[2]
          .toLowerCase()
          .replace(/[^a-z0-9]/g, '-')
          .replace(/-{2,}/g, '-')
          .replace(/^-|-$/g, '') || 'unknown';
      normalizedKey = diskMatch[1] + normalized;
    }
    const storageOverrideKey = storageOverrideKeyMap.get(normalizedKey);
    if (storageOverrideKey) {
      normalizedKey = storageOverrideKey;
    }

    const priority = normalizedKey === key ? 1 : 0;
    const existingPriority = priorityByKey.get(normalizedKey);
    if (existingPriority !== undefined && existingPriority > priority) {
      continue;
    }

    priorityByKey.set(normalizedKey, priority);
    cleanedOverrides[normalizedKey] = value;
  }

  return cleanedOverrides;
};

export const hostOverrideIdCandidates = (resource: Resource): string[] => {
  const data = platformData(resource);
  const agent = asRecord(data?.agent);
  return uniqueIds(
    getActionableAgentIdFromResource(resource),
    resource.discoveryTarget?.agentId,
    resource.agent?.agentId,
    agent?.agentId,
    data?.agentId,
    resource.id,
  );
};

export const nodeOverrideIdCandidates = (
  node: Pick<Node, 'id' | 'name' | 'instance' | 'host' | 'linkedAgentId'>,
): string[] =>
  uniqueIds(
    node.linkedAgentId,
    node.id,
    node.instance && node.name ? `${node.instance}-${node.name}` : undefined,
    node.instance && node.name ? `${node.instance}:${node.name}` : undefined,
    node.name,
    node.host,
  );

export const dockerHostOverrideIdCandidates = (resource: Resource): string[] => {
  const data = platformData(resource);
  const docker = asRecord(data?.docker);
  const discoveryTarget = resource.discoveryTarget;
  return uniqueIds(
    isAppContainerDiscoveryResourceType(discoveryTarget?.resourceType)
      ? discoveryTarget?.resourceId
      : undefined,
    docker?.hostSourceId,
    data?.hostSourceId,
    discoveryTarget?.agentId,
    resource.id,
  );
};

export const dockerContainerOverrideIdCandidates = (host: Resource, shortId: string): string[] =>
  uniqueIds(...dockerHostOverrideIdCandidates(host).map((hostId) => `docker:${hostId}/${shortId}`));

export const buildContainerRuntimeResources = ({
  allResources,
  dockerHostResources,
}: {
  allResources: Resource[];
  dockerHostResources: Resource[];
}): Resource[] => {
  const resourceById = new Map(allResources.map((resource) => [resource.id, resource]));
  const runtimes: Resource[] = [];
  const seen = new Set<string>();

  const addRuntime = (resource: Resource | undefined) => {
    if (!resource || seen.has(resource.id)) return;
    seen.add(resource.id);
    runtimes.push(resource);
  };

  dockerHostResources.forEach(addRuntime);
  allResources.forEach((resource) => {
    if (isTrueNASSystemResource(resource)) {
      addRuntime(resource);
    }
  });
  allResources.forEach((resource) => {
    if (resource.type !== 'app-container' || !resource.parentId) return;
    addRuntime(resourceById.get(resource.parentId));
  });

  return runtimes;
};

interface BuildProjectedOverridesArgs {
  rawConfig: Record<string, RawOverrideConfig>;
  nodeResources: Resource[];
  vmResources: Resource[];
  containerResources: Resource[];
  storageResources: Resource[];
  agentResourceList: Resource[];
  containerRuntimeResources: Resource[];
  getChildren: (resourceId: string) => Resource[];
  pbsInstanceById: Map<string, PBSInstance>;
  allResources?: Resource[];
}

export const buildProjectedOverrides = ({
  rawConfig,
  nodeResources,
  vmResources,
  containerResources,
  storageResources,
  agentResourceList,
  containerRuntimeResources,
  getChildren,
  pbsInstanceById,
  allResources = [],
}: BuildProjectedOverridesArgs): Override[] => {
  const overridesList: Override[] = [];
  const overrideIndexByID = new Map<string, number>();
  const dockerHostMap = new Map<string, Resource>();
  const dockerContainerMap = new Map<
    string,
    { host: Resource; container: Resource; containerShortId: string }
  >();
  const agentMap = new Map<string, Resource>();
  const guestMap = new Map<string, Resource>();
  const storageMap = new Map<string, Resource>();
  const alertPlatformMap = new Map<
    string,
    { resource: Resource; type: Override['type']; resourceType: string }
  >();

  const upsertProjectedOverride = (override: Override) => {
    const existingIndex = overrideIndexByID.get(override.id);
    if (existingIndex !== undefined) {
      overridesList[existingIndex] = override;
      return;
    }
    overrideIndexByID.set(override.id, overridesList.length);
    overridesList.push(override);
  };

  const alertResourceIdCandidates = (resource: Resource): string[] =>
    uniqueIds(
      resource.id,
      ...(resource.canonicalIdentity?.supersededIds ?? []),
      resource.metricsTarget?.resourceId,
      resource.discoveryTarget?.resourceId,
      resource.platformId,
    );

  const isTrueNASResource = (resource: Resource): boolean =>
    isTrueNASSystemResource(resource) ||
    resource.platformType === 'truenas' ||
    resource.sources?.includes('truenas') ||
    resource.storage?.platform === 'truenas';

  const isVMwareResource = (resource: Resource): boolean =>
    resource.platformType === 'vmware-vsphere' ||
    resource.sources?.includes('vmware') ||
    Boolean(resource.vmware) ||
    resource.storage?.platform === 'vmware-vsphere';

  allResources.forEach((resource) => {
    let type: Override['type'] | undefined;
    let resourceType = '';
    switch (resource.type) {
      case 'k8s-cluster':
        type = 'kubernetesCluster';
        resourceType = 'Kubernetes Cluster';
        break;
      case 'k8s-node':
        type = 'kubernetesNode';
        resourceType = 'Kubernetes Node';
        break;
      case 'k8s-namespace':
        type = 'kubernetesNamespace';
        resourceType = 'Kubernetes Namespace';
        break;
      case 'k8s-deployment':
        type = 'kubernetesDeployment';
        resourceType = 'Kubernetes Deployment';
        break;
      case 'pod':
        type = 'kubernetesPod';
        resourceType = 'Kubernetes Pod';
        break;
      case 'agent':
        if (isTrueNASResource(resource)) {
          type = 'truenasSystem';
          resourceType = 'TrueNAS System';
        } else if (isVMwareResource(resource)) {
          type = 'vmwareHost';
          resourceType = 'vSphere Host';
        }
        break;
      case 'vm':
        if (isVMwareResource(resource)) {
          type = 'vmwareVm';
          resourceType = 'vSphere VM';
        }
        break;
      case 'physical_disk':
        if (isTrueNASResource(resource)) {
          type = 'truenasDisk';
          resourceType = 'TrueNAS Disk';
        }
        break;
      case 'storage':
      case 'pool':
      case 'dataset':
        if (isTrueNASResource(resource)) {
          const topology = resource.storage?.topology || resource.type;
          type = topology === 'dataset' ? 'truenasDataset' : 'truenasPool';
          resourceType = topology === 'dataset' ? 'TrueNAS Dataset' : 'TrueNAS Pool';
        } else if (isVMwareResource(resource)) {
          type = 'vmwareDatastore';
          resourceType = 'vSphere Datastore';
        }
        break;
      case 'network':
        if (isVMwareResource(resource)) {
          type = 'vmwareNetwork';
          resourceType = 'vSphere Network';
        }
        break;
    }
    if (!type) return;
    alertResourceIdCandidates(resource).forEach((id) => {
      alertPlatformMap.set(id, { resource, type, resourceType });
    });
  });

  const storageCoords = (resource: Resource): { node: string; instance: string } => {
    const data = platformData(resource);
    if (resource.type === 'datastore') {
      const instance =
        (data?.pbsInstanceId as string | undefined) ||
        resource.parentId ||
        resource.platformId ||
        'pbs';
      const node = (data?.pbsInstanceName as string | undefined) || instance;
      return { node, instance };
    }

    return {
      node: (data?.node as string | undefined) || '',
      instance: (data?.instance as string | undefined) || resource.platformId || '',
    };
  };

  containerRuntimeResources.forEach((host) => {
    dockerHostOverrideIdCandidates(host).forEach((id) => {
      dockerHostMap.set(id, host);
    });

    const containers = getChildren(host.id).filter((resource) => resource.type === 'app-container');

    containers.forEach((container) => {
      const shortId = container.id.includes('/') ? container.id.split('/').pop()! : container.id;
      dockerContainerOverrideIdCandidates(host, shortId).forEach((resourceId) => {
        dockerContainerMap.set(resourceId, { host, container, containerShortId: shortId });
      });
    });
  });

  agentResourceList.forEach((agentResource) => {
    hostOverrideIdCandidates(agentResource).forEach((id) => {
      agentMap.set(id, agentResource);
    });
  });

  storageResources.forEach((storageResource) => {
    const canonicalID = storageOverrideActionId(storageResource);
    if (canonicalID) {
      storageMap.set(canonicalID, storageResource);
    }
    storageOverrideIdCandidates(storageResource).forEach((candidate) => {
      storageMap.set(candidate, storageResource);
    });
  });

  buildSharedStorageLegacyKeyMap(storageResources).forEach((canonicalID, legacyID) => {
    const storageResource = storageMap.get(canonicalID);
    if (storageResource) {
      storageMap.set(legacyID, storageResource);
    }
  });

  [...vmResources, ...containerResources].forEach((guest) => {
    guestOverrideIdCandidates(guest).forEach((candidate) => {
      guestMap.set(candidate, guest);
    });
  });

  Object.entries(rawConfig).forEach(([key, thresholds]) => {
    const alertPlatformResource = alertPlatformMap.get(key);
    if (alertPlatformResource) {
      const { resource, type, resourceType } = alertPlatformResource;
      upsertProjectedOverride({
        id: key,
        name: getAlertResourceDisplayLabel(resource),
        type,
        resourceType,
        node:
          resource.parentName ||
          resource.kubernetes?.nodeName ||
          resource.truenas?.hostname ||
          resource.vmware?.runtimeHostName ||
          resource.vmware?.clusterName ||
          resource.vmware?.datacenterName ||
          resource.vmware?.vcenterHost,
        instance:
          resource.kubernetes?.clusterName ||
          resource.kubernetes?.namespace ||
          (resource.truenas ? 'TrueNAS' : undefined) ||
          resource.vmware?.connectionName ||
          resource.vmware?.vcenterHost ||
          (resource.vmware ? 'vSphere' : undefined),
        disabled: thresholds.disabled || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const dockerHost = dockerHostMap.get(key);
    if (dockerHost) {
      upsertProjectedOverride({
        id: key,
        name: getAlertResourceDisplayLabel(dockerHost),
        type: 'dockerHost',
        resourceType: 'Container Runtime',
        disableConnectivity: thresholds.disableConnectivity || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const dockerContainer = dockerContainerMap.get(key);
    if (dockerContainer) {
      const { host, container, containerShortId } = dockerContainer;
      const containerName = getAlertResourceDisplayLabel(container, containerShortId);
      upsertProjectedOverride({
        id: key,
        name: containerName,
        type: 'dockerContainer',
        resourceType: 'Container',
        node: getAlertResourceDisplayLabel(host),
        instance: getAlertResourceDisplayLabel(host),
        disabled: thresholds.disabled || false,
        disableConnectivity: thresholds.disableConnectivity || false,
        poweredOffSeverity:
          thresholds.poweredOffSeverity === 'critical'
            ? 'critical'
            : thresholds.poweredOffSeverity === 'warning'
              ? 'warning'
              : undefined,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    if (key.startsWith('docker:')) {
      const [, rest] = key.split(':', 2);
      const [hostId, containerId] = (rest || '').split('/', 2);
      if (containerId) {
        upsertProjectedOverride({
          id: key,
          name: containerId,
          type: 'dockerContainer',
          resourceType: 'Container',
          node: hostId,
          disabled: thresholds.disabled || false,
          disableConnectivity: thresholds.disableConnectivity || false,
          poweredOffSeverity:
            thresholds.poweredOffSeverity === 'critical'
              ? 'critical'
              : thresholds.poweredOffSeverity === 'warning'
                ? 'warning'
                : undefined,
          thresholds: extractTriggerValues(thresholds),
        });
        return;
      }

      upsertProjectedOverride({
        id: key,
        name: hostId || key,
        type: 'dockerHost',
        resourceType: 'Container Runtime',
        disableConnectivity: thresholds.disableConnectivity || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const diskMatch = key.match(/^agent:(.+)\/disk:(.+)$/);
    if (diskMatch) {
      const [, agentId, diskLabel] = diskMatch;
      const agent = agentMap.get(agentId);
      upsertProjectedOverride({
        id: key,
        name: diskLabel.replace(/-/g, '/'),
        type: 'agentDisk',
        resourceType: 'Agent Disk',
        node: agent ? getAlertResourceDisplayLabel(agent) : agentId,
        disabled: thresholds.disabled || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const agentResource = agentMap.get(key);
    if (agentResource) {
      const displayName = getAlertResourceDisplayLabel(agentResource);
      const data = platformData(agentResource);
      const agent = asRecord(data?.agent);
      upsertProjectedOverride({
        id: key,
        name: displayName,
        type: 'agent',
        resourceType: 'Agent',
        node: displayName,
        instance:
          asString(agent?.platform) ||
          asString(agent?.osName) ||
          asString(data?.platform) ||
          asString(data?.osName) ||
          '',
        disabled: thresholds.disabled || false,
        disableConnectivity: thresholds.disableConnectivity || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    // PBS datastore override keys are also "pbs-" prefixed (the canonical
    // "<instance-id>/<name>" and legacy "<instance-id>-<name>" forms), so an
    // instance miss must fall through to the storage lookup below instead of
    // swallowing the key (#1591).
    if (key.startsWith('pbs-')) {
      const pbs = pbsInstanceById.get(key);
      if (pbs) {
        upsertProjectedOverride({
          id: key,
          name: pbs.name,
          type: 'pbs',
          resourceType: 'PBS',
          disableConnectivity: thresholds.disableConnectivity || false,
          thresholds: extractTriggerValues(thresholds),
        });
        return;
      }
    }

    const node = nodeResources.find((resource) => resource.id === key);
    if (node) {
      upsertProjectedOverride({
        id: key,
        name: getAlertResourceDisplayLabel(node),
        type: 'agent',
        resourceType: 'Agent',
        disableConnectivity: thresholds.disableConnectivity || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const storage = storageMap.get(key);
    if (storage) {
      const coords = storageCoords(storage);
      upsertProjectedOverride({
        id: storageOverrideActionId(storage),
        name: getAlertResourceDisplayLabel(storage),
        type: 'storage',
        resourceType: 'Storage',
        node: coords.node,
        instance: coords.instance,
        disabled: thresholds.disabled || false,
        thresholds: extractTriggerValues(thresholds),
      });
      return;
    }

    const guest =
      guestMap.get(key) ||
      vmResources.find((resource) => resource.id === key) ||
      containerResources.find((resource) => resource.id === key);
    if (!guest) {
      return;
    }

    const guestIdentity = getGuestOverrideIdentity(guest);
    upsertProjectedOverride({
      id: guestOverrideStorageId(guest) || key,
      name: getAlertResourceDisplayLabel(guest),
      type: 'guest',
      resourceType: guest.type === 'vm' ? 'VM' : 'Container',
      vmid: guestIdentity?.vmid ?? guessNumericId(guest.id),
      node: guestIdentity?.node ?? '',
      instance: guestIdentity?.instance ?? guest.platformId,
      disabled: thresholds.disabled || false,
      disableConnectivity: thresholds.disableConnectivity || false,
      poweredOffSeverity:
        thresholds.poweredOffSeverity === 'critical'
          ? 'critical'
          : thresholds.poweredOffSeverity === 'warning'
            ? 'warning'
            : undefined,
      thresholds: extractTriggerValues(thresholds),
      backup: thresholds.backup,
      snapshot: thresholds.snapshot,
    });
  });

  return overridesList;
};
