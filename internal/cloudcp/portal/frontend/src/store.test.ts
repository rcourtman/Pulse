import { describe, expect, it, vi } from 'vitest';

import { createAnonymousBootstrap, createPortalStore, normalizeBootstrap } from './store';
import type { PortalBootstrapData } from './types';

const bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'> = {
  has_self_hosted_commercial: false,
  public_site_url: 'https://pulserelay.pro',
  support_email: 'support@pulserelay.pro',
  commercial_api_base_url: '/api/portal/commercial',
  portal_path: '/portal',
  bootstrap_path: '/api/portal/bootstrap',
  magic_link_request_path: '/api/public/magic-link/request',
  signup_path: '/signup',
  logout_path: '/auth/logout',
  account_api_base_path: '/api/accounts',
  portal_api_base_path: '/api/portal',
};

describe('portal store', function() {
  it('normalizes anonymous bootstrap state from defaults', function() {
    var bootstrap = createAnonymousBootstrap(bootstrapDefaults, {
      signup_path: '/join',
    });

    expect(bootstrap.authenticated).toBe(false);
    expect(bootstrap.email).toBe('');
    expect(bootstrap.signup_path).toBe('/join');
    expect(bootstrap.accounts).toEqual([]);
  });

  it('normalizes partial bootstrap payloads into tracked account state', function() {
    var bootstrap = normalizeBootstrap(bootstrapDefaults, {
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
          members: [],
          workspaces: [],
        },
      ],
    });

    expect(bootstrap.authenticated).toBe(true);
    expect(bootstrap.email).toBe('owner@example.com');
    expect(bootstrap.accounts).toHaveLength(1);
    expect(bootstrap.portal_path).toBe('/portal');
  });

  it('publishes bootstrap changes through a subscription boundary', function() {
    var store = createPortalStore(bootstrapDefaults, null);
    var listener = vi.fn();
    var unsubscribe = store.subscribeBootstrap(listener);

    store.setBootstrap({
      authenticated: true,
      email: 'owner@example.com',
    });

    expect(listener).toHaveBeenCalledTimes(1);
    expect(store.getBootstrap().authenticated).toBe(true);
    expect(store.getBootstrap().email).toBe('owner@example.com');

    unsubscribe();
    store.setBootstrap({
      authenticated: false,
      email: '',
    });

    expect(listener).toHaveBeenCalledTimes(1);
  });

  it('seeds hosted access state from the bootstrap roster snapshot', function() {
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
          members: [
            { email: 'owner@example.com', role: 'owner', user_id: 'u_1', created_at: '2026-03-28T10:00:00Z' },
          ],
          workspaces: [],
        },
      ],
    });

    var entry = store.getAccountState().byAccountID.acct_1;
    expect(entry.accessQuery.status).toBe('ready');
    expect(entry.accessQuery.error).toBe('');
    expect(entry.accessQuery.data).toEqual([
      { email: 'owner@example.com', role: 'owner', user_id: 'u_1', created_at: '2026-03-28T10:00:00Z' },
    ]);

    store.setBootstrap({
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
          members: [
            { email: 'admin@example.com', role: 'admin', user_id: 'u_2', created_at: '2026-03-28T11:00:00Z' },
          ],
          workspaces: [],
        },
      ],
    });

    expect(store.getAccountState().byAccountID.acct_1.accessQuery.data).toEqual([
      { email: 'admin@example.com', role: 'admin', user_id: 'u_2', created_at: '2026-03-28T11:00:00Z' },
    ]);
  });

  it('keeps login and service state in the same owned store with selective notifications', function() {
    var store = createPortalStore(bootstrapDefaults, null);
    var loginListener = vi.fn();
    var billingListener = vi.fn();

    store.subscribeLogin(loginListener);
    store.subscribeBilling(billingListener);

    store.updateLoginState(function(loginState) {
      loginState.emailValue = 'typed@example.com';
    }, { notify: false });
    store.updateBillingState(function(billingState) {
      billingState.openBillingPanelID = 'retrieve-billing-panel';
    });

    expect(store.getLoginState().emailValue).toBe('typed@example.com');
    expect(store.getBillingState().openBillingPanelID).toBe('retrieve-billing-panel');
    expect(loginListener).not.toHaveBeenCalled();
    expect(billingListener).toHaveBeenCalledTimes(1);
  });
});
