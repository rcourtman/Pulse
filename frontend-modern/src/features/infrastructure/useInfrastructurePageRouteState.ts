import { createEffect, createSignal, onCleanup, type Accessor, type Setter, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import type { Resource } from '@/types/resource';
import { normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';
import {
  buildInfrastructurePath,
  INFRASTRUCTURE_PATH,
  INFRASTRUCTURE_QUERY_PARAMS,
  parseInfrastructureLinkSearch,
} from '@/routing/resourceLinks';
import { areSearchParamsEquivalent } from '@/utils/searchParams';

interface InfrastructurePageRouteStateOptions {
  resources: Accessor<Resource[]>;
  filteredResources: Accessor<Resource[]>;
  initialLoadComplete: Accessor<boolean>;
  selectedSource: Accessor<string>;
  setSelectedSource: Setter<string>;
  searchQuery: Accessor<string>;
  setSearchQuery: Setter<string>;
}

export function useInfrastructurePageRouteState(options: InfrastructurePageRouteStateOptions) {
  const {
    resources,
    filteredResources,
    initialLoadComplete,
    selectedSource,
    setSelectedSource,
    searchQuery,
    setSearchQuery,
  } = options;
  const location = useLocation();
  const navigate = useNavigate();

  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);
  const [hoveredResourceId, setHoveredResourceId] = createSignal<string | null>(null);
  const [highlightedResourceId, setHighlightedResourceId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  const [handledSourceParam, setHandledSourceParam] = createSignal<string | null>(null);
  const [handledQueryParam, setHandledQueryParam] = createSignal('');

  let highlightTimer: number | undefined;
  let pendingUrlSyncHandle: number | null = null;
  let pendingUrlSyncPath: string | null = null;

  const scheduleUrlSyncNavigate = (nextPath: string) => {
    pendingUrlSyncPath = nextPath;
    if (pendingUrlSyncHandle !== null) return;
    pendingUrlSyncHandle = window.setTimeout(() => {
      pendingUrlSyncHandle = null;
      const target = pendingUrlSyncPath;
      pendingUrlSyncPath = null;
      if (!target) return;
      const current = `${untrack(() => location.pathname)}${untrack(() => location.search)}`;
      if (current === target) return;
      navigate(target, { replace: true });
    }, 0);
  };

  createEffect(() => {
    const { resource: resourceId } = parseInfrastructureLinkSearch(location.search);
    if (!resourceId || resourceId === handledResourceId()) return;
    const matching = resources().some((resource) => resource.id === resourceId);
    if (!matching) return;
    setExpandedResourceId(resourceId);
    setHighlightedResourceId(resourceId);
    setHandledResourceId(resourceId);
    if (highlightTimer) {
      window.clearTimeout(highlightTimer);
    }
    highlightTimer = window.setTimeout(() => {
      setHighlightedResourceId(null);
    }, 2000);
  });

  createEffect(() => {
    const { resource: resourceId } = parseInfrastructureLinkSearch(location.search);
    if (resourceId) return;
    if (handledResourceId() === null) return;

    if (expandedResourceId() !== null) {
      setExpandedResourceId(null);
    }
    if (highlightedResourceId() !== null) {
      setHighlightedResourceId(null);
    }
    setHandledResourceId(null);
  });

  createEffect(() => {
    const { source: sourceParam } = parseInfrastructureLinkSearch(location.search);
    if (!sourceParam) {
      const previous = (handledSourceParam() ?? '').trim();
      if (previous) {
        if (selectedSource() !== '') setSelectedSource('');
        setHandledSourceParam('');
      } else if (handledSourceParam() === null) {
        setHandledSourceParam('');
      }
      return;
    }
    if (sourceParam === handledSourceParam()) return;
    const normalized = normalizeSourcePlatformKey(sourceParam) ?? '';
    setSelectedSource(normalized);
    setHandledSourceParam(sourceParam);
  });

  createEffect(() => {
    const { query: nextSearch } = parseInfrastructureLinkSearch(location.search);
    const normalized = nextSearch ?? '';
    if (normalized !== handledQueryParam()) {
      if (normalized !== untrack(searchQuery)) {
        setSearchQuery(normalized);
      }
      setHandledQueryParam(normalized);
    }
  });

  createEffect(() => {
    if (location.pathname !== INFRASTRUCTURE_PATH) return;

    const parsed = parseInfrastructureLinkSearch(location.search);
    const urlSource = parsed.source ?? '';
    const urlQuery = parsed.query ?? '';
    const urlResource = parsed.resource ?? '';
    if ((handledSourceParam() ?? '') !== urlSource) return;
    if (handledQueryParam() !== urlQuery) return;
    if (urlResource && handledResourceId() !== urlResource && !initialLoadComplete()) return;

    const nextSource = selectedSource();
    const nextQuery = searchQuery().trim();
    const currentLinkedResource = parsed.resource;
    const selectedResourceId = expandedResourceId();
    const shouldPreserveIncomingResource =
      !selectedResourceId && Boolean(currentLinkedResource) && !initialLoadComplete();
    const nextResource = shouldPreserveIncomingResource
      ? currentLinkedResource
      : (selectedResourceId ?? '');

    const managedPath = buildInfrastructurePath({
      source: nextSource || null,
      query: nextQuery || null,
      resource: nextResource || null,
    });
    const managedUrl = new URL(managedPath, 'http://pulse.local');
    const currentParams = new URLSearchParams(location.search);
    const nextParams = new URLSearchParams(location.search);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.source);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.query);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.resource);
    managedUrl.searchParams.forEach((value, key) => {
      nextParams.set(key, value);
    });

    if (!areSearchParamsEquivalent(currentParams, nextParams)) {
      const nextSearch = nextParams.toString();
      const nextPath = nextSearch ? `${INFRASTRUCTURE_PATH}?${nextSearch}` : INFRASTRUCTURE_PATH;
      scheduleUrlSyncNavigate(nextPath);
    }
  });

  createEffect(() => {
    const hoveredId = hoveredResourceId();
    if (!hoveredId) return;
    const exists = filteredResources().some((resource) => resource.id === hoveredId);
    if (!exists) {
      setHoveredResourceId(null);
    }
  });

  onCleanup(() => {
    if (pendingUrlSyncHandle !== null) {
      window.clearTimeout(pendingUrlSyncHandle);
      pendingUrlSyncHandle = null;
      pendingUrlSyncPath = null;
    }
    if (highlightTimer) {
      window.clearTimeout(highlightTimer);
    }
  });

  return {
    expandedResourceId,
    setExpandedResourceId,
    hoveredResourceId,
    setHoveredResourceId,
    highlightedResourceId,
  };
}

export type InfrastructurePageRouteState = ReturnType<typeof useInfrastructurePageRouteState>;
