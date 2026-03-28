import { beginMutationState, beginQueryState, failMutationState, failQueryState, resetMutationState, resolveQueryState, succeedMutationState } from './async_state';
import { PortalAPIError } from './api';
import type { PortalAPI } from './api';
import { focusElement, getElement, renderAccountUI as renderAccountUIState } from './account_view';
import { ensurePortalAccountUIEntry } from './state';
import type { PortalStore } from './store';
import type { PortalAccessMember } from './types';

export interface AccountRuntimeDeps {
  api: PortalAPI;
  store: PortalStore;
  refreshBootstrap(): Promise<boolean>;
  showToast(message: string, isError?: boolean): void;
}

export interface AccountRuntime {
  toggleAddWorkspace(accountID: string): void;
  selectWorkspace(accountID: string, workspaceID: string): void;
  clearWorkspaceSelection(accountID: string): void;
  openBilling(accountID: string): Promise<void>;
  toggleAccess(accountID: string): void;
  ensureAccessVisible(accountID: string): void;
  inviteMember(accountID: string): Promise<void>;
  createWorkspace(accountID: string): Promise<void>;
  manageWorkspaceAction(accountID: string, tenantID: string, action: string, name: string): Promise<void>;
  removeMember(accountID: string, userID: string, email: string): Promise<void>;
  changeRole(accountID: string, userID: string, newRole: string): Promise<void>;
}

export function installAccountRuntime(deps: AccountRuntimeDeps): AccountRuntime {
  var getPortalPath = function(): string {
    return deps.store.getBootstrap().portal_path;
  };

  var revealElementIfNeeded = function(element: HTMLElement | null): void {
    if (!element) return;
    var viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
    if (!viewportHeight) return;
    var rect = element.getBoundingClientRect();
    if (rect.top >= 0 && rect.top <= viewportHeight - 72 && rect.bottom > 0) {
      return;
    }
    if (typeof element.scrollIntoView === 'function') {
      element.scrollIntoView({ block: 'start', inline: 'nearest' });
    }
  };

  var revealElementWhenReady = function(elementID: string, callback?: () => void): void {
    var reveal = function(): void {
      revealElementIfNeeded(getElement<HTMLElement>(elementID));
      if (callback) callback();
    };
    if (typeof window.requestAnimationFrame === 'function') {
      window.requestAnimationFrame(reveal);
      return;
    }
    reveal();
  };

  var refreshOrRedirect = async function(): Promise<boolean> {
    if (!await deps.refreshBootstrap()) {
      window.location.href = getPortalPath();
      return false;
    }
    return true;
  };

  var renderAccountRuntime = function(): void {
    renderAccountUIState(deps.store.getAccountState(), deps.store.getBootstrap().accounts || []);
  };

  var loadAccessRoster = async function(accountID: string): Promise<void> {
    var section = getElement<HTMLElement>('access-section-' + accountID);
    if (!section) return;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.accessVisible = true;
      beginQueryState(entry.accessQuery, []);
    });
    try {
      var members = await deps.api.listMembers(accountID) as PortalAccessMember[];
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        resolveQueryState(entry.accessQuery, Array.isArray(members) ? members : []);
      });
    } catch (error) {
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        failQueryState(entry.accessQuery, [], error instanceof Error ? error.message : 'Network error.');
      });
    }
  };

  var refreshAccountAccessSection = async function(accountID: string): Promise<boolean> {
    if (!await refreshOrRedirect()) {
      return false;
    }
    var section = getElement<HTMLElement>('access-section-' + accountID);
    if (!section) {
      return true;
    }
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.accessVisible = true;
    });
    await loadAccessRoster(accountID);
    return true;
  };

  var toggleAddWorkspace = function(accountID: string): void {
    var shouldFocus = false;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.addWorkspaceOpen = !entry.addWorkspaceOpen;
      if (entry.addWorkspaceOpen) {
        entry.accessVisible = false;
        entry.selectedWorkspaceID = '';
      }
      shouldFocus = entry.addWorkspaceOpen;
    });
    if (shouldFocus) {
      revealElementWhenReady('workspace-management-' + accountID, function() {
        focusElement('ws-name-' + accountID);
      });
    }
  };

  var selectWorkspace = function(accountID: string, workspaceID: string): void {
    var selectedWorkspaceID = '';
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.selectedWorkspaceID = entry.selectedWorkspaceID === workspaceID ? '' : workspaceID;
      if (entry.selectedWorkspaceID) {
        entry.accessVisible = false;
        entry.addWorkspaceOpen = false;
      }
      selectedWorkspaceID = entry.selectedWorkspaceID;
    });
    if (selectedWorkspaceID) {
      revealElementWhenReady('workspace-management-' + accountID);
    }
  };

  var clearWorkspaceSelection = function(accountID: string): void {
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.selectedWorkspaceID = '';
    });
  };

  var createWorkspace = async function(accountID: string): Promise<void> {
    var nameEl = getElement<HTMLInputElement>('ws-name-' + accountID);
    if (!nameEl) return;
    var name = nameEl.value.trim();
    if (!name) {
      nameEl.focus();
      return;
    }
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      beginMutationState(entry.createWorkspace);
    });
    try {
      await deps.api.createWorkspace(accountID, { display_name: name });
      if (!await refreshOrRedirect()) {
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          resetMutationState(entry.createWorkspace);
        }, { notify: false });
        return;
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.addWorkspaceOpen = false;
        succeedMutationState(entry.createWorkspace);
      });
      deps.showToast('Workspace created!');
    } catch (error) {
      var message = error instanceof Error ? error.message : 'Failed to create workspace.';
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        failMutationState(entry.createWorkspace, message);
      }, { notify: false });
      deps.showToast(message, true);
    }
  };

  var manageWorkspaceAction = async function(accountID: string, tenantID: string, action: string, name: string): Promise<void> {
    var verb = action === 'suspend' ? 'Suspend' : action === 'delete' ? 'Delete' : '';
    if (!verb) return;
    if (!window.confirm(verb + ' workspace "' + name + '"?')) return;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      beginMutationState(entry.manageWorkspace);
    });
    try {
      if (action === 'suspend') {
        await deps.api.suspendWorkspace(accountID, tenantID);
      } else {
        await deps.api.deleteWorkspace(accountID, tenantID);
      }
      if (!await refreshOrRedirect()) {
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          resetMutationState(entry.manageWorkspace);
        }, { notify: false });
        return;
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.selectedWorkspaceID = '';
        succeedMutationState(entry.manageWorkspace);
      });
      deps.showToast(verb + 'ed workspace.');
    } catch (error) {
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        failMutationState(entry.manageWorkspace, error instanceof Error ? error.message : 'Failed to ' + verb.toLowerCase() + ' workspace.');
      }, { notify: false });
      deps.showToast(error instanceof Error ? error.message : 'Failed to ' + verb.toLowerCase() + ' workspace.', true);
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

  var toggleAccess = function(accountID: string): void {
    var nextVisible = false;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.accessVisible = !entry.accessVisible;
      if (entry.accessVisible) {
        entry.selectedWorkspaceID = '';
        entry.addWorkspaceOpen = false;
      }
      nextVisible = entry.accessVisible;
    });
    if (nextVisible) {
      void loadAccessRoster(accountID);
    }
  };

  var ensureAccessVisible = function(accountID: string): void {
    var shouldLoad = false;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      if (!entry.accessVisible) {
        entry.accessVisible = true;
        entry.selectedWorkspaceID = '';
        entry.addWorkspaceOpen = false;
      }
      shouldLoad = entry.accessQuery.status === 'idle' || entry.accessQuery.status === 'error';
    });
    if (shouldLoad) {
      void loadAccessRoster(accountID);
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
      if (!await refreshAccountAccessSection(accountID)) {
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
      if (!await refreshAccountAccessSection(accountID)) {
        return;
      }
      deps.showToast('Role updated.');
    } catch (error) {
      if (error instanceof PortalAPIError && error.status === 409) {
        deps.showToast('Cannot demote last owner.', true);
        await loadAccessRoster(accountID);
        return;
      }
      deps.showToast(error instanceof Error ? error.message : 'Failed to update role.', true);
      await loadAccessRoster(accountID);
    }
  };

  var removeMember = async function(accountID: string, userID: string, email: string): Promise<void> {
    if (!window.confirm('Remove ' + email + ' from this account?')) return;
    try {
      await deps.api.removeMember(accountID, userID);
      if (!await refreshAccountAccessSection(accountID)) {
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
    selectWorkspace: selectWorkspace,
    clearWorkspaceSelection: clearWorkspaceSelection,
    openBilling: openBilling,
    toggleAccess: toggleAccess,
    ensureAccessVisible: ensureAccessVisible,
    inviteMember: inviteMember,
    createWorkspace: createWorkspace,
    manageWorkspaceAction: manageWorkspaceAction,
    removeMember: removeMember,
    changeRole: changeRole,
  };
}
