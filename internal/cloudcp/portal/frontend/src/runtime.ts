import type { PortalBootstrapData } from './types';

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

const embeddedBootstrap = readEmbeddedBootstrap();

const bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'> = {
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

function normalizeAccounts(accounts: Partial<PortalBootstrapData>['accounts']): PortalBootstrapData['accounts'] {
  return Array.isArray(accounts) ? accounts : [];
}

export function createAnonymousBootstrap(overrides: Partial<PortalBootstrapData> = {}): PortalBootstrapData {
  return {
    authenticated: false,
    email: '',
    ...bootstrapDefaults,
    ...overrides,
    accounts: normalizeAccounts(overrides.accounts),
  };
}

export function normalizeBootstrap(raw: Partial<PortalBootstrapData> | null | undefined): PortalBootstrapData {
  return createAnonymousBootstrap(raw || {});
}

let bootstrapState: PortalBootstrapData = normalizeBootstrap(embeddedBootstrap);
const renderSubscribers = new Set<() => void>();

export function getBootstrap(): PortalBootstrapData {
  return bootstrapState;
}

export function setBootstrap(nextBootstrap: Partial<PortalBootstrapData> | PortalBootstrapData): PortalBootstrapData {
  bootstrapState = normalizeBootstrap(nextBootstrap);
  return bootstrapState;
}

export function getCommercialAPIBaseURL(): string {
  return bootstrapState.commercial_api_base_url;
}

export function getPortalPath(): string {
  return bootstrapState.portal_path;
}

export function getBootstrapPath(): string {
  return bootstrapState.bootstrap_path;
}

export function getMagicLinkRequestPath(): string {
  return bootstrapState.magic_link_request_path;
}

export function getSignupPath(): string {
  return bootstrapState.signup_path;
}

export function getLogoutPath(): string {
  return bootstrapState.logout_path;
}

export function getAccountAPIBasePath(): string {
  return bootstrapState.account_api_base_path;
}

export function getPortalAPIBasePath(): string {
  return bootstrapState.portal_api_base_path;
}

export function notifyPortalRender(): void {
  renderSubscribers.forEach((listener) => {
    listener();
  });
}

export function subscribePortalRender(listener: () => void): () => void {
  renderSubscribers.add(listener);
  return () => {
    renderSubscribers.delete(listener);
  };
}
