import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { createPortalAPI } from './api';
import { installPortalApp } from './app';
import { installAuthController } from './auth_controller';
import { createPortalStore } from './store';
import type { PortalBootstrapData } from './types';

async function flushAsync() {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  await new Promise(function(resolve) {
    setTimeout(resolve, 0);
  });
}

const bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'> = {
  has_self_hosted_commercial: false,
  email_sign_in_available: true,
  provider_hosted_mode: false,
  public_site_url: 'https://pulserelay.pro',
  support_email: 'support@pulserelay.pro',
  commercial_api_base_url: '/api/portal/commercial',
  portal_path: '/portal',
  bootstrap_path: '/api/portal/bootstrap',
  magic_link_request_path: '/magic-link',
  signup_path: '/signup',
  logout_path: '/logout',
  account_api_base_path: '/api/accounts',
  portal_api_base_path: '/api/portal',
};

describe('auth controller', function() {
  var listeners: Array<[string, EventListenerOrEventListenerObject, boolean | AddEventListenerOptions | undefined]>;
  beforeEach(function() {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    listeners = [];
    var addListener = document.addEventListener.bind(document);
    vi.spyOn(document, 'addEventListener').mockImplementation(function(type, listener, options) {
      listeners.push([type, listener, options]);
      addListener(type, listener, options);
    });
  });

  afterEach(function() {
    listeners.forEach(function(args) { document.removeEventListener(...args); });
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('syncs bootstrap email without overwriting local input state', function() {
    var store = createPortalStore(bootstrapDefaults, {
      email: 'buyer@example.com',
    });
    var controller = installAuthController({
      api: createPortalAPI({ getBootstrap: function() { return store.getBootstrap(); } }),
      store,
    });

    expect(controller.getLoginState().emailValue).toBe('buyer@example.com');

    controller.getLoginState().emailValue = 'typed@example.com';
    store.setBootstrap({
      email: 'other@example.com',
    });
    expect(controller.getLoginState().emailValue).toBe('typed@example.com');
  });

  it('tracks login email input and completes magic-link request state', async function() {
    var store = createPortalStore(bootstrapDefaults, {});
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async function() {
          return { message: 'If that email is registered, you will receive a magic link shortly.' };
        },
        text: async function() {
          return '';
        },
      })
    );

    var controller = installAuthController({
      api: createPortalAPI({ getBootstrap: function() { return store.getBootstrap(); } }),
      store,
    });

    document.body.innerHTML =
      '<form data-portal-form="sign-in">' +
      '<input id="portal-login-email" type="email" required data-portal-input="login-email">' +
      '<button id="send" type="submit">Send</button></form>';

    var input = document.getElementById('portal-login-email') as HTMLInputElement;
    input.value = 'buyer@example.com';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    expect(controller.getLoginState().emailValue).toBe('buyer@example.com');

    (document.getElementById('send') as HTMLButtonElement).click();

    await flushAsync();
    expect(controller.getLoginState().request.pending).toBe(false);
    expect(controller.getLoginState().request.error).toBe('');
    expect(controller.getLoginState().success).toBe(true);
    expect(controller.getLoginState().successMessage).toContain('If that email is registered');
    expect(fetch).toHaveBeenCalledWith(
      '/magic-link',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'buyer@example.com', target: 'portal' }),
      })
    );
  });

  function mountSignIn(overrides: Partial<PortalBootstrapData> = {}) {
    document.body.innerHTML = '<div id="portal-user-info"></div><div id="portal-app-root"></div><div id="toast"></div>';
    var store = createPortalStore(bootstrapDefaults, { authenticated: false, email: '', accounts: [], ...overrides });
    installPortalApp({ bootstrapDefaults, store });
    return store;
  }

  function fillEmail(email = 'buyer@example.com') {
    var input = document.getElementById('portal-login-email') as HTMLInputElement;
    input.value = email;
    input.dispatchEvent(new Event('input', { bubbles: true }));
  }

  function submitSignIn() {
    var event = new Event('submit', { bubbles: true, cancelable: true });
    document.getElementById('portal-login-form')!.dispatchEvent(event);
    return event;
  }

  function jsonResponse(status: number, payload: unknown = {}) {
    return { ok: status >= 200 && status < 300, status, headers: new Headers({ 'content-type': 'application/json' }), json: async function() { return payload; } };
  }

  it('submits the real signed-out form once while pending, then permits deliberate resend', async function() {
    var settle!: (value: ReturnType<typeof jsonResponse>) => void;
    var fetchMock = vi.fn().mockImplementation(function() { return new Promise(function(resolve) { settle = resolve; }); });
    vi.stubGlobal('fetch', fetchMock);
    var store = mountSignIn();
    fillEmail();

    expect(submitSignIn().defaultPrevented).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(store.getLoginState().request.pending).toBe(true);
    expect(document.getElementById('portal-login-form')?.getAttribute('aria-busy')).toBe('true');
    expect((document.getElementById('portal-login-email') as HTMLInputElement).disabled).toBe(true);
    expect((document.getElementById('portal-login-send') as HTMLButtonElement).disabled).toBe(true);
    submitSignIn();
    // Even stale resend markup cannot reset the pending guard.
    var staleResend = document.createElement('a');
    staleResend.setAttribute('data-portal-action', 'resend-magic-link');
    document.body.appendChild(staleResend);
    staleResend.click();
    expect(fetchMock).toHaveBeenCalledTimes(1);

    settle(jsonResponse(200));
    await flushAsync();
    expect(store.getLoginState().success).toBe(true);
    expect(document.querySelector('[role="status"]')?.textContent).toContain('If that email is registered');
    (document.querySelector('[data-portal-action="resend-magic-link"]') as HTMLAnchorElement).click();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    settle(jsonResponse(200));
    await flushAsync();
    expect(store.getLoginState().request.pending).toBe(false);
  });

  it.each(['', 'not-an-email'])('keeps an invalid native email %j local', function(email) {
    var fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    mountSignIn();
    fillEmail(email);
    (document.getElementById('portal-login-send') as HTMLButtonElement).click();
    submitSignIn();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it.each([429, 500])('restores the named form and keyboard focus after HTTP %i without automatic retry', async function(status) {
    var fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(status)).mockResolvedValueOnce(jsonResponse(200));
    vi.stubGlobal('fetch', fetchMock);
    var store = mountSignIn();
    fillEmail();
    submitSignIn();
    await flushAsync();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(store.getLoginState().request.error).toContain(status === 429 ? 'Too many requests' : 'Network error');
    expect(document.querySelector('[role="alert"]')?.textContent).toBe(store.getLoginState().request.error);
    expect(document.getElementById('portal-login-form')?.getAttribute('aria-busy')).toBe('false');
    var input = document.getElementById('portal-login-email') as HTMLInputElement;
    expect(input.value).toBe('buyer@example.com');
    expect(input.disabled).toBe(false);
    expect(document.activeElement).toBe(input);
    expect((document.getElementById('portal-login-send') as HTMLButtonElement).disabled).toBe(false);
    submitSignIn();
    await flushAsync();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(store.getLoginState().success).toBe(true);
  });

  it('retains generic unknown-account success and the same magic-link payload', async function() {
    var fetchMock = vi.fn().mockResolvedValue(jsonResponse(404, { error: 'unknown_account' }));
    vi.stubGlobal('fetch', fetchMock);
    var store = mountSignIn();
    fillEmail();
    submitSignIn();
    await flushAsync();
    expect(store.getLoginState().success).toBe(true);
    expect(document.querySelector('[role="status"]')?.textContent).toContain('If that email is registered');
    expect(document.body.textContent).not.toContain('unknown_account');
    expect(fetchMock).toHaveBeenCalledWith('/magic-link', expect.objectContaining({ method: 'POST', body: JSON.stringify({ email: 'buyer@example.com', target: 'portal' }) }));
  });

  it('leaves unrelated form submissions and the no-email operator route alone', function() {
    var fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    mountSignIn({ email_sign_in_available: false, provider_hosted_mode: true });
    expect(document.body.textContent).toContain('Sign-in links come from your control plane host');
    expect(document.getElementById('portal-login-form')).toBeNull();
    var other = document.createElement('form');
    document.body.appendChild(other);
    var event = new Event('submit', { bubbles: true, cancelable: true });
    other.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    // A stale email form must not bypass the server-declared capability.
    other.setAttribute('data-portal-form', 'sign-in');
    other.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
