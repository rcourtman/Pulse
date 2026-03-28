import type { PortalBootstrapData, RefundState, BillingStatus, VerificationFlowState } from './types';

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

export function getElement<T extends HTMLElement = HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null;
}

export function asHTMLElement(target: EventTarget | null): HTMLElement | null {
  return target instanceof HTMLElement ? target : null;
}

export function escapeText(value: unknown): string {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

export function escapeAttribute(value: unknown): string {
  return escapeText(value)
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

export function readValue(id: string): string {
  var el = getElement<FormValueElement>(id);
  return el ? el.value.trim() : '';
}

export function focusElement(id: string): void {
  var el = getElement<FormValueElement>(id);
  if (el) el.focus();
}

export function setVisible(id: string, visible: boolean): void {
  var el = getElement(id);
  if (el) {
    el.hidden = !visible;
  }
}

export function setValue(id: string, value: string): void {
  var el = getElement<FormValueElement>(id);
  if (el) {
    el.value = value;
  }
}

export function renderBillingStatus(id: string, status: BillingStatus): void {
  var el = getElement(id);
  if (!el) return;
  if (!status.visible) {
    el.textContent = '';
    el.className = 'billing-status';
    return;
  }
  el.textContent = status.message;
  el.className = 'billing-status visible' + (status.error ? ' error' : ' success');
}

export function renderButton(id: string | undefined, disabled: boolean, label: string | undefined): void {
  if (!id || !label) return;
  var button = getElement<HTMLButtonElement>(id);
  if (!button) return;
  button.disabled = disabled;
  button.textContent = label;
}

export function renderOpenBillingPanels(openBillingPanelID: string): void {
  var panels = ['manage-billing-panel', 'retrieve-billing-panel', 'refund-billing-panel', 'data-billing-panel'];
  var emptyPanel = getElement('billing-panel-empty');
  if (emptyPanel) {
    emptyPanel.classList.toggle('visible', !openBillingPanelID);
  }
  for (var i = 0; i < panels.length; i++) {
    var panel = getElement(panels[i]);
    if (!panel) continue;
    panel.classList.toggle('visible', panels[i] === openBillingPanelID);
  }
  var billingButtons = document.querySelectorAll<HTMLElement>('[data-account-billing-action="open-billing-panel"]');
  for (var j = 0; j < billingButtons.length; j += 1) {
    var button = billingButtons[j];
    var row = button.closest('.billing-action-row');
    if (!row) continue;
    row.classList.toggle('active', button.getAttribute('data-account-billing-panel') === openBillingPanelID);
  }
}

export function renderRefundPanel(refundState: RefundState, bootstrap: PortalBootstrapData): void {
  var root = getElement('refund-billing-root');
  if (!root) return;
  var refundSupportURL = (bootstrap.public_site_url || '') + '/refund.html?email=' + encodeURIComponent(refundState.emailValue || '');
  root.innerHTML = '' +
    '<h3>Refund requests</h3>' +
    '<p>Process an eligible self-serve refund for a self-hosted purchase. This revokes the associated license immediately.</p>' +
    '<div class="warning"><strong>Warning:</strong> completing a refund immediately revokes the affected license. This should only be used when the refund window and commercial contract allow it.</div>' +
    '<div class="form-group">' +
      '<label for="refund-inline-email">Email address</label>' +
      '<input type="email" id="refund-inline-email" value="' + escapeAttribute(refundState.emailValue || '') + '" autocomplete="email" data-account-billing-input="refund-email">' +
    '</div>' +
    '<div class="form-group">' +
      '<label for="refund-inline-token">License key</label>' +
      '<input type="text" id="refund-inline-token" value="' + escapeAttribute(refundState.tokenValue || '') + '" placeholder="pulse_xxxxx" data-account-billing-input="refund-token">' +
    '</div>' +
    '<div class="form-actions">' +
      '<button class="btn-danger" type="button" id="refund-inline-submit" data-account-billing-action="refund-inline-submit">Process Refund</button>' +
    '</div>' +
    '<div class="helper-text">If this purchase is not eligible for self-serve refund, use the public support path instead: <a href="' + escapeAttribute(refundSupportURL) + '">open refund support page</a>.</div>' +
    '<div class="billing-status" id="refund-inline-status"></div>';
}

export function renderManagePanel(flowState: VerificationFlowState): void {
  var root = getElement('manage-billing-root');
  if (!root) return;
  root.innerHTML = '' +
    '<h3>Manage subscriptions</h3>' +
    '<p>Request a verification code for the commercial email, then open the Stripe customer portal for billing changes, invoices, and subscription actions.</p>' +
    '<div id="manage-inline-step1">' +
      '<div class="form-group">' +
        '<label for="manage-inline-email">Email address</label>' +
        '<input type="email" id="manage-inline-email" value="' + escapeAttribute(flowState.emailValue || '') + '" autocomplete="email" data-account-billing-input="manage-email">' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-primary" type="button" id="manage-inline-request" data-account-billing-action="manage-inline-request">Send Verification Code</button>' +
      '</div>' +
    '</div>' +
    '<div id="manage-inline-step2"' + (flowState.step2Visible ? '' : ' hidden') + '>' +
      '<div class="form-group">' +
        '<label for="manage-inline-code">Verification code</label>' +
        '<input type="text" id="manage-inline-code" value="' + escapeAttribute(flowState.codeValue || '') + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-billing-input="manage-code">' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-primary" type="button" id="manage-inline-confirm" data-account-billing-action="manage-inline-confirm">Open Customer Portal</button>' +
      '</div>' +
      '<div class="helper-text">Need a new code? <a href="#" id="manage-inline-resend" data-account-billing-action="manage-inline-resend">Send again</a></div>' +
    '</div>' +
    '<div class="billing-status" id="manage-inline-status"></div>';
}

export function renderRetrievePanel(flowState: VerificationFlowState): void {
  var root = getElement('retrieve-billing-root');
  if (!root) return;
  var result = flowState.result as {
    invoice_url?: string;
    token?: string;
    tier?: string;
    issued_at?: string;
    expires_at?: string | null;
    email?: string;
  } | null;
  var invoiceURL = result && result.invoice_url ? result.invoice_url : '#';
  root.innerHTML = '' +
    '<h3>Retrieve licenses</h3>' +
    '<p>Request a verification code for the commercial email, then reveal the current active self-hosted license without leaving Pulse Account.</p>' +
    '<div id="retrieve-inline-step1">' +
      '<div class="form-group">' +
        '<label for="retrieve-inline-email">Email address</label>' +
        '<input type="email" id="retrieve-inline-email" value="' + escapeAttribute(flowState.emailValue || '') + '" autocomplete="email" data-account-billing-input="retrieve-email">' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-primary" type="button" id="retrieve-inline-request" data-account-billing-action="retrieve-inline-request">Send Verification Code</button>' +
      '</div>' +
    '</div>' +
    '<div id="retrieve-inline-step2"' + (flowState.step2Visible ? '' : ' hidden') + '>' +
      '<div class="form-group">' +
        '<label for="retrieve-inline-code">Verification code</label>' +
        '<input type="text" id="retrieve-inline-code" value="' + escapeAttribute(flowState.codeValue || '') + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-billing-input="retrieve-code">' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-primary" type="button" id="retrieve-inline-confirm" data-account-billing-action="retrieve-inline-confirm">Show License</button>' +
        '<button class="btn-secondary" type="button" id="retrieve-inline-copy" data-account-billing-action="retrieve-inline-copy"' + (result ? '' : ' hidden') + '>Copy License Key</button>' +
        '<a class="btn-secondary" id="retrieve-inline-invoice" href="' + escapeAttribute(invoiceURL) + '" target="_blank" rel="noopener"' + (result && result.invoice_url ? '' : ' hidden') + '>View Invoice</a>' +
      '</div>' +
      '<div class="helper-text">Use the latest active self-hosted license for this commercial email.</div>' +
    '</div>' +
    '<div class="billing-status" id="retrieve-inline-status"></div>' +
    '<div id="retrieve-inline-result" class="billing-result"' + (result ? '' : ' hidden') + '>' +
      '<label for="retrieve-inline-token">License key</label>' +
      '<textarea id="retrieve-inline-token" readonly>' + escapeText(result ? result.token : '') + '</textarea>' +
      '<div class="result-grid">' +
        '<div><div class="result-meta-label">Plan</div><div class="result-meta-value" id="retrieve-inline-tier">' + escapeText(result ? result.tier : '') + '</div></div>' +
        '<div><div class="result-meta-label">Issued</div><div class="result-meta-value" id="retrieve-inline-issued">' + escapeText(result ? new Date(result.issued_at).toLocaleString() : '') + '</div></div>' +
        '<div><div class="result-meta-label">Expires</div><div class="result-meta-value" id="retrieve-inline-expires">' + escapeText(result ? (result.expires_at ? new Date(result.expires_at).toLocaleString() : 'Does not expire') : '') + '</div></div>' +
        '<div><div class="result-meta-label">Purchase Email</div><div class="result-meta-value" id="retrieve-inline-email-value">' + escapeText(result ? result.email : '') + '</div></div>' +
      '</div>' +
    '</div>';
}

export function renderExportPanel(flowState: VerificationFlowState): void {
  var root = getElement('data-export-root');
  if (!root) return;
  root.innerHTML = '' +
    '<h4>Export My Data</h4>' +
    '<div id="data-export-step1">' +
      '<div class="form-group">' +
        '<label for="data-export-email">Email address</label>' +
        '<input type="email" id="data-export-email" value="' + escapeAttribute(flowState.emailValue || '') + '" autocomplete="email" data-account-billing-input="data-export-email">' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-primary" type="button" id="data-export-request" data-account-billing-action="data-export-request">Send Verification Code</button>' +
      '</div>' +
    '</div>' +
    '<div id="data-export-step2"' + (flowState.step2Visible ? '' : ' hidden') + '>' +
      '<div class="form-group">' +
        '<label for="data-export-code">Verification code</label>' +
        '<input type="text" id="data-export-code" value="' + escapeAttribute(flowState.codeValue || '') + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-billing-input="data-export-code">' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-primary" type="button" id="data-export-confirm" data-account-billing-action="data-export-confirm">Export My Data</button>' +
      '</div>' +
      '<div class="helper-text">Need a new code? <a href="#" id="data-export-resend" data-account-billing-action="data-export-resend">Send again</a></div>' +
    '</div>' +
    '<div class="billing-status" id="data-export-status"></div>' +
    '<div id="data-export-result" class="billing-result"' + (flowState.result ? '' : ' hidden') + '>' +
      '<label for="data-export-payload">Export payload</label>' +
      '<textarea id="data-export-payload" readonly>' + escapeText(flowState.result ? JSON.stringify(flowState.result, null, 2) : '') + '</textarea>' +
    '</div>';
}

export function renderExportResult(result: unknown): void {
  setVisible('data-export-result', !!result);
  setValue('data-export-payload', result ? JSON.stringify(result, null, 2) : '');
}

export function renderDeletePanel(flowState: VerificationFlowState): void {
  var root = getElement('data-delete-root');
  if (!root) return;
  root.innerHTML = '' +
    '<h4>Delete My Data</h4>' +
    '<div class="warning"><strong>Warning:</strong> deleting commercial data also revokes license records and cannot be undone.</div>' +
    '<div id="data-delete-step1">' +
      '<div class="form-group">' +
        '<label for="data-delete-email">Email address</label>' +
        '<input type="email" id="data-delete-email" value="' + escapeAttribute(flowState.emailValue || '') + '" autocomplete="email" data-account-billing-input="data-delete-email">' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-danger" type="button" id="data-delete-request" data-account-billing-action="data-delete-request">Send Verification Code</button>' +
      '</div>' +
    '</div>' +
    '<div id="data-delete-step2"' + (flowState.step2Visible ? '' : ' hidden') + '>' +
      '<div class="form-group">' +
        '<label for="data-delete-code">Verification code</label>' +
        '<input type="text" id="data-delete-code" value="' + escapeAttribute(flowState.codeValue || '') + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-billing-input="data-delete-code">' +
      '</div>' +
      '<div class="checkbox-row">' +
        '<input type="checkbox" id="data-delete-confirm-check"' + (flowState.checkboxChecked ? ' checked' : '') + '>' +
        '<span>I understand this permanently deletes my commercial data and revokes associated licenses.</span>' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-danger" type="button" id="data-delete-confirm" data-account-billing-action="data-delete-confirm">Delete My Data</button>' +
      '</div>' +
      '<div class="helper-text">Need a new code? <a href="#" id="data-delete-resend" data-account-billing-action="data-delete-resend">Send again</a></div>' +
    '</div>' +
    '<div class="billing-status" id="data-delete-status"></div>';
}
