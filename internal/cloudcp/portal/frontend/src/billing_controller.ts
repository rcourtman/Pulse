import { asHTMLElement } from './billing_view';

export interface BillingControllerDeps {
  setShellSection: (section: 'overview' | 'workspaces' | 'access' | 'billing' | 'support') => void;
  toggleBillingPanel: (panelID: string) => void;
  clearBillingPanel: () => void;
  focusElement: (id: string) => void;
  requestVerificationCode: (flowID: 'manage' | 'retrieve' | 'export' | 'delete') => void;
  resendVerificationCode: (flowID: 'manage' | 'export' | 'delete', event?: Event) => void;
  confirmVerificationCode: (flowID: 'manage' | 'retrieve' | 'export' | 'delete') => void;
  copyRetrievedLicense: () => void;
  submitRefund: () => void;
  reloadUpgradePricing: () => void;
  startUpgradeCheckout: (planKey: string, tier: string, billingCycle: string) => void;
  updateInputValue: (inputKind: string, value: string) => void;
  updateDeleteConfirmation: (checked: boolean) => void;
}

export function installBillingController(deps: BillingControllerDeps): void {
  function revealBillingPanelWhenReady(panelID: string): void {
    var reveal = function(): void {
      var panel = document.getElementById(panelID) as HTMLElement | null;
      if (!panel || panel.hidden || typeof panel.scrollIntoView !== 'function') return;
      panel.scrollIntoView({ block: 'start', inline: 'nearest' });
    };
    if (typeof window.requestAnimationFrame === 'function') {
      window.requestAnimationFrame(function() {
        window.requestAnimationFrame(reveal);
      });
      return;
    }
    reveal();
  }

  document.addEventListener('click', function(event) {
    var target = asHTMLElement(event.target)?.closest('[data-account-billing-action]');
    if (!target) return;
    var action = target.getAttribute('data-account-billing-action') || '';
    var panelID = target.getAttribute('data-account-billing-panel') || '';
    var focusID = target.getAttribute('data-account-billing-focus') || '';

    switch (action) {
      case 'open-billing-panel':
        event.preventDefault();
        deps.setShellSection('billing');
        deps.toggleBillingPanel(panelID);
        var targetElement = target as HTMLElement;
        if (typeof targetElement.blur === 'function') {
          targetElement.blur();
        }
        deps.focusElement(focusID);
        if (panelID) {
          revealBillingPanelWhenReady(panelID);
        }
        return;
      case 'clear-billing-panel':
        event.preventDefault();
        deps.setShellSection('billing');
        deps.clearBillingPanel();
        return;
      case 'manage-inline-request':
        event.preventDefault();
        deps.requestVerificationCode('manage');
        return;
      case 'manage-inline-resend':
        deps.resendVerificationCode('manage', event);
        return;
      case 'manage-inline-confirm':
        event.preventDefault();
        deps.confirmVerificationCode('manage');
        return;
      case 'retrieve-inline-request':
        event.preventDefault();
        deps.requestVerificationCode('retrieve');
        return;
      case 'retrieve-inline-confirm':
        event.preventDefault();
        deps.confirmVerificationCode('retrieve');
        return;
      case 'retrieve-inline-copy':
        event.preventDefault();
        deps.copyRetrievedLicense();
        return;
      case 'refund-inline-submit':
        event.preventDefault();
        deps.submitRefund();
        return;
      case 'upgrade-reload-pricing':
        event.preventDefault();
        deps.reloadUpgradePricing();
        return;
      case 'upgrade-start-checkout':
        event.preventDefault();
        deps.startUpgradeCheckout(
          target.getAttribute('data-upgrade-plan-key') || '',
          target.getAttribute('data-upgrade-tier') || '',
          target.getAttribute('data-upgrade-billing-cycle') || '',
        );
        return;
      case 'data-export-request':
        event.preventDefault();
        deps.requestVerificationCode('export');
        return;
      case 'data-export-resend':
        deps.resendVerificationCode('export', event);
        return;
      case 'data-export-confirm':
        event.preventDefault();
        deps.confirmVerificationCode('export');
        return;
      case 'data-delete-request':
        event.preventDefault();
        deps.requestVerificationCode('delete');
        return;
      case 'data-delete-resend':
        deps.resendVerificationCode('delete', event);
        return;
      case 'data-delete-confirm':
        event.preventDefault();
        deps.confirmVerificationCode('delete');
        return;
      default:
        return;
    }
  });

  document.addEventListener('input', function(event) {
    var target = asHTMLElement(event.target) as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null;
    if (!target) return;
    var inputKind = target.getAttribute('data-account-billing-input') || '';
    if (!inputKind) return;
    deps.updateInputValue(inputKind, target.value);
  });

  document.addEventListener('change', function(event) {
    var target = asHTMLElement(event.target) as HTMLInputElement | null;
    if (!target || target.id !== 'data-delete-confirm-check') return;
    deps.updateDeleteConfirmation(!!target.checked);
  });
}
