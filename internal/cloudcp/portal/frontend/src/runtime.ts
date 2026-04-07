import { createPortalStore } from './store';
import type { PortalStore } from './store';
import type { PortalBootstrapData } from './types';

export interface PortalRuntimeHandoff {
  email: string;
  openBillingPanelID: string;
  upgradeCheckoutIntentID: string;
  upgradeFeatureKey: string;
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

function normalizeUpgradeCheckoutIntentID(
  value: string | null | undefined,
): string {
  var trimmed = String(value || '').trim();
  if (!trimmed) return '';
  return /^[A-Za-z0-9_-]+$/.test(trimmed) ? trimmed : '';
}

function normalizeHandoffBillingPanel(value: string | null): string {
  switch (String(value || '').trim()) {
    case 'upgrade':
      return 'upgrade-billing-panel';
    case 'manage':
      return 'manage-billing-panel';
    case 'retrieve':
      return 'retrieve-billing-panel';
    case 'refund':
      return 'refund-billing-panel';
    case 'data':
      return 'data-billing-panel';
    default:
      return '';
  }
}

function normalizeUpgradeFeatureKey(value: string | null): string {
  return String(value || '').trim();
}

export function readPortalRuntimeHandoff(
  locationHref: string | undefined = window.location.href,
): PortalRuntimeHandoff {
  try {
    var params = new URL(locationHref).searchParams;
    return {
      email: normalizeHandoffEmail(params.get('email')),
      openBillingPanelID: normalizeHandoffBillingPanel(params.get('service')),
      upgradeCheckoutIntentID: normalizeUpgradeCheckoutIntentID(params.get('checkout_intent_id')),
      upgradeFeatureKey: normalizeUpgradeFeatureKey(params.get('feature')),
    };
  } catch {
    return {
      email: '',
      openBillingPanelID: '',
      upgradeCheckoutIntentID: '',
      upgradeFeatureKey: '',
    };
  }
}

export function createBootstrapDefaults(
  embeddedBootstrap: Partial<PortalBootstrapData>
): Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'> {
  var signupPath = typeof embeddedBootstrap.signup_path === 'string'
    ? embeddedBootstrap.signup_path
    : '/signup';
  return {
    has_self_hosted_commercial: embeddedBootstrap.has_self_hosted_commercial === true,
    public_site_url: embeddedBootstrap.public_site_url || 'https://pulserelay.pro',
    support_email: embeddedBootstrap.support_email || 'support@pulserelay.pro',
    commercial_api_base_url: embeddedBootstrap.commercial_api_base_url || '',
    portal_path: embeddedBootstrap.portal_path || '/portal',
    bootstrap_path: embeddedBootstrap.bootstrap_path || '/api/portal/bootstrap',
    magic_link_request_path: embeddedBootstrap.magic_link_request_path || '/api/public/magic-link/request',
    signup_path: signupPath,
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
    store.updateBillingState(function(billingState) {
      billingState.flows.manage.emailValue = handoff.email;
      billingState.flows.retrieve.emailValue = handoff.email;
      billingState.flows.export.emailValue = handoff.email;
      billingState.flows.delete.emailValue = handoff.email;
      billingState.refund.emailValue = handoff.email;
    }, { notify: false });
  }
  if (handoff.openBillingPanelID) {
    store.setActiveShellSection('billing');
    store.updateBillingState(function(billingState) {
      billingState.openBillingPanelID = handoff.openBillingPanelID;
      billingState.upgradeCheckoutIntentID = handoff.upgradeCheckoutIntentID;
      billingState.upgradeFeatureKey = handoff.upgradeFeatureKey;
    }, { notify: false });
  } else if (handoff.upgradeFeatureKey || handoff.upgradeCheckoutIntentID) {
    store.setActiveShellSection('billing');
    store.updateBillingState(function(billingState) {
      billingState.upgradeCheckoutIntentID = handoff.upgradeCheckoutIntentID;
      billingState.upgradeFeatureKey = handoff.upgradeFeatureKey;
    }, { notify: false });
  }
  return {
    bootstrapDefaults: bootstrapDefaults,
    embeddedBootstrap: embeddedBootstrap,
    handoff: handoff,
    store: store,
  };
}
