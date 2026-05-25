import {
  AGENTS_PATH,
  DOCKER_PATH,
  KUBERNETES_PATH,
  PATROL_PATH,
  PROXMOX_PATH,
  RECOVERY_PATH,
  STORAGE_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
  WORKLOADS_PATH,
} from './resourceLinks';

export type AppTabId =
  | 'agents'
  | 'proxmox'
  | 'docker'
  | 'kubernetes'
  | 'truenas'
  | 'vmware'
  | 'workloads'
  | 'storage'
  | 'recovery'
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
  if (path.startsWith(WORKLOADS_PATH)) return 'workloads';
  if (path.startsWith(STORAGE_PATH)) return 'storage';
  if (path.startsWith(RECOVERY_PATH)) return 'recovery';
  if (path.startsWith('/alerts')) return 'alerts';
  if (path.startsWith(PATROL_PATH) || path.startsWith('/ai')) return 'ai';
  if (path.startsWith('/settings')) return 'settings';
  if (path.startsWith('/operations')) return 'settings';
  return null;
}
