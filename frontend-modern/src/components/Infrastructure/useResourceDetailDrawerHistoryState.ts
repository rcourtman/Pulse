import { createMemo, createResource, createSignal } from 'solid-js';
import type {
  Resource,
  ResourceChangeKind,
  ResourceChangeSourceAdapter,
  ResourceChangeSourceType,
} from '@/types/resource';
import type { ResourceIntelligence } from '@/types/aiIntelligence';
import { AIAPI } from '@/api/ai';
import { ResourceAPI } from '@/api/resources';

interface UseResourceDetailDrawerHistoryStateOptions {
  resource: Resource;
}

export const useResourceDetailDrawerHistoryState = (
  options: UseResourceDetailDrawerHistoryStateOptions,
) => {
  const { resource } = options;

  const resourceFacetId = createMemo(() => resource.id.trim());
  const [timelineKindFilter, setTimelineKindFilter] = createSignal<ResourceChangeKind | ''>('');
  const [timelineSourceTypeFilter, setTimelineSourceTypeFilter] = createSignal<
    ResourceChangeSourceType | ''
  >('');
  const [timelineSourceAdapterFilter, setTimelineSourceAdapterFilter] = createSignal<
    ResourceChangeSourceAdapter | ''
  >('');

  const resourceFacetRequest = createMemo(() => {
    const id = resourceFacetId();
    return id ? { id } : null;
  });

  const [resourceFacets, { refetch: refetchResourceFacets }] = createResource(
    resourceFacetRequest,
    async (request) => {
      if (!request?.id) return null;
      return ResourceAPI.getFacetBundle(request.id, { limit: 25 });
    },
    { initialValue: null },
  );

  const [resourceIntelligence] = createResource(
    resourceFacetRequest,
    async (request) => {
      if (!request?.id) return null;
      return AIAPI.getResourceIntelligence(request.id);
    },
    { initialValue: null as ResourceIntelligence | null },
  );

  const timelineFacetRequest = createMemo(() => {
    const id = resourceFacetId();
    if (!id) return null;
    const kind = timelineKindFilter();
    const sourceType = timelineSourceTypeFilter();
    const sourceAdapter = timelineSourceAdapterFilter();
    if (!kind && !sourceType && !sourceAdapter) return null;
    return { id, kind, sourceType, sourceAdapter };
  });

  const [timelineFacets, { refetch: refetchTimelineFacets }] = createResource(
    timelineFacetRequest,
    async (request) => {
      if (!request) return null;
      return ResourceAPI.getFacetBundle(request.id, {
        limit: 25,
        kind: request.kind || undefined,
        sourceType: request.sourceType || undefined,
        sourceAdapter: request.sourceAdapter || undefined,
      });
    },
    { initialValue: null },
  );

  const resourceTimeline = createMemo(
    () => resourceFacets()?.recentChanges ?? resource.recentChanges ?? [],
  );
  const resourceFacetCounts = createMemo(() => resourceFacets()?.counts ?? resource.facetCounts ?? null);
  const historyFacetBundle = createMemo(() =>
    timelineFacetRequest() ? (timelineFacets() ?? resourceFacets()) : resourceFacets(),
  );
  const historyFacetCounts = createMemo(
    () => historyFacetBundle()?.counts ?? resourceFacetCounts() ?? null,
  );
  const historyRecentChanges = createMemo(() => historyFacetBundle()?.recentChanges ?? resourceTimeline());
  const historyTimeline = createMemo(() => historyRecentChanges());
  const hasTimelineFilters = createMemo(() =>
    Boolean(timelineKindFilter() || timelineSourceTypeFilter() || timelineSourceAdapterFilter()),
  );
  const historyLoadingLabel = createMemo(() => {
    if (timelineFacetRequest()) {
      return timelineFacets.loading ? 'Refreshing filtered changes...' : 'Filtered changes loaded';
    }
    return resourceFacets.loading ? 'Refreshing changes...' : 'Changes loaded';
  });
  const resourceTimelineCount = createMemo(
    () => historyFacetCounts()?.recentChanges ?? historyRecentChanges().length,
  );
  const sortedResourceTimeline = createMemo(() =>
    [...historyTimeline()].sort((left, right) => {
      const leftTime = Date.parse(left.observedAt || '');
      const rightTime = Date.parse(right.observedAt || '');
      return (
        (Number.isFinite(rightTime) ? rightTime : 0) - (Number.isFinite(leftTime) ? leftTime : 0)
      );
    }),
  );
  const facetBundleError = createMemo(() => {
    const error = timelineFacetRequest() ? timelineFacets.error : resourceFacets.error;
    if (!error) return '';
    return (error as Error)?.message || 'Failed to load resource history';
  });

  const refetchHistoryFacets = () => {
    if (timelineFacetRequest()) {
      return refetchTimelineFacets();
    }
    return refetchResourceFacets();
  };

  return {
    timelineKindFilter,
    setTimelineKindFilter,
    timelineSourceTypeFilter,
    setTimelineSourceTypeFilter,
    timelineSourceAdapterFilter,
    setTimelineSourceAdapterFilter,
    resourceIntelligence,
    resourceTimeline,
    historyFacetCounts,
    historyRecentChanges,
    hasTimelineFilters,
    historyLoadingLabel,
    resourceTimelineCount,
    sortedResourceTimeline,
    facetBundleError,
    refetchHistoryFacets,
  };
};

export type ResourceDetailDrawerHistoryState = ReturnType<typeof useResourceDetailDrawerHistoryState>;
