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
  var highlightHTML = highlights.map(function(item) {
    return '<span class="service-action-highlight">' + escapeHTML(item) + '</span>';
  }).join('');
  return (
    '<article class="service-action-row">' +
      '<div class="service-action-main">' +
        '<div class="service-action-tags">' +
          '<span class="service-card-kicker">' + kicker + '</span>' +
          '<span class="service-action-tag">Self-hosted</span>' +
        '</div>' +
        '<div class="service-action-copy">' +
          '<h3>' + title + '</h3>' +
          '<p>' + description + '</p>' +
        '</div>' +
        '<div class="service-action-highlights">' + highlightHTML + '</div>' +
      '</div>' +
      '<div class="service-action-cta">' +
        '<button class="btn-secondary service-action-button" type="button" id="' + id + '" data-account-service-action="open-service-panel" data-account-service-panel="' + panelID + '" data-account-service-focus="' + focusID + '" data-shell-target="services">' + escapeHTML(actionLabel) + '</button>' +
      '</div>' +
    '</article>'
  );
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
  if (status === 'healthy') return 'Ready for operator work';
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
        '<h4>' + escapeHTML(suspendedCount > 0 ? 'Active fleet is stable' : 'Fleet is stable') + '</h4>' +
        '<p>' + escapeHTML(suspendedCount > 0
          ? 'Active hosted workspaces are healthy. Suspended workspaces stay parked until you resume them.'
          : 'Every active hosted workspace currently reports a healthy posture.'
        ) + '</p>' +
        '<div class="overview-stable-list">' +
          '<div class="overview-stable-item"><strong>Healthy now</strong><span>' + escapeHTML(suspendedCount > 0
            ? 'Active hosted workspaces are clear for routine operator work.'
            : 'All active hosted workspaces are clear for routine operator work.'
          ) + '</span></div>' +
          (suspendedCount > 0
            ? '<div class="overview-stable-item"><strong>Suspended stays parked</strong><span>' + escapeHTML(String(suspendedCount) + ' workspace' + (suspendedCount === 1 ? ' is' : 's are') + ' suspended and intentionally out of the day-to-day operator path.') + '</span></div>'
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

function renderAccountContextStrip(account: PortalAccountSummary): string {
  var workspaceLabel = workspaceCountLabel((account.workspaces || []).length);

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
        '<p>' + escapeHTML(account.kind === 'msp' ? 'Operator workspace account' : 'Hosted account operations') + '</p>' +
      '</div>' +
    '</section>'
  );
}

function shellSectionButton(section: PortalShellSection, activeSection: PortalShellSection, title: string, copy: string): string {
  return (
    '<button class="portal-shell-nav-link' + (activeSection === section ? ' active' : '') + '" type="button" data-shell-action="activate-section" data-shell-section="' + section + '">' +
      '<span class="portal-shell-nav-label">' + title + '</span>' +
      '<span class="portal-shell-nav-copy">' + copy + '</span>' +
    '</button>'
  );
}

function renderShellNavigation(accounts: PortalAccountSummary[], supportEmail: string, activeSection: PortalShellSection): string {
  var hosted = hasHostedAccounts(accounts);
  return (
    '<aside class="portal-shell-nav" aria-label="Pulse Account sections">' +
      '<div class="portal-shell-nav-header">' +
        '<div class="portal-shell-nav-eyebrow">Pulse Account</div>' +
        '<div class="portal-shell-nav-title">' + (hosted ? 'Operator console' : 'Account console') + '</div>' +
        '<div class="portal-shell-nav-support">' + (hosted ? 'Hosted operations and account services' : 'Commercial account services and support') + '</div>' +
      '</div>' +
      '<div class="portal-shell-nav-group">' +
        shellSectionButton('overview', activeSection, 'Overview', hosted ? 'Posture, accounts, and quick actions' : 'Account summary and access state') +
        shellSectionButton('workspaces', activeSection, hosted ? 'Workspaces' : 'Hosted access', hosted ? 'Hosted fleet and lifecycle actions' : 'No hosted workspaces are attached yet') +
        shellSectionButton('team', activeSection, 'Team', hosted ? 'Access and operator roster' : 'Account membership') +
        shellSectionButton('services', activeSection, 'Account services', 'Licenses, billing, refunds, and privacy') +
        shellSectionButton('support', activeSection, 'Support', hosted ? 'Escalation and account support' : (supportEmail || 'Support contact')) +
      '</div>' +
    '</aside>'
  );
}

function renderWorkspaceCard(account: PortalAccountSummary, workspace: PortalWorkspaceSummary, accountAPIBasePath: string): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  var createdLabel = formatWorkspaceDate(workspace.created_at);
  var metaParts = [
    '<span class="workspace-meta-item">' + escapeHTML(workspace.id) + '</span>',
  ];
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
  } else {
    openAction = '<span class="workspace-state-label">' + escapeHTML(titleCase(state || 'Unknown')) + '</span>';
  }

  var manageAction = '';
  if (account.can_manage && (state === 'active' || state === 'suspended' || state === 'failed')) {
    manageAction =
      '<button type="button" class="btn-secondary btn-workspace-manage" data-action="select-workspace" data-account-id="' +
      escapeAttr(account.id) +
      '" data-workspace-id="' +
      escapeAttr(workspace.id) +
      '">Manage</button>';
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
    ? 'Hosted posture needs review'
    : checkingCount > 0
      ? 'Hosted posture is still settling'
      : suspendedCount > 0
        ? 'Active fleet is stable'
        : 'Hosted posture is stable';
  var postureCopy = unhealthyCount > 0
    ? 'One or more workspaces still need attention before the hosted fleet is trustworthy.'
    : checkingCount > 0
      ? 'The hosted fleet is mostly healthy, but some workspaces are still waiting on a completed health check.'
      : suspendedCount > 0
        ? 'Active hosted workspaces are healthy while suspended workspaces stay parked until you resume them.'
        : 'The hosted fleet is healthy and ready for routine operator work.';
  var nextStepTitle = unhealthyCount > 0
    ? 'Start in Workspaces'
    : checkingCount > 0
      ? 'Review pending checks'
      : suspendedCount > 0
        ? 'Active fleet is clear'
        : 'Fleet is clear';
  var nextStepCopy = unhealthyCount > 0
    ? 'One or more workspaces need review before you treat the hosted fleet as trustworthy.'
    : checkingCount > 0
      ? 'The fleet is mostly healthy, but there are still workspaces waiting on a completed health check.'
      : suspendedCount > 0
        ? 'Active hosted workspaces look stable. Resume a suspended workspace only when you are ready to bring it back into the operator path.'
        : 'Hosted posture looks stable. Move into team or account services only if you need to change access or billing.';
  var nextStepChecklist = unhealthyCount > 0
    ? (
      '<div class="overview-next-checklist">' +
        '<div class="overview-next-check"><strong>1. Review attention items</strong><span>Open the fleet and inspect any workspace marked as checking or needs attention.</span></div>' +
        '<div class="overview-next-check"><strong>2. Resolve operator blockers</strong><span>Use Team if the right people are not already attached to the hosted account.</span></div>' +
        '<div class="overview-next-check"><strong>3. Escalate billing separately</strong><span>Keep account billing or self-hosted license work out of the workspace review flow.</span></div>' +
      '</div>'
    )
    : checkingCount > 0
      ? (
        '<div class="overview-next-checklist">' +
          '<div class="overview-next-check"><strong>1. Verify pending health checks</strong><span>Open the workspaces still settling and confirm they are safe to operate.</span></div>' +
          '<div class="overview-next-check"><strong>2. Keep the roster lean</strong><span>Review Team only if a pending workspace needs a different operator mix.</span></div>' +
          '<div class="overview-next-check"><strong>3. Leave billing as a separate action</strong><span>Use account services or billing only when the hosted fleet is already understood.</span></div>' +
        '</div>'
      )
      : (
        '<div class="overview-next-checklist">' +
          '<div class="overview-next-check"><strong>1. Use Workspaces for the next operational task</strong><span>Open a client workspace directly when you are ready to do hosted work.</span></div>' +
          '<div class="overview-next-check"><strong>2. Use Team for access changes only</strong><span>Keep roster changes explicit instead of mixing them into routine workspace work.</span></div>' +
          '<div class="overview-next-check"><strong>3. Use account services when the task is commercial</strong><span>Licenses, refunds, privacy, and self-hosted billing stay in their own section.</span></div>' +
        '</div>'
      );
  var nextStepActions =
    '<div class="overview-next-actions">' +
      '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">Open workspaces</button>' +
      '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="' + (account.can_manage ? 'team' : 'services') + '">' + (account.can_manage ? 'Review team access' : 'Open account services') + '</button>' +
    '</div>';

  return (
    '<section class="account-content-panel account-content-panel-overview">' +
      '<div class="account-stage-header account-stage-header-overview">' +
        '<div>' +
          '<div class="account-panel-kicker">Overview</div>' +
          '<h3>Hosted posture</h3>' +
          '<p>Start here to judge fleet posture, pick the next operator action, and keep commercial account work separate from hosted operations.</p>' +
        '</div>' +
      '</div>' +
      '<div class="account-command-deck">' +
        '<div class="account-overview-card">' +
          '<div class="account-overview-lead">' +
            '<div class="account-panel-kicker">Fleet posture</div>' +
            '<h3>' + escapeHTML(postureTitle) + '</h3>' +
            '<p>' + escapeHTML(postureCopy) + '</p>' +
            '<div class="account-overview-callout">' + escapeHTML(account.kind === 'msp'
              ? 'Use this console to run client workspaces, account billing, and operator access from one place.'
              : 'Use this console to run hosted workspaces, account billing, and operator access from one place.'
            ) + '</div>' +
          '</div>' +
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
        '</div>' +
        '<div class="account-overview-secondary">' +
          '<div class="overview-side-card overview-side-card-primary">' +
            '<div class="account-panel-kicker">Next move</div>' +
            '<h4>' + escapeHTML(nextStepTitle) + '</h4>' +
            '<p>' + escapeHTML(nextStepCopy) + '</p>' +
            nextStepChecklist +
            nextStepActions +
          '</div>' +
          renderAttentionPanel(workspaces) +
        '</div>' +
      '</div>' +
    '</section>'
  );
}

function renderAccountWorkspaceSection(account: PortalAccountSummary, accountAPIBasePath: string): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var readyCount = countReadyWorkspaces(workspaces);
  var checkingCount = countWorkspacesByHealth(workspaces, 'checking');
  var unhealthyCount = countWorkspacesByHealth(workspaces, 'unhealthy');
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
            '<h3>Workspace desk</h3>' +
            '<p>Select one workspace from the fleet to inspect posture, lifecycle, and the next explicit operator action.</p>' +
          '</div>' +
          '<button type="button" class="btn-secondary btn-compact" id="workspace-management-close-' +
          escapeAttr(account.id) +
          '" data-action="clear-workspace-selection" data-account-id="' +
          escapeAttr(account.id) +
          '">Done</button>' +
        '</div>' +
        '<div class="workspace-management-empty" id="workspace-management-empty-' +
        escapeAttr(account.id) +
        '">' +
          '<div class="workspace-management-empty-copy">No workspace selected yet.</div>' +
          '<div class="workspace-management-empty-grid">' +
            '<div class="workspace-management-empty-card">' +
              '<div class="account-panel-kicker">Account actions</div>' +
              '<h4>Keep account-wide actions separate</h4>' +
              '<p>Billing, team, and workspace creation stay account-wide even when you are focused on one workspace.</p>' +
              '<div class="workspace-management-empty-actions">' + workspaceDeskActions + '</div>' +
              addWorkspaceForm +
            '</div>' +
            '<div class="workspace-management-empty-card workspace-management-empty-card-muted">' +
              '<div class="account-panel-kicker">When you pick a workspace</div>' +
              '<div class="workspace-management-empty-checklist">' +
                '<div class="workspace-management-empty-check"><strong>Inspect posture</strong><span>Load the workspace first, then decide whether it is routine work, review work, or a parked suspended system.</span></div>' +
                '<div class="workspace-management-empty-check"><strong>Confirm lifecycle</strong><span>Check whether the workspace is active, checking, failed, or suspended before taking the next action.</span></div>' +
                '<div class="workspace-management-empty-check"><strong>Then act deliberately</strong><span>Use the desk to manage one workspace at a time instead of mixing account-wide actions into the same flow.</span></div>' +
              '</div>' +
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
          '<div class="workspace-list-summary">Review the hosted fleet, open a workspace, and keep lifecycle actions explicit.</div>' +
          '<div class="workspace-list-stats">' +
            '<span class="workspace-list-stat"><strong>' + String(workspaces.length) + '</strong> total</span>' +
            '<span class="workspace-list-stat"><strong>' + String(readyCount) + '</strong> ready</span>' +
            '<span class="workspace-list-stat"><strong>' + String(checkingCount) + '</strong> checking</span>' +
            '<span class="workspace-list-stat workspace-list-stat-attention"><strong>' + String(unhealthyCount) + '</strong> needs attention</span>' +
            '<span class="workspace-list-stat"><strong>' + String(suspendedCount) + '</strong> suspended</span>' +
          '</div>' +
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
            '<p>Use the fleet view to open workspaces, watch health posture, and keep management actions explicit.</p>' +
          '</div>' +
          '<div class="account-stage-header-actions">' + workspaceHeaderActions + '</div>' +
        '</div>' +
      '</div>' +
      '<div class="workspace-operations-shell">' +
        '<div class="workspace-operations-main">' +
          workspaceHTML +
        '</div>' +
        '<div class="workspace-operations-detail">' +
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
        '<div class="team-policy-row"><strong>Admin</strong><span>Hosted operations plus billing for the account.</span></div>' +
        '<div class="team-policy-row"><strong>Tech</strong><span>Workspace operations without billing ownership.</span></div>' +
        '<div class="team-policy-row"><strong>Read-only</strong><span>State review and verification without control-plane changes.</span></div>' +
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
            '<h3>Control who can operate this account</h3>' +
            '<p>Owners manage billing and access. Admins and techs keep the hosted fleet running day to day.</p>' +
          '</div>' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">Done</button>' +
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
            '<div class="team-review-desk">' +
              '<div class="team-panel-heading team-panel-heading-tight">' +
                '<div class="account-panel-kicker">Review desk</div>' +
                '<h4>Keep access disciplined</h4>' +
                '<p>Use the roster as a controlled operator surface, not a dumping ground for vague shared access.</p>' +
              '</div>' +
              '<div class="team-review-grid">' +
                '<div class="team-review-card">' +
                  '<strong>Owners stay rare</strong>' +
                  '<span>Reserve Owner for billing, team, and full hosted control. Default to Admin, Tech, or Read-only first.</span>' +
                '</div>' +
                '<div class="team-review-card">' +
                  '<strong>Keep operators narrow</strong>' +
                  '<span>Use Tech for workspace operations and Read-only for verification instead of handing out broader access.</span>' +
                '</div>' +
                '<div class="team-review-card">' +
                  '<strong>Remove stale access fast</strong>' +
                  '<span>If someone no longer owns the work, remove them instead of leaving dormant access attached to the account.</span>' +
                '</div>' +
              '</div>' +
            '</div>' +
          '</div>' +
          '<div class="team-side-column">' +
            '<div class="team-operations-panel">' +
              '<div class="team-panel-heading team-panel-heading-tight">' +
                '<div class="account-panel-kicker">Access desk</div>' +
                '<h4>Invite and role policy</h4>' +
                '<p>Keep the roster deliberate. Invite the smallest role first, then tighten access as responsibilities become clearer.</p>' +
              '</div>' +
              '<div class="team-operations-grid">' +
                '<div class="team-invite-panel">' +
                  '<div class="team-panel-heading">' +
                    '<h4>Invite someone new</h4>' +
                    '<p>Add another operator with the minimum role they need for this account.</p>' +
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
      '<h2>Support and escalation</h2>' +
      '<p>Use support when hosted access looks wrong, billing does not behave as expected, or you need help with commercial licensing and privacy actions.</p>' +
      '<div class="portal-support-card-grid">' +
        '<div class="portal-support-card">' +
          '<h3>Account support</h3>' +
          '<p>For access, tenant handoff, team, and billing issues, contact the hosted operations desk.</p>' +
          '<a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || '') + '">' + escapeHTML(context.bootstrap.support_email || '') + '</a>' +
        '</div>' +
        '<div class="portal-support-card">' +
          '<h3>Commercial services</h3>' +
          '<p>Self-hosted subscriptions, license recovery, refunds, and privacy requests all route through the same account surface.</p>' +
          '<button type="button" class="btn-secondary" data-shell-action="activate-section" data-shell-section="services">Open account services</button>' +
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
  var serviceHeading = hosted ? 'Self-hosted licenses and billing' : 'Account services';
  var serviceNote = hosted
    ? 'Hosted operations live above. Use these commercial tools for self-hosted licenses, billing, refunds, and privacy actions.'
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
              '</div>' +
              '<div class="service-note">' + serviceNote + '</div>' +
            '</div>' +
            '<div class="service-shell">' +
              '<aside class="service-shell-sidebar">' +
                '<div class="service-shell-sidebar-head">' +
                  '<div class="account-panel-kicker">Commercial actions</div>' +
                  '<h3>Billing, licenses, refunds, and privacy</h3>' +
                  '<p>Keep self-hosted commercial work here. Hosted workspace operations stay in Workspaces and Team.</p>' +
                '</div>' +
                '<div class="service-action-list">' +
                  renderServiceActionRow('open-manage-service', 'Billing', 'Manage subscriptions', 'Open billing', 'Open Stripe billing access for existing self-hosted subscriptions without leaving the Pulse Account shell.', 'manage-service-panel', 'manage-inline-email', ['Invoices and plan changes', 'Subscription self-service']) +
                  renderServiceActionRow('open-retrieve-service', 'Licenses', 'Retrieve licenses', 'Open license recovery', 'Recover the latest active self-hosted license and invoice link for a commercial email address.', 'retrieve-service-panel', 'retrieve-inline-email', ['Latest active license', 'Invoice lookup']) +
                  renderServiceActionRow('open-refund-service', 'Refunds', 'Refund requests', 'Open refunds', 'Request an immediate self-serve refund for eligible self-hosted purchases with explicit revocation confirmation.', 'refund-service-panel', 'refund-inline-email', ['Eligibility check', 'Explicit revocation']) +
                  renderServiceActionRow('open-data-service', 'Privacy', 'Data and privacy', 'Open privacy tools', 'Request commercial data export or deletion without leaving the account shell.', 'data-service-panel', 'data-export-email', ['Export or deletion', 'Support escalation path']) +
                '</div>' +
              '</aside>' +
              '<div class="service-shell-main">' +
                '<div class="service-detail-shell">' +
                  '<div class="service-panel service-panel-empty visible" id="service-panel-empty">' +
                    '<div class="account-panel-kicker">Task desk</div>' +
                    '<h3>Choose the next commercial action</h3>' +
                    '<p>Open a billing, license, refund, or privacy flow from the service navigator. The active request stays here so this area behaves like one commercial operating desk.</p>' +
                    '<div class="service-empty-command-grid">' +
                      '<div class="service-empty-command-card">' +
                        '<div class="service-empty-column-title">Start here</div>' +
                        '<div class="service-empty-points service-empty-points-stack">' +
                          '<div class="service-empty-point"><strong>Verify first</strong><span>Each flow confirms the commercial email before opening sensitive actions.</span></div>' +
                          '<div class="service-empty-point"><strong>One task at a time</strong><span>Keep the active request in this desk until you finish or switch tools.</span></div>' +
                        '</div>' +
                      '</div>' +
                      '<div class="service-empty-command-card service-empty-command-card-wide">' +
                        '<div class="service-empty-column-title">Available flows</div>' +
                        '<div class="service-empty-flow-list">' +
                          '<div class="service-empty-flow"><strong>Billing</strong><span>Stripe customer portal access after verification.</span></div>' +
                          '<div class="service-empty-flow"><strong>Licenses</strong><span>Recover the latest active self-hosted license and invoice link.</span></div>' +
                          '<div class="service-empty-flow"><strong>Refunds</strong><span>Confirm eligibility before revoking active commercial access.</span></div>' +
                          '<div class="service-empty-flow"><strong>Privacy</strong><span>Request export or deletion without leaving Pulse Account.</span></div>' +
                        '</div>' +
                      '</div>' +
                      '<div class="service-empty-command-card service-empty-command-card-support">' +
                        '<div class="service-empty-column-title">Support</div>' +
                        '<div class="service-empty-checklist">' +
                          '<div class="service-empty-check"><strong>Escalate quickly</strong><span>If billing, licenses, refunds, or privacy behave unexpectedly, escalate from this surface.</span></div>' +
                        '</div>' +
                        '<div class="service-empty-support">Need help with billing, refund, privacy, or license actions? <a class="portal-support-link" href="mailto:' +
                        escapeAttr(context.bootstrap.support_email || '') +
                        '">' +
                        escapeHTML(context.bootstrap.support_email || '') +
                        '</a></div>' +
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
      '<h1>Sign in to the account surface</h1>' +
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
