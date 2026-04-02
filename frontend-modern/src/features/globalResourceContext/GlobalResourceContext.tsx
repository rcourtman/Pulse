import {
  createContext,
  createEffect,
  createMemo,
  onCleanup,
  untrack,
  useContext,
  type Accessor,
  type JSX,
} from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import {
  buildPathWithGlobalResourceContext,
  parseGlobalResourceContextSearch,
} from '@/routing/resourceLinks';
import { useStorageRecoveryResources, useUnifiedResources } from '@/hooks/useUnifiedResources';
import type { Resource } from '@/types/resource';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import { createRouteStateNavigateScheduler } from '@/utils/routeStateNavigation';
import {
  buildGlobalResourceContextBasePath,
  buildSurfacePathWithGlobalResourceContext,
  resolveGlobalResourceContextClearTarget,
  resolveGlobalResourceContextSurface,
} from './globalResourceContextModel';

export interface GlobalResourceContextValue {
  canPinResource: (resource: Resource | null | undefined) => boolean;
  canPinResourceId: (resourceId: string | null | undefined) => boolean;
  clearGlobalResourceContext: () => void;
  contextLabel: Accessor<string | null>;
  contextResource: Accessor<Resource | null>;
  contextResourceId: Accessor<string | null>;
  hasGlobalResourceContext: Accessor<boolean>;
  isPinnedGlobalResource: (resourceId: string | null | undefined) => boolean;
  setGlobalResourceContext: (resource: Resource | null | undefined) => void;
  setGlobalResourceContextById: (resourceId: string | null | undefined) => void;
  buildPlatformRoute: (
    surface: 'dashboard' | 'infrastructure' | 'workloads' | 'storage' | 'recovery',
  ) => string;
}

const EMPTY_CONTEXT_ACCESSOR = () => null;
const EMPTY_FALSE_ACCESSOR = () => false;

const DEFAULT_GLOBAL_RESOURCE_CONTEXT: GlobalResourceContextValue = {
  canPinResource: () => false,
  canPinResourceId: () => false,
  clearGlobalResourceContext: () => undefined,
  contextLabel: EMPTY_CONTEXT_ACCESSOR,
  contextResource: EMPTY_CONTEXT_ACCESSOR,
  contextResourceId: EMPTY_CONTEXT_ACCESSOR,
  hasGlobalResourceContext: EMPTY_FALSE_ACCESSOR,
  isPinnedGlobalResource: () => false,
  setGlobalResourceContext: () => undefined,
  setGlobalResourceContextById: () => undefined,
  buildPlatformRoute: (surface) => buildGlobalResourceContextBasePath(surface),
};

const GlobalResourceContext = createContext<GlobalResourceContextValue>(
  DEFAULT_GLOBAL_RESOURCE_CONTEXT,
);

export const useGlobalResourceContext = (): GlobalResourceContextValue =>
  useContext(GlobalResourceContext) ?? DEFAULT_GLOBAL_RESOURCE_CONTEXT;

export function GlobalResourceContextProvider(props: { children: JSX.Element }) {
  const location = useLocation();
  const navigate = useNavigate();
  const routeStateNavigate = createRouteStateNavigateScheduler(
    navigate,
    () => `${untrack(() => location.pathname)}${untrack(() => location.search || '')}`,
  );

  const shellResources = useUnifiedResources();
  const storageRecoveryResources = useStorageRecoveryResources();

  const resourcesById = createMemo(() => {
    const resources = new Map<string, Resource>();
    for (const resource of shellResources.resources() || []) {
      if (resource?.id) resources.set(resource.id, resource);
    }
    for (const resource of storageRecoveryResources.resources() || []) {
      if (resource?.id) resources.set(resource.id, resource);
    }
    return resources;
  });

  const contextResourceId = createMemo<string | null>(() => {
    const parsed = parseGlobalResourceContextSearch(location.search || '');
    return parsed.resource || null;
  });

  const contextResource = createMemo<Resource | null>(() => {
    const resourceId = contextResourceId();
    if (!resourceId) return null;
    return resourcesById().get(resourceId) ?? null;
  });

  const contextLabel = createMemo(() => {
    const resource = contextResource();
    if (resource) {
      return getPreferredResourceDisplayName(resource) || resource.id;
    }
    return contextResourceId();
  });

  const hasGlobalResourceContext = createMemo(() => Boolean(contextResourceId()));

  const scheduleCurrentPathWithContext = (resourceId: string | null) => {
    const currentPath = `${location.pathname}${location.search || ''}`;
    const nextPath = buildPathWithGlobalResourceContext(currentPath, resourceId);
    if (nextPath === currentPath) {
      return;
    }
    routeStateNavigate.schedule(nextPath);
  };

  const clearGlobalResourceContext = () => {
    const currentPath = `${location.pathname}${location.search || ''}`;
    const nextPath = resolveGlobalResourceContextClearTarget({
      currentPath,
      resource: contextResource(),
    });
    if (!nextPath || nextPath === currentPath) {
      return;
    }
    routeStateNavigate.schedule(nextPath);
  };

  const setGlobalResourceContext = (resource: Resource | null | undefined) => {
    scheduleCurrentPathWithContext(resource?.id ?? null);
  };
  const setGlobalResourceContextById = (resourceId: string | null | undefined) => {
    const normalizedResourceId = resourceId?.trim() || null;
    if (!normalizedResourceId || !resourcesById().has(normalizedResourceId)) {
      scheduleCurrentPathWithContext(null);
      return;
    }
    scheduleCurrentPathWithContext(normalizedResourceId);
  };

  const buildPlatformRoute: GlobalResourceContextValue['buildPlatformRoute'] = (surface) => {
    const resource = contextResource();
    if (!resource) {
      return buildGlobalResourceContextBasePath(surface);
    }
    return buildSurfacePathWithGlobalResourceContext(surface, resource);
  };

  const canPinResource = (resource: Resource | null | undefined): boolean =>
    Boolean(resource?.id?.trim() && resourcesById().has(resource.id.trim()));
  const canPinResourceId = (resourceId: string | null | undefined): boolean =>
    Boolean(resourceId?.trim() && resourcesById().has(resourceId.trim()));

  const isPinnedGlobalResource = (resourceId: string | null | undefined): boolean =>
    Boolean(resourceId && contextResourceId() === resourceId.trim());

  createEffect(() => {
    const resource = contextResource();
    if (!resource) {
      return;
    }
    const currentPath = `${location.pathname}${location.search || ''}`;
    const surface = resolveGlobalResourceContextSurface(location.pathname);
    if (!surface) {
      return;
    }

    const targetBasePath = buildGlobalResourceContextBasePath(surface);
    const pathWithoutContext = buildPathWithGlobalResourceContext(currentPath, null);
    if (pathWithoutContext !== targetBasePath) {
      return;
    }

    const nextPath = buildPlatformRoute(surface);
    if (nextPath !== currentPath) {
      routeStateNavigate.schedule(nextPath);
    }
  });

  onCleanup(() => {
    routeStateNavigate.cleanup();
  });

  const value: GlobalResourceContextValue = {
    canPinResource,
    canPinResourceId,
    clearGlobalResourceContext,
    contextLabel,
    contextResource,
    contextResourceId,
    hasGlobalResourceContext,
    isPinnedGlobalResource,
    setGlobalResourceContext,
    setGlobalResourceContextById,
    buildPlatformRoute,
  };

  return (
    <GlobalResourceContext.Provider value={value}>{props.children}</GlobalResourceContext.Provider>
  );
}
