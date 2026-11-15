import { beginMutationState, beginQueryState, failMutationState, failQueryState, resetMutationState, resolveQueryState, succeedMutationState } from './async_state';
import { PortalAPIError } from './api';
import type { PortalAPI } from './api';
import { focusElement, getElement, renderAccountUI as renderAccountUIState } from './account_view';
import { ensurePortalAccountUIEntry } from './state';
import type { PortalStore } from './store';
import type { PortalAccessJob, PortalAccessMember, PortalWorkspaceMutationState } from './types';

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
  ensureAccessVisible(accountID: string): void;
  setAccessJob(accountID: string, job: PortalAccessJob): Promise<void>;
  clearAccessJob(accountID: string): void;
  inviteMember(accountID: string): Promise<void>;
  createWorkspace(accountID: string): Promise<void>;
  refreshWorkspaceState(accountID: string): Promise<void>;
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
    var headerHeight = document.querySelector('header')?.getBoundingClientRect().height || 0;
    if (rect.top >= headerHeight + 12 && rect.top <= viewportHeight - 72 && rect.bottom > 0) {
      return;
    }
    if (typeof element.scrollIntoView === 'function') {
      element.style.scrollMarginTop = (headerHeight + 12) + 'px';
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

  var accountUsesClientLanguage = function(accountID: string): boolean {
    var bootstrap = deps.store.getBootstrap();
    var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
    for (var i = 0; i < accounts.length; i += 1) {
      if (accounts[i].id === accountID) {
        return accounts[i].kind === 'msp';
      }
    }
    return false;
  };

  var findWorkspaceIDByName = function(accountID: string, displayName: string): string {
    var bootstrap = deps.store.getBootstrap();
    var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
    for (var i = 0; i < accounts.length; i += 1) {
      if (accounts[i].id !== accountID) continue;
      var workspaces = Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces : [];
      for (var j = 0; j < workspaces.length; j += 1) {
        if (workspaces[j].display_name === displayName) {
          return workspaces[j].id;
        }
      }
    }
    return '';
  };

  var renderAccountRuntime = function(): void {
    var bootstrap = deps.store.getBootstrap();
    renderAccountUIState(deps.store.getAccountState(), bootstrap.accounts || [], bootstrap.account_api_base_path || '');
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
        entry.activeAccessJob = '';
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
        entry.activeAccessJob = '';
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

  // Maps the control plane's workspace_limit_reached payload to copy a
  // provider can act on; returns '' for any other error payload.
  function workspaceLimitMessageFromPayload(payload: unknown, clientLanguage: boolean): string {
    if (!payload || typeof payload !== 'object') return '';
    var body = payload as { error?: unknown; current?: unknown; limit?: unknown };
    if (body.error !== 'workspace_limit_reached') return '';
    var entity = clientLanguage ? 'client workspaces' : 'workspaces';
    var counts = typeof body.current === 'number' && typeof body.limit === 'number' && body.limit > 0
      ? ' (' + body.current + ' of ' + body.limit + ' in use)'
      : '';
    return 'Your plan limit for ' + entity + ' is reached' + counts + '. Remove a ' +
      (clientLanguage ? 'client' : 'workspace') + ' or move to a bigger plan to add more.';
  }

  var createWorkspace = async function(accountID: string): Promise<void> {
    var currentEntry = deps.store.getAccountState().byAccountID[accountID];
    if (currentEntry?.createWorkspace.pending || currentEntry?.manageWorkspace.pending || currentEntry?.manageWorkspace.recoveryRequired) return;
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
      var created = await deps.api.createWorkspace(accountID, { display_name: name });
      if (!await refreshOrRedirect()) {
        deps.store.updateAccountState(function(accountState) {
          var entry = ensurePortalAccountUIEntry(accountState, accountID);
          resetMutationState(entry.createWorkspace);
        });
        return;
      }
      var createdWorkspaceID = created && typeof created.id === 'string' ? created.id : '';
      if (!createdWorkspaceID) {
        createdWorkspaceID = findWorkspaceIDByName(accountID, name);
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.addWorkspaceOpen = false;
        entry.selectedWorkspaceID = createdWorkspaceID;
        entry.accessVisible = false;
        entry.activeAccessJob = '';
        succeedMutationState(entry.createWorkspace);
      });
      revealElementWhenReady('workspace-management-' + accountID);
      deps.showToast(accountUsesClientLanguage(accountID) ? 'Client added. Finish onboarding next.' : 'Workspace created. Finish setup next.');
    } catch (error) {
      var clientLanguage = accountUsesClientLanguage(accountID);
      var message = error instanceof Error ? error.message : (clientLanguage ? 'Failed to add client.' : 'Failed to create workspace.');
      if (error instanceof PortalAPIError) {
        var limitMessage = workspaceLimitMessageFromPayload(error.payload, clientLanguage);
        if (limitMessage) message = limitMessage;
      }
      deps.store.updateAccountState(function(accountState) {
        var entry = ensurePortalAccountUIEntry(accountState, accountID);
        failMutationState(entry.createWorkspace, message);
      });
      deps.showToast(message, true);
    }
  };

  function findAccount(accountID: string) {
    return deps.store.getBootstrap().accounts.find(function(account) { return account.id === accountID; });
  }

  function currentWorkspaceAction(accountID: string, workspaceID: string): string {
    var account = findAccount(accountID);
    var workspace = account?.workspaces.find(function(item) { return item.id === workspaceID; });
    if (!account?.can_manage || !workspace) return '';
    if (workspace.state === 'active') return 'suspend';
    if (workspace.state === 'suspended' || workspace.state === 'failed') return 'delete';
    return '';
  }

  // Do not steal focus from someone who moved on while the request was pending.
  function trackWorkspaceFocus(accountID: string) {
    var origin = document.activeElement;
    var moved = false;
    var onFocus = function(event: FocusEvent) { if (event.target !== origin) moved = true; };
    document.addEventListener('focusin', onFocus);
    return function(elementID: string) {
      document.removeEventListener('focusin', onFocus);
      if (elementID && !moved && deps.store.getShellState().activeSection === 'workspaces') {
        revealElementIfNeeded(getElement<HTMLElement>('workspace-recovery-' + accountID));
        focusElement(elementID);
      }
    };
  }

  function isCurrentWorkspaceRequest(accountID: string, request: PortalWorkspaceMutationState): boolean {
    return deps.store.getBootstrap().authenticated &&
      deps.store.getAccountState().byAccountID[accountID]?.manageWorkspace === request;
  }

  function recordWorkspaceRead(accountID: string, request: PortalWorkspaceMutationState): boolean {
    var account = findAccount(accountID);
    if (!account) return false;
    var workspace = account.workspaces.find(function(item) { return item.id === request.workspaceID; });
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      request.pending = false;
      request.refreshing = false;
      request.recoveryRequired = false;
      request.error = '';
      request.notice = workspace
        ? 'Latest state for "' + workspace.display_name + '": ' + workspace.state + '. No action was repeated.'
        : '"' + request.workspaceName + '" is no longer listed in this account. No action was repeated.';
      // A result for A must not close the panel the reader opened for B.
      if (!workspace && entry.selectedWorkspaceID === request.workspaceID) entry.selectedWorkspaceID = '';
    });
    return true;
  }

  var refreshWorkspaceState = async function(accountID: string): Promise<void> {
    var request = deps.store.getAccountState().byAccountID[accountID]?.manageWorkspace;
    if (!request?.recoveryRequired || request.pending || request.refreshing) return;
    var finishFocus = trackWorkspaceFocus(accountID);
    deps.store.updateAccountState(function() { request.refreshing = true; });
    var refreshed = false;
    try { refreshed = await deps.refreshBootstrap(); } catch { /* Keep the last loaded facts. */ }
    if (isCurrentWorkspaceRequest(accountID, request)) {
      if (!refreshed || !recordWorkspaceRead(accountID, request)) {
        deps.store.updateAccountState(function() {
          request.refreshing = false;
          request.error = 'Could not refresh the current state. The last loaded facts may be out of date. No action was repeated.';
        });
      }
      finishFocus(request.recoveryRequired ? 'workspace-recovery-refresh-' + accountID : 'workspace-recovery-' + accountID);
    } else { finishFocus(''); }
  };

  var manageWorkspaceAction = async function(accountID: string, tenantID: string, action: string, name: string): Promise<void> {
    var entry = deps.store.getAccountState().byAccountID[accountID];
    if (action !== 'suspend' && action !== 'delete') return;
    if (!entry || entry.manageWorkspace.pending || entry.manageWorkspace.recoveryRequired) return;
    if (action !== currentWorkspaceAction(accountID, tenantID)) return;
    var workspace = findAccount(accountID)?.workspaces.find(function(item) { return item.id === tenantID; });
    if (!workspace) return;
    name = workspace.display_name;
    var verb = action === 'suspend' ? 'Suspend' : 'Delete';
    var entityName = accountUsesClientLanguage(accountID) ? 'client' : 'workspace';
    if (!window.confirm(verb + ' ' + entityName + ' "' + name + '"?')) return;
    // Confirmation is not authority: the current identity/state still decides.
    if (deps.store.getAccountState().byAccountID[accountID] !== entry || action !== currentWorkspaceAction(accountID, tenantID) || entry.manageWorkspace.pending || entry.manageWorkspace.recoveryRequired) return;
    var request = entry.manageWorkspace;
    var finishFocus = trackWorkspaceFocus(accountID);
    deps.store.updateAccountState(function() {
      beginMutationState(request);
      request.workspaceID = tenantID;
      request.workspaceName = name;
      request.action = action as 'suspend' | 'delete';
      request.notice = '';
      request.outcome = 'unknown';
    });
    var accepted = false;
    try {
      if (action === 'suspend') await deps.api.suspendWorkspace(accountID, tenantID);
      else await deps.api.deleteWorkspace(accountID, tenantID);
      accepted = true;
      if (!isCurrentWorkspaceRequest(accountID, request)) return;
      request.outcome = 'accepted';
      var refreshed = await deps.refreshBootstrap();
      if (!isCurrentWorkspaceRequest(accountID, request)) return;
      if (!refreshed || !recordWorkspaceRead(accountID, request)) throw new Error('Could not load the latest account state.');
      var current = findAccount(accountID)?.workspaces.find(function(item) { return item.id === tenantID; });
      if (action === 'suspend' ? current?.state === 'suspended' : !current) {
        deps.store.updateAccountState(function(state) {
          var currentEntry = ensurePortalAccountUIEntry(state, accountID);
          if (currentEntry.selectedWorkspaceID === tenantID) currentEntry.selectedWorkspaceID = '';
        });
        deps.showToast((action === 'suspend' ? 'Suspended' : 'Deleted') + ' ' + entityName + '.');
      }
    } catch (error) {
      if (!isCurrentWorkspaceRequest(accountID, request)) return;
      deps.store.updateAccountState(function() {
        request.pending = false;
        request.outcome = accepted ? 'accepted' : error instanceof PortalAPIError && error.status >= 400 && error.status < 500 ? 'rejected' : 'unknown';
        request.recoveryRequired = true;
        request.error = error instanceof Error ? error.message : 'The current result could not be confirmed.';
      });
    } finally {
      finishFocus(isCurrentWorkspaceRequest(accountID, request)
        ? (request.recoveryRequired ? 'workspace-recovery-refresh-' : 'workspace-recovery-') + accountID : '');
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

  var ensureAccessVisible = function(accountID: string): void {
    var shouldLoad = false;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      if (!entry.accessVisible) {
        entry.accessVisible = true;
      }
      entry.selectedWorkspaceID = '';
      entry.addWorkspaceOpen = false;
      shouldLoad = entry.accessQuery.status === 'idle' || entry.accessQuery.status === 'error';
    });
    if (shouldLoad) {
      void loadAccessRoster(accountID);
    }
  };

  var setAccessJob = async function(accountID: string, job: PortalAccessJob): Promise<void> {
    var nextJob = '' as PortalAccessJob;
    var shouldLoad = false;
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.accessVisible = true;
      entry.selectedWorkspaceID = '';
      entry.addWorkspaceOpen = false;
      entry.activeAccessJob = entry.activeAccessJob === job ? '' : job;
      nextJob = entry.activeAccessJob;
      shouldLoad = entry.accessQuery.status === 'idle' || entry.accessQuery.status === 'error';
    });
    if (shouldLoad) {
      await loadAccessRoster(accountID);
    }
    if (nextJob) {
      revealElementWhenReady('access-detail-' + accountID, function() {
        if (nextJob === 'invite') {
          focusElement('invite-email-' + accountID);
        }
      });
    }
  };

  var clearAccessJob = function(accountID: string): void {
    deps.store.updateAccountState(function(accountState) {
      var entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.activeAccessJob = '';
    });
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
      var result = await deps.api.inviteMember(accountID, { email: email, role: roleEl.value });
      emailEl.value = '';
      if (!await refreshAccountAccessSection(accountID)) {
        return;
      }
      deps.showToast(result && result.state === 'active' ? 'Member added.' : 'Invitation saved.');
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
    ensureAccessVisible: ensureAccessVisible,
    setAccessJob: setAccessJob,
    clearAccessJob: clearAccessJob,
    inviteMember: inviteMember,
    createWorkspace: createWorkspace,
    manageWorkspaceAction: manageWorkspaceAction,
    refreshWorkspaceState: refreshWorkspaceState,
    removeMember: removeMember,
    changeRole: changeRole,
  };
}
