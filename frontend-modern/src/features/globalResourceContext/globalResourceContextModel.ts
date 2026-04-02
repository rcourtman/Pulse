import { DASHBOARD_PATH } from '@/routing/resourceLinks';
import {
  buildInfrastructurePath,
  buildInfrastructureResourceHref,
  buildPathWithGlobalResourceContext,
  buildRecoveryHrefForResource,
  buildRecoveryPath,
  buildStorageHrefForResource,
  buildStoragePath,
  buildWorkloadsHrefForResource,
  buildWorkloadsPath,
  stripGlobalResourceContextFromPath,
} from '@/routing/resourceLinks';
import type { Resource } from '@/types/resource';
import { areSearchParamsEquivalent } from '@/utils/searchParams';

export type GlobalResourceContextSurface =
  | 'dashboard'
  | 'infrastructure'
  | 'workloads'
  | 'storage'
  | 'recovery';

const normalizePath = (path: string): string => {
  const url = new URL(path, 'http://pulse.local');
  const search = url.searchParams.toString();
  return `${url.pathname}${search ? `?${search}` : ''}`;
};

export const resolveGlobalResourceContextSurface = (
  pathname: string,
): GlobalResourceContextSurface | null => {
  if (pathname.startsWith('/dashboard')) return 'dashboard';
  if (pathname.startsWith('/infrastructure')) return 'infrastructure';
  if (pathname.startsWith('/workloads')) return 'workloads';
  if (pathname.startsWith('/storage') || pathname.startsWith('/ceph')) return 'storage';
  if (pathname.startsWith('/recovery')) return 'recovery';
  return null;
};

export const buildGlobalResourceContextBasePath = (
  surface: GlobalResourceContextSurface,
): string => {
  switch (surface) {
    case 'dashboard':
      return DASHBOARD_PATH;
    case 'infrastructure':
      return buildInfrastructurePath();
    case 'workloads':
      return buildWorkloadsPath();
    case 'storage':
      return buildStoragePath();
    case 'recovery':
      return buildRecoveryPath();
  }
};

export const buildGlobalResourceContextScopedPath = (
  surface: GlobalResourceContextSurface,
  resource: Resource | null | undefined,
): string => {
  if (!resource) {
    return buildGlobalResourceContextBasePath(surface);
  }

  switch (surface) {
    case 'dashboard':
      return DASHBOARD_PATH;
    case 'infrastructure':
      return buildInfrastructureResourceHref(resource.id) ?? buildInfrastructurePath();
    case 'workloads':
      return buildWorkloadsHrefForResource(resource) ?? buildWorkloadsPath();
    case 'storage':
      return buildStorageHrefForResource(resource) ?? buildStoragePath();
    case 'recovery':
      return buildRecoveryHrefForResource(resource) ?? buildRecoveryPath();
  }
};

export const buildSurfacePathWithGlobalResourceContext = (
  surface: GlobalResourceContextSurface,
  resource: Resource | null | undefined,
): string =>
  buildPathWithGlobalResourceContext(
    buildGlobalResourceContextScopedPath(surface, resource),
    resource?.id ?? null,
  );

export const areGlobalResourceContextPathsEquivalent = (
  leftPath: string,
  rightPath: string,
): boolean => {
  const leftUrl = new URL(stripGlobalResourceContextFromPath(leftPath), 'http://pulse.local');
  const rightUrl = new URL(stripGlobalResourceContextFromPath(rightPath), 'http://pulse.local');
  if (leftUrl.pathname !== rightUrl.pathname) {
    return false;
  }
  return areSearchParamsEquivalent(leftUrl.searchParams, rightUrl.searchParams);
};

export const resolveGlobalResourceContextClearTarget = (options: {
  currentPath: string;
  resource: Resource | null | undefined;
}): string | null => {
  const surface = resolveGlobalResourceContextSurface(
    new URL(options.currentPath, 'http://pulse.local').pathname,
  );
  if (!surface) {
    const stripped = stripGlobalResourceContextFromPath(options.currentPath);
    return normalizePath(stripped) === normalizePath(options.currentPath) ? null : stripped;
  }

  const strippedCurrentPath = stripGlobalResourceContextFromPath(options.currentPath);
  const scopedPath = buildGlobalResourceContextScopedPath(surface, options.resource);
  if (areGlobalResourceContextPathsEquivalent(strippedCurrentPath, scopedPath)) {
    return buildGlobalResourceContextBasePath(surface);
  }

  return normalizePath(strippedCurrentPath) === normalizePath(options.currentPath)
    ? null
    : strippedCurrentPath;
};
