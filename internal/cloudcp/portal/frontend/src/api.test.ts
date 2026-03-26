import { beforeEach, describe, expect, it, vi } from 'vitest';

import { createPortalAPI } from './api';
import type { PortalBootstrapData } from './types';

const bootstrap: PortalBootstrapData = {
  authenticated: true,
  email: 'owner@example.com',
  public_site_url: 'https://pulserelay.pro',
  support_email: 'support@pulserelay.pro',
  commercial_api_base_url: 'https://license.pulserelay.pro',
  portal_path: '/portal',
  bootstrap_path: '/api/portal/bootstrap',
  magic_link_request_path: '/api/public/magic-link/request',
  signup_path: '/signup',
  logout_path: '/auth/logout',
  account_api_base_path: '/api/accounts',
  portal_api_base_path: '/api/portal',
  accounts: [],
};

describe('portal api', function() {
  beforeEach(function() {
    vi.restoreAllMocks();
  });

  it('parses bootstrap JSON through the shared api client', async function() {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async function() {
        return {
          authenticated: true,
          email: 'updated@example.com',
          accounts: [],
        };
      },
    }));

    var api = createPortalAPI({
      getBootstrap: function() {
        return bootstrap;
      },
    });

    var result = await api.fetchBootstrap();
    expect(result.email).toBe('updated@example.com');
  });

  it('raises typed api errors with payload-derived messages', async function() {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async function() {
        return { error: 'Member already exists.' };
      },
    }));

    var api = createPortalAPI({
      getBootstrap: function() {
        return bootstrap;
      },
    });

    await expect(api.inviteMember('acct_1', { email: 'member@example.com', role: 'admin' })).rejects.toMatchObject({
      name: 'PortalAPIError',
      status: 409,
      message: 'Member already exists.',
    });
  });
});
