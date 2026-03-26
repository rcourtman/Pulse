import { describe, expect, it } from 'vitest';

import {
  clearFlowStatus,
  createPortalLoginState,
  createPortalServiceState,
  resetVerificationFlowState,
  setFlowStatus,
  syncLoginStateBootstrapEmail,
  syncServiceStateBootstrapEmail,
  toggleServicePanelState,
  updateDeleteConfirmation,
  updateServiceInputValue,
} from './state';

describe('portal state', function() {
  it('syncs bootstrap email only into empty login state', function() {
    var loginState = createPortalLoginState();
    syncLoginStateBootstrapEmail(loginState, 'first@example.com');
    expect(loginState.emailValue).toBe('first@example.com');

    syncLoginStateBootstrapEmail(loginState, 'second@example.com');
    expect(loginState.emailValue).toBe('first@example.com');
  });

  it('creates service state with independent flows and bootstrap email defaults', function() {
    var serviceState = createPortalServiceState();
    syncServiceStateBootstrapEmail(serviceState, 'owner@example.com');

    expect(serviceState.flows.manage.emailValue).toBe('owner@example.com');
    expect(serviceState.flows.retrieve.emailValue).toBe('owner@example.com');
    expect(serviceState.refund.emailValue).toBe('owner@example.com');

    serviceState.flows.manage.emailValue = 'override@example.com';
    syncServiceStateBootstrapEmail(serviceState, 'owner@example.com');
    expect(serviceState.flows.manage.emailValue).toBe('override@example.com');
  });

  it('toggles service panels and routes input updates by field kind', function() {
    var serviceState = createPortalServiceState();

    toggleServicePanelState(serviceState, 'manage-service-panel');
    expect(serviceState.openPanelID).toBe('manage-service-panel');
    toggleServicePanelState(serviceState, 'manage-service-panel');
    expect(serviceState.openPanelID).toBe('');

    updateServiceInputValue(serviceState, 'retrieve-email', 'buyer@example.com');
    updateServiceInputValue(serviceState, 'retrieve-code', '123456');
    updateServiceInputValue(serviceState, 'refund-token', 'pulse_token');
    updateDeleteConfirmation(serviceState, true);

    expect(serviceState.flows.retrieve.emailValue).toBe('buyer@example.com');
    expect(serviceState.flows.retrieve.codeValue).toBe('123456');
    expect(serviceState.refund.tokenValue).toBe('pulse_token');
    expect(serviceState.flows.delete.checkboxChecked).toBe(true);
  });

  it('preserves email while resetting verification flow state', function() {
    var serviceState = createPortalServiceState();
    serviceState.flows.export.emailValue = 'buyer@example.com';
    serviceState.flows.export.codeValue = '999999';
    serviceState.flows.export.pendingEmail = 'buyer@example.com';
    serviceState.flows.export.result = { ok: true };
    setFlowStatus(serviceState, 'export', 'done', false);

    resetVerificationFlowState(serviceState, 'export');

    expect(serviceState.flows.export.emailValue).toBe('buyer@example.com');
    expect(serviceState.flows.export.codeValue).toBe('');
    expect(serviceState.flows.export.pendingEmail).toBe('');
    expect(serviceState.flows.export.result).toBeNull();
    expect(serviceState.flows.export.status.visible).toBe(false);

    setFlowStatus(serviceState, 'export', 'broken', true);
    expect(serviceState.flows.export.status.message).toBe('broken');
    clearFlowStatus(serviceState, 'export');
    expect(serviceState.flows.export.status.visible).toBe(false);
  });
});
