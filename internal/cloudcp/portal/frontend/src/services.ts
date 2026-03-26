import { getBootstrap, getCommercialAPIBaseURL as readCommercialAPIBaseURL, subscribePortalRender } from './runtime';
import { installServicesController } from './services_controller';
import {
  focusElement,
  getElement,
  readValue,
  renderButton,
  renderDeletePanel,
  renderExportPanel,
  renderExportResult,
  renderManagePanel,
  renderOpenPanels,
  renderRefundPanel,
  renderRetrievePanel,
  renderStatus,
  setValue,
  setVisible,
} from './services_view';
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

  function toggleServicePanel(panelID) {
    serviceState.openPanelID = serviceState.openPanelID === panelID ? '' : panelID;
    renderOpenPanels(serviceState.openPanelID);
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
    renderRefundPanel(serviceState.refund, getBootstrap());
    renderButton('refund-inline-submit', serviceState.refund.submitting, serviceState.refund.submitting ? 'Processing...' : 'Process Refund');
    renderStatus('refund-inline-status', serviceState.refund.status);
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
      renderPanel: renderManagePanel
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
      renderPanel: renderRetrievePanel
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
      renderPanel: renderExportPanel,
      renderResult: renderExportResult
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
      renderPanel: renderDeletePanel
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
    renderOpenPanels(serviceState.openPanelID);
    renderAllFlows();
  }

  renderServiceRuntime();
  subscribePortalRender(renderServiceRuntime);

  function updateInputValue(inputKind: string, value: string) {
    switch (inputKind) {
      case 'manage-email':
        serviceState.flows.manage.emailValue = value;
        return;
      case 'manage-code':
        serviceState.flows.manage.codeValue = value;
        return;
      case 'retrieve-email':
        serviceState.flows.retrieve.emailValue = value;
        return;
      case 'retrieve-code':
        serviceState.flows.retrieve.codeValue = value;
        return;
      case 'refund-email':
        serviceState.refund.emailValue = value;
        return;
      case 'refund-token':
        serviceState.refund.tokenValue = value;
        return;
      case 'data-export-email':
        serviceState.flows.export.emailValue = value;
        return;
      case 'data-export-code':
        serviceState.flows.export.codeValue = value;
        return;
      case 'data-delete-email':
        serviceState.flows.delete.emailValue = value;
        return;
      case 'data-delete-code':
        serviceState.flows.delete.codeValue = value;
        return;
      default:
        return;
    }
  }

  installServicesController({
    toggleServicePanel,
    focusElement,
    requestVerificationCode: function(flowID) {
      void requestVerificationCode(flowID);
    },
    resendVerificationCode: function(flowID, event) {
      void resendVerificationCode(flowID, event);
    },
    confirmVerificationCode: function(flowID) {
      void confirmVerificationCode(flowID);
    },
    copyRetrievedLicense: function() {
      void copyRetrievedLicense();
    },
    submitRefund: function() {
      void submitRefund();
    },
    updateInputValue,
    updateDeleteConfirmation: function(checked) {
      serviceState.flows.delete.checkboxChecked = checked;
    },
  });
