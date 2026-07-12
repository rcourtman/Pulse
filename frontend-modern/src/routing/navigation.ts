import {
  DOCKER_PATH,
  ACTIONS_PATH,
  KUBERNETES_PATH,
  PATROL_PATH,
  PROXMOX_PATH,
  STANDALONE_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
} from './resourceLinks';

export type AppTabId =
  | 'standalone'
  | 'proxmox'
  | 'docker'
  | 'kubernetes'
  | 'truenas'
  | 'vmware'
  | 'alerts'
  | 'actions'
  | 'ai'
  | 'settings';

export type ActiveAppTabId = AppTabId | null;

export function getActiveTabForPath(path: string): ActiveAppTabId {
  if (path.startsWith(PROXMOX_PATH)) return 'proxmox';
  if (path.startsWith(DOCKER_PATH)) return 'docker';
  if (path.startsWith(KUBERNETES_PATH)) return 'kubernetes';
  if (path.startsWith(TRUENAS_PATH)) return 'truenas';
  if (path.startsWith(VMWARE_PATH)) return 'vmware';
  if (path.startsWith(STANDALONE_PATH)) return 'standalone';
  if (path.startsWith('/alerts')) return 'alerts';
  if (path.startsWith(ACTIONS_PATH)) return 'actions';
  if (path.startsWith(PATROL_PATH)) return 'ai';
  if (path.startsWith('/settings')) return 'settings';
  return null;
}
