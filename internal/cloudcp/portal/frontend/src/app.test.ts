import { beforeEach, describe, expect, it, vi } from 'vitest';

import { installPortalApp } from './app';
import { createPortalRuntime } from './runtime';
import { createPortalStore } from './store';
import type { PortalBootstrapData } from './types';

function jsonResponse(payload: unknown, ok = true, status = ok ? 200 : 500) {
  return {
    ok,
    status,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: vi.fn().mockResolvedValue(payload),
    text: vi.fn().mockResolvedValue(typeof payload === 'string' ? payload : JSON.stringify(payload)),
  };
}

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

  it('completes the signed-out magic-link flow through the real shell and auth runtime', async function() {
    var store = createPortalStore(bootstrapDefaults, {
      authenticated: false,
      email: '',
      accounts: [],
    });
    var fetchMock = vi.fn().mockResolvedValue(jsonResponse({}));
    vi.stubGlobal('fetch', fetchMock);

    installPortalApp({
      bootstrapDefaults: bootstrapDefaults,
      store: store,
    });

    var emailInput = document.getElementById('portal-login-email') as HTMLInputElement | null;
    expect(emailInput).not.toBeNull();
    emailInput!.value = 'buyer@example.com';
    emailInput!.dispatchEvent(new Event('input', { bubbles: true }));

    document.getElementById('portal-login-send')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await flushAsync();

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/public/magic-link/request',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'buyer@example.com' }),
      })
    );
    expect(document.getElementById('portal-app-root')?.textContent).toContain('Magic link sent. Check your inbox and click the link to sign in.');
  });

  it('completes the retrieve-license flow through the real authenticated app shell', async function() {
    var accounts = [
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
    ];
    var store = createPortalStore(bootstrapDefaults, {
      authenticated: true,
      email: 'owner@example.com',
      accounts: accounts,
    });
    var fetchMock = vi.fn(async function(input: RequestInfo | URL) {
      var url = String(input);
      if (url === '/api/portal/bootstrap') {
        return jsonResponse({
          authenticated: true,
          email: 'owner@example.com',
          accounts: accounts,
        });
      }
      if (url === 'https://license.pulserelay.pro/v1/retrieve-license/request') {
        return jsonResponse({});
      }
      if (url === 'https://license.pulserelay.pro/v1/retrieve-license') {
        return jsonResponse({
          license: {
            token: 'pulse_token_123',
            tier: 'Pro',
            issued_at: '2026-03-01T10:00:00Z',
            expires_at: null,
            email: 'owner@example.com',
          },
        });
      }
      throw new Error('Unexpected fetch: ' + url);
    });
    vi.stubGlobal('fetch', fetchMock);

    var app = installPortalApp({
      bootstrapDefaults: bootstrapDefaults,
      store: store,
    });
    await app.startupRefresh;

    document.getElementById('open-retrieve-service')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    var retrievePanel = document.getElementById('retrieve-service-panel');
    expect(retrievePanel?.classList.contains('visible')).toBe(true);

    var emailInput = document.getElementById('retrieve-inline-email') as HTMLInputElement | null;
    expect(emailInput?.value).toBe('owner@example.com');

    document.getElementById('retrieve-inline-request')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await flushAsync();

    var codeInput = document.getElementById('retrieve-inline-code') as HTMLInputElement | null;
    expect(codeInput).not.toBeNull();
    codeInput!.value = '123456';
    codeInput!.dispatchEvent(new Event('input', { bubbles: true }));
    document.getElementById('retrieve-inline-confirm')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await flushAsync();

    expect(fetchMock.mock.calls).toContainEqual([
      'https://license.pulserelay.pro/v1/retrieve-license/request',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'owner@example.com' }),
      }),
    ]);
    expect(fetchMock.mock.calls).toContainEqual([
      'https://license.pulserelay.pro/v1/retrieve-license',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'owner@example.com', code: '123456' }),
      }),
    ]);
    expect((store.getServiceState().flows.retrieve.result as { token?: string } | null)?.token).toBe('pulse_token_123');
    expect((document.getElementById('retrieve-inline-result') as HTMLElement | null)?.style.display).toBe('block');
    expect((document.getElementById('retrieve-inline-copy') as HTMLButtonElement | null)?.style.display).toBe('inline-block');
    expect(document.getElementById('retrieve-inline-status')?.textContent).toContain('License retrieved successfully.');
  });

  it('loads team membership through the real authenticated account shell', async function() {
    var accounts = [
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
    ];
    var store = createPortalStore(bootstrapDefaults, {
      authenticated: true,
      email: 'owner@example.com',
      accounts: accounts,
    });
    var fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        authenticated: true,
        email: 'owner@example.com',
        accounts: accounts,
      }))
      .mockResolvedValueOnce(jsonResponse([
        { email: 'owner@example.com', role: 'owner', user_id: 'u_1' },
      ]));
    vi.stubGlobal('fetch', fetchMock);

    var app = installPortalApp({
      bootstrapDefaults: bootstrapDefaults,
      store: store,
    });
    await app.startupRefresh;

    document.querySelector('[data-action="toggle-team"][data-account-id="acct_1"]')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await flushAsync();

    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/accounts/acct_1/members');
    expect(document.getElementById('team-section-acct_1')?.classList.contains('visible')).toBe(true);
    expect(document.getElementById('team-list-acct_1')?.textContent).toContain('owner@example.com');
  });
});
