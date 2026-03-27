import type {
  PortalAccountState,
  PortalAccountSummary,
  PortalAccountUIEntry,
  PortalTeamMember,
  PortalWorkspaceSummary,
} from './types';

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

function workspaceActionLabel(workspace: PortalWorkspaceSummary): string {
  return workspace.state === 'active' ? 'Suspend workspace' : 'Delete workspace';
}

function workspaceSummary(workspace: PortalWorkspaceSummary): string {
  if (workspace.health_status === 'healthy') return 'Live updates and health checks are currently good.';
  if (workspace.health_status === 'unhealthy') return 'This workspace needs attention before it is trustworthy.';
  return 'This workspace is still waiting on a completed health check.';
}

function workspaceMeta(workspace: PortalWorkspaceSummary): string {
  var parts = [workspace.state];
  if (workspace.health_status) parts.push(workspace.health_status);
  if (workspace.created_at) {
    var date = new Date(workspace.created_at);
    if (!Number.isNaN(date.getTime())) {
      parts.push('Created ' + date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }));
    }
  }
  return parts.join(' · ');
}

function findWorkspace(account: PortalAccountSummary, workspaceID: string): PortalWorkspaceSummary | null {
  for (var i = 0; i < account.workspaces.length; i += 1) {
    if (account.workspaces[i].id === workspaceID) return account.workspaces[i];
  }
  return null;
}

export function renderWorkspaceManagement(account: PortalAccountSummary, entry: PortalAccountUIEntry): void {
  var panel = getElement<HTMLElement>('workspace-management-' + account.id);
  if (!panel) return;
  var empty = getElement<HTMLElement>('workspace-management-empty-' + account.id);
  var content = getElement<HTMLElement>('workspace-management-content-' + account.id);
  var title = getElement<HTMLElement>('workspace-management-title-' + account.id);
  var meta = getElement<HTMLElement>('workspace-management-meta-' + account.id);
  var summary = getElement<HTMLElement>('workspace-management-summary-' + account.id);
  var actionButton = getElement<HTMLButtonElement>('workspace-management-action-' + account.id);
  var closeButton = getElement<HTMLButtonElement>('workspace-management-close-' + account.id);
  if (!empty || !content || !title || !meta || !summary || !actionButton || !closeButton) return;

  var workspace = entry.selectedWorkspaceID ? findWorkspace(account, entry.selectedWorkspaceID) : null;
  var hasSelection = !!workspace;
  panel.classList.toggle('visible', hasSelection);
  empty.hidden = hasSelection;
  content.hidden = !hasSelection;

  if (!workspace) {
    actionButton.disabled = false;
    actionButton.removeAttribute('data-workspace-id');
    actionButton.removeAttribute('data-workspace-name');
    actionButton.removeAttribute('data-workspace-action');
    return;
  }

  title.textContent = workspace.display_name;
  meta.textContent = workspaceMeta(workspace);
  summary.textContent = workspaceSummary(workspace);
  actionButton.textContent = workspaceActionLabel(workspace);
  actionButton.disabled = entry.manageWorkspace.pending;
  actionButton.setAttribute('data-workspace-id', workspace.id);
  actionButton.setAttribute('data-workspace-name', workspace.display_name);
  actionButton.setAttribute('data-workspace-action', workspace.state === 'active' ? 'suspend' : 'delete');
  closeButton.disabled = entry.manageWorkspace.pending;
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

function countMembersByRole(members: PortalTeamMember[], role: string): number {
  var count = 0;
  for (var i = 0; i < members.length; i += 1) {
    if (members[i].role === role) count += 1;
  }
  return count;
}

function renderTeamStats(accountID: string, entry: PortalAccountUIEntry): void {
  var stats = getElement<HTMLElement>('team-stats-' + accountID);
  if (!stats) return;
  if (!entry.teamVisible) {
    stats.innerHTML = '';
    return;
  }
  if (entry.teamQuery.status === 'loading') {
    stats.innerHTML = '<div class="team-stat-card"><span class="team-stat-label">Roster</span><span class="team-stat-value">Loading…</span></div>';
    return;
  }
  if (entry.teamQuery.status === 'error') {
    stats.innerHTML = '<div class="team-stat-card"><span class="team-stat-label">Roster</span><span class="team-stat-value team-stat-error">Needs attention</span></div>';
    return;
  }

  var members = entry.teamQuery.data;
  stats.innerHTML =
    '<div class="team-stat-card"><span class="team-stat-label">Members</span><span class="team-stat-value">' + String(members.length) + '</span></div>' +
    '<div class="team-stat-card"><span class="team-stat-label">Owners</span><span class="team-stat-value">' + String(countMembersByRole(members, 'owner')) + '</span></div>' +
    '<div class="team-stat-card"><span class="team-stat-label">Admins</span><span class="team-stat-value">' + String(countMembersByRole(members, 'admin')) + '</span></div>' +
    '<div class="team-stat-card"><span class="team-stat-label">Operators</span><span class="team-stat-value">' + String(countMembersByRole(members, 'tech') + countMembersByRole(members, 'read_only')) + '</span></div>';
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
  renderTeamStats(accountID, entry);

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

export function renderAccountUI(accountState: PortalAccountState, accounts: PortalAccountSummary[]): void {
  var accountIDs = Object.keys(accountState.byAccountID);
  for (var i = 0; i < accountIDs.length; i += 1) {
    var accountID = accountIDs[i];
    var entry = accountState.byAccountID[accountID];
    var account = null as PortalAccountSummary | null;
    for (var j = 0; j < accounts.length; j += 1) {
      if (accounts[j].id === accountID) {
        account = accounts[j];
        break;
      }
    }
    renderAddWorkspaceSection(accountID, entry);
    if (account) renderWorkspaceManagement(account, entry);
    renderTeamSection(accountID, entry);
  }
}
