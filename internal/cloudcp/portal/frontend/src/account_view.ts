import type { PortalAccountState, PortalAccountUIEntry, PortalTeamMember } from './types';

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

export function getElement<T extends HTMLElement = HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null;
}

export function asHTMLElement(target: EventTarget | null): HTMLElement | null {
  return target instanceof HTMLElement ? target : null;
}

export function focusElement(id: string): void {
  var input = getElement<FormValueElement>(id);
  if (input) input.focus();
}

export function renderWorkspaceMenus(accountID: string, entry: PortalAccountUIEntry): void {
  var menus = document.querySelectorAll<HTMLElement>('[data-workspace-menu-account-id="' + accountID + '"]');
  for (var i = 0; i < menus.length; i += 1) {
    var menu = menus[i];
    var workspaceID = menu.getAttribute('data-workspace-id') || '';
    var open = entry.openWorkspaceMenuID === workspaceID;
    menu.hidden = !open;
    var button = getElement<HTMLElement>('workspace-menu-button-' + accountID + '-' + workspaceID);
    if (button) {
      button.setAttribute('aria-expanded', open ? 'true' : 'false');
    }
  }
}

function setTbodyMessage(tbody: HTMLElement, msg: string, isError: boolean): void {
  tbody.textContent = '';
  var tr = document.createElement('tr');
  var td = document.createElement('td');
  td.setAttribute('colspan', '3');
  td.className = 'team-message-cell' + (isError ? ' error' : '');
  td.textContent = msg;
  tr.appendChild(td);
  tbody.appendChild(tr);
}

function renderTeamMemberRoleCell(accountID: string, member: PortalTeamMember, isOwner: boolean): HTMLTableCellElement {
  var tdRole = document.createElement('td');
  if (member.role === 'owner' && !isOwner) {
    tdRole.textContent = 'owner';
    return tdRole;
  }

  var sel = document.createElement('select');
  var roles = isOwner ? ['owner', 'admin', 'tech', 'read_only'] : ['admin', 'tech', 'read_only'];
  for (var j = 0; j < roles.length; j += 1) {
    var opt = document.createElement('option');
    opt.value = roles[j];
    opt.textContent = roles[j].replace('_', ' ');
    if (member.role === roles[j]) opt.selected = true;
    sel.appendChild(opt);
  }
  sel.setAttribute('data-action', 'change-role');
  sel.setAttribute('data-account-id', accountID);
  sel.setAttribute('data-user-id', member.user_id);
  tdRole.appendChild(sel);
  return tdRole;
}

function renderTeamMemberActionCell(accountID: string, member: PortalTeamMember, isOwner: boolean): HTMLTableCellElement {
  var tdAction = document.createElement('td');
  if (member.role === 'owner' && !isOwner) {
    return tdAction;
  }

  var btn = document.createElement('button');
  btn.type = 'button';
  btn.className = 'btn-remove';
  btn.textContent = 'Remove';
  btn.setAttribute('data-action', 'remove-member');
  btn.setAttribute('data-account-id', accountID);
  btn.setAttribute('data-user-id', member.user_id);
  btn.setAttribute('data-member-email', member.email);
  tdAction.appendChild(btn);
  return tdAction;
}

export function renderAddWorkspaceSection(accountID: string, entry: PortalAccountUIEntry): void {
  var form = getElement<HTMLElement>('add-ws-form-' + accountID);
  var spinner = getElement<HTMLElement>('ws-spinner-' + accountID);
  if (!form) return;
  form.classList.toggle('visible', entry.addWorkspaceOpen);
  if (spinner) {
    spinner.hidden = !entry.createWorkspace.pending;
  }
}

export function renderTeamSection(accountID: string, entry: PortalAccountUIEntry): void {
  var section = getElement<HTMLElement>('team-section-' + accountID);
  var tbody = getElement<HTMLElement>('team-list-' + accountID);
  if (!section || !tbody) return;

  var actorRole = section.getAttribute('data-actor-role') || '';
  var isOwner = actorRole === 'owner';
  section.classList.toggle('visible', entry.teamVisible);

  if (!entry.teamVisible) {
    return;
  }
  if (entry.teamQuery.status === 'loading') {
    setTbodyMessage(tbody, 'Loading…', false);
    return;
  }
  if (entry.teamQuery.status === 'error') {
    setTbodyMessage(tbody, entry.teamQuery.error, true);
    return;
  }
  if (!entry.teamQuery.data.length) {
    setTbodyMessage(tbody, 'No team members.', false);
    return;
  }

  tbody.textContent = '';
  for (var i = 0; i < entry.teamQuery.data.length; i += 1) {
    var member = entry.teamQuery.data[i];
    var tr = document.createElement('tr');
    var tdEmail = document.createElement('td');
    tdEmail.textContent = member.email;
    tr.appendChild(tdEmail);
    tr.appendChild(renderTeamMemberRoleCell(accountID, member, isOwner));
    tr.appendChild(renderTeamMemberActionCell(accountID, member, isOwner));
    tbody.appendChild(tr);
  }
}

export function renderAccountUI(accountState: PortalAccountState): void {
  var accountIDs = Object.keys(accountState.byAccountID);
  for (var i = 0; i < accountIDs.length; i += 1) {
    var accountID = accountIDs[i];
    var entry = accountState.byAccountID[accountID];
    renderAddWorkspaceSection(accountID, entry);
    renderWorkspaceMenus(accountID, entry);
    renderTeamSection(accountID, entry);
  }
}
