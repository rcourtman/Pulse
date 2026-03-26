import { beforeEach, describe, expect, it, vi } from 'vitest';

import { installPortalApp } from './app';
import { createBootstrapDefaults, createPortalRuntime } from './runtime';
import { createPortalStore } from './store';
import type { PortalBootstrapData } from './types';

const bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'> = {
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
};

describe('portal app', function() {
  beforeEach(function() {
    document.body.innerHTML = `
      <div id="portal-user-info"></div>
      <div id="portal-app-root"></div>
      <div id="toast"></div>
    `;
    vi.restoreAllMocks();
  });

  it('refreshes authenticated startup bootstrap through the owned app entrypoint', async function() {
    var store = createPortalStore(bootstrapDefaults, {
      authenticated: true,
      email: 'owner@example.com',
      accounts: [],
    });

    var fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async function() {
        return {
          authenticated: true,
          email: 'updated@example.com',
          accounts: [],
        };
      },
    });
    vi.stubGlobal('fetch', fetchMock);

    var app = installPortalApp({
      bootstrapDefaults: bootstrapDefaults,
      store: store,
    });

    await app.startupRefresh;

    expect(fetchMock).toHaveBeenCalledWith('/api/portal/bootstrap', {
      headers: { Accept: 'application/json' },
    });
    expect(store.getBootstrap().email).toBe('updated@example.com');
  });

  it('falls back to anonymous bootstrap when startup refresh is unauthorized', async function() {
    var store = createPortalStore(bootstrapDefaults, {
      authenticated: true,
      email: 'owner@example.com',
      accounts: [
        {
          id: 'acct_1',
          name: 'Acme MSP',
          kind: 'msp',
          kind_label: 'MSP',
          role: 'owner',
          can_manage: true,
          has_billing: true,
          workspaces: [],
        },
      ],
    });

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
    }));

    var app = installPortalApp({
      bootstrapDefaults: bootstrapDefaults,
      store: store,
    });

    await app.startupRefresh;

    expect(store.getBootstrap().authenticated).toBe(false);
    expect(store.getBootstrap().email).toBe('');
    expect(store.getBootstrap().accounts).toEqual([]);
  });

  it('starts from the owned runtime factory instead of import-time globals', function() {
    document.body.innerHTML = `
      <script id="pulse-account-bootstrap" type="application/json">
        {"authenticated":true,"email":"owner@example.com","public_site_url":"https://cloud.pulserelay.pro","accounts":[]}
      </script>
      <div id="portal-user-info"></div>
      <div id="portal-app-root"></div>
      <div id="toast"></div>
    `;

    var runtime = createPortalRuntime({
      authenticated: true,
      email: 'owner@example.com',
      public_site_url: 'https://cloud.pulserelay.pro',
      accounts: [],
    });

    expect(runtime.bootstrapDefaults.public_site_url).toBe('https://cloud.pulserelay.pro');
    expect(runtime.store.getBootstrap().authenticated).toBe(true);
    expect(runtime.store.getBootstrap().email).toBe('owner@example.com');
  });
});
