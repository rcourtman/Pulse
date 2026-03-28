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

function renderServiceActionRow(
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
    '<article class="service-action-row">' +
      '<div class="service-action-main">' +
        '<div class="service-action-tags service-action-tags-tight">' +
          '<span class="service-card-kicker">' + kicker + '</span>' +
          '<span class="service-action-meta-chip">' + escapeHTML(meta) + '</span>' +
        '</div>' +
        '<div class="service-action-copy">' +
          '<h3>' + title + '</h3>' +
          '<p>' + description + '</p>' +
        '</div>' +
      '</div>' +
      '<div class="service-action-cta">' +
        '<button class="btn-secondary service-action-button" type="button" id="' + id + '" data-account-service-action="open-service-panel" data-account-service-panel="' + panelID + '" data-account-service-focus="' + focusID + '" data-shell-target="services">' + escapeHTML(actionLabel) + '</button>' +
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
          ? 'Active hosted workspaces are healthy. Suspended workspaces stay parked until you resume them.'
          : 'Every active hosted workspace currently reports a healthy status.'
        ) + '</p>' +
        '<div class="overview-stable-list">' +
          '<div class="overview-stable-item"><strong>Healthy now</strong><span>' + escapeHTML(suspendedCount > 0
            ? 'Active hosted workspaces are clear for routine use.'
            : 'All active hosted workspaces are clear for routine use.'
          ) + '</span></div>' +
          (suspendedCount > 0
            ? '<div class="overview-stable-item"><strong>Suspended stays parked</strong><span>' + escapeHTML(String(suspendedCount) + ' workspace' + (suspendedCount === 1 ? ' is' : 's are') + ' suspended and intentionally out of day-to-day use.') + '</span></div>'
            : '') +
          '<div class="overview-stable-item"><strong>Use Team only for change</strong><span>Keep roster edits explicit instead of mixing them into normal workspace work.</span></div>' +
          '<div class="overview-stable-item"><strong>Keep billing separate</strong><span>Use account services only when the task is commercial, not operational.</span></div>' +
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
      '<p>These workspaces should be checked before you treat the hosted fleet as fully healthy.</p>' +
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
        '<p>' + escapeHTML(account.kind === 'msp' ? 'Hosted workspace account for workspace access, team control, and billing.' : 'Hosted account for workspace access, team control, and billing.') + '</p>' +
      '</div>' +
      '<div class="portal-account-context-summary">' +
        '<div class="portal-account-context-stat">' +
          '<span>Role</span>' +
          '<strong>' + escapeHTML(titleCase(account.role)) + '</strong>' +
        '</div>' +
        '<div class="portal-account-context-stat">' +
          '<span>Hosted fleet</span>' +
          '<strong>' + escapeHTML(workspaceLabel) + '</strong>' +
        '</div>' +
        '<div class="portal-account-context-stat">' +
          '<span>Commercial</span>' +
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
  var canManage = false;
  for (var i = 0; i < accounts.length; i += 1) {
    if (accounts[i].can_manage) {
      canManage = true;
      break;
    }
  }
  return (
    '<aside class="portal-shell-nav" aria-label="Pulse Account sections">' +
      '<div class="portal-shell-nav-header">' +
        '<div class="portal-shell-nav-eyebrow">Pulse Account</div>' +
        '<div class="portal-shell-nav-title">Account center</div>' +
        '<div class="portal-shell-nav-support">' + (hosted ? 'Hosted workspaces, account access, and commercial services' : 'Commercial account services and support') + '</div>' +
      '</div>' +
      '<div class="portal-shell-nav-group">' +
        shellSectionButton('overview', activeSection, '01', 'Overview', hosted ? 'Status, priorities, and next actions' : 'Account summary and access state', hosted ? String(totalWorkspaces) + ' total' : 'Summary') +
        shellSectionButton('workspaces', activeSection, '02', hosted ? 'Workspaces' : 'Hosted access', hosted ? 'Hosted workspaces and lifecycle actions' : 'No hosted workspaces are attached yet', hosted ? String(readyWorkspaces) + ' ready' : 'None') +
        shellSectionButton('team', activeSection, '03', 'Team', hosted ? 'Access and team roster' : 'Account membership', canManage ? 'Manage' : 'View') +
        shellSectionButton('services', activeSection, '04', 'Account services', 'Licenses, billing, refunds, and privacy', '4 tools') +
        shellSectionButton('support', activeSection, '05', 'Support', hosted ? 'Escalation and account support' : (supportEmail || 'Support contact'), supportEmail ? 'Email' : 'Help') +
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

function renderAccountOverviewSection(account: PortalAccountSummary): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var totalCount = workspaces.length;
  var readyCount = countReadyWorkspaces(workspaces);
  var healthyCount = countWorkspacesByHealth(workspaces, 'healthy');
  var checkingCount = countWorkspacesByHealth(workspaces, 'checking');
  var unhealthyCount = countWorkspacesByHealth(workspaces, 'unhealthy');
  var suspendedCount = countWorkspacesByState(workspaces, 'suspended');
  var postureTitle = unhealthyCount > 0
    ? 'Needs review'
    : checkingCount > 0
      ? 'Still settling'
      : suspendedCount > 0
        ? 'Active fleet is stable'
        : 'Fleet is stable';
  var postureCopy = unhealthyCount > 0
    ? 'One or more workspaces still need attention before the hosted fleet is trustworthy.'
    : checkingCount > 0
      ? 'The hosted fleet is mostly healthy, but some workspaces are still waiting on a completed health check.'
      : suspendedCount > 0
        ? 'Active hosted workspaces are healthy while suspended workspaces stay parked until you resume them.'
        : 'The hosted fleet is healthy and ready for routine use.';
  var nextStepTitle = unhealthyCount > 0
    ? 'Start in workspaces'
    : checkingCount > 0
      ? 'Review pending checks'
      : suspendedCount > 0
        ? 'Next step'
        : 'Next step';
  var nextStepCopy = unhealthyCount > 0
    ? 'One or more workspaces need review before you treat the hosted fleet as trustworthy.'
    : checkingCount > 0
      ? 'The fleet is mostly healthy, but there are still workspaces waiting on a completed health check.'
      : suspendedCount > 0
        ? 'Active hosted workspaces look stable. Resume a suspended workspace only when you are ready to bring it back into regular use.'
        : 'Everything looks stable. Move into Team or Account services only if you need to change access or billing.';
  var nextStepChecklist = unhealthyCount > 0
    ? (
      '<div class="overview-next-checklist">' +
        '<div class="overview-next-check"><strong>1. Review attention items</strong><span>Open the fleet and inspect any workspace marked as checking or needs attention.</span></div>' +
        '<div class="overview-next-check"><strong>2. Resolve access blockers</strong><span>Use Team if the right people are not already attached to the hosted account.</span></div>' +
        '<div class="overview-next-check"><strong>3. Escalate billing separately</strong><span>Keep account billing or self-hosted license work out of the workspace review flow.</span></div>' +
      '</div>'
    )
    : checkingCount > 0
      ? (
        '<div class="overview-next-checklist">' +
          '<div class="overview-next-check"><strong>1. Verify pending checks</strong><span>Open the workspaces still settling and confirm they are safe to operate.</span></div>' +
          '<div class="overview-next-check"><strong>2. Keep access deliberate</strong><span>Change Team only if a pending workspace needs a different mix of access.</span></div>' +
          '<div class="overview-next-check"><strong>3. Keep commercial work separate</strong><span>Use account services or billing only when the hosted fleet is already understood.</span></div>' +
        '</div>'
      )
      : (
        '<div class="overview-next-checklist">' +
          '<div class="overview-next-check"><strong>1. Open a workspace for the next operational task</strong><span>Move into Workspaces when you are ready to do hosted work.</span></div>' +
          '<div class="overview-next-check"><strong>2. Change access in Team only</strong><span>Keep roster changes explicit instead of mixing them into routine workspace work.</span></div>' +
          '<div class="overview-next-check"><strong>3. Keep billing and privacy separate</strong><span>Licenses, refunds, privacy, and self-hosted billing stay in their own section.</span></div>' +
        '</div>'
      );
  var nextStepActions =
    '<div class="overview-next-actions">' +
      '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">Open workspaces</button>' +
      '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="' + (account.can_manage ? 'team' : 'services') + '">' + (account.can_manage ? 'Review team access' : 'Open account services') + '</button>' +
    '</div>';
  var accountScopeCopy = account.kind === 'msp'
    ? 'Manage client workspaces, billing, and team access from one place.'
    : 'Manage hosted workspaces, billing, and team access from one place.';
  var overviewBriefStrip =
    '<div class="account-overview-brief-strip">' +
      '<div class="account-overview-brief-point account-overview-brief-point-wide">' +
        '<strong>Account scope</strong>' +
        '<span>' + escapeHTML(accountScopeCopy) + '</span>' +
      '</div>' +
      '<div class="account-overview-brief-point">' +
        '<strong>Hosted path</strong>' +
        '<span>Use Workspaces for tenant access and lifecycle changes, and Team only when access needs to change.</span>' +
      '</div>' +
      '<div class="account-overview-brief-point">' +
        '<strong>Commercial path</strong>' +
        '<span>Keep licenses, refunds, privacy, and self-hosted billing in Account services instead of mixing them into hosted work.</span>' +
      '</div>' +
    '</div>';

  return (
    '<section class="account-content-panel account-content-panel-overview">' +
      '<div class="account-stage-header account-stage-header-overview">' +
        '<div>' +
          '<div class="account-panel-kicker">Overview</div>' +
          '<h3>Hosted status</h3>' +
          '<p>Review hosted status first, then move into the next section.</p>' +
          renderSectionContextChips([
            String(totalCount) + ' total',
            String(readyCount) + ' ready',
            suspendedCount > 0 ? String(suspendedCount) + ' suspended' : 'Active fleet',
          ]) +
          renderOverviewMetricStrip(totalCount, readyCount, checkingCount, unhealthyCount, suspendedCount) +
        '</div>' +
      '</div>' +
      '<div class="account-command-deck">' +
        '<div class="account-overview-main-column">' +
          '<div class="account-overview-card account-overview-briefing-card">' +
            '<div class="account-overview-lead">' +
              '<div class="account-panel-kicker">Fleet status</div>' +
              '<h3>' + escapeHTML(postureTitle) + '</h3>' +
              '<p>' + escapeHTML(postureCopy) + '</p>' +
              overviewBriefStrip +
            '</div>' +
            '<div class="account-overview-next-block">' +
              '<div class="account-panel-kicker">Next move</div>' +
              '<h4>' + escapeHTML(nextStepTitle) + '</h4>' +
              '<p>' + escapeHTML(nextStepCopy) + '</p>' +
              nextStepChecklist +
              nextStepActions +
            '</div>' +
          '</div>' +
        '</div>' +
        '<div class="account-overview-side-column">' +
          renderAttentionPanel(workspaces) +
        '</div>' +
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
    if (account.has_billing) {
      workspaceHeaderActions +=
        '<button type="button" class="btn-secondary btn-compact" data-action="open-billing" data-account-id="' +
        escapeAttr(account.id) +
        '">Manage billing</button>';
    }
    workspaceHeaderActions +=
      '<button type="button" class="btn-secondary btn-compact" data-action="toggle-team" data-account-id="' +
      escapeAttr(account.id) +
      '" data-shell-target="team">Manage team</button>';

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
    if (account.has_billing) {
      workspaceDeskActions +=
        '<button type="button" class="btn-secondary btn-compact" data-action="open-billing" data-account-id="' +
        escapeAttr(account.id) +
        '">Manage billing</button>';
    }
    workspaceDeskActions +=
      '<button type="button" class="btn-secondary btn-compact" data-action="toggle-team" data-account-id="' +
      escapeAttr(account.id) +
      '" data-shell-target="team">Manage team</button>';

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
          '<div class="workspace-management-empty-copy">Pick one workspace for lifecycle review. Keep hosted billing, team changes, and new workspace creation in account actions.</div>' +
          '<div class="workspace-management-empty-shell">' +
            '<div class="workspace-management-empty-actions-card">' +
              '<div class="workspace-management-empty-actions-copy">' +
                '<div class="account-panel-kicker">Account actions</div>' +
                '<h4>Keep account-wide actions separate</h4>' +
                '<p>Use this area for new workspaces, hosted billing, and team changes. Keep row actions focused on a single workspace.</p>' +
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
            '<div class="account-panel-kicker">Workspace fleet</div>' +
            '<h3>Hosted fleet</h3>' +
            '<p>Open workspaces, review status, and keep lifecycle actions explicit.</p>' +
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

function renderAccountTeamSection(account: PortalAccountSummary): string {
  var accessPolicy =
    '<div class="team-policy-panel">' +
      '<div class="team-panel-heading">' +
        '<h4>Access model</h4>' +
        '<p>Assign the smallest role that still lets someone do the work they own on this account.</p>' +
      '</div>' +
      '<div class="team-policy-list">' +
        '<div class="team-policy-row"><strong>Owner</strong><span>Billing, team access, and full hosted control.</span></div>' +
        '<div class="team-policy-row"><strong>Admin</strong><span>Hosted workspace control plus billing for the account.</span></div>' +
        '<div class="team-policy-row"><strong>Tech</strong><span>Workspace control without billing ownership.</span></div>' +
        '<div class="team-policy-row"><strong>Read-only</strong><span>State review and verification without control-plane changes.</span></div>' +
      '</div>' +
    '</div>';
  var reviewDesk =
    '<div class="team-review-shell">' +
      '<div class="team-review-strip">' +
        '<div class="team-panel-heading team-panel-heading-tight">' +
          '<div class="account-panel-kicker">Access review</div>' +
          '<h4>Keep access disciplined</h4>' +
          '<p>Use the roster as a controlled access list, not a dumping ground for vague shared access.</p>' +
        '</div>' +
        '<div class="team-review-grid">' +
          '<div class="team-review-card">' +
            '<strong>Owners stay rare</strong>' +
            '<span>Reserve Owner for billing, team, and full hosted control. Default to Admin, Tech, or Read-only first.</span>' +
          '</div>' +
          '<div class="team-review-card">' +
            '<strong>Keep access narrow</strong>' +
            '<span>Use Tech for workspace control and Read-only for verification instead of handing out broader access.</span>' +
          '</div>' +
          '<div class="team-review-card">' +
            '<strong>Remove stale access fast</strong>' +
            '<span>If someone no longer owns the work, remove them instead of leaving dormant access attached to the account.</span>' +
          '</div>' +
        '</div>' +
      '</div>' +
    '</div>';

  return (
    '<section class="account-content-panel account-content-panel-team">' +
      '<section class="team-management-panel team-section team-section-shell" id="team-section-' +
      escapeAttr(account.id) +
      '" data-actor-role="' +
      escapeAttr(account.role) +
      '">' +
        '<div class="team-management-header">' +
          '<div>' +
            '<div class="account-panel-kicker">Team management</div>' +
            '<h3>Account access</h3>' +
            '<p>Owners govern billing and access. Admins and techs keep hosted work moving day to day.</p>' +
            renderSectionContextChips([
              account.can_manage ? 'Managed roster' : 'View only',
              'Least privilege',
              'Hosted access',
            ]) +
          '</div>' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">Close team view</button>' +
        '</div>' +
        '<div class="team-management-stats" id="team-stats-' +
        escapeAttr(account.id) +
        '"></div>' +
        '<div class="team-management-grid">' +
          '<div class="team-roster-column">' +
            '<div class="team-roster">' +
              '<div class="team-panel-heading">' +
                '<h4>People on this account</h4>' +
                '<p>Keep the roster small and role assignment explicit. The people listed here are the ones who can operate the hosted fleet.</p>' +
              '</div>' +
              '<div class="team-roster-list" id="team-list-' +
              escapeAttr(account.id) +
              '">' +
                '<div class="team-list-message">Loading…</div>' +
              '</div>' +
            '</div>' +
            reviewDesk +
          '</div>' +
          '<div class="team-side-column">' +
            '<div class="team-operations-panel">' +
              '<div class="team-panel-heading team-panel-heading-tight">' +
                '<div class="account-panel-kicker">Access controls</div>' +
                '<h4>Invite and role policy</h4>' +
                '<p>Keep the roster deliberate. Invite the smallest role first, then tighten access as responsibilities become clearer.</p>' +
              '</div>' +
              '<div class="team-operations-grid">' +
                '<div class="team-invite-panel">' +
                  '<div class="team-panel-heading">' +
                    '<h4>Invite someone new</h4>' +
                    '<p>Add another person with the minimum role they need for this account.</p>' +
                  '</div>' +
                  '<div class="team-invite">' +
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

function renderSupportSection(context: ShellViewContext): string {
  return (
    '<section class="portal-support-panel">' +
      '<div class="account-panel-kicker">Support</div>' +
      '<h2>Support</h2>' +
      '<p>Use this section when hosted access looks wrong, billing behaves unexpectedly, or you need help with commercial requests.</p>' +
      renderSectionContextChips(['Hosted issues', 'Commercial requests', context.bootstrap.support_email ? 'Email' : 'Support']) +
      '<div class="portal-support-brief-strip">' +
        '<div class="portal-support-brief-card">' +
          '<strong>Hosted path</strong>' +
          '<span>Workspace access, team control, tenant handoff, and hosted billing stay on the hosted account route.</span>' +
        '</div>' +
        '<div class="portal-support-brief-card">' +
          '<strong>Commercial path</strong>' +
          '<span>Self-hosted billing, licenses, refunds, and privacy requests stay in Account services until escalation is needed.</span>' +
        '</div>' +
        '<div class="portal-support-brief-card">' +
          '<strong>Escalate with context</strong>' +
          '<span>Include the exact account, workspace, section, and failed action so support can continue the same path immediately.</span>' +
        '</div>' +
      '</div>' +
      '<div class="portal-support-layout">' +
        '<div class="portal-support-route-grid">' +
          '<div class="portal-support-route-card">' +
            '<div class="account-panel-kicker">Hosted account</div>' +
            '<h3>Hosted support</h3>' +
            '<p>Use this route when tenant handoff, workspace access, team control, or hosted billing looks wrong.</p>' +
            '<div class="portal-support-points">' +
              '<div class="portal-support-point"><strong>Route here for hosted issues</strong><span>Access, handoff, team, and hosted billing all belong on the hosted account path.</span></div>' +
              '<div class="portal-support-point"><strong>Keep the account context intact</strong><span>Include the account, workspace, and action that failed so support can pick up the same issue quickly.</span></div>' +
            '</div>' +
            '<div class="portal-support-actions">' +
              '<a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || '') + '">' + escapeHTML(context.bootstrap.support_email || '') + '</a>' +
            '</div>' +
          '</div>' +
          '<div class="portal-support-route-card">' +
            '<div class="account-panel-kicker">Commercial</div>' +
            '<h3>Commercial requests</h3>' +
            '<p>Self-hosted subscriptions, license recovery, refunds, and privacy requests all route through Account services first.</p>' +
            '<div class="portal-support-points">' +
              '<div class="portal-support-point"><strong>Start in Account services</strong><span>Use Billing, Licenses, Refunds, or Privacy before escalating a commercial issue.</span></div>' +
              '<div class="portal-support-point"><strong>Escalate from the same path</strong><span>Keep the request in one place instead of splitting it between billing and account sections.</span></div>' +
            '</div>' +
            '<div class="portal-support-actions">' +
              '<button type="button" class="btn-secondary" data-shell-action="activate-section" data-shell-section="services">Open account services</button>' +
            '</div>' +
          '</div>' +
        '</div>' +
        '<div class="portal-support-runbook">' +
          '<div class="account-panel-kicker">Escalation guide</div>' +
          '<h3>Choose the right path</h3>' +
          '<p>Keep hosted workspace issues, commercial requests, and pure support escalation on their own paths so the next person does not have to reconstruct the account state.</p>' +
          '<div class="portal-support-runbook-brief">' +
            '<div class="portal-support-runbook-brief-card"><strong>Confirm scope</strong><span>Decide first whether the issue is a hosted workspace problem, a commercial self-service request, or direct support escalation.</span></div>' +
            '<div class="portal-support-runbook-brief-card"><strong>Keep paths separate</strong><span>Workspace and team problems stay out of billing, refund, privacy, and license work.</span></div>' +
            '<div class="portal-support-runbook-brief-card"><strong>Escalate with facts</strong><span>Bring the exact account, workspace, section, and failed action so support starts with the same state you saw.</span></div>' +
          '</div>' +
          '<div class="portal-support-runbook-grid">' +
            '<div class="portal-support-runbook-section">' +
              '<div class="portal-support-runbook-section-title">Route checklist</div>' +
              '<div class="portal-support-runbook-list">' +
                '<div class="portal-support-runbook-step"><strong>1. Confirm the scope</strong><span>Decide whether the problem is a hosted workspace issue, a commercial self-service request, or direct support escalation.</span></div>' +
                '<div class="portal-support-runbook-step"><strong>2. Keep hosted and commercial separate</strong><span>Workspace and team problems stay in their own sections. Billing, license, refund, and privacy work stay in Account services.</span></div>' +
                '<div class="portal-support-runbook-step"><strong>3. Escalate with context</strong><span>Include the account, workspace, and exact failed action so the escalation path starts with the same facts you saw.</span></div>' +
              '</div>' +
            '</div>' +
            '<div class="portal-support-runbook-section">' +
              '<div class="portal-support-runbook-section-title">Escalation packet</div>' +
              '<div class="portal-support-handoff-note">' +
                '<strong>Include in the escalation</strong>' +
                '<span>Account name, workspace name if relevant, the section you were in, the exact button or request that failed, and whether the issue was hosted or commercial.</span>' +
              '</div>' +
            '</div>' +
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
    return renderAccountOverviewSection(account);
  }).join('');
}

export function renderAuthenticatedPortalHTML(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var hosted = hasHostedAccounts(accounts);
  var activeSection = context.activeSection || 'overview';
  var serviceHeading = hosted ? 'Self-hosted commercial services' : 'Account services';
  var serviceNote = hosted
    ? 'Hosted workspace changes stay in Workspaces and Team. Use this area only for self-hosted commercial requests.'
    : 'Use these account tools for self-hosted licenses, billing, refunds, and privacy actions.';
  var hostedContent = accounts.map(function(account) {
    return (
      '<section class="account-surface">' +
        '<div class="account-surface-body">' +
          renderAccountOverviewSection(account) +
          renderAccountWorkspaceSection(account, context.accountAPIBasePath) +
          renderAccountTeamSection(account) +
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
          '<section class="portal-content-panel portal-content-panel-services service-section" id="account-services-section">' +
            '<div class="service-header">' +
              '<div>' +
                '<div class="account-panel-kicker">Account services</div>' +
                '<h2>' + serviceHeading + '</h2>' +
                renderSectionContextChips([
                  hosted ? 'Self-hosted only' : 'Commercial',
                  '4 tools',
                ]) +
              '</div>' +
              '<div class="service-note">' + serviceNote + '</div>' +
            '</div>' +
            '<div class="service-shell">' +
              '<aside class="service-shell-sidebar">' +
                '<div class="service-shell-sidebar-head">' +
                  '<div class="account-panel-kicker">Service navigator</div>' +
                  '<h3>Self-hosted tools</h3>' +
                  '<p>Pick one commercial request and keep it separate from hosted workspaces and team changes.</p>' +
                '</div>' +
                '<div class="service-action-list">' +
                  renderServiceActionRow('open-manage-service', 'Billing', 'Manage subscriptions', 'Billing', 'Open Stripe for self-hosted plan, invoice, and payment changes.', 'manage-service-panel', 'manage-inline-email', ['Plan changes', 'Invoices']) +
                  renderServiceActionRow('open-retrieve-service', 'Licenses', 'Retrieve licenses', 'Licenses', 'Recover the latest active self-hosted license and invoice link.', 'retrieve-service-panel', 'retrieve-inline-email', ['Latest active license', 'Invoice lookup']) +
                  renderServiceActionRow('open-refund-service', 'Refunds', 'Refund requests', 'Refunds', 'Request a self-serve refund when the purchase is still eligible.', 'refund-service-panel', 'refund-inline-email', ['Eligibility check', 'Revocation']) +
                  renderServiceActionRow('open-data-service', 'Privacy', 'Data and privacy', 'Privacy', 'Request export or deletion for commercial account data.', 'data-service-panel', 'data-export-email', ['Export', 'Deletion']) +
                '</div>' +
                '<div class="service-inline-support">' +
                  '<div class="account-panel-kicker">Commercial routing</div>' +
                  '<h4>Keep this request separate</h4>' +
                  '<p>Hosted workspace work stays in Workspaces and Team. Use Support only when a self-hosted billing, license, refund, or privacy request cannot complete cleanly.</p>' +
                  '<div class="service-inline-support-points">' +
                    '<div class="service-inline-support-point"><strong>Hosted stays hosted</strong><span>Tenant handoff, team access, and hosted billing do not belong in this commercial section.</span></div>' +
                    '<div class="service-inline-support-point"><strong>Escalate with context</strong><span>Bring the service name and exact failed action if a commercial request needs support.</span></div>' +
                  '</div>' +
                  '<div class="service-inline-support-actions">' +
                    '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="support">Open support</button>' +
                  '</div>' +
                '</div>' +
              '</aside>' +
              '<div class="service-shell-main">' +
                '<div class="service-detail-shell">' +
                  '<div class="service-panel service-panel-empty visible" id="service-panel-empty">' +
                    '<div class="account-panel-kicker">Service brief</div>' +
                    '<h3>Choose a service to begin</h3>' +
                    '<p>Use one self-hosted request at a time. Each service verifies commercial identity first, then keeps billing, license, refund, or privacy work contained in one place.</p>' +
                    '<div class="service-empty-shell">' +
                      '<div class="service-empty-primary">' +
                        '<div class="service-empty-section service-empty-section-compact">' +
                          '<div class="service-empty-column-title">What each tool does</div>' +
                          '<div class="service-empty-flow-list">' +
                            '<div class="service-empty-flow"><strong>Billing</strong><span>Stripe customer portal access for invoices, payment methods, and plan changes.</span></div>' +
                            '<div class="service-empty-flow"><strong>Licenses</strong><span>Recover the latest active self-hosted license and the matching invoice link.</span></div>' +
                            '<div class="service-empty-flow"><strong>Refunds</strong><span>Check eligibility before revoking active commercial access.</span></div>' +
                            '<div class="service-empty-flow"><strong>Privacy</strong><span>Request export or deletion without leaving Pulse Account.</span></div>' +
                          '</div>' +
                        '</div>' +
                        '<div class="service-empty-section service-empty-section-compact">' +
                          '<div class="service-empty-column-title">Before you start</div>' +
                          '<div class="service-empty-points service-empty-points-stack">' +
                            '<div class="service-empty-point"><strong>One request</strong><span>Keep a single commercial task active instead of bouncing across sections.</span></div>' +
                            '<div class="service-empty-point"><strong>Identity first</strong><span>Verification happens before any sensitive account action opens.</span></div>' +
                            '<div class="service-empty-point"><strong>Stay focused</strong><span>Keep one commercial request in flight instead of bouncing between services.</span></div>' +
                          '</div>' +
                        '</div>' +
                      '</div>' +
                      '<div class="service-empty-side">' +
                        '<div class="service-empty-section service-empty-section-support">' +
                          '<div class="service-empty-column-title">Escalation</div>' +
                          '<div class="service-empty-checklist">' +
                            '<div class="service-empty-check"><strong>Escalate quickly</strong><span>If billing, licenses, refunds, or privacy do not behave as expected, escalate from this section.</span></div>' +
                            '<div class="service-empty-check"><strong>Commercial packet</strong><span>Bring the service name, commercial email, and the exact failed action so support starts with the same request state.</span></div>' +
                          '</div>' +
                          '<div class="service-empty-support">Need help with billing, refund, privacy, or license requests? <a class="portal-support-link" href="mailto:' +
                          escapeAttr(context.bootstrap.support_email || '') +
                          '">' +
                          escapeHTML(context.bootstrap.support_email || '') +
                          '</a></div>' +
                          '<div class="service-empty-actions">' +
                            '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="support">Open support</button>' +
                          '</div>' +
                        '</div>' +
                      '</div>' +
                    '</div>' +
                  '</div>' +
                  '<div class="service-panel" id="manage-service-panel"><div id="manage-service-root"></div></div>' +
                  '<div class="service-panel" id="retrieve-service-panel"><div id="retrieve-service-root"></div></div>' +
                  '<div class="service-panel" id="refund-service-panel"><div id="refund-service-root"></div></div>' +
                  '<div class="service-panel" id="data-service-panel">' +
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
    statusHTML = '<div class="service-status visible error">' + escapeHTML(context.loginState.request.error) + '</div>';
  } else if (context.loginState.success) {
    var successMessage = context.loginState.successMessage || 'If that email is registered, a magic link is on the way.';
    statusHTML =
      '<div class="service-status visible success">' +
      escapeHTML(successMessage) +
      '<br><br><strong>Don\'t see it?</strong> <a href="#" data-portal-action="resend-magic-link">Send a new link</a>.' +
      '</div>';
  }
  return (
    '<section class="intro-card">' +
      '<div class="account-panel-kicker">Pulse Account</div>' +
      '<h1>Sign in to Pulse Account</h1>' +
      '<p>Use one commercial email address to get into hosted workspaces, MSP access, billing, license recovery, refunds, and privacy actions.</p>' +
    '</section>' +
    '<section class="service-section service-section-auth">' +
      '<div class="service-panel visible auth-panel">' +
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
