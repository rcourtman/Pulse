import { apiFetchJSON } from '@/utils/apiClient';
import type { Role, UserRoleAssignment, Permission } from '@/types/rbac';

export const RBACAPI = {
  // Roles
  getRoles: () => apiFetchJSON<Role[]>('/api/admin/roles'),
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
  getUsers: () => apiFetchJSON<UserRoleAssignment[]>('/api/admin/users'),
  getUserAssignment: (username: string) =>
    apiFetchJSON<UserRoleAssignment>(`/api/admin/users/${encodeURIComponent(username)}/roles`),
  updateUserRoles: (username: string, roleIds: string[]) =>
    apiFetchJSON(`/api/admin/users/${encodeURIComponent(username)}/roles`, {
      method: 'PUT',
      body: JSON.stringify({ roleIds }),
    }),
  deleteUser: (username: string) =>
    apiFetchJSON(`/api/admin/users/${encodeURIComponent(username)}`, { method: 'DELETE' }),

  getUserPermissions: async (username: string): Promise<Permission[]> => {
    const permissions = await apiFetchJSON<unknown>(
      `/api/admin/users/${encodeURIComponent(username)}/permissions`,
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
