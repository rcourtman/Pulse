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

function formatWorkspaceDate(value: unknown): string {
  if (!value) return '';
  var date = new Date(String(value));
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function roleBadgeHTML(role: string): string {
  if (role === 'owner') return '<span class="badge" style="background:#f1f5f9;color:#64748b">Owner</span>';
  if (role === 'admin') return '<span class="badge" style="background:#f1f5f9;color:#64748b">Admin</span>';
  if (role === 'tech') return '<span class="badge" style="background:#f1f5f9;color:#64748b">Tech</span>';
  return '';
}

function renderWorkspaceCard(account: PortalAccountSummary, workspace: PortalWorkspaceSummary, accountAPIBasePath: string): string {
  var state = String(workspace.state || '');
  var safeState = escapeHTML(state);
  var createdLabel = formatWorkspaceDate(workspace.created_at);
  var openAction = '';
  if (state === 'active') {
    openAction =
      '<form method="POST" action="' +
      escapeAttr(accountAPIBasePath + '/' + account.id + '/tenants/' + workspace.id + '/handoff') +
      '">' +
      '<button type="submit" class="btn-primary">Open →</button>' +
      '</form>';
  } else {
    openAction = '<span style="font-size:13px;color:#94a3b8">' + safeState + '</span>';
  }

  var manageAction = '';
  if (account.can_manage && (state === 'active' || state === 'suspended' || state === 'failed')) {
    manageAction =
      '<button type="button" class="btn-danger" data-action="workspace-manage" data-account-id="' +
      escapeAttr(account.id) +
      '" data-workspace-id="' +
      escapeAttr(workspace.id) +
      '" data-workspace-state="' +
      escapeAttr(state) +
      '" data-workspace-name="' +
      escapeAttr(workspace.display_name) +
      '">⋯</button>';
  }

  var createdMeta = createdLabel ? '<span class="ws-created">Created ' + escapeHTML(createdLabel) + '</span>' : '';
  return (
    '<div class="workspace-card">' +
      '<div class="ws-info">' +
        '<span class="ws-name">' + escapeHTML(workspace.display_name) + '</span>' +
        '<div class="ws-meta">' +
          (workspace.healthy
            ? '<span class="badge badge-healthy">Healthy</span>'
            : '<span class="badge badge-unhealthy">Checking</span>') +
          '<span class="badge badge-' + safeState + '">' + safeState + '</span>' +
          createdMeta +
        '</div>' +
      '</div>' +
      '<div class="ws-actions">' +
        openAction +
        manageAction +
      '</div>' +
    '</div>'
  );
}

function renderAccountSection(account: PortalAccountSummary, accountAPIBasePath: string): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var workspaceHTML = '';
  if (workspaces.length === 0) {
    workspaceHTML = '<div class="empty-state"><p>No workspaces yet. Create one to get started.</p></div>';
  } else {
    workspaceHTML =
      '<div class="workspace-list">' +
      workspaces.map(function(workspace) {
        return renderWorkspaceCard(account, workspace, accountAPIBasePath);
      }).join('') +
      '</div>';
  }

  var actions = '';
  var teamSection = '';
  var addWorkspaceForm = '';
  if (account.can_manage) {
    actions =
      '<div class="account-actions">' +
      (account.kind === 'msp'
        ? '<button type="button" class="btn-secondary" id="add-ws-btn-' +
          escapeAttr(account.id) +
          '" data-action="toggle-add-workspace" data-account-id="' +
          escapeAttr(account.id) +
          '">+ Add workspace</button>'
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
      '</div>';

    teamSection =
      '<div class="team-section" id="team-section-' +
      escapeAttr(account.id) +
      '" data-actor-role="' +
      escapeAttr(account.role) +
      '">' +
      '<h3>Team members</h3>' +
      '<table class="team-table">' +
      '<thead><tr><th>Email</th><th>Role</th><th></th></tr></thead>' +
      '<tbody id="team-list-' +
      escapeAttr(account.id) +
      '">' +
      '<tr><td colspan="3" style="color:#94a3b8;text-align:center;padding:16px">Loading…</td></tr>' +
      '</tbody>' +
      '</table>' +
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
      '<button type="button" class="btn-primary" style="padding:8px 14px;font-size:13px" data-action="invite-member" data-account-id="' +
      escapeAttr(account.id) +
      '">Invite</button>' +
      '</div>' +
      '</div>';

    if (account.kind === 'msp') {
      addWorkspaceForm =
        '<div class="add-workspace-form" id="add-ws-form-' +
        escapeAttr(account.id) +
        '">' +
        '<label for="ws-name-' +
        escapeAttr(account.id) +
        '">Workspace name (e.g. client name)</label>' +
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
        '"></div>' +
        '</div>' +
        '</div>';
    }
  }

  return (
    '<section class="account-section">' +
      '<div class="account-header">' +
        '<h2>' + escapeHTML(account.name) + '</h2>' +
        '<span class="badge badge-' + escapeHTML(account.kind) + '">' + escapeHTML(account.kind_label) + '</span>' +
        roleBadgeHTML(account.role) +
      '</div>' +
      workspaceHTML +
      actions +
      teamSection +
      addWorkspaceForm +
    '</section>'
  );
}

export function renderHeaderHTML(context: ShellViewContext): string {
  if (context.bootstrap.authenticated) {
    return (
      '<span>' + escapeHTML(context.bootstrap.email || '') + '</span>' +
      '<button class="logout-btn" id="logout-btn" type="button">Sign out</button>'
    );
  }
  return '<a class="logout-btn" href="' + escapeAttr(context.signupPath) + '" style="text-decoration:none">Create account</a>';
}

export function renderAccountsHTML(context: ShellViewContext): string {
  var safeAccounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  if (safeAccounts.length === 0) {
    return (
      '<div class="empty-state" style="margin-top:48px">' +
        '<p>No workspaces found. If you just signed up, check your email for setup instructions.</p>' +
        '<p style="margin-top:12px;font-size:13px">Need help? Contact <a href="mailto:' +
        escapeAttr(context.bootstrap.support_email || '') +
        '" style="color:#1d4ed8">' +
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
  return (
    '<section class="intro-card">' +
      '<h1>Pulse Account</h1>' +
      '<p>Manage Cloud workspaces, MSP access, and self-hosted commercial account services from one account surface. Hosted workspace lifecycle lives here today, and the self-hosted billing, license recovery, refund, and privacy tools below now share the same Pulse Account shell instead of staying fragmented across public utility pages.</p>' +
    '</section>' +
    '<div id="accounts-root">' + renderAccountsHTML(context) + '</div>' +
    '<section class="service-section">' +
      '<div class="service-header">' +
        '<h2>Other account services</h2>' +
        '<div class="service-note">Self-hosted commercial account actions now live here. The public utility pages remain as compatibility entry points, not the primary account surface.</div>' +
      '</div>' +
      '<div class="service-grid">' +
        '<button class="service-card service-card-button" type="button" id="open-manage-service" data-account-service-action="open-service-panel" data-account-service-panel="manage-service-panel" data-account-service-focus="manage-inline-email">' +
          '<h3>Manage subscriptions</h3>' +
          '<p>Open Stripe billing access for existing self-hosted subscriptions without leaving the Pulse Account shell.</p>' +
        '</button>' +
        '<button class="service-card service-card-button" type="button" id="open-retrieve-service" data-account-service-action="open-service-panel" data-account-service-panel="retrieve-service-panel" data-account-service-focus="retrieve-inline-email">' +
          '<h3>Retrieve licenses</h3>' +
          '<p>Recover the latest active self-hosted license and invoice link for a commercial email address.</p>' +
        '</button>' +
        '<button class="service-card service-card-button" type="button" id="open-refund-service" data-account-service-action="open-service-panel" data-account-service-panel="refund-service-panel" data-account-service-focus="refund-inline-email">' +
          '<h3>Refund requests</h3>' +
          '<p>Request an immediate self-serve refund for eligible self-hosted purchases with explicit revocation confirmation.</p>' +
        '</button>' +
        '<button class="service-card service-card-button" type="button" id="open-data-service" data-account-service-action="open-service-panel" data-account-service-panel="data-service-panel" data-account-service-focus="data-export-email">' +
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
    '</section>'
  );
}

export function renderSignedOutPortalHTML(context: ShellViewContext): string {
  var statusHTML = '';
  if (context.loginState.error) {
    statusHTML = '<div class="service-status visible error">' + escapeHTML(context.loginState.error) + '</div>';
  } else if (context.loginState.success) {
    statusHTML =
      '<div class="service-status visible success">' +
        'Magic link sent. Check your inbox and click the link to sign in.' +
        '<br><br><strong>Don\'t see it?</strong> <a href="#" data-portal-action="resend-magic-link">Send a new link</a>.' +
      '</div>';
  }
  return (
    '<section class="intro-card">' +
      '<h1>Pulse Account</h1>' +
      '<p>Sign in to manage Cloud workspaces, MSP access, and commercial account services from one account surface.</p>' +
    '</section>' +
    '<section class="service-section">' +
      '<div class="service-panel visible">' +
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
          (context.loginState.sending ? 'Sending…' : 'Send magic link') +
          '</button>' +
          '<a class="btn-secondary" href="' + escapeAttr(context.signupPath) + '" style="text-decoration:none">Create an account</a>' +
        '</div>' +
        statusHTML +
      '</div>' +
    '</section>'
  );
}
