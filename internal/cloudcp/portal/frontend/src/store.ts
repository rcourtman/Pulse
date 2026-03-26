import type { PortalBootstrapData } from './types';

export interface PortalStore {
  getBootstrap(): PortalBootstrapData;
  setBootstrap(nextBootstrap: Partial<PortalBootstrapData> | PortalBootstrapData): PortalBootstrapData;
  subscribe(listener: () => void): () => void;
}

export function createAnonymousBootstrap(
  bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'>,
  overrides: Partial<PortalBootstrapData> = {}
): PortalBootstrapData {
  return {
    authenticated: false,
    email: '',
    ...bootstrapDefaults,
    ...overrides,
    accounts: normalizeAccounts(overrides.accounts),
  };
}

export function normalizeBootstrap(
  bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'>,
  raw: Partial<PortalBootstrapData> | null | undefined
): PortalBootstrapData {
  return createAnonymousBootstrap(bootstrapDefaults, raw || {});
}

export function createPortalStore(
  bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'>,
  initialBootstrap: Partial<PortalBootstrapData> | null | undefined
): PortalStore {
  var bootstrapState = normalizeBootstrap(bootstrapDefaults, initialBootstrap);
  var subscribers = new Set<() => void>();

  return {
    getBootstrap: function() {
      return bootstrapState;
    },
    setBootstrap: function(nextBootstrap) {
      bootstrapState = normalizeBootstrap(bootstrapDefaults, nextBootstrap);
      subscribers.forEach(function(listener) {
        listener();
      });
      return bootstrapState;
    },
    subscribe: function(listener) {
      subscribers.add(listener);
      return function() {
        subscribers.delete(listener);
      };
    },
  };
}

function normalizeAccounts(accounts: Partial<PortalBootstrapData>['accounts']): PortalBootstrapData['accounts'] {
  return Array.isArray(accounts) ? accounts : [];
}
