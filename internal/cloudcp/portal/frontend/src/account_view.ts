import type {
  PortalAccountState,
  PortalAccountSummary,
  PortalAccountUIEntry,
  PortalTeamMember,
  PortalWorkspaceSummary,
} from './types';

type FormValueElement = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

function normalizedTeamRole(role: string): string {
  if (role === 'member') return 'read_only';
  return role || 'read_only';
}

function roleLabel(role: string): string {
  switch (normalizedTeamRole(role)) {
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
  switch (normalizedTeamRole(role)) {
    case 'owner':
      return 'Full account control, including billing, team access, and hosted operations.';
    case 'admin':
      return 'Can manage hosted operations and billing for this account.';
    case 'tech':
      return 'Can operate hosted workspaces without billing ownership.';
    case 'read_only':
      return 'Can review hosted state without making control-plane changes.';
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
    return 'This workspace looks ready for normal operator work. Use the fleet table to open it, or suspend it here if you are intentionally taking it out of service.';
  }
  if (workspace.state === 'active' && workspace.health_status === 'checking') {
    return 'This workspace is active but still waiting on a completed health check. Review it before you treat the hosted posture as settled.';
  }
  if (workspace.health_status === 'unhealthy') {
    return 'This workspace needs review before it is treated as trustworthy. Use the management action only when you intend to suspend or remove it from the hosted fleet.';
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
  panel.classList.add('visible');
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

function setContainerMessage(container: HTMLElement, msg: string, isError: boolean): void {
  container.textContent = '';
  var message = document.createElement('div');
  message.className = 'team-list-message' + (isError ? ' error' : '');
  message.textContent = msg;
  container.appendChild(message);
}

function countMembersByRole(members: PortalTeamMember[], role: string): number {
  var count = 0;
  for (var i = 0; i < members.length; i += 1) {
    if (normalizedTeamRole(members[i].role) === role) count += 1;
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

function renderTeamRoleControl(accountID: string, member: PortalTeamMember, isOwner: boolean): HTMLElement {
  var currentRole = normalizedTeamRole(member.role);
  if (currentRole === 'owner' && !isOwner) {
    var locked = document.createElement('span');
    locked.className = 'team-role-badge';
    locked.textContent = roleLabel(currentRole);
    return locked;
  }

  var sel = document.createElement('select');
  sel.className = 'team-role-select';
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
  return sel;
}

function renderTeamMemberAction(accountID: string, member: PortalTeamMember, isOwner: boolean): HTMLElement | null {
  if (normalizedTeamRole(member.role) === 'owner' && !isOwner) {
    return null;
  }

  var btn = document.createElement('button');
  btn.type = 'button';
  btn.className = 'btn-remove';
  btn.textContent = 'Remove';
  btn.setAttribute('data-action', 'remove-member');
  btn.setAttribute('data-account-id', accountID);
  btn.setAttribute('data-user-id', member.user_id);
  btn.setAttribute('data-member-email', member.email);
  return btn;
}

function renderTeamMemberRow(accountID: string, member: PortalTeamMember, isOwner: boolean): HTMLElement {
  var row = document.createElement('div');
  row.className = 'team-member-row';

  var identity = document.createElement('div');
  identity.className = 'team-member-identity';

  var topline = document.createElement('div');
  topline.className = 'team-member-topline';

  var email = document.createElement('div');
  email.className = 'team-member-email';
  email.textContent = member.email;
  topline.appendChild(email);

  var roleBadge = document.createElement('span');
  roleBadge.className = 'team-inline-role-badge';
  roleBadge.textContent = roleLabel(member.role);
  topline.appendChild(roleBadge);

  identity.appendChild(topline);

  var caption = document.createElement('div');
  caption.className = 'team-member-caption';
  caption.textContent = roleCapabilityCopy(member.role);
  identity.appendChild(caption);

  var controls = document.createElement('div');
  controls.className = 'team-member-controls';
  controls.appendChild(renderTeamRoleControl(accountID, member, isOwner));
  var action = renderTeamMemberAction(accountID, member, isOwner);
  if (action) controls.appendChild(action);

  row.appendChild(identity);
  row.appendChild(controls);
  return row;
}

function ensureRosterHead(container: HTMLElement): void {
  var existing = container.querySelector('.team-roster-head');
  if (existing) return;
  var head = document.createElement('div');
  head.className = 'team-roster-head';
  head.innerHTML =
    '<span>Operator</span>' +
    '<span>Controls</span>';
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

export function renderTeamSection(accountID: string, entry: PortalAccountUIEntry): void {
  var section = getElement<HTMLElement>('team-section-' + accountID);
  var roster = getElement<HTMLElement>('team-list-' + accountID);
  if (!section || !roster) return;

  var actorRole = section.getAttribute('data-actor-role') || '';
  var isOwner = actorRole === 'owner';
  section.classList.toggle('visible', entry.teamVisible);
  renderTeamStats(accountID, entry);

  if (!entry.teamVisible) {
    return;
  }
  if (entry.teamQuery.status === 'loading') {
    setContainerMessage(roster, 'Loading…', false);
    return;
  }
  if (entry.teamQuery.status === 'error') {
    setContainerMessage(roster, entry.teamQuery.error, true);
    return;
  }
  if (!entry.teamQuery.data.length) {
    setContainerMessage(roster, 'No team members.', false);
    return;
  }

  roster.textContent = '';
  ensureRosterHead(roster);
  for (var i = 0; i < entry.teamQuery.data.length; i += 1) {
    var member = entry.teamQuery.data[i];
    roster.appendChild(renderTeamMemberRow(accountID, member, isOwner));
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
