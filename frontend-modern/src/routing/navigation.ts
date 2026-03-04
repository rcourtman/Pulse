import { INFRASTRUCTURE_PATH, WORKLOADS_PATH } from './resourceLinks';

export type AppTabId =
  | 'dashboard'
  | 'infrastructure'
  | 'workloads'
  | 'storage'
  | 'recovery'
  | 'alerts'
  | 'ai'
  | 'settings'
  | 'operations';

export function getActiveTabForPath(path: string): AppTabId {
  if (path.startsWith('/dashboard')) return 'dashboard';
  if (path.startsWith(INFRASTRUCTURE_PATH)) return 'infrastructure';
  if (path.startsWith(WORKLOADS_PATH)) return 'workloads';
  if (path.startsWith('/storage')) return 'storage';
  if (path.startsWith('/ceph')) return 'storage';
  if (path.startsWith('/recovery')) return 'recovery';
  if (path.startsWith('/alerts')) return 'alerts';
  if (path.startsWith('/ai')) return 'ai';
  if (path.startsWith('/settings')) return 'settings';
  if (path.startsWith('/operations')) return 'operations';
  return 'infrastructure';
}
