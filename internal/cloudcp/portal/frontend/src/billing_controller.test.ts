import { beforeEach, describe, expect, it, vi } from 'vitest';

import { installBillingController } from './billing_controller';

describe('services controller', function() {
  beforeEach(function() {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });

  it('routes service actions to the matching handlers', function() {
    vi.stubGlobal('requestAnimationFrame', function(callback: FrameRequestCallback) {
      callback(0);
      return 1;
    });
    var deps = {
      setShellSection: vi.fn(),
      toggleBillingPanel: vi.fn(),
      clearBillingPanel: vi.fn(),
      focusElement: vi.fn(),
      requestVerificationCode: vi.fn(),
      resendVerificationCode: vi.fn(),
      confirmVerificationCode: vi.fn(),
      copyRetrievedLicense: vi.fn(),
      submitRefund: vi.fn(),
      updateInputValue: vi.fn(),
      updateDeleteConfirmation: vi.fn(),
    };

    installBillingController(deps);

    document.body.innerHTML =
      '<button id="open" data-account-billing-action="open-billing-panel" data-account-billing-panel="retrieve-billing-panel" data-account-billing-focus="retrieve-inline-email">Open</button>' +
      '<button id="close" data-account-billing-action="clear-billing-panel">Close</button>' +
      '<button id="request" data-account-billing-action="manage-inline-request">Request</button>' +
      '<button id="resend" data-account-billing-action="data-delete-resend">Resend</button>' +
      '<button id="confirm" data-account-billing-action="retrieve-inline-confirm">Confirm</button>' +
      '<button id="copy" data-account-billing-action="retrieve-inline-copy">Copy</button>' +
      '<button id="refund" data-account-billing-action="refund-inline-submit">Refund</button>' +
      '<div id="retrieve-billing-panel"></div>';

    var scrollIntoView = vi.fn();
    Object.defineProperty(document.getElementById('retrieve-billing-panel') as HTMLElement, 'scrollIntoView', {
      value: scrollIntoView,
      configurable: true,
    });

    document.getElementById('open')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(deps.setShellSection).toHaveBeenCalledWith('billing');
    expect(deps.toggleBillingPanel).toHaveBeenCalledWith('retrieve-billing-panel');
    expect(deps.focusElement).toHaveBeenCalledWith('retrieve-inline-email');
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', inline: 'nearest' });

    document.getElementById('close')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(deps.clearBillingPanel).toHaveBeenCalled();

    document.getElementById('request')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(deps.requestVerificationCode).toHaveBeenCalledWith('manage');

    document.getElementById('resend')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(deps.resendVerificationCode).toHaveBeenCalledWith('delete', expect.any(MouseEvent));

    document.getElementById('confirm')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(deps.confirmVerificationCode).toHaveBeenCalledWith('retrieve');

    document.getElementById('copy')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(deps.copyRetrievedLicense).toHaveBeenCalled();

    document.getElementById('refund')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(deps.submitRefund).toHaveBeenCalled();
  });

  it('routes input and checkbox changes into state update hooks', function() {
    var deps = {
      setShellSection: vi.fn(),
      toggleBillingPanel: vi.fn(),
      clearBillingPanel: vi.fn(),
      focusElement: vi.fn(),
      requestVerificationCode: vi.fn(),
      resendVerificationCode: vi.fn(),
      confirmVerificationCode: vi.fn(),
      copyRetrievedLicense: vi.fn(),
      submitRefund: vi.fn(),
      updateInputValue: vi.fn(),
      updateDeleteConfirmation: vi.fn(),
    };

    installBillingController(deps);

    document.body.innerHTML =
      '<input id="retrieve-email" data-account-billing-input="retrieve-email">' +
      '<input id="data-delete-confirm-check" type="checkbox">';

    var emailInput = document.getElementById('retrieve-email') as HTMLInputElement;
    emailInput.value = 'buyer@example.com';
    emailInput.dispatchEvent(new Event('input', { bubbles: true }));
    expect(deps.updateInputValue).toHaveBeenCalledWith('retrieve-email', 'buyer@example.com');

    var checkbox = document.getElementById('data-delete-confirm-check') as HTMLInputElement;
    checkbox.checked = true;
    checkbox.dispatchEvent(new Event('change', { bubbles: true }));
    expect(deps.updateDeleteConfirmation).toHaveBeenCalledWith(true);
  });
});
