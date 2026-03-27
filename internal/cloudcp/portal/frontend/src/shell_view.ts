import type {
  PortalAccountSummary,
  PortalBootstrapData,
  PortalLoginState,
  PortalWorkspaceSummary,
} from './types';

export interface ShellViewContext {
  bootstrap: PortalBootstrapData;
  loginState: PortalLoginState;
  signupPath: string;
  accountAPIBasePath: string;
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

function workspaceStatusCopy(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  if (status === 'healthy') return 'Live updates and health checks are currently good.';
  if (status === 'unhealthy') return 'This workspace needs attention before it is trustworthy.';
  return 'This workspace is still waiting on a completed health check.';
}

function renderOverviewBand(accounts: PortalAccountSummary[]): string {
  var hosted = hasHostedAccounts(accounts);
  var workspaceTotal = countWorkspaces(accounts);
  var title = hosted ? 'Pulse Account' : 'Self-hosted Pulse Account';
  var summary = hosted
    ? 'Open hosted workspaces, manage account access, and handle commercial account services from one place.'
    : 'Use this account for self-hosted billing, license recovery, refunds, and privacy actions. Hosted workspace access will appear here when it is attached to this email.';

  return (
    '<section class="portal-hero">' +
      '<div class="portal-hero-copy">' +
        '<div class="portal-hero-kicker">' + (hosted ? 'Hosted access is active on this account.' : 'No hosted workspace access is attached to this account yet.') + '</div>' +
        '<h1>' + title + '</h1>' +
        '<p>' + summary + '</p>' +
      '</div>' +
      '<div class="portal-hero-stats">' +
        '<div class="portal-hero-stat">' +
          '<span class="portal-hero-stat-label">Hosted access</span>' +
          '<span class="portal-hero-stat-value">' + (hosted ? 'Active' : 'Not attached') + '</span>' +
        '</div>' +
        '<div class="portal-hero-stat">' +
          '<span class="portal-hero-stat-label">Accounts</span>' +
          '<span class="portal-hero-stat-value">' + (accounts.length === 1 ? '1 account' : String(accounts.length) + ' accounts') + '</span>' +
        '</div>' +
        '<div class="portal-hero-stat">' +
          '<span class="portal-hero-stat-label">Workspaces</span>' +
          '<span class="portal-hero-stat-value">' + String(workspaceTotal) + '</span>' +
        '</div>' +
      '</div>' +
    '</section>'
  );
}

function renderShellNavigation(accounts: PortalAccountSummary[], supportEmail: string): string {
  var hosted = hasHostedAccounts(accounts);
  return (
    '<nav class="portal-section-nav" aria-label="Pulse Account sections">' +
      '<a class="portal-section-link" href="#hosted-operations-section">' +
        '<span class="portal-section-link-label">' + (hosted ? 'Hosted operations' : 'Hosted access') + '</span>' +
        '<span class="portal-section-link-copy">' + (hosted ? 'Workspaces, teams, and hosted billing' : 'No hosted workspaces are attached yet') + '</span>' +
      '</a>' +
      '<a class="portal-section-link" href="#account-services-section">' +
        '<span class="portal-section-link-label">Account services</span>' +
        '<span class="portal-section-link-copy">Licenses, billing, refunds, and privacy</span>' +
      '</a>' +
      '<a class="portal-section-link" href="mailto:' + escapeAttr(supportEmail || '') + '">' +
        '<span class="portal-section-link-label">Support</span>' +
        '<span class="portal-section-link-copy">' + escapeHTML(supportEmail || '') + '</span>' +
      '</a>' +
    '</nav>'
  );
}

function renderWorkspaceCard(account: PortalAccountSummary, workspace: PortalWorkspaceSummary, accountAPIBasePath: string): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  var createdLabel = formatWorkspaceDate(workspace.created_at);
  var metaParts = [
    '<span class="workspace-meta-item">' + escapeHTML(titleCase(state || 'unknown')) + '</span>',
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
    '<article class="workspace-card">' +
      '<div class="workspace-card-main">' +
        '<div class="workspace-card-topline">' +
          '<div class="workspace-title-group">' +
            '<h4 class="workspace-name">' + escapeHTML(workspace.display_name) + '</h4>' +
            '<div class="workspace-meta">' + metaParts.join('') + '</div>' +
          '</div>' +
          '<div class="workspace-badges">' +
            healthBadgeHTML(workspace) +
            '<span class="badge badge-' + escapeHTML(state || 'unknown') + '">' + escapeHTML(titleCase(state || 'Unknown')) + '</span>' +
          '</div>' +
        '</div>' +
        '<p class="workspace-summary">' + escapeHTML(workspaceStatusCopy(workspace)) + '</p>' +
      '</div>' +
      '<div class="workspace-actions">' +
        openAction +
        manageAction +
      '</div>' +
    '</article>'
  );
}

function renderAccountSection(account: PortalAccountSummary, accountAPIBasePath: string): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var summaryText = accountKindLabel(account) + ' · ' + titleCase(account.role) + ' · ' + workspaceCountLabel(workspaces.length);
  var healthyCount = countWorkspacesByHealth(workspaces, 'healthy');
  var checkingCount = countWorkspacesByHealth(workspaces, 'checking');
  var unhealthyCount = countWorkspacesByHealth(workspaces, 'unhealthy');
  var activeCount = countWorkspacesByState(workspaces, 'active');
  var operationsCopy = account.kind === 'msp'
    ? 'Manage the client fleet from this account surface. Workspace creation, billing, and team actions belong here.'
    : 'Use this account surface to open hosted workspaces, manage billing, and control access for this hosted account.';

  var actions = '';
  var teamSection = '';
  var addWorkspaceForm = '';
  var workspaceManagement = '';
  if (account.can_manage) {
    actions =
      '<div class="account-operations-panel">' +
        '<div class="account-panel-copy">' +
          '<div class="account-panel-kicker">Account operations</div>' +
          '<h3>Run the hosted side from here</h3>' +
          '<p>' + escapeHTML(operationsCopy) + '</p>' +
        '</div>' +
        '<div class="account-actions">' +
          (account.kind === 'msp'
            ? '<button type="button" class="btn-secondary" id="add-ws-btn-' +
              escapeAttr(account.id) +
              '" data-action="toggle-add-workspace" data-account-id="' +
              escapeAttr(account.id) +
              '">Add workspace</button>'
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
          '">Manage team</button>' +
        '</div>' +
      '</div>';

    teamSection =
      '<section class="team-management-panel team-section" id="team-section-' +
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
          '<button type="button" class="btn-secondary btn-compact" data-action="toggle-team" data-account-id="' +
          escapeAttr(account.id) +
          '">Done</button>' +
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
            '<table class="team-table">' +
              '<thead><tr><th>Email</th><th>Role</th><th></th></tr></thead>' +
              '<tbody id="team-list-' +
              escapeAttr(account.id) +
              '">' +
                '<tr><td colspan="3" class="team-message-cell">Loading…</td></tr>' +
              '</tbody>' +
            '</table>' +
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
              '<button type="button" class="btn-primary btn-compact" data-action="invite-member" data-account-id="' +
              escapeAttr(account.id) +
              '">Invite</button>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</section>';

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
        '">Choose a workspace to manage from the fleet above.</div>' +
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

  var workspaceHTML = workspaces.length
    ? '<div class="workspace-list">' + workspaces.map(function(workspace) {
        return renderWorkspaceCard(account, workspace, accountAPIBasePath);
      }).join('') + '</div>'
    : '<div class="empty-state"><p>No hosted workspaces yet. Create one to get started.</p></div>';

  return (
    '<section class="account-surface">' +
      '<div class="account-surface-header">' +
        '<div class="account-heading">' +
          '<div class="account-eyebrow">' + escapeHTML(accountKindLabel(account)) + '</div>' +
          '<h2>' + escapeHTML(account.name) + '</h2>' +
          '<div class="account-summary">' + escapeHTML(summaryText) + '</div>' +
        '</div>' +
        '<div class="account-badges">' +
          '<span class="badge badge-' + escapeHTML(account.kind) + '">' + escapeHTML(account.kind_label) + '</span>' +
          roleBadgeHTML(account.role) +
        '</div>' +
      '</div>' +
      '<div class="account-surface-body">' +
        '<aside class="account-overview-rail">' +
          '<div class="account-overview-card">' +
            '<div class="account-panel-kicker">Hosted posture</div>' +
            '<h3>' + escapeHTML(accountKindLabel(account)) + '</h3>' +
            '<p>' + escapeHTML(summaryText) + '</p>' +
          '</div>' +
          '<div class="account-status-grid">' +
            '<div class="account-stat-card">' +
              '<span class="account-stat-label">Active workspaces</span>' +
              '<span class="account-stat-value">' + String(activeCount) + '</span>' +
            '</div>' +
            '<div class="account-stat-card">' +
              '<span class="account-stat-label">Healthy</span>' +
              '<span class="account-stat-value account-stat-healthy">' + String(healthyCount) + '</span>' +
            '</div>' +
            '<div class="account-stat-card">' +
              '<span class="account-stat-label">Checking</span>' +
              '<span class="account-stat-value account-stat-checking">' + String(checkingCount) + '</span>' +
            '</div>' +
            '<div class="account-stat-card">' +
              '<span class="account-stat-label">Needs attention</span>' +
              '<span class="account-stat-value account-stat-unhealthy">' + String(unhealthyCount) + '</span>' +
            '</div>' +
          '</div>' +
          actions +
          addWorkspaceForm +
        '</aside>' +
        '<div class="account-main-stage">' +
          '<div class="account-stage-header">' +
            '<div>' +
              '<div class="account-panel-kicker">Workspace fleet</div>' +
              '<h3>Open hosted Pulse workspaces</h3>' +
              '<p>Review fleet health, move into a workspace, and keep lifecycle actions explicit.</p>' +
            '</div>' +
          '</div>' +
          workspaceHTML +
          '<div class="account-management-grid">' +
            workspaceManagement +
            teamSection +
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
    return renderAccountSection(account, context.accountAPIBasePath);
  }).join('');
}

export function renderAuthenticatedPortalHTML(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var hosted = hasHostedAccounts(accounts);
  var serviceHeading = hosted ? 'Self-hosted licenses and billing' : 'Account services';
  var serviceNote = hosted
    ? 'Hosted operations live above. Use these commercial tools for self-hosted licenses, billing, refunds, and privacy actions.'
    : 'Use these account tools for self-hosted licenses, billing, refunds, and privacy actions.';

  return (
    '<div class="portal-shell">' +
      renderOverviewBand(accounts) +
      '<div class="portal-frame">' +
        '<aside class="portal-rail">' +
          renderShellNavigation(accounts, context.bootstrap.support_email || '') +
          '<div class="portal-rail-panel">' +
            '<div class="account-panel-kicker">Support</div>' +
            '<h3>One account surface</h3>' +
            '<p>This portal should be the place where hosted access and commercial account actions meet. Hosted workspaces sit above. Self-hosted services remain available below.</p>' +
            '<a class="portal-support-link" href="mailto:' + escapeAttr(context.bootstrap.support_email || '') + '">' + escapeHTML(context.bootstrap.support_email || '') + '</a>' +
          '</div>' +
        '</aside>' +
        '<div class="portal-stage">' +
          '<section class="portal-top-section" id="hosted-operations-section">' +
            '<div class="portal-top-section-header">' +
              '<div class="account-panel-kicker">' + (hosted ? 'Hosted operations' : 'Hosted access') + '</div>' +
              '<h2>' + (hosted ? 'Run hosted accounts from one place' : 'No hosted workspaces are attached yet') + '</h2>' +
              '<p>' + (hosted
                ? 'Use this area for workspace access, fleet operations, hosted billing, and team management.'
                : 'This account does not currently have hosted workspace access. If that is unexpected, contact support while using the commercial tools below.') + '</p>' +
            '</div>' +
            '<div id="accounts-root">' + renderAccountsHTML(context) + '</div>' +
          '</section>' +
          '<section class="service-section" id="account-services-section">' +
            '<div class="service-header">' +
              '<div>' +
                '<div class="account-panel-kicker">Account services</div>' +
                '<h2>' + serviceHeading + '</h2>' +
              '</div>' +
              '<div class="service-note">' + serviceNote + '</div>' +
            '</div>' +
            '<div class="service-grid">' +
              '<button class="service-card service-card-button" type="button" id="open-manage-service" data-account-service-action="open-service-panel" data-account-service-panel="manage-service-panel" data-account-service-focus="manage-inline-email">' +
                '<span class="service-card-kicker">Billing</span>' +
                '<h3>Manage subscriptions</h3>' +
                '<p>Open Stripe billing access for existing self-hosted subscriptions without leaving the Pulse Account shell.</p>' +
              '</button>' +
              '<button class="service-card service-card-button" type="button" id="open-retrieve-service" data-account-service-action="open-service-panel" data-account-service-panel="retrieve-service-panel" data-account-service-focus="retrieve-inline-email">' +
                '<span class="service-card-kicker">Licenses</span>' +
                '<h3>Retrieve licenses</h3>' +
                '<p>Recover the latest active self-hosted license and invoice link for a commercial email address.</p>' +
              '</button>' +
              '<button class="service-card service-card-button" type="button" id="open-refund-service" data-account-service-action="open-service-panel" data-account-service-panel="refund-service-panel" data-account-service-focus="refund-inline-email">' +
                '<span class="service-card-kicker">Refunds</span>' +
                '<h3>Refund requests</h3>' +
                '<p>Request an immediate self-serve refund for eligible self-hosted purchases with explicit revocation confirmation.</p>' +
              '</button>' +
              '<button class="service-card service-card-button" type="button" id="open-data-service" data-account-service-action="open-service-panel" data-account-service-panel="data-service-panel" data-account-service-focus="data-export-email">' +
                '<span class="service-card-kicker">Privacy</span>' +
                '<h3>Data and privacy</h3>' +
                '<p>Request commercial data export or deletion without leaving the account shell.</p>' +
              '</button>' +
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
