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
        '<button class="btn-secondary service-action-button" type="button" id="' + id + '" data-account-service-action="open-service-panel" data-account-service-panel="' + panelID + '" data-account-service-focus="' + focusID + '" data-shell-target="services">Open</button>' +
      '</div>' +
    '</article>'
  );
}

function renderOverviewQuickAction(section: PortalShellSection, title: string, copy: string): string {
  return (
    '<button class="overview-quick-action" type="button" data-shell-action="activate-section" data-shell-section="' + section + '">' +
      '<span class="overview-quick-action-title">' + title + '</span>' +
      '<span class="overview-quick-action-copy">' + copy + '</span>' +
    '</button>'
  );
}

function workspaceStatusCopy(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  if (status === 'healthy') return 'Live updates and health checks are currently good.';
  if (status === 'unhealthy') return 'This workspace needs attention before it is trustworthy.';
  return 'This workspace is still waiting on a completed health check.';
}

function workspaceRowNote(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
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
  if (!attention.length) {
    return (
      '<div class="overview-side-card">' +
        '<div class="account-panel-kicker">Attention</div>' +
        '<h4>Fleet is stable</h4>' +
        '<p>Every visible hosted workspace currently reports a healthy posture.</p>' +
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

function renderOverviewBand(accounts: PortalAccountSummary[]): string {
  var hosted = hasHostedAccounts(accounts);
  var workspaceTotal = countWorkspaces(accounts);
  var statusText = hosted ? 'Hosted access is active on this account.' : 'No hosted workspace access is attached to this account yet.';
  var summary = hosted
    ? 'Hosted operations, operator access, and commercial account services.'
    : 'Billing, license recovery, refunds, and privacy actions until hosted access is attached.';

  return (
    '<section class="portal-shell-head">' +
      '<div class="portal-shell-head-main">' +
        '<div class="portal-shell-head-kicker">Pulse Account</div>' +
        '<div class="portal-shell-head-row">' +
          '<div class="portal-shell-head-copy">' +
            '<div class="portal-shell-head-brand-row">' +
              '<h1 class="portal-shell-head-title">' + (hosted ? 'Operator console' : 'Account console') + '</h1>' +
              '<span class="portal-shell-head-chip">' + (hosted ? 'Operator ready' : 'Self-hosted only') + '</span>' +
            '</div>' +
            '<p><strong>' + statusText + '</strong> ' + summary + '</p>' +
          '</div>' +
          '<div class="portal-shell-head-stats">' +
            '<div class="portal-shell-head-stat">' +
              '<span class="portal-shell-head-stat-label">Hosted access</span>' +
              '<span class="portal-shell-head-stat-value">' + (hosted ? 'Active' : 'Not attached') + '</span>' +
            '</div>' +
            '<div class="portal-shell-head-stat">' +
              '<span class="portal-shell-head-stat-label">Accounts</span>' +
              '<span class="portal-shell-head-stat-value">' + (accounts.length === 1 ? '1 account' : String(accounts.length) + ' accounts') + '</span>' +
            '</div>' +
            '<div class="portal-shell-head-stat">' +
              '<span class="portal-shell-head-stat-label">Workspace fleet</span>' +
              '<span class="portal-shell-head-stat-value">' + (workspaceTotal ? workspaceCountLabel(workspaceTotal) : '0 workspaces') + '</span>' +
            '</div>' +
          '</div>' +
        '</div>' +
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
        shellSectionButton('support', activeSection, 'Support', supportEmail || 'Support contact') +
      '</div>' +
      '<div class="portal-shell-nav-footer">' +
        '<div class="portal-shell-nav-footer-label">Need help?</div>' +
        '<a class="portal-shell-nav-footer-link" href="mailto:' + escapeAttr(supportEmail || 'support@pulserelay.pro') + '">' + escapeHTML(supportEmail || 'support@pulserelay.pro') + '</a>' +
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
    '<article class="workspace-row">' +
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
  var healthyCount = countWorkspacesByHealth(workspaces, 'healthy');
  var checkingCount = countWorkspacesByHealth(workspaces, 'checking');
  var unhealthyCount = countWorkspacesByHealth(workspaces, 'unhealthy');
  var activeCount = countWorkspacesByState(workspaces, 'active');
  var postureTitle = unhealthyCount > 0
    ? 'Hosted posture needs review'
    : checkingCount > 0
      ? 'Hosted posture is still settling'
      : 'Hosted posture is stable';
  var postureCopy = unhealthyCount > 0
    ? 'One or more workspaces still need attention before the hosted fleet is trustworthy.'
    : checkingCount > 0
      ? 'The hosted fleet is mostly healthy, but some workspaces are still waiting on a completed health check.'
      : 'The hosted fleet is healthy and ready for routine operator work.';
  var operationsCopy = account.kind === 'msp'
    ? 'Manage the client fleet from this account surface. Workspace creation, billing, and team actions belong here.'
    : 'Use this account surface to open hosted workspaces, manage billing, and control access for this hosted account.';

  var actions = '';
  var addWorkspaceForm = '';
  if (account.can_manage) {
    actions =
      '<div class="account-action-strip">' +
        '<div class="account-action-copy">' +
          '<div class="account-panel-kicker">Account operations</div>' +
          '<p>' + escapeHTML(operationsCopy) + '</p>' +
        '</div>' +
        '<div class="account-actions">' +
          (account.kind === 'msp'
            ? '<button type="button" class="btn-secondary" id="add-ws-btn-' +
              escapeAttr(account.id) +
              '" data-action="toggle-add-workspace" data-account-id="' +
              escapeAttr(account.id) +
              '" data-shell-target="workspaces">Add workspace</button>'
            : '') +
          (account.has_billing
            ? '<button type="button" class="btn-secondary" data-action="open-billing" data-account-id="' +
              escapeAttr(account.id) +
              '">Manage billing</button>'
            : '') +
          '<button type="button" class="btn-secondary" id="team-btn-' +
          escapeAttr(account.id) +
          '" data-action="toggle-team" data-account-id="' +
          escapeAttr(account.id) +
          '" data-shell-target="team">Manage team</button>' +
        '</div>' +
      '</div>';

    if (account.kind === 'msp') {
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
  }

  return (
    '<section class="account-content-panel account-content-panel-overview">' +
      '<div class="account-command-deck">' +
        '<div class="account-overview-card">' +
          '<div class="account-overview-lead">' +
            '<div class="account-panel-kicker">Hosted posture</div>' +
            '<h3>' + escapeHTML(postureTitle) + '</h3>' +
            '<p>' + escapeHTML(postureCopy) + '</p>' +
            '<div class="account-overview-callout">' + escapeHTML(account.kind === 'msp'
              ? 'Use this console to run client workspaces, account billing, and operator access from one place.'
              : 'Use this console to run hosted workspaces, account billing, and operator access from one place.'
            ) + '</div>' +
          '</div>' +
          '<div class="account-metric-strip">' +
            '<div class="account-stat-card account-stat-card-inline">' +
              '<span class="account-stat-label">Active workspaces</span>' +
              '<span class="account-stat-value">' + String(activeCount) + '</span>' +
            '</div>' +
            '<div class="account-stat-card account-stat-card-inline">' +
              '<span class="account-stat-label">Healthy</span>' +
              '<span class="account-stat-value account-stat-healthy">' + String(healthyCount) + '</span>' +
            '</div>' +
            '<div class="account-stat-card account-stat-card-inline">' +
              '<span class="account-stat-label">Checking</span>' +
              '<span class="account-stat-value account-stat-checking">' + String(checkingCount) + '</span>' +
            '</div>' +
            '<div class="account-stat-card account-stat-card-inline">' +
              '<span class="account-stat-label">Needs attention</span>' +
              '<span class="account-stat-value account-stat-unhealthy">' + String(unhealthyCount) + '</span>' +
            '</div>' +
          '</div>' +
        '</div>' +
        actions +
        addWorkspaceForm +
        '<div class="account-overview-secondary">' +
          '<div class="overview-side-card overview-side-card-primary">' +
            '<div class="account-panel-kicker">Operator overview</div>' +
            '<h4>Start from the next action, not the whole account</h4>' +
            '<p>Use the overview for posture, then move into workspaces, team, or account services depending on what needs attention.</p>' +
            '<div class="overview-quick-actions">' +
              renderOverviewQuickAction('workspaces', 'Open workspaces', 'Review the hosted fleet and move into a workspace') +
              renderOverviewQuickAction('team', 'Review team access', 'Check who can operate billing and hosted workspaces') +
              renderOverviewQuickAction('services', 'Open account services', 'Handle billing, licenses, refunds, and privacy actions') +
            '</div>' +
          '</div>' +
          renderAttentionPanel(workspaces) +
        '</div>' +
      '</div>' +
    '</section>'
  );
}

function renderAccountWorkspaceSection(account: PortalAccountSummary, accountAPIBasePath: string): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var healthyCount = countWorkspacesByHealth(workspaces, 'healthy');
  var checkingCount = countWorkspacesByHealth(workspaces, 'checking');
  var unhealthyCount = countWorkspacesByHealth(workspaces, 'unhealthy');
  var workspaceManagement = '';
  if (account.can_manage) {
    var workspaceDeskActions = '';
    if (account.kind === 'msp') {
      workspaceDeskActions +=
        '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' +
        escapeAttr(account.id) +
        '">Add workspace</button>';
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
            '<h3>Review one workspace at a time</h3>' +
            '<p>Select a workspace from the fleet to review its lifecycle state and run explicit management actions.</p>' +
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
          '<div class="workspace-management-empty-copy">Choose a workspace to manage from the fleet above.</div>' +
          '<div class="workspace-management-empty-actions">' + workspaceDeskActions + '</div>' +
          '<div class="workspace-management-empty-note">Use the fleet table for workspace-level work, or run account-wide billing and team actions from here.</div>' +
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
            '<span class="workspace-list-stat"><strong>' + String(healthyCount) + '</strong> healthy</span>' +
            '<span class="workspace-list-stat"><strong>' + String(checkingCount) + '</strong> checking</span>' +
            '<span class="workspace-list-stat workspace-list-stat-attention"><strong>' + String(unhealthyCount) + '</strong> needs attention</span>' +
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
        '<div>' +
          '<div class="account-panel-kicker">Workspace fleet</div>' +
          '<h3>Hosted fleet</h3>' +
          '<p>Use the fleet view to open workspaces, watch health posture, and keep management actions explicit.</p>' +
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
              '<div class="team-invite-guide">' +
                '<div class="team-invite-guide-row"><strong>Admin</strong><span>Billing and hosted operations.</span></div>' +
                '<div class="team-invite-guide-row"><strong>Tech</strong><span>Operational access without billing control.</span></div>' +
                '<div class="team-invite-guide-row"><strong>Read-only</strong><span>Review hosted state without making changes.</span></div>' +
              '</div>' +
              '<button type="button" class="btn-primary btn-compact" data-action="invite-member" data-account-id="' +
              escapeAttr(account.id) +
              '">Invite</button>' +
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
    var workspaceLabel = workspaceCountLabel((account.workspaces || []).length);
    return (
      '<section class="account-surface">' +
        '<div class="account-surface-header">' +
          '<div class="account-heading">' +
            '<div class="account-eyebrow">' + escapeHTML(accountKindLabel(account)) + '</div>' +
            '<h2>' + escapeHTML(account.name) + '</h2>' +
            '<div class="account-summary">' + escapeHTML(account.kind === 'msp' ? 'Operator workspace account' : 'Hosted account operations') + '</div>' +
          '</div>' +
          '<div class="account-context-strip">' +
            '<span class="account-context-chip">' + escapeHTML(account.kind_label) + '</span>' +
            '<span class="account-context-chip">' + escapeHTML(titleCase(account.role)) + '</span>' +
            '<span class="account-context-chip">' + escapeHTML(workspaceLabel) + '</span>' +
          '</div>' +
        '</div>' +
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
          renderOverviewBand(accounts) +
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
                '<div class="service-list-intro">' +
                  '<div class="account-panel-kicker">Commercial actions</div>' +
                  '<p>Use these tools for self-hosted billing, license recovery, refunds, and privacy. Hosted workspace operations stay in Workspaces and Team.</p>' +
                '</div>' +
                '<div class="service-action-list">' +
                  renderServiceActionRow('open-manage-service', 'Billing', 'Manage subscriptions', 'Open Stripe billing access for existing self-hosted subscriptions without leaving the Pulse Account shell.', 'manage-service-panel', 'manage-inline-email', ['Invoices and plan changes', 'Subscription self-service']) +
                  renderServiceActionRow('open-retrieve-service', 'Licenses', 'Retrieve licenses', 'Recover the latest active self-hosted license and invoice link for a commercial email address.', 'retrieve-service-panel', 'retrieve-inline-email', ['Latest active license', 'Invoice lookup']) +
                  renderServiceActionRow('open-refund-service', 'Refunds', 'Refund requests', 'Request an immediate self-serve refund for eligible self-hosted purchases with explicit revocation confirmation.', 'refund-service-panel', 'refund-inline-email', ['Eligibility check', 'Explicit revocation']) +
                  renderServiceActionRow('open-data-service', 'Privacy', 'Data and privacy', 'Request commercial data export or deletion without leaving the account shell.', 'data-service-panel', 'data-export-email', ['Export or deletion', 'Support escalation path']) +
                '</div>' +
              '</aside>' +
              '<div class="service-shell-main">' +
                '<div class="service-detail-shell">' +
                  '<div class="service-panel service-panel-empty visible" id="service-panel-empty">' +
                    '<div class="account-panel-kicker">Select a service</div>' +
                    '<h3>Choose a commercial account action</h3>' +
                    '<p>Open a billing, license, refund, or privacy flow from the service navigator. The active request stays here so the account-services area behaves like one working surface instead of a list of disconnected tools.</p>' +
                    '<div class="service-empty-points">' +
                      '<div class="service-empty-point"><strong>Billing</strong><span>Open Stripe customer portal access after verification.</span></div>' +
                      '<div class="service-empty-point"><strong>Licenses</strong><span>Recover the latest active self-hosted license and invoice link.</span></div>' +
                      '<div class="service-empty-point"><strong>Privacy</strong><span>Request export or deletion without leaving Pulse Account.</span></div>' +
                    '</div>' +
                    '<div class="service-empty-support">Need help with billing, refund, privacy, or license actions? <a class="portal-support-link" href="mailto:' +
                    escapeAttr(context.bootstrap.support_email || '') +
                    '">' +
                    escapeHTML(context.bootstrap.support_email || '') +
                    '</a></div>' +
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
                '<div class="service-inline-support">' +
                  '<div class="account-panel-kicker">Support</div>' +
                  '<p>Use support if a billing, refund, privacy, or license flow does not behave as expected for this account.</p>' +
                  '<a class="portal-support-link" href="mailto:' +
                  escapeAttr(context.bootstrap.support_email || '') +
                  '">' +
                  escapeHTML(context.bootstrap.support_email || '') +
                  '</a>' +
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
