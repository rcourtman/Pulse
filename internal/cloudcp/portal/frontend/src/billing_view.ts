import type {
  PortalBillingState,
  PortalBootstrapData,
  RefundState,
  BillingStatus,
  VerificationFlowState,
} from './types';

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
  if (!el) return;
  if (typeof el.focus === 'function') {
    try {
      el.focus({ preventScroll: true });
      return;
    } catch (_) {
      el.focus();
    }
  }
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

function renderUpgradePlansHTML(billingState: PortalBillingState): string {
  var pricing = billingState.upgradePricing.data;
  if (!pricing || !Array.isArray(pricing.plans)) {
    return '';
  }

  var plans = pricing.plans.filter(function(plan) {
    return Array.isArray(plan.buttons) && plan.buttons.some(function(button) {
      return button.kind === 'checkout' && button.planKey && button.billingCycle;
    });
  });
  if (plans.length === 0) {
    return '';
  }

  var portalHandoffID = String(billingState.upgradePortalHandoffID || '').trim();
  var checkoutDisabled = billingState.upgradeCheckout.pending ||
    !portalHandoffID ||
    billingState.upgradePortalHandoff.status === 'loading' ||
    billingState.upgradePortalHandoff.status !== 'ready';

  return '<div class="billing-upgrade-plan-grid">' + plans.map(function(plan) {
    var buttons = Array.isArray(plan.buttons) ? plan.buttons : [];
    var checkoutButtons = buttons.filter(function(button) {
      return button.kind === 'checkout' && button.planKey && button.billingCycle;
    });
    return (
      '<article class="billing-upgrade-plan-card' + (plan.highlight ? ' highlight' : '') + '">' +
        (plan.badge ? '<div class="billing-upgrade-plan-badge">' + escapeText(plan.badge) + '</div>' : '') +
        '<div class="billing-upgrade-plan-header">' +
          '<div class="billing-upgrade-plan-kicker">' + escapeText(plan.tierKicker) + '</div>' +
          '<h4>' + escapeText(plan.title) + '</h4>' +
          '<div class="billing-upgrade-plan-price">' + escapeText(plan.price) + '</div>' +
          '<div class="billing-upgrade-plan-period">' + escapeText(plan.period) + '</div>' +
        '</div>' +
        '<p class="billing-upgrade-plan-blurb">' + escapeText(plan.blurb) + '</p>' +
        '<ul class="billing-upgrade-plan-features">' + plan.features.map(function(feature) {
          return (
            '<li class="billing-upgrade-plan-feature tone-' + escapeAttribute(feature.tone) + '">' +
              '<span class="billing-upgrade-plan-feature-copy">' + String(feature.html || '') + '</span>' +
            '</li>'
          );
        }).join('') + '</ul>' +
        (plan.note ? '<div class="helper-text">' + escapeText(plan.note) + '</div>' : '') +
        '<div class="form-actions">' + checkoutButtons.map(function(button) {
          return (
            '<button type="button" class="' + escapeAttribute(button.className || 'btn-primary') + '"' +
              ' data-account-billing-action="upgrade-start-checkout"' +
              ' data-upgrade-plan-key="' + escapeAttribute(button.planKey || '') + '"' +
              ' data-upgrade-tier="' + escapeAttribute(button.tier || '') + '"' +
              ' data-upgrade-billing-cycle="' + escapeAttribute(button.billingCycle || '') + '"' +
              (checkoutDisabled ? ' disabled' : '') +
              '>' +
              escapeText(button.label) +
            '</button>'
          );
        }).join('') + '</div>' +
      '</article>'
    );
  }).join('') + '</div>';
}

export function renderUpgradePanel(billingState: PortalBillingState, _bootstrap: PortalBootstrapData): void {
  var root = getElement('upgrade-billing-root');
  if (!root) return;

  var featureKey = String(billingState.upgradeFeatureKey || '').trim();
  var portalHandoffID = String(billingState.upgradePortalHandoffID || '').trim();
  var pricingState = billingState.upgradePricing;
  var handoffState = billingState.upgradePortalHandoff;
  var explainer = pricingState.data && pricingState.data.explainer ? pricingState.data.explainer : '';
  var summaryItems = [] as string[];

  if (billingState.upgradeCheckout.pending) {
    summaryItems.push('<div class="billing-status visible">Redirecting to secure checkout...</div>');
  }
  if (billingState.upgradeCheckout.error) {
    summaryItems.push('<div class="billing-status visible error">' + escapeText(billingState.upgradeCheckout.error) + '</div>');
  }
  if (!portalHandoffID) {
    summaryItems.push(
      '<div class="billing-status visible error">Open this upgrade from Pulse Pro billing so Pulse Account can verify the secure upgrade handoff before checkout.</div>',
    );
  } else if (handoffState.status === 'loading') {
    summaryItems.push('<div class="billing-status visible">Verifying the secure Pulse Pro upgrade handoff...</div>');
  } else if (handoffState.status === 'error') {
    summaryItems.push('<div class="billing-status visible error">' + escapeText(handoffState.error || 'Failed to verify the secure Pulse Pro upgrade handoff.') + '</div>');
  } else if (handoffState.status === 'ready') {
    summaryItems.push('<div class="billing-status visible success">Pulse Account will return completed checkout directly to Pulse Pro billing.</div>');
  }
  if (pricingState.status === 'loading' && !pricingState.data) {
    summaryItems.push('<p>Loading self-hosted plan options...</p>');
  }
  if (pricingState.status === 'error') {
    summaryItems.push(
      '<div class="billing-status visible error">' + escapeText(pricingState.error || 'Failed to load self-hosted plans.') + '</div>' +
      '<div class="form-actions"><button type="button" class="btn-secondary" data-account-billing-action="upgrade-reload-pricing">Retry plan load</button></div>',
    );
  }
  if (explainer) {
    summaryItems.push('<div class="helper-text">' + explainer + '</div>');
  }

  root.innerHTML =
    '<div class="billing-upgrade-root">' +
      summaryItems.join('') +
      renderUpgradePlansHTML(billingState) +
      (pricingState.status === 'ready' && pricingState.data && pricingState.data.description
        ? '<div class="helper-text">' + escapeText(pricingState.data.description) + '</div>'
        : '') +
      '<div class="helper-text">' +
        (featureKey === 'max_monitored_systems'
          ? 'Pulse Account compares self-hosted tiers and sends completed monitored-system upgrades straight back to Pulse Pro billing.'
          : 'Pulse Account compares self-hosted tiers and sends completed checkout straight back to Pulse Pro billing.') +
      '</div>' +
    '</div>';
}

export function renderButton(id: string | undefined, disabled: boolean, label: string | undefined): void {
  if (!id || !label) return;
  var button = getElement<HTMLButtonElement>(id);
  if (!button) return;
  button.disabled = disabled;
  button.textContent = label;
}

export function renderOpenBillingPanels(openBillingPanelID: string): void {
  var shell = document.querySelector('.billing-shell') as HTMLElement | null;
  var detailShell = getElement('billing-detail-shell');
  var panels = [
    'upgrade-billing-panel',
    'manage-billing-panel',
    'retrieve-billing-panel',
    'refund-billing-panel',
    'data-billing-panel',
  ];
  var hasOpenPanel = !!openBillingPanelID;
  if (shell) {
    shell.classList.toggle('billing-shell-job-open', hasOpenPanel);
    shell.classList.toggle('billing-shell-idle', !hasOpenPanel);
  }
  if (detailShell) {
    detailShell.hidden = !hasOpenPanel;
  }
  for (var i = 0; i < panels.length; i++) {
    var panel = getElement(panels[i]);
    if (!panel) continue;
    var isActive = panels[i] === openBillingPanelID;
    panel.hidden = !isActive;
    panel.classList.toggle('visible', isActive);
  }
  var billingButtons = document.querySelectorAll<HTMLElement>('[data-account-billing-action="open-billing-panel"]');
  for (var j = 0; j < billingButtons.length; j += 1) {
    var button = billingButtons[j];
    var row = button.closest('.billing-action-row');
    if (!row) continue;
    row.classList.toggle('active', button.getAttribute('data-account-billing-panel') === openBillingPanelID);
  }
  if (hasOpenPanel && detailShell) {
    var reveal = function(): void {
      if (typeof detailShell.scrollIntoView !== 'function') return;
      var rect = detailShell.getBoundingClientRect();
      if (rect.top >= 0 && rect.top <= window.innerHeight - 72 && rect.bottom > 0) {
        return;
      }
      detailShell.scrollIntoView({ block: 'start', inline: 'nearest' });
    };
    if (typeof window.requestAnimationFrame === 'function') {
      window.requestAnimationFrame(reveal);
      return;
    }
    reveal();
  }
}

export function renderRefundPanel(refundState: RefundState, bootstrap: PortalBootstrapData): void {
  var root = getElement('refund-billing-root');
  if (!root) return;
  var refundSupportURL = (bootstrap.public_site_url || '') + '/refund.html?email=' + encodeURIComponent(refundState.emailValue || '');
  root.innerHTML = '' +
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
