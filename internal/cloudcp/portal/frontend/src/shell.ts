import {
  createAnonymousBootstrap,
  getAccountAPIBasePath,
  getBootstrap,
  getBootstrapPath,
  getCommercialAPIBaseURL,
  getLogoutPath,
  getMagicLinkRequestPath,
  getPortalAPIBasePath,
  getPortalPath,
  getSignupPath,
  notifyPortalRender,
  setBootstrap,
} from './runtime';
import { installAccountController } from './account_controller';
import { installAuthController } from './auth_controller';
import {
  renderAuthenticatedPortalHTML,
  renderHeaderHTML,
  renderSignedOutPortalHTML,
} from './shell_view';
import type { PortalBootstrapData } from './types';

var portalBootstrap: PortalBootstrapData = getBootstrap();
var LICENSE_API_BASE = getCommercialAPIBaseURL();
var PORTAL_PATH = getPortalPath();
var BOOTSTRAP_PATH = getBootstrapPath();
var MAGIC_LINK_REQUEST_PATH = getMagicLinkRequestPath();
var SIGNUP_PATH = getSignupPath();
var LOGOUT_PATH = getLogoutPath();
var ACCOUNT_API_BASE_PATH = getAccountAPIBasePath();
var PORTAL_API_BASE_PATH = getPortalAPIBasePath();

type ToastElement = HTMLElement & { _timer?: ReturnType<typeof setTimeout> };

function renderHeader() {
  var userInfo = document.getElementById('portal-user-info');
  if (!userInfo) return;
  userInfo.innerHTML = renderHeaderHTML({
    bootstrap: portalBootstrap,
    loginState: authController.getLoginState(),
    signupPath: SIGNUP_PATH,
    accountAPIBasePath: ACCOUNT_API_BASE_PATH,
  });
}

function renderPortalApp() {
  renderHeader();
  var root = document.getElementById('portal-app-root');
  if (!root) return;
  var context = {
    bootstrap: portalBootstrap,
    loginState: authController.getLoginState(),
    signupPath: SIGNUP_PATH,
    accountAPIBasePath: ACCOUNT_API_BASE_PATH,
  };
  root.innerHTML = portalBootstrap.authenticated
    ? renderAuthenticatedPortalHTML(context)
    : renderSignedOutPortalHTML(context);
  notifyPortalRender();
}

function applyBootstrap(data) {
  portalBootstrap = setBootstrap(data || createAnonymousBootstrap());
  LICENSE_API_BASE = getCommercialAPIBaseURL();
  PORTAL_PATH = getPortalPath();
  BOOTSTRAP_PATH = getBootstrapPath();
  MAGIC_LINK_REQUEST_PATH = getMagicLinkRequestPath();
  SIGNUP_PATH = getSignupPath();
  LOGOUT_PATH = getLogoutPath();
  ACCOUNT_API_BASE_PATH = getAccountAPIBasePath();
  PORTAL_API_BASE_PATH = getPortalAPIBasePath();
  if (!portalBootstrap.authenticated) {
    authController.syncBootstrapEmail(portalBootstrap.email || '');
  }
  renderPortalApp();
}

async function refreshBootstrap() {
  if (!BOOTSTRAP_PATH) return false;
  try {
    var response = await fetch(BOOTSTRAP_PATH, {
      headers: { 'Accept': 'application/json' }
    });
    if (response.status === 401) {
      applyBootstrap(createAnonymousBootstrap());
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
    return MAGIC_LINK_REQUEST_PATH;
  },
  getLogoutPath: function() {
    return LOGOUT_PATH;
  },
  getPortalPath: function() {
    return PORTAL_PATH;
  },
  renderPortal: renderPortalApp,
});

installAccountController({
  getAccountAPIBasePath: function() {
    return ACCOUNT_API_BASE_PATH;
  },
  getPortalAPIBasePath: function() {
    return PORTAL_API_BASE_PATH;
  },
  getPortalPath: function() {
    return PORTAL_PATH;
  },
  refreshBootstrap: refreshBootstrap,
  showToast: showToast
});

applyBootstrap(portalBootstrap);
if (portalBootstrap.authenticated) {
  refreshBootstrap();
}
