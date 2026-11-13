import {
  beginMutationState,
  beginQueryState,
  failMutationState,
  failQueryState,
  resolveQueryState,
  succeedMutationState,
} from './async_state';
import { PortalAPIError, type PortalAPI } from './api';
import { createVerificationSendWindow, trimVerificationEmail, verificationEmailKey } from './verification_send';
import { installBillingController } from './billing_controller';
import {
  clearFlowStatus,
  createPortalBillingState,
  emptyStatus,
  resetVerificationFlowState,
  setFlowStatus,
  setRefundStatus,
  toggleBillingPanelState,
  updateDeleteConfirmation,
  updateBillingInputValue,
} from './state';
import type { PortalStore } from './store';
import {
  focusElement,
  getElement,
  readValue,
  renderButton,
  renderDeletePanel,
  renderExportPanel,
  renderExportResult,
  renderManagePanel,
  renderOpenBillingPanels,
  renderRefundPanel,
  renderRetrievePanel,
  renderBillingStatus,
  renderUpgradePanel,
  setValue,
  setVisible,
} from './billing_view';
import type {
  PortalBillingFlowID,
  PortalBillingState,
  PortalCheckoutSessionCreateResponse,
  PortalUpgradePortalHandoffModel,
  PortalUpgradePricingModel,
  VerificationFlowState,
} from './types';

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
  resendID?: string;
  requestLabel: string;
  requestPendingLabel: string;
  confirmLabel?: string;
  confirmPendingLabel?: string;
  requestErrorMessage: string;
  confirmErrorMessage: string;
  readCodeValue?: () => string;
  onRequestStart?: () => void;
  beforeConfirm?: () => boolean;
  applyConfirmSuccessState?: (billingState: PortalBillingState, data: any, email?: string) => void;
  afterConfirmSuccess?: (data: any, email?: string) => void;
  renderPanel: (flowState: VerificationFlowState) => void;
  renderResult?: (result: unknown) => void;
}

export interface BillingRuntimeDeps {
  api: PortalAPI;
  store: PortalStore;
}

export function installBillingRuntime(deps: BillingRuntimeDeps): void {
  var api = deps.api;
  var store = deps.store;
  var sendWindow = createVerificationSendWindow();
  var flowVersions = { manage: 0, retrieve: 0, export: 0, delete: 0 };
  var sessionVersion = 0;
  var departed = false;
  var sendTimer: ReturnType<typeof setInterval> | undefined;
  var sessionIdentity = currentSessionIdentity();

  function currentSessionIdentity(): string {
    var bootstrap = store.getBootstrap();
    return JSON.stringify([bootstrap.authenticated, bootstrap.email, bootstrap.commercial_api_base_url]);
  }

  function sameMailbox(left: string, right: string): boolean {
    return !!verificationEmailKey(left) && verificationEmailKey(left) === verificationEmailKey(right);
  }

  function confirmPendingFor(email: string): boolean {
    return Object.values(getBillingState().flows).some(function(state) {
      return state.confirm.pending && sameMailbox(state.pendingEmail, email);
    });
  }

  function updateSendControls(): void {
    (Object.keys(verificationFlows) as PortalBillingFlowID[]).forEach(function(flowID) {
      var flow = verificationFlows[flowID];
      var state = getBillingState().flows[flowID];
      var windowState = sendWindow.state(state.emailValue);
      var confirmPending = confirmPendingFor(state.emailValue);
      var label = windowState.pending ? flow.requestPendingLabel
        : windowState.seconds ? 'Request again in ' + windowState.seconds + 's' : flow.requestLabel;
      renderButton(flow.requestButtonID, departed || windowState.pending || !!windowState.seconds || confirmPending, label);
      var note = getElement(flow.statusID + '-send-note');
      if (note) {
        var message = windowState.pending ? 'A code request for this email is in progress.'
          : windowState.seconds ? 'One request per minute per email, shared across actions. Codes only work for their original action.' : '';
        if (note.textContent !== message) note.textContent = message;
        note.hidden = !message;
      }
      var resend = flow.resendID ? getElement(flow.resendID) : null;
      if (resend) {
        var resendWindow = sendWindow.state(state.pendingEmail);
        var blocked = departed || resendWindow.pending || !!resendWindow.seconds || confirmPendingFor(state.pendingEmail);
        resend.setAttribute('aria-disabled', String(blocked));
        var resendLabel = resendWindow.pending ? 'Request pending…'
          : resendWindow.seconds ? 'Wait ' + resendWindow.seconds + 's' : 'Send again';
        if (resend.textContent !== resendLabel) resend.textContent = resendLabel;
      }
      var targetMatches = sameMailbox(state.emailValue, state.pendingEmail);
      var emailInput = getElement<HTMLInputElement>(flow.emailInputID);
      if (emailInput) emailInput.disabled = state.confirm.pending;
      var codeInput = flow.codeInputID ? getElement<HTMLInputElement>(flow.codeInputID) : null;
      if (codeInput) codeInput.disabled = state.confirm.pending || sendWindow.state(state.pendingEmail).pending;
      if (flowID === 'delete') {
        var checkbox = getElement<HTMLInputElement>('data-delete-confirm-check');
        if (checkbox) checkbox.disabled = state.confirm.pending || sendWindow.state(state.pendingEmail).pending;
      }
      renderButton(flow.confirmButtonID,
        departed || state.confirm.pending || sendWindow.state(state.pendingEmail).pending || !targetMatches,
        state.confirm.pending ? flow.confirmPendingLabel : flow.confirmLabel);
      if (flow.step2ID) setVisible(flow.step2ID, state.step2Visible && targetMatches);
    });
    if (!sendWindow.active() && sendTimer !== undefined) {
      clearInterval(sendTimer);
      sendTimer = undefined;
    }
  }

  function watchSendWindow(): void {
    if (sendTimer === undefined) sendTimer = setInterval(updateSendControls, 1000);
  }

  function retireVerificationState(seedEmail = ''): void {
    sessionVersion += 1;
    sendWindow.clear();
    if (sendTimer !== undefined) clearInterval(sendTimer);
    sendTimer = undefined;
    updateBillingState(function(state) {
      state.flows = createPortalBillingState().flows;
      Object.values(state.flows).forEach(function(flow) { flow.emailValue = seedEmail; });
    }, false);
  }

  window.addEventListener('pagehide', function() {
    departed = true;
    retireVerificationState();
    renderAllFlows();
  });
  window.addEventListener('pageshow', function() {
    departed = false;
    updateSendControls();
  });

  store.updateBillingState(function(billingState) {
    if (!billingState.flows) {
      var nextState = createPortalBillingState();
      billingState.openBillingPanelID = nextState.openBillingPanelID;
      billingState.upgradeFeatureKey = nextState.upgradeFeatureKey;
      billingState.upgradePortalHandoffID = nextState.upgradePortalHandoffID;
      billingState.upgradePortalHandoff = nextState.upgradePortalHandoff;
      billingState.upgradePricing = nextState.upgradePricing;
      billingState.upgradeCheckout = nextState.upgradeCheckout;
      billingState.flows = nextState.flows;
      billingState.refund = nextState.refund;
    }
  }, { notify: false });

  function getBillingState() {
    return store.getBillingState();
  }

  function updateBillingState(mutator: (state: PortalBillingState) => void, notify = true) {
    return store.updateBillingState(mutator, { notify: notify });
  }

  function toggleBillingPanel(panelID) {
    updateBillingState(function(billingState) {
      toggleBillingPanelState(billingState, panelID);
    });
  }

  function clearBillingPanel() {
    updateBillingState(function(billingState) {
      billingState.openBillingPanelID = '';
    });
  }

  function renderFlow(flowID: PortalBillingFlowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var flowState = getBillingState().flows[flowID];
    if (flow.renderPanel) {
      flow.renderPanel(flowState);
    }
    renderButton(flow.confirmButtonID, flowState.confirm.pending, flowState.confirm.pending ? flow.confirmPendingLabel : flow.confirmLabel);
    renderBillingStatus(flow.statusID, flowState.status);
    if (flow.step2ID) {
      setVisible(flow.step2ID, flowState.step2Visible);
    }
    if (flow.renderResult) {
      flow.renderResult(flowState.result);
    }
    updateSendControls();
  }

  function renderAllFlows() {
    renderUpgrade();
    renderFlow('manage');
    renderFlow('retrieve');
    renderFlow('export');
    renderFlow('delete');
    renderRefund();
  }

  function renderRefund() {
    var refundState = getBillingState().refund;
    renderRefundPanel(refundState, store.getBootstrap());
    renderButton('refund-inline-submit', refundState.submit.pending, refundState.submit.pending ? 'Processing...' : 'Process Refund');
    renderBillingStatus('refund-inline-status', refundState.status);
  }

  function renderUpgrade() {
    renderUpgradePanel(getBillingState(), store.getBootstrap());
  }

  async function loadUpgradePricing(force: boolean) {
    var billingState = getBillingState();
    if (!force && (billingState.upgradePricing.status === 'loading' || billingState.upgradePricing.status === 'ready')) {
      return;
    }
    updateBillingState(function(nextBillingState) {
      beginQueryState(nextBillingState.upgradePricing, null);
    });
    try {
      var pricing = await api.getCommercialJSON<PortalUpgradePricingModel>('/v1/public/pricing-model?track=v6');
      updateBillingState(function(nextBillingState) {
        resolveQueryState(nextBillingState.upgradePricing, pricing);
      });
    } catch (err) {
      updateBillingState(function(nextBillingState) {
        failQueryState(
          nextBillingState.upgradePricing,
          null,
          err instanceof Error ? err.message : 'Failed to load self-hosted plans.',
        );
      });
    }
  }

  async function resolveUpgradePortalHandoff(force: boolean) {
    var billingState = getBillingState();
    var portalHandoffID = String(billingState.upgradePortalHandoffID || '').trim();
    if (!portalHandoffID) return;
    if (!force && (billingState.upgradePortalHandoff.status === 'loading' || billingState.upgradePortalHandoff.status === 'ready')) {
      return;
    }
    updateBillingState(function(nextBillingState) {
      beginQueryState(nextBillingState.upgradePortalHandoff, null);
    });
    try {
      var handoff = await api.getCommercialJSON<PortalUpgradePortalHandoffModel>(
        '/v1/checkout/portal-handoff?portal_handoff_id=' + encodeURIComponent(portalHandoffID),
      );
      updateBillingState(function(nextBillingState) {
        resolveQueryState(nextBillingState.upgradePortalHandoff, handoff);
        nextBillingState.upgradeFeatureKey = String(handoff.feature || '').trim();
      }, false);
    } catch (err) {
      updateBillingState(function(nextBillingState) {
        failQueryState(
          nextBillingState.upgradePortalHandoff,
          null,
          err instanceof Error ? err.message : 'Failed to verify the secure plan upgrade handoff.',
        );
      });
    }
  }

  async function startUpgradeCheckout(planKey: string, tier: string, billingCycle: string) {
    if (!planKey || !tier || !billingCycle) return;
    var billingState = getBillingState();
    var portalHandoffID = String(billingState.upgradePortalHandoffID || '').trim();
    var handoffLifecycle = String(
      (billingState.upgradePortalHandoff.data && billingState.upgradePortalHandoff.data.status) || '',
    ).trim();
    if (!portalHandoffID || billingState.upgradePortalHandoff.status !== 'ready') {
      updateBillingState(function(nextBillingState) {
        failMutationState(
          nextBillingState.upgradeCheckout,
          'Pulse Account could not verify the secure upgrade handoff. Reopen the upgrade flow from the Plans page in Pulse.',
        );
      });
      return;
    }
    if (handoffLifecycle === 'completed') {
      updateBillingState(function(nextBillingState) {
        failMutationState(
          nextBillingState.upgradeCheckout,
          'This secure upgrade handoff already completed. Return to the Plans page in Pulse to review the live plan state.',
        );
      });
      return;
    }
    updateBillingState(function(nextBillingState) {
      beginMutationState(nextBillingState.upgradeCheckout);
    });
    try {
      var trialRequested = billingState.upgradeTrialRequested === true &&
        tier === 'pro' &&
        billingCycle === 'monthly';
      var data = await api.postCommercialJSON<PortalCheckoutSessionCreateResponse>('/v1/checkout/session', {
        plan_key: planKey,
        tier: tier,
        billing_cycle: billingCycle,
        portal_handoff_id: portalHandoffID,
        ...(trialRequested ? { trial: true } : {}),
      });
      if (!data || !data.url) {
        throw new Error('Checkout URL was not returned.');
      }
      updateBillingState(function(nextBillingState) {
        succeedMutationState(nextBillingState.upgradeCheckout);
      });
      window.location.href = data.url;
    } catch (err) {
      updateBillingState(function(nextBillingState) {
        failMutationState(
          nextBillingState.upgradeCheckout,
          err instanceof Error ? err.message : 'Failed to start checkout.',
        );
      });
    }
  }

  var verificationFlows: Record<PortalBillingFlowID, VerificationFlowDefinition> = {
    manage: {
      requestPath: '/v1/manage/request',
      confirmPath: '/v1/manage',
      panelID: 'manage-billing-panel',
      emailInputID: 'manage-inline-email',
      codeInputID: 'manage-inline-code',
      requestButtonID: 'manage-inline-request',
      confirmButtonID: 'manage-inline-confirm',
      step2ID: 'manage-inline-step2',
      statusID: 'manage-inline-status',
      resendID: 'manage-inline-resend',
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Open Customer Portal',
      confirmPendingLabel: 'Redirecting...',
      requestErrorMessage: 'Failed to send verification code',
      confirmErrorMessage: 'Failed to open customer portal',
      readCodeValue: function() {
        return getBillingState().flows.manage.codeValue;
      },
      onRequestStart: function() {},
      afterConfirmSuccess: function(data) {
        window.location.href = data.url;
      },
      renderPanel: renderManagePanel,
    },
    retrieve: {
      requestPath: '/v1/retrieve-license/request',
      confirmPath: '/v1/retrieve-license',
      panelID: 'retrieve-billing-panel',
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
      requestErrorMessage: 'Failed to send verification code',
      confirmErrorMessage: 'Failed to retrieve license',
      readCodeValue: function() {
        return getBillingState().flows.retrieve.codeValue;
      },
      onRequestStart: function() {
        updateBillingState(function(billingState) {
          billingState.flows.retrieve.result = null;
        }, false);
      },
      applyConfirmSuccessState: function(billingState, data) {
        billingState.flows.retrieve.result = data.license;
        billingState.flows.retrieve.codeValue = '';
        setFlowStatus(billingState, 'retrieve', 'License retrieved successfully.', false);
      },
      renderPanel: renderRetrievePanel,
    },
    export: {
      requestPath: '/v1/gdpr/request-export',
      confirmPath: '/v1/gdpr/export',
      panelID: 'data-billing-panel',
      emailInputID: 'data-export-email',
      codeInputID: 'data-export-code',
      requestButtonID: 'data-export-request',
      confirmButtonID: 'data-export-confirm',
      step2ID: 'data-export-step2',
      statusID: 'data-export-status',
      resendID: 'data-export-resend',
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Export My Data',
      confirmPendingLabel: 'Exporting...',
      requestErrorMessage: 'Request failed',
      confirmErrorMessage: 'Export failed',
      readCodeValue: function() {
        return getBillingState().flows.export.codeValue;
      },
      onRequestStart: function() {
        updateBillingState(function(billingState) {
          billingState.flows.export.result = null;
        }, false);
      },
      applyConfirmSuccessState: function(billingState, data) {
        var emailValue = billingState.flows.export.emailValue;
        resetVerificationFlowState(billingState, 'export');
        billingState.flows.export.emailValue = emailValue;
        billingState.flows.export.result = data;
        setFlowStatus(billingState, 'export', 'Data export retrieved successfully.', false);
      },
      renderPanel: renderExportPanel,
      renderResult: renderExportResult,
    },
    delete: {
      requestPath: '/v1/gdpr/request-delete',
      confirmPath: '/v1/gdpr/confirm-delete',
      panelID: 'data-billing-panel',
      emailInputID: 'data-delete-email',
      codeInputID: 'data-delete-code',
      requestButtonID: 'data-delete-request',
      confirmButtonID: 'data-delete-confirm',
      step2ID: 'data-delete-step2',
      statusID: 'data-delete-status',
      resendID: 'data-delete-resend',
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Delete My Data',
      confirmPendingLabel: 'Deleting...',
      requestErrorMessage: 'Request failed',
      confirmErrorMessage: 'Deletion failed',
      readCodeValue: function() {
        return getBillingState().flows.delete.codeValue;
      },
      beforeConfirm: function() {
        if (!getElement<HTMLInputElement>('data-delete-confirm-check')?.checked) {
          updateBillingState(function(billingState) {
            setFlowStatus(billingState, 'delete', 'You must confirm that you understand this action is permanent.', true);
          });
          return false;
        }
        return true;
      },
      applyConfirmSuccessState: function(billingState, data) {
        var emailValue = billingState.flows.delete.emailValue;
        resetVerificationFlowState(billingState, 'delete');
        billingState.flows.delete.emailValue = emailValue;
        setFlowStatus(
          billingState,
          'delete',
          data.deleted_count > 0 && data.stripe_reminder ? data.message + ' ' + data.stripe_reminder : data.message,
          false
        );
      },
      afterConfirmSuccess: function() {
        var checkbox = getElement<HTMLInputElement>('data-delete-confirm-check');
        if (checkbox) {
          checkbox.checked = false;
        }
      },
      renderPanel: renderDeletePanel,
    },
  };

  async function requestVerificationCode(flowID: PortalBillingFlowID, resend = false) {
    var flow = verificationFlows[flowID];
    if (!flow || departed) return;
    var state = getBillingState().flows[flowID];
    var email = trimVerificationEmail(resend ? state.pendingEmail : state.emailValue);
    if (!email) {
      focusElement(flow.emailInputID);
      return;
    }
    if (!sameMailbox(state.emailValue, email) || confirmPendingFor(email)) return;
    var reservation = sendWindow.begin(email);
    if (!reservation) {
      updateSendControls();
      return;
    }
    var version = ++flowVersions[flowID];
    var session = sessionVersion;
    // A deliberate new send can replace the challenge. A blocked local send
    // above leaves the earlier same-action code and consent untouched.
    if (flow.onRequestStart) flow.onRequestStart();
    updateBillingState(function(billingState) {
      var next = billingState.flows[flowID];
      next.codeValue = '';
      next.checkboxChecked = false;
      beginMutationState(next.request);
      clearFlowStatus(billingState, flowID);
    });
    watchSendWindow();
    function current(): boolean {
      return session === sessionVersion && version === flowVersions[flowID] &&
        !departed && sameMailbox(getBillingState().flows[flowID].emailValue, email);
    }
    function showCodeEntry(message: string, error: boolean): void {
      if (!current()) return;
      updateBillingState(function(billingState) {
        var next = billingState.flows[flowID];
        next.pendingEmail = email;
        next.step2Visible = !!flow.step2ID;
        succeedMutationState(next.request);
        setFlowStatus(billingState, flowID, message, error);
      });
    }
    try {
      await api.postCommercialJSON(flow.requestPath, { email: email });
      sendWindow.settle(reservation, true);
      showCodeEntry('If eligible, check your email. Use the latest code for this action.', false);
    } catch (err) {
      var definiteRejection = err instanceof PortalAPIError && [400, 429, 500].includes(err.status);
      sendWindow.settle(reservation, !definiteRejection);
      if (!definiteRejection) {
        showCodeEntry('The request outcome is unknown. If a code arrives, use the latest code for this action.', true);
      } else if (current()) {
        var message = err instanceof Error ? err.message : flow.requestErrorMessage;
        updateBillingState(function(billingState) {
          failMutationState(billingState.flows[flowID].request, message);
          setFlowStatus(billingState, flowID, message, true);
        });
      }
    } finally {
      updateSendControls();
    }
  }

  function resendVerificationCode(flowID: PortalBillingFlowID, event?: Event) {
    if (event) event.preventDefault();
    return requestVerificationCode(flowID, true);
  }

  async function confirmVerificationCode(flowID: PortalBillingFlowID) {
    var flow = verificationFlows[flowID];
    if (!flow || departed) return;
    var state = getBillingState().flows[flowID];
    var email = state.pendingEmail;
    if (state.confirm.pending || sendWindow.state(email).pending || !sameMailbox(state.emailValue, email)) return;
    var version = flowVersions[flowID];
    var session = sessionVersion;
    function current(): boolean {
      return session === sessionVersion && version === flowVersions[flowID] &&
        !departed && sameMailbox(getBillingState().flows[flowID].emailValue, email);
    }
    var code = flow.readCodeValue ? flow.readCodeValue() : readValue(flow.codeInputID);
    if (!email || !code) return;
    if (flow.beforeConfirm && flow.beforeConfirm() === false) {
      return;
    }
    updateBillingState(function(billingState) {
      beginMutationState(billingState.flows[flowID].confirm);
    });
    try {
      var data = await api.postCommercialJSON(flow.confirmPath, { email: email, code: code });
      if (!current()) return;
      flowVersions[flowID] += 1;
      updateBillingState(function(billingState) {
        succeedMutationState(billingState.flows[flowID].confirm);
        if (flow.applyConfirmSuccessState) {
          flow.applyConfirmSuccessState(billingState, data, email);
        }
      });
      if (flow.afterConfirmSuccess) {
        flow.afterConfirmSuccess(data, email);
      }
    } catch (err) {
      if (!current()) return;
      var message = err instanceof Error ? err.message : flow.confirmErrorMessage;
      updateBillingState(function(billingState) {
        failMutationState(billingState.flows[flowID].confirm, message);
        setFlowStatus(billingState, flowID, message, true);
      });
    }
  }

  async function copyRetrievedLicense() {
    var result = getBillingState().flows.retrieve.result as { token?: string } | null;
    var token = result && result.token ? result.token : '';
    if (!token) return;
    try {
      await navigator.clipboard.writeText(token);
      updateBillingState(function(billingState) {
        setFlowStatus(billingState, 'retrieve', 'License key copied to clipboard.', false);
      });
    } catch (_) {
      updateBillingState(function(billingState) {
        setFlowStatus(billingState, 'retrieve', 'Failed to copy automatically. Please copy the key manually.', true);
      });
    }
  }

  async function submitRefund() {
    var email = getBillingState().refund.emailValue;
    var token = getBillingState().refund.tokenValue;
    if (!email || !token) return;
    if (!confirm('Are you sure? This will immediately revoke the license and request the refund.')) return;
    updateBillingState(function(billingState) {
      beginMutationState(billingState.refund.submit);
      billingState.refund.status = emptyStatus();
    });
    try {
      await api.postCommercialJSON('/v1/self-refund', { email: email, token: token });
      updateBillingState(function(billingState) {
        billingState.refund.tokenValue = '';
        succeedMutationState(billingState.refund.submit);
        setRefundStatus(billingState, 'Success! Your refund has been processed. Stripe will follow up by email.', false);
      });
    } catch (err) {
      var message = err instanceof Error ? err.message : 'Refund failed';
      updateBillingState(function(billingState) {
        failMutationState(billingState.refund.submit, message);
        setRefundStatus(billingState, message, true);
      });
    }
  }

  function renderBillingRuntime() {
    var billingState = getBillingState();
    if (
      billingState.openBillingPanelID === 'upgrade-billing-panel' ||
      !!billingState.upgradeFeatureKey ||
      !!billingState.upgradePortalHandoffID
    ) {
      void loadUpgradePricing(false);
      void resolveUpgradePortalHandoff(false);
    }
    renderOpenBillingPanels(getBillingState().openBillingPanelID);
    renderAllFlows();
  }

  renderBillingRuntime();
  store.subscribeBootstrap(function() {
    var identity = currentSessionIdentity();
    if (identity !== sessionIdentity) {
      sessionIdentity = identity;
      retireVerificationState(store.getBootstrap().authenticated ? store.getBootstrap().email : '');
    }
    renderBillingRuntime();
  });
  store.subscribeBilling(renderBillingRuntime);

  installBillingController({
    setShellSection: function(section) {
      store.setActiveShellSection(section);
    },
    toggleBillingPanel,
    clearBillingPanel,
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
    reloadUpgradePricing: function() {
      void loadUpgradePricing(true);
    },
    startUpgradeCheckout: function(planKey, tier, billingCycle) {
      void startUpgradeCheckout(planKey, tier, billingCycle);
    },
    updateInputValue: function(inputKind, value) {
      var emailFlows: Record<string, PortalBillingFlowID> = {
        'manage-email': 'manage', 'retrieve-email': 'retrieve',
        'data-export-email': 'export', 'data-delete-email': 'delete',
      };
      var editedFlow = emailFlows[inputKind];
      if (editedFlow && getBillingState().flows[editedFlow].confirm.pending) {
        setValue(verificationFlows[editedFlow].emailInputID, getBillingState().flows[editedFlow].emailValue);
        return;
      }
      var changedMailbox = editedFlow &&
        verificationEmailKey(getBillingState().flows[editedFlow].emailValue) !== verificationEmailKey(value);
      updateBillingState(function(billingState) {
        updateBillingInputValue(billingState, inputKind, value);
        if (changedMailbox) {
          flowVersions[editedFlow!] += 1;
          resetVerificationFlowState(billingState, editedFlow!);
        }
      }, false);
      if (changedMailbox && editedFlow) {
        var flow = verificationFlows[editedFlow];
        setVisible(flow.step2ID!, false);
        setValue(flow.codeInputID!, '');
        var checkbox = getElement<HTMLInputElement>('data-delete-confirm-check');
        if (editedFlow === 'delete' && checkbox) checkbox.checked = false;
        renderBillingStatus(flow.statusID, getBillingState().flows[editedFlow].status);
        if (editedFlow === 'retrieve') {
          setVisible('retrieve-inline-result', false);
          setValue('retrieve-inline-token', '');
          setVisible('retrieve-inline-copy', false);
          setVisible('retrieve-inline-invoice', false);
        }
        if (editedFlow === 'export') renderExportResult(null);
      }
      updateSendControls();
    },
    updateDeleteConfirmation: function(checked) {
      updateBillingState(function(billingState) {
        updateDeleteConfirmation(billingState, checked);
      }, false);
    },
  });
}
