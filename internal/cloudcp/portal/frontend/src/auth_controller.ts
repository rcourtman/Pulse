import { createPortalLoginState, syncLoginStateBootstrapEmail } from './state';
import type { PortalLoginState } from './types';
import type { PortalStore } from './store';

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

export interface AuthControllerDeps {
  store: PortalStore;
}

export interface AuthController {
  getLoginState: () => PortalLoginState;
}

function asHTMLElement(target: EventTarget | null): HTMLElement | null {
  return target instanceof HTMLElement ? target : null;
}

function getElement<T extends HTMLElement = HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null;
}

export function installAuthController(deps: AuthControllerDeps): AuthController {
  deps.store.updateLoginState(function(loginState) {
    var bootstrap = deps.store.getBootstrap();
    syncLoginStateBootstrapEmail(loginState, bootstrap.email || '');
  }, { notify: false });

  async function sendMagicLink() {
    var loginState = deps.store.getLoginState();
    var email = String(loginState.emailValue || '').trim();
    if (!email) {
      var input = getElement<FormValueElement>('portal-login-email');
      if (input) input.focus();
      return;
    }
    deps.store.updateLoginState(function(nextState) {
      nextState.sending = true;
      nextState.error = '';
      nextState.success = false;
    });
    try {
      var response = await fetch(deps.store.getBootstrap().magic_link_request_path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email })
      });
      if (response.ok || response.status === 404) {
        deps.store.updateLoginState(function(nextState) {
          nextState.sending = false;
          nextState.success = true;
        });
        return;
      }
      deps.store.updateLoginState(function(nextState) {
        nextState.error = response.status === 429
          ? 'Too many requests. Please wait a moment and try again.'
          : 'Something went wrong. Please try again.';
      });
    } catch (_) {
      deps.store.updateLoginState(function(nextState) {
        nextState.error = 'Network error. Please check your connection and try again.';
      }, { notify: false });
    }
    deps.store.updateLoginState(function(nextState) {
      nextState.sending = false;
    });
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
          deps.store.updateLoginState(function(nextState) {
            nextState.success = false;
            nextState.error = '';
          });
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
        await fetch(deps.store.getBootstrap().logout_path, { method: 'POST' });
      } catch (_) {}
      window.location.href = deps.store.getBootstrap().portal_path;
    })();
  });

  document.addEventListener('input', function(event) {
    var target = asHTMLElement(event.target) as FormValueElement | null;
    if (!target) return;
    if (target.getAttribute('data-portal-input') === 'login-email') {
      deps.store.updateLoginState(function(nextState) {
        nextState.emailValue = target.value;
      }, { notify: false });
    }
  });

  return {
    getLoginState: function() {
      return deps.store.getLoginState();
    },
  };
}
