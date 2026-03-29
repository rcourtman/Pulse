import type { PortalBootstrapData, PortalShellSection } from './types';

export function preferredPortalShellSection(
  bootstrap: Pick<PortalBootstrapData, 'authenticated' | 'has_self_hosted_commercial' | 'accounts'>
): PortalShellSection {
  var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
  var hasHostedAccounts = accounts.length > 0;
  var hasSelfHostedCommercial = bootstrap.has_self_hosted_commercial === true || !hasHostedAccounts;

  if (!bootstrap.authenticated) {
    return 'overview';
  }
  if (hasHostedAccounts) {
    return 'workspaces';
  }
  if (hasSelfHostedCommercial) {
    return 'billing';
  }
  return 'overview';
}
