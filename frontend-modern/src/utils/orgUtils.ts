import type { Organization, OrganizationRole } from '@/api/orgs';

const defaultBadgeClass = 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300';

const roleBadgeClasses: Record<OrganizationRole, string> = {
  owner: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  admin: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  editor: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200',
  viewer: defaultBadgeClass,
  member: defaultBadgeClass,
};

export const normalizeRole = (role: OrganizationRole): Exclude<OrganizationRole, 'member'> => {
  if (role === 'member') return 'viewer';
  return role;
};

export const canManageOrg = (org: Organization | null, currentUser?: string): boolean => {
  if (!org || !currentUser) return false;
  if (org.ownerUserId === currentUser) return true;
  const role = normalizeRole(org.members?.find((member) => member.userId === currentUser)?.role ?? 'viewer');
  return role === 'admin' || role === 'owner';
};

export const roleBadgeClass = (role: string): string => {
  return roleBadgeClasses[role as OrganizationRole] ?? defaultBadgeClass;
};

export const formatOrgDate = (value?: string): string => {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};
