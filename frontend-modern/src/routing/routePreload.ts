import {
  buildInfrastructurePath,
  buildProxmoxPath,
  buildRecoveryPath,
  buildStoragePath,
  buildWorkloadsPath,
  DOCKER_PATH,
  KUBERNETES_PATH,
  PATROL_PATH,
  PROXMOX_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
} from '@/routing/resourceLinks';

type RoutePreloader = {
  id: string;
  matches: (route: string) => boolean;
  preload: () => Promise<void>;
};

const ROOT_INFRASTRUCTURE_PATH = buildInfrastructurePath();
const ROOT_PROXMOX_PATH = buildProxmoxPath();
const ROOT_WORKLOADS_PATH = buildWorkloadsPath();
const STORAGE_PATH = buildStoragePath();
const RECOVERY_ROUTE_PATH = buildRecoveryPath();
const ALERTS_PATH = '/alerts';
const SETTINGS_PATH = '/settings';
const routePreloadCache = new Map<string, Promise<void>>();

export const APP_SHELL_ROUTE_PRELOAD_PATHS = [
  ROOT_PROXMOX_PATH,
  ROOT_WORKLOADS_PATH,
  RECOVERY_ROUTE_PATH,
  PATROL_PATH,
  ALERTS_PATH,
  STORAGE_PATH,
  SETTINGS_PATH,
] as const;

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
    id: 'proxmox',
    matches: (route) => route === PROXMOX_PATH || route.startsWith(`${PROXMOX_PATH}/`),
    preload: () =>
      import('@/pages/Proxmox').then(() => undefined),
  },
  {
    id: 'docker',
    matches: (route) => route === DOCKER_PATH || route.startsWith(`${DOCKER_PATH}/`),
    preload: () =>
      import('@/pages/Docker').then(() => undefined),
  },
  {
    id: 'kubernetes',
    matches: (route) => route === KUBERNETES_PATH || route.startsWith(`${KUBERNETES_PATH}/`),
    preload: () =>
      import('@/pages/Kubernetes').then(() => undefined),
  },
  {
    id: 'truenas',
    matches: (route) => route === TRUENAS_PATH || route.startsWith(`${TRUENAS_PATH}/`),
    preload: () =>
      import('@/pages/TrueNAS').then(() => undefined),
  },
  {
    id: 'vmware',
    matches: (route) => route === VMWARE_PATH || route.startsWith(`${VMWARE_PATH}/`),
    preload: () =>
      import('@/pages/Vmware').then(() => undefined),
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
      import('@/pages/Workloads').then(() => undefined),
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
      import('@/pages/Recovery').then(() => undefined),
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
