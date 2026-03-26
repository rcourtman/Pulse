import {
  createPortalLoginState,
  createPortalServiceState,
  syncLoginStateBootstrapEmail,
  syncServiceStateBootstrapEmail,
} from './state';
import type { PortalBootstrapData, PortalLoginState, PortalServiceState } from './types';

interface MutationOptions {
  notify?: boolean;
}

export interface PortalStore {
  getBootstrap(): PortalBootstrapData;
  getLoginState(): PortalLoginState;
  getServiceState(): PortalServiceState;
  setBootstrap(nextBootstrap: Partial<PortalBootstrapData> | PortalBootstrapData): PortalBootstrapData;
  updateLoginState(mutator: (state: PortalLoginState) => void, options?: MutationOptions): PortalLoginState;
  updateServiceState(mutator: (state: PortalServiceState) => void, options?: MutationOptions): PortalServiceState;
  subscribeBootstrap(listener: () => void): () => void;
  subscribeLogin(listener: () => void): () => void;
  subscribeServices(listener: () => void): () => void;
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
  var loginState = createPortalLoginState();
  var serviceState = createPortalServiceState();
  syncLoginStateBootstrapEmail(loginState, bootstrapState.email || '');
  syncServiceStateBootstrapEmail(serviceState, bootstrapState.email || '');
  var bootstrapSubscribers = new Set<() => void>();
  var loginSubscribers = new Set<() => void>();
  var serviceSubscribers = new Set<() => void>();

  function notify(subscribers: Set<() => void>) {
    subscribers.forEach(function(listener) {
      listener();
    });
  }

  return {
    getBootstrap: function() {
      return bootstrapState;
    },
    getLoginState: function() {
      return loginState;
    },
    getServiceState: function() {
      return serviceState;
    },
    setBootstrap: function(nextBootstrap) {
      bootstrapState = normalizeBootstrap(bootstrapDefaults, nextBootstrap);
      syncLoginStateBootstrapEmail(loginState, bootstrapState.email || '');
      syncServiceStateBootstrapEmail(serviceState, bootstrapState.email || '');
      notify(bootstrapSubscribers);
      return bootstrapState;
    },
    updateLoginState: function(mutator, options) {
      mutator(loginState);
      if (!options || options.notify !== false) {
        notify(loginSubscribers);
      }
      return loginState;
    },
    updateServiceState: function(mutator, options) {
      mutator(serviceState);
      if (!options || options.notify !== false) {
        notify(serviceSubscribers);
      }
      return serviceState;
    },
    subscribeBootstrap: function(listener) {
      bootstrapSubscribers.add(listener);
      return function() {
        bootstrapSubscribers.delete(listener);
      };
    },
    subscribeLogin: function(listener) {
      loginSubscribers.add(listener);
      return function() {
        loginSubscribers.delete(listener);
      };
    },
    subscribeServices: function(listener) {
      serviceSubscribers.add(listener);
      return function() {
        serviceSubscribers.delete(listener);
      };
    },
  };
}

function normalizeAccounts(accounts: Partial<PortalBootstrapData>['accounts']): PortalBootstrapData['accounts'] {
  return Array.isArray(accounts) ? accounts : [];
}
