export function normalizePortalRole(role: string): string {
  if (role === 'member') return 'read_only';
  return role || 'read_only';
}

export function portalRoleLabel(role: string): string {
  switch (normalizePortalRole(role)) {
    case 'owner':
      return 'Owner';
    case 'admin':
      return 'Admin';
    case 'tech':
      return 'Tech';
    case 'read_only':
      return 'Read-only';
    case 'member':
      return 'Member';
    default:
      return role || 'Member';
  }
}

export function portalRoleCapabilityCopy(role: string, clientLanguage = false): string {
  switch (normalizePortalRole(role)) {
    case 'owner':
      return clientLanguage
        ? 'Full account control, including billing, access control, and client control.'
        : 'Full account control, including billing, access control, and workspace control.';
    case 'admin':
      return clientLanguage
        ? 'Can manage clients and billing for this account.'
        : 'Can manage workspaces and billing for this account.';
    case 'tech':
      return clientLanguage
        ? 'Can manage clients without billing ownership.'
        : 'Can manage workspaces without billing ownership.';
    case 'read_only':
      return clientLanguage
        ? 'Can review client status without making control-plane changes.'
        : 'Can review workspace status without making control-plane changes.';
    case 'member':
      return 'Has access through the account roster.';
    default:
      return 'Has access through the account roster.';
  }
}
