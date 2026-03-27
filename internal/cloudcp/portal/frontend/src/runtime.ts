import { createPortalStore } from './store';
import type { PortalStore } from './store';
import type { PortalBootstrapData } from './types';

export interface PortalRuntimeHandoff {
  email: string;
  openPanelID: string;
}

export interface PortalRuntime {
  bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'>;
  embeddedBootstrap: Partial<PortalBootstrapData>;
  handoff: PortalRuntimeHandoff;
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

function normalizeHandoffEmail(value: string | null): string {
  return String(value || '').trim();
}

function normalizeHandoffService(value: string | null): string {
  switch (String(value || '').trim()) {
    case 'manage':
      return 'manage-service-panel';
    case 'retrieve':
      return 'retrieve-service-panel';
    case 'refund':
      return 'refund-service-panel';
    case 'data':
      return 'data-service-panel';
    default:
      return '';
  }
}

export function readPortalRuntimeHandoff(locationHref: string | undefined = window.location.href): PortalRuntimeHandoff {
  try {
    var params = new URL(locationHref).searchParams;
    return {
      email: normalizeHandoffEmail(params.get('email')),
      openPanelID: normalizeHandoffService(params.get('service')),
    };
  } catch {
    return {
      email: '',
      openPanelID: '',
    };
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
  embeddedBootstrap: Partial<PortalBootstrapData> = readEmbeddedBootstrap(),
  handoff: PortalRuntimeHandoff = readPortalRuntimeHandoff()
): PortalRuntime {
  var bootstrapDefaults = createBootstrapDefaults(embeddedBootstrap);
  var store = createPortalStore(bootstrapDefaults, embeddedBootstrap);
  if (handoff.email) {
    store.updateLoginState(function(loginState) {
      loginState.emailValue = handoff.email;
    }, { notify: false });
    store.updateServiceState(function(serviceState) {
      serviceState.flows.manage.emailValue = handoff.email;
      serviceState.flows.retrieve.emailValue = handoff.email;
      serviceState.flows.export.emailValue = handoff.email;
      serviceState.flows.delete.emailValue = handoff.email;
      serviceState.refund.emailValue = handoff.email;
    }, { notify: false });
  }
  if (handoff.openPanelID) {
    store.updateServiceState(function(serviceState) {
      serviceState.openPanelID = handoff.openPanelID;
    }, { notify: false });
  }
  return {
    bootstrapDefaults: bootstrapDefaults,
    embeddedBootstrap: embeddedBootstrap,
    handoff: handoff,
    store: store,
  };
}
