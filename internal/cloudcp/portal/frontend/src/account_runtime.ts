import { focusElement, getElement, renderAccountUI as renderAccountUIState } from './account_view';
import { ensurePortalAccountUIEntry } from './state';
import type { PortalStore } from './store';
import type { PortalTeamMember } from './types';

export interface AccountRuntimeDeps {
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
  var getAccountAPIBasePath = function(): string {
    return deps.store.getBootstrap().account_api_base_path;
  };

  var getPortalAPIBasePath = function(): string {
    return deps.store.getBootstrap().portal_api_base_path;
  };

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
      var r = await fetch(getAccountAPIBasePath() + '/' + encodeURIComponent(accountID) + '/members');
      if (!r.ok) {
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          entry.teamLoading = false;
          entry.teamError = 'Failed to load team.';
        });
        return;
      }
      var members = await r.json() as PortalTeamMember[];
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamLoading = false;
        entry.teamError = '';
        entry.teamMembers = Array.isArray(members) ? members : [];
      });
    } catch {
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamLoading = false;
        entry.teamError = 'Network error.';
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
      var resp = await fetch(getAccountAPIBasePath() + '/' + accountID + '/tenants', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ display_name: name }),
      });
      if (!resp.ok) {
        var err = await resp.json().catch(function() { return {}; });
        deps.showToast((err && err.error) || 'Failed to create workspace', true);
        return;
      }
      if (!await refreshOrRedirect()) {
        return;
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.addWorkspaceOpen = false;
      });
      deps.showToast('Workspace created!');
    } catch {
      deps.showToast('Network error. Please try again.', true);
    } finally {
      if (spinner) spinner.style.display = 'none';
    }
  };

  var manageWorkspace = async function(evt: Event, accountID: string, tenantID: string, state: string, name: string): Promise<void> {
    evt.stopPropagation();
    var action = state === 'active' ? 'Suspend' : 'Delete';
    if (!window.confirm(action + ' workspace "' + name + '"?')) return;
    var method = state === 'active' ? 'PATCH' : 'DELETE';
    var body = state === 'active' ? JSON.stringify({ state: 'suspended' }) : undefined;
    try {
      var response = await fetch(getAccountAPIBasePath() + '/' + accountID + '/tenants/' + tenantID, {
        method: method,
        headers: body ? { 'Content-Type': 'application/json' } : {},
        body: body,
      });
      if (!response.ok) {
        deps.showToast('Failed to ' + action.toLowerCase() + ' workspace.', true);
        return;
      }
      if (!await refreshOrRedirect()) {
        return;
      }
      deps.showToast(action + 'd workspace.');
    } catch {
      deps.showToast('Network error.', true);
    }
  };

  var openBilling = async function(accountID: string): Promise<void> {
    try {
      var r = await fetch(getPortalAPIBasePath() + '/billing?account_id=' + encodeURIComponent(accountID), { method: 'POST' });
      if (!r.ok) {
        var err = await r.json().catch(function() { return {}; });
        deps.showToast((err && err.error) || 'Failed to open billing portal.', true);
        return;
      }
      var data = await r.json();
      if (data && data.url) {
        window.location.href = data.url;
      } else {
        deps.showToast('Failed to open billing portal.', true);
      }
    } catch {
      deps.showToast('Network error.', true);
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
      var r = await fetch(getAccountAPIBasePath() + '/' + encodeURIComponent(accountID) + '/members', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email, role: roleEl.value }),
      });
      if (r.status === 409) {
        deps.showToast('Member already exists.', true);
        return;
      }
      if (!r.ok) {
        var err = await r.text();
        deps.showToast(err || 'Failed to invite member.', true);
        return;
      }
      emailEl.value = '';
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      deps.showToast('Member invited!');
    } catch {
      deps.showToast('Network error.', true);
    }
  };

  var changeRole = async function(accountID: string, userID: string, newRole: string): Promise<void> {
    try {
      var r = await fetch(getAccountAPIBasePath() + '/' + encodeURIComponent(accountID) + '/members/' + encodeURIComponent(userID), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ role: newRole }),
      });
      if (r.status === 409) {
        deps.showToast('Cannot demote last owner.', true);
        await loadTeam(accountID);
        return;
      }
      if (!r.ok) {
        deps.showToast('Failed to update role.', true);
        await loadTeam(accountID);
        return;
      }
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      deps.showToast('Role updated.');
    } catch {
      deps.showToast('Network error.', true);
      await loadTeam(accountID);
    }
  };

  var removeMember = async function(accountID: string, userID: string, email: string): Promise<void> {
    if (!window.confirm('Remove ' + email + ' from this account?')) return;
    try {
      var r = await fetch(getAccountAPIBasePath() + '/' + encodeURIComponent(accountID) + '/members/' + encodeURIComponent(userID), {
        method: 'DELETE',
      });
      if (r.status === 409) {
        deps.showToast('Cannot remove last owner.', true);
        return;
      }
      if (!r.ok) {
        deps.showToast('Failed to remove member.', true);
        return;
      }
      if (!await refreshAccountTeamSection(accountID)) {
        return;
      }
      deps.showToast('Member removed.');
    } catch {
      deps.showToast('Network error.', true);
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
