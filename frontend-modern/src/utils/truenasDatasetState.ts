import type { Resource } from '@/types/resource';
import { getResourceSourceStatus } from '@/utils/resourceSourceHealth';
import { asTrimmedString } from '@/utils/stringUtils';

// A read-only TrueNAS dataset raises no incident, so its state tag
// (datasetStateTag in internal/truenas/provider.go) is the only place the
// provider says why it is impaired. Locked and unmounted datasets raise
// incidents too, which callers prefer when present. Shares and apps carry
// their own state tags, so only dataset resources should be read this way.
const TRUENAS_DATASET_STATE_SUMMARIES: Record<string, string> = {
  'state:locked': 'Dataset is locked',
  'state:unmounted': 'Dataset is not mounted',
  'state:readonly': 'Dataset is read-only',
};

export const getTrueNASDatasetStateSummary = (
  tags: readonly string[] | undefined,
): string | null => {
  for (const tag of tags ?? []) {
    const summary = TRUENAS_DATASET_STATE_SUMMARIES[tag.trim().toLowerCase()];
    if (summary) return summary;
  }
  return null;
};

const CONNECTED_STATUS = /^(online|running|healthy|connected|ok)$/i;
const ERROR_STATUS = /^(error|failed|unauthorized)$/i;

export type ProviderStaleNotice = {
  badge: string;
  detail: string;
  error?: string;
};

export type TrueNASStaleNotice = ProviderStaleNotice;

export const buildProviderStaleNotice = (
  entry: { status?: unknown; error?: unknown } | undefined,
  provider: string,
): ProviderStaleNotice | null => {
  const status = typeof entry?.status === 'string' ? entry.status.trim() : '';
  if (!entry || CONNECTED_STATUS.test(status)) return null;
  const error = asTrimmedString(entry.error);
  const prefix =
    error && ERROR_STATUS.test(status)
      ? `${provider} collection error`
      : `${provider} has not updated this recently`;
  return {
    badge: 'Stale data',
    detail: error ? `${prefix}: ${error}` : prefix,
    ...(error ? { error } : {}),
  };
};

// Prefer top-level sourceStatus written by live WebSocket and REST adaptation,
// falling back to platformData.sourceStatus for older fixtures so live recovery
// clears a stale REST copy without waiting for a reload.
export const getTrueNASStaleNotice = (resource: Resource): TrueNASStaleNotice | null =>
  buildProviderStaleNotice(
    resource.sourceStatus?.truenas ?? getResourceSourceStatus(resource, 'truenas'),
    'TrueNAS',
  );

export const isTrueNASSourceImpaired = (resource: Resource): boolean =>
  getTrueNASStaleNotice(resource) !== null;
