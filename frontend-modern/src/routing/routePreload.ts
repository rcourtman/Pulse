import {
  buildInfrastructurePath,
  buildRecoveryPath,
  buildStoragePath,
  buildWorkloadsPath,
  DASHBOARD_PATH,
  PATROL_PATH,
} from '@/routing/resourceLinks';

type RoutePreloader = {
  id: string;
  matches: (route: string) => boolean;
  preload: () => Promise<void>;
};

const ROOT_INFRASTRUCTURE_PATH = buildInfrastructurePath();
const ROOT_WORKLOADS_PATH = buildWorkloadsPath();
const STORAGE_PATH = buildStoragePath();
const RECOVERY_ROUTE_PATH = buildRecoveryPath();
const ALERTS_PATH = '/alerts';
const SETTINGS_PATH = '/settings';
const routePreloadCache = new Map<string, Promise<void>>();

function normalizeRoute(route: string): string {
  const [pathname] = route.split(/[?#]/, 1);
  if (!pathname) return '';
  if (pathname.length > 1 && pathname.endsWith('/')) {
    return pathname.slice(0, -1);
  }
  return pathname;
}

const ROUTE_PRELOADERS: readonly RoutePreloader[] = [
  {
    id: 'dashboard',
    matches: (route) => route === DASHBOARD_PATH,
    preload: () =>
      import('@/pages/Dashboard').then(() => undefined),
  },
  {
    id: 'infrastructure',
    matches: (route) => route === ROOT_INFRASTRUCTURE_PATH,
    preload: () =>
      import('@/pages/Infrastructure').then(() => undefined),
  },
  {
    id: 'workloads',
    matches: (route) => route === ROOT_WORKLOADS_PATH,
    preload: () =>
      import('@/components/Dashboard/Dashboard').then(() => undefined),
  },
  {
    id: 'storage',
    matches: (route) => route === STORAGE_PATH,
    preload: () =>
      import('@/pages/Storage').then(() => undefined),
  },
  {
    id: 'recovery',
    matches: (route) => route === RECOVERY_ROUTE_PATH,
    preload: () =>
      import('@/pages/RecoveryRoute').then(() => undefined),
  },
  {
    id: 'alerts',
    matches: (route) => route === ALERTS_PATH || route.startsWith(`${ALERTS_PATH}/`),
    preload: () =>
      import('@/pages/Alerts').then(() => undefined),
  },
  {
    id: 'patrol',
    matches: (route) => route === PATROL_PATH || route.startsWith(`${PATROL_PATH}/`),
    preload: () =>
      import('@/pages/AIIntelligence').then(() => undefined),
  },
  {
    id: 'settings',
    matches: (route) => route === SETTINGS_PATH || route.startsWith(`${SETTINGS_PATH}/`),
    preload: () =>
      import('@/components/Settings/Settings').then(() => undefined),
  },
] as const;

export async function preloadRouteModule(route: string): Promise<void> {
  const normalizedRoute = normalizeRoute(route);
  if (!normalizedRoute) return;

  const preloader = ROUTE_PRELOADERS.find((candidate) => candidate.matches(normalizedRoute));
  if (!preloader) return;

  const cached = routePreloadCache.get(preloader.id);
  if (cached) {
    return cached;
  }

  const promise = preloader.preload().catch((error) => {
    routePreloadCache.delete(preloader.id);
    throw error;
  });

  routePreloadCache.set(preloader.id, promise);
  await promise;
}
