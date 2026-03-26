(function() {
  var runtime = window.PulseAccountPortal || {};
  var serviceState = {
    pending: {
      manage: '',
      retrieve: '',
      export: '',
      delete: '',
    },
  };

  function getCommercialAPIBaseURL() {
    return runtime.getCommercialAPIBaseURL ? runtime.getCommercialAPIBaseURL() : '';
  }

  function serviceFetch(path, body) {
    return fetch(getCommercialAPIBaseURL() + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  function getElement(id) {
    return document.getElementById(id);
  }

  function setServiceStatus(id, message, isError) {
    var el = getElement(id);
    if (!el) return;
    el.textContent = message;
    el.className = 'service-status visible' + (isError ? ' error' : ' success');
  }

  function setButtonState(id, disabled, text) {
    var button = getElement(id);
    if (!button) return;
    button.disabled = disabled;
    button.textContent = text;
  }

  function focusElement(id) {
    var el = getElement(id);
    if (el) el.focus();
  }

  function readValue(id) {
    var el = getElement(id);
    return el ? el.value.trim() : '';
  }

  function setVisible(id, visible) {
    var el = getElement(id);
    if (el) {
      el.style.display = visible ? 'block' : 'none';
    }
  }

  function toggleServicePanel(panelID) {
    var panels = ['manage-service-panel', 'retrieve-service-panel', 'refund-service-panel', 'data-service-panel'];
    for (var i = 0; i < panels.length; i++) {
      var panel = getElement(panels[i]);
      if (!panel) continue;
      panel.classList.toggle('visible', panels[i] === panelID ? !panel.classList.contains('visible') : false);
    }
  }

  function getFlowConfig(flowID) {
    return verificationFlows[flowID] || null;
  }

  async function requestVerificationCode(flowID) {
    var flow = getFlowConfig(flowID);
    if (!flow) return;
    var email = readValue(flow.emailInputID);
    if (!email) {
      focusElement(flow.emailInputID);
      return;
    }
    if (flow.beforeRequest) {
      flow.beforeRequest();
    }
    setButtonState(flow.requestButtonID, true, flow.requestPendingLabel);
    try {
      var res = await serviceFetch(flow.requestPath, { email: email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.requestErrorMessage);
      serviceState.pending[flow.stateKey] = email;
      if (flow.step2ID) {
        setVisible(flow.step2ID, true);
      }
      setServiceStatus(flow.statusID, flow.requestSuccessMessage, false);
    } catch (err) {
      setServiceStatus(flow.statusID, err.message, true);
    } finally {
      setButtonState(flow.requestButtonID, false, flow.requestLabel);
    }
  }

  async function resendVerificationCode(flowID, event) {
    if (event) event.preventDefault();
    var flow = getFlowConfig(flowID);
    if (!flow) return;
    var email = serviceState.pending[flow.stateKey];
    if (!email) return;
    try {
      var res = await serviceFetch(flow.requestPath, { email: email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.requestErrorMessage);
      setServiceStatus(flow.statusID, flow.resendSuccessMessage, false);
    } catch (err) {
      setServiceStatus(flow.statusID, err.message, true);
    }
  }

  async function confirmVerificationCode(flowID) {
    var flow = getFlowConfig(flowID);
    if (!flow) return;
    var email = serviceState.pending[flow.stateKey];
    var code = readValue(flow.codeInputID);
    if (!email || !code) return;
    if (flow.beforeConfirm && flow.beforeConfirm() === false) {
      return;
    }
    setButtonState(flow.confirmButtonID, true, flow.confirmPendingLabel);
    try {
      var res = await serviceFetch(flow.confirmPath, { email: email, code: code });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || flow.confirmErrorMessage);
      flow.onConfirmSuccess(data, email);
    } catch (err) {
      setServiceStatus(flow.statusID, err.message, true);
      if (flow.restoreOnError !== false) {
        setButtonState(flow.confirmButtonID, false, flow.confirmLabel);
      }
      return;
    }
    if (flow.restoreOnSuccess !== false) {
      setButtonState(flow.confirmButtonID, false, flow.confirmLabel);
    }
  }

  function resetVerificationFlow(flow) {
    serviceState.pending[flow.stateKey] = '';
    if (flow.codeInputID) {
      var codeInput = getElement(flow.codeInputID);
      if (codeInput) codeInput.value = '';
    }
    if (flow.step2ID) {
      setVisible(flow.step2ID, false);
    }
  }

  var verificationFlows = {
    manage: {
      stateKey: 'manage',
      requestPath: '/v1/manage/request',
      confirmPath: '/v1/manage',
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
      restoreOnSuccess: false,
      onConfirmSuccess: function(data) {
        window.location.href = data.url;
      }
    },
    retrieve: {
      stateKey: 'retrieve',
      requestPath: '/v1/retrieve-license/request',
      confirmPath: '/v1/retrieve-license',
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
      beforeRequest: function() {
        setVisible('retrieve-inline-result', false);
      },
      onConfirmSuccess: function(data) {
        var license = data.license;
        getElement('retrieve-inline-token').value = license.token;
        getElement('retrieve-inline-tier').textContent = license.tier;
        getElement('retrieve-inline-issued').textContent = new Date(license.issued_at).toLocaleString();
        getElement('retrieve-inline-expires').textContent = license.expires_at ? new Date(license.expires_at).toLocaleString() : 'Does not expire';
        getElement('retrieve-inline-email-value').textContent = license.email;
        setVisible('retrieve-inline-result', true);
        getElement('retrieve-inline-copy').style.display = 'inline-block';
        var invoice = getElement('retrieve-inline-invoice');
        if (license.invoice_url) {
          invoice.href = license.invoice_url;
          invoice.style.display = 'inline-block';
        } else {
          invoice.href = '#';
          invoice.style.display = 'none';
        }
        setServiceStatus('retrieve-inline-status', 'License retrieved successfully.', false);
      }
    },
    export: {
      stateKey: 'export',
      requestPath: '/v1/gdpr/request-export',
      confirmPath: '/v1/gdpr/export',
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
      beforeRequest: function() {
        setVisible('data-export-result', false);
      },
      onConfirmSuccess: function(data) {
        getElement('data-export-payload').value = JSON.stringify(data, null, 2);
        setVisible('data-export-result', true);
        setServiceStatus('data-export-status', 'Data export retrieved successfully.', false);
        resetVerificationFlow(verificationFlows.export);
      }
    },
    delete: {
      stateKey: 'delete',
      requestPath: '/v1/gdpr/request-delete',
      confirmPath: '/v1/gdpr/confirm-delete',
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
          setServiceStatus('data-delete-status', 'You must confirm that you understand this action is permanent.', true);
          return false;
        }
        return true;
      },
      onConfirmSuccess: function(data) {
        setServiceStatus('data-delete-status', data.deleted_count > 0 && data.stripe_reminder ? data.message + ' ' + data.stripe_reminder : data.message, false);
        getElement('data-delete-confirm-check').checked = false;
        resetVerificationFlow(verificationFlows.delete);
      }
    }
  };

  async function copyRetrievedLicense() {
    var token = getElement('retrieve-inline-token').value;
    if (!token) return;
    try {
      await navigator.clipboard.writeText(token);
      setServiceStatus('retrieve-inline-status', 'License key copied to clipboard.', false);
    } catch (_) {
      setServiceStatus('retrieve-inline-status', 'Failed to copy automatically. Please copy the key manually.', true);
    }
  }

  async function submitRefund() {
    var email = readValue('refund-inline-email');
    var token = readValue('refund-inline-token');
    if (!email || !token) return;
    if (!confirm('Are you sure? This will immediately revoke the license and request the refund.')) return;
    setButtonState('refund-inline-submit', true, 'Processing...');
    try {
      var res = await serviceFetch('/v1/self-refund', { email: email, token: token });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Refund failed');
      getElement('refund-inline-token').value = '';
      setServiceStatus('refund-inline-status', 'Success! Your refund has been processed. Stripe will follow up by email.', false);
    } catch (err) {
      setServiceStatus('refund-inline-status', err.message, true);
    } finally {
      setButtonState('refund-inline-submit', false, 'Process Refund');
    }
  }

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
})();
