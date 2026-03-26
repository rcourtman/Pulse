import {
  bootstrapDefaults,
  portalStore,
} from './runtime';
import { createAnonymousBootstrap } from './store';
import { installAccountController } from './account_controller';
import { installAuthController } from './auth_controller';
import {
  renderAuthenticatedPortalHTML,
  renderHeaderHTML,
  renderSignedOutPortalHTML,
} from './shell_view';

type ToastElement = HTMLElement & { _timer?: ReturnType<typeof setTimeout> };

function renderHeader() {
  var userInfo = document.getElementById('portal-user-info');
  if (!userInfo) return;
  var portalBootstrap = portalStore.getBootstrap();
  userInfo.innerHTML = renderHeaderHTML({
    bootstrap: portalBootstrap,
    loginState: authController.getLoginState(),
    signupPath: portalBootstrap.signup_path,
    accountAPIBasePath: portalBootstrap.account_api_base_path,
  });
}

function renderPortalApp() {
  renderHeader();
  var root = document.getElementById('portal-app-root');
  if (!root) return;
  var portalBootstrap = portalStore.getBootstrap();
  var context = {
    bootstrap: portalBootstrap,
    loginState: authController.getLoginState(),
    signupPath: portalBootstrap.signup_path,
    accountAPIBasePath: portalBootstrap.account_api_base_path,
  };
  root.innerHTML = portalBootstrap.authenticated
    ? renderAuthenticatedPortalHTML(context)
    : renderSignedOutPortalHTML(context);
}

function applyBootstrap(data) {
  var portalBootstrap = portalStore.setBootstrap(data || createAnonymousBootstrap(bootstrapDefaults));
  if (!portalBootstrap.authenticated) {
    authController.syncBootstrapEmail(portalBootstrap.email || '');
  }
}

async function refreshBootstrap() {
  var bootstrap = portalStore.getBootstrap();
  if (!bootstrap.bootstrap_path) return false;
  try {
    var response = await fetch(bootstrap.bootstrap_path, {
      headers: { 'Accept': 'application/json' }
    });
    if (response.status === 401) {
      applyBootstrap(createAnonymousBootstrap(bootstrapDefaults));
      return true;
    }
    if (!response.ok) return false;
    var data = await response.json();
    applyBootstrap(data);
    return true;
  } catch (_) {}
  return false;
}

function showToast(msg, isError = false) {
  var t = document.getElementById('toast') as ToastElement | null;
  if (!t) return;
  t.textContent = msg;
  t.className = 'toast visible' + (isError ? ' error' : '');
  clearTimeout(t._timer);
  t._timer = setTimeout(function() { t.className = 'toast'; }, 4000);
}

var authController = installAuthController({
  getMagicLinkRequestPath: function() {
    return portalStore.getBootstrap().magic_link_request_path;
  },
  getLogoutPath: function() {
    return portalStore.getBootstrap().logout_path;
  },
  getPortalPath: function() {
    return portalStore.getBootstrap().portal_path;
  },
  renderPortal: renderPortalApp,
});

installAccountController({
  getAccountAPIBasePath: function() {
    return portalStore.getBootstrap().account_api_base_path;
  },
  getPortalAPIBasePath: function() {
    return portalStore.getBootstrap().portal_api_base_path;
  },
  getPortalPath: function() {
    return portalStore.getBootstrap().portal_path;
  },
  refreshBootstrap: refreshBootstrap,
  showToast: showToast
});

portalStore.subscribe(function() {
  renderPortalApp();
});

applyBootstrap(portalStore.getBootstrap());
if (portalStore.getBootstrap().authenticated) {
  refreshBootstrap();
}
