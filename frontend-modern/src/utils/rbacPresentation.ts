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

export function getUserAssignmentsUpdateErrorMessage(): string {
  return 'Failed to update user roles';
}
