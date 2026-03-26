import { PortalAPIError } from './api';
import type { PortalAPI } from './api';
import { focusElement, getElement, renderAccountUI as renderAccountUIState } from './account_view';
import { ensurePortalAccountUIEntry } from './state';
import type { PortalStore } from './store';
import type { PortalTeamMember } from './types';

export interface AccountRuntimeDeps {
  api: PortalAPI;
  store: PortalStore;
  refreshBootstrap(): Promise<boolean>;
  showToast(message: string, isError?: boolean): void;
}

export interface AccountRuntime {
  toggleAddWorkspace(accountID: string): void;
  openBilling(accountID: string): Promise<void>;
  toggleTeam(accountID: string): void;
  inviteMember(accountID: string): Promise<void>;
  createWorkspace(accountID: string): Promise<void>;
  manageWorkspace(event: Event, accountID: string, tenantID: string, state: string, name: string): Promise<void>;
  removeMember(accountID: string, userID: string, email: string): Promise<void>;
  changeRole(accountID: string, userID: string, newRole: string): Promise<void>;
}

export function installAccountRuntime(deps: AccountRuntimeDeps): AccountRuntime {
  var getPortalPath = function(): string {
    return deps.store.getBootstrap().portal_path;
  };

  var refreshOrRedirect = async function(): Promise<boolean> {
    if (!await deps.refreshBootstrap()) {
      window.location.href = getPortalPath();
      return false;
    }
    return true;
  };

  var renderAccountRuntime = function(): void {
    renderAccountUIState(deps.store.getAccountState());
  };

  var loadTeam = async function(accountID: string): Promise<void> {
    var section = getElement<HTMLElement>('team-section-' + accountID);
    if (!section) return;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.teamVisible = true;
      entry.teamLoading = true;
      entry.teamError = '';
      entry.teamMembers = [];
    });
    try {
      var members = await deps.api.listMembers(accountID) as PortalTeamMember[];
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamLoading = false;
        entry.teamError = '';
        entry.teamMembers = Array.isArray(members) ? members : [];
      });
    } catch (error) {
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamLoading = false;
        entry.teamError = error instanceof Error ? error.message : 'Network error.';
      });
    }
  };

  var refreshAccountTeamSection = async function(accountID: string): Promise<boolean> {
    if (!await refreshOrRedirect()) {
      return false;
    }
    var section = getElement<HTMLElement>('team-section-' + accountID);
    if (!section) {
      return true;
    }
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.teamVisible = true;
    });
    await loadTeam(accountID);
    return true;
  };

  var toggleAddWorkspace = function(accountID: string): void {
    var shouldFocus = false;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.addWorkspaceOpen = !entry.addWorkspaceOpen;
      shouldFocus = entry.addWorkspaceOpen;
    });
    if (shouldFocus) {
      focusElement('ws-name-' + accountID);
    }
  };

  var createWorkspace = async function(accountID: string): Promise<void> {
    var nameEl = getElement<HTMLInputElement>('ws-name-' + accountID);
    if (!nameEl) return;
    var name = nameEl.value.trim();
    if (!name) {
      nameEl.focus();
      return;
    }
    var spinner = getElement<HTMLElement>('ws-spinner-' + accountID);
    if (spinner) spinner.style.display = 'block';
    try {
      await deps.api.createWorkspace(accountID, { display_name: name });
      if (!await refreshOrRedirect()) {
        return;
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.addWorkspaceOpen = false;
      });
      deps.showToast('Workspace created!');
    } catch (error) {
      deps.showToast(error instanceof Error ? error.message : 'Failed to create workspace.', true);
    } finally {
      if (spinner) spinner.style.display = 'none';
    }
  };

  var manageWorkspace = async function(evt: Event, accountID: string, tenantID: string, state: string, name: string): Promise<void> {
    evt.stopPropagation();
    var action = state === 'active' ? 'Suspend' : 'Delete';
    if (!window.confirm(action + ' workspace "' + name + '"?')) return;
    try {
      if (state === 'active') {
        await deps.api.suspendWorkspace(accountID, tenantID);
      } else {
        await deps.api.deleteWorkspace(accountID, tenantID);
      }
      if (!await refreshOrRedirect()) {
        return;
      }
      deps.showToast(action + 'd workspace.');
    } catch (error) {
      deps.showToast(error instanceof Error ? error.message : 'Failed to ' + action.toLowerCase() + ' workspace.', true);
    }
  };

  var openBilling = async function(accountID: string): Promise<void> {
    try {
      var data = await deps.api.openBilling(accountID);
      if (data && data.url) {
        window.location.href = data.url;
      } else {
        deps.showToast('Failed to open billing portal.', true);
      }
    } catch (error) {
      deps.showToast(error instanceof Error ? error.message : 'Failed to open billing portal.', true);
    }
  };

  var toggleTeam = function(accountID: string): void {
    var nextVisible = false;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.teamVisible = !entry.teamVisible;
      nextVisible = entry.teamVisible;
    });
    if (nextVisible) {
      void loadTeam(accountID);
    }
  };

  var inviteMember = async function(accountID: string): Promise<void> {
    var emailEl = getElement<HTMLInputElement>('invite-email-' + accountID);
    var roleEl = getElement<HTMLSelectElement>('invite-role-' + accountID);
    if (!emailEl || !roleEl) return;
    var email = emailEl.value.trim();
    if (!email) {
      emailEl.focus();
      return;
    }
    try {
      await deps.api.inviteMember(accountID, { email: email, role: roleEl.value });
      emailEl.value = '';
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      deps.showToast('Member invited!');
    } catch (error) {
      if (error instanceof PortalAPIError && error.status === 409) {
        deps.showToast('Member already exists.', true);
        return;
      }
      deps.showToast(error instanceof Error ? error.message : 'Failed to invite member.', true);
    }
  };

  var changeRole = async function(accountID: string, userID: string, newRole: string): Promise<void> {
    try {
      await deps.api.updateMemberRole(accountID, userID, { role: newRole });
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      deps.showToast('Role updated.');
    } catch (error) {
      if (error instanceof PortalAPIError && error.status === 409) {
        deps.showToast('Cannot demote last owner.', true);
        await loadTeam(accountID);
        return;
      }
      deps.showToast(error instanceof Error ? error.message : 'Failed to update role.', true);
      await loadTeam(accountID);
    }
  };

  var removeMember = async function(accountID: string, userID: string, email: string): Promise<void> {
    if (!window.confirm('Remove ' + email + ' from this account?')) return;
    try {
      await deps.api.removeMember(accountID, userID);
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      deps.showToast('Member removed.');
    } catch (error) {
      if (error instanceof PortalAPIError && error.status === 409) {
        deps.showToast('Cannot remove last owner.', true);
        return;
      }
      deps.showToast(error instanceof Error ? error.message : 'Failed to remove member.', true);
    }
  };

  deps.store.subscribeAccount(renderAccountRuntime);
  deps.store.subscribeBootstrap(renderAccountRuntime);

  return {
    toggleAddWorkspace: toggleAddWorkspace,
    openBilling: openBilling,
    toggleTeam: toggleTeam,
    inviteMember: inviteMember,
    createWorkspace: createWorkspace,
    manageWorkspace: manageWorkspace,
    removeMember: removeMember,
    changeRole: changeRole,
  };
}
