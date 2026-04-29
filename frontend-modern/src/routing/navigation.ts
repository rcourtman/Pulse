import { INFRASTRUCTURE_PATH, PATROL_PATH, WORKLOADS_PATH } from './resourceLinks';

export type AppTabId =
  | 'infrastructure'
  | 'workloads'
  | 'storage'
  | 'recovery'
  | 'alerts'
  | 'ai'
  | 'settings';

export type ActiveAppTabId = AppTabId | null;

export function getActiveTabForPath(path: string): ActiveAppTabId {
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
