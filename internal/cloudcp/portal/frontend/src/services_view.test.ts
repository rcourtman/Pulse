import { beforeEach, describe, expect, it } from 'vitest';

import {
  renderDeletePanel,
  renderExportPanel,
  renderExportResult,
  renderManagePanel,
  renderOpenPanels,
  renderRefundPanel,
  renderRetrievePanel,
  renderStatus,
} from './services_view';
import type { PortalBootstrapData, RefundState, ServiceStatus, VerificationFlowState } from './types';

function createBootstrap(overrides: Partial<PortalBootstrapData> = {}): PortalBootstrapData {
  return {
    authenticated: true,
    email: 'owner@example.com',
    public_site_url: 'https://pulserelay.pro',
    support_email: 'support@pulserelay.pro',
    commercial_api_base_url: 'https://license.pulserelay.pro',
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
      '<div id="manage-service-panel" class="service-panel"></div>' +
      '<div id="retrieve-service-panel" class="service-panel"></div>' +
      '<div id="refund-service-panel" class="service-panel"></div>' +
      '<div id="data-service-panel" class="service-panel"></div>';

    renderOpenPanels('retrieve-service-panel');

    expect(document.getElementById('retrieve-service-panel')?.classList.contains('visible')).toBe(true);
    expect(document.getElementById('manage-service-panel')?.classList.contains('visible')).toBe(false);
    expect(document.getElementById('refund-service-panel')?.classList.contains('visible')).toBe(false);
  });

  it('renders retrieve panel result state with invoice and token metadata', function() {
    document.body.innerHTML = '<div id="retrieve-service-root"></div>';

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

    expect(copyButton.style.display).toBe('inline-block');
    expect(invoiceLink.href).toBe('https://license.pulserelay.pro/invoices/inv_123');
    expect(invoiceLink.style.display).toBe('inline-block');
    expect(tokenArea.value).toBe('pulse_key_123');
    expect(document.getElementById('retrieve-inline-email-value')?.textContent).toBe('buyer@example.com');
    expect(document.getElementById('retrieve-inline-result')?.style.display).toBe('block');
  });

  it('renders refund and delete panels from owned state', function() {
    document.body.innerHTML =
      '<div id="refund-service-root"></div>' +
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
    expect(document.getElementById('refund-service-root')?.innerHTML).toContain('/refund.html?email=owner%40example.com');
    expect(deleteCheck.checked).toBe(true);
    expect(document.getElementById('data-delete-step2')?.style.display).toBe('block');
  });

  it('renders export panel and updates export result payload visibility', function() {
    document.body.innerHTML =
      '<div id="data-export-root"></div>' +
      '<div id="data-export-result" style="display:none"></div>' +
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
    expect(document.getElementById('data-export-step2')?.style.display).toBe('block');
    expect(document.getElementById('data-export-result')?.style.display).toBe('block');
    expect((document.getElementById('data-export-payload') as HTMLTextAreaElement).value).toBe(
      JSON.stringify(payload, null, 2)
    );
  });

  it('renders manage panel and status classes from service state', function() {
    document.body.innerHTML =
      '<div id="manage-service-root"></div>' +
      '<div id="manage-inline-status" class="service-status"></div>';

    renderManagePanel(
      createFlowState({
        emailValue: 'owner@example.com',
        codeValue: '123456',
        step2Visible: true,
      })
    );
    renderStatus('manage-inline-status', {
      visible: true,
      message: 'Code sent.',
      error: false,
    } satisfies ServiceStatus);

    expect((document.getElementById('manage-inline-email') as HTMLInputElement).value).toBe('owner@example.com');
    expect((document.getElementById('manage-inline-code') as HTMLInputElement).value).toBe('123456');
    expect(document.getElementById('manage-inline-step2')?.style.display).toBe('block');
    expect(document.getElementById('manage-inline-status')?.className).toContain('success');
    expect(document.getElementById('manage-inline-status')?.textContent).toBe('Code sent.');
  });
});
