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
import {
  renderAuthenticatedPortalHTML,
  renderHeaderHTML,
  renderSignedOutPortalHTML,
} from './shell_view';
import type { PortalBootstrapData, PortalLoginState } from './types';

var portalBootstrap: PortalBootstrapData = getBootstrap();
var LICENSE_API_BASE = getCommercialAPIBaseURL();
var PORTAL_PATH = getPortalPath();
var BOOTSTRAP_PATH = getBootstrapPath();
var MAGIC_LINK_REQUEST_PATH = getMagicLinkRequestPath();
var SIGNUP_PATH = getSignupPath();
var LOGOUT_PATH = getLogoutPath();
var ACCOUNT_API_BASE_PATH = getAccountAPIBasePath();
var PORTAL_API_BASE_PATH = getPortalAPIBasePath();

var loginState: PortalLoginState = {
  emailValue: '',
  sending: false,
  success: false,
  error: '',
};

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;
type ToastElement = HTMLElement & { _timer?: ReturnType<typeof setTimeout> };

function getElement<T extends HTMLElement = HTMLElement>(id): T | null {
  return document.getElementById(id) as T | null;
}

function asHTMLElement(target: EventTarget | null): HTMLElement | null {
  return target instanceof HTMLElement ? target : null;
}

function renderHeader() {
  var userInfo = document.getElementById('portal-user-info');
  if (!userInfo) return;
  userInfo.innerHTML = renderHeaderHTML({
    bootstrap: portalBootstrap,
    loginState: loginState,
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
    loginState: loginState,
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
  if (!portalBootstrap.authenticated && !loginState.emailValue) {
    loginState.emailValue = portalBootstrap.email || '';
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
  var t = getElement<ToastElement>('toast');
  if (!t) return;
  t.textContent = msg;
  t.className = 'toast visible' + (isError ? ' error' : '');
  clearTimeout(t._timer);
  t._timer = setTimeout(function() { t.className = 'toast'; }, 4000);
}

function resetLoginState(options) {
  loginState.sending = false;
  loginState.error = '';
  loginState.success = false;
  if (options && options.keepEmail) return;
  loginState.emailValue = '';
}

async function sendMagicLink() {
  var email = String(loginState.emailValue || '').trim();
  if (!email) {
    var input = getElement<FormValueElement>('portal-login-email');
    if (input) input.focus();
    return;
  }
  loginState.sending = true;
  loginState.error = '';
  loginState.success = false;
  renderPortalApp();
  try {
    var response = await fetch(MAGIC_LINK_REQUEST_PATH, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    });
    if (response.ok || response.status === 404) {
      loginState.sending = false;
      loginState.success = true;
      renderPortalApp();
      return;
    }
    if (response.status === 429) {
      loginState.error = 'Too many requests. Please wait a moment and try again.';
    } else {
      loginState.error = 'Something went wrong. Please try again.';
    }
  } catch (_) {
    loginState.error = 'Network error. Please check your connection and try again.';
  }
  loginState.sending = false;
  renderPortalApp();
}

document.addEventListener('click', function(event) {
  var portalActionEl = asHTMLElement(event.target)?.closest('[data-portal-action]');
  if (portalActionEl) {
    var portalAction = portalActionEl.getAttribute('data-portal-action') || '';
    switch (portalAction) {
      case 'send-magic-link':
        event.preventDefault();
        sendMagicLink();
        return;
      case 'resend-magic-link':
        event.preventDefault();
        loginState.success = false;
        loginState.error = '';
        renderPortalApp();
        sendMagicLink();
        return;
      default:
        break;
    }
  }

  var logoutBtn = asHTMLElement(event.target)?.closest('#logout-btn') as HTMLButtonElement | null;
  if (logoutBtn) {
    event.preventDefault();
    logoutBtn.disabled = true;
    logoutBtn.textContent = 'Signing out…';
    (async function() {
      try {
        await fetch(LOGOUT_PATH, { method: 'POST' });
      } catch (_) {}
      window.location.href = PORTAL_PATH;
    })();
    return;
  }

});

document.addEventListener('input', function(event) {
  var target = asHTMLElement(event.target) as FormValueElement | null;
  if (!target) return;
  if (target.getAttribute('data-portal-input') === 'login-email') {
    loginState.emailValue = target.value;
  }
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

loginState.emailValue = portalBootstrap.email || '';
applyBootstrap(portalBootstrap);
if (portalBootstrap.authenticated) {
  refreshBootstrap();
}
