import type { PortalStore } from './store';
import {
  renderAuthenticatedPortalHTML,
  renderHeaderHTML,
  renderSignedOutPortalHTML,
} from './shell_view';

export interface ShellDeps {
  store: PortalStore;
}

export function installShell(deps: ShellDeps): void {
  function renderHeader() {
    var userInfo = document.getElementById('portal-user-info');
    if (!userInfo) return;
    var portalBootstrap = deps.store.getBootstrap();
    userInfo.innerHTML = renderHeaderHTML({
      bootstrap: portalBootstrap,
      loginState: deps.store.getLoginState(),
      signupPath: portalBootstrap.signup_path,
      accountAPIBasePath: portalBootstrap.account_api_base_path,
    });
  }

  function renderPortalApp() {
    renderHeader();
    var root = document.getElementById('portal-app-root');
    if (!root) return;
    var portalBootstrap = deps.store.getBootstrap();
    var context = {
      bootstrap: portalBootstrap,
      loginState: deps.store.getLoginState(),
      signupPath: portalBootstrap.signup_path,
      accountAPIBasePath: portalBootstrap.account_api_base_path,
    };
    root.innerHTML = portalBootstrap.authenticated
      ? renderAuthenticatedPortalHTML(context)
      : renderSignedOutPortalHTML(context);
  }

  deps.store.subscribeBootstrap(function() {
    renderPortalApp();
  });

  deps.store.subscribeLogin(function() {
    renderPortalApp();
  });

  renderPortalApp();
}
