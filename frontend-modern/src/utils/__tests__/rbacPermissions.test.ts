import { describe, expect, it } from 'vitest';
import {
  createDefaultRBACPermission,
  RBAC_PERMISSION_ACTIONS,
  RBAC_PERMISSION_RESOURCES,
} from '@/utils/rbacPermissions';

describe('rbacPermissions', () => {
  it('exports canonical RBAC permission actions and resources', () => {
    expect(RBAC_PERMISSION_ACTIONS).toEqual(['read', 'write', 'delete', 'admin', '*']);
    expect(RBAC_PERMISSION_RESOURCES).toEqual([
      'settings',
      'audit_logs',
      'nodes',
      'users',
      'license',
      '*',
    ]);
  });

  it('creates the canonical default RBAC permission', () => {
    expect(createDefaultRBACPermission()).toEqual({ action: 'read', resource: 'nodes' });
  });
});
