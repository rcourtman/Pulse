import type { Resource } from '@/types/resource';
import {
  getPrimaryResourceIdentityRows,
  getResourceIdentityAliases,
  type ResourceIdentityRow,
} from '@/utils/resourceIdentity';
import { formatIdentifierLabel } from '@/utils/textPresentation';
import type { DiscoveryConfig } from './resourceDetailDiscoveryModel';
import type { PlatformData } from './resourceDetailMappers';

export type ResourceDetailDrawerSourceSection = {
  id: string;
  label: string;
  payload: unknown;
};

export type ResourceDetailDrawerIdentityView = {
  identityAliasValues: string[];
  identityIpValues: string[];
  primaryIdentityRows: ResourceIdentityRow[];
  identityCardHasRichData: boolean;
  aliasPreviewValues: string[];
  hasAliasOverflow: boolean;
};

const ALIAS_COLLAPSE_THRESHOLD = 4;

export const buildResourceIdentityView = (resource: Resource): ResourceDetailDrawerIdentityView => {
  const identityAliasValues = getResourceIdentityAliases(resource);
  const identityIpValues = resource.identity?.ips ?? [];
  const primaryIdentityRows = getPrimaryResourceIdentityRows(resource);

  return {
    identityAliasValues,
    identityIpValues,
    primaryIdentityRows,
    identityCardHasRichData:
      primaryIdentityRows.length > 0 ||
      identityIpValues.length > 0 ||
      (resource.tags?.length || 0) > 0 ||
      identityAliasValues.length > 0,
    aliasPreviewValues: identityAliasValues.slice(0, ALIAS_COLLAPSE_THRESHOLD),
    hasAliasOverflow: identityAliasValues.length > ALIAS_COLLAPSE_THRESHOLD,
  };
};

export const buildDiscoveryContextSummary = (
  discoveryConfig: DiscoveryConfig | null,
): string | null => {
  if (!discoveryConfig || discoveryConfig.resourceType === 'agent') {
    return null;
  }

  const discoveryMode = `${formatIdentifierLabel(discoveryConfig.resourceType)} analysis`;
  return discoveryConfig.hostname
    ? `${discoveryMode} via ${discoveryConfig.hostname}`
    : discoveryMode;
};

export const buildSourceSections = (
  platformData?: PlatformData,
): ResourceDetailDrawerSourceSection[] => {
  if (!platformData) {
    return [];
  }

  return [
    { id: 'proxmox', label: 'Proxmox', payload: platformData.proxmox },
    { id: 'agent', label: 'Agent', payload: platformData.agent },
    { id: 'docker', label: 'Containers', payload: platformData.docker },
    { id: 'pbs', label: 'PBS', payload: platformData.pbs },
    { id: 'pmg', label: 'PMG', payload: platformData.pmg },
    { id: 'kubernetes', label: 'Kubernetes', payload: platformData.kubernetes },
    { id: 'vmware', label: 'vSphere', payload: platformData.vmware },
    { id: 'metrics', label: 'Metrics', payload: platformData.metrics },
  ].filter((section) => section.payload !== undefined);
};

export const buildIdentityMatchInfo = (platformData?: PlatformData): unknown =>
  platformData?.identityMatch ??
  platformData?.matchResults ??
  platformData?.matchCandidates ??
  platformData?.matches ??
  undefined;

export const buildResourceDebugBundle = (options: {
  resource: Resource;
  platformData?: PlatformData;
  sourceStatus: NonNullable<PlatformData['sourceStatus']>;
  identityMatchInfo: unknown;
}) => ({
  resource: options.resource,
  identity: {
    resourceIdentity: options.resource.identity,
    matchInfo: options.identityMatchInfo,
  },
  sources: {
    sourceStatus: options.sourceStatus,
    proxmox: options.platformData?.proxmox,
    agent: options.platformData?.agent,
    docker: options.platformData?.docker,
    pbs: options.platformData?.pbs,
    pmg: options.platformData?.pmg,
    kubernetes: options.platformData?.kubernetes,
    vmware: options.platformData?.vmware,
    metrics: options.platformData?.metrics,
  },
});
