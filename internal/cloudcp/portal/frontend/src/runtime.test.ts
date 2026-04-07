import { describe, expect, it } from 'vitest';

import { createBootstrapDefaults, createPortalRuntime, readPortalRuntimeHandoff } from './runtime';

describe('portal runtime', function() {
  it('derives bootstrap defaults from embedded bootstrap data', function() {
    var defaults = createBootstrapDefaults({
      has_self_hosted_commercial: true,
      public_site_url: 'https://cloud.pulserelay.pro',
      support_email: 'help@pulserelay.pro',
      commercial_api_base_url: '/api/portal/commercial',
      portal_path: '/account',
      bootstrap_path: '/api/account/bootstrap',
      magic_link_request_path: '/api/public/account/magic-link/request',
      signup_path: '/join',
      logout_path: '/signout',
      account_api_base_path: '/api/account-links',
      portal_api_base_path: '/api/account',
    });

    expect(defaults.public_site_url).toBe('https://cloud.pulserelay.pro');
    expect(defaults.has_self_hosted_commercial).toBe(true);
    expect(defaults.support_email).toBe('help@pulserelay.pro');
    expect(defaults.portal_path).toBe('/account');
    expect(defaults.bootstrap_path).toBe('/api/account/bootstrap');
  });

  it('keeps local preview API roots intact for harness-driven development', function() {
    var defaults = createBootstrapDefaults({
      has_self_hosted_commercial: false,
      commercial_api_base_url: '/__portal_preview/commercial',
      bootstrap_path: '/api/portal/bootstrap',
      account_api_base_path: '/api/accounts',
      portal_api_base_path: '/api/portal',
    });

    expect(defaults.commercial_api_base_url).toBe('/__portal_preview/commercial');
    expect(defaults.bootstrap_path).toBe('/api/portal/bootstrap');
    expect(defaults.account_api_base_path).toBe('/api/accounts');
    expect(defaults.portal_api_base_path).toBe('/api/portal');
  });

  it('preserves an explicitly empty signup path', function() {
    var defaults = createBootstrapDefaults({
      signup_path: '',
    });

    expect(defaults.signup_path).toBe('');
  });

  it('creates a store from runtime bootstrap input', function() {
    var runtime = createPortalRuntime({
      authenticated: true,
      email: 'owner@example.com',
      has_self_hosted_commercial: false,
      accounts: [],
      commercial_api_base_url: '/api/portal/commercial',
    });

    expect(runtime.store.getBootstrap().authenticated).toBe(true);
    expect(runtime.store.getBootstrap().email).toBe('owner@example.com');
    expect(runtime.bootstrapDefaults.commercial_api_base_url).toBe('/api/portal/commercial');
  });

  it('derives canonical email and billing handoff from the portal URL', function() {
    var handoff = readPortalRuntimeHandoff(
      'https://cloud.pulserelay.pro/portal?email=buyer%40example.com&service=upgrade&purchase_handoff_url=' +
        encodeURIComponent('https://pulse.example.com/auth/license-purchase-handoff?purchase_handoff_id=pch1_signed') +
        '&checkout=cancelled',
    );

    expect(handoff.email).toBe('buyer@example.com');
    expect(handoff.openBillingPanelID).toBe('upgrade-billing-panel');
    expect(handoff.upgradeHandoffURL).toBe(
      'https://pulse.example.com/auth/license-purchase-handoff?purchase_handoff_id=pch1_signed',
    );
    expect(handoff.upgradeFeatureKey).toBe('');
    expect(handoff.upgradeCheckoutStatus).toBe('cancelled');
  });

  it('applies email and billing handoff to the initial portal store', function() {
    var runtime = createPortalRuntime(
      {
        authenticated: false,
        email: '',
        has_self_hosted_commercial: false,
        accounts: [],
      },
      {
        email: 'buyer@example.com',
        openBillingPanelID: 'refund-billing-panel',
        upgradeHandoffURL: '',
        upgradeFeatureKey: '',
        upgradeCheckoutStatus: '',
      }
    );

    expect(runtime.handoff.email).toBe('buyer@example.com');
    expect(runtime.store.getLoginState().emailValue).toBe('buyer@example.com');
    expect(runtime.store.getBillingState().refund.emailValue).toBe('buyer@example.com');
    expect(runtime.store.getBillingState().openBillingPanelID).toBe('refund-billing-panel');
  });

  it('promotes upgrade handoff intents into the billing shell state', function() {
    var runtime = createPortalRuntime(
      {
        authenticated: false,
        email: '',
        has_self_hosted_commercial: false,
        accounts: [],
      },
      {
        email: '',
        openBillingPanelID: 'upgrade-billing-panel',
        upgradeHandoffURL:
          'https://pulse.example.com/auth/license-purchase-handoff?purchase_handoff_id=pch1_signed',
        upgradeFeatureKey: '',
        upgradeCheckoutStatus: 'cancelled',
      }
    );

    expect(runtime.store.getShellState().activeSection).toBe('billing');
    expect(runtime.store.getBillingState().openBillingPanelID).toBe('upgrade-billing-panel');
    expect(runtime.store.getBillingState().upgradeHandoffURL).toBe(
      'https://pulse.example.com/auth/license-purchase-handoff?purchase_handoff_id=pch1_signed',
    );
    expect(runtime.store.getBillingState().upgradeFeatureKey).toBe('');
    expect(runtime.store.getBillingState().upgradeCheckoutStatus).toBe('cancelled');
  });
});
