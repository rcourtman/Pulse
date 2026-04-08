import {
  beginMutationState,
  beginQueryState,
  failMutationState,
  failQueryState,
  resolveQueryState,
  succeedMutationState,
} from './async_state';
import type { PortalAPI } from './api';
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
  PortalUpgradeCheckoutIntentModel,
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

  store.updateBillingState(function(billingState) {
    if (!billingState.flows) {
      var nextState = createPortalBillingState();
      billingState.openBillingPanelID = nextState.openBillingPanelID;
      billingState.upgradeFeatureKey = nextState.upgradeFeatureKey;
      billingState.upgradePortalHandoffID = nextState.upgradePortalHandoffID;
      billingState.upgradePortalHandoff = nextState.upgradePortalHandoff;
      billingState.upgradeCheckoutIntentID = nextState.upgradeCheckoutIntentID;
      billingState.upgradeCheckoutIntent = nextState.upgradeCheckoutIntent;
      billingState.upgradePricing = nextState.upgradePricing;
      billingState.upgradeCheckout = nextState.upgradeCheckout;
      billingState.flows = nextState.flows;
      billingState.refund = nextState.refund;
    }
  }, { notify: false });

  function getBillingState() {
    return store.getBillingState();
  }

  function updateBillingState(mutator, notify = true) {
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
    renderButton(flow.requestButtonID, flowState.request.pending, flowState.request.pending ? flow.requestPendingLabel : flow.requestLabel);
    renderButton(flow.confirmButtonID, flowState.confirm.pending, flowState.confirm.pending ? flow.confirmPendingLabel : flow.confirmLabel);
    renderBillingStatus(flow.statusID, flowState.status);
    if (flow.step2ID) {
      setVisible(flow.step2ID, flowState.step2Visible);
    }
    if (flow.renderResult) {
      flow.renderResult(flowState.result);
    }
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

  function resetVerificationFlow(flowID: PortalBillingFlowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    updateBillingState(function(billingState) {
      resetVerificationFlowState(billingState, flowID);
    }, false);
    if (flow.codeInputID) {
      setValue(flow.codeInputID, '');
    }
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

  async function resolveUpgradeCheckoutIntent(force: boolean) {
    var billingState = getBillingState();
    var portalHandoffID = String(billingState.upgradePortalHandoffID || '').trim();
    var checkoutIntentID = String(billingState.upgradeCheckoutIntentID || '').trim();
    if (!portalHandoffID && !checkoutIntentID) return;
    if (!force && (billingState.upgradeCheckoutIntent.status === 'loading' || billingState.upgradeCheckoutIntent.status === 'ready')) {
      return;
    }
    updateBillingState(function(nextBillingState) {
      if (portalHandoffID) {
        beginQueryState(nextBillingState.upgradePortalHandoff, null);
      }
      beginQueryState(nextBillingState.upgradeCheckoutIntent, null);
    });
    try {
      var resolvedCheckoutIntentID = checkoutIntentID;
      var resolvedFeature = '';
      if (portalHandoffID) {
        var handoff = await api.getCommercialJSON<PortalUpgradePortalHandoffModel>(
          '/v1/checkout/portal-handoff?portal_handoff_id=' + encodeURIComponent(portalHandoffID),
        );
        resolvedCheckoutIntentID = String(handoff.checkout_intent_id || '').trim();
        resolvedFeature = String(handoff.feature || '').trim();
        updateBillingState(function(nextBillingState) {
          resolveQueryState(nextBillingState.upgradePortalHandoff, handoff);
        }, false);
      } else {
        var legacyResult = await api.getCommercialJSON<PortalUpgradeCheckoutIntentModel>(
          '/v1/checkout/intent?checkout_intent_id=' + encodeURIComponent(checkoutIntentID),
        );
        resolvedCheckoutIntentID = String(legacyResult.checkout_intent_id || '').trim();
        resolvedFeature = String(legacyResult.feature || '').trim();
        updateBillingState(function(nextBillingState) {
          resolveQueryState(nextBillingState.upgradeCheckoutIntent, legacyResult);
        }, false);
      }
      if (!resolvedCheckoutIntentID) {
        throw new Error('Pulse Account could not verify the secure Pulse Pro upgrade handoff.');
      }
      updateBillingState(function(nextBillingState) {
        resolveQueryState(nextBillingState.upgradeCheckoutIntent, {
          checkout_intent_id: resolvedCheckoutIntentID,
          feature: resolvedFeature,
        });
        nextBillingState.upgradeCheckoutIntentID = resolvedCheckoutIntentID;
        nextBillingState.upgradeFeatureKey = resolvedFeature;
      });
    } catch (err) {
      updateBillingState(function(nextBillingState) {
        if (portalHandoffID) {
          failQueryState(
            nextBillingState.upgradePortalHandoff,
            null,
            err instanceof Error ? err.message : 'Failed to verify the secure Pulse Pro upgrade handoff.',
          );
        }
        failQueryState(
          nextBillingState.upgradeCheckoutIntent,
          null,
          err instanceof Error ? err.message : 'Failed to verify the secure Pulse Pro upgrade handoff.',
        );
      });
    }
  }

  async function startUpgradeCheckout(planKey: string, tier: string, billingCycle: string) {
    if (!planKey || !tier || !billingCycle) return;
    var checkoutIntentID = String(getBillingState().upgradeCheckoutIntentID || '').trim();
    if (!checkoutIntentID) {
      updateBillingState(function(nextBillingState) {
        failMutationState(
          nextBillingState.upgradeCheckout,
          'Pulse Account could not verify the secure upgrade handoff. Reopen the upgrade flow from Pulse Pro billing.',
        );
      });
      return;
    }
    updateBillingState(function(nextBillingState) {
      beginMutationState(nextBillingState.upgradeCheckout);
    });
    try {
      var data = await api.postCommercialJSON<PortalCheckoutSessionCreateResponse>('/v1/checkout/session', {
        plan_key: planKey,
        tier: tier,
        billing_cycle: billingCycle,
        checkout_intent_id: checkoutIntentID,
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
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Open Customer Portal',
      confirmPendingLabel: 'Redirecting...',
      requestSuccessMessage: 'Verification code sent. Check your email.',
      resendSuccessMessage: 'New verification code sent.',
      requestErrorMessage: 'Failed to send verification code',
      confirmErrorMessage: 'Failed to open customer portal',
      readEmailValue: function() {
        return getBillingState().flows.manage.emailValue;
      },
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
      requestSuccessMessage: 'Verification code sent. Check your email.',
      resendSuccessMessage: 'New verification code sent.',
      requestErrorMessage: 'Failed to send verification code',
      confirmErrorMessage: 'Failed to retrieve license',
      readEmailValue: function() {
        return getBillingState().flows.retrieve.emailValue;
      },
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
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Export My Data',
      confirmPendingLabel: 'Exporting...',
      requestSuccessMessage: 'Verification code sent. Check your email.',
      resendSuccessMessage: 'New verification code sent.',
      requestErrorMessage: 'Request failed',
      confirmErrorMessage: 'Export failed',
      readEmailValue: function() {
        return getBillingState().flows.export.emailValue;
      },
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
      requestLabel: 'Send Verification Code',
      requestPendingLabel: 'Sending...',
      confirmLabel: 'Delete My Data',
      confirmPendingLabel: 'Deleting...',
      requestSuccessMessage: 'Verification code sent. Check your email.',
      resendSuccessMessage: 'New verification code sent.',
      requestErrorMessage: 'Request failed',
      confirmErrorMessage: 'Deletion failed',
      readEmailValue: function() {
        return getBillingState().flows.delete.emailValue;
      },
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

  async function requestVerificationCode(flowID: PortalBillingFlowID) {
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
    updateBillingState(function(billingState) {
      beginMutationState(billingState.flows[flowID].request);
      clearFlowStatus(billingState, flowID);
    });
    try {
      await api.postCommercialJSON(flow.requestPath, { email: email });
      updateBillingState(function(billingState) {
        billingState.flows[flowID].pendingEmail = email;
        billingState.flows[flowID].step2Visible = !!flow.step2ID;
        succeedMutationState(billingState.flows[flowID].request);
        setFlowStatus(billingState, flowID, flow.requestSuccessMessage, false);
      });
    } catch (err) {
      var message = err instanceof Error ? err.message : flow.requestErrorMessage;
      updateBillingState(function(billingState) {
        failMutationState(billingState.flows[flowID].request, message);
        setFlowStatus(billingState, flowID, message, true);
      });
    }
  }

  async function resendVerificationCode(flowID: PortalBillingFlowID, event?: Event) {
    if (event) event.preventDefault();
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = getBillingState().flows[flowID].pendingEmail;
    if (!email) return;
    try {
      await api.postCommercialJSON(flow.requestPath, { email: email });
      updateBillingState(function(billingState) {
        setFlowStatus(billingState, flowID, flow.resendSuccessMessage, false);
      });
    } catch (err) {
      updateBillingState(function(billingState) {
        setFlowStatus(billingState, flowID, err instanceof Error ? err.message : flow.requestErrorMessage, true);
      });
    }
  }

  async function confirmVerificationCode(flowID: PortalBillingFlowID) {
    var flow = verificationFlows[flowID];
    if (!flow) return;
    var email = getBillingState().flows[flowID].pendingEmail;
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
      !!billingState.upgradePortalHandoffID ||
      !!billingState.upgradeCheckoutIntentID
    ) {
      void loadUpgradePricing(false);
      void resolveUpgradeCheckoutIntent(false);
    }
    renderOpenBillingPanels(getBillingState().openBillingPanelID);
    renderAllFlows();
  }

  renderBillingRuntime();
  store.subscribeBootstrap(renderBillingRuntime);
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
      updateBillingState(function(billingState) {
        updateBillingInputValue(billingState, inputKind, value);
      }, false);
    },
    updateDeleteConfirmation: function(checked) {
      updateBillingState(function(billingState) {
        updateDeleteConfirmation(billingState, checked);
      }, false);
    },
  });
}
