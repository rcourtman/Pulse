import { describe, expect, it, vi, beforeEach } from 'vitest';
import { RBACAPI } from '../rbac';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('RBACAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getRoles', () => {
    it('fetches all roles', async () => {
      const mockRoles = [{ id: 'role-1', name: 'Admin' }];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockRoles);

      const result = await RBACAPI.getRoles();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/roles');
      expect(result).toEqual(mockRoles);
    });
  });

  describe('getRole', () => {
    it('fetches a single role by id', async () => {
      const mockRole = { id: 'role-1', name: 'Admin' };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockRole);

      const result = await RBACAPI.getRole('role-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/roles/role-1');
      expect(result).toEqual(mockRole);
    });

    it('encodes special characters in role id', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({});

      await RBACAPI.getRole('role/1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/roles/role%2F1');
    });
  });

  describe('saveRole', () => {
    it('uses POST for new role', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'new-role', name: 'New Role' });

      await RBACAPI.saveRole({ id: 'new-role', name: 'New Role' } as any);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/admin/roles',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    it('uses PUT for existing role', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'role-1', name: 'Updated' });

      await RBACAPI.saveRole({ id: 'role-1', name: 'Updated', createdAt: '2024-01-01' } as any);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/admin/roles/role-1',
        expect.objectContaining({ method: 'PUT' }),
      );
    });
  });

  describe('deleteRole', () => {
    it('deletes a role', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await RBACAPI.deleteRole('role-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/admin/roles/role-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });

  describe('getUsers', () => {
    it('fetches all users', async () => {
      const mockUsers = [{ userId: 'user-1', role: 'admin' }];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockUsers);

      const result = await RBACAPI.getUsers();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/users');
      expect(result).toEqual(mockUsers);
    });
  });

  describe('getUserAssignment', () => {
    it('fetches user role assignment', async () => {
      const mockAssignment = { userId: 'user-1', role: 'admin' };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockAssignment);

      const result = await RBACAPI.getUserAssignment('user-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/users/user-1/roles');
      expect(result).toEqual(mockAssignment);
    });
  });

  describe('updateUserRoles', () => {
    it('updates user roles', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      await RBACAPI.updateUserRoles('user-1', ['role-1', 'role-2']);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/admin/users/user-1/roles',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ roleIds: ['role-1', 'role-2'] }),
        }),
      );
    });
  });

  describe('getUserPermissions', () => {
    it('fetches user permissions', async () => {
      const mockPermissions = [{ name: 'read:alerts' }];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockPermissions);

      const result = await RBACAPI.getUserPermissions('user-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/users/user-1/permissions');
      expect(result).toEqual(mockPermissions);
    });
  });
});
