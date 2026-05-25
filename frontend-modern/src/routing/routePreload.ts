import {
  buildProxmoxPath,
  buildStandalonePath,
  DOCKER_PATH,
  KUBERNETES_PATH,
  PATROL_PATH,
  PROXMOX_PATH,
  STANDALONE_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
} from '@/routing/resourceLinks';

type RoutePreloader = {
  id: string;
  matches: (route: string) => boolean;
  preload: () => Promise<void>;
};

const ROOT_PROXMOX_PATH = buildProxmoxPath();
const ROOT_STANDALONE_PATH = buildStandalonePath();
const ALERTS_PATH = '/alerts';
const SETTINGS_PATH = '/settings';
const routePreloadCache = new Map<string, Promise<void>>();

export const APP_SHELL_ROUTE_PRELOAD_PATHS = [
  ROOT_PROXMOX_PATH,
  ROOT_STANDALONE_PATH,
  PATROL_PATH,
  ALERTS_PATH,
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
    preload: () => import('@/pages/Proxmox').then(() => undefined),
  },
  {
    id: 'docker',
    matches: (route) => route === DOCKER_PATH || route.startsWith(`${DOCKER_PATH}/`),
    preload: () => import('@/pages/Docker').then(() => undefined),
  },
  {
    id: 'kubernetes',
    matches: (route) => route === KUBERNETES_PATH || route.startsWith(`${KUBERNETES_PATH}/`),
    preload: () => import('@/pages/Kubernetes').then(() => undefined),
  },
  {
    id: 'truenas',
    matches: (route) => route === TRUENAS_PATH || route.startsWith(`${TRUENAS_PATH}/`),
    preload: () => import('@/pages/TrueNAS').then(() => undefined),
  },
  {
    id: 'vmware',
    matches: (route) => route === VMWARE_PATH || route.startsWith(`${VMWARE_PATH}/`),
    preload: () => import('@/pages/Vmware').then(() => undefined),
  },
  {
    id: 'standalone',
    matches: (route) => route === STANDALONE_PATH || route.startsWith(`${STANDALONE_PATH}/`),
    preload: () => import('@/pages/Standalone').then(() => undefined),
  },
  {
    id: 'alerts',
    matches: (route) => route === ALERTS_PATH || route.startsWith(`${ALERTS_PATH}/`),
    preload: () => import('@/pages/Alerts').then(() => undefined),
  },
  {
    id: 'patrol',
    matches: (route) => route === PATROL_PATH || route.startsWith(`${PATROL_PATH}/`),
    preload: () => import('@/pages/AIIntelligence').then(() => undefined),
  },
  {
    id: 'settings',
    matches: (route) => route === SETTINGS_PATH || route.startsWith(`${SETTINGS_PATH}/`),
    preload: () => import('@/components/Settings/Settings').then(() => undefined),
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
