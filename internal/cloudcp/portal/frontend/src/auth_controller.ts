import { beginMutationState, failMutationState, resetMutationState, succeedMutationState } from './async_state';
import { PortalAPIError } from './api';
import type { PortalAPI } from './api';
import { createMutationState, syncLoginStateBootstrapEmail } from './state';
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

function asHTMLElement(target: EventTarget | null): HTMLElement | null {
  return target instanceof HTMLElement ? target : null;
}

function getElement<T extends HTMLElement = HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null;
}

export function installAuthController(deps: AuthControllerDeps): AuthController {
  // Keep request state outside the button: a bootstrap refresh can replace
  // the header while the sign-out request is still in flight.
  var logoutRequest = createMutationState();

  function renderLogoutState(): void {
    if (!deps.store.getBootstrap().authenticated) resetMutationState(logoutRequest);
    var button = getElement<HTMLButtonElement>('logout-btn');
    if (button) {
      button.disabled = logoutRequest.pending;
      button.textContent = logoutRequest.pending ? 'Signing out…' : 'Sign out';
      button.setAttribute('aria-busy', String(logoutRequest.pending));
    }
    var error = getElement<HTMLElement>('portal-logout-error');
    if (error) {
      error.textContent = logoutRequest.error;
      error.hidden = !logoutRequest.error;
    }
  }

  async function signOut(): Promise<void> {
    if (logoutRequest.pending || !deps.store.getBootstrap().authenticated) return;
    beginMutationState(logoutRequest);
    renderLogoutState();
    try {
      await deps.api.logout();
    } catch (_) {
      // A failed response cannot tell us whether the server revoked the
      // session. Do not imply success, discard the current task or retry it.
      failMutationState(logoutRequest, 'Sign out could not be confirmed. You may still be signed in. Try Sign out again.');
      renderLogoutState();
      getElement<HTMLButtonElement>('logout-btn')?.focus();
      return;
    }
    window.location.href = deps.store.getBootstrap().portal_path;
  }

  deps.store.updateLoginState(function(loginState) {
    var bootstrap = deps.store.getBootstrap();
    syncLoginStateBootstrapEmail(loginState, bootstrap.email || '');
  }, { notify: false });

  deps.store.subscribeBootstrap(renderLogoutState);
  deps.store.subscribeLogin(renderLogoutState);
  renderLogoutState();

  async function sendMagicLink() {
    if (deps.store.getBootstrap().email_sign_in_available === false) {
      // The signed-out view replaces the form with operator instructions in
      // this state; this guard covers any stale markup.
      return;
    }
    var loginState = deps.store.getLoginState();
    if (loginState.request.pending) return;
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
        nextState.successMessage = String(response?.message || '').trim();
      });
      return;
    } catch (error) {
      if (error instanceof PortalAPIError && error.status === 404) {
        deps.store.updateLoginState(function(nextState) {
          succeedMutationState(nextState.request);
          nextState.success = true;
          nextState.successMessage = '';
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
      getElement<HTMLInputElement>('portal-login-email')?.focus();
    }
  }

  document.addEventListener('submit', function(event) {
    var form = event.target;
    if (!(form instanceof HTMLFormElement) || form.getAttribute('data-portal-form') !== 'sign-in') return;
    event.preventDefault();
    if (!form.reportValidity()) return;
    void sendMagicLink();
  });

  document.addEventListener('click', function(event) {
    var portalActionEl = asHTMLElement(event.target)?.closest('[data-portal-action]');
    if (portalActionEl) {
      var portalAction = portalActionEl.getAttribute('data-portal-action') || '';
      switch (portalAction) {
        case 'resend-magic-link':
          event.preventDefault();
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
    void signOut();
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
