import type { RawOverrideConfig } from '@/types/alerts';

import { guestOverrideIdCandidates, guestOverrideStorageId } from '../guestOverrideIdentity';
import type { Resource as TableResource } from './tableTypes';
import type { Override } from './types';

type OverrideIdentityResource = Pick<
  TableResource,
  'id' | 'type' | 'vmid' | 'node' | 'instance' | 'overrideIdCandidates' | 'overrideStorageId'
>;

const exactOverrideIdentity = (resource?: OverrideIdentityResource) => ({
  candidateIds:
    resource?.overrideIdCandidates && resource.overrideIdCandidates.length > 0
      ? resource.overrideIdCandidates
      : resource?.id
        ? [resource.id]
        : [],
  storageId: resource?.overrideStorageId || resource?.id || '',
});

export const getOverridePersistenceIdentity = (resource?: OverrideIdentityResource) => {
  if (!resource || resource.type !== 'guest') {
    return exactOverrideIdentity(resource);
  }

  const candidateIds = guestOverrideIdCandidates(resource);
  return {
    candidateIds: candidateIds.length > 0 ? candidateIds : [resource.id],
    storageId: guestOverrideStorageId(resource) || resource.id,
  };
};

export const findOverrideForResource = (
  overrides: Override[],
  resource?: OverrideIdentityResource,
): Override | undefined => {
  const { candidateIds } = getOverridePersistenceIdentity(resource);
  return overrides.find((override) => candidateIds.includes(override.id));
};

export const findRawOverrideConfigForResource = (
  rawOverridesConfig: Record<string, RawOverrideConfig>,
  resource?: OverrideIdentityResource,
): RawOverrideConfig | undefined => {
  const { candidateIds } = getOverridePersistenceIdentity(resource);
  return candidateIds
    .map((candidate) => rawOverridesConfig[candidate])
    .find((override): override is RawOverrideConfig => Boolean(override));
};

export const stripOverrideCandidates = (
  overrides: Override[],
  resource?: OverrideIdentityResource,
): Override[] => {
  const { candidateIds } = getOverridePersistenceIdentity(resource);
  if (candidateIds.length === 0) {
    return overrides;
  }
  return overrides.filter((override) => !candidateIds.includes(override.id));
};

export const stripRawOverrideCandidates = (
  rawOverridesConfig: Record<string, RawOverrideConfig>,
  resource?: OverrideIdentityResource,
): Record<string, RawOverrideConfig> => {
  const nextRawConfig = { ...rawOverridesConfig };
  const { candidateIds } = getOverridePersistenceIdentity(resource);
  candidateIds.forEach((candidate) => {
    delete nextRawConfig[candidate];
  });
  return nextRawConfig;
};
