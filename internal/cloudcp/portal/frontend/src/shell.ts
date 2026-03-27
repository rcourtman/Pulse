import type { PortalStore } from './store';
import {
  renderAuthenticatedPortalHTML,
  renderHeaderHTML,
  renderSignedOutPortalHTML,
} from './shell_view';
import type { PortalShellSection } from './types';

export interface ShellDeps {
  store: PortalStore;
  onSectionChange?: (section: PortalShellSection) => void;
}

export function installShell(deps: ShellDeps): void {
  function syncShellSection() {
    var root = document.querySelector('.portal-shell') as HTMLElement | null;
    var activeSection = deps.store.getShellState().activeSection;
    if (root) {
      root.setAttribute('data-shell-section', activeSection);
    }
    var links = document.querySelectorAll('[data-shell-action="activate-section"]');
    links.forEach(function(node) {
      var button = node as HTMLElement;
      button.classList.toggle('active', button.getAttribute('data-shell-section') === activeSection);
    });
  }

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
      activeSection: deps.store.getShellState().activeSection,
    };
    root.innerHTML = portalBootstrap.authenticated
      ? renderAuthenticatedPortalHTML(context)
      : renderSignedOutPortalHTML(context);
    syncShellSection();
  }

  deps.store.subscribeBootstrap(function() {
    renderPortalApp();
  });

  deps.store.subscribeLogin(function() {
    renderPortalApp();
  });

  deps.store.subscribeShell(function() {
    syncShellSection();
  });

  document.addEventListener('click', function(event) {
    var target = event.target instanceof HTMLElement ? event.target.closest('[data-shell-action="activate-section"]') as HTMLElement | null : null;
    if (!target) return;
    event.preventDefault();
    var section = (target.getAttribute('data-shell-section') || 'overview') as PortalShellSection;
    deps.store.setActiveShellSection(section);
    if (deps.onSectionChange) {
      deps.onSectionChange(section);
    }
  });

  renderPortalApp();
}
