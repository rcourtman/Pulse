import type { OrganizationRole } from '@/api/orgs';

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
  { value: 'viewer', label: 'All members (Viewer)' },
  { value: 'editor', label: 'Editor or higher' },
  { value: 'admin', label: 'Admin or Owner' },
];

export function getOrganizationShareRoleOption(role: unknown) {
  // A missing or unsupported saved threshold is not evidence of a Viewer share.
  return ORGANIZATION_SHARE_ROLE_OPTIONS.find((option) => option.value === role);
}
