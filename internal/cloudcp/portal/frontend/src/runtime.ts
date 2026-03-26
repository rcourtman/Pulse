import { createPortalStore } from './store';
import type { PortalStore } from './store';
import type { PortalBootstrapData } from './types';

export interface PortalRuntime {
  bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'>;
  embeddedBootstrap: Partial<PortalBootstrapData>;
  store: PortalStore;
}

function readEmbeddedBootstrap(): Partial<PortalBootstrapData> {
  const bootstrapEl = document.getElementById('pulse-account-bootstrap');
  if (!bootstrapEl) {
    return {};
  }
  try {
    return JSON.parse(bootstrapEl.textContent || '{}') as Partial<PortalBootstrapData>;
  } catch {
    return {};
  }
}

export function createBootstrapDefaults(
  embeddedBootstrap: Partial<PortalBootstrapData>
): Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'> {
  return {
    public_site_url: embeddedBootstrap.public_site_url || 'https://pulserelay.pro',
    support_email: embeddedBootstrap.support_email || 'support@pulserelay.pro',
    commercial_api_base_url: embeddedBootstrap.commercial_api_base_url || '',
    portal_path: embeddedBootstrap.portal_path || '/portal',
    bootstrap_path: embeddedBootstrap.bootstrap_path || '/api/portal/bootstrap',
    magic_link_request_path: embeddedBootstrap.magic_link_request_path || '/api/public/magic-link/request',
    signup_path: embeddedBootstrap.signup_path || '/signup',
    logout_path: embeddedBootstrap.logout_path || '/auth/logout',
    account_api_base_path: embeddedBootstrap.account_api_base_path || '/api/accounts',
    portal_api_base_path: embeddedBootstrap.portal_api_base_path || '/api/portal',
  };
}

export function createPortalRuntime(
  embeddedBootstrap: Partial<PortalBootstrapData> = readEmbeddedBootstrap()
): PortalRuntime {
  var bootstrapDefaults = createBootstrapDefaults(embeddedBootstrap);
  return {
    bootstrapDefaults: bootstrapDefaults,
    embeddedBootstrap: embeddedBootstrap,
    store: createPortalStore(bootstrapDefaults, embeddedBootstrap),
  };
}
