import { apiFetch, clearAuth } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

/**
 * Multi-organization mode limits the default organization to instance
 * administrators and its members. A signed-in user who belongs to no other
 * organization yet has no organization the app can open, and the server
 * answers its bootstrap read with `default_org_access_denied` even after the
 * client drops a stale selection.
 */
export const isNoOrganizationAccessResponse = async (response: Response): Promise<boolean> => {
  if (response.status !== 403) return false;
  try {
    const body = await response.clone().json();
    return body?.error === 'default_org_access_denied';
  } catch {
    return false;
  }
};

/** Reports whether a failed bootstrap failed because the user can use no organization. */
export const probeNoOrganizationAccess = async (): Promise<boolean> => {
  try {
    const response = await apiFetch('/api/state/summary', {
      recoverDefaultOrg: true,
      headers: { 'X-Requested-With': 'XMLHttpRequest', Accept: 'application/json' },
    });
    return await isNoOrganizationAccessResponse(response);
  } catch {
    return false;
  }
};

/**
 * Ends the session of a user who never entered the app. Proxy-authenticated
 * users leave through the proxy's logout page, as the app shell's sign-out
 * does; there is no app state to clean up beyond the stored credentials.
 */
export const signOutWithoutOrganization = async (): Promise<void> => {
  let destination = '/';
  try {
    const response = await apiFetch('/api/security/status', { skipOrgContext: true });
    const status = await response.json();
    if (typeof status?.proxyAuthLogoutURL === 'string' && status.proxyAuthLogoutURL) {
      destination = status.proxyAuthLogoutURL;
    }
  } catch {
    // Sign out to the app root.
  }
  try {
    await apiFetch('/api/logout', { method: 'POST', skipOrgContext: true, retry: false });
  } catch (error) {
    logger.warn('Sign-out request failed', error);
  }
  clearAuth();
  window.location.assign(destination);
};
