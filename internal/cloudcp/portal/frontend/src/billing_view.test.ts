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
    billingState.upgradeFeatureKey = 'max_monitored_systems';
    billingState.upgradeHandoffURL =
      'https://pulse.example.com/auth/license-purchase-handoff?purchase_handoff_id=pch1_signed';
    billingState.upgradeActivationURLTemplate = 'https://pulse.example.com/auth/license-purchase-activate?purchase_return_token=prt_signed&session_id={CHECKOUT_SESSION_ID}';
    billingState.upgradeHandoff.status = 'ready';
    billingState.upgradeHandoff.data = {
      feature: 'max_monitored_systems',
      activation_url_template: billingState.upgradeActivationURLTemplate,
    };
    billingState.upgradePricing.status = 'ready';
    billingState.upgradePricing.data = {
      title: 'Pricing',
      description: 'Canonical pricing model',
      explainer: 'Pulse counts <strong>monitored systems</strong>.',
      plans: [
        {
          tierKicker: 'Pro+',
          title: 'Pro+',
          price: '$14.99',
          period: '$129/year available too',
          blurb: 'More room.',
          features: [{ tone: 'check', html: 'Up to <strong>50 monitored systems</strong>' }],
          buttons: [
            {
              kind: 'checkout',
              className: 'btn btn-primary',
              tier: 'pro_plus',
              planKey: 'price_pro_plus_annual',
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
      'Pulse Account will return completed checkout directly to Pulse Pro billing.',
    );
    expect(document.getElementById('upgrade-billing-root')?.innerHTML).not.toContain('Activate in Pulse Pro');
    expect(document.getElementById('upgrade-billing-root')?.innerHTML).not.toContain('ppk_live_preview');
    expect(
      (document.querySelector('[data-account-billing-action="upgrade-start-checkout"]') as HTMLButtonElement).disabled,
    ).toBe(false);
  });

  it('renders a blocked checkout state until the Pulse Pro handoff is verified', function() {
    document.body.innerHTML = '<div id="upgrade-billing-root"></div>';

    var billingState = createPortalBillingState();
    billingState.upgradeHandoffURL =
      'https://pulse.example.com/auth/license-purchase-handoff?purchase_handoff_id=pch1_signed';
    billingState.upgradeHandoff.status = 'error';
    billingState.upgradeHandoff.error = 'Pulse Account could not verify the secure return path.';
    billingState.upgradePricing.status = 'ready';
    billingState.upgradePricing.data = {
      title: 'Pricing',
      description: 'Canonical pricing model',
      plans: [
        {
          tierKicker: 'Relay',
          title: 'Relay',
          price: '$4.99',
          period: '$39/year available too',
          blurb: 'Secure remote access and mobile access.',
          features: [{ tone: 'check', html: 'Up to <strong>8 monitored systems</strong>' }],
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
      'Pulse Account could not verify the secure return path.',
    );
    expect(
      (document.querySelector('[data-account-billing-action="upgrade-start-checkout"]') as HTMLButtonElement).disabled,
    ).toBe(true);
  });
});
