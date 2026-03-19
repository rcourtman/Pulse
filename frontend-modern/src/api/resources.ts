import { apiFetchJSON } from '@/utils/apiClient';
import type {
  ResourceChange,
  ResourceChangeKind,
  ResourceChangeSourceAdapter,
  ResourceChangeSourceType,
  ResourceFacetCounts,
} from '@/types/resource';

export interface ResourceTimelineQueryOptions {
  since?: string | number | Date;
  limit?: number;
  kind?: ResourceChangeKind;
  sourceType?: ResourceChangeSourceType;
  sourceAdapter?: ResourceChangeSourceAdapter;
}

export interface ResourceTimelineResponse {
  resourceId: string;
  recentChanges: ResourceChange[];
  count: number;
}

export interface ResourceFacetBundle {
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
  if (options?.kind) {
    params.set('kind', options.kind);
  }
  if (options?.sourceType) {
    params.set('sourceType', options.sourceType);
  }
  if (options?.sourceAdapter) {
    params.set('sourceAdapter', options.sourceAdapter);
  }
  const query = params.toString();
  return query ? `?${query}` : '';
};

const fetchFacet = async <T>(url: string): Promise<T> =>
  apiFetchJSON<T>(url, {
    cache: 'no-store',
  });

export class ResourceAPI {
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
    return fetchFacet<ResourceFacetBundle>(
      `${buildFacetPath(resourceId, 'facets')}${buildTimelineQuery(options)}`,
    );
  }
}
