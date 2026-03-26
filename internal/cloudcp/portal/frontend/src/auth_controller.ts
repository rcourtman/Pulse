import { beginMutationState, failMutationState, resetMutationState, succeedMutationState } from './async_state';
import { PortalAPIError } from './api';
import type { PortalAPI } from './api';
import { createPortalLoginState, syncLoginStateBootstrapEmail } from './state';
import type { PortalLoginState } from './types';
import type { PortalStore } from './store';

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

export interface AuthControllerDeps {
  api: PortalAPI;
  store: PortalStore;
}

export interface AuthController {
  getLoginState: () => PortalLoginState;
}

const GENERIC_MAGIC_LINK_MESSAGE = "If that email is registered, you'll receive a magic link shortly.";

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
      beginMutationState(nextState.request);
      nextState.success = false;
      nextState.successMessage = '';
    });
    try {
      var response = await deps.api.requestMagicLink(email);
      deps.store.updateLoginState(function(nextState) {
        succeedMutationState(nextState.request);
        nextState.success = true;
        nextState.successMessage = String(response && response.message || '').trim() || GENERIC_MAGIC_LINK_MESSAGE;
      });
      return;
    } catch (error) {
      if (error instanceof PortalAPIError && error.status === 404) {
        deps.store.updateLoginState(function(nextState) {
          succeedMutationState(nextState.request);
          nextState.success = true;
          nextState.successMessage = GENERIC_MAGIC_LINK_MESSAGE;
        });
        return;
      }
      deps.store.updateLoginState(function(nextState) {
        failMutationState(
          nextState.request,
          error instanceof PortalAPIError && error.status === 429
            ? 'Too many requests. Please wait a moment and try again.'
            : 'Network error. Please check your connection and try again.'
        );
      });
    }
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
            nextState.successMessage = '';
            resetMutationState(nextState.request);
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
        await deps.api.logout();
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
