import { apiFetchJSON } from '@/utils/apiClient';
import type { Role, UserRoleAssignment, Permission } from '@/types/rbac';
import type { RBACAdminRequestOptions } from '@/types/api';

const readRBAC = <T>(url: string, options?: RBACAdminRequestOptions) =>
  options === undefined ? apiFetchJSON<T>(url) : apiFetchJSON<T>(url, options);

export const RBACAPI = {
  // Roles
  getRoles: (options?: RBACAdminRequestOptions) => readRBAC<Role[]>('/api/admin/roles', options),
  getRole: (id: string) => apiFetchJSON<Role>(`/api/admin/roles/${encodeURIComponent(id)}`),
  saveRole: (role: Role) => {
    const method = role.createdAt ? 'PUT' : 'POST';
    const url =
      method === 'PUT' ? `/api/admin/roles/${encodeURIComponent(role.id)}` : '/api/admin/roles';
    return apiFetchJSON<Role>(url, {
      method,
      body: JSON.stringify(role),
    });
  },
  deleteRole: (id: string) =>
    apiFetchJSON(`/api/admin/roles/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  // User Assignments
  getUsers: (options?: RBACAdminRequestOptions) =>
    readRBAC<UserRoleAssignment[]>('/api/admin/users', options),
  getUserAssignment: (username: string) =>
    apiFetchJSON<UserRoleAssignment>(`/api/admin/users/${encodeURIComponent(username)}/roles`),
  updateUserRoles: (username: string, roleIds: string[], options?: RBACAdminRequestOptions) =>
    apiFetchJSON(`/api/admin/users/${encodeURIComponent(username)}/roles`, {
      ...options,
      method: 'PUT',
      body: JSON.stringify({ roleIds }),
    }),
  deleteUser: (username: string, options?: RBACAdminRequestOptions) =>
    apiFetchJSON(`/api/admin/users/${encodeURIComponent(username)}`, {
      ...options,
      method: 'DELETE',
    }),

  getUserPermissions: async (
    username: string,
    options?: RBACAdminRequestOptions,
  ): Promise<Permission[]> => {
    const permissions = await readRBAC<unknown>(
      `/api/admin/users/${encodeURIComponent(username)}/permissions`,
      options,
    );
    if (
      !Array.isArray(permissions) ||
      !permissions.every(
        (permission) =>
          permission !== null &&
          typeof permission === 'object' &&
          !Array.isArray(permission) &&
          typeof permission.action === 'string' &&
          permission.action.trim().length > 0 &&
          typeof permission.resource === 'string' &&
          permission.resource.trim().length > 0,
      )
    ) {
      throw new Error('Invalid saved permissions response.');
    }
    return permissions;
  },
};
