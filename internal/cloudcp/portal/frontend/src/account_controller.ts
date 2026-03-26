import { asHTMLElement, focusElement, getElement, renderAccountUI as renderAccountUIState } from './account_view';
import { ensurePortalAccountUIEntry } from './state';
import type { PortalStore } from './store';
import type { PortalTeamMember } from './types';

export interface AccountControllerDeps {
  store: PortalStore;
  refreshBootstrap(): Promise<boolean>;
  showToast(message: string, isError?: boolean): void;
}

export function installAccountController(deps: AccountControllerDeps): void {
  const getAccountAPIBasePath = (): string => deps.store.getBootstrap().account_api_base_path;
  const getPortalAPIBasePath = (): string => deps.store.getBootstrap().portal_api_base_path;
  const getPortalPath = (): string => deps.store.getBootstrap().portal_path;

  const refreshOrRedirect = async (): Promise<boolean> => {
    if (!await deps.refreshBootstrap()) {
      window.location.href = getPortalPath();
      return false;
    }
    return true;
  };

  const renderAccountRuntime = (): void => {
    renderAccountUIState(deps.store.getAccountState());
  };

  const loadTeam = async (accountID: string): Promise<void> => {
    const section = getElement<HTMLElement>('team-section-' + accountID);
    if (!section) return;
    deps.store.updateAccountState((accountState) => {
      const entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.teamVisible = true;
      entry.teamLoading = true;
      entry.teamError = '';
      entry.teamMembers = [];
    });
    try {
      const r = await fetch(getAccountAPIBasePath() + '/' + encodeURIComponent(accountID) + '/members');
      if (!r.ok) {
        deps.store.updateAccountState((accountState) => {
          const entry = ensurePortalAccountUIEntry(accountState, accountID);
          entry.teamLoading = false;
          entry.teamError = 'Failed to load team.';
        });
        return;
      }
      const members = await r.json() as PortalTeamMember[];
      deps.store.updateAccountState((accountState) => {
        const entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamLoading = false;
        entry.teamError = '';
        entry.teamMembers = Array.isArray(members) ? members : [];
      });
    } catch {
      deps.store.updateAccountState((accountState) => {
        const entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.teamLoading = false;
        entry.teamError = 'Network error.';
      });
    }
  };

  const refreshAccountTeamSection = async (accountID: string): Promise<boolean> => {
    if (!await refreshOrRedirect()) {
      return false;
    }
    const section = getElement<HTMLElement>('team-section-' + accountID);
    if (!section) {
      return true;
    }
    deps.store.updateAccountState((accountState) => {
      const entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.teamVisible = true;
    });
    await loadTeam(accountID);
    return true;
  };

  const toggleAddWorkspace = (accountID: string): void => {
    let shouldFocus = false;
    deps.store.updateAccountState((accountState) => {
      const entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.addWorkspaceOpen = !entry.addWorkspaceOpen;
      shouldFocus = entry.addWorkspaceOpen;
    });
    if (shouldFocus) {
      focusElement('ws-name-' + accountID);
    }
  };

  const createWorkspace = async (accountID: string): Promise<void> => {
    const nameEl = getElement<HTMLInputElement>('ws-name-' + accountID);
    if (!nameEl) return;
    const name = nameEl.value.trim();
    if (!name) {
      nameEl.focus();
      return;
    }
    const spinner = getElement<HTMLElement>('ws-spinner-' + accountID);
    if (spinner) spinner.style.display = 'block';
    try {
      const resp = await fetch(getAccountAPIBasePath() + '/' + accountID + '/tenants', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ display_name: name }),
      });
      if (!resp.ok) {
        const err = await resp.json().catch(() => ({}));
        deps.showToast((err && err.error) || 'Failed to create workspace', true);
        return;
      }
      if (!await refreshOrRedirect()) {
        return;
      }
      deps.store.updateAccountState((accountState) => {
        const entry = ensurePortalAccountUIEntry(accountState, accountID);
        entry.addWorkspaceOpen = false;
      });
      deps.showToast('Workspace created!');
    } catch {
      deps.showToast('Network error. Please try again.', true);
    } finally {
      if (spinner) spinner.style.display = 'none';
    }
  };

  const suspendOrDelete = async (evt: Event, accountID: string, tenantID: string, state: string, name: string): Promise<void> => {
    evt.stopPropagation();
    const action = state === 'active' ? 'Suspend' : 'Delete';
    if (!window.confirm(action + ' workspace "' + name + '"?')) return;
    const method = state === 'active' ? 'PATCH' : 'DELETE';
    const body = state === 'active' ? JSON.stringify({ state: 'suspended' }) : undefined;
    try {
      const response = await fetch(getAccountAPIBasePath() + '/' + accountID + '/tenants/' + tenantID, {
        method,
        headers: body ? { 'Content-Type': 'application/json' } : {},
        body,
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

  const openBilling = async (accountID: string): Promise<void> => {
    try {
      const r = await fetch(getPortalAPIBasePath() + '/billing?account_id=' + encodeURIComponent(accountID), { method: 'POST' });
      if (!r.ok) {
        const err = await r.json().catch(() => ({}));
        deps.showToast((err && err.error) || 'Failed to open billing portal.', true);
        return;
      }
      const data = await r.json();
      if (data && data.url) {
        window.location.href = data.url;
      } else {
        deps.showToast('Failed to open billing portal.', true);
      }
    } catch {
      deps.showToast('Network error.', true);
    }
  };

  const toggleTeam = (accountID: string): void => {
    let nextVisible = false;
    deps.store.updateAccountState((accountState) => {
      const entry = ensurePortalAccountUIEntry(accountState, accountID);
      entry.teamVisible = !entry.teamVisible;
      nextVisible = entry.teamVisible;
    });
    if (nextVisible) {
      void loadTeam(accountID);
    }
  };

  const inviteMember = async (accountID: string): Promise<void> => {
    const emailEl = getElement<HTMLInputElement>('invite-email-' + accountID);
    const roleEl = getElement<HTMLSelectElement>('invite-role-' + accountID);
    if (!emailEl || !roleEl) return;
    const email = emailEl.value.trim();
    if (!email) {
      emailEl.focus();
      return;
    }
    try {
      const r = await fetch(getAccountAPIBasePath() + '/' + encodeURIComponent(accountID) + '/members', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, role: roleEl.value }),
      });
      if (r.status === 409) {
        deps.showToast('Member already exists.', true);
        return;
      }
      if (!r.ok) {
        const err = await r.text();
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

  const changeRole = async (accountID: string, userID: string, newRole: string): Promise<void> => {
    try {
      const r = await fetch(getAccountAPIBasePath() + '/' + encodeURIComponent(accountID) + '/members/' + encodeURIComponent(userID), {
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

  const removeMember = async (accountID: string, userID: string, email: string): Promise<void> => {
    if (!window.confirm('Remove ' + email + ' from this account?')) return;
    try {
      const r = await fetch(getAccountAPIBasePath() + '/' + encodeURIComponent(accountID) + '/members/' + encodeURIComponent(userID), {
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

  document.addEventListener('click', function(event) {
    const actionEl = asHTMLElement(event.target)?.closest('[data-action]');
    if (!actionEl) return;
    const action = actionEl.getAttribute('data-action') || '';
    const accountID = actionEl.getAttribute('data-account-id') || '';

    switch (action) {
      case 'toggle-add-workspace':
        event.preventDefault();
        toggleAddWorkspace(accountID);
        return;
      case 'open-billing':
        event.preventDefault();
        void openBilling(accountID);
        return;
      case 'toggle-team':
        event.preventDefault();
        toggleTeam(accountID);
        return;
      case 'invite-member':
        event.preventDefault();
        void inviteMember(accountID);
        return;
      case 'create-workspace':
        event.preventDefault();
        void createWorkspace(accountID);
        return;
      case 'workspace-manage':
        event.preventDefault();
        void suspendOrDelete(
          event,
          accountID,
          actionEl.getAttribute('data-workspace-id') || '',
          actionEl.getAttribute('data-workspace-state') || '',
          actionEl.getAttribute('data-workspace-name') || '',
        );
        return;
      case 'remove-member':
        event.preventDefault();
        void removeMember(
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
    const target = asHTMLElement(event.target) as HTMLSelectElement | null;
    if (!target || target.getAttribute('data-action') !== 'change-role') return;
    void changeRole(
      target.getAttribute('data-account-id') || '',
      target.getAttribute('data-user-id') || '',
      target.value,
    );
  });

  deps.store.subscribeAccount(renderAccountRuntime);
  deps.store.subscribeBootstrap(renderAccountRuntime);
}
