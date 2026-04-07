import { describe, expect, it } from 'vitest';

import {
  clearFlowStatus,
  createPortalLoginState,
  createPortalBillingState,
  resetVerificationFlowState,
  setFlowStatus,
  syncLoginStateBootstrapEmail,
  syncBillingStateBootstrapEmail,
  toggleBillingPanelState,
  updateDeleteConfirmation,
  updateBillingInputValue,
} from './state';

describe('portal state', function() {
  it('syncs bootstrap email only into empty login state', function() {
    var loginState = createPortalLoginState();
    syncLoginStateBootstrapEmail(loginState, 'first@example.com');
    expect(loginState.emailValue).toBe('first@example.com');
    expect(loginState.request.pending).toBe(false);

    syncLoginStateBootstrapEmail(loginState, 'second@example.com');
    expect(loginState.emailValue).toBe('first@example.com');
  });

  it('creates service state with independent flows and bootstrap email defaults', function() {
    var billingState = createPortalBillingState();
    syncBillingStateBootstrapEmail(billingState, 'owner@example.com');

    expect(billingState.flows.manage.emailValue).toBe('owner@example.com');
    expect(billingState.flows.retrieve.emailValue).toBe('owner@example.com');
    expect(billingState.refund.emailValue).toBe('owner@example.com');
    expect(billingState.flows.manage.request.pending).toBe(false);
    expect(billingState.flows.manage.confirm.pending).toBe(false);
    expect(billingState.refund.submit.pending).toBe(false);
    expect(billingState.upgradeFeatureKey).toBe('');
    expect(billingState.upgradeReturnURL).toBe('');
    expect(billingState.upgradePurchaseReturnToken).toBe('');
    expect(billingState.upgradeCheckoutSessionID).toBe('');
    expect(billingState.upgradeCheckoutStatus).toBe('');
    expect(billingState.upgradePricing.status).toBe('idle');
    expect(billingState.upgradeCheckoutResult.status).toBe('idle');

    billingState.flows.manage.emailValue = 'override@example.com';
    syncBillingStateBootstrapEmail(billingState, 'owner@example.com');
    expect(billingState.flows.manage.emailValue).toBe('override@example.com');
  });

  it('toggles service panels and routes input updates by field kind', function() {
    var billingState = createPortalBillingState();

    toggleBillingPanelState(billingState, 'manage-billing-panel');
    expect(billingState.openBillingPanelID).toBe('manage-billing-panel');
    toggleBillingPanelState(billingState, 'manage-billing-panel');
    expect(billingState.openBillingPanelID).toBe('');

    updateBillingInputValue(billingState, 'retrieve-email', 'buyer@example.com');
    updateBillingInputValue(billingState, 'retrieve-code', '123456');
    updateBillingInputValue(billingState, 'refund-token', 'pulse_token');
    updateDeleteConfirmation(billingState, true);

    expect(billingState.flows.retrieve.emailValue).toBe('buyer@example.com');
    expect(billingState.flows.retrieve.codeValue).toBe('123456');
    expect(billingState.refund.tokenValue).toBe('pulse_token');
    expect(billingState.flows.delete.checkboxChecked).toBe(true);
  });

  it('preserves email while resetting verification flow state', function() {
    var billingState = createPortalBillingState();
    billingState.flows.export.emailValue = 'buyer@example.com';
    billingState.flows.export.codeValue = '999999';
    billingState.flows.export.pendingEmail = 'buyer@example.com';
    billingState.flows.export.result = { ok: true };
    billingState.flows.export.request.pending = true;
    billingState.flows.export.confirm.pending = true;
    setFlowStatus(billingState, 'export', 'done', false);

    resetVerificationFlowState(billingState, 'export');

    expect(billingState.flows.export.emailValue).toBe('buyer@example.com');
    expect(billingState.flows.export.codeValue).toBe('');
    expect(billingState.flows.export.pendingEmail).toBe('');
    expect(billingState.flows.export.result).toBeNull();
    expect(billingState.flows.export.request.pending).toBe(false);
    expect(billingState.flows.export.confirm.pending).toBe(false);
    expect(billingState.flows.export.status.visible).toBe(false);

    setFlowStatus(billingState, 'export', 'broken', true);
    expect(billingState.flows.export.status.message).toBe('broken');
    clearFlowStatus(billingState, 'export');
    expect(billingState.flows.export.status.visible).toBe(false);
  });
});
