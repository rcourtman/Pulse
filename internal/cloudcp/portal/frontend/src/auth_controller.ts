import type { PortalLoginState } from './types';

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

export interface AuthControllerDeps {
  getMagicLinkRequestPath: () => string;
  getLogoutPath: () => string;
  getPortalPath: () => string;
  renderPortal: () => void;
}

export interface AuthController {
  getLoginState: () => PortalLoginState;
  syncBootstrapEmail: (email: string) => void;
}

function asHTMLElement(target: EventTarget | null): HTMLElement | null {
  return target instanceof HTMLElement ? target : null;
}

function getElement<T extends HTMLElement = HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null;
}

export function installAuthController(deps: AuthControllerDeps): AuthController {
  var loginState: PortalLoginState = {
    emailValue: '',
    sending: false,
    success: false,
    error: '',
  };

  function syncBootstrapEmail(email: string) {
    if (!loginState.emailValue) {
      loginState.emailValue = email || '';
    }
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
    deps.renderPortal();
    try {
      var response = await fetch(deps.getMagicLinkRequestPath(), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email })
      });
      if (response.ok || response.status === 404) {
        loginState.sending = false;
        loginState.success = true;
        deps.renderPortal();
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
    deps.renderPortal();
  }

  document.addEventListener('click', function(event) {
    var portalActionEl = asHTMLElement(event.target)?.closest('[data-portal-action]');
    if (portalActionEl) {
      var portalAction = portalActionEl.getAttribute('data-portal-action') || '';
      switch (portalAction) {
        case 'send-magic-link':
          event.preventDefault();
          void sendMagicLink();
          return;
        case 'resend-magic-link':
          event.preventDefault();
          loginState.success = false;
          loginState.error = '';
          deps.renderPortal();
          void sendMagicLink();
          return;
        default:
          break;
      }
    }

    var logoutBtn = asHTMLElement(event.target)?.closest('#logout-btn') as HTMLButtonElement | null;
    if (!logoutBtn) {
      return;
    }
    event.preventDefault();
    logoutBtn.disabled = true;
    logoutBtn.textContent = 'Signing out…';
    (async function() {
      try {
        await fetch(deps.getLogoutPath(), { method: 'POST' });
      } catch (_) {}
      window.location.href = deps.getPortalPath();
    })();
  });

  document.addEventListener('input', function(event) {
    var target = asHTMLElement(event.target) as FormValueElement | null;
    if (!target) return;
    if (target.getAttribute('data-portal-input') === 'login-email') {
      loginState.emailValue = target.value;
    }
  });

  return {
    getLoginState: function() {
      return loginState;
    },
    syncBootstrapEmail,
  };
}
