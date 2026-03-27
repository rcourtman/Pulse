import { asHTMLElement } from './account_view';
import type { AccountRuntime } from './account_runtime';

export interface AccountControllerDeps {
  runtime: AccountRuntime;
}

export function installAccountController(deps: AccountControllerDeps): void {
  document.addEventListener('click', function(event) {
    var target = asHTMLElement(event.target);
    if (!target) return;
    var actionEl = target.closest('[data-action]');
    if (!target.closest('.workspace-menu-wrap')) {
      deps.runtime.closeWorkspaceMenus();
    }
    if (!actionEl) return;
    var action = actionEl.getAttribute('data-action') || '';
    var accountID = actionEl.getAttribute('data-account-id') || '';

    switch (action) {
      case 'toggle-add-workspace':
        event.preventDefault();
        deps.runtime.toggleAddWorkspace(accountID);
        return;
      case 'open-billing':
        event.preventDefault();
        void deps.runtime.openBilling(accountID);
        return;
      case 'toggle-team':
        event.preventDefault();
        deps.runtime.toggleTeam(accountID);
        return;
      case 'invite-member':
        event.preventDefault();
        void deps.runtime.inviteMember(accountID);
        return;
      case 'create-workspace':
        event.preventDefault();
        void deps.runtime.createWorkspace(accountID);
        return;
      case 'toggle-workspace-menu':
        event.preventDefault();
        deps.runtime.toggleWorkspaceMenu(
          accountID,
          actionEl.getAttribute('data-workspace-id') || '',
        );
        return;
      case 'workspace-action':
        event.preventDefault();
        void deps.runtime.manageWorkspaceAction(
          accountID,
          actionEl.getAttribute('data-workspace-id') || '',
          actionEl.getAttribute('data-workspace-action') || '',
          actionEl.getAttribute('data-workspace-name') || '',
        );
        return;
      case 'remove-member':
        event.preventDefault();
        void deps.runtime.removeMember(
          accountID,
          actionEl.getAttribute('data-user-id') || '',
          actionEl.getAttribute('data-member-email') || '',
        );
        return;
      default:
        return;
    }
  });

  document.addEventListener('change', function(event) {
    var target = asHTMLElement(event.target) as HTMLSelectElement | null;
    if (!target || target.getAttribute('data-action') !== 'change-role') return;
    void deps.runtime.changeRole(
      target.getAttribute('data-account-id') || '',
      target.getAttribute('data-user-id') || '',
      target.value,
    );
  });
}
