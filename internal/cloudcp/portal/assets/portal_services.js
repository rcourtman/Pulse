(function() {
  var runtime = window.PulseAccountPortal || {};
  var serviceState = {
    openPanelID: '',
    flows: {
    manage: newVerificationFlowState(),
    retrieve: newVerificationFlowState(),
      export: newVerificationFlowState(),
      delete: newVerificationFlowState(),
    },
    refund: {
      submitting: false,
      status: emptyStatus(),
    },
  };

  function newVerificationFlowState() {
    return {
      pendingEmail: '',
      requesting: false,
      confirming: false,
      step2Visible: false,
      status: emptyStatus(),
      result: null,
      emailValue: '',
      codeValue: '',
    };
  }

  function emptyStatus() {
    return {
      visible: false,
      message: '',
      error: false,
    };
  }

  function getCommercialAPIBaseURL() {
    return runtime.getCommercialAPIBaseURL ? runtime.getCommercialAPIBaseURL() : '';
  }

  function getElement(id) {
    return document.getElementById(id);
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
    var el = getElement(id);
    return el ? el.value.trim() : '';
  }

  function focusElement(id) {
    var el = getElement(id);
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
    var el = getElement(id);
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

  function setFlowStatus(flowID, message, isError) {
    serviceState.flows[flowID].status = {
      visible: true,
      message: message,
      error: !!isError,
    };
  }

  function clearFlowStatus(flowID) {
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
    var button = getElement(id);
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
    renderButton('refund-inline-submit', serviceState.refund.submitting, serviceState.refund.submitting ? 'Processing...' : 'Process Refund');
    renderStatus('refund-inline-status', serviceState.refund.status);
  }

  function resetVerificationFlow(flowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var previous = serviceState.flows[flowID];
    serviceState.flows[flowID] = newVerificationFlowState();
    serviceState.flows[flowID].emailValue = previous.emailValue;
    if (flow.codeInputID) {
      setValue(flow.codeInputID, '');
    }
  }

  var verificationFlows = {
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
      onRequestStart: function() {},
      onConfirmSuccess: function(data) {
        window.location.href = data.url;
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
        var result = flowState.result;
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
      onRequestStart: function() {
        serviceState.flows.export.result = null;
      },
      onConfirmSuccess: function(data) {
        serviceState.flows.export.result = data;
        setFlowStatus('export', 'Data export retrieved successfully.', false);
        resetVerificationFlow('export');
        serviceState.flows.export.result = data;
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
      beforeConfirm: function() {
        if (!getElement('data-delete-confirm-check').checked) {
          setFlowStatus('delete', 'You must confirm that you understand this action is permanent.', true);
          renderFlow('delete');
          return false;
        }
        return true;
      },
      onConfirmSuccess: function(data) {
        getElement('data-delete-confirm-check').checked = false;
        resetVerificationFlow('delete');
        setFlowStatus('delete', data.deleted_count > 0 && data.stripe_reminder ? data.message + ' ' + data.stripe_reminder : data.message, false);
      }
    }
  };

  async function requestVerificationCode(flowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = flow.readEmailValue ? flow.readEmailValue() : readValue(flow.emailInputID);
    if (!email) {
      focusElement(flow.emailInputID);
      return;
    }
    flow.onRequestStart();
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

  async function resendVerificationCode(flowID, event) {
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

  async function confirmVerificationCode(flowID) {
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
    var token = serviceState.flows.retrieve.result ? serviceState.flows.retrieve.result.token : '';
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
    var email = readValue('refund-inline-email');
    var token = readValue('refund-inline-token');
    if (!email || !token) return;
    if (!confirm('Are you sure? This will immediately revoke the license and request the refund.')) return;
    serviceState.refund.submitting = true;
    serviceState.refund.status = emptyStatus();
    renderRefund();
    try {
      var res = await serviceFetch('/v1/self-refund', { email: email, token: token });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Refund failed');
      setValue('refund-inline-token', '');
      setRefundStatus('Success! Your refund has been processed. Stripe will follow up by email.', false);
    } catch (err) {
      setRefundStatus(err.message, true);
    } finally {
      serviceState.refund.submitting = false;
      renderRefund();
    }
  }

  renderOpenPanels();
  serviceState.flows.retrieve.emailValue = (runtime.getBootstrap ? ((runtime.getBootstrap() || {}).email || '') : '');
  renderAllFlows();

  document.addEventListener('click', function(event) {
    var target = event.target.closest('[data-account-service-action]');
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
    var target = event.target;
    if (!target) return;
    var inputKind = target.getAttribute('data-account-service-input') || '';
    switch (inputKind) {
      case 'retrieve-email':
        serviceState.flows.retrieve.emailValue = target.value;
        return;
      case 'retrieve-code':
        serviceState.flows.retrieve.codeValue = target.value;
        return;
      default:
        return;
    }
  });
})();
