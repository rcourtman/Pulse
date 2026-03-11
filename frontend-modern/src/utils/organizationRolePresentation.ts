import type { OrganizationRole } from '@/api/orgs';
import { normalizeRole } from '@/utils/orgUtils';

export type ShareAccessRole = 'viewer' | 'editor' | 'admin';

export const ORGANIZATION_MEMBER_ROLE_OPTIONS: Array<{
  value: OrganizationRole;
  label: string;
}> = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'editor', label: 'Editor' },
  { value: 'admin', label: 'Admin' },
  { value: 'owner', label: 'Owner' },
];

export const ORGANIZATION_SHARE_ROLE_OPTIONS: Array<{
  value: ShareAccessRole;
  label: string;
}> = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'editor', label: 'Editor' },
  { value: 'admin', label: 'Admin' },
];

export function normalizeOrganizationShareRole(role: OrganizationRole): ShareAccessRole {
  const normalized = normalizeRole(role);
  if (normalized === 'admin' || normalized === 'editor') return normalized;
  return 'viewer';
}
