import {
  createPortalAccountState,
  createPortalLoginState,
  createPortalServiceState,
  syncLoginStateBootstrapEmail,
  syncServiceStateBootstrapEmail,
} from './state';
import type { PortalAccountState, PortalBootstrapData, PortalLoginState, PortalServiceState } from './types';

interface MutationOptions {
  notify?: boolean;
}

export interface PortalStore {
  getBootstrap(): PortalBootstrapData;
  getAccountState(): PortalAccountState;
  getLoginState(): PortalLoginState;
  getServiceState(): PortalServiceState;
  setBootstrap(nextBootstrap: Partial<PortalBootstrapData> | PortalBootstrapData): PortalBootstrapData;
  updateAccountState(mutator: (state: PortalAccountState) => void, options?: MutationOptions): PortalAccountState;
  updateLoginState(mutator: (state: PortalLoginState) => void, options?: MutationOptions): PortalLoginState;
  updateServiceState(mutator: (state: PortalServiceState) => void, options?: MutationOptions): PortalServiceState;
  subscribeAccount(listener: () => void): () => void;
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
  var accountState = createPortalAccountState();
  var loginState = createPortalLoginState();
  var serviceState = createPortalServiceState();
  syncLoginStateBootstrapEmail(loginState, bootstrapState.email || '');
  syncServiceStateBootstrapEmail(serviceState, bootstrapState.email || '');
  var accountSubscribers = new Set<() => void>();
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
    getAccountState: function() {
      return accountState;
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
    updateAccountState: function(mutator, options) {
      mutator(accountState);
      if (!options || options.notify !== false) {
        notify(accountSubscribers);
      }
      return accountState;
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
    subscribeAccount: function(listener) {
      accountSubscribers.add(listener);
      return function() {
        accountSubscribers.delete(listener);
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
