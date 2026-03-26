import { describe, expect, it } from 'vitest';

import { createBootstrapDefaults, createPortalRuntime, readPortalRuntimeHandoff } from './runtime';

describe('portal runtime', function() {
  it('derives bootstrap defaults from embedded bootstrap data', function() {
    var defaults = createBootstrapDefaults({
      public_site_url: 'https://cloud.pulserelay.pro',
      support_email: 'help@pulserelay.pro',
      commercial_api_base_path: '/api/portal/commercial',
      portal_path: '/account',
      bootstrap_path: '/api/account/bootstrap',
      magic_link_request_path: '/api/public/account/magic-link/request',
      signup_path: '/join',
      logout_path: '/signout',
      account_api_base_path: '/api/account-links',
      portal_api_base_path: '/api/account',
    });

    expect(defaults.public_site_url).toBe('https://cloud.pulserelay.pro');
    expect(defaults.support_email).toBe('help@pulserelay.pro');
    expect(defaults.portal_path).toBe('/account');
    expect(defaults.bootstrap_path).toBe('/api/account/bootstrap');
  });

  it('creates a store from runtime bootstrap input', function() {
    var runtime = createPortalRuntime({
      authenticated: true,
      email: 'owner@example.com',
      accounts: [],
      commercial_api_base_path: '/api/portal/commercial',
    });

    expect(runtime.store.getBootstrap().authenticated).toBe(true);
    expect(runtime.store.getBootstrap().email).toBe('owner@example.com');
    expect(runtime.bootstrapDefaults.commercial_api_base_path).toBe('/api/portal/commercial');
  });

  it('derives canonical email and service handoff from the portal URL', function() {
    var handoff = readPortalRuntimeHandoff('https://cloud.pulserelay.pro/portal?email=buyer%40example.com&service=retrieve');

    expect(handoff.email).toBe('buyer@example.com');
    expect(handoff.openPanelID).toBe('retrieve-service-panel');
  });

  it('applies email and service handoff to the initial portal store', function() {
    var runtime = createPortalRuntime(
      {
        authenticated: false,
        email: '',
        accounts: [],
      },
      {
        email: 'buyer@example.com',
        openPanelID: 'refund-service-panel',
      }
    );

    expect(runtime.handoff.email).toBe('buyer@example.com');
    expect(runtime.store.getLoginState().emailValue).toBe('buyer@example.com');
    expect(runtime.store.getServiceState().refund.emailValue).toBe('buyer@example.com');
    expect(runtime.store.getServiceState().openPanelID).toBe('refund-service-panel');
  });
});
