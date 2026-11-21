import type { Permission } from '@/types/rbac';

export interface RBACFeatureGateCopy {
  title: string;
  body: string;
}

export interface RBACFeatureGateCopyOptions {
  showCommercialCopy?: boolean;
}

export interface UserAssignmentsEmptyStateCopy {
  title: string;
  body: string;
  ssoHint: string;
  syncHint: string;
}

interface UserIdentityPresentation {
  username: string;
  displayName?: string;
  email?: string;
  providerType?: string;
  providerId?: string;
}

interface RoleIdentityPresentation {
  id: string;
  name: string;
}

export function getRoleIdentityLabel(role: RoleIdentityPresentation): string {
  // Names are mutable and need not be unique. Never normalize the saved key.
  return role.name.trim() && role.name !== role.id
    ? `${role.name} (Role ID: ${role.id})`
    : `Role ID: ${role.id}`;
}

export function getRoleChangeSuccessMessage(role: RoleIdentityPresentation, deleting: boolean) {
  return `Role ${getRoleIdentityLabel(role)} ${deleting ? 'deleted' : 'saved'}`;
}

export function getRoleChangeUnconfirmedMessage(role: RoleIdentityPresentation, deleting: boolean) {
  return `The ${deleting ? 'deletion' : 'save'} of role ${getRoleIdentityLabel(role)} could not be confirmed. It may already have been applied. Check saved roles and review their permissions before making another change.`;
}

export function getUserIdentityDisplayName(user: UserIdentityPresentation): string {
  return user.displayName?.trim() || user.email?.trim() || user.username;
}

export function getUserIdentityLabel(user: UserIdentityPresentation): string {
  const name = getUserIdentityDisplayName(user);
  return name === user.username ? name : `${name} (${user.username})`;
}

export function getUserIdentityProviderLabel(user: UserIdentityPresentation): string {
  const providerType = user.providerType?.trim().toUpperCase();
  const providerId = user.providerId?.trim();
  return [providerType, providerId].filter(Boolean).join(' · ');
}

export function getRBACFeatureGateCopy(
  kind: 'roles' | 'user-assignments',
  options: RBACFeatureGateCopyOptions = {},
): RBACFeatureGateCopy {
  const showCommercialCopy = options.showCommercialCopy !== false;
  if (kind === 'roles') {
    return {
      title: 'Custom Roles',
      body: showCommercialCopy
        ? 'Define granular permissions and custom access tiers on paid self-hosted and hosted plans.'
        : 'Define granular permissions and custom access tiers when RBAC is enabled for this instance.',
    };
  }

  return {
    title: 'Centralized Access Control',
    body: showCommercialCopy
      ? 'Assign multi-tier roles to users and manage infrastructure-wide security policies on paid self-hosted and hosted plans.'
      : 'Assign roles to users and review access policy when RBAC is enabled for this instance.',
  };
}

export function getUserAssignmentsEmptyStateCopy(): UserAssignmentsEmptyStateCopy {
  return {
    title: 'No users yet',
    body: "Users appear here automatically when they sign in via SSO (OIDC/SAML) or proxy authentication. Once they've logged in, you can assign roles to control their access.",
    ssoHint: 'Configure SSO in Security settings',
    syncHint: 'Users sync on first login',
  };
}

export function getRolesEmptyState(): string {
  return 'No roles available.';
}

export function getRolesLoadErrorMessage(): string {
  return 'Failed to load roles';
}

export function getRolesDeleteErrorMessage(): string {
  return 'Failed to delete role';
}

export function getRolesRequiredFieldsMessage(): string {
  return 'ID and Name are required';
}

export function getRolesSaveErrorMessage(): string {
  return 'Failed to save role';
}

export function getUserAssignmentsLoadErrorMessage(): string {
  return 'Failed to load user assignments';
}

function hasCompleteRoleInventory(roles: unknown[], requireRules: boolean): boolean {
  const ids = new Set<string>();
  return roles.every((entry) => {
    const role = entry as {
      id?: unknown;
      name?: unknown;
      description?: unknown;
      isBuiltIn?: unknown;
      permissions?: unknown;
    } | null;
    if (
      !role ||
      typeof role.id !== 'string' ||
      !role.id.trim() ||
      ids.has(role.id) ||
      typeof role.name !== 'string' ||
      typeof role.description !== 'string' ||
      (requireRules &&
        ((role.isBuiltIn !== undefined && typeof role.isBuiltIn !== 'boolean') ||
          !Array.isArray(role.permissions) ||
          role.permissions.some(
            (permission) =>
              !permission ||
              typeof permission.action !== 'string' ||
              !permission.action.trim() ||
              typeof permission.resource !== 'string' ||
              !permission.resource.trim(),
          )))
    )
      return false;
    ids.add(role.id);
    return true;
  });
}

/** The Roles editor consumes built-in status and rule targets as well as identity. */
export function isCompleteRoleEditorInventory(roles: unknown): boolean {
  return Array.isArray(roles) && hasCompleteRoleInventory(roles, true);
}

/** Require both lists before presenting saved User Access or opening its editor. */
export function isCompleteUserAccessInventory(users: unknown, roles: unknown): boolean {
  if (!Array.isArray(users) || !Array.isArray(roles)) return false;
  const usernames = new Set<string>();
  return (
    users.every((user) => {
      if (
        !user ||
        typeof user.username !== 'string' ||
        !user.username.trim() ||
        usernames.has(user.username) ||
        !Array.isArray(user.roleIds) ||
        user.roleIds.some((id: unknown) => typeof id !== 'string' || !id.trim())
      )
        return false;
      usernames.add(user.username);
      return true;
    }) &&
    // Assignment choices identify roles; the separate permissions read describes access.
    hasCompleteRoleInventory(roles, false)
  );
}

export function getUserAssignmentsUpdateErrorMessage(): string {
  return 'Failed to update user roles';
}

/** Compare local copies so opaque IDs and the actual request order stay intact. */
export function sameRoleIDs(first: readonly string[], second: readonly string[]): boolean {
  return JSON.stringify([...first].sort()) === JSON.stringify([...second].sort());
}

export const USER_ACCESS_REMOVAL_COPY = {
  self: 'You cannot remove your own user access.',
  effect: 'This removes all role assignments and revokes active Pulse sessions for:',
  identityProvider:
    'This does not disable the account at the identity provider. A later authorized SSO login will create the user record again.',
} as const;

export function getUserAssignmentsDeleteErrorMessage(error?: unknown): string {
  if (error && typeof error === 'object') {
    const typedError = error as { code?: unknown; status?: unknown };
    if (typedError.status === 403 && typedError.code === 'self_deprovision_denied') {
      return USER_ACCESS_REMOVAL_COPY.self;
    }
  }
  return 'Failed to remove user access';
}

// Both panels describe saved rules, not a client-side authorization verdict.
export function savedRuleEffect(effect: unknown) {
  if (effect === undefined || effect === '' || effect === 'allow') return 'Allow';
  return effect === 'deny' ? 'Deny' : null;
}

export function savedRuleConditions(conditions: unknown): [string, string][] | null {
  if (conditions === undefined) return [];
  if (!conditions || typeof conditions !== 'object' || Array.isArray(conditions)) return null;
  const entries = Object.entries(conditions);
  return entries.every(([, value]) => typeof value === 'string') ? entries : null;
}

export function savedRuleQualifier(permission: Permission): string {
  const effect = savedRuleEffect(permission.effect) ?? 'Effect unknown';
  const count = savedRuleConditions(permission.conditions)?.length;
  return effect + (count === undefined ? ' · Conditions unknown' : count ? ' · Conditional' : '');
}
