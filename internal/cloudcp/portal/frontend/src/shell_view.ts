import type {
  PortalAccountSummary,
  PortalBootstrapData,
  PortalLoginState,
  PortalShellSection,
  PortalWorkspaceSummary,
} from './types';

export interface ShellViewContext {
  bootstrap: PortalBootstrapData;
  loginState: PortalLoginState;
  signupPath: string;
  accountAPIBasePath: string;
  activeSection?: PortalShellSection;
}

function escapeHTML(value: unknown): string {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function escapeAttr(value: unknown): string {
  return escapeHTML(value);
}

function titleCase(value: string): string {
  if (!value) return '';
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function formatWorkspaceDate(value: unknown): string {
  if (!value) return '';
  var date = new Date(String(value));
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function workspaceHealthState(workspace: PortalWorkspaceSummary): 'healthy' | 'checking' | 'unhealthy' {
  if (workspace.health_status === 'healthy' || workspace.health_status === 'checking' || workspace.health_status === 'unhealthy') {
    return workspace.health_status;
  }
  if (workspace.healthy) return 'healthy';
  if (workspace.last_health_check) return 'unhealthy';
  return 'checking';
}

function roleBadgeHTML(role: string): string {
  return '<span class="badge badge-role">' + escapeHTML(titleCase(role || 'member')) + '</span>';
}

function accountKindLabel(account: PortalAccountSummary): string {
  if (account.kind === 'msp') return 'MSP account';
  if (account.kind === 'cloud') return 'Cloud account';
  if (account.kind === 'individual') return 'Hosted account';
  return account.kind_label ? account.kind_label + ' account' : 'Account';
}

function workspaceCountLabel(count: number): string {
  return count === 1 ? '1 workspace' : String(count) + ' workspaces';
}

function hasHostedAccounts(accounts: PortalAccountSummary[]): boolean {
  return accounts.length > 0;
}

function countWorkspaces(accounts: PortalAccountSummary[]): number {
  var total = 0;
  for (var i = 0; i < accounts.length; i += 1) {
    total += Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces.length : 0;
  }
  return total;
}

function collectWorkspaces(accounts: PortalAccountSummary[]): PortalWorkspaceSummary[] {
  var results: PortalWorkspaceSummary[] = [];
  for (var i = 0; i < accounts.length; i += 1) {
    var workspaces = Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces : [];
    for (var j = 0; j < workspaces.length; j += 1) {
      results.push(workspaces[j]);
    }
  }
  return results;
}

function countWorkspacesByState(workspaces: PortalWorkspaceSummary[], state: string): number {
  var count = 0;
  for (var i = 0; i < workspaces.length; i += 1) {
    if (String(workspaces[i].state || '') === state) count += 1;
  }
  return count;
}

function countWorkspacesByHealth(workspaces: PortalWorkspaceSummary[], status: 'healthy' | 'checking' | 'unhealthy'): number {
  var count = 0;
  for (var i = 0; i < workspaces.length; i += 1) {
    if (workspaceHealthState(workspaces[i]) === status) count += 1;
  }
  return count;
}

function countReadyWorkspaces(workspaces: PortalWorkspaceSummary[]): number {
  var count = 0;
  for (var i = 0; i < workspaces.length; i += 1) {
    if (String(workspaces[i].state || '') === 'active' && workspaceHealthState(workspaces[i]) === 'healthy') {
      count += 1;
    }
  }
  return count;
}

function healthBadgeHTML(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  if (status === 'healthy') {
    return '<span class="badge badge-healthy">Healthy</span>';
  }
  if (status === 'unhealthy') {
    return '<span class="badge badge-unhealthy">Needs attention</span>';
  }
  return '<span class="badge badge-checking">Checking</span>';
}

function renderBillingActionRow(
  id: string,
  kicker: string,
  title: string,
  actionLabel: string,
  description: string,
  panelID: string,
  focusID: string,
  highlights: string[]
): string {
  var meta = highlights.join(' • ');
  return (
    '<article class="billing-action-row">' +
      '<div class="billing-action-main">' +
        '<div class="billing-action-tags billing-action-tags-tight">' +
          '<span class="billing-card-kicker">' + kicker + '</span>' +
          '<span class="billing-action-meta-chip">' + escapeHTML(meta) + '</span>' +
        '</div>' +
        '<div class="billing-action-copy">' +
          '<h3>' + title + '</h3>' +
          '<p>' + description + '</p>' +
        '</div>' +
      '</div>' +
      '<div class="billing-action-cta">' +
        '<button class="btn-secondary billing-action-button" type="button" id="' + id + '" data-account-billing-action="open-billing-panel" data-account-billing-panel="' + panelID + '" data-account-billing-focus="' + focusID + '" data-shell-target="billing">' + escapeHTML(actionLabel) + '</button>' +
      '</div>' +
    '</article>'
  );
}

function renderSectionContextChips(chips: string[]): string {
  if (!chips.length) return '';
  return '<div class="section-context-strip">' + chips.map(function(chip) {
    return '<span class="section-context-chip">' + escapeHTML(chip) + '</span>';
  }).join('') + '</div>';
}

function workspaceStatusCopy(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  if (state === 'suspended') return 'This workspace is suspended and will stay closed until you resume it.';
  if (state === 'failed') return 'This workspace needs attention before it is trustworthy.';
  if (status === 'healthy') return 'Live updates and health checks are currently good.';
  if (status === 'unhealthy') return 'This workspace needs attention before it is trustworthy.';
  return 'This workspace is still waiting on a completed health check.';
}

function workspaceRowNote(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  if (state === 'suspended') return 'Suspended until you resume it';
  if (state === 'failed') return 'Review this workspace before treating it as stable';
  if (status === 'healthy') return 'Ready to use';
  if (status === 'unhealthy') return 'Review this workspace before treating it as stable';
  return 'Awaiting a completed health check';
}

function attentionWorkspaces(workspaces: PortalWorkspaceSummary[]): PortalWorkspaceSummary[] {
  var results: PortalWorkspaceSummary[] = [];
  for (var i = 0; i < workspaces.length; i += 1) {
    var status = workspaceHealthState(workspaces[i]);
    if (status === 'unhealthy' || status === 'checking') {
      results.push(workspaces[i]);
    }
  }
  return results;
}

function renderAttentionPanel(workspaces: PortalWorkspaceSummary[]): string {
  var attention = attentionWorkspaces(workspaces);
  var suspendedCount = countWorkspacesByState(workspaces, 'suspended');
  if (!attention.length) {
    return (
      '<div class="overview-side-card overview-side-card-stable">' +
        '<div class="account-panel-kicker">Attention</div>' +
        '<h4>' + escapeHTML(suspendedCount > 0 ? 'No active blockers' : 'Fleet is clear') + '</h4>' +
        '<p>' + escapeHTML(suspendedCount > 0
          ? 'Active workspaces are healthy. Suspended workspaces stay parked until you resume them.'
          : 'Every active workspace currently reports a healthy status.'
        ) + '</p>' +
        '<div class="overview-stable-list">' +
          '<div class="overview-stable-item"><strong>Healthy now</strong><span>' + escapeHTML(suspendedCount > 0
            ? 'Active workspaces are clear for routine use.'
            : 'All active workspaces are clear for routine use.'
          ) + '</span></div>' +
          (suspendedCount > 0
            ? '<div class="overview-stable-item"><strong>Suspended stays parked</strong><span>' + escapeHTML(String(suspendedCount) + ' workspace' + (suspendedCount === 1 ? ' is' : 's are') + ' suspended and intentionally out of day-to-day use.') + '</span></div>'
            : '') +
          '<div class="overview-stable-item"><strong>Use Access only for change</strong><span>Keep roster edits explicit instead of mixing them into normal workspace work.</span></div>' +
          '<div class="overview-stable-item"><strong>Keep billing separate</strong><span>Use Billing only when the task is commercial, not operational.</span></div>' +
        '</div>' +
      '</div>'
    );
  }

  var items = attention.slice(0, 3).map(function(workspace) {
    return (
      '<div class="overview-alert-row">' +
        '<div class="overview-alert-main">' +
          '<strong>' + escapeHTML(workspace.display_name) + '</strong>' +
          '<span>' + escapeHTML(workspaceStatusCopy(workspace)) + '</span>' +
        '</div>' +
        healthBadgeHTML(workspace) +
      '</div>'
    );
  }).join('');

  return (
    '<div class="overview-side-card">' +
      '<div class="account-panel-kicker">Attention</div>' +
      '<h4>Needs review</h4>' +
      '<p>These workspaces should be checked before you treat the workspace list as fully healthy.</p>' +
      '<div class="overview-alert-list">' + items + '</div>' +
    '</div>'
  );
}

function renderOverviewMetricStrip(
  totalCount: number,
  readyCount: number,
  checkingCount: number,
  unhealthyCount: number,
  suspendedCount: number
): string {
  return (
    '<div class="account-overview-metrics account-overview-metrics-header">' +
      '<div class="account-panel-kicker">Live status</div>' +
      '<div class="account-metric-strip">' +
        '<div class="account-stat-card account-stat-card-inline">' +
          '<span class="account-stat-label">Total</span>' +
          '<span class="account-stat-value">' + String(totalCount) + '</span>' +
        '</div>' +
        '<div class="account-stat-card account-stat-card-inline">' +
          '<span class="account-stat-label">Ready now</span>' +
          '<span class="account-stat-value account-stat-healthy">' + String(readyCount) + '</span>' +
        '</div>' +
        '<div class="account-stat-card account-stat-card-inline">' +
          '<span class="account-stat-label">Checking</span>' +
          '<span class="account-stat-value account-stat-checking">' + String(checkingCount) + '</span>' +
        '</div>' +
        '<div class="account-stat-card account-stat-card-inline">' +
          '<span class="account-stat-label">Needs attention</span>' +
          '<span class="account-stat-value account-stat-unhealthy">' + String(unhealthyCount) + '</span>' +
        '</div>' +
        '<div class="account-stat-card account-stat-card-inline">' +
          '<span class="account-stat-label">Suspended</span>' +
          '<span class="account-stat-value">' + String(suspendedCount) + '</span>' +
        '</div>' +
      '</div>' +
    '</div>'
  );
}

function renderAccountContextStrip(account: PortalAccountSummary): string {
  var workspaceLabel = workspaceCountLabel((account.workspaces || []).length);
  var billingLabel = account.has_billing ? 'Billing enabled' : 'Billing offline';

  return (
    '<section class="portal-account-context">' +
      '<div class="portal-account-context-copy">' +
        '<div class="portal-account-context-meta">' +
          '<span class="account-eyebrow">' + escapeHTML(accountKindLabel(account)) + '</span>' +
          '<span class="portal-account-context-separator">/</span>' +
          '<span class="portal-account-context-access">' + escapeHTML(titleCase(account.role)) + ' access</span>' +
        '</div>' +
        '<div class="portal-account-context-row">' +
          '<h2>' + escapeHTML(account.name) + '</h2>' +
          '<div class="portal-account-context-chips">' +
            '<span class="account-context-chip">' + escapeHTML(account.kind_label) + '</span>' +
            '<span class="account-context-chip">' + escapeHTML(titleCase(account.role)) + '</span>' +
            '<span class="account-context-chip">' + escapeHTML(workspaceLabel) + '</span>' +
          '</div>' +
        '</div>' +
        '<p>' + escapeHTML(account.kind === 'msp' ? 'Hosted workspace account for workspace access, access control, and billing.' : 'Hosted account for workspace access, access control, and billing.') + '</p>' +
      '</div>' +
      '<div class="portal-account-context-summary">' +
        '<div class="portal-account-context-stat">' +
          '<span>Access</span>' +
          '<strong>' + escapeHTML(titleCase(account.role)) + '</strong>' +
        '</div>' +
        '<div class="portal-account-context-stat">' +
          '<span>Workspaces</span>' +
          '<strong>' + escapeHTML(workspaceLabel) + '</strong>' +
        '</div>' +
        '<div class="portal-account-context-stat">' +
          '<span>Billing</span>' +
          '<strong>' + escapeHTML(billingLabel) + '</strong>' +
        '</div>' +
      '</div>' +
    '</section>'
  );
}

function shellSectionButton(section: PortalShellSection, activeSection: PortalShellSection, index: string, title: string, copy: string, badge?: string): string {
  var badgeHTML = badge
    ? '<span class="portal-shell-nav-badge">' + escapeHTML(badge) + '</span>'
    : '';
  return (
    '<button class="portal-shell-nav-link' + (activeSection === section ? ' active' : '') + '" type="button" data-shell-action="activate-section" data-shell-section="' + section + '">' +
      '<span class="portal-shell-nav-row">' +
        '<span class="portal-shell-nav-label-group">' +
          '<span class="portal-shell-nav-index">' + escapeHTML(index) + '</span>' +
          '<span class="portal-shell-nav-label">' + title + '</span>' +
        '</span>' +
        badgeHTML +
      '</span>' +
      '<span class="portal-shell-nav-copy">' + copy + '</span>' +
    '</button>'
  );
}

function renderShellNavigation(accounts: PortalAccountSummary[], supportEmail: string, activeSection: PortalShellSection): string {
  var hosted = hasHostedAccounts(accounts);
  var workspaces = collectWorkspaces(accounts);
  var totalWorkspaces = workspaces.length;
  var readyWorkspaces = countReadyWorkspaces(workspaces);
  var attentionCount = attentionWorkspaces(workspaces).length;
  var hostedBillingCount = 0;
  var canManage = false;
  for (var i = 0; i < accounts.length; i += 1) {
    if (accounts[i].can_manage) {
      canManage = true;
    }
    if (accounts[i].has_billing) {
      hostedBillingCount += 1;
    }
  }
  return (
    '<aside class="portal-shell-nav" aria-label="Pulse Account sections">' +
      '<div class="portal-shell-nav-header">' +
        '<div class="portal-shell-nav-eyebrow">Pulse Account</div>' +
        '<div class="portal-shell-nav-title">Account tasks</div>' +
        '<div class="portal-shell-nav-support">' + (hosted ? 'Start with the job you need to finish: workspace work, access, billing, then escalation.' : 'Use billing tools first and escalate only when the self-serve path stops.' ) + '</div>' +
      '</div>' +
      '<div class="portal-shell-nav-group">' +
        shellSectionButton('overview', activeSection, '01', 'Overview', 'What needs attention, what is ready, and the next obvious action.', attentionCount > 0 ? String(attentionCount) + ' review' : (hosted ? String(readyWorkspaces) + ' ready' : 'Summary')) +
        shellSectionButton('workspaces', activeSection, '02', 'Workspaces', hosted ? 'Open a workspace, review lifecycle state, or create one.' : 'No hosted workspaces are attached yet.', hosted ? String(readyWorkspaces) + ' ready' : 'None') +
        shellSectionButton('access', activeSection, '03', 'Access', hosted ? 'Invite people, change roles, and remove account access.' : 'Account access and membership controls.', canManage ? 'Manage' : 'View') +
        shellSectionButton('billing', activeSection, '04', 'Billing', hostedBillingCount > 0 ? 'Hosted billing first, then self-hosted licenses, refunds, and privacy only when relevant.' : 'Self-hosted billing, licenses, refunds, and privacy.', hostedBillingCount > 0 ? (hostedBillingCount > 1 ? 'Hosted +' : 'Hosted') : 'Self-hosted') +
        shellSectionButton('support', activeSection, '05', 'Support', 'Escalation only after the workspace, access, or billing path is exhausted.', supportEmail ? 'Email' : 'Help') +
      '</div>' +
    '</aside>'
  );
}

function renderWorkspaceCard(account: PortalAccountSummary, workspace: PortalWorkspaceSummary, accountAPIBasePath: string): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  var createdLabel = formatWorkspaceDate(workspace.created_at);
  var metaParts = [];
  if (createdLabel) {
    metaParts.push('<span class="workspace-meta-item">Created ' + escapeHTML(createdLabel) + '</span>');
  }
  if (workspace.last_health_check && status === 'healthy') {
    metaParts.push('<span class="workspace-meta-item">Checked recently</span>');
  }

  var openAction = '';
  if (state === 'active') {
    openAction =
      '<form method="POST" action="' +
      escapeAttr(accountAPIBasePath + '/' + account.id + '/tenants/' + workspace.id + '/handoff') +
      '">' +
      '<button type="submit" class="btn-primary">Open workspace</button>' +
      '</form>';
  }

  var manageAction = '';
  if (account.can_manage && (state === 'active' || state === 'suspended' || state === 'failed')) {
    manageAction =
      '<button type="button" class="btn-secondary btn-workspace-manage" data-action="select-workspace" data-account-id="' +
      escapeAttr(account.id) +
      '" data-workspace-id="' +
      escapeAttr(workspace.id) +
      '">Lifecycle</button>';
  }

  return (
    '<article class="workspace-row workspace-row-health-' + escapeAttr(status) + ' workspace-row-state-' + escapeAttr(state || 'unknown') + '" data-workspace-row="' + escapeAttr(workspace.id) + '">' +
      '<div class="workspace-row-primary">' +
        '<div class="workspace-row-heading">' +
          '<h4 class="workspace-name">' + escapeHTML(workspace.display_name) + '</h4>' +
          '<div class="workspace-meta">' + metaParts.join('') + '</div>' +
        '</div>' +
        '<div class="workspace-row-note">' + escapeHTML(workspaceRowNote(workspace)) + '</div>' +
      '</div>' +
      '<div class="workspace-row-status-cell workspace-row-status-cell-badge">' +
        healthBadgeHTML(workspace) +
      '</div>' +
      '<div class="workspace-row-status-cell workspace-row-status-cell-badge">' +
        '<span class="badge badge-' + escapeHTML(state || 'unknown') + '">' + escapeHTML(titleCase(state || 'Unknown')) + '</span>' +
      '</div>' +
      '<div class="workspace-actions">' +
        openAction +
        manageAction +
      '</div>' +
    '</article>'
  );
}

function readyWorkspaces(workspaces: PortalWorkspaceSummary[]): PortalWorkspaceSummary[] {
  var results: PortalWorkspaceSummary[] = [];
  for (var i = 0; i < workspaces.length; i += 1) {
    if (String(workspaces[i].state || '') === 'active' && workspaceHealthState(workspaces[i]) === 'healthy') {
      results.push(workspaces[i]);
    }
  }
  return results;
}

function renderWorkspaceHandoffForm(accountID: string, workspaceID: string, accountAPIBasePath: string, label: string, buttonClassName = 'btn-secondary btn-compact'): string {
  if (!accountAPIBasePath) {
    return '<button class="' + escapeAttr(buttonClassName) + '" type="button" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(label) + '</button>';
  }
  return (
    '<form method="POST" action="' +
    escapeAttr(accountAPIBasePath + '/' + accountID + '/tenants/' + workspaceID + '/handoff') +
    '">' +
    '<button type="submit" class="' + escapeAttr(buttonClassName) + '">' + escapeHTML(label) + '</button>' +
    '</form>'
  );
}

function renderOverviewAttentionCard(workspaces: PortalWorkspaceSummary[]): string {
  var attention = attentionWorkspaces(workspaces);
  if (!attention.length) {
    return (
      '<article class="overview-task-card">' +
        '<div class="account-panel-kicker">Needs attention</div>' +
        '<h4>Nothing urgent</h4>' +
        '<p>No active workspace is currently asking for review.</p>' +
        '<div class="overview-task-list">' +
          '<div class="overview-task-item"><strong>Healthy now</strong><span>Active workspaces look clear for routine use.</span></div>' +
          '<div class="overview-task-item"><strong>Suspended stays parked</strong><span>Suspended workspaces stay out of the way until you deliberately resume them.</span></div>' +
        '</div>' +
      '</article>'
    );
  }

  return (
    '<article class="overview-task-card overview-task-card-attention">' +
      '<div class="account-panel-kicker">Needs attention</div>' +
      '<h4>Review these first</h4>' +
      '<p>These workspaces still need a human check before you treat the account as settled.</p>' +
      '<div class="overview-task-list">' +
        attention.slice(0, 3).map(function(workspace) {
          return (
            '<div class="overview-task-item">' +
              '<strong>' + escapeHTML(workspace.display_name) + '</strong>' +
              '<span>' + escapeHTML(workspaceStatusCopy(workspace)) + '</span>' +
            '</div>'
          );
        }).join('') +
      '</div>' +
    '</article>'
  );
}

function renderOverviewReadyCard(account: PortalAccountSummary, workspaces: PortalWorkspaceSummary[], accountAPIBasePath: string): string {
  var ready = readyWorkspaces(workspaces);
  if (!ready.length) {
    return (
      '<article class="overview-task-card">' +
        '<div class="account-panel-kicker">Ready</div>' +
        '<h4>No workspace is ready yet</h4>' +
        '<p>Use Workspaces to review current state before you start routine work.</p>' +
      '</article>'
    );
  }

  return (
    '<article class="overview-task-card">' +
      '<div class="account-panel-kicker">Ready</div>' +
      '<h4>Open and work</h4>' +
      '<p>These workspaces are active and healthy right now.</p>' +
      '<div class="overview-task-list">' +
        ready.slice(0, 3).map(function(workspace) {
          return (
            '<div class="overview-task-item overview-task-item-action">' +
              '<div class="overview-task-copy">' +
                '<strong>' + escapeHTML(workspace.display_name) + '</strong>' +
                '<span>' + escapeHTML(workspaceRowNote(workspace)) + '</span>' +
              '</div>' +
              renderWorkspaceHandoffForm(account.id, workspace.id, accountAPIBasePath, 'Open workspace') +
            '</div>'
          );
        }).join('') +
      '</div>' +
    '</article>'
  );
}

function renderOverviewNextActionCard(account: PortalAccountSummary, workspaces: PortalWorkspaceSummary[], accountAPIBasePath: string): string {
  var attention = attentionWorkspaces(workspaces);
  var ready = readyWorkspaces(workspaces);
  var primaryAction = '';
  var secondaryAction = '';
  var title = '';
  var description = '';

  if (attention.length) {
    title = 'Review workspace health';
    description = 'Open Workspaces and resolve the pending health or lifecycle questions before you do anything else.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">Review workspaces</button>';
    secondaryAction = account.can_manage
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Review access</button>'
      : '';
  } else if (ready.length) {
    title = 'Open the next workspace';
    description = 'The most obvious next step is to open a ready workspace and continue the actual work there.';
    primaryAction = renderWorkspaceHandoffForm(account.id, ready[0].id, accountAPIBasePath, 'Open ' + ready[0].display_name, 'btn-primary btn-compact');
    secondaryAction = ready.length > 1
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">See all workspaces</button>'
      : '';
  } else if (account.kind === 'msp' && account.can_manage) {
    title = 'Create the next workspace';
    description = 'There is no ready workspace yet, so the next clear action is to create one from the Workspaces section.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(account.id) + '">Create workspace</button>';
    secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Manage access</button>';
  } else if (account.has_billing && account.can_manage) {
    title = 'Handle billing in its own place';
    description = 'Operational work is clear, so the next separate task is billing if that is what you came here to change.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
    secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Review access</button>';
  } else {
    title = 'Choose the right task path';
    description = 'If this is an access change, go to Access. If it is a billing or license issue, go to Billing. Support is only for escalation.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="' + (account.can_manage ? 'access' : 'billing') + '">' + (account.can_manage ? 'Open access' : 'Open billing') + '</button>';
    secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="support">Escalate</button>';
  }

  return (
    '<article class="overview-task-card overview-task-card-next">' +
      '<div class="account-panel-kicker">Next action</div>' +
      '<h4>' + escapeHTML(title) + '</h4>' +
      '<p>' + escapeHTML(description) + '</p>' +
      '<div class="overview-task-actions">' +
        primaryAction +
        secondaryAction +
      '</div>' +
    '</article>'
  );
}

function renderAccountOverviewSection(account: PortalAccountSummary, accountAPIBasePath: string): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var totalCount = workspaces.length;
  var readyCount = countReadyWorkspaces(workspaces);
  var checkingCount = countWorkspacesByHealth(workspaces, 'checking');
  var unhealthyCount = countWorkspacesByHealth(workspaces, 'unhealthy');
  var suspendedCount = countWorkspacesByState(workspaces, 'suspended');
  var attentionCount = attentionWorkspaces(workspaces).length;

  return (
    '<section class="account-content-panel account-content-panel-overview">' +
      '<div class="account-stage-header account-stage-header-overview overview-stage-header">' +
        '<div>' +
          '<div class="account-panel-kicker">Overview</div>' +
          '<h3>Account brief</h3>' +
          '<p>Start with the immediate question, not the platform model.</p>' +
          renderSectionContextChips([
            String(attentionCount) + ' attention',
            String(readyCount) + ' ready',
            String(totalCount) + ' total',
            suspendedCount > 0 ? String(suspendedCount) + ' suspended' : 'No suspended',
          ]) +
        '</div>' +
      '</div>' +
      '<div class="overview-task-grid">' +
        renderOverviewAttentionCard(workspaces) +
        renderOverviewReadyCard(account, workspaces, accountAPIBasePath) +
        renderOverviewNextActionCard(account, workspaces, accountAPIBasePath) +
      '</div>' +
      '<div class="overview-task-summary-bar">' +
        '<div class="overview-task-summary-pill"><strong>Healthy</strong><span>' + String(countWorkspacesByHealth(workspaces, 'healthy')) + '</span></div>' +
        '<div class="overview-task-summary-pill"><strong>Checking</strong><span>' + String(checkingCount) + '</span></div>' +
        '<div class="overview-task-summary-pill"><strong>Needs attention</strong><span>' + String(unhealthyCount) + '</span></div>' +
        '<div class="overview-task-summary-pill"><strong>Suspended</strong><span>' + String(suspendedCount) + '</span></div>' +
      '</div>' +
    '</section>'
  );
}

function renderAccountWorkspaceSection(account: PortalAccountSummary, accountAPIBasePath: string): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var readyCount = countReadyWorkspaces(workspaces);
  var suspendedCount = countWorkspacesByState(workspaces, 'suspended');
  var workspaceManagement = '';
  var addWorkspaceForm = '';
  var workspaceHeaderActions = '';
  if (account.can_manage) {
    if (account.kind === 'msp') {
      workspaceHeaderActions +=
        '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' +
        escapeAttr(account.id) +
        '">Add workspace</button>';
    }

    var workspaceDeskActions = '';
    if (account.kind === 'msp') {
      workspaceDeskActions +=
        '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' +
        escapeAttr(account.id) +
        '">Add workspace</button>';
      addWorkspaceForm =
        '<div class="add-workspace-form" id="add-ws-form-' +
        escapeAttr(account.id) +
        '">' +
          '<label for="ws-name-' +
          escapeAttr(account.id) +
          '">Workspace name (for example, a client name)</label>' +
          '<input type="text" id="ws-name-' +
          escapeAttr(account.id) +
          '" placeholder="Acme Corp" maxlength="80" autocomplete="off">' +
          '<div class="form-actions">' +
            '<button type="button" class="btn-primary" data-action="create-workspace" data-account-id="' +
            escapeAttr(account.id) +
            '">Create workspace</button>' +
            '<button type="button" class="btn-secondary" data-action="toggle-add-workspace" data-account-id="' +
            escapeAttr(account.id) +
            '">Cancel</button>' +
            '<div class="spinner" id="ws-spinner-' +
            escapeAttr(account.id) +
            '" hidden></div>' +
          '</div>' +
        '</div>';
    }
    workspaceManagement =
      '<section class="workspace-management-panel" id="workspace-management-' +
      escapeAttr(account.id) +
      '">' +
        '<div class="workspace-management-header">' +
          '<div>' +
            '<div class="account-panel-kicker">Workspace management</div>' +
            '<h3>Lifecycle</h3>' +
            '<p>Inspect one workspace at a time and keep account-level actions separate.</p>' +
          '</div>' +
          '<button type="button" class="btn-secondary btn-compact" id="workspace-management-close-' +
          escapeAttr(account.id) +
          '" data-action="clear-workspace-selection" data-account-id="' +
          escapeAttr(account.id) +
          '">Close panel</button>' +
        '</div>' +
        '<div class="workspace-management-empty" id="workspace-management-empty-' +
        escapeAttr(account.id) +
        '">' +
          '<div class="workspace-management-empty-copy">Pick one workspace for lifecycle review. Keep access and billing changes in their own sections.</div>' +
          '<div class="workspace-management-empty-shell">' +
            '<div class="workspace-management-empty-actions-card">' +
              '<div class="workspace-management-empty-actions-copy">' +
                '<div class="account-panel-kicker">Workspace tasks</div>' +
                '<h4>Keep this section workspace-only</h4>' +
                '<p>Create a workspace here when you need one. Access changes belong in Access, and billing changes belong in Billing.</p>' +
              '</div>' +
              '<div class="workspace-management-empty-actions">' + workspaceDeskActions + '</div>' +
              addWorkspaceForm +
            '</div>' +
            '<div class="workspace-management-empty-rules">' +
              '<div class="workspace-management-empty-rule"><strong>Inspect status</strong><span>Open the workspace first and confirm whether it is routine work, review work, or a parked suspended system.</span></div>' +
              '<div class="workspace-management-empty-rule"><strong>Confirm lifecycle</strong><span>Check active, checking, failed, or suspended state before you take the next step.</span></div>' +
              '<div class="workspace-management-empty-rule"><strong>Stay deliberate</strong><span>Review one workspace at a time instead of mixing fleet and account actions together.</span></div>' +
            '</div>' +
          '</div>' +
        '</div>' +
        '<div class="workspace-management-content" id="workspace-management-content-' +
        escapeAttr(account.id) +
        '" hidden>' +
          '<div class="workspace-management-meta" id="workspace-management-meta-' +
          escapeAttr(account.id) +
          '"></div>' +
          '<h4 id="workspace-management-title-' +
          escapeAttr(account.id) +
          '"></h4>' +
          '<p class="workspace-management-summary" id="workspace-management-summary-' +
          escapeAttr(account.id) +
          '"></p>' +
          '<div class="workspace-management-facts">' +
            '<div class="workspace-management-fact">' +
              '<span>Health</span>' +
              '<strong id="workspace-management-health-' + escapeAttr(account.id) + '"></strong>' +
            '</div>' +
            '<div class="workspace-management-fact">' +
              '<span>Lifecycle</span>' +
              '<strong id="workspace-management-lifecycle-' + escapeAttr(account.id) + '"></strong>' +
            '</div>' +
            '<div class="workspace-management-fact">' +
              '<span>Created</span>' +
              '<strong id="workspace-management-created-' + escapeAttr(account.id) + '"></strong>' +
            '</div>' +
          '</div>' +
          '<div class="workspace-management-guidance" id="workspace-management-guidance-' +
          escapeAttr(account.id) +
          '"></div>' +
          '<div class="workspace-management-actions">' +
            '<button type="button" class="btn-danger" id="workspace-management-action-' +
            escapeAttr(account.id) +
            '" data-action="workspace-action" data-account-id="' +
            escapeAttr(account.id) +
            '">Manage workspace</button>' +
          '</div>' +
        '</div>' +
      '</section>';

  }

  var workspaceHTML = workspaces.length
    ? '<div class="workspace-list-wrap">' +
          '<div class="workspace-list-toolbar">' +
            '<div class="workspace-list-summary">Open a workspace to work in it. Use the lifecycle view only when you are reviewing state or making account-level changes.</div>' +
        '</div>' +
        '<div class="workspace-list-head">' +
          '<span>Workspace</span>' +
          '<span>Health</span>' +
          '<span>Lifecycle</span>' +
          '<span>Actions</span>' +
        '</div>' +
        '<div class="workspace-list">' + workspaces.map(function(workspace) {
        return renderWorkspaceCard(account, workspace, accountAPIBasePath);
      }).join('') + '</div>' +
      '</div>'
    : '<div class="empty-state"><p>No hosted workspaces yet. Create one to get started.</p></div>';

  return (
    '<section class="account-content-panel account-content-panel-workspaces">' +
      '<div class="account-stage-header">' +
        '<div class="account-stage-header-row">' +
          '<div>' +
            '<div class="account-panel-kicker">Workspaces</div>' +
            '<h3>Workspaces</h3>' +
            '<p>Open a workspace, review lifecycle state, or create a new one without mixing in access or billing work.</p>' +
            renderSectionContextChips([
              String(workspaces.length) + ' total',
              String(readyCount) + ' ready',
              String(suspendedCount) + ' suspended',
            ]) +
          '</div>' +
          '<div class="account-stage-header-actions">' + workspaceHeaderActions + '</div>' +
        '</div>' +
      '</div>' +
      '<div class="workspace-operations-shell workspace-operations-shell-idle" id="workspace-operations-shell-' +
        escapeAttr(account.id) +
        '">' +
        '<div class="workspace-operations-main">' +
          workspaceHTML +
        '</div>' +
        '<div class="workspace-operations-detail" id="workspace-operations-detail-' +
          escapeAttr(account.id) +
          '">' +
          workspaceManagement +
        '</div>' +
      '</div>' +
    '</section>'
  );
}

function renderAccountAccessSection(account: PortalAccountSummary): string {
  var accessPolicy =
    '<div class="access-policy-panel">' +
      '<div class="access-panel-heading">' +
        '<h4>Role rules</h4>' +
        '<p>Invite the smallest role that still lets someone do the work they actually own on this account.</p>' +
      '</div>' +
      '<div class="access-policy-list">' +
        '<div class="access-policy-row"><strong>Owner</strong><span>Billing, access control, and full account control.</span></div>' +
        '<div class="access-policy-row"><strong>Admin</strong><span>Workspace control plus billing for the account.</span></div>' +
        '<div class="access-policy-row"><strong>Tech</strong><span>Workspace control without billing ownership.</span></div>' +
        '<div class="access-policy-row"><strong>Read-only</strong><span>Workspace review and verification without control-plane changes.</span></div>' +
      '</div>' +
    '</div>';
  var reviewDesk =
    '<div class="access-review-shell">' +
      '<div class="access-review-strip">' +
        '<div class="access-panel-heading access-panel-heading-tight">' +
          '<div class="account-panel-kicker">Access review</div>' +
          '<h4>Keep access explicit</h4>' +
          '<p>Use the roster as a controlled access list, not a vague shared bucket.</p>' +
        '</div>' +
        '<div class="access-review-grid">' +
            '<div class="access-review-card">' +
              '<strong>Owners stay rare</strong>' +
            '<span>Reserve Owner for billing, access control, and full account control. Default to Admin, Tech, or Read-only first.</span>' +
            '</div>' +
          '<div class="access-review-card">' +
            '<strong>Keep access narrow</strong>' +
            '<span>Use Tech for workspace control and Read-only for verification instead of handing out broader access.</span>' +
          '</div>' +
          '<div class="access-review-card">' +
            '<strong>Remove stale access fast</strong>' +
            '<span>If someone no longer owns the work, remove them instead of leaving dormant access attached to the account.</span>' +
          '</div>' +
        '</div>' +
      '</div>' +
    '</div>';

  return (
    '<section class="account-content-panel account-content-panel-access">' +
      '<section class="access-management-panel access-section access-section-shell" id="access-section-' +
      escapeAttr(account.id) +
      '" data-actor-role="' +
      escapeAttr(account.role) +
      '">' +
        '<div class="access-management-header">' +
          '<div>' +
            '<div class="account-panel-kicker">Access</div>' +
            '<h3>People and roles</h3>' +
            '<p>Invite people, change roles, and remove stale access without mixing that work into workspace operations.</p>' +
            renderSectionContextChips([
              account.can_manage ? 'Managed roster' : 'View only',
              'Invite',
              'Roles',
              'Remove access',
            ]) +
          '</div>' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">Back to workspaces</button>' +
        '</div>' +
        '<div class="access-management-stats" id="access-stats-' +
        escapeAttr(account.id) +
        '"></div>' +
        '<div class="access-management-grid">' +
          '<div class="access-roster-column">' +
            '<div class="access-roster">' +
              '<div class="access-panel-heading">' +
                '<h4>People on this account</h4>' +
                '<p>Every person here can reach this hosted account. Keep the list small and the role choice explicit.</p>' +
              '</div>' +
              '<div class="access-roster-list" id="access-list-' +
              escapeAttr(account.id) +
              '">' +
                '<div class="access-list-message">Loading…</div>' +
              '</div>' +
            '</div>' +
            reviewDesk +
          '</div>' +
          '<div class="access-side-column">' +
            '<div class="access-operations-panel">' +
              '<div class="access-panel-heading access-panel-heading-tight">' +
                '<div class="account-panel-kicker">Access controls</div>' +
                '<h4>Invite, role, remove</h4>' +
                '<p>Use this column when you need to add someone, tighten a role, or remove access that no longer belongs here.</p>' +
              '</div>' +
              '<div class="access-operations-grid">' +
                '<div class="access-invite-panel">' +
                  '<div class="access-panel-heading">' +
                    '<h4>Invite someone new</h4>' +
                    '<p>Add a person with the minimum role they need for this account.</p>' +
                  '</div>' +
                  '<div class="access-invite">' +
                    '<div><label for="invite-email-' +
                    escapeAttr(account.id) +
                    '">Email</label><input type="email" id="invite-email-' +
                    escapeAttr(account.id) +
                    '" placeholder="user@example.com" autocomplete="off"></div>' +
                    '<div><label for="invite-role-' +
                    escapeAttr(account.id) +
                    '">Role</label><select id="invite-role-' +
                    escapeAttr(account.id) +
                    '"><option value="admin">Admin</option><option value="tech">Tech</option><option value="read_only">Read-only</option></select></div>' +
                    '<button type="button" class="btn-primary btn-compact" data-action="invite-member" data-account-id="' +
                    escapeAttr(account.id) +
                    '">Invite</button>' +
                  '</div>' +
                '</div>' +
                accessPolicy +
              '</div>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</section>' +
    '</section>'
  );
}

function renderHostedBillingCards(accounts: PortalAccountSummary[]): string {
  var hostedBillingAccounts = accounts.filter(function(account) {
    return account.has_billing;
  });
  if (!hostedBillingAccounts.length) {
    return (
      '<div class="billing-task-card billing-task-card-muted">' +
        '<div class="account-panel-kicker">Hosted billing</div>' +
        '<h3>No hosted billing attached</h3>' +
        '<p>Use the self-hosted billing tools below only if you are managing a self-hosted purchase.</p>' +
      '</div>'
    );
  }

  return hostedBillingAccounts.map(function(account) {
    var actionHTML = account.can_manage
      ? '<button type="button" class="btn-primary btn-compact" data-action="open-billing" data-account-id="' + escapeAttr(account.id) + '">Open hosted billing</button>'
      : '<div class="billing-task-note">An owner or admin on this account needs to open hosted billing.</div>';
    return (
      '<article class="billing-task-card">' +
        '<div class="account-panel-kicker">Hosted billing</div>' +
        '<h3>' + escapeHTML(account.name) + '</h3>' +
        '<p>Invoices, payment methods, and hosted subscription changes for this account live here.</p>' +
        '<div class="billing-task-points">' +
          '<div class="billing-task-point"><strong>Use when hosted billing is the job</strong><span>Keep workspace lifecycle work in Workspaces and access changes in Access.</span></div>' +
          '<div class="billing-task-point"><strong>Stay account-specific</strong><span>Open billing from the exact hosted account you want to change.</span></div>' +
        '</div>' +
        '<div class="billing-task-actions">' + actionHTML + '</div>' +
      '</article>'
    );
  }).join('');
}

function renderSupportSection(context: ShellViewContext): string {
  return (
    '<section class="portal-support-panel">' +
      '<div class="account-panel-kicker">Support</div>' +
      '<h2>Escalation</h2>' +
      '<p>Come here only after the workspace, access, or billing path has stopped you.</p>' +
      renderSectionContextChips(['Escalation only', 'Bring context', context.bootstrap.support_email ? 'Email' : 'Support']) +
      '<div class="portal-support-layout">' +
        '<div class="portal-support-route-grid">' +
          '<div class="portal-support-route-card">' +
            '<div class="account-panel-kicker">Hosted account</div>' +
            '<h3>Hosted problems</h3>' +
            '<p>Use support for hosted issues only after you have already tried the Workspaces or Access path.</p>' +
            '<div class="portal-support-points">' +
              '<div class="portal-support-point"><strong>Use after the hosted path fails</strong><span>Workspace access, lifecycle, and account access should already be narrowed before you escalate.</span></div>' +
              '<div class="portal-support-point"><strong>Keep the account context intact</strong><span>Include the account, workspace, and action that failed so support can pick up the same issue quickly.</span></div>' +
            '</div>' +
            '<div class="portal-support-actions">' +
              '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">Open workspaces</button>' +
              '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">Open access</button>' +
              '<a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || '') + '">' + escapeHTML(context.bootstrap.support_email || '') + '</a>' +
            '</div>' +
          '</div>' +
          '<div class="portal-support-route-card">' +
            '<div class="account-panel-kicker">Billing</div>' +
            '<h3>Billing and self-hosted issues</h3>' +
            '<p>Use this route only after hosted billing or the self-hosted billing tools have failed to complete the request cleanly.</p>' +
            '<div class="portal-support-points">' +
              '<div class="portal-support-point"><strong>Start in Billing</strong><span>Hosted billing, licenses, refunds, and privacy should already be narrowed to one failed request.</span></div>' +
              '<div class="portal-support-point"><strong>Escalate from the same path</strong><span>Bring the billing tool or hosted billing action that failed instead of reopening the story from scratch.</span></div>' +
            '</div>' +
            '<div class="portal-support-actions">' +
              '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>' +
              '<a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || '') + '">' + escapeHTML(context.bootstrap.support_email || '') + '</a>' +
            '</div>' +
          '</div>' +
        '</div>' +
        '<div class="portal-support-runbook">' +
          '<div class="account-panel-kicker">Escalation packet</div>' +
          '<h3>What to send</h3>' +
          '<p>Keep the packet short and concrete so support can pick up the exact failed path immediately.</p>' +
          '<div class="portal-support-runbook-list">' +
            '<div class="portal-support-runbook-step"><strong>1. Name the path</strong><span>Say whether the failed path was Workspaces, Access, or Billing.</span></div>' +
            '<div class="portal-support-runbook-step"><strong>2. Name the account</strong><span>Include the hosted account and workspace when relevant, or the billing email for self-hosted work.</span></div>' +
            '<div class="portal-support-runbook-step"><strong>3. Name the action</strong><span>Include the exact button, form, or billing tool that failed and what happened next.</span></div>' +
          '</div>' +
        '</div>' +
      '</div>' +
    '</section>'
  );
}

export function renderHeaderHTML(context: ShellViewContext): string {
  if (context.bootstrap.authenticated) {
    return (
      '<div class="header-account-chip">' +
        '<span class="header-account-email">' + escapeHTML(context.bootstrap.email || '') + '</span>' +
        '<button class="logout-btn" id="logout-btn" type="button">Sign out</button>' +
      '</div>'
    );
  }
  return '<a class="logout-btn link-button" href="' + escapeAttr(context.signupPath) + '">Create account</a>';
}

export function renderAccountsHTML(context: ShellViewContext): string {
  var safeAccounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  if (safeAccounts.length === 0) {
    return (
      '<div class="empty-state empty-state-spaced">' +
        '<p>No hosted workspaces are attached to this account.</p>' +
        '<p class="support-copy">You can still use the self-hosted licensing and billing tools below.</p>' +
        '<p class="support-copy">Need help? Contact <a href="mailto:' +
        escapeAttr(context.bootstrap.support_email || '') +
        '" class="support-link">' +
        escapeHTML(context.bootstrap.support_email || '') +
        '</a></p>' +
      '</div>'
    );
  }
  return safeAccounts.map(function(account) {
    return renderAccountOverviewSection(account, context.accountAPIBasePath);
  }).join('');
}

export function renderAuthenticatedPortalHTML(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var hosted = hasHostedAccounts(accounts);
  var activeSection = context.activeSection || 'overview';
  var hostedBillingCount = accounts.filter(function(account) {
    return account.has_billing;
  }).length;
  var billingNote = hosted
    ? 'Hosted billing lives here when it applies. Self-hosted licenses, refunds, and privacy stay separate underneath it.'
    : 'Use this billing surface for self-hosted subscriptions, licenses, refunds, and privacy requests.';
  var hostedContent = accounts.map(function(account) {
    return (
      '<section class="account-surface">' +
        '<div class="account-surface-body">' +
          renderAccountOverviewSection(account, context.accountAPIBasePath) +
          renderAccountWorkspaceSection(account, context.accountAPIBasePath) +
          renderAccountAccessSection(account) +
        '</div>' +
      '</section>'
    );
  }).join('');

  return (
    '<div class="portal-shell" data-shell-section="' + activeSection + '">' +
      '<div class="portal-shell-layout">' +
        renderShellNavigation(accounts, context.bootstrap.support_email || '', activeSection) +
        '<div class="portal-shell-main">' +
          (accounts.length === 1 ? renderAccountContextStrip(accounts[0]) : '') +
          '<section class="portal-content-panel portal-content-panel-overview">' +
            '<div id="accounts-root">' + hostedContent + '</div>' +
          '</section>' +
          '<section class="portal-content-panel portal-content-panel-billing billing-section" id="billing-section">' +
            '<div class="billing-header">' +
              '<div>' +
                '<div class="account-panel-kicker">Billing</div>' +
                '<h2>Billing</h2>' +
                renderSectionContextChips([
                  hostedBillingCount > 0 ? 'Hosted billing' : 'No hosted billing',
                  'Self-hosted tools',
                ]) +
              '</div>' +
              '<div class="billing-note">' + billingNote + '</div>' +
            '</div>' +
            '<div class="billing-overview-grid">' + renderHostedBillingCards(accounts) + '</div>' +
            '<div class="billing-shell">' +
              '<aside class="billing-shell-sidebar">' +
                '<div class="billing-shell-sidebar-head">' +
                  '<div class="account-panel-kicker">Self-hosted billing</div>' +
                  '<h3>Self-hosted tools</h3>' +
                  '<p>Use these only when the job is a self-hosted purchase, license, refund, or privacy request.</p>' +
                '</div>' +
                '<div class="billing-action-list">' +
                  renderBillingActionRow('open-manage-billing', 'Self-hosted billing', 'Manage subscriptions', 'Billing', 'Open Stripe for self-hosted plan, invoice, and payment changes.', 'manage-billing-panel', 'manage-inline-email', ['Plan changes', 'Invoices']) +
                  renderBillingActionRow('open-retrieve-billing', 'Licenses', 'Retrieve licenses', 'Licenses', 'Recover the latest active self-hosted license and invoice link.', 'retrieve-billing-panel', 'retrieve-inline-email', ['Latest active license', 'Invoice lookup']) +
                  renderBillingActionRow('open-refund-billing', 'Refunds', 'Refund requests', 'Refunds', 'Request a self-serve refund when the purchase is still eligible.', 'refund-billing-panel', 'refund-inline-email', ['Eligibility check', 'Revocation']) +
                  renderBillingActionRow('open-data-billing', 'Privacy', 'Data and privacy', 'Privacy', 'Request export or deletion for commercial account data.', 'data-billing-panel', 'data-export-email', ['Export', 'Deletion']) +
                '</div>' +
                '<div class="billing-inline-support">' +
                  '<div class="account-panel-kicker">Escalation</div>' +
                  '<h4>Keep the billing request contained</h4>' +
                  '<p>Use Support only when a hosted billing or self-hosted billing request cannot complete cleanly from this section.</p>' +
                  '<div class="billing-inline-support-points">' +
                    '<div class="billing-inline-support-point"><strong>Hosted billing first when present</strong><span>Use the hosted billing cards above before you touch the self-hosted tools.</span></div>' +
                    '<div class="billing-inline-support-point"><strong>Escalate with context</strong><span>Bring the billing tool or hosted billing action and the exact failed step if you need support.</span></div>' +
                  '</div>' +
                  '<div class="billing-inline-support-actions">' +
                    '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="support">Open support</button>' +
                  '</div>' +
                '</div>' +
              '</aside>' +
              '<div class="billing-shell-main">' +
                '<div class="billing-detail-shell">' +
                  '<div class="billing-panel billing-panel-empty visible" id="billing-panel-empty">' +
                    '<div class="account-panel-kicker">Billing brief</div>' +
                    '<h3>Choose the billing task</h3>' +
                    '<p>Pick one self-hosted billing request at a time. Verification happens first so billing, license, refund, or privacy work stays contained.</p>' +
                    '<div class="billing-empty-shell">' +
                      '<div class="billing-empty-primary">' +
                        '<div class="billing-empty-section billing-empty-section-compact">' +
                          '<div class="billing-empty-column-title">What each tool does</div>' +
                          '<div class="billing-empty-flow-list">' +
                            '<div class="billing-empty-flow"><strong>Billing</strong><span>Stripe customer portal access for self-hosted invoices, payment methods, and plan changes.</span></div>' +
                            '<div class="billing-empty-flow"><strong>Licenses</strong><span>Recover the latest active self-hosted license and the matching invoice link.</span></div>' +
                            '<div class="billing-empty-flow"><strong>Refunds</strong><span>Check eligibility before revoking active commercial access.</span></div>' +
                            '<div class="billing-empty-flow"><strong>Privacy</strong><span>Request export or deletion without leaving Pulse Account.</span></div>' +
                          '</div>' +
                        '</div>' +
                        '<div class="billing-empty-section billing-empty-section-compact">' +
                          '<div class="billing-empty-column-title">Before you start</div>' +
                          '<div class="billing-empty-points billing-empty-points-stack">' +
                            '<div class="billing-empty-point"><strong>One request</strong><span>Keep a single billing request active instead of bouncing across sections.</span></div>' +
                            '<div class="billing-empty-point"><strong>Identity first</strong><span>Verification happens before any sensitive account action opens.</span></div>' +
                            '<div class="billing-empty-point"><strong>Stay focused</strong><span>Use the hosted billing cards above when the request is tied to a hosted workspace account.</span></div>' +
                          '</div>' +
                        '</div>' +
                      '</div>' +
                      '<div class="billing-empty-side">' +
                        '<div class="billing-empty-section billing-empty-section-support">' +
                          '<div class="billing-empty-column-title">Escalation</div>' +
                          '<div class="billing-empty-checklist">' +
                            '<div class="billing-empty-check"><strong>Escalate quickly</strong><span>If hosted billing or the self-hosted tools do not behave as expected, escalate from this section.</span></div>' +
                            '<div class="billing-empty-check"><strong>Billing packet</strong><span>Bring the billing tool, commercial email, and the exact failed action so support starts with the same request state.</span></div>' +
                          '</div>' +
                          '<div class="billing-empty-support">Need help with billing, refund, privacy, or license requests? <a class="portal-support-link" href="mailto:' +
                          escapeAttr(context.bootstrap.support_email || '') +
                          '">' +
                          escapeHTML(context.bootstrap.support_email || '') +
                          '</a></div>' +
                          '<div class="billing-empty-actions">' +
                            '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="support">Open support</button>' +
                          '</div>' +
                        '</div>' +
                      '</div>' +
                    '</div>' +
                  '</div>' +
                  '<div class="billing-panel" id="manage-billing-panel"><div id="manage-billing-root"></div></div>' +
                  '<div class="billing-panel" id="retrieve-billing-panel"><div id="retrieve-billing-root"></div></div>' +
                  '<div class="billing-panel" id="refund-billing-panel"><div id="refund-billing-root"></div></div>' +
                  '<div class="billing-panel" id="data-billing-panel">' +
                    '<h3>Data and privacy</h3>' +
                    '<p>Request export or deletion of the commercial data tied to an email address. Payment data held directly by Stripe still requires support handling.</p>' +
                    '<div class="subsection"><div id="data-export-root"></div></div>' +
                    '<div class="subsection"><div id="data-delete-root"></div></div>' +
                    '<div class="helper-text">Payment-card data stays with Stripe. For Stripe deletion support, contact <a href="mailto:' +
                    escapeAttr(context.bootstrap.support_email || '') +
                    '">' +
                    escapeHTML(context.bootstrap.support_email || '') +
                    '</a>.</div>' +
                  '</div>' +
                '</div>' +
              '</div>' +
            '</div>' +
          '</section>' +
          '<section class="portal-content-panel portal-content-panel-support">' +
            renderSupportSection(context) +
          '</section>' +
        '</div>' +
      '</div>' +
    '</div>'
  );
}

export function renderSignedOutPortalHTML(context: ShellViewContext): string {
  var statusHTML = '';
  if (context.loginState.request.error) {
    statusHTML = '<div class="billing-status visible error">' + escapeHTML(context.loginState.request.error) + '</div>';
  } else if (context.loginState.success) {
    var successMessage = context.loginState.successMessage || 'If that email is registered, a magic link is on the way.';
    statusHTML =
      '<div class="billing-status visible success">' +
      escapeHTML(successMessage) +
      '<br><br><strong>Don\'t see it?</strong> <a href="#" data-portal-action="resend-magic-link">Send a new link</a>.' +
      '</div>';
  }
  return (
    '<section class="intro-card">' +
      '<div class="account-panel-kicker">Pulse Account</div>' +
      '<h1>Sign in to Pulse Account</h1>' +
      '<p>Use one commercial email address to get into workspaces, MSP access, billing, license recovery, refunds, and privacy actions.</p>' +
    '</section>' +
    '<section class="billing-section billing-section-auth">' +
      '<div class="billing-panel visible auth-panel">' +
        '<h3>Sign in</h3>' +
        '<p>Enter the commercial email address for your Pulse account. I will send a magic link so you can open Pulse Account without managing a password.</p>' +
        '<div class="form-group">' +
          '<label for="portal-login-email">Email address</label>' +
          '<input id="portal-login-email" type="email" autocomplete="email" placeholder="you@example.com" value="' +
          escapeAttr(context.loginState.emailValue || '') +
          '" data-portal-input="login-email">' +
        '</div>' +
        '<div class="form-actions">' +
          '<button class="btn-primary" id="portal-login-send" type="button" data-portal-action="send-magic-link">' +
          (context.loginState.request.pending ? 'Sending…' : 'Send magic link') +
          '</button>' +
          '<a class="btn-secondary link-button" href="' + escapeAttr(context.signupPath) + '">Create an account</a>' +
        '</div>' +
        statusHTML +
      '</div>' +
    '</section>'
  );
}
