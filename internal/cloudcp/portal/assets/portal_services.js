(function() {
  var runtime = window.PulseAccountPortal || {};
  var serviceState = {
    manageEmail: '',
    retrieveEmail: '',
    exportEmail: '',
    deleteEmail: '',
  };

  function getCommercialAPIBaseURL() {
    return runtime.getCommercialAPIBaseURL ? runtime.getCommercialAPIBaseURL() : '';
  }

  function setServiceStatus(id, message, isError) {
    var el = document.getElementById(id);
    if (!el) return;
    el.textContent = message;
    el.className = 'service-status visible' + (isError ? ' error' : ' success');
  }

  function toggleServicePanel(panelID) {
    var panels = ['manage-service-panel', 'retrieve-service-panel', 'refund-service-panel', 'data-service-panel'];
    for (var i = 0; i < panels.length; i++) {
      var panel = document.getElementById(panels[i]);
      if (!panel) continue;
      panel.classList.toggle('visible', panels[i] === panelID ? !panel.classList.contains('visible') : false);
    }
  }

  function focusElement(id) {
    var el = document.getElementById(id);
    if (el) el.focus();
  }

  function serviceFetch(path, body) {
    return fetch(getCommercialAPIBaseURL() + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
  }

  async function openServicePanel(panelID, focusID) {
    toggleServicePanel(panelID);
    focusElement(focusID);
  }

  async function requestManageCode() {
    var button = document.getElementById('manage-inline-request');
    var email = document.getElementById('manage-inline-email').value.trim();
    if (!email) { focusElement('manage-inline-email'); return; }
    button.disabled = true;
    button.textContent = 'Sending...';
    try {
      var res = await serviceFetch('/v1/manage/request', { email: email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to send verification code');
      serviceState.manageEmail = email;
      document.getElementById('manage-inline-step2').style.display = 'block';
      setServiceStatus('manage-inline-status', 'Verification code sent. Check your email.', false);
    } catch (err) {
      setServiceStatus('manage-inline-status', err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Send Verification Code';
    }
  }

  async function resendManageCode(event) {
    event.preventDefault();
    if (!serviceState.manageEmail) return;
    try {
      var res = await serviceFetch('/v1/manage/request', { email: serviceState.manageEmail });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to send verification code');
      setServiceStatus('manage-inline-status', 'New verification code sent.', false);
    } catch (err) {
      setServiceStatus('manage-inline-status', err.message, true);
    }
  }

  async function confirmManage() {
    var button = document.getElementById('manage-inline-confirm');
    var code = document.getElementById('manage-inline-code').value.trim();
    if (!serviceState.manageEmail || !code) return;
    button.disabled = true;
    button.textContent = 'Redirecting...';
    try {
      var res = await serviceFetch('/v1/manage', { email: serviceState.manageEmail, code: code });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to open customer portal');
      window.location.href = data.url;
    } catch (err) {
      setServiceStatus('manage-inline-status', err.message, true);
      button.disabled = false;
      button.textContent = 'Open Customer Portal';
    }
  }

  async function requestRetrieveCode() {
    var button = document.getElementById('retrieve-inline-request');
    var email = document.getElementById('retrieve-inline-email').value.trim();
    if (!email) { focusElement('retrieve-inline-email'); return; }
    button.disabled = true;
    button.textContent = 'Sending...';
    document.getElementById('retrieve-inline-result').style.display = 'none';
    try {
      var res = await serviceFetch('/v1/retrieve-license/request', { email: email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to send verification code');
      serviceState.retrieveEmail = email;
      document.getElementById('retrieve-inline-step2').style.display = 'block';
      setServiceStatus('retrieve-inline-status', 'Verification code sent. Check your email.', false);
    } catch (err) {
      setServiceStatus('retrieve-inline-status', err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Send Verification Code';
    }
  }

  async function confirmRetrieve() {
    var button = document.getElementById('retrieve-inline-confirm');
    var code = document.getElementById('retrieve-inline-code').value.trim();
    if (!serviceState.retrieveEmail || !code) return;
    button.disabled = true;
    button.textContent = 'Loading...';
    try {
      var res = await serviceFetch('/v1/retrieve-license', { email: serviceState.retrieveEmail, code: code });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to retrieve license');
      var license = data.license;
      document.getElementById('retrieve-inline-token').value = license.token;
      document.getElementById('retrieve-inline-tier').textContent = license.tier;
      document.getElementById('retrieve-inline-issued').textContent = new Date(license.issued_at).toLocaleString();
      document.getElementById('retrieve-inline-expires').textContent = license.expires_at ? new Date(license.expires_at).toLocaleString() : 'Does not expire';
      document.getElementById('retrieve-inline-email-value').textContent = license.email;
      document.getElementById('retrieve-inline-result').style.display = 'block';
      document.getElementById('retrieve-inline-copy').style.display = 'inline-block';
      if (license.invoice_url) {
        var invoice = document.getElementById('retrieve-inline-invoice');
        invoice.href = license.invoice_url;
        invoice.style.display = 'inline-block';
      }
      setServiceStatus('retrieve-inline-status', 'License retrieved successfully.', false);
    } catch (err) {
      setServiceStatus('retrieve-inline-status', err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Show License';
    }
  }

  async function copyRetrievedLicense() {
    var token = document.getElementById('retrieve-inline-token').value;
    if (!token) return;
    try {
      await navigator.clipboard.writeText(token);
      setServiceStatus('retrieve-inline-status', 'License key copied to clipboard.', false);
    } catch (_) {
      setServiceStatus('retrieve-inline-status', 'Failed to copy automatically. Please copy the key manually.', true);
    }
  }

  async function submitRefund() {
    var button = document.getElementById('refund-inline-submit');
    var email = document.getElementById('refund-inline-email').value.trim();
    var token = document.getElementById('refund-inline-token').value.trim();
    if (!email || !token) return;
    if (!confirm('Are you sure? This will immediately revoke the license and request the refund.')) return;
    button.disabled = true;
    button.textContent = 'Processing...';
    try {
      var res = await serviceFetch('/v1/self-refund', { email: email, token: token });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Refund failed');
      document.getElementById('refund-inline-token').value = '';
      setServiceStatus('refund-inline-status', 'Success! Your refund has been processed. Stripe will follow up by email.', false);
    } catch (err) {
      setServiceStatus('refund-inline-status', err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Process Refund';
    }
  }

  async function requestExportCode() {
    var button = document.getElementById('data-export-request');
    var email = document.getElementById('data-export-email').value.trim();
    if (!email) return;
    button.disabled = true;
    button.textContent = 'Sending...';
    document.getElementById('data-export-result').style.display = 'none';
    try {
      var res = await serviceFetch('/v1/gdpr/request-export', { email: email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Request failed');
      serviceState.exportEmail = email;
      document.getElementById('data-export-step2').style.display = 'block';
      setServiceStatus('data-export-status', 'Verification code sent. Check your email.', false);
    } catch (err) {
      setServiceStatus('data-export-status', err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Send Verification Code';
    }
  }

  async function resendExportCode(event) {
    event.preventDefault();
    if (!serviceState.exportEmail) return;
    try {
      var res = await serviceFetch('/v1/gdpr/request-export', { email: serviceState.exportEmail });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Request failed');
      setServiceStatus('data-export-status', 'New verification code sent.', false);
    } catch (err) {
      setServiceStatus('data-export-status', err.message, true);
    }
  }

  async function confirmExport() {
    var button = document.getElementById('data-export-confirm');
    var code = document.getElementById('data-export-code').value.trim();
    if (!serviceState.exportEmail || !code) return;
    button.disabled = true;
    button.textContent = 'Exporting...';
    try {
      var res = await serviceFetch('/v1/gdpr/export', { email: serviceState.exportEmail, code: code });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Export failed');
      document.getElementById('data-export-payload').value = JSON.stringify(data, null, 2);
      document.getElementById('data-export-result').style.display = 'block';
      setServiceStatus('data-export-status', 'Data export retrieved successfully.', false);
      document.getElementById('data-export-code').value = '';
      serviceState.exportEmail = '';
      document.getElementById('data-export-step2').style.display = 'none';
    } catch (err) {
      setServiceStatus('data-export-status', err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Export My Data';
    }
  }

  async function requestDeleteCode() {
    var button = document.getElementById('data-delete-request');
    var email = document.getElementById('data-delete-email').value.trim();
    if (!email) return;
    button.disabled = true;
    button.textContent = 'Sending...';
    try {
      var res = await serviceFetch('/v1/gdpr/request-delete', { email: email });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Request failed');
      serviceState.deleteEmail = email;
      document.getElementById('data-delete-step2').style.display = 'block';
      setServiceStatus('data-delete-status', 'Verification code sent. Check your email.', false);
    } catch (err) {
      setServiceStatus('data-delete-status', err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Send Verification Code';
    }
  }

  async function resendDeleteCode(event) {
    event.preventDefault();
    if (!serviceState.deleteEmail) return;
    try {
      var res = await serviceFetch('/v1/gdpr/request-delete', { email: serviceState.deleteEmail });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Request failed');
      setServiceStatus('data-delete-status', 'New verification code sent.', false);
    } catch (err) {
      setServiceStatus('data-delete-status', err.message, true);
    }
  }

  async function confirmDelete() {
    var button = document.getElementById('data-delete-confirm');
    var code = document.getElementById('data-delete-code').value.trim();
    if (!serviceState.deleteEmail || !code) return;
    if (!document.getElementById('data-delete-confirm-check').checked) {
      setServiceStatus('data-delete-status', 'You must confirm that you understand this action is permanent.', true);
      return;
    }
    button.disabled = true;
    button.textContent = 'Deleting...';
    try {
      var res = await serviceFetch('/v1/gdpr/confirm-delete', { email: serviceState.deleteEmail, code: code });
      var data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Deletion failed');
      setServiceStatus('data-delete-status', data.deleted_count > 0 && data.stripe_reminder ? data.message + ' ' + data.stripe_reminder : data.message, false);
      document.getElementById('data-delete-step2').style.display = 'none';
      document.getElementById('data-delete-code').value = '';
      document.getElementById('data-delete-confirm-check').checked = false;
      serviceState.deleteEmail = '';
    } catch (err) {
      setServiceStatus('data-delete-status', err.message, true);
    } finally {
      button.disabled = false;
      button.textContent = 'Delete My Data';
    }
  }

  document.addEventListener('click', function(event) {
    var target = event.target.closest('[data-account-service-action]');
    if (!target) return;
    var action = target.getAttribute('data-account-service-action') || '';
    switch (action) {
      case 'open-manage-service':
        event.preventDefault();
        openServicePanel('manage-service-panel', 'manage-inline-email');
        return;
      case 'open-retrieve-service':
        event.preventDefault();
        openServicePanel('retrieve-service-panel', 'retrieve-inline-email');
        return;
      case 'open-refund-service':
        event.preventDefault();
        openServicePanel('refund-service-panel', 'refund-inline-email');
        return;
      case 'open-data-service':
        event.preventDefault();
        openServicePanel('data-service-panel', 'data-export-email');
        return;
      case 'manage-inline-request':
        event.preventDefault();
        requestManageCode();
        return;
      case 'manage-inline-resend':
        resendManageCode(event);
        return;
      case 'manage-inline-confirm':
        event.preventDefault();
        confirmManage();
        return;
      case 'retrieve-inline-request':
        event.preventDefault();
        requestRetrieveCode();
        return;
      case 'retrieve-inline-confirm':
        event.preventDefault();
        confirmRetrieve();
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
        requestExportCode();
        return;
      case 'data-export-resend':
        resendExportCode(event);
        return;
      case 'data-export-confirm':
        event.preventDefault();
        confirmExport();
        return;
      case 'data-delete-request':
        event.preventDefault();
        requestDeleteCode();
        return;
      case 'data-delete-resend':
        resendDeleteCode(event);
        return;
      case 'data-delete-confirm':
        event.preventDefault();
        confirmDelete();
        return;
      default:
        return;
    }
  });
})();
