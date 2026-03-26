import { ensurePortalAccountUIEntry } from './state';
import type { PortalStore } from './store';
import type { PortalTeamMember } from './types';

export interface AccountControllerDeps {
  store: PortalStore;
  refreshBootstrap(): Promise<boolean>;
  showToast(message: string, isError?: boolean): void;
}

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

function getElement<T extends HTMLElement = HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null;
}

function asHTMLElement(target: EventTarget | null): HTMLElement | null {
  return target instanceof HTMLElement ? target : null;
}

function setTbodyMessage(tbody: HTMLElement, msg: string, isError: boolean): void {
  tbody.textContent = '';
  const tr = document.createElement('tr');
  const td = document.createElement('td');
  td.setAttribute('colspan', '3');
  td.style.cssText = 'text-align:center;padding:16px;color:' + (isError ? '#991b1b' : '#94a3b8');
  td.textContent = msg;
  tr.appendChild(td);
  tbody.appendChild(tr);
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

  const renderAddWorkspaceSection = (accountID: string): void => {
    const form = getElement<HTMLElement>('add-ws-form-' + accountID);
    if (!form) return;
    const entry = ensurePortalAccountUIEntry(deps.store.getAccountState(), accountID);
    form.classList.toggle('visible', entry.addWorkspaceOpen);
  };

  const renderTeamSection = (accountID: string): void => {
    const section = getElement<HTMLElement>('team-section-' + accountID);
    const tbody = getElement<HTMLElement>('team-list-' + accountID);
    if (!section || !tbody) return;
    const entry = ensurePortalAccountUIEntry(deps.store.getAccountState(), accountID);
    const actorRole = section.getAttribute('data-actor-role') || '';
    const isOwner = actorRole === 'owner';
    section.classList.toggle('visible', entry.teamVisible);

    if (!entry.teamVisible) {
      return;
    }
    if (entry.teamLoading) {
      setTbodyMessage(tbody, 'Loading…', false);
      return;
    }
    if (entry.teamError) {
      setTbodyMessage(tbody, entry.teamError, true);
      return;
    }
    if (!entry.teamMembers.length) {
      setTbodyMessage(tbody, 'No team members.', false);
      return;
    }

    const allRoles = ['owner', 'admin', 'tech', 'read_only'];
    const nonOwnerRoles = ['admin', 'tech', 'read_only'];
    tbody.textContent = '';
    for (let i = 0; i < entry.teamMembers.length; i += 1) {
      const member = entry.teamMembers[i];
      const tr = document.createElement('tr');
      const tdEmail = document.createElement('td');
      tdEmail.textContent = member.email;
      tr.appendChild(tdEmail);

      const tdRole = document.createElement('td');
      if (member.role === 'owner' && !isOwner) {
        tdRole.textContent = 'owner';
      } else {
        const sel = document.createElement('select');
        const roles = isOwner ? allRoles : nonOwnerRoles;
        for (let j = 0; j < roles.length; j += 1) {
          const opt = document.createElement('option');
          opt.value = roles[j];
          opt.textContent = roles[j].replace('_', ' ');
          if (member.role === roles[j]) opt.selected = true;
          sel.appendChild(opt);
        }
        sel.setAttribute('data-action', 'change-role');
        sel.setAttribute('data-account-id', accountID);
        sel.setAttribute('data-user-id', member.user_id);
        tdRole.appendChild(sel);
      }
      tr.appendChild(tdRole);

      const tdAction = document.createElement('td');
      if (!(member.role === 'owner' && !isOwner)) {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'btn-remove';
        btn.textContent = 'Remove';
        btn.setAttribute('data-action', 'remove-member');
        btn.setAttribute('data-account-id', accountID);
        btn.setAttribute('data-user-id', member.user_id);
        btn.setAttribute('data-member-email', member.email);
        tdAction.appendChild(btn);
      }
      tr.appendChild(tdAction);
      tbody.appendChild(tr);
    }
  };

  const renderAccountUI = (): void => {
    const state = deps.store.getAccountState();
    const accountIDs = Object.keys(state.byAccountID);
    for (let i = 0; i < accountIDs.length; i += 1) {
      renderAddWorkspaceSection(accountIDs[i]);
      renderTeamSection(accountIDs[i]);
    }
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
      const input = getElement<FormValueElement>('ws-name-' + accountID);
      if (input) input.focus();
    }
  };

  const createWorkspace = async (accountID: string): Promise<void> => {
    const nameEl = getElement<FormValueElement>('ws-name-' + accountID);
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

  deps.store.subscribeAccount(renderAccountUI);
  deps.store.subscribeBootstrap(renderAccountUI);
}
