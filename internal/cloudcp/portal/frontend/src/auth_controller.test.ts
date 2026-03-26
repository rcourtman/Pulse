import { beforeEach, describe, expect, it, vi } from 'vitest';

import { createPortalAPI } from './api';
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
  public_site_url: 'https://pulserelay.pro',
  support_email: 'support@pulserelay.pro',
  commercial_api_base_url: 'https://license.pulserelay.pro',
  portal_path: '/portal',
  bootstrap_path: '/api/portal/bootstrap',
  magic_link_request_path: '/magic-link',
  signup_path: '/signup',
  logout_path: '/logout',
  account_api_base_path: '/api/accounts',
  portal_api_base_path: '/api/portal',
};

describe('auth controller', function() {
  beforeEach(function() {
    document.body.innerHTML = '';
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
          return {
            success: true,
            message: "If that email is registered, you'll receive a magic link shortly.",
          };
        },
      })
    );

    var controller = installAuthController({
      api: createPortalAPI({ getBootstrap: function() { return store.getBootstrap(); } }),
      store,
    });

    document.body.innerHTML =
      '<input id="portal-login-email" data-portal-input="login-email">' +
      '<button id="send" data-portal-action="send-magic-link">Send</button>';

    var input = document.getElementById('portal-login-email') as HTMLInputElement;
    input.value = 'buyer@example.com';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    expect(controller.getLoginState().emailValue).toBe('buyer@example.com');

    document.getElementById('send')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    await flushAsync();
    expect(controller.getLoginState().request.pending).toBe(false);
    expect(controller.getLoginState().request.error).toBe('');
    expect(controller.getLoginState().success).toBe(true);
    expect(controller.getLoginState().successMessage).toBe("If that email is registered, you'll receive a magic link shortly.");
    expect(fetch).toHaveBeenCalledWith(
      '/magic-link',
      expect.objectContaining({
        method: 'POST',
      })
    );
  });
});
