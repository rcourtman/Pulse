import type {
  PortalAccountSummary,
  PortalBootstrapData,
  PortalLoginState,
  PortalShellSection,
  PortalWorkspaceSummary,
} from './types';
import { portalRoleLabel } from './account_roles';

export interface ShellViewContext {
  bootstrap: PortalBootstrapData;
  loginState: PortalLoginState;
  signupPath: string;
  accountAPIBasePath: string;
  activeSection?: PortalShellSection;
}

interface OverviewWorkspaceEntry {
  account: PortalAccountSummary;
  workspace: PortalWorkspaceSummary;
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

function hasSelfHostedCommercial(bootstrap: PortalBootstrapData): boolean {
  var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
  return bootstrap.has_self_hosted_commercial === true || !hasHostedAccounts(accounts);
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

function collectOverviewWorkspaceEntries(accounts: PortalAccountSummary[]): OverviewWorkspaceEntry[] {
  var results: OverviewWorkspaceEntry[] = [];
  for (var i = 0; i < accounts.length; i += 1) {
    var workspaces = Array.isArray(accounts[i].workspaces) ? accounts[i].workspaces : [];
    for (var j = 0; j < workspaces.length; j += 1) {
      results.push({
        account: accounts[i],
        workspace: workspaces[j],
      });
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

function accountContextRoleMeta(account: PortalAccountSummary): string {
  return portalRoleLabel(account.role) + (account.can_manage ? ' access' : ' role');
}

function accountContextLeadCopy(account: PortalAccountSummary): string {
  var accountPrefix = account.kind === 'msp' ? 'Hosted workspace account' : 'Hosted account';
  if (account.can_manage) {
    return accountPrefix + (account.has_billing
      ? ' for workspace access, access control, and billing.'
      : ' for workspace access and access control.');
  }
  return accountPrefix + (account.has_billing
    ? ' where you can open workspaces and review who already has access. An owner or admin handles access changes and billing.'
    : ' where you can open workspaces and review who already has access. An owner or admin handles account changes.');
}

function accountContextAccessSummary(account: PortalAccountSummary): string {
  return account.can_manage ? portalRoleLabel(account.role) : 'View only';
}

function accountContextBillingSummary(account: PortalAccountSummary): string {
  if (!account.has_billing) return 'Not attached';
  return account.can_manage ? 'Billing enabled' : 'Owner/admin required';
}

function renderAccountContextStrip(account: PortalAccountSummary): string {
  var workspaceLabel = workspaceCountLabel((account.workspaces || []).length);
  var billingLabel = accountContextBillingSummary(account);

  return (
    '<section class="portal-account-context">' +
      '<div class="portal-account-context-copy">' +
        '<div class="portal-account-context-meta">' +
          '<span class="account-eyebrow">' + escapeHTML(accountKindLabel(account)) + '</span>' +
          '<span class="portal-account-context-separator">/</span>' +
          '<span class="portal-account-context-access">' + escapeHTML(accountContextRoleMeta(account)) + '</span>' +
        '</div>' +
        '<div class="portal-account-context-row">' +
          '<h2>' + escapeHTML(account.name) + '</h2>' +
          '<div class="portal-account-context-chips">' +
            '<span class="account-context-chip">' + escapeHTML(account.kind_label) + '</span>' +
            '<span class="account-context-chip">' + escapeHTML(portalRoleLabel(account.role)) + '</span>' +
            '<span class="account-context-chip">' + escapeHTML(workspaceLabel) + '</span>' +
          '</div>' +
        '</div>' +
        '<p>' + escapeHTML(accountContextLeadCopy(account)) + '</p>' +
      '</div>' +
      '<div class="portal-account-context-summary">' +
        '<div class="portal-account-context-stat">' +
          '<span>Access</span>' +
          '<strong>' + escapeHTML(accountContextAccessSummary(account)) + '</strong>' +
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

function workspaceNavCopy(hosted: boolean, canManage: boolean): string {
  if (!hosted) {
    return 'Unavailable on this account. Hosted workspaces are not attached here.';
  }
  if (canManage) {
    return 'Open a workspace, review lifecycle state, or create one.';
  }
  return 'Open a workspace and review current state. An owner or admin must create or change hosted workspaces.';
}

function accessNavCopy(hosted: boolean, canManage: boolean): string {
  if (!hosted) {
    return 'Unavailable on this account. Hosted roster and role controls live only on hosted workspace accounts.';
  }
  if (canManage) {
    return 'Invite people, change roles, and remove account access.';
  }
  return 'Review who already has access to this hosted account. An owner or admin must make changes.';
}

function billingNavCopy(hostedBillingCount: number, canManageHostedBilling: boolean): string {
  if (hostedBillingCount > 0) {
    if (canManageHostedBilling) {
      return 'Hosted billing first, then self-hosted licenses, refunds, and privacy only when relevant.';
    }
    return 'Hosted billing is attached here, but an owner or admin must open it.';
  }
  return 'Self-hosted billing, licenses, refunds, and privacy.';
}

function supportNavCopy(hosted: boolean, canManageHostedTasks: boolean): string {
  if (!hosted) {
    return 'Escalation only after the billing path is exhausted.';
  }
  if (canManageHostedTasks) {
    return 'Escalation only after the workspace, access, or billing path is exhausted.';
  }
  return 'Escalation only after the review, owner/admin, or billing path is exhausted.';
}

function renderShellNavigation(accounts: PortalAccountSummary[], supportEmail: string, activeSection: PortalShellSection): string {
  var hosted = hasHostedAccounts(accounts);
  var workspaces = collectWorkspaces(accounts);
  var totalWorkspaces = workspaces.length;
  var readyWorkspaces = countReadyWorkspaces(workspaces);
  var attentionCount = attentionWorkspaces(workspaces).length;
  var hostedBillingCount = 0;
  var canManageHostedBilling = false;
  var canManage = false;
  for (var i = 0; i < accounts.length; i += 1) {
    if (accounts[i].can_manage) {
      canManage = true;
    }
    if (accounts[i].has_billing) {
      hostedBillingCount += 1;
      if (accounts[i].can_manage) {
        canManageHostedBilling = true;
      }
    }
  }
  return (
    '<aside class="portal-shell-nav" aria-label="Pulse Account sections">' +
      '<div class="portal-shell-nav-header">' +
        '<div class="portal-shell-nav-eyebrow">Pulse Account</div>' +
        '<div class="portal-shell-nav-title">Account tasks</div>' +
        '<div class="portal-shell-nav-support">' + (hosted ? 'Start with the job you need to finish: workspace work, access, billing, then escalation.' : 'Use billing tools first and escalate only when the self-serve path stops.') + '</div>' +
      '</div>' +
      '<div class="portal-shell-nav-group">' +
        shellSectionButton('overview', activeSection, '01', 'Overview', 'What needs attention, what is ready, and the next obvious action.', attentionCount > 0 ? String(attentionCount) + ' review' : (hosted ? String(readyWorkspaces) + ' ready' : 'Summary')) +
        shellSectionButton('workspaces', activeSection, '02', 'Workspaces', workspaceNavCopy(hosted, canManage), hosted ? String(readyWorkspaces) + ' ready' : 'Unavailable') +
        shellSectionButton('access', activeSection, '03', 'Access', accessNavCopy(hosted, canManage), hosted ? (canManage ? 'Manage' : 'View') : 'Unavailable') +
        shellSectionButton('billing', activeSection, '04', 'Billing', billingNavCopy(hostedBillingCount, canManageHostedBilling), hostedBillingCount > 0 ? (hostedBillingCount > 1 ? 'Hosted +' : 'Hosted') : 'Self-hosted') +
        shellSectionButton('support', activeSection, '05', 'Support', supportNavCopy(hosted, canManage), supportEmail ? 'Email' : 'Help') +
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

function attentionOverviewEntries(entries: OverviewWorkspaceEntry[]): OverviewWorkspaceEntry[] {
  var results: OverviewWorkspaceEntry[] = [];
  for (var i = 0; i < entries.length; i += 1) {
    var status = workspaceHealthState(entries[i].workspace);
    if (status === 'unhealthy' || status === 'checking') {
      results.push(entries[i]);
    }
  }
  return results;
}

function readyOverviewEntries(entries: OverviewWorkspaceEntry[]): OverviewWorkspaceEntry[] {
  var results: OverviewWorkspaceEntry[] = [];
  for (var i = 0; i < entries.length; i += 1) {
    if (String(entries[i].workspace.state || '') === 'active' && workspaceHealthState(entries[i].workspace) === 'healthy') {
      results.push(entries[i]);
    }
  }
  return results;
}

function overviewWorkspaceContext(entry: OverviewWorkspaceEntry, includeAccountName: boolean, note: string): string {
  if (!includeAccountName) return note;
  return entry.account.name + ' · ' + note;
}

function overviewBillingSeparationCopy(
  accounts: PortalAccountSummary[],
  showSelfHostedCommercial: boolean
): { title: string; copy: string } {
  var hostedBillingCount = 0;
  var canManageHostedBilling = false;
  for (var i = 0; i < accounts.length; i += 1) {
    if (accounts[i].has_billing) {
      hostedBillingCount += 1;
      if (accounts[i].can_manage) {
        canManageHostedBilling = true;
      }
    }
  }

  if (!accounts.length) {
    return {
      title: 'Billing stays separate',
      copy: 'Self-hosted billing, licenses, refunds, and privacy stay in Billing.',
    };
  }

  if (showSelfHostedCommercial) {
    if (hostedBillingCount > 0) {
      return {
        title: 'Billing stays separate',
        copy: canManageHostedBilling
          ? 'Hosted billing stays in Billing, and self-hosted tools appear there only when relevant.'
          : 'Hosted billing stays in Billing, an owner or admin opens it, and self-hosted tools appear there only when relevant.',
      };
    }
    return {
      title: 'Billing stays separate',
      copy: 'Self-hosted tools appear in Billing only when they are relevant to this account.',
    };
  }

  if (hostedBillingCount > 0) {
    return {
      title: 'Hosted billing stays separate',
      copy: canManageHostedBilling
        ? 'Use Billing only for hosted invoices, payment methods, or subscription changes.'
        : 'Hosted billing stays in Billing, and an owner or admin must open it.',
    };
  }

  return {
    title: 'Billing stays separate',
    copy: 'Use Billing only when the task is commercial, not operational.',
  };
}

function renderOverviewAttentionCard(
  accounts: PortalAccountSummary[],
  entries: OverviewWorkspaceEntry[],
  showSelfHostedCommercial: boolean
): string {
  var attention = attentionOverviewEntries(entries);
  var includeAccountName = accounts.length > 1;
  var suspendedCount = countWorkspacesByState(entries.map(function(entry) {
    return entry.workspace;
  }), 'suspended');
  var billingSeparation = overviewBillingSeparationCopy(accounts, showSelfHostedCommercial);
  if (!attention.length) {
    return (
      '<article class="overview-task-card">' +
        '<div class="account-panel-kicker">Needs attention</div>' +
        '<h4>Nothing urgent</h4>' +
        '<p>' + escapeHTML(entries.length > 0
          ? 'No active workspace is currently asking for review.'
          : 'No hosted workspace is currently asking for review.'
        ) + '</p>' +
        '<div class="overview-task-list">' +
          '<div class="overview-task-item"><strong>Healthy now</strong><span>' + escapeHTML(entries.length > 0
            ? 'Active workspaces look clear for routine use.'
            : 'There is no hosted workspace waiting for review yet.'
          ) + '</span></div>' +
          '<div class="overview-task-item"><strong>' + escapeHTML(suspendedCount > 0 ? 'Suspended stays parked' : billingSeparation.title) + '</strong><span>' + escapeHTML(suspendedCount > 0
            ? String(suspendedCount) + ' suspended workspace' + (suspendedCount === 1 ? ' stays' : 's stay') + ' out of the way until you deliberately resume it.'
            : billingSeparation.copy
          ) + '</span></div>' +
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
        attention.slice(0, 3).map(function(entry) {
          return (
            '<div class="overview-task-item">' +
              '<strong>' + escapeHTML(entry.workspace.display_name) + '</strong>' +
              '<span>' + escapeHTML(overviewWorkspaceContext(entry, includeAccountName, workspaceStatusCopy(entry.workspace))) + '</span>' +
            '</div>'
          );
        }).join('') +
      '</div>' +
    '</article>'
  );
}

function renderOverviewReadyCard(
  accounts: PortalAccountSummary[],
  entries: OverviewWorkspaceEntry[],
  accountAPIBasePath: string
): string {
  var ready = readyOverviewEntries(entries);
  var includeAccountName = accounts.length > 1;
  if (!ready.length) {
    return (
      '<article class="overview-task-card">' +
        '<div class="account-panel-kicker">Ready</div>' +
        '<h4>' + escapeHTML(accounts.length > 0 ? 'No workspace is ready yet' : 'Billing tools are ready') + '</h4>' +
        '<p>' + escapeHTML(accounts.length > 0
          ? 'Use Workspaces to review current state before you start routine work.'
          : 'Use Billing for self-hosted subscriptions, licenses, refunds, and privacy requests.'
        ) + '</p>' +
      '</article>'
    );
  }

  return (
    '<article class="overview-task-card">' +
      '<div class="account-panel-kicker">Ready</div>' +
      '<h4>Open and work</h4>' +
      '<p>These workspaces are active and healthy right now.</p>' +
      '<div class="overview-task-list">' +
        ready.slice(0, 3).map(function(entry) {
          return (
            '<div class="overview-task-item overview-task-item-action">' +
              '<div class="overview-task-copy">' +
                '<strong>' + escapeHTML(entry.workspace.display_name) + '</strong>' +
                '<span>' + escapeHTML(overviewWorkspaceContext(entry, includeAccountName, workspaceRowNote(entry.workspace))) + '</span>' +
              '</div>' +
              renderWorkspaceHandoffForm(entry.account.id, entry.workspace.id, accountAPIBasePath, 'Open workspace') +
            '</div>'
          );
        }).join('') +
      '</div>' +
    '</article>'
  );
}

function renderOverviewNextActionCard(accounts: PortalAccountSummary[], entries: OverviewWorkspaceEntry[], accountAPIBasePath: string): string {
  var attention = attentionOverviewEntries(entries);
  var ready = readyOverviewEntries(entries);
  var primaryAction = '';
  var secondaryAction = '';
  var title = '';
  var description = '';
  var creatableAccount = accounts.find(function(account) {
    return account.kind === 'msp' && account.can_manage;
  }) || null;
  var billingAccount = accounts.find(function(account) {
    return account.has_billing && account.can_manage;
  }) || null;
  var accessAccount = accounts.find(function(account) {
    return account.can_manage;
  }) || null;

  if (attention.length) {
    title = 'Review workspace health';
    description = attention.length > 1
      ? 'Open Workspaces and resolve the pending health or lifecycle questions before you do anything else.'
      : 'Start in Workspaces with ' + attention[0].workspace.display_name + ' before you do anything else.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">Review workspaces</button>';
    secondaryAction = accessAccount
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Review access</button>'
      : '';
  } else if (ready.length) {
    title = 'Open the next workspace';
    description = accounts.length > 1
      ? 'The clearest next step is to open ' + ready[0].workspace.display_name + ' in ' + ready[0].account.name + ' and continue the actual work there.'
      : 'The most obvious next step is to open a ready workspace and continue the actual work there.';
    primaryAction = renderWorkspaceHandoffForm(ready[0].account.id, ready[0].workspace.id, accountAPIBasePath, 'Open ' + ready[0].workspace.display_name, 'btn-primary btn-compact');
    secondaryAction = ready.length > 1
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">See all workspaces</button>'
      : '';
  } else if (creatableAccount) {
    title = 'Create the next workspace';
    description = 'There is no ready workspace yet, so the next clear action is to create one in ' + creatableAccount.name + '.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(creatableAccount.id) + '">Create workspace</button>';
    secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Manage access</button>';
  } else if (billingAccount) {
    title = 'Handle billing in its own place';
    description = 'Operational work is clear, so the next separate task is billing if that is what you came here to change.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
    secondaryAction = accessAccount
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Review access</button>'
      : '';
  } else if (!accounts.length) {
    title = 'Open billing';
    description = 'No hosted workspace is attached, so the next obvious action is Billing for self-hosted subscriptions, licenses, refunds, or privacy requests.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
  } else if (accessAccount) {
    title = 'Handle access in its own place';
    description = 'If the next task is people or roles, keep it in Access instead of mixing it into routine workspace work.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open access</button>';
    secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
  } else {
    title = 'Choose the right task path';
    description = 'If this is an access change, go to Access. If it is a billing or license issue, go to Billing. Support is only for escalation.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
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

function renderShellOverviewSection(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var entries = collectOverviewWorkspaceEntries(accounts);
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
  var totalCount = entries.length;
  var readyCount = readyOverviewEntries(entries).length;
  var attentionCount = attentionOverviewEntries(entries).length;
  var suspendedCount = countWorkspacesByState(collectWorkspaces(accounts), 'suspended');
  var chips = accounts.length
    ? [
      accounts.length === 1 ? '1 account' : String(accounts.length) + ' accounts',
      workspaceCountLabel(totalCount),
      String(readyCount) + ' ready',
      attentionCount > 0 ? String(attentionCount) + ' attention' : 'Nothing urgent',
      suspendedCount > 0 ? String(suspendedCount) + ' suspended' : 'No suspended',
    ]
    : ['No hosted account', 'Billing available', 'Support only on escalation'];

  return (
    '<section class="account-content-panel account-content-panel-overview">' +
      '<div class="account-stage-header account-stage-header-overview overview-stage-header">' +
        '<div>' +
          '<div class="account-panel-kicker">Overview</div>' +
          '<h3>Account triage</h3>' +
          '<p>Only three questions matter here.</p>' +
          renderSectionContextChips(chips) +
        '</div>' +
      '</div>' +
      '<div class="overview-task-grid">' +
        renderOverviewAttentionCard(accounts, entries, showSelfHostedCommercial) +
        renderOverviewReadyCard(accounts, entries, context.accountAPIBasePath) +
        renderOverviewNextActionCard(accounts, entries, context.accountAPIBasePath) +
      '</div>' +
    '</section>'
  );
}

function renderNoHostedWorkspacesSection(): string {
  return (
    '<section class="account-content-panel account-content-panel-workspaces">' +
      '<div class="account-stage-header">' +
        '<div>' +
          '<div class="account-panel-kicker">Workspaces</div>' +
          '<h3>Workspaces</h3>' +
          '<p>No hosted workspace is attached to this account.</p>' +
          renderSectionContextChips(['None attached', 'Billing instead']) +
        '</div>' +
      '</div>' +
      '<div class="empty-state empty-state-spaced">' +
        '<p>There is nothing to open or manage here yet.</p>' +
        '<p class="support-copy">Use Billing for self-hosted subscriptions, licenses, refunds, or privacy requests.</p>' +
      '</div>' +
    '</section>'
  );
}

function renderNoHostedAccessSection(): string {
  return (
    '<section class="account-content-panel account-content-panel-access">' +
      '<div class="account-stage-header">' +
        '<div>' +
          '<div class="account-panel-kicker">Access</div>' +
          '<h3>Access</h3>' +
          '<p>No hosted account roster is attached here.</p>' +
          renderSectionContextChips(['No hosted roster', 'Billing instead']) +
        '</div>' +
      '</div>' +
      '<div class="empty-state empty-state-spaced">' +
        '<p>There are no hosted roles or invites to manage for this account right now.</p>' +
        '<p class="support-copy">If the task is commercial access to licenses, refunds, or privacy, stay in Billing.</p>' +
      '</div>' +
    '</section>'
  );
}

function renderAccountWorkspaceSection(account: PortalAccountSummary, accountAPIBasePath: string): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var readyCount = countReadyWorkspaces(workspaces);
  var suspendedCount = countWorkspacesByState(workspaces, 'suspended');
  var sectionCopy = account.can_manage
    ? 'Open a workspace, review lifecycle state, or create a new one without mixing in access or billing work.'
    : 'Open a workspace and review current state here. An owner or admin must create or change hosted workspaces.';
  var workspaceListSummary = account.can_manage
    ? 'Open a workspace to work in it. Use the lifecycle view only when you are reviewing state or making account-level changes.'
    : 'Open a workspace to do the actual work. An owner or admin must handle lifecycle or creation changes.';
  var workspaceManagement = '';
  var addWorkspaceForm = '';
  var workspaceHeaderActions = '';
  if (account.can_manage) {
    if (account.kind === 'msp') {
      workspaceHeaderActions +=
        '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' +
        escapeAttr(account.id) +
        '">Create workspace</button>';
    }

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
    workspaceManagement =
      '<section class="workspace-management-panel workspace-management-panel-idle" id="workspace-management-' +
      escapeAttr(account.id) +
      '" hidden>' +
        '<div class="workspace-management-header">' +
          '<div>' +
            '<div class="account-panel-kicker">Workspace task</div>' +
            '<h3>Work on one workspace</h3>' +
            '<p>Open lifecycle for one workspace, or create a new one. Keep access and billing separate.</p>' +
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
          '<div class="workspace-management-empty-shell">' +
            '<div class="workspace-management-empty-actions-card">' +
              '<div class="workspace-management-empty-actions-copy">' +
                '<div class="account-panel-kicker">Create workspace</div>' +
                '<h4>Open a new hosted workspace</h4>' +
                '<p>Create one workspace here when you need a new customer or operating boundary.</p>' +
              '</div>' +
              addWorkspaceForm +
            '</div>' +
            '<div class="workspace-management-empty-note">Access changes stay in Access. Billing changes stay in Billing.</div>' +
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
            '<div class="workspace-list-summary">' + escapeHTML(workspaceListSummary) + '</div>' +
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
    : '<div class="empty-state"><p>' + escapeHTML(account.can_manage ? 'No hosted workspaces yet. Create one to get started.' : 'No hosted workspaces are attached yet. An owner or admin must create the first one.') + '</p></div>';

  return (
    '<section class="account-content-panel account-content-panel-workspaces">' +
      '<div class="account-stage-header">' +
        '<div class="account-stage-header-row">' +
          '<div>' +
            '<div class="account-panel-kicker">Workspaces</div>' +
            '<h3>Workspaces</h3>' +
            '<p>' + escapeHTML(sectionCopy) + '</p>' +
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
          '" hidden>' +
          workspaceManagement +
        '</div>' +
      '</div>' +
    '</section>'
  );
}

function renderAccountAccessSection(account: PortalAccountSummary): string {
  var accessHeaderTitle = account.can_manage ? 'Manage access' : 'Review access';
  var accessHeaderCopy = account.can_manage
    ? 'Review the hosted roster, then open one access job at a time.'
    : 'Review who already has access to this hosted account. An owner or admin must make changes.';
  var accessTaskStrip = account.can_manage
    ? (
      '<div class="access-task-strip">' +
        '<button type="button" class="access-task-button" id="access-task-invite-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="invite">Invite people</button>' +
        '<button type="button" class="access-task-button" id="access-task-change_role-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="change_role">Change roles</button>' +
        '<button type="button" class="access-task-button" id="access-task-remove-' + escapeAttr(account.id) + '" data-action="set-access-job" data-account-id="' + escapeAttr(account.id) + '" data-access-job="remove">Remove access</button>' +
      '</div>'
    )
    : renderSectionContextChips(['View roster', 'Owner or admin required']);
  var accessRoleGuide =
    '<div class="access-policy-panel">' +
      '<div class="access-panel-heading">' +
        '<h4>' + (account.can_manage ? 'Choose the smallest role' : 'Role meanings') + '</h4>' +
        '<p>' + (account.can_manage
          ? 'Match each person to the narrowest role that still lets them do the job they own.'
          : 'Use these role meanings to understand what each person on this roster can do.') + '</p>' +
      '</div>' +
      '<div class="access-policy-list">' +
        '<div class="access-policy-row"><strong>Owner</strong><span>Full account, billing, and access control.</span></div>' +
        '<div class="access-policy-row"><strong>Admin</strong><span>Workspace control, billing, and roster management.</span></div>' +
        '<div class="access-policy-row"><strong>Tech</strong><span>Workspace control without billing or roster ownership.</span></div>' +
        '<div class="access-policy-row"><strong>Read-only</strong><span>Review access without control-plane changes.</span></div>' +
      '</div>' +
    '</div>';
  var accessInvitePanel = account.can_manage
    ? (
      '<div class="access-invite-panel">' +
        '<div class="access-panel-heading">' +
          '<h4>Invite people</h4>' +
          '<p>Add one person with the minimum role they need on this account.</p>' +
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
      '</div>'
    )
    : '';
  var accessChangeRolePanel =
    '<div class="access-job-note-panel">' +
      '<div class="access-panel-heading">' +
        '<h4>Change roles on the roster</h4>' +
        '<p>Use the role column in the roster to change one person at a time. Keep each person on the smallest role they need.</p>' +
      '</div>' +
    '</div>' +
    accessRoleGuide;
  var accessRemovePanel =
    '<div class="access-job-note-panel">' +
      '<div class="access-panel-heading">' +
        '<h4>Remove stale access</h4>' +
        '<p>Use removal only when this person should no longer be on this hosted account. Owners may still be protected when they are the last owner.</p>' +
      '</div>' +
      '<div class="access-remove-points">' +
        '<div class="access-remove-point"><strong>Pick the exact person</strong><span>Use the roster to remove one account member at a time.</span></div>' +
        '<div class="access-remove-point"><strong>Keep current owners safe</strong><span>The last owner cannot be removed until another owner exists.</span></div>' +
      '</div>' +
    '</div>';

  return (
    '<section class="account-content-panel account-content-panel-access">' +
      '<section class="access-management-panel access-section access-section-shell" id="access-section-' +
      escapeAttr(account.id) +
      '" data-actor-role="' +
      escapeAttr(account.role) +
      '" data-can-manage="' +
      escapeAttr(account.can_manage ? 'true' : 'false') +
      '">' +
        '<div class="access-management-header">' +
          '<div>' +
            '<div class="account-panel-kicker">Access</div>' +
            '<h3>' + accessHeaderTitle + '</h3>' +
            '<p>' + accessHeaderCopy + '</p>' +
            accessTaskStrip +
          '</div>' +
        '</div>' +
        '<div class="access-management-stats" id="access-stats-' +
        escapeAttr(account.id) +
        '"></div>' +
        '<div class="access-shell access-shell-idle" id="access-shell-' + escapeAttr(account.id) + '">' +
          '<div class="access-shell-main">' +
            '<div class="access-roster-column">' +
              '<div class="access-roster">' +
                '<div class="access-panel-heading">' +
                  '<h4>People on this account</h4>' +
                  '<p>' + (account.can_manage
                    ? 'Review the hosted roster here, then open the exact access job you need.'
                    : 'Review the hosted roster here. An owner or admin must make changes.') + '</p>' +
                '</div>' +
                '<div class="access-roster-list" id="access-list-' +
                escapeAttr(account.id) +
                '">' +
                  '<div class="access-list-message">Loading…</div>' +
                '</div>' +
              '</div>' +
            '</div>' +
          '</div>' +
          (account.can_manage
            ? (
              '<div class="access-shell-detail" id="access-detail-' + escapeAttr(account.id) + '" hidden>' +
                '<div class="access-task-panel" id="access-task-panel-' + escapeAttr(account.id) + '" hidden>' +
                  '<div class="access-task-header">' +
                    '<div>' +
                      '<div class="account-panel-kicker">Access task</div>' +
                      '<h4 id="access-task-title-' + escapeAttr(account.id) + '">Invite people</h4>' +
                      '<p id="access-task-copy-' + escapeAttr(account.id) + '"></p>' +
                    '</div>' +
                    '<button type="button" class="btn-secondary btn-compact" data-action="clear-access-job" data-account-id="' + escapeAttr(account.id) + '">Close panel</button>' +
                  '</div>' +
                  '<div class="access-task-body" id="access-task-body-invite-' + escapeAttr(account.id) + '" hidden>' +
                    accessInvitePanel +
                    accessRoleGuide +
                  '</div>' +
                  '<div class="access-task-body" id="access-task-body-change_role-' + escapeAttr(account.id) + '" hidden>' +
                    accessChangeRolePanel +
                  '</div>' +
                  '<div class="access-task-body" id="access-task-body-remove-' + escapeAttr(account.id) + '" hidden>' +
                    accessRemovePanel +
                  '</div>' +
                '</div>' +
              '</div>'
            )
            : '') +
        '</div>' +
      '</section>' +
    '</section>'
  );
}

function renderHostedBillingCards(accounts: PortalAccountSummary[], showSelfHostedCommercial: boolean): string {
  var hostedBillingAccounts = accounts.filter(function(account) {
    return account.has_billing;
  });
  if (!hostedBillingAccounts.length) {
    return (
      '<div class="billing-task-card billing-task-card-muted">' +
        '<div class="account-panel-kicker">Hosted billing</div>' +
        '<h3>No hosted billing attached</h3>' +
        '<p>' + escapeHTML(showSelfHostedCommercial
          ? 'Use the self-hosted billing tools below only if you are managing a self-hosted purchase.'
          : 'Hosted invoices, payment methods, and subscription changes are not attached to this Pulse account right now.'
        ) + '</p>' +
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

function renderBillingTaskPanel(title: string, copy: string, panelID: string, bodyHTML: string): string {
  return (
    '<section class="billing-panel" id="' + escapeAttr(panelID) + '" hidden>' +
      '<div class="billing-task-header">' +
        '<div>' +
          '<div class="account-panel-kicker">Billing task</div>' +
          '<h3>' + escapeHTML(title) + '</h3>' +
          '<p>' + escapeHTML(copy) + '</p>' +
        '</div>' +
        '<button type="button" class="btn-secondary btn-compact" data-account-billing-action="clear-billing-panel">Close panel</button>' +
      '</div>' +
      '<div class="billing-task-body">' + bodyHTML + '</div>' +
    '</section>'
  );
}

function renderSupportSection(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var hasHostedAccounts = accounts.length > 0;
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
  var supportEmail = context.bootstrap.support_email || '';
  var canManageHostedTasks = false;
  for (var i = 0; i < accounts.length; i += 1) {
    if (accounts[i].can_manage) {
      canManageHostedTasks = true;
      break;
    }
  }
  var hostedViewOnly = hasHostedAccounts && !canManageHostedTasks;
  var supportLead = hasHostedAccounts
    ? (hostedViewOnly
      ? (showSelfHostedCommercial
        ? 'Use support only when the same Workspaces review, Access review, owner/admin, or Billing path has already stopped you.'
        : 'Use support only when the same Workspaces review, Access review, owner/admin, or hosted Billing path has already stopped you.')
      : (showSelfHostedCommercial
        ? 'Use support only when the Workspaces, Access, or Billing path has already stopped you.'
        : 'Use support only when the Workspaces, Access, or hosted Billing path has already stopped you.'))
    : 'Use support only when the Billing path has already stopped you.';
  var supportChips = hasHostedAccounts
    ? ['Escalation only', hostedViewOnly ? 'Owner/admin first' : (showSelfHostedCommercial ? 'Bring context' : 'Hosted only'), supportEmail ? 'Email' : 'Support']
    : ['Escalation only', 'Billing only', supportEmail ? 'Email' : 'Support'];
  var routeCards = hasHostedAccounts
    ? (
      '<div class="portal-support-route-card">' +
        '<div class="account-panel-kicker">Hosted path</div>' +
        '<h3>' + (hostedViewOnly ? 'Hosted review or owner/admin path failed' : 'Workspace or access path failed') + '</h3>' +
        '<p>' + (hostedViewOnly
          ? 'Go back to the hosted task first. Review the same workspace or roster here, then have an owner or admin run the blocked change before you escalate.'
          : 'Go back to the hosted task first. Escalate only when the same workspace or access path still cannot finish the job.') + '</p>' +
        '<div class="portal-support-points">' +
          '<div class="portal-support-point"><strong>' + (hostedViewOnly ? 'Review the same task' : 'Start from the same task') + '</strong><span>' + (hostedViewOnly
            ? 'Use Workspaces to confirm workspace state and Access to confirm the current roster before you escalate.'
            : 'Use Workspaces for lifecycle issues and Access for roster issues before you escalate.') + '</span></div>' +
          '<div class="portal-support-point"><strong>' + (hostedViewOnly ? 'Name the blocked owner/admin action' : 'Keep the hosted context intact') + '</strong><span>' + (hostedViewOnly
            ? 'Include the account, workspace, and the lifecycle or access change that still needs an owner or admin.'
            : 'Include the account, workspace, and failed action so support inherits the same request.') + '</span></div>' +
        '</div>' +
        '<div class="portal-support-actions">' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">' + (hostedViewOnly ? 'Review workspaces' : 'Open workspaces') + '</button>' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">' + (hostedViewOnly ? 'Review access' : 'Open access') + '</button>' +
          '<a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + '</a>' +
        '</div>' +
      '</div>' +
      '<div class="portal-support-route-card">' +
        '<div class="account-panel-kicker">Billing path</div>' +
        '<h3>' + (hostedViewOnly
          ? (showSelfHostedCommercial ? 'Billing or owner/admin path failed' : 'Hosted billing or owner/admin path failed')
          : (showSelfHostedCommercial ? 'Billing path failed' : 'Hosted billing path failed')) + '</h3>' +
        '<p>' + (hostedViewOnly
          ? (showSelfHostedCommercial
            ? 'Use this route only after the relevant billing job has failed, or the affected hosted account still needs an owner or admin to finish hosted billing.'
            : 'Use this route only after the affected hosted account still needs an owner or admin to finish hosted billing and that path still cannot complete cleanly.')
          : (showSelfHostedCommercial
            ? 'Use this route only after hosted billing or one self-hosted billing job has failed to complete cleanly.'
            : 'Use this route only after hosted billing has failed to complete cleanly.')) + '</p>' +
        '<div class="portal-support-points">' +
          '<div class="portal-support-point"><strong>Name the billing job</strong><span>' + (hostedViewOnly
            ? (showSelfHostedCommercial
              ? 'Say whether the failed path was hosted billing, licenses, refunds, or privacy, and whether hosted billing still needed an owner or admin.'
              : 'Say whether the failed path was hosted billing and whether the account still needed an owner or admin to open it.')
            : (showSelfHostedCommercial
              ? 'Say whether the failed path was hosted billing, licenses, refunds, or privacy.'
              : 'Say whether the failed path was hosted billing.')) + '</span></div>' +
          '<div class="portal-support-point"><strong>Keep the request intact</strong><span>' + (hostedViewOnly
            ? (showSelfHostedCommercial
              ? 'Bring the same account or billing email and the failed owner/admin or billing step instead of reopening the story.'
              : 'Bring the same hosted account and the failed billing or owner/admin step instead of reopening the story.')
            : (showSelfHostedCommercial
              ? 'Bring the same account or billing email and the failed action instead of reopening the story.'
              : 'Bring the same hosted account and the failed billing action instead of reopening the story.')) + '</span></div>' +
        '</div>' +
        '<div class="portal-support-actions">' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>' +
          '<a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + '</a>' +
        '</div>' +
      '</div>'
    )
    : (
      '<div class="portal-support-route-card">' +
        '<div class="account-panel-kicker">Billing path</div>' +
        '<h3>Self-hosted billing path failed</h3>' +
        '<p>Use this route only after a self-hosted billing, license, refund, or privacy job has failed to complete cleanly.</p>' +
        '<div class="portal-support-points">' +
          '<div class="portal-support-point"><strong>Name the billing job</strong><span>Say whether the failed path was billing, licenses, refunds, or privacy.</span></div>' +
          '<div class="portal-support-point"><strong>Keep the purchase context intact</strong><span>Bring the same commercial email and the failed action instead of reopening the story.</span></div>' +
        '</div>' +
        '<div class="portal-support-actions">' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>' +
          '<a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + '</a>' +
        '</div>' +
      '</div>'
    );
  var runbookSteps = hasHostedAccounts
    ? (
      '<div class="portal-support-runbook-step"><strong>1. Failed path</strong><span>' + (hostedViewOnly
        ? (showSelfHostedCommercial
          ? 'Say whether the blocked path was Workspaces review, Access review, owner/admin hosted change, hosted billing, licenses, refunds, or privacy.'
          : 'Say whether the blocked path was Workspaces review, Access review, owner/admin hosted change, or hosted billing.')
        : (showSelfHostedCommercial
          ? 'Say whether the blocked path was Workspaces, Access, hosted billing, licenses, refunds, or privacy.'
          : 'Say whether the blocked path was Workspaces, Access, or hosted billing.')) + '</span></div>' +
      '<div class="portal-support-runbook-step"><strong>2. Account or email</strong><span>' + (hostedViewOnly
        ? (showSelfHostedCommercial
          ? 'Include the hosted account and workspace for the blocked review or owner/admin path, or the commercial billing email for self-hosted work.'
          : 'Include the hosted account and workspace or hosted billing account that still needed owner/admin action.')
        : (showSelfHostedCommercial
          ? 'Include the hosted account and workspace when relevant, or the commercial billing email for self-hosted work.'
          : 'Include the hosted account and workspace or hosted billing account that the failed path belongs to.')) + '</span></div>' +
      '<div class="portal-support-runbook-step"><strong>3. Failed action</strong><span>Name the exact button, form, or billing step that failed and what happened next.</span></div>'
    )
    : (
      '<div class="portal-support-runbook-step"><strong>1. Billing job</strong><span>Say whether the blocked path was billing, licenses, refunds, or privacy.</span></div>' +
      '<div class="portal-support-runbook-step"><strong>2. Purchase email</strong><span>Include the commercial billing email used for the self-hosted purchase.</span></div>' +
      '<div class="portal-support-runbook-step"><strong>3. Failed action</strong><span>Name the exact button, form, or billing step that failed and what happened next.</span></div>'
    );
  return (
    '<section class="portal-support-panel">' +
      '<div class="account-panel-kicker">Support</div>' +
      '<h2>Escalation only</h2>' +
      '<p>' + escapeHTML(supportLead) + '</p>' +
      renderSectionContextChips(supportChips) +
      '<div class="portal-support-layout">' +
        '<div class="portal-support-route-grid">' +
          routeCards +
        '</div>' +
        '<div class="portal-support-runbook">' +
          '<div class="account-panel-kicker">What to send</div>' +
          '<h3>Keep the escalation short</h3>' +
          '<p>Support should inherit the same request, not reconstruct it from scratch.</p>' +
          '<div class="portal-support-runbook-list">' +
            runbookSteps +
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
  return renderShellOverviewSection(context);
}

export function renderAuthenticatedPortalHTML(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var hosted = hasHostedAccounts(accounts);
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
  var activeSection = context.activeSection || 'overview';
  var hostedBillingCount = accounts.filter(function(account) {
    return account.has_billing;
  }).length;
  var billingNote = hosted
    ? (showSelfHostedCommercial
      ? 'Use hosted billing first when the request belongs to a hosted workspace account. Self-hosted licenses, refunds, and privacy stay separate underneath it.'
      : 'Use this billing surface only for hosted billing on your hosted workspace accounts.')
    : 'Use this billing surface only for self-hosted subscriptions, licenses, refunds, and privacy requests.';
  var selfHostedBillingEscalationCopy = hosted
    ? 'Escalate with the same hosted billing action or self-hosted path and the exact failed step.'
    : 'Escalate with the same self-hosted billing path and the exact failed step.';
  var hostedContent = accounts.length
    ? accounts.map(function(account) {
      return (
        '<section class="account-surface">' +
          '<div class="account-surface-body">' +
            renderAccountWorkspaceSection(account, context.accountAPIBasePath) +
            renderAccountAccessSection(account) +
          '</div>' +
        '</section>'
      );
    }).join('')
    : renderNoHostedWorkspacesSection() + renderNoHostedAccessSection();

  return (
    '<div class="portal-shell" data-shell-section="' + activeSection + '">' +
      '<div class="portal-shell-layout">' +
        renderShellNavigation(accounts, context.bootstrap.support_email || '', activeSection) +
        '<div class="portal-shell-main">' +
          (accounts.length === 1 ? renderAccountContextStrip(accounts[0]) : '') +
          '<section class="portal-content-panel portal-content-panel-overview">' +
            renderShellOverviewSection(context) +
            '<div id="accounts-root">' + hostedContent + '</div>' +
          '</section>' +
          '<section class="portal-content-panel portal-content-panel-billing billing-section" id="billing-section">' +
            '<div class="billing-header">' +
              '<div>' +
                '<div class="account-panel-kicker">Billing</div>' +
                '<h2>Billing</h2>' +
                renderSectionContextChips([
                  hostedBillingCount > 0 ? 'Hosted billing' : 'No hosted billing',
                  showSelfHostedCommercial ? 'Self-hosted tools' : 'Hosted only',
                ]) +
              '</div>' +
              '<div class="billing-note">' + billingNote + '</div>' +
            '</div>' +
            (hosted
              ? ('<div class="billing-overview-grid">' + renderHostedBillingCards(accounts, showSelfHostedCommercial) + '</div>')
              : '') +
            (showSelfHostedCommercial
              ? ('<div class="billing-shell billing-shell-idle">' +
              '<div class="billing-shell-main">' +
                '<div class="billing-shell-main-head">' +
                  '<div class="account-panel-kicker">Self-hosted billing</div>' +
                  '<h3>Pick the self-hosted job</h3>' +
                  '<p>Use self-hosted billing only for self-hosted purchases. Open one path at a time when hosted billing does not apply.</p>' +
                '</div>' +
                '<div class="billing-action-list">' +
                  renderBillingActionRow('open-manage-billing', 'Self-hosted billing', 'Manage subscriptions', 'Billing', 'Open Stripe for self-hosted plan, invoice, and payment changes.', 'manage-billing-panel', 'manage-inline-email', ['Plan changes', 'Invoices']) +
                  renderBillingActionRow('open-retrieve-billing', 'Licenses', 'Retrieve licenses', 'Licenses', 'Recover the latest active self-hosted license and invoice link.', 'retrieve-billing-panel', 'retrieve-inline-email', ['Latest active license', 'Invoice lookup']) +
                  renderBillingActionRow('open-refund-billing', 'Refunds', 'Refund requests', 'Refunds', 'Request a self-serve refund when the purchase is still eligible.', 'refund-billing-panel', 'refund-inline-email', ['Eligibility check', 'Revocation']) +
                  renderBillingActionRow('open-data-billing', 'Privacy', 'Data and privacy', 'Privacy', 'Request export or deletion for commercial account data.', 'data-billing-panel', 'data-export-email', ['Export', 'Deletion']) +
                '</div>' +
                '<div class="billing-inline-support">' +
                  '<div class="account-panel-kicker">Escalation only</div>' +
                  '<h4>Use Support only after Billing fails</h4>' +
                  '<p>' + selfHostedBillingEscalationCopy + '</p>' +
                  '<div class="billing-inline-support-actions">' +
                    '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="support">Open support</button>' +
                    '<a class="portal-support-link" href="mailto:' +
                    escapeAttr(context.bootstrap.support_email || '') +
                    '">' +
                    escapeHTML(context.bootstrap.support_email || '') +
                    '</a>' +
                  '</div>' +
                '</div>' +
              '</div>' +
              '<div class="billing-shell-detail" id="billing-detail-shell" hidden>' +
                renderBillingTaskPanel(
                  'Manage subscriptions',
                  'Open Stripe for self-hosted plan, invoice, and payment changes.',
                  'manage-billing-panel',
                  '<div id="manage-billing-root"></div>'
                ) +
                renderBillingTaskPanel(
                  'Retrieve licenses',
                  'Recover the latest active self-hosted license and invoice link.',
                  'retrieve-billing-panel',
                  '<div id="retrieve-billing-root"></div>'
                ) +
                renderBillingTaskPanel(
                  'Refund requests',
                  'Request a self-serve refund when the purchase is still eligible.',
                  'refund-billing-panel',
                  '<div id="refund-billing-root"></div>'
                ) +
                renderBillingTaskPanel(
                  'Data and privacy',
                  'Request export or deletion for commercial account data.',
                  'data-billing-panel',
                  '<div class="subsection"><div id="data-export-root"></div></div>' +
                  '<div class="subsection"><div id="data-delete-root"></div></div>' +
                  '<div class="helper-text">Payment-card data stays with Stripe. For Stripe deletion support, contact <a href="mailto:' +
                  escapeAttr(context.bootstrap.support_email || '') +
                  '">' +
                  escapeHTML(context.bootstrap.support_email || '') +
                  '</a>.</div>'
                ) +
              '</div>' +
            '</div>')
              : '') +
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
