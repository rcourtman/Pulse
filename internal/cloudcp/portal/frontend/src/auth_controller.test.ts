import { beforeEach, describe, expect, it, vi } from 'vitest';

import { installAuthController } from './auth_controller';

describe('auth controller', function() {
  beforeEach(function() {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });

  it('syncs bootstrap email without overwriting local input state', function() {
    var controller = installAuthController({
      getMagicLinkRequestPath: function() {
        return '/magic-link';
      },
      getLogoutPath: function() {
        return '/logout';
      },
      getPortalPath: function() {
        return '/portal';
      },
      renderPortal: function() {},
    });

    controller.syncBootstrapEmail('buyer@example.com');
    expect(controller.getLoginState().emailValue).toBe('buyer@example.com');

    controller.getLoginState().emailValue = 'typed@example.com';
    controller.syncBootstrapEmail('other@example.com');
    expect(controller.getLoginState().emailValue).toBe('typed@example.com');
  });

  it('tracks login email input and completes magic-link request state', async function() {
    var renderPortal = vi.fn();
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
      })
    );

    var controller = installAuthController({
      getMagicLinkRequestPath: function() {
        return '/magic-link';
      },
      getLogoutPath: function() {
        return '/logout';
      },
      getPortalPath: function() {
        return '/portal';
      },
      renderPortal,
    });

    document.body.innerHTML =
      '<input id="portal-login-email" data-portal-input="login-email">' +
      '<button id="send" data-portal-action="send-magic-link">Send</button>';

    var input = document.getElementById('portal-login-email') as HTMLInputElement;
    input.value = 'buyer@example.com';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    expect(controller.getLoginState().emailValue).toBe('buyer@example.com');

    document.getElementById('send')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    await Promise.resolve();
    await Promise.resolve();
    expect(controller.getLoginState().sending).toBe(false);
    expect(controller.getLoginState().success).toBe(true);
    expect(renderPortal).toHaveBeenCalled();
    expect(fetch).toHaveBeenCalledWith(
      '/magic-link',
      expect.objectContaining({
        method: 'POST',
      })
    );
  });
});
