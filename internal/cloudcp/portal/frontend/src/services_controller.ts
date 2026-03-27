import { asHTMLElement } from './services_view';

export interface ServicesControllerDeps {
  setShellSection: (section: 'overview' | 'workspaces' | 'team' | 'services' | 'support') => void;
  toggleServicePanel: (panelID: string) => void;
  focusElement: (id: string) => void;
  requestVerificationCode: (flowID: 'manage' | 'retrieve' | 'export' | 'delete') => void;
  resendVerificationCode: (flowID: 'manage' | 'export' | 'delete', event?: Event) => void;
  confirmVerificationCode: (flowID: 'manage' | 'retrieve' | 'export' | 'delete') => void;
  copyRetrievedLicense: () => void;
  submitRefund: () => void;
  updateInputValue: (inputKind: string, value: string) => void;
  updateDeleteConfirmation: (checked: boolean) => void;
}

export function installServicesController(deps: ServicesControllerDeps): void {
  document.addEventListener('click', function(event) {
    var target = asHTMLElement(event.target)?.closest('[data-account-service-action]');
    if (!target) return;
    var action = target.getAttribute('data-account-service-action') || '';
    var panelID = target.getAttribute('data-account-service-panel') || '';
    var focusID = target.getAttribute('data-account-service-focus') || '';

    switch (action) {
      case 'open-service-panel':
        event.preventDefault();
        deps.setShellSection('services');
        deps.toggleServicePanel(panelID);
        deps.focusElement(focusID);
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
    var inputKind = target.getAttribute('data-account-service-input') || '';
    if (!inputKind) return;
    deps.updateInputValue(inputKind, target.value);
  });

  document.addEventListener('change', function(event) {
    var target = asHTMLElement(event.target) as HTMLInputElement | null;
    if (!target || target.id !== 'data-delete-confirm-check') return;
    deps.updateDeleteConfirmation(!!target.checked);
  });
}
