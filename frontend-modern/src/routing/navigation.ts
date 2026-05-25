import {
  AGENTS_PATH,
  DOCKER_PATH,
  KUBERNETES_PATH,
  PATROL_PATH,
  PROXMOX_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
} from './resourceLinks';

export type AppTabId =
  | 'agents'
  | 'proxmox'
  | 'docker'
  | 'kubernetes'
  | 'truenas'
  | 'vmware'
  | 'alerts'
  | 'ai'
  | 'settings';

export type ActiveAppTabId = AppTabId | null;

export function getActiveTabForPath(path: string): ActiveAppTabId {
  if (path.startsWith(PROXMOX_PATH)) return 'proxmox';
  if (path.startsWith(DOCKER_PATH)) return 'docker';
  if (path.startsWith(KUBERNETES_PATH)) return 'kubernetes';
  if (path.startsWith(TRUENAS_PATH)) return 'truenas';
  if (path.startsWith(VMWARE_PATH)) return 'vmware';
  if (path.startsWith(AGENTS_PATH)) return 'agents';
  if (path.startsWith('/alerts')) return 'alerts';
  if (path.startsWith(PATROL_PATH) || path.startsWith('/ai')) return 'ai';
  if (path.startsWith('/settings')) return 'settings';
  if (path.startsWith('/operations')) return 'settings';
  return null;
}
