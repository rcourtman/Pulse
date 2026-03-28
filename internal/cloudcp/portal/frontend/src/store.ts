import {
  createPortalAccountState,
  createPortalLoginState,
  createPortalShellState,
  createPortalBillingState,
  syncLoginStateBootstrapEmail,
  syncBillingStateBootstrapEmail,
} from './state';
import type { PortalAccountState, PortalBootstrapData, PortalLoginState, PortalBillingState, PortalShellSection, PortalShellState } from './types';

interface MutationOptions {
  notify?: boolean;
}

export interface PortalStore {
  getBootstrap(): PortalBootstrapData;
  getAccountState(): PortalAccountState;
  getLoginState(): PortalLoginState;
  getShellState(): PortalShellState;
  getBillingState(): PortalBillingState;
  setBootstrap(nextBootstrap: Partial<PortalBootstrapData> | PortalBootstrapData): PortalBootstrapData;
  updateAccountState(mutator: (state: PortalAccountState) => void, options?: MutationOptions): PortalAccountState;
  updateLoginState(mutator: (state: PortalLoginState) => void, options?: MutationOptions): PortalLoginState;
  setActiveShellSection(section: PortalShellSection): PortalShellState;
  updateBillingState(mutator: (state: PortalBillingState) => void, options?: MutationOptions): PortalBillingState;
  subscribeAccount(listener: () => void): () => void;
  subscribeBootstrap(listener: () => void): () => void;
  subscribeLogin(listener: () => void): () => void;
  subscribeShell(listener: () => void): () => void;
  subscribeBilling(listener: () => void): () => void;
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
  var shellState = createPortalShellState();
  var billingState = createPortalBillingState();
  syncLoginStateBootstrapEmail(loginState, bootstrapState.email || '');
  syncBillingStateBootstrapEmail(billingState, bootstrapState.email || '');
  var accountSubscribers = new Set<() => void>();
  var bootstrapSubscribers = new Set<() => void>();
  var loginSubscribers = new Set<() => void>();
  var shellSubscribers = new Set<() => void>();
  var billingSubscribers = new Set<() => void>();

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
    getShellState: function() {
      return shellState;
    },
    getBillingState: function() {
      return billingState;
    },
    setBootstrap: function(nextBootstrap) {
      bootstrapState = normalizeBootstrap(bootstrapDefaults, nextBootstrap);
      syncLoginStateBootstrapEmail(loginState, bootstrapState.email || '');
      syncBillingStateBootstrapEmail(billingState, bootstrapState.email || '');
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
    setActiveShellSection: function(section) {
      shellState.activeSection = section;
      notify(shellSubscribers);
      return shellState;
    },
    updateBillingState: function(mutator, options) {
      mutator(billingState);
      if (!options || options.notify !== false) {
        notify(billingSubscribers);
      }
      return billingState;
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
    subscribeShell: function(listener) {
      shellSubscribers.add(listener);
      return function() {
        shellSubscribers.delete(listener);
      };
    },
    subscribeBilling: function(listener) {
      billingSubscribers.add(listener);
      return function() {
        billingSubscribers.delete(listener);
      };
    },
  };
}

function normalizeAccounts(accounts: Partial<PortalBootstrapData>['accounts']): PortalBootstrapData['accounts'] {
  return Array.isArray(accounts) ? accounts : [];
}
