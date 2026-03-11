import type { Permission } from '@/types/rbac';

export const RBAC_PERMISSION_ACTIONS = ['read', 'write', 'delete', 'admin', '*'] as const;

export const RBAC_PERMISSION_RESOURCES = [
  'settings',
  'audit_logs',
  'nodes',
  'users',
  'license',
  '*',
] as const;

export function createDefaultRBACPermission(): Permission {
  return { action: 'read', resource: 'nodes' };
}
