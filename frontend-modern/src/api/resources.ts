import { apiFetchJSON } from '@/utils/apiClient';
import type {
  ResourceCapability,
  ResourceChange,
  ResourceRelationship,
} from '@/types/resource';

export interface ResourceCapabilitiesResponse {
  resourceId: string;
  capabilities: ResourceCapability[];
  count: number;
}

export interface ResourceRelationshipsResponse {
  resourceId: string;
  relationships: ResourceRelationship[];
  count: number;
}

export interface ResourceTimelineQueryOptions {
  since?: string | number | Date;
  limit?: number;
}

export interface ResourceTimelineResponse {
  resourceId: string;
  recentChanges: ResourceChange[];
  count: number;
}

export interface ResourceFacetCounts {
  capabilities: number;
  relationships: number;
  recentChanges: number;
}

export interface ResourceFacetBundle {
  capabilities: ResourceCapability[];
  relationships: ResourceRelationship[];
  recentChanges: ResourceChange[];
  counts: ResourceFacetCounts;
}

const normalizeResourceId = (resourceId: string): string => resourceId.trim();

const buildFacetPath = (resourceId: string, suffix: string): string =>
  `/api/resources/${encodeURIComponent(normalizeResourceId(resourceId))}/${suffix}`;

const buildTimelineQuery = (options?: ResourceTimelineQueryOptions): string => {
  const params = new URLSearchParams();
  if (options?.since !== undefined && options.since !== null && `${options.since}`.trim()) {
    const date = options.since instanceof Date ? options.since : new Date(options.since);
    if (Number.isFinite(date.getTime())) {
      params.set('since', date.toISOString());
    }
  }
  if (Number.isFinite(options?.limit ?? NaN) && (options?.limit ?? 0) > 0) {
    params.set('limit', String(Math.trunc(options?.limit ?? 0)));
  }
  const query = params.toString();
  return query ? `?${query}` : '';
};

const fetchFacet = async <T>(url: string): Promise<T> =>
  apiFetchJSON<T>(url, {
    cache: 'no-store',
  });

export class ResourceAPI {
  static async getCapabilities(resourceId: string): Promise<ResourceCapabilitiesResponse> {
    return fetchFacet<ResourceCapabilitiesResponse>(buildFacetPath(resourceId, 'capabilities'));
  }

  static async getRelationships(resourceId: string): Promise<ResourceRelationshipsResponse> {
    return fetchFacet<ResourceRelationshipsResponse>(buildFacetPath(resourceId, 'relationships'));
  }

  static async getTimeline(
    resourceId: string,
    options?: ResourceTimelineQueryOptions,
  ): Promise<ResourceTimelineResponse> {
    return fetchFacet<ResourceTimelineResponse>(
      `${buildFacetPath(resourceId, 'timeline')}${buildTimelineQuery(options)}`,
    );
  }

  static async getFacetBundle(
    resourceId: string,
    options?: ResourceTimelineQueryOptions,
  ): Promise<ResourceFacetBundle> {
    const [capabilities, relationships, recentChanges] = await Promise.all([
      ResourceAPI.getCapabilities(resourceId),
      ResourceAPI.getRelationships(resourceId),
      ResourceAPI.getTimeline(resourceId, options),
    ]);

    return {
      capabilities: capabilities.capabilities ?? [],
      relationships: relationships.relationships ?? [],
      recentChanges: recentChanges.recentChanges ?? [],
      counts: {
        capabilities: capabilities.count ?? 0,
        relationships: relationships.count ?? 0,
        recentChanges: recentChanges.count ?? 0,
      },
    };
  }
}
