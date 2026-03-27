import { installAccountController } from './account_controller';
import { installAccountRuntime } from './account_runtime';
import { createPortalAPI, PortalAPIError } from './api';
import { installAuthController } from './auth_controller';
import { installServicesRuntime } from './services';
import { installShell } from './shell';
import { createAnonymousBootstrap } from './store';
import { createPortalRuntime } from './runtime';
import type { PortalStore } from './store';
import type { PortalBootstrapData } from './types';

type ToastElement = HTMLElement & { _timer?: ReturnType<typeof setTimeout> };

export interface PortalAppDeps {
  bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'>;
  store: PortalStore;
}

export interface PortalApp {
  applyBootstrap: (data: Partial<PortalBootstrapData> | PortalBootstrapData | null | undefined) => PortalBootstrapData;
  refreshBootstrap: () => Promise<boolean>;
  showToast: (message: string, isError?: boolean) => void;
  startupRefresh: Promise<boolean> | null;
}

export function installPortalApp(deps: PortalAppDeps): PortalApp {
  var api = createPortalAPI({
    getBootstrap: function() {
      return deps.store.getBootstrap();
    },
  });

  function applyBootstrap(data: Partial<PortalBootstrapData> | PortalBootstrapData | null | undefined): PortalBootstrapData {
    return deps.store.setBootstrap(data || createAnonymousBootstrap(deps.bootstrapDefaults));
  }

  async function refreshBootstrap(): Promise<boolean> {
    var bootstrap = deps.store.getBootstrap();
    if (!bootstrap.bootstrap_path) return false;
    try {
      var data = await api.fetchBootstrap();
      applyBootstrap(data);
      return true;
    } catch (error) {
      if (error instanceof PortalAPIError && error.status === 401) {
        applyBootstrap(createAnonymousBootstrap(deps.bootstrapDefaults));
        return true;
      }
    }
    return false;
  }

  function showToast(message: string, isError = false) {
    var toast = document.getElementById('toast') as ToastElement | null;
    if (!toast) return;
    toast.textContent = message;
    toast.className = 'toast visible' + (isError ? ' error' : '');
    clearTimeout(toast._timer);
    toast._timer = setTimeout(function() {
      toast.className = 'toast';
    }, 4000);
  }

  var accountRuntime = installAccountRuntime({
    api: api,
    store: deps.store,
    refreshBootstrap: refreshBootstrap,
    showToast: showToast,
  });

  installShell({
    store: deps.store,
    onSectionChange: function(section) {
      if (section === 'team') {
        var accounts = deps.store.getBootstrap().accounts || [];
        if (accounts[0]) {
          accountRuntime.ensureTeamVisible(accounts[0].id);
        }
      }
    },
  });

  installServicesRuntime({
    api: api,
    store: deps.store,
  });

  installAuthController({
    api: api,
    store: deps.store,
  });

  installAccountController({
    runtime: accountRuntime,
    setShellSection: function(section) {
      deps.store.setActiveShellSection(section);
    },
  });

  var startupRefresh = deps.store.getBootstrap().authenticated ? refreshBootstrap() : null;

  return {
    applyBootstrap: applyBootstrap,
    refreshBootstrap: refreshBootstrap,
    showToast: showToast,
    startupRefresh: startupRefresh,
  };
}

export function startPortalApp(): PortalApp {
  var runtime = createPortalRuntime();
  return installPortalApp({
    bootstrapDefaults: runtime.bootstrapDefaults,
    store: runtime.store,
  });
}
