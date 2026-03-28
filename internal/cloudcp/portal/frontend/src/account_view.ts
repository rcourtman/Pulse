import type {
  PortalAccessJob,
  PortalAccountState,
  PortalAccountSummary,
  PortalAccountUIEntry,
  PortalAccessMember,
  PortalWorkspaceSummary,
} from './types';

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

function normalizedAccessRole(role: string): string {
  if (role === 'member') return 'read_only';
  return role || 'read_only';
}

function roleLabel(role: string): string {
  switch (normalizedAccessRole(role)) {
    case 'owner':
      return 'Owner';
    case 'admin':
      return 'Admin';
    case 'tech':
      return 'Tech';
    case 'read_only':
      return 'Read-only';
    case 'member':
      return 'Member';
    default:
      return role || 'Member';
  }
}

function roleCapabilityCopy(role: string): string {
  switch (normalizedAccessRole(role)) {
    case 'owner':
      return 'Full account control, including billing, access control, and workspace control.';
    case 'admin':
      return 'Can manage workspaces and billing for this account.';
    case 'tech':
      return 'Can manage workspaces without billing ownership.';
    case 'read_only':
      return 'Can review workspace status without making control-plane changes.';
    case 'member':
      return 'Has access through the account roster.';
    default:
      return 'Has access through the account roster.';
  }
}

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

function workspaceHealthLabel(workspace: PortalWorkspaceSummary): string {
  if (workspace.health_status === 'healthy') return 'Healthy';
  if (workspace.health_status === 'unhealthy') return 'Needs attention';
  return 'Checking';
}

function workspaceCreatedLabel(workspace: PortalWorkspaceSummary): string {
  if (!workspace.created_at) return 'Unknown';
  var date = new Date(workspace.created_at);
  if (Number.isNaN(date.getTime())) return 'Unknown';
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function workspaceGuidance(workspace: PortalWorkspaceSummary): string {
  if (workspace.state === 'active' && workspace.health_status === 'healthy') {
    return 'This workspace looks ready for normal use. Use the fleet table to open it, or suspend it here if you are intentionally taking it out of service.';
  }
  if (workspace.state === 'active' && workspace.health_status === 'checking') {
    return 'This workspace is active but still waiting on a completed health check. Review it before you treat the account status as settled.';
  }
  if (workspace.health_status === 'unhealthy') {
    return 'This workspace needs review before it is treated as trustworthy. Use the management action only when you intend to suspend or remove it from the workspace list.';
  }
  if (workspace.state === 'suspended') {
    return 'This workspace is already suspended. The remaining lifecycle action here is deletion, so treat it as a deliberate irreversible step.';
  }
  return 'Review the lifecycle state before taking the next explicit action for this workspace.';
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
  summary.textContent = workspaceSummary(workspace);
  health.textContent = workspaceHealthLabel(workspace);
  lifecycle.textContent = workspace.state ? workspace.state.charAt(0).toUpperCase() + workspace.state.slice(1) : 'Unknown';
  created.textContent = workspaceCreatedLabel(workspace);
  guidance.textContent = workspaceGuidance(workspace);
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
    if (normalizedAccessRole(members[i].role) === role) count += 1;
  }
  return count;
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
    stats.innerHTML =
      '<div class="access-stat-card"><span class="access-stat-label">Roster</span><span class="access-stat-value">Loading…</span></div>' +
      '<div class="access-stat-card"><span class="access-stat-label">Mode</span><span class="access-stat-value">' + (canManage ? 'Manage' : 'View') + '</span></div>';
    return;
  }
  if (entry.accessQuery.status === 'error') {
    stats.innerHTML =
      '<div class="access-stat-card"><span class="access-stat-label">Roster</span><span class="access-stat-value access-stat-error">Needs attention</span></div>' +
      '<div class="access-stat-card"><span class="access-stat-label">Mode</span><span class="access-stat-value">' + (canManage ? 'Manage' : 'View') + '</span></div>';
    return;
  }

  var members = entry.accessQuery.data;
  stats.innerHTML =
    '<div class="access-stat-card"><span class="access-stat-label">Members</span><span class="access-stat-value">' + String(members.length) + '</span></div>' +
    '<div class="access-stat-card"><span class="access-stat-label">Owners</span><span class="access-stat-value">' + String(countMembersByRole(members, 'owner')) + '</span></div>' +
    '<div class="access-stat-card"><span class="access-stat-label">Admins</span><span class="access-stat-value">' + String(countMembersByRole(members, 'admin')) + '</span></div>' +
    '<div class="access-stat-card"><span class="access-stat-label">Operators</span><span class="access-stat-value">' + String(countMembersByRole(members, 'tech') + countMembersByRole(members, 'read_only')) + '</span></div>';
}

function createAccessControlCell(className: string): HTMLDivElement {
  var cell = document.createElement('div');
  cell.className = 'access-control-cell ' + className;
  return cell;
}

function renderAccessRoleControl(accountID: string, member: PortalAccessMember, isOwner: boolean, canManage: boolean, activeJob: PortalAccessJob): HTMLElement {
  var currentRole = normalizedAccessRole(member.role);
  var group = createAccessControlCell('access-control-cell-role');
  if (!canManage || activeJob !== 'change_role') {
    var badge = document.createElement('span');
    badge.className = 'access-role-badge';
    badge.textContent = roleLabel(currentRole);
    group.appendChild(badge);
    return group;
  }
  if (currentRole === 'owner' && !isOwner) {
    var locked = document.createElement('span');
    locked.className = 'access-role-badge';
    locked.textContent = roleLabel(currentRole);
    group.appendChild(locked);
    return group;
  }

  var sel = document.createElement('select');
  sel.className = 'access-role-select';
  var roles = isOwner ? ['owner', 'admin', 'tech', 'read_only'] : ['admin', 'tech', 'read_only'];
  for (var j = 0; j < roles.length; j += 1) {
    var opt = document.createElement('option');
    opt.value = roles[j];
    opt.textContent = roleLabel(roles[j]);
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
  var group = createAccessControlCell('access-control-cell-access');
  if (!canManage) {
    return null;
  }
  if (activeJob !== 'remove') {
    var idleText = document.createElement('span');
    idleText.className = 'access-control-locked';
    idleText.textContent = activeJob === 'change_role' ? 'Role change' : 'Review only';
    group.appendChild(idleText);
    return group;
  }
  if (normalizedAccessRole(member.role) === 'owner' && !isOwner) {
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
  var row = document.createElement('div');
  row.className = 'access-member-row' + (canManage ? '' : ' access-member-row-readonly');

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
  roleBadge.textContent = roleLabel(member.role);
  topline.appendChild(roleBadge);

  identity.appendChild(topline);

  var caption = document.createElement('div');
  caption.className = 'access-member-caption';
  caption.textContent = roleCapabilityCopy(member.role);
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
  var head = document.createElement('div');
  head.className = 'access-roster-head' + (canManage ? '' : ' access-roster-head-readonly');
  head.innerHTML = canManage
    ? (
      '<span>Operator</span>' +
      '<span>' + (activeJob === 'change_role' ? 'New role' : 'Role') + '</span>' +
      '<span>' + (activeJob === 'remove' ? 'Remove' : 'Action') + '</span>'
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
    setContainerMessage(roster, 'Roster needs attention', entry.accessQuery.error, true);
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
