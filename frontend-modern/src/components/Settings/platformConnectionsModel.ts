export type PlatformConnectionsView = 'proxmox' | 'truenas';

export interface PlatformConnectionsTabDefinition {
  id: PlatformConnectionsView;
  label: string;
  path: string;
}

export const PLATFORM_CONNECTIONS_PREFIX = '/settings/infrastructure/platforms';
const PLATFORM_CONNECTIONS_PROXMOX_PREFIX = `${PLATFORM_CONNECTIONS_PREFIX}/proxmox`;
const PLATFORM_CONNECTIONS_TRUENAS_PREFIX = `${PLATFORM_CONNECTIONS_PREFIX}/truenas`;
const LEGACY_PROXMOX_PREFIX = '/settings/infrastructure/proxmox';
const LEGACY_PROXMOX_API_PREFIX = '/settings/infrastructure/api';
const LEGACY_TRUENAS_PREFIX = '/settings/infrastructure/truenas';

export const PLATFORM_CONNECTIONS_TABS: readonly PlatformConnectionsTabDefinition[] = [
  {
    id: 'proxmox',
    label: 'Proxmox',
    path: PLATFORM_CONNECTIONS_PROXMOX_PREFIX,
  },
  {
    id: 'truenas',
    label: 'TrueNAS',
    path: PLATFORM_CONNECTIONS_TRUENAS_PREFIX,
  },
];

export function getPlatformConnectionsViewFromPath(pathname: string): PlatformConnectionsView {
  if (pathname.startsWith(PLATFORM_CONNECTIONS_TRUENAS_PREFIX)) {
    return 'truenas';
  }
  if (pathname.startsWith(LEGACY_TRUENAS_PREFIX)) {
    return 'truenas';
  }
  if (pathname.startsWith(PLATFORM_CONNECTIONS_PREFIX)) {
    return 'proxmox';
  }
  if (
    pathname.startsWith(LEGACY_PROXMOX_PREFIX) ||
    pathname.startsWith(LEGACY_PROXMOX_API_PREFIX)
  ) {
    return 'proxmox';
  }
  return 'proxmox';
}

export function buildPlatformConnectionsPath(view: PlatformConnectionsView): string {
  return (
    PLATFORM_CONNECTIONS_TABS.find((tab) => tab.id === view)?.path ??
    PLATFORM_CONNECTIONS_PROXMOX_PREFIX
  );
}
