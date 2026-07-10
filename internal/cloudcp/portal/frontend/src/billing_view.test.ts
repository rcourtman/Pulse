import { beforeEach, describe, expect, it } from 'vitest';

import {
  renderDeletePanel,
  renderExportPanel,
  renderExportResult,
  renderManagePanel,
  renderOpenBillingPanels,
  renderRefundPanel,
  renderRetrievePanel,
  renderBillingStatus,
  renderUpgradePanel,
} from './billing_view';
import type { PortalBootstrapData, RefundState, BillingStatus, VerificationFlowState } from './types';
import { createPortalBillingState } from './state';

function createBootstrap(overrides: Partial<PortalBootstrapData> = {}): PortalBootstrapData {
  return {
    authenticated: true,
    email: 'owner@example.com',
    has_self_hosted_commercial: true,
    email_sign_in_available: true,
    provider_hosted_mode: false,
    public_site_url: 'https://pulserelay.pro',
    support_email: 'support@pulserelay.pro',
    commercial_api_base_url: '/api/portal/commercial',
    portal_path: '/portal',
    bootstrap_path: '/api/portal/bootstrap',
    magic_link_request_path: '/auth/magic-link',
    signup_path: '/signup',
    logout_path: '/auth/logout',
    account_api_base_path: '/api/accounts',
    portal_api_base_path: '/api/portal',
    accounts: [],
    ...overrides,
  };
}

function createFlowState(overrides: Partial<VerificationFlowState> = {}): VerificationFlowState {
  return {
    pendingEmail: '',
    request: {
      pending: false,
      error: '',
    },
    confirm: {
      pending: false,
      error: '',
    },
    step2Visible: false,
    status: {
      visible: false,
      message: '',
      error: false,
    },
    result: null,
    emailValue: '',
    codeValue: '',
    checkboxChecked: false,
    ...overrides,
  };
}

function createRefundState(overrides: Partial<RefundState> = {}): RefundState {
  return {
    emailValue: '',
    tokenValue: '',
    submit: {
      pending: false,
      error: '',
    },
    status: {
      visible: false,
      message: '',
      error: false,
    },
    ...overrides,
  };
}

describe('services view', function() {
  beforeEach(function() {
    document.body.innerHTML = '';
  });

  it('toggles the visible service panel by id', function() {
    document.body.innerHTML =
      '<div class="billing-shell billing-shell-idle">' +
      '<div id="billing-detail-shell" hidden></div>' +
      '<article class="billing-action-row"><button data-account-billing-action="open-billing-panel" data-account-billing-panel="manage-billing-panel"></button></article>' +
      '<article class="billing-action-row"><button data-account-billing-action="open-billing-panel" data-account-billing-panel="retrieve-billing-panel"></button></article>' +
      '<div id="manage-billing-panel" class="billing-panel" hidden></div>' +
      '<div id="retrieve-billing-panel" class="billing-panel" hidden></div>' +
      '<div id="refund-billing-panel" class="billing-panel" hidden></div>' +
      '<div id="data-billing-panel" class="billing-panel" hidden></div>' +
      '</div>';

    renderOpenBillingPanels('retrieve-billing-panel');

    expect(document.querySelector('.billing-shell')?.classList.contains('billing-shell-job-open')).toBe(true);
    expect((document.getElementById('billing-detail-shell') as HTMLElement).hidden).toBe(false);
    expect(document.getElementById('retrieve-billing-panel')?.classList.contains('visible')).toBe(true);
    expect((document.getElementById('retrieve-billing-panel') as HTMLElement).hidden).toBe(false);
    expect(document.getElementById('manage-billing-panel')?.classList.contains('visible')).toBe(false);
    expect(document.getElementById('refund-billing-panel')?.classList.contains('visible')).toBe(false);
    expect(document.querySelectorAll('.billing-action-row.active')).toHaveLength(1);

    renderOpenBillingPanels('');
    expect(document.querySelector('.billing-shell')?.classList.contains('billing-shell-idle')).toBe(true);
    expect((document.getElementById('billing-detail-shell') as HTMLElement).hidden).toBe(true);
  });

  it('renders retrieve panel result state with invoice and token metadata', function() {
    document.body.innerHTML = '<div id="retrieve-billing-root"></div>';

    renderRetrievePanel(
      createFlowState({
        emailValue: 'buyer@example.com',
        codeValue: '123456',
        step2Visible: true,
        result: {
          invoice_url: 'https://license.pulserelay.pro/invoices/inv_123',
          token: 'pulse_key_123',
          tier: 'Relay',
          issued_at: '2026-03-26T10:00:00Z',
          expires_at: null,
          email: 'buyer@example.com',
        },
      })
    );

    var copyButton = document.getElementById('retrieve-inline-copy') as HTMLButtonElement;
    var invoiceLink = document.getElementById('retrieve-inline-invoice') as HTMLAnchorElement;
    var tokenArea = document.getElementById('retrieve-inline-token') as HTMLTextAreaElement;

    expect(copyButton.hidden).toBe(false);
    expect(invoiceLink.href).toBe('https://license.pulserelay.pro/invoices/inv_123');
    expect(invoiceLink.hidden).toBe(false);
    expect(tokenArea.value).toBe('pulse_key_123');
    expect(document.getElementById('retrieve-inline-email-value')?.textContent).toBe('buyer@example.com');
    expect((document.getElementById('retrieve-inline-result') as HTMLElement).hidden).toBe(false);
  });

  it('renders refund and delete panels from owned state', function() {
    document.body.innerHTML =
      '<div id="refund-billing-root"></div>' +
      '<div id="data-delete-root"></div>';

    renderRefundPanel(
      createRefundState({
        emailValue: 'owner@example.com',
        tokenValue: 'pulse_token',
      }),
      createBootstrap()
    );
    renderDeletePanel(
      createFlowState({
        emailValue: 'owner@example.com',
        codeValue: '654321',
        step2Visible: true,
        checkboxChecked: true,
      })
    );

    var refundEmail = document.getElementById('refund-inline-email') as HTMLInputElement;
    var refundToken = document.getElementById('refund-inline-token') as HTMLInputElement;
    var deleteCheck = document.getElementById('data-delete-confirm-check') as HTMLInputElement;

    expect(refundEmail.value).toBe('owner@example.com');
    expect(refundToken.value).toBe('pulse_token');
    expect(document.getElementById('refund-billing-root')?.innerHTML).toContain('/refund.html?email=owner%40example.com');
    expect(deleteCheck.checked).toBe(true);
    expect((document.getElementById('data-delete-step2') as HTMLElement).hidden).toBe(false);
  });

  it('renders export panel and updates export result payload visibility', function() {
    document.body.innerHTML =
      '<div id="data-export-root"></div>' +
      '<div id="data-export-result" hidden></div>' +
      '<textarea id="data-export-payload"></textarea>';

    renderExportPanel(
      createFlowState({
        emailValue: 'owner@example.com',
        step2Visible: true,
        result: {
          accounts: 2,
        },
      })
    );

    var payload = { accounts: 3, workspaces: 5 };
    renderExportResult(payload);

    expect((document.getElementById('data-export-email') as HTMLInputElement).value).toBe('owner@example.com');
    expect((document.getElementById('data-export-step2') as HTMLElement).hidden).toBe(false);
    expect((document.getElementById('data-export-result') as HTMLElement).hidden).toBe(false);
    expect((document.getElementById('data-export-payload') as HTMLTextAreaElement).value).toBe(
      JSON.stringify(payload, null, 2)
    );
  });

  it('renders manage panel and status classes from service state', function() {
    document.body.innerHTML =
      '<div id="manage-billing-root"></div>' +
      '<div id="manage-inline-status" class="billing-status"></div>';

    renderManagePanel(
      createFlowState({
        emailValue: 'owner@example.com',
        codeValue: '123456',
        step2Visible: true,
      })
    );
    renderBillingStatus('manage-inline-status', {
      visible: true,
      message: 'Code sent.',
      error: false,
    } satisfies BillingStatus);

    expect((document.getElementById('manage-inline-email') as HTMLInputElement).value).toBe('owner@example.com');
    expect((document.getElementById('manage-inline-code') as HTMLInputElement).value).toBe('123456');
    expect((document.getElementById('manage-inline-step2') as HTMLElement).hidden).toBe(false);
    expect(document.getElementById('manage-inline-status')?.className).toContain('success');
    expect(document.getElementById('manage-inline-status')?.textContent).toBe('Code sent.');
  });

  it('renders upgrade panel with verified direct-return checkout actions', function() {
    document.body.innerHTML = '<div id="upgrade-billing-root"></div>';

    var billingState = createPortalBillingState();
    billingState.upgradeFeatureKey = 'self_hosted_plan';
    billingState.upgradePortalHandoffID = 'cph_signed';
    billingState.upgradePortalHandoff.status = 'ready';
    billingState.upgradePortalHandoff.data = {
      portal_handoff_id: 'cph_signed',
      feature: 'self_hosted_plan',
      status: 'resolved',
    };
    billingState.upgradePricing.status = 'ready';
    billingState.upgradePricing.data = {
      title: 'Pricing',
      description: 'Canonical pricing model',
      explainer:
        'Community keeps core monitoring free. Relay gets your Pulse web UI securely reachable from anywhere. Pro adds Patrol control, alert investigation, verified fixes, and 90-day history.',
      plans: [
        {
          tierKicker: 'Pro',
          title: 'Pro',
          price: '$79/year',
          period: 'or $8.99/month',
          blurb: 'The operator tier for Patrol control, alert investigation, verified fixes, and 90-day history.',
          features: [{ tone: 'check', html: 'Core <strong>self-hosted monitoring</strong> included' }],
          buttons: [
            {
              kind: 'checkout',
              className: 'btn btn-primary',
              tier: 'pro',
              planKey: 'price_pro_annual',
              billingCycle: 'annual',
              label: 'Buy Annual',
            },
          ],
        },
      ],
    };

    renderUpgradePanel(billingState, createBootstrap());

    expect(document.getElementById('upgrade-billing-root')?.innerHTML).toContain('Buy Annual');
    expect(document.getElementById('upgrade-billing-root')?.innerHTML).toContain(
      'Pulse Account will return completed checkout directly to the Plans page in Pulse.',
    );
    expect(document.getElementById('upgrade-billing-root')?.innerHTML).toContain(
      'Pulse Account keeps checkout tied to the Pulse instance that opened it, so completed Relay or Pro purchases return to the right Plans page automatically.',
    );
    expect(document.getElementById('upgrade-billing-root')?.innerHTML).toContain(
      'Community keeps core monitoring free. Relay gets your Pulse web UI securely reachable from anywhere. Pro adds Patrol control, alert investigation, verified fixes, and 90-day history.',
    );
    expect(document.getElementById('upgrade-billing-root')?.innerHTML).not.toMatch(
      /Unlimited[\s\S]{0,80}self-hosted monitoring/i,
    );
    expect(document.getElementById('upgrade-billing-root')?.innerHTML).not.toContain('Continue to Plans');
    expect(document.getElementById('upgrade-billing-root')?.innerHTML).not.toContain('ppk_live_preview');
    expect(
      (document.querySelector('[data-account-billing-action="upgrade-start-checkout"]') as HTMLButtonElement).disabled,
    ).toBe(false);
  });

  it('renders a blocked checkout state until the plan handoff is verified', function() {
    document.body.innerHTML = '<div id="upgrade-billing-root"></div>';

    var billingState = createPortalBillingState();
    billingState.upgradePortalHandoffID = 'cph_signed';
    billingState.upgradePortalHandoff.status = 'error';
    billingState.upgradePortalHandoff.error = 'Pulse Account could not verify the secure plan upgrade handoff.';
    billingState.upgradePricing.status = 'ready';
    billingState.upgradePricing.data = {
      title: 'Pricing',
      description: 'Canonical pricing model',
      plans: [
        {
          tierKicker: 'Relay',
          title: 'Relay',
          price: '$39/year',
          period: 'or $4.99/month',
          blurb: 'Secure remote web access and mobile app pairing.',
          features: [{ tone: 'check', html: 'Core <strong>self-hosted monitoring</strong> included' }],
          buttons: [
            {
              kind: 'checkout',
              className: 'btn btn-primary',
              tier: 'relay',
              planKey: 'price_relay_annual',
              billingCycle: 'annual',
              label: 'Buy Annual',
            },
          ],
        },
      ],
    };

    renderUpgradePanel(billingState, createBootstrap());

    expect(document.getElementById('upgrade-billing-root')?.innerHTML).toContain(
      'Pulse Account could not verify the secure plan upgrade handoff.',
    );
    expect(
      (document.querySelector('[data-account-billing-action="upgrade-start-checkout"]') as HTMLButtonElement).disabled,
    ).toBe(true);
  });

  it('blocks a completed secure upgrade handoff from starting checkout again', function() {
    document.body.innerHTML = '<div id="upgrade-billing-root"></div>';

    var billingState = createPortalBillingState();
    billingState.upgradePortalHandoffID = 'cph_completed';
    billingState.upgradePortalHandoff.status = 'ready';
    billingState.upgradePortalHandoff.data = {
      portal_handoff_id: 'cph_completed',
      feature: 'self_hosted_plan',
      status: 'completed',
    };
    billingState.upgradePricing.status = 'ready';
    billingState.upgradePricing.data = {
      title: 'Pricing',
      description: 'Canonical pricing model',
      plans: [
        {
          tierKicker: 'Pro',
          title: 'Pro',
          price: '$8.99',
          period: '$79/year available too',
          blurb: 'Patrol control, alert investigation, and verified fixes.',
          features: [{ tone: 'check', html: 'Core <strong>self-hosted monitoring</strong> included' }],
          buttons: [
            {
              kind: 'checkout',
              className: 'btn btn-primary',
              tier: 'pro',
              planKey: 'price_pro_annual',
              billingCycle: 'annual',
              label: 'Buy Annual',
            },
          ],
        },
      ],
    };

    renderUpgradePanel(billingState, createBootstrap());

    expect(document.getElementById('upgrade-billing-root')?.innerHTML).toContain(
      'This secure upgrade handoff already completed. Return to the Plans page in Pulse to review the live plan state.',
    );
    expect(
      (document.querySelector('[data-account-billing-action="upgrade-start-checkout"]') as HTMLButtonElement).disabled,
    ).toBe(true);
  });

  it('blocks legacy checkout-intent arrivals without a verified portal handoff', function() {
    document.body.innerHTML = '<div id="upgrade-billing-root"></div>';

    var billingState = createPortalBillingState();
    billingState.upgradePricing.status = 'ready';
    billingState.upgradePricing.data = {
      title: 'Pricing',
      description: 'Canonical pricing model',
      plans: [
        {
          tierKicker: 'Pro',
          title: 'Pro',
          price: '$8.99',
          period: '$79/year available too',
          blurb: 'Patrol control, alert investigation, and verified fixes.',
          features: [{ tone: 'check', html: 'Core <strong>self-hosted monitoring</strong> included' }],
          buttons: [
            {
              kind: 'checkout',
              className: 'btn btn-primary',
              tier: 'pro',
              planKey: 'price_pro_annual',
              billingCycle: 'annual',
              label: 'Buy Annual',
            },
          ],
        },
      ],
    };

    renderUpgradePanel(billingState, createBootstrap());

    expect(document.getElementById('upgrade-billing-root')?.innerHTML).toContain(
      'Open this upgrade from the Plans page in Pulse so Pulse Account can verify the secure plan upgrade handoff before checkout.',
    );
    expect(
      (document.querySelector('[data-account-billing-action="upgrade-start-checkout"]') as HTMLButtonElement).disabled,
    ).toBe(true);
  });
});
