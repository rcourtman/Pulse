import { beforeEach, describe, expect, it, vi } from 'vitest';

import { installServicesController } from './services_controller';

describe('services controller', function() {
  beforeEach(function() {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });

  it('routes service actions to the matching handlers', function() {
    var deps = {
      toggleServicePanel: vi.fn(),
      focusElement: vi.fn(),
      requestVerificationCode: vi.fn(),
      resendVerificationCode: vi.fn(),
      confirmVerificationCode: vi.fn(),
      copyRetrievedLicense: vi.fn(),
      submitRefund: vi.fn(),
      updateInputValue: vi.fn(),
      updateDeleteConfirmation: vi.fn(),
    };

    installServicesController(deps);

    document.body.innerHTML =
      '<button id="open" data-account-service-action="open-service-panel" data-account-service-panel="retrieve-service-panel" data-account-service-focus="retrieve-inline-email">Open</button>' +
      '<button id="request" data-account-service-action="manage-inline-request">Request</button>' +
      '<button id="resend" data-account-service-action="data-delete-resend">Resend</button>' +
      '<button id="confirm" data-account-service-action="retrieve-inline-confirm">Confirm</button>' +
      '<button id="copy" data-account-service-action="retrieve-inline-copy">Copy</button>' +
      '<button id="refund" data-account-service-action="refund-inline-submit">Refund</button>';

    document.getElementById('open')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(deps.toggleServicePanel).toHaveBeenCalledWith('retrieve-service-panel');
    expect(deps.focusElement).toHaveBeenCalledWith('retrieve-inline-email');

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
      toggleServicePanel: vi.fn(),
      focusElement: vi.fn(),
      requestVerificationCode: vi.fn(),
      resendVerificationCode: vi.fn(),
      confirmVerificationCode: vi.fn(),
      copyRetrievedLicense: vi.fn(),
      submitRefund: vi.fn(),
      updateInputValue: vi.fn(),
      updateDeleteConfirmation: vi.fn(),
    };

    installServicesController(deps);

    document.body.innerHTML =
      '<input id="retrieve-email" data-account-service-input="retrieve-email">' +
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
