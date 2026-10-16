import type { SecurityStatus } from '@/types/config';

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

export function getUserIdentityDisplayName(user: UserIdentityPresentation): string {
  return user.displayName?.trim() || user.email?.trim() || user.username;
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

export function getTeamOnboardingCopy() {
  return {
    title: 'Add a teammate',
    add: 'Add teammate',
    refresh: 'Refresh',
    close: 'Close',
    closeLabel: 'Close team onboarding',
    retry: 'Retry',
    configureSSO: 'Configure Single Sign-On',
    authenticationSettings: 'Authentication settings',
    reviewSSO: 'Review Single Sign-On',
    introduction:
      'Use your existing sign-in provider to give a teammate access to this Pulse instance.',
    loading: 'Checking team sign-in',
    loadError: 'Could not check team sign-in. Retry before sharing the link.',
    setupTitle: 'Set up team sign-in first',
    setupBody:
      'Connect an OIDC or SAML identity provider in Single Sign-On, or configure trusted proxy authentication. Each teammate needs their own identity. Do not share the instance administrator password.',
    configuredTitle: '1. Check who can sign in',
    configuredBody:
      'Confirm your teammate is allowed by your identity provider and Pulse sign-in restrictions. A configured provider does not confirm that their account has access.',
    shareTitle: '2. Share the sign-in link',
    shareBody:
      'Send this link to your teammate and ask them to sign in with their own account. Use an address they can reach from their network.',
    linkLabel: 'Pulse sign-in link',
    invalidLink: 'Open Pulse using its normal HTTP or HTTPS address before sharing a sign-in link.',
    copy: 'Copy link',
    copied: 'Link copied',
    copyError: 'Could not copy the link. Select the link above and copy it manually.',
    reviewTitle: '3. Review their access after sign-in',
    reviewBody:
      'After their first successful sign-in, refresh the user list and choose Manage Access beside their name. Check their assigned roles and give only the access they need.',
    mappings:
      'Identity-provider group mappings apply at sign-in and can replace manually assigned roles on their next login. Change mapped access at the provider or in Single Sign-On.',
    removal:
      'To remove access, use Remove in the user list to clear roles and revoke active Pulse sessions. Also remove permission at the sign-in provider to prevent another login.',
    review: 'Refresh user list',
  };
}

export function getTeamSignInMethods(
  status: Pick<SecurityStatus, 'ssoEnabled' | 'ssoProviders' | 'hasProxyAuth'>,
): string[] {
  const methods = status.ssoEnabled
    ? (status.ssoProviders || []).map(
        (provider) => `${provider.displayName || provider.name} (${provider.type.toUpperCase()})`,
      )
    : [];
  if (status.hasProxyAuth) methods.push('Trusted proxy authentication');
  return methods;
}

export function getTeamSignInUrl(origin: string): string | null {
  try {
    const url = new URL(origin);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return null;
    // The instance's normal login entry lets the teammate choose their provider.
    // Never share the administrator's current route, query string, or fragment.
    return new URL('/login', url.origin).href;
  } catch {
    return null;
  }
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

export function getUserAssignmentsDeleteErrorMessage(error?: unknown): string {
  if (error && typeof error === 'object') {
    const typedError = error as { code?: unknown; status?: unknown };
    if (typedError.status === 403 && typedError.code === 'self_deprovision_denied') {
      return 'You cannot remove your own user access.';
    }
  }
  return 'Failed to remove user access';
}
