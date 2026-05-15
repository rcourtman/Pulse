import {
  DOCKER_PATH,
  INFRASTRUCTURE_PATH,
  KUBERNETES_PATH,
  PATROL_PATH,
  PROXMOX_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
  WORKLOADS_PATH,
} from './resourceLinks';

export type AppTabId =
  | 'proxmox'
  | 'docker'
  | 'kubernetes'
  | 'truenas'
  | 'vmware'
  | 'infrastructure'
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
  if (path.startsWith(INFRASTRUCTURE_PATH)) return 'infrastructure';
  if (path.startsWith(WORKLOADS_PATH)) return 'workloads';
  if (path.startsWith('/storage')) return 'storage';
  if (path.startsWith('/ceph')) return 'storage';
  if (path.startsWith('/recovery')) return 'recovery';
  if (path.startsWith('/alerts')) return 'alerts';
  if (path.startsWith(PATROL_PATH) || path.startsWith('/ai')) return 'ai';
  if (path.startsWith('/settings')) return 'settings';
  if (path.startsWith('/operations')) return 'settings';
  return null;
}
