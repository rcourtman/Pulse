import { asHTMLElement } from './account_view';
import type { AccountRuntime } from './account_runtime';

export interface AccountControllerDeps {
  runtime: AccountRuntime;
  setShellSection: (section: 'overview' | 'workspaces' | 'access' | 'billing' | 'support') => void;
}

export function installAccountController(deps: AccountControllerDeps): void {
  document.addEventListener('click', function(event) {
    var target = asHTMLElement(event.target);
    if (!target) return;
    var actionEl = target.closest('[data-action]');
    if (!actionEl) return;
    var action = actionEl.getAttribute('data-action') || '';
    var accountID = actionEl.getAttribute('data-account-id') || '';

    switch (action) {
      case 'toggle-add-workspace':
        event.preventDefault();
        deps.setShellSection('workspaces');
        deps.runtime.toggleAddWorkspace(accountID);
        return;
      case 'open-billing':
        event.preventDefault();
        void deps.runtime.openBilling(accountID);
        return;
      case 'show-access':
        event.preventDefault();
        deps.setShellSection('access');
        deps.runtime.ensureAccessVisible(accountID);
        return;
      case 'set-access-job':
        event.preventDefault();
        deps.setShellSection('access');
        void deps.runtime.setAccessJob(accountID, (actionEl.getAttribute('data-access-job') || '') as 'invite' | 'change_role' | 'remove');
        return;
      case 'clear-access-job':
        event.preventDefault();
        deps.setShellSection('access');
        deps.runtime.clearAccessJob(accountID);
        return;
      case 'invite-member':
        event.preventDefault();
        void deps.runtime.inviteMember(accountID);
        return;
      case 'create-workspace':
        event.preventDefault();
        deps.setShellSection('workspaces');
        void deps.runtime.createWorkspace(accountID);
        return;
      case 'select-workspace':
        event.preventDefault();
        deps.setShellSection('workspaces');
        deps.runtime.selectWorkspace(
          accountID,
          actionEl.getAttribute('data-workspace-id') || '',
        );
        return;
      case 'clear-workspace-selection':
        event.preventDefault();
        deps.setShellSection('workspaces');
        deps.runtime.clearWorkspaceSelection(accountID);
        return;
      case 'workspace-action':
        event.preventDefault();
        deps.setShellSection('workspaces');
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
