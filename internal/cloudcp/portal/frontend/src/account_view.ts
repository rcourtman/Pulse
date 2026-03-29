import type {
  PortalAccessJob,
  PortalAccountState,
  PortalAccountSummary,
  PortalAccountUIEntry,
  PortalAccessMember,
  PortalWorkspaceSummary,
} from './types';
import { normalizePortalRole, portalRoleCapabilityCopy, portalRoleLabel } from './account_roles';
import { workspaceGuidanceCopy, workspaceHealthLabel, workspaceStatusCopy } from './workspace_presentation';

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

function workspaceCreatedLabel(workspace: PortalWorkspaceSummary): string {
  if (!workspace.created_at) return 'Unknown';
  var date = new Date(workspace.created_at);
  if (Number.isNaN(date.getTime())) return 'Unknown';
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
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
  var shell = getElement<HTMLElement>('workspace-operations-shell-' + account.id);
  var detail = getElement<HTMLElement>('workspace-operations-detail-' + account.id);
  if (!panel) return;
  var empty = getElement<HTMLElement>('workspace-management-empty-' + account.id);
  var content = getElement<HTMLElement>('workspace-management-content-' + account.id);
  var title = getElement<HTMLElement>('workspace-management-title-' + account.id);
  var meta = getElement<HTMLElement>('workspace-management-meta-' + account.id);
  var summary = getElement<HTMLElement>('workspace-management-summary-' + account.id);
  var health = getElement<HTMLElement>('workspace-management-health-' + account.id);
  var lifecycle = getElement<HTMLElement>('workspace-management-lifecycle-' + account.id);
  var created = getElement<HTMLElement>('workspace-management-created-' + account.id);
  var guidance = getElement<HTMLElement>('workspace-management-guidance-' + account.id);
  var actionButton = getElement<HTMLButtonElement>('workspace-management-action-' + account.id);
  var closeButton = getElement<HTMLButtonElement>('workspace-management-close-' + account.id);
  if (!empty || !content || !title || !meta || !summary || !health || !lifecycle || !created || !guidance || !actionButton || !closeButton) return;

  var workspace = entry.selectedWorkspaceID ? findWorkspace(account, entry.selectedWorkspaceID) : null;
  var hasSelection = !!workspace;
  var showDetail = hasSelection || entry.addWorkspaceOpen;
  var rows = document.querySelectorAll<HTMLElement>('[data-workspace-row]');
  for (var i = 0; i < rows.length; i += 1) {
    rows[i].classList.toggle('selected', !!workspace && rows[i].getAttribute('data-workspace-row') === workspace.id);
  }
  if (shell) {
    shell.classList.toggle('workspace-operations-shell-selected', hasSelection);
    shell.classList.toggle('workspace-operations-shell-idle', !showDetail);
    shell.classList.toggle('workspace-operations-shell-form-open', entry.addWorkspaceOpen);
  }
  if (detail) {
    detail.classList.toggle('workspace-operations-detail-selected', hasSelection);
    detail.classList.toggle('workspace-operations-detail-idle', !showDetail);
    detail.hidden = !showDetail;
  }
  panel.classList.toggle('workspace-management-panel-selected', hasSelection);
  panel.classList.toggle('workspace-management-panel-idle', !hasSelection);
  panel.classList.toggle('visible', showDetail);
  panel.hidden = !showDetail;
  empty.hidden = hasSelection || !showDetail;
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
  summary.textContent = workspaceStatusCopy(workspace);
  health.textContent = workspaceHealthLabel(workspace);
  lifecycle.textContent = workspace.state ? workspace.state.charAt(0).toUpperCase() + workspace.state.slice(1) : 'Unknown';
  created.textContent = workspaceCreatedLabel(workspace);
  guidance.textContent = workspaceGuidanceCopy(workspace);
  actionButton.textContent = workspaceActionLabel(workspace);
  actionButton.disabled = entry.manageWorkspace.pending;
  actionButton.setAttribute('data-workspace-id', workspace.id);
  actionButton.setAttribute('data-workspace-name', workspace.display_name);
  actionButton.setAttribute('data-workspace-action', workspace.state === 'active' ? 'suspend' : 'delete');
  closeButton.disabled = entry.manageWorkspace.pending;
}

function setContainerMessage(container: HTMLElement, title: string, msg: string, isError: boolean): void {
  container.textContent = '';
  container.classList.add('state-only');
  var message = document.createElement('div');
  message.className = 'access-list-message' + (isError ? ' error' : '');
  var heading = document.createElement('strong');
  heading.className = 'access-list-message-title';
  heading.textContent = title;
  var copy = document.createElement('span');
  copy.className = 'access-list-message-copy';
  copy.textContent = msg;
  message.appendChild(heading);
  message.appendChild(copy);
  container.appendChild(message);
}

function countMembersByRole(members: PortalAccessMember[], role: string): number {
  var count = 0;
  for (var i = 0; i < members.length; i += 1) {
    if (normalizePortalRole(members[i].role) === role) count += 1;
  }
  return count;
}

function renderAccessStatsSummary(summary: string, isError: boolean): string {
  return '<div class="access-stat-summary' + (isError ? ' access-stat-summary-error' : '') + '">' + summary + '</div>';
}

function accessJobTitle(job: PortalAccessJob): string {
  switch (job) {
    case 'invite':
      return 'Invite people';
    case 'change_role':
      return 'Change roles';
    case 'remove':
      return 'Remove access';
    default:
      return '';
  }
}

function accessJobCopy(job: PortalAccessJob): string {
  switch (job) {
    case 'invite':
      return 'Add one person with the minimum role they need on this account.';
    case 'change_role':
      return 'Use the roster to change one person at a time and keep each person on the smallest role they need.';
    case 'remove':
      return 'Use removal only when this person should no longer be on this hosted account.';
    default:
      return '';
  }
}

function renderAccessStats(accountID: string, entry: PortalAccountUIEntry, canManage: boolean): void {
  var stats = getElement<HTMLElement>('access-stats-' + accountID);
  if (!stats) return;
  if (!entry.accessVisible) {
    stats.innerHTML = '';
    return;
  }
  if (entry.accessQuery.status === 'loading') {
    stats.innerHTML = renderAccessStatsSummary('Access • ' + (canManage ? 'Manage access' : 'View roster') + ' • Loading roster', false);
    return;
  }
  if (entry.accessQuery.status === 'error') {
    stats.innerHTML = renderAccessStatsSummary('Access • ' + (canManage ? 'Manage access' : 'View roster') + ' • Load failed', true);
    return;
  }

  var members = entry.accessQuery.data;
  stats.innerHTML = renderAccessStatsSummary(
    'Members ' + String(members.length) + ' • ' +
      'Owners ' + String(countMembersByRole(members, 'owner')) + ' • ' +
      'Admins ' + String(countMembersByRole(members, 'admin')) + ' • ' +
      'Operators ' + String(countMembersByRole(members, 'tech') + countMembersByRole(members, 'read_only')),
    false
  );
}

function createAccessControlCell(className: string): HTMLDivElement {
  var cell = document.createElement('div');
  cell.className = 'access-control-cell ' + className;
  return cell;
}

function renderAccessRoleControl(accountID: string, member: PortalAccessMember, isOwner: boolean, canManage: boolean, activeJob: PortalAccessJob): HTMLElement {
  var currentRole = normalizePortalRole(member.role);
  var group = createAccessControlCell('access-control-cell-role');
  if (!canManage || activeJob !== 'change_role') {
    var badge = document.createElement('span');
    badge.className = 'access-role-badge';
    badge.textContent = portalRoleLabel(currentRole);
    group.appendChild(badge);
    return group;
  }
  if (currentRole === 'owner' && !isOwner) {
    var locked = document.createElement('span');
    locked.className = 'access-role-badge';
    locked.textContent = portalRoleLabel(currentRole);
    group.appendChild(locked);
    return group;
  }

  var sel = document.createElement('select');
  sel.className = 'access-role-select';
  var roles = isOwner ? ['owner', 'admin', 'tech', 'read_only'] : ['admin', 'tech', 'read_only'];
  for (var j = 0; j < roles.length; j += 1) {
    var opt = document.createElement('option');
    opt.value = roles[j];
    opt.textContent = portalRoleLabel(roles[j]);
    if (currentRole === roles[j]) opt.selected = true;
    sel.appendChild(opt);
  }
  sel.setAttribute('data-action', 'change-role');
  sel.setAttribute('data-account-id', accountID);
  sel.setAttribute('data-user-id', member.user_id);
  group.appendChild(sel);
  return group;
}

function renderAccessMemberAction(accountID: string, member: PortalAccessMember, isOwner: boolean, canManage: boolean, activeJob: PortalAccessJob): HTMLElement | null {
  if (!canManage || activeJob !== 'remove') {
    return null;
  }
  var group = createAccessControlCell('access-control-cell-access');
  if (normalizePortalRole(member.role) === 'owner' && !isOwner) {
    var lockedText = document.createElement('span');
    lockedText.className = 'access-control-locked';
    lockedText.textContent = 'Locked';
    group.appendChild(lockedText);
    return group;
  }

  var btn = document.createElement('button');
  btn.type = 'button';
  btn.className = 'btn-remove';
  btn.textContent = 'Remove access';
  btn.setAttribute('data-action', 'remove-member');
  btn.setAttribute('data-account-id', accountID);
  btn.setAttribute('data-user-id', member.user_id);
  btn.setAttribute('data-member-email', member.email);
  group.appendChild(btn);
  return group;
}

function renderAccessMemberRow(accountID: string, member: PortalAccessMember, isOwner: boolean, canManage: boolean, activeJob: PortalAccessJob): HTMLElement {
  var showActionColumn = canManage && activeJob === 'remove';
  var row = document.createElement('div');
  row.className = 'access-member-row' + (showActionColumn ? '' : ' access-member-row-readonly');

  var identity = document.createElement('div');
  identity.className = 'access-member-identity';

  var topline = document.createElement('div');
  topline.className = 'access-member-topline';

  var email = document.createElement('div');
  email.className = 'access-member-email';
  email.textContent = member.email;
  topline.appendChild(email);

  var roleBadge = document.createElement('span');
  roleBadge.className = 'access-inline-role-badge';
  roleBadge.textContent = portalRoleLabel(member.role);
  topline.appendChild(roleBadge);

  identity.appendChild(topline);

  var caption = document.createElement('div');
  caption.className = 'access-member-caption';
  caption.textContent = portalRoleCapabilityCopy(member.role);
  identity.appendChild(caption);

  row.appendChild(identity);
  row.appendChild(renderAccessRoleControl(accountID, member, isOwner, canManage, activeJob));
  var actionCell = renderAccessMemberAction(accountID, member, isOwner, canManage, activeJob);
  if (actionCell) {
    row.appendChild(actionCell);
  }
  return row;
}

function renderAccessRosterHead(container: HTMLElement, activeJob: PortalAccessJob, canManage: boolean): void {
  var showActionColumn = canManage && activeJob === 'remove';
  var head = document.createElement('div');
  head.className = 'access-roster-head' + (showActionColumn ? '' : ' access-roster-head-readonly');
  head.innerHTML = showActionColumn
    ? (
      '<span>Operator</span>' +
      '<span>Role</span>' +
      '<span>Remove</span>'
    )
    : (
      '<span>Operator</span>' +
      '<span>Role</span>'
    );
  container.appendChild(head);
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

export function renderAccessSection(accountID: string, entry: PortalAccountUIEntry): void {
  var section = getElement<HTMLElement>('access-section-' + accountID);
  var roster = getElement<HTMLElement>('access-list-' + accountID);
  if (!section || !roster) return;
  var rosterPanel = roster.closest('.access-roster') as HTMLElement | null;
  var shell = getElement<HTMLElement>('access-shell-' + accountID);
  var detail = getElement<HTMLElement>('access-detail-' + accountID);
  var taskPanel = getElement<HTMLElement>('access-task-panel-' + accountID);
  var taskTitle = getElement<HTMLElement>('access-task-title-' + accountID);
  var taskCopy = getElement<HTMLElement>('access-task-copy-' + accountID);
  var taskButtons = {
    invite: getElement<HTMLElement>('access-task-invite-' + accountID),
    change_role: getElement<HTMLElement>('access-task-change_role-' + accountID),
    remove: getElement<HTMLElement>('access-task-remove-' + accountID),
  };
  var taskBodies = {
    invite: getElement<HTMLElement>('access-task-body-invite-' + accountID),
    change_role: getElement<HTMLElement>('access-task-body-change_role-' + accountID),
    remove: getElement<HTMLElement>('access-task-body-remove-' + accountID),
  };

  var actorRole = section.getAttribute('data-actor-role') || '';
  var isOwner = actorRole === 'owner';
  var canManage = section.getAttribute('data-can-manage') === 'true';
  var activeJob = canManage ? entry.activeAccessJob : '';
  section.classList.toggle('visible', entry.accessVisible);
  renderAccessStats(accountID, entry, canManage);
  if (shell) {
    shell.classList.toggle('access-shell-job-open', !!activeJob);
    shell.classList.toggle('access-shell-idle', !activeJob);
  }
  if (detail) detail.hidden = !activeJob;
  if (taskPanel) taskPanel.hidden = !activeJob;
  if (taskTitle) taskTitle.textContent = accessJobTitle(activeJob);
  if (taskCopy) taskCopy.textContent = accessJobCopy(activeJob);
  taskButtons.invite?.classList.toggle('is-active', activeJob === 'invite');
  taskButtons.change_role?.classList.toggle('is-active', activeJob === 'change_role');
  taskButtons.remove?.classList.toggle('is-active', activeJob === 'remove');
  if (taskBodies.invite) taskBodies.invite.hidden = activeJob !== 'invite';
  if (taskBodies.change_role) taskBodies.change_role.hidden = activeJob !== 'change_role';
  if (taskBodies.remove) taskBodies.remove.hidden = activeJob !== 'remove';

  if (!entry.accessVisible) {
    return;
  }
  if (entry.accessQuery.status === 'loading') {
    if (rosterPanel) rosterPanel.classList.add('state-only');
    setContainerMessage(roster, 'Loading roster', 'Checking who currently has access to this account.', false);
    return;
  }
  if (entry.accessQuery.status === 'error') {
    if (rosterPanel) rosterPanel.classList.add('state-only');
    setContainerMessage(roster, 'Failed to load roster', entry.accessQuery.error, true);
    return;
  }
  if (!entry.accessQuery.data.length) {
    if (rosterPanel) rosterPanel.classList.add('state-only');
    setContainerMessage(
      roster,
      'No one added yet',
      canManage
        ? 'Invite someone when this hosted account needs shared access.'
        : 'There is no hosted roster to review yet on this account.',
      false
    );
    return;
  }

  roster.textContent = '';
  roster.classList.remove('state-only');
  if (rosterPanel) rosterPanel.classList.remove('state-only');
  renderAccessRosterHead(roster, activeJob, canManage);
  for (var i = 0; i < entry.accessQuery.data.length; i += 1) {
    var member = entry.accessQuery.data[i];
    roster.appendChild(renderAccessMemberRow(accountID, member, isOwner, canManage, activeJob));
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
    renderAccessSection(accountID, entry);
  }
}
