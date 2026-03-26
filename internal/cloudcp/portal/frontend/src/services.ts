import { getBootstrap, getCommercialAPIBaseURL as readCommercialAPIBaseURL, subscribePortalRender } from './runtime';
import type { RefundState, ServiceStatus, VerificationFlowState } from './types';

type FlowID = 'manage' | 'retrieve' | 'export' | 'delete';

interface VerificationFlowDefinition {
  requestPath: string;
  confirmPath: string;
  panelID: string;
  emailInputID: string;
  codeInputID?: string;
  requestButtonID: string;
  confirmButtonID?: string;
  step2ID?: string;
  statusID: string;
  requestLabel: string;
  requestPendingLabel: string;
  confirmLabel?: string;
  confirmPendingLabel?: string;
  requestSuccessMessage: string;
  resendSuccessMessage: string;
  requestErrorMessage: string;
  confirmErrorMessage: string;
  readEmailValue?: () => string;
  readCodeValue?: () => string;
  onRequestStart?: () => void;
  beforeConfirm?: () => boolean;
  onConfirmSuccess: (data: any, email?: string) => void;
  renderPanel: (flowState: VerificationFlowState) => void;
  renderResult?: (result: unknown) => void;
}

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

  var serviceState = {
    openPanelID: '',
    flows: {
      manage: newVerificationFlowState(),
      retrieve: newVerificationFlowState(),
      export: newVerificationFlowState(),
      delete: newVerificationFlowState(),
    },
    refund: {
      emailValue: '',
      tokenValue: '',
      submitting: false,
      status: emptyStatus(),
    },
  } as { openPanelID: string; flows: Record<FlowID, VerificationFlowState>; refund: RefundState };

  function newVerificationFlowState(): VerificationFlowState {
    return {
      pendingEmail: '',
      requesting: false,
      confirming: false,
      step2Visible: false,
      status: emptyStatus(),
      result: null,
      emailValue: '',
      codeValue: '',
      checkboxChecked: false,
    };
  }

  function emptyStatus(): ServiceStatus {
    return {
      visible: false,
      message: '',
      error: false,
    };
  }

  function getCommercialAPIBaseURL() {
    return readCommercialAPIBaseURL();
  }

  function getElement<T extends HTMLElement = HTMLElement>(id): T | null {
    return document.getElementById(id) as T | null;
  }

  function asHTMLElement(target: EventTarget | null): HTMLElement | null {
    return target instanceof HTMLElement ? target : null;
  }

  function escapeText(value) {
    return String(value || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  function escapeAttribute(value) {
    return escapeText(value)
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function readValue(id) {
    var el = getElement<FormValueElement>(id);
    return el ? el.value.trim() : '';
  }

  function focusElement(id) {
    var el = getElement<FormValueElement>(id);
    if (el) el.focus();
  }

  function setVisible(id, visible) {
    var el = getElement(id);
    if (el) {
      el.style.display = visible ? 'block' : 'none';
    }
  }

  function setText(id, value) {
    var el = getElement(id);
    if (el) {
      el.textContent = value;
    }
  }

  function setValue(id, value) {
    var el = getElement<FormValueElement>(id);
    if (el) {
      el.value = value;
    }
  }

  function serviceFetch(path, body) {
    return fetch(getCommercialAPIBaseURL() + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  function setFlowStatus(flowID: FlowID, message, isError) {
    serviceState.flows[flowID].status = {
      visible: true,
      message: message,
      error: !!isError,
    };
  }

  function clearFlowStatus(flowID: FlowID) {
    serviceState.flows[flowID].status = emptyStatus();
  }

  function setRefundStatus(message, isError) {
    serviceState.refund.status = {
      visible: true,
      message: message,
      error: !!isError,
    };
  }

  function renderStatus(id, status) {
    var el = getElement(id);
    if (!el) return;
    if (!status.visible) {
      el.textContent = '';
      el.className = 'service-status';
      return;
    }
    el.textContent = status.message;
    el.className = 'service-status visible' + (status.error ? ' error' : ' success');
  }

  function renderButton(id, disabled, label) {
    var button = getElement<HTMLButtonElement>(id);
    if (!button) return;
    button.disabled = disabled;
    button.textContent = label;
  }

  function toggleServicePanel(panelID) {
    serviceState.openPanelID = serviceState.openPanelID === panelID ? '' : panelID;
    renderOpenPanels();
  }

  function renderOpenPanels() {
    var panels = ['manage-service-panel', 'retrieve-service-panel', 'refund-service-panel', 'data-service-panel'];
    for (var i = 0; i < panels.length; i++) {
      var panel = getElement(panels[i]);
      if (!panel) continue;
      panel.classList.toggle('visible', panels[i] === serviceState.openPanelID);
    }
  }

  function renderFlow(flowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var flowState = serviceState.flows[flowID];
    if (flow.renderPanel) {
      flow.renderPanel(flowState);
    }
    renderButton(flow.requestButtonID, flowState.requesting, flowState.requesting ? flow.requestPendingLabel : flow.requestLabel);
    renderButton(flow.confirmButtonID, flowState.confirming, flowState.confirming ? flow.confirmPendingLabel : flow.confirmLabel);
    renderStatus(flow.statusID, flowState.status);
    if (flow.step2ID) {
      setVisible(flow.step2ID, flowState.step2Visible);
    }
    if (flow.renderResult) {
      flow.renderResult(flowState.result);
    }
  }

  function renderAllFlows() {
    renderFlow('manage');
    renderFlow('retrieve');
    renderFlow('export');
    renderFlow('delete');
    renderRefund();
  }

  function renderRefund() {
    renderRefundPanel();
    renderButton('refund-inline-submit', serviceState.refund.submitting, serviceState.refund.submitting ? 'Processing...' : 'Process Refund');
    renderStatus('refund-inline-status', serviceState.refund.status);
  }

  function renderRefundPanel() {
    var root = getElement('refund-service-root');
    if (!root) return;
    var bootstrap = getBootstrap();
    var refundSupportURL = (bootstrap.public_site_url || '') + '/refund.html?email=' + encodeURIComponent(serviceState.refund.emailValue || '');
    root.innerHTML = '' +
      '<h3>Refund requests</h3>' +
      '<p>Process an eligible self-serve refund for a self-hosted purchase. This revokes the associated license immediately.</p>' +
      '<div class="warning"><strong>Warning:</strong> completing a refund immediately revokes the affected license. This should only be used when the refund window and commercial contract allow it.</div>' +
      '<div class="form-group">' +
        '<label for="refund-inline-email">Email address</label>' +
        '<input type="email" id="refund-inline-email" value="' + escapeAttribute(serviceState.refund.emailValue || '') + '" autocomplete="email" data-account-service-input="refund-email">' +
      '</div>' +
      '<div class="form-group">' +
        '<label for="refund-inline-token">License key</label>' +
        '<input type="text" id="refund-inline-token" value="' + escapeAttribute(serviceState.refund.tokenValue || '') + '" placeholder="pulse_xxxxx" data-account-service-input="refund-token">' +
      '</div>' +
      '<div class="form-actions">' +
        '<button class="btn-danger" type="button" id="refund-inline-submit" data-account-service-action="refund-inline-submit">Process Refund</button>' +
      '</div>' +
      '<div class="helper-text">If this purchase is not eligible for self-serve refund, use the public support path instead: <a href="' + escapeAttribute(refundSupportURL) + '">open refund support page</a>.</div>' +
      '<div class="service-status" id="refund-inline-status"></div>';
  }

  function resetVerificationFlow(flowID: FlowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var previous = serviceState.flows[flowID];
    serviceState.flows[flowID] = newVerificationFlowState();
    serviceState.flows[flowID].emailValue = previous.emailValue;
    if (flow.codeInputID) {
      setValue(flow.codeInputID, '');
    }
  }

  var verificationFlows: Record<FlowID, VerificationFlowDefinition> = {
    manage: {
      requestPath: '/v1/manage/request',
      confirmPath: '/v1/manage',
      panelID: 'manage-service-panel',
      emailInputID: 'manage-inline-email',
      codeInputID: 'manage-inline-code',
      requestButtonID: 'manage-inline-request',
      confirmButtonID: 'manage-inline-confirm',
      step2ID: 'manage-inline-step2',
      statusID: 'manage-inline-status',
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Open Customer Portal',
      confirmPendingLabel: 'Redirecting...',
      requestSuccessMessage: 'Verification code sent. Check your email.',
      resendSuccessMessage: 'New verification code sent.',
      requestErrorMessage: 'Failed to send verification code',
      confirmErrorMessage: 'Failed to open customer portal',
      readEmailValue: function() {
        return serviceState.flows.manage.emailValue;
      },
      readCodeValue: function() {
        return serviceState.flows.manage.codeValue;
      },
      onRequestStart: function() {},
      onConfirmSuccess: function(data) {
        window.location.href = data.url;
      },
      renderPanel: function(flowState) {
        var root = getElement('manage-service-root');
        if (!root) return;
        root.innerHTML = '' +
          '<h3>Manage subscriptions</h3>' +
          '<p>Request a verification code for the commercial email, then open the Stripe customer portal for billing changes, invoices, and subscription actions.</p>' +
          '<div id="manage-inline-step1">' +
            '<div class="form-group">' +
              '<label for="manage-inline-email">Email address</label>' +
              '<input type="email" id="manage-inline-email" value="' + escapeAttribute(flowState.emailValue || '') + '" autocomplete="email" data-account-service-input="manage-email">' +
            '</div>' +
            '<div class="form-actions">' +
              '<button class="btn-primary" type="button" id="manage-inline-request" data-account-service-action="manage-inline-request">Send Verification Code</button>' +
            '</div>' +
          '</div>' +
          '<div id="manage-inline-step2" style="display:' + (flowState.step2Visible ? 'block' : 'none') + '">' +
            '<div class="form-group">' +
              '<label for="manage-inline-code">Verification code</label>' +
              '<input type="text" id="manage-inline-code" value="' + escapeAttribute(flowState.codeValue || '') + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="manage-code">' +
            '</div>' +
            '<div class="form-actions">' +
              '<button class="btn-primary" type="button" id="manage-inline-confirm" data-account-service-action="manage-inline-confirm">Open Customer Portal</button>' +
            '</div>' +
            '<div class="helper-text">Need a new code? <a href="#" id="manage-inline-resend" data-account-service-action="manage-inline-resend">Send again</a></div>' +
          '</div>' +
          '<div class="service-status" id="manage-inline-status"></div>';
      }
    },
    retrieve: {
      requestPath: '/v1/retrieve-license/request',
      confirmPath: '/v1/retrieve-license',
      panelID: 'retrieve-service-panel',
      emailInputID: 'retrieve-inline-email',
      codeInputID: 'retrieve-inline-code',
      requestButtonID: 'retrieve-inline-request',
      confirmButtonID: 'retrieve-inline-confirm',
      step2ID: 'retrieve-inline-step2',
      statusID: 'retrieve-inline-status',
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Show License',
      confirmPendingLabel: 'Loading...',
      requestSuccessMessage: 'Verification code sent. Check your email.',
      resendSuccessMessage: 'New verification code sent.',
      requestErrorMessage: 'Failed to send verification code',
      confirmErrorMessage: 'Failed to retrieve license',
      readEmailValue: function() {
        return serviceState.flows.retrieve.emailValue;
      },
      readCodeValue: function() {
        return serviceState.flows.retrieve.codeValue;
      },
      onRequestStart: function() {
        serviceState.flows.retrieve.result = null;
      },
      onConfirmSuccess: function(data) {
        serviceState.flows.retrieve.result = data.license;
        serviceState.flows.retrieve.codeValue = '';
        setFlowStatus('retrieve', 'License retrieved successfully.', false);
      },
      renderPanel: function(flowState) {
        var root = getElement('retrieve-service-root');
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
        var invoiceDisplay = result && result.invoice_url ? 'inline-block' : 'none';
        var copyDisplay = result ? 'inline-block' : 'none';
        var resultDisplay = result ? 'block' : 'none';
        root.innerHTML = '' +
          '<h3>Retrieve licenses</h3>' +
          '<p>Request a verification code for the commercial email, then reveal the current active self-hosted license without leaving Pulse Account.</p>' +
          '<div id="retrieve-inline-step1">' +
            '<div class="form-group">' +
              '<label for="retrieve-inline-email">Email address</label>' +
              '<input type="email" id="retrieve-inline-email" value="' + escapeAttribute(flowState.emailValue || '') + '" autocomplete="email" data-account-service-input="retrieve-email">' +
            '</div>' +
            '<div class="form-actions">' +
              '<button class="btn-primary" type="button" id="retrieve-inline-request" data-account-service-action="retrieve-inline-request">Send Verification Code</button>' +
            '</div>' +
          '</div>' +
          '<div id="retrieve-inline-step2" style="display:' + (flowState.step2Visible ? 'block' : 'none') + '">' +
            '<div class="form-group">' +
              '<label for="retrieve-inline-code">Verification code</label>' +
              '<input type="text" id="retrieve-inline-code" value="' + escapeAttribute(flowState.codeValue || '') + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="retrieve-code">' +
            '</div>' +
            '<div class="form-actions">' +
              '<button class="btn-primary" type="button" id="retrieve-inline-confirm" data-account-service-action="retrieve-inline-confirm">Show License</button>' +
              '<button class="btn-secondary" type="button" id="retrieve-inline-copy" data-account-service-action="retrieve-inline-copy" style="display:' + copyDisplay + '">Copy License Key</button>' +
              '<a class="btn-secondary" id="retrieve-inline-invoice" href="' + escapeAttribute(invoiceURL) + '" target="_blank" rel="noopener" style="display:' + invoiceDisplay + '">View Invoice</a>' +
            '</div>' +
            '<div class="helper-text">Use the latest active self-hosted license for this commercial email.</div>' +
          '</div>' +
          '<div class="service-status" id="retrieve-inline-status"></div>' +
          '<div id="retrieve-inline-result" style="display:' + resultDisplay + '; margin-top:14px">' +
            '<label for="retrieve-inline-token">License key</label>' +
            '<textarea id="retrieve-inline-token" readonly>' + escapeText(result ? result.token : '') + '</textarea>' +
            '<div class="result-grid">' +
              '<div><div class="result-meta-label">Plan</div><div class="result-meta-value" id="retrieve-inline-tier">' + escapeText(result ? result.tier : '') + '</div></div>' +
              '<div><div class="result-meta-label">Issued</div><div class="result-meta-value" id="retrieve-inline-issued">' + escapeText(result ? new Date(result.issued_at).toLocaleString() : '') + '</div></div>' +
              '<div><div class="result-meta-label">Expires</div><div class="result-meta-value" id="retrieve-inline-expires">' + escapeText(result ? (result.expires_at ? new Date(result.expires_at).toLocaleString() : 'Does not expire') : '') + '</div></div>' +
              '<div><div class="result-meta-label">Purchase Email</div><div class="result-meta-value" id="retrieve-inline-email-value">' + escapeText(result ? result.email : '') + '</div></div>' +
            '</div>' +
          '</div>';
      },
      renderResult: function(result) {
        void result;
      }
    },
    export: {
      requestPath: '/v1/gdpr/request-export',
      confirmPath: '/v1/gdpr/export',
      panelID: 'data-service-panel',
      emailInputID: 'data-export-email',
      codeInputID: 'data-export-code',
      requestButtonID: 'data-export-request',
      confirmButtonID: 'data-export-confirm',
      step2ID: 'data-export-step2',
      statusID: 'data-export-status',
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Export My Data',
      confirmPendingLabel: 'Exporting...',
      requestSuccessMessage: 'Verification code sent. Check your email.',
      resendSuccessMessage: 'New verification code sent.',
      requestErrorMessage: 'Request failed',
      confirmErrorMessage: 'Export failed',
      readEmailValue: function() {
        return serviceState.flows.export.emailValue;
      },
      readCodeValue: function() {
        return serviceState.flows.export.codeValue;
      },
      onRequestStart: function() {
        serviceState.flows.export.result = null;
      },
      onConfirmSuccess: function(data) {
        serviceState.flows.export.result = data;
        serviceState.flows.export.codeValue = '';
        setFlowStatus('export', 'Data export retrieved successfully.', false);
        resetVerificationFlow('export');
        serviceState.flows.export.result = data;
      },
      renderPanel: function(flowState) {
        var root = getElement('data-export-root');
        if (!root) return;
        var resultDisplay = flowState.result ? 'block' : 'none';
        root.innerHTML = '' +
          '<h4>Export My Data</h4>' +
          '<div id="data-export-step1">' +
            '<div class="form-group">' +
              '<label for="data-export-email">Email address</label>' +
              '<input type="email" id="data-export-email" value="' + escapeAttribute(flowState.emailValue || '') + '" autocomplete="email" data-account-service-input="data-export-email">' +
            '</div>' +
            '<div class="form-actions">' +
              '<button class="btn-primary" type="button" id="data-export-request" data-account-service-action="data-export-request">Send Verification Code</button>' +
            '</div>' +
          '</div>' +
          '<div id="data-export-step2" style="display:' + (flowState.step2Visible ? 'block' : 'none') + '">' +
            '<div class="form-group">' +
              '<label for="data-export-code">Verification code</label>' +
              '<input type="text" id="data-export-code" value="' + escapeAttribute(flowState.codeValue || '') + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="data-export-code">' +
            '</div>' +
            '<div class="form-actions">' +
              '<button class="btn-primary" type="button" id="data-export-confirm" data-account-service-action="data-export-confirm">Export My Data</button>' +
            '</div>' +
            '<div class="helper-text">Need a new code? <a href="#" id="data-export-resend" data-account-service-action="data-export-resend">Send again</a></div>' +
          '</div>' +
          '<div class="service-status" id="data-export-status"></div>' +
          '<div id="data-export-result" style="display:' + resultDisplay + '; margin-top:14px">' +
            '<label for="data-export-payload">Export payload</label>' +
            '<textarea id="data-export-payload" readonly>' + escapeText(flowState.result ? JSON.stringify(flowState.result, null, 2) : '') + '</textarea>' +
          '</div>';
      },
      renderResult: function(result) {
        setVisible('data-export-result', !!result);
        setValue('data-export-payload', result ? JSON.stringify(result, null, 2) : '');
      }
    },
    delete: {
      requestPath: '/v1/gdpr/request-delete',
      confirmPath: '/v1/gdpr/confirm-delete',
      panelID: 'data-service-panel',
      emailInputID: 'data-delete-email',
      codeInputID: 'data-delete-code',
      requestButtonID: 'data-delete-request',
      confirmButtonID: 'data-delete-confirm',
      step2ID: 'data-delete-step2',
      statusID: 'data-delete-status',
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Delete My Data',
      confirmPendingLabel: 'Deleting...',
      requestSuccessMessage: 'Verification code sent. Check your email.',
      resendSuccessMessage: 'New verification code sent.',
      requestErrorMessage: 'Request failed',
      confirmErrorMessage: 'Deletion failed',
      readEmailValue: function() {
        return serviceState.flows.delete.emailValue;
      },
      readCodeValue: function() {
        return serviceState.flows.delete.codeValue;
      },
      beforeConfirm: function() {
        if (!getElement<HTMLInputElement>('data-delete-confirm-check')?.checked) {
          setFlowStatus('delete', 'You must confirm that you understand this action is permanent.', true);
          renderFlow('delete');
          return false;
        }
        return true;
      },
      onConfirmSuccess: function(data) {
        var checkbox = getElement<HTMLInputElement>('data-delete-confirm-check');
        if (checkbox) {
          checkbox.checked = false;
        }
        resetVerificationFlow('delete');
        setFlowStatus('delete', data.deleted_count > 0 && data.stripe_reminder ? data.message + ' ' + data.stripe_reminder : data.message, false);
      },
      renderPanel: function(flowState) {
        var root = getElement('data-delete-root');
        if (!root) return;
        root.innerHTML = '' +
          '<h4>Delete My Data</h4>' +
          '<div class="warning"><strong>Warning:</strong> deleting commercial data also revokes license records and cannot be undone.</div>' +
          '<div id="data-delete-step1">' +
            '<div class="form-group">' +
              '<label for="data-delete-email">Email address</label>' +
              '<input type="email" id="data-delete-email" value="' + escapeAttribute(flowState.emailValue || '') + '" autocomplete="email" data-account-service-input="data-delete-email">' +
            '</div>' +
            '<div class="form-actions">' +
              '<button class="btn-danger" type="button" id="data-delete-request" data-account-service-action="data-delete-request">Send Verification Code</button>' +
            '</div>' +
          '</div>' +
          '<div id="data-delete-step2" style="display:' + (flowState.step2Visible ? 'block' : 'none') + '">' +
            '<div class="form-group">' +
              '<label for="data-delete-code">Verification code</label>' +
              '<input type="text" id="data-delete-code" value="' + escapeAttribute(flowState.codeValue || '') + '" inputmode="numeric" pattern="[0-9]{6}" placeholder="123456" data-account-service-input="data-delete-code">' +
            '</div>' +
            '<div class="checkbox-row">' +
              '<input type="checkbox" id="data-delete-confirm-check"' + (flowState.checkboxChecked ? ' checked' : '') + '>' +
              '<span>I understand this permanently deletes my commercial data and revokes associated licenses.</span>' +
            '</div>' +
            '<div class="form-actions">' +
              '<button class="btn-danger" type="button" id="data-delete-confirm" data-account-service-action="data-delete-confirm">Delete My Data</button>' +
            '</div>' +
            '<div class="helper-text">Need a new code? <a href="#" id="data-delete-resend" data-account-service-action="data-delete-resend">Send again</a></div>' +
          '</div>' +
          '<div class="service-status" id="data-delete-status"></div>';
      }
    }
  };

  async function requestVerificationCode(flowID: FlowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = flow.readEmailValue ? flow.readEmailValue() : readValue(flow.emailInputID);
    if (!email) {
      focusElement(flow.emailInputID);
      return;
    }
    if (flow.onRequestStart) {
      flow.onRequestStart();
    }
    serviceState.flows[flowID].requesting = true;
    clearFlowStatus(flowID);
    renderFlow(flowID);
    try {
      var res = await serviceFetch(flow.requestPath, { email: email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.requestErrorMessage);
      serviceState.flows[flowID].pendingEmail = email;
      serviceState.flows[flowID].step2Visible = !!flow.step2ID;
      setFlowStatus(flowID, flow.requestSuccessMessage, false);
    } catch (err) {
      setFlowStatus(flowID, err.message, true);
    } finally {
      serviceState.flows[flowID].requesting = false;
      renderFlow(flowID);
    }
  }

  async function resendVerificationCode(flowID: FlowID, event) {
    if (event) event.preventDefault();
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = serviceState.flows[flowID].pendingEmail;
    if (!email) return;
    try {
      var res = await serviceFetch(flow.requestPath, { email: email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.requestErrorMessage);
      setFlowStatus(flowID, flow.resendSuccessMessage, false);
    } catch (err) {
      setFlowStatus(flowID, err.message, true);
    }
    renderFlow(flowID);
  }

  async function confirmVerificationCode(flowID: FlowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = serviceState.flows[flowID].pendingEmail;
    var code = flow.readCodeValue ? flow.readCodeValue() : readValue(flow.codeInputID);
    if (!email || !code) return;
    if (flow.beforeConfirm && flow.beforeConfirm() === false) {
      return;
    }
    serviceState.flows[flowID].confirming = true;
    renderFlow(flowID);
    try {
      var res = await serviceFetch(flow.confirmPath, { email: email, code: code });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.confirmErrorMessage);
      flow.onConfirmSuccess(data, email);
    } catch (err) {
      setFlowStatus(flowID, err.message, true);
    } finally {
      serviceState.flows[flowID].confirming = false;
      renderFlow(flowID);
    }
  }

  async function copyRetrievedLicense() {
    var result = serviceState.flows.retrieve.result as { token?: string } | null;
    var token = result && result.token ? result.token : '';
    if (!token) return;
    try {
      await navigator.clipboard.writeText(token);
      setFlowStatus('retrieve', 'License key copied to clipboard.', false);
    } catch (_) {
      setFlowStatus('retrieve', 'Failed to copy automatically. Please copy the key manually.', true);
    }
    renderFlow('retrieve');
  }

  async function submitRefund() {
    var email = serviceState.refund.emailValue;
    var token = serviceState.refund.tokenValue;
    if (!email || !token) return;
    if (!confirm('Are you sure? This will immediately revoke the license and request the refund.')) return;
    serviceState.refund.submitting = true;
    serviceState.refund.status = emptyStatus();
    renderRefund();
    try {
      var res = await serviceFetch('/v1/self-refund', { email: email, token: token });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Refund failed');
      serviceState.refund.tokenValue = '';
      setRefundStatus('Success! Your refund has been processed. Stripe will follow up by email.', false);
    } catch (err) {
      setRefundStatus(err.message, true);
    } finally {
      serviceState.refund.submitting = false;
      renderRefund();
    }
  }

  function syncServiceStateFromBootstrap() {
    var bootstrap = getBootstrap();
    if (!bootstrap.authenticated) {
      return;
    }
    if (!serviceState.flows.manage.emailValue) serviceState.flows.manage.emailValue = bootstrap.email || '';
    if (!serviceState.flows.retrieve.emailValue) serviceState.flows.retrieve.emailValue = bootstrap.email || '';
    if (!serviceState.flows.export.emailValue) serviceState.flows.export.emailValue = bootstrap.email || '';
    if (!serviceState.flows.delete.emailValue) serviceState.flows.delete.emailValue = bootstrap.email || '';
    if (!serviceState.refund.emailValue) serviceState.refund.emailValue = bootstrap.email || '';
  }

  function renderServiceRuntime() {
    syncServiceStateFromBootstrap();
    renderOpenPanels();
    renderAllFlows();
  }

  renderServiceRuntime();
  subscribePortalRender(renderServiceRuntime);

  document.addEventListener('click', function(event) {
    var target = asHTMLElement(event.target)?.closest('[data-account-service-action]');
    if (!target) return;
    var action = target.getAttribute('data-account-service-action') || '';
    var panelID = target.getAttribute('data-account-service-panel') || '';
    var focusID = target.getAttribute('data-account-service-focus') || '';

    switch (action) {
      case 'open-service-panel':
        event.preventDefault();
        toggleServicePanel(panelID);
        focusElement(focusID);
        return;
      case 'manage-inline-request':
        event.preventDefault();
        requestVerificationCode('manage');
        return;
      case 'manage-inline-resend':
        resendVerificationCode('manage', event);
        return;
      case 'manage-inline-confirm':
        event.preventDefault();
        confirmVerificationCode('manage');
        return;
      case 'retrieve-inline-request':
        event.preventDefault();
        requestVerificationCode('retrieve');
        return;
      case 'retrieve-inline-confirm':
        event.preventDefault();
        confirmVerificationCode('retrieve');
        return;
      case 'retrieve-inline-copy':
        event.preventDefault();
        copyRetrievedLicense();
        return;
      case 'refund-inline-submit':
        event.preventDefault();
        submitRefund();
        return;
      case 'data-export-request':
        event.preventDefault();
        requestVerificationCode('export');
        return;
      case 'data-export-resend':
        resendVerificationCode('export', event);
        return;
      case 'data-export-confirm':
        event.preventDefault();
        confirmVerificationCode('export');
        return;
      case 'data-delete-request':
        event.preventDefault();
        requestVerificationCode('delete');
        return;
      case 'data-delete-resend':
        resendVerificationCode('delete', event);
        return;
      case 'data-delete-confirm':
        event.preventDefault();
        confirmVerificationCode('delete');
        return;
      default:
        return;
    }
  });

  document.addEventListener('input', function(event) {
    var target = asHTMLElement(event.target) as FormValueElement | null;
    if (!target) return;
    var inputKind = target.getAttribute('data-account-service-input') || '';
    switch (inputKind) {
      case 'manage-email':
        serviceState.flows.manage.emailValue = target.value;
        return;
      case 'manage-code':
        serviceState.flows.manage.codeValue = target.value;
        return;
      case 'retrieve-email':
        serviceState.flows.retrieve.emailValue = target.value;
        return;
      case 'retrieve-code':
        serviceState.flows.retrieve.codeValue = target.value;
        return;
      case 'refund-email':
        serviceState.refund.emailValue = target.value;
        return;
      case 'refund-token':
        serviceState.refund.tokenValue = target.value;
        return;
      case 'data-export-email':
        serviceState.flows.export.emailValue = target.value;
        return;
      case 'data-export-code':
        serviceState.flows.export.codeValue = target.value;
        return;
      case 'data-delete-email':
        serviceState.flows.delete.emailValue = target.value;
        return;
      case 'data-delete-code':
        serviceState.flows.delete.codeValue = target.value;
        return;
      default:
        return;
    }
  });

  document.addEventListener('change', function(event) {
    var target = asHTMLElement(event.target) as HTMLInputElement | null;
    if (!target || target.id !== 'data-delete-confirm-check') return;
    serviceState.flows.delete.checkboxChecked = !!target.checked;
  });
