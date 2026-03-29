import type {
  PortalAccountSummary,
  PortalBootstrapData,
  PortalLoginState,
  PortalShellSection,
  PortalWorkspaceSummary,
} from './types';
import { portalRoleLabel } from './account_roles';
import { preferredPortalShellSection } from './shell_section';
import { workspaceHealthLabel, workspaceHealthState, workspaceRowNote, workspaceStatusCopy } from './workspace_presentation';

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

function accountKindLabel(account: PortalAccountSummary): string {
  if (account.kind === 'msp') return 'MSP account';
  if (account.kind === 'cloud') return 'Cloud account';
  if (account.kind === 'individual') return 'Hosted account';
  return account.kind_label ? account.kind_label + ' account' : 'Account';
}

function workspaceCountLabel(count: number): string {
  return count === 1 ? '1 workspace' : String(count) + ' workspaces';
}

function workspaceTotalChipLabel(count: number): string {
  return count === 1 ? '1 workspace total' : String(count) + ' workspaces total';
}

function reviewWorkspaceHeadline(count: number): string {
  return count === 1 ? '1 workspace needs review' : String(count) + ' workspaces need review';
}

function readyWorkspaceHeadline(count: number): string {
  return count === 1 ? '1 workspace is ready to use' : String(count) + ' workspaces are ready to use';
}

function readyWorkspaceSectionChipLabel(count: number): string {
  return count === 1 ? '1 workspace ready to use' : String(count) + ' workspaces ready to use';
}

function reviewWorkspaceChipLabel(count: number): string {
  return count === 1 ? '1 workspace to review' : String(count) + ' workspaces to review';
}

function readyWorkspaceChipLabel(count: number): string {
  return count === 1 ? '1 ready workspace' : String(count) + ' ready workspaces';
}

function suspendedWorkspaceChipLabel(count: number): string {
  return count === 1 ? '1 suspended workspace' : String(count) + ' suspended workspaces';
}

function suspendedWorkspaceSectionChipLabel(count: number): string {
  return count === 1 ? '1 workspace suspended' : String(count) + ' workspaces suspended';
}

function billingHeaderChipLabels(hostedBillingCount: number, showSelfHostedCommercial: boolean): string[] {
  return [
    hostedBillingCount > 0 ? 'Hosted billing attached' : 'No hosted billing attached',
    showSelfHostedCommercial ? 'Self-hosted billing available' : 'Hosted billing only',
  ];
}

function supportSectionChipLabels(hasHostedAccounts: boolean, hostedViewOnly: boolean, supportEmail: string): string[] {
  if (hasHostedAccounts) {
    return [
      'Escalation only',
      hostedViewOnly ? 'Review Workspaces or Access, or contact owner/admin first' : 'Open Workspaces, Access, or Billing first',
      supportEmail ? 'Email support' : 'Support route',
    ];
  }
  return ['Escalation only', 'Open Billing first', supportEmail ? 'Email support' : 'Support route'];
}

function supportLeadCopy(hasHostedAccounts: boolean, hostedViewOnly: boolean, showSelfHostedCommercial: boolean): string {
  if (!hasHostedAccounts) return 'Use Support only after Billing fails.';
  if (hostedViewOnly) {
    return showSelfHostedCommercial
      ? 'Use Support only after Workspaces review, Access review, owner/admin handoff, or Billing fails.'
      : 'Use Support only after Workspaces review, Access review, owner/admin handoff, or hosted Billing fails.';
  }
  return showSelfHostedCommercial
    ? 'Use Support only after Workspaces, Access, or Billing fails.'
    : 'Use Support only after Workspaces, Access, or hosted Billing fails.';
}

function hostedSupportRouteTitle(hostedViewOnly: boolean): string {
  return hostedViewOnly ? 'Review or owner/admin handoff failed' : 'Workspace or access failed';
}

function hostedSupportRouteDescription(hostedViewOnly: boolean): string {
  return hostedViewOnly
    ? 'Recheck the same workspace or roster first. Escalate only if the required owner or admin change still cannot complete.'
    : 'Retry the same workspace or access task first. Escalate only if the same task still fails.';
}

function hostedSupportRouteTaskCopy(hostedViewOnly: boolean): string {
  return hostedViewOnly
    ? 'Use Workspaces for workspace state or Access for roster state before you escalate.'
    : 'Use Workspaces for lifecycle work or Access for roster work before you escalate.';
}

function hostedSupportRouteContextLabel(hostedViewOnly: boolean): string {
  return hostedViewOnly ? 'Owner/admin handoff' : 'Failed step';
}

function hostedSupportRouteContextCopy(hostedViewOnly: boolean): string {
  return hostedViewOnly
    ? 'Include the account, workspace, and the change that still needs owner or admin action.'
    : 'Include the account, workspace, and the failed step.';
}

function billingSupportRouteTitle(hostedViewOnly: boolean, showSelfHostedCommercial: boolean): string {
  if (hostedViewOnly) {
    return showSelfHostedCommercial ? 'Billing or owner/admin handoff failed' : 'Hosted billing or owner/admin handoff failed';
  }
  return showSelfHostedCommercial ? 'Billing failed' : 'Hosted billing failed';
}

function billingSupportRouteDescription(hostedViewOnly: boolean, showSelfHostedCommercial: boolean): string {
  if (hostedViewOnly) {
    return showSelfHostedCommercial
      ? 'Escalate only if hosted billing still needs owner/admin action or a self-hosted billing task still fails.'
      : 'Escalate only if hosted billing still needs owner/admin action or still fails.';
  }
  return showSelfHostedCommercial
    ? 'Escalate only if hosted billing or a self-hosted billing task still fails.'
    : 'Escalate only if hosted billing still fails.';
}

function billingSupportRouteJobCopy(hostedViewOnly: boolean, showSelfHostedCommercial: boolean): string {
  if (hostedViewOnly) {
    return showSelfHostedCommercial
      ? 'Hosted billing, licenses, refunds, or privacy. Say if hosted billing still needed owner/admin action.'
      : 'Hosted billing. Say if it still needed owner/admin action.';
  }
  return showSelfHostedCommercial
    ? 'Hosted billing, licenses, refunds, or privacy.'
    : 'Hosted billing.';
}

function billingSupportRouteAccountCopy(hostedViewOnly: boolean, showSelfHostedCommercial: boolean): string {
  if (showSelfHostedCommercial) {
    return 'Include the hosted account for hosted billing or the billing email for self-hosted work.';
  }
  return hostedViewOnly
    ? 'Include the hosted account that still needed owner/admin billing action.'
    : 'Include the hosted account for the failed billing step.';
}

function supportRunbookPathCopy(hasHostedAccounts: boolean, hostedViewOnly: boolean, showSelfHostedCommercial: boolean): string {
  if (!hasHostedAccounts) return 'Billing, licenses, refunds, or privacy.';
  if (hostedViewOnly) {
    return showSelfHostedCommercial
      ? 'Workspaces review, Access review, owner/admin handoff, hosted billing, licenses, refunds, or privacy.'
      : 'Workspaces review, Access review, owner/admin handoff, or hosted billing.';
  }
  return showSelfHostedCommercial
    ? 'Workspaces, Access, hosted billing, licenses, refunds, or privacy.'
    : 'Workspaces, Access, or hosted billing.';
}

function supportRunbookAccountCopy(hasHostedAccounts: boolean, showSelfHostedCommercial: boolean): string {
  if (!hasHostedAccounts) return 'Commercial billing email used for the self-hosted purchase.';
  return showSelfHostedCommercial
    ? 'Hosted account and workspace when relevant, or billing email for self-hosted work.'
    : 'Hosted account and workspace, or hosted billing account.';
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
    return '<span class="badge badge-healthy">' + escapeHTML(workspaceHealthLabel(workspace)) + '</span>';
  }
  if (status === 'unhealthy') {
    return '<span class="badge badge-unhealthy">' + escapeHTML(workspaceHealthLabel(workspace)) + '</span>';
  }
  return '<span class="badge badge-checking">' + escapeHTML(workspaceHealthLabel(workspace)) + '</span>';
}

function renderBillingActionRow(
  id: string,
  title: string,
  actionLabel: string,
  description: string,
  panelID: string,
  focusID: string,
  highlights: string[]
): string {
  var meta = escapeHTML(highlights.join(' • '));
  return (
    '<article class="billing-action-row">' +
      '<div class="billing-action-main">' +
        '<div class="billing-action-copy">' +
          '<h3>' + title + '</h3>' +
          '<p>' + description + '</p>' +
        '</div>' +
        '<div class="billing-action-meta">' + meta + '</div>' +
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

function accountContextRoleMeta(account: PortalAccountSummary): string {
  return portalRoleLabel(account.role) + ' role';
}

function accountContextLeadCopy(account: PortalAccountSummary): string {
  if (account.can_manage) {
    return account.has_billing
      ? 'Manage workspaces, access, and billing for this account.'
      : 'Manage workspaces and access for this account.';
  }
  return account.has_billing
    ? 'Open workspaces and review access here. Billing and account changes require an owner or admin.'
    : 'Open workspaces and review access here. Account changes require an owner or admin.';
}

function renderAccountContextStrip(account: PortalAccountSummary): string {
  return (
    '<section class="portal-account-context">' +
      '<div class="portal-account-context-copy">' +
        '<div class="portal-account-context-meta">' +
          '<span class="account-eyebrow">' + escapeHTML(accountKindLabel(account)) + '</span>' +
          '<span class="portal-account-context-separator">•</span>' +
          '<span class="portal-account-context-access">' + escapeHTML(accountContextRoleMeta(account)) + '</span>' +
        '</div>' +
        '<div class="portal-account-context-row portal-account-context-row-title">' +
          '<h2>' + escapeHTML(account.name) + '</h2>' +
        '</div>' +
        '<p>' + escapeHTML(accountContextLeadCopy(account)) + '</p>' +
      '</div>' +
    '</section>'
  );
}

function renderPortalContextStrip(accounts: PortalAccountSummary[], showSelfHostedCommercial: boolean): string {
  if (accounts.length === 1) {
    return renderAccountContextStrip(accounts[0]);
  }
  if (accounts.length > 1) {
    return (
      '<section class="portal-account-context">' +
        '<div class="portal-account-context-copy">' +
          '<div class="portal-account-context-meta">' +
            '<span class="account-eyebrow">Pulse Account</span>' +
            '<span class="portal-account-context-separator">•</span>' +
            '<span class="portal-account-context-access">' + escapeHTML(accounts.length === 1 ? '1 hosted account' : String(accounts.length) + ' hosted accounts') + '</span>' +
          '</div>' +
          '<div class="portal-account-context-row portal-account-context-row-title">' +
            '<h2>Accounts</h2>' +
          '</div>' +
          '<p>Open workspaces, review access, and handle billing from one account surface.</p>' +
        '</div>' +
      '</section>'
    );
  }
  return (
    '<section class="portal-account-context">' +
      '<div class="portal-account-context-copy">' +
        '<div class="portal-account-context-meta">' +
          '<span class="account-eyebrow">Billing account</span>' +
          '<span class="portal-account-context-separator">•</span>' +
          '<span class="portal-account-context-access">' + escapeHTML(showSelfHostedCommercial ? 'Self-hosted commercial' : 'Commercial account') + '</span>' +
        '</div>' +
        '<div class="portal-account-context-row portal-account-context-row-title">' +
          '<h2>Billing</h2>' +
        '</div>' +
        '<p>Use Billing for self-hosted subscriptions, licenses, refunds, and privacy requests.</p>' +
      '</div>' +
    '</section>'
  );
}

interface ShellNavEntry {
  section: PortalShellSection;
  title: string;
}

function visibleShellSections(bootstrap: PortalBootstrapData): ShellNavEntry[] {
  var accounts = Array.isArray(bootstrap.accounts) ? bootstrap.accounts : [];
  var hosted = hasHostedAccounts(accounts);
  var showSelfHostedCommercial = hasSelfHostedCommercial(bootstrap);
  var hostedBillingCount = 0;
  for (var i = 0; i < accounts.length; i += 1) {
    if (accounts[i].has_billing) {
      hostedBillingCount += 1;
    }
  }

  var sections: ShellNavEntry[] = [];
  if (hosted) {
    sections.push({ section: 'workspaces', title: 'Workspaces' });
    sections.push({ section: 'access', title: 'Access' });
  }
  if (hostedBillingCount > 0 || showSelfHostedCommercial) {
    sections.push({ section: 'billing', title: 'Billing' });
  }
  sections.push({ section: 'support', title: 'Support' });
  if (hosted) {
    sections.push({ section: 'overview', title: 'Overview' });
  }
  return sections;
}

function shellSectionButton(section: PortalShellSection, activeSection: PortalShellSection, title: string): string {
  return (
    '<button class="portal-shell-nav-link' + (activeSection === section ? ' active' : '') + '" type="button" data-shell-action="activate-section" data-shell-section="' + section + '">' +
      '<span class="portal-shell-nav-row">' +
        '<span class="portal-shell-nav-label">' + title + '</span>' +
      '</span>' +
    '</button>'
  );
}

function renderShellNavigation(bootstrap: PortalBootstrapData, activeSection: PortalShellSection): string {
  var sections = visibleShellSections(bootstrap);
  return (
    '<nav class="portal-shell-nav" aria-label="Pulse Account sections">' +
      '<div class="portal-shell-nav-group">' +
        sections.map(function(entry) {
          return shellSectionButton(entry.section, activeSection, entry.title);
        }).join('') +
      '</div>' +
    '</nav>'
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
  var ready = readyOverviewEntries(entries);
  var includeAccountName = accounts.length > 1;
  var suspendedCount = countWorkspacesByState(entries.map(function(entry) {
    return entry.workspace;
  }), 'suspended');
  if (!attention.length) {
    return (
      '<article class="overview-task-card">' +
        '<div class="account-panel-kicker">Needs attention</div>' +
        '<h4>' + escapeHTML(accounts.length > 0 ? reviewWorkspaceHeadline(0) : '0 hosted workspaces need review') + '</h4>' +
        '<p>' + escapeHTML(entries.length > 0
          ? 'No active workspace is failed or waiting on a completed health check.'
          : accounts.length > 0
            ? 'No hosted workspace is attached to this account yet.'
            : 'No hosted account is attached to this sign-in.'
        ) + '</p>' +
        '<div class="overview-task-list">' +
          '<div class="overview-task-item"><strong>Ready</strong><span>' + escapeHTML(entries.length > 0
            ? readyWorkspaceHeadline(ready.length)
            : accounts.length > 0
              ? readyWorkspaceHeadline(0)
              : '0 hosted workspaces are ready to use'
          ) + '</span></div>' +
          '<div class="overview-task-item"><strong>Suspended</strong><span>' + escapeHTML(suspendedCount > 0
            ? suspendedCount === 1
              ? '1 workspace is suspended. Resume it before opening it again.'
              : String(suspendedCount) + ' workspaces are suspended. Resume them before opening them again.'
            : '0 suspended workspaces.'
          ) + '</span></div>' +
        '</div>' +
      '</article>'
    );
  }

  return (
    '<article class="overview-task-card overview-task-card-attention">' +
      '<div class="account-panel-kicker">Needs attention</div>' +
      '<h4>' + escapeHTML(reviewWorkspaceHeadline(attention.length)) + '</h4>' +
      '<p>Each listed workspace is failed or still waiting on a completed health check.</p>' +
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
  var totalWorkspaces = countWorkspaces(accounts);
  var canManageHosted = accounts.some(function(account) {
    return account.can_manage;
  });
  var suspendedCount = countWorkspacesByState(entries.map(function(entry) {
    return entry.workspace;
  }), 'suspended');
  if (!ready.length) {
    return (
      '<article class="overview-task-card">' +
        '<div class="account-panel-kicker">Ready</div>' +
        '<h4>' + escapeHTML(!accounts.length
          ? 'Billing is available'
          : readyWorkspaceHeadline(0)
        ) + '</h4>' +
        '<p>' + escapeHTML(!accounts.length
          ? 'Use Billing for self-hosted subscriptions, licenses, refunds, and privacy requests.'
          : totalWorkspaces > 0
            ? suspendedCount === totalWorkspaces
              ? 'Every hosted workspace is suspended right now.'
              : 'Open Workspaces to see the current state of each hosted workspace.'
            : canManageHosted
              ? 'No hosted workspace exists yet. Create the first one in Workspaces.'
              : 'No hosted workspace exists yet. An owner or admin must create the first one.'
        ) + '</p>' +
      '</article>'
    );
  }

  return (
    '<article class="overview-task-card">' +
      '<div class="account-panel-kicker">Ready</div>' +
      '<h4>' + escapeHTML(readyWorkspaceHeadline(ready.length)) + '</h4>' +
      '<p>Each listed workspace is active and passed its latest health check.</p>' +
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

function renderOverviewNextActionCard(
  accounts: PortalAccountSummary[],
  entries: OverviewWorkspaceEntry[],
  accountAPIBasePath: string,
  showSelfHostedCommercial: boolean
): string {
  var attention = attentionOverviewEntries(entries);
  var ready = readyOverviewEntries(entries);
  var primaryAction = '';
  var secondaryAction = '';
  var title = '';
  var description = '';
  var totalWorkspaces = countWorkspaces(accounts);
  var creatableAccount = accounts.find(function(account) {
    return account.kind === 'msp' && account.can_manage;
  }) || null;
  var billingAccount = accounts.find(function(account) {
    return account.has_billing && account.can_manage;
  }) || null;
  var accessAccount = accounts.find(function(account) {
    return account.can_manage;
  }) || null;
  var hostedViewOnly = accounts.length > 0 && !accessAccount;

  if (attention.length) {
    title = 'Open Workspaces';
    description = attention.length > 1
      ? 'Open Workspaces to review each failed or pending workspace.'
      : 'Open Workspaces to review ' + attention[0].workspace.display_name + '.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">Open Workspaces</button>';
    secondaryAction = accessAccount
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>'
      : '';
  } else if (ready.length) {
    title = 'Open workspace';
    description = accounts.length > 1
      ? 'Open ' + ready[0].workspace.display_name + ' in ' + ready[0].account.name + '.'
      : 'Open the ready workspace.';
    primaryAction = renderWorkspaceHandoffForm(ready[0].account.id, ready[0].workspace.id, accountAPIBasePath, 'Open ' + ready[0].workspace.display_name, 'btn-primary btn-compact');
    secondaryAction = ready.length > 1
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">See all workspaces</button>'
      : '';
  } else if (creatableAccount) {
    title = 'Create workspace';
    description = 'No workspace is ready. Create a workspace in ' + creatableAccount.name + '.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(creatableAccount.id) + '">Create workspace</button>';
    secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
  } else if (billingAccount) {
    title = 'Open billing';
    description = 'Use Billing for invoices, payment methods, or subscription changes.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
    secondaryAction = accessAccount
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>'
      : '';
  } else if (!accounts.length) {
    title = 'Open billing';
    description = 'Use Billing for self-hosted subscriptions, licenses, refunds, or privacy requests.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
  } else if (hostedViewOnly) {
    if (totalWorkspaces > 0) {
      title = 'Review workspace state';
      description = 'No workspace is ready. Open Workspaces to review current state, then hand off changes to an owner or admin.';
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="workspaces">Open Workspaces</button>';
      secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
    } else {
      title = 'Review who can act';
      description = showSelfHostedCommercial
        ? 'No hosted workspace is attached. Review Access to see who can manage this hosted account, or use Billing for self-hosted tasks.'
        : 'No hosted workspace is attached yet. Review Access to see who can create or manage the first workspace on this account.';
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
      secondaryAction = showSelfHostedCommercial
        ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>'
        : '';
    }
  } else if (accessAccount) {
    title = 'Open access';
    description = 'Use Access for invites, role changes, or access removal.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open access</button>';
    secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
  } else {
    title = 'Open billing or support';
    description = 'Use Billing for commercial work. Use Support only after the billing path fails.';
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
      readyWorkspaceChipLabel(readyCount),
      reviewWorkspaceChipLabel(attentionCount),
      suspendedWorkspaceChipLabel(suspendedCount),
    ]
    : ['No hosted account', '0 hosted workspaces', 'Billing available', 'Support only on escalation'];

  return (
    '<section class="account-content-panel account-content-panel-overview">' +
      '<div class="account-stage-header account-stage-header-overview overview-stage-header">' +
        '<div>' +
          '<div class="account-panel-kicker">Overview</div>' +
          '<h3>Current state</h3>' +
          '<p>Hosted workspace counts, current state, and the next step.</p>' +
          renderSectionContextChips(chips) +
        '</div>' +
      '</div>' +
      '<div class="overview-task-grid">' +
        renderOverviewAttentionCard(accounts, entries, showSelfHostedCommercial) +
        renderOverviewReadyCard(accounts, entries, context.accountAPIBasePath) +
        renderOverviewNextActionCard(accounts, entries, context.accountAPIBasePath, showSelfHostedCommercial) +
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
    : 'Open a workspace here. An owner or admin must handle lifecycle or creation changes.';
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
              workspaceTotalChipLabel(workspaces.length),
              readyWorkspaceSectionChipLabel(readyCount),
              suspendedWorkspaceSectionChipLabel(suspendedCount),
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
        '<div class="billing-task-meta">Keep workspace lifecycle work in Workspaces and access changes in Access.</div>' +
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
  var supportLead = supportLeadCopy(hasHostedAccounts, hostedViewOnly, showSelfHostedCommercial);
  var supportChips = supportSectionChipLabels(hasHostedAccounts, hostedViewOnly, supportEmail);
  var routeCards = hasHostedAccounts
    ? (
      '<div class="portal-support-route-card">' +
        '<div class="account-panel-kicker">Workspaces or Access</div>' +
        '<h3>' + hostedSupportRouteTitle(hostedViewOnly) + '</h3>' +
        '<p>' + hostedSupportRouteDescription(hostedViewOnly) + '</p>' +
        '<div class="portal-support-points">' +
          '<div class="portal-support-point"><strong>Task</strong><span>' + hostedSupportRouteTaskCopy(hostedViewOnly) + '</span></div>' +
          '<div class="portal-support-point"><strong>' + hostedSupportRouteContextLabel(hostedViewOnly) + '</strong><span>' + hostedSupportRouteContextCopy(hostedViewOnly) + '</span></div>' +
        '</div>' +
        '<div class="portal-support-actions">' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">Open Workspaces</button>' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">Open Access</button>' +
          '<a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + '</a>' +
        '</div>' +
      '</div>' +
      '<div class="portal-support-route-card">' +
        '<div class="account-panel-kicker">' + (showSelfHostedCommercial ? 'Billing' : 'Hosted billing') + '</div>' +
        '<h3>' + billingSupportRouteTitle(hostedViewOnly, showSelfHostedCommercial) + '</h3>' +
        '<p>' + billingSupportRouteDescription(hostedViewOnly, showSelfHostedCommercial) + '</p>' +
        '<div class="portal-support-points">' +
          '<div class="portal-support-point"><strong>Billing job</strong><span>' + billingSupportRouteJobCopy(hostedViewOnly, showSelfHostedCommercial) + '</span></div>' +
          '<div class="portal-support-point"><strong>Account or email</strong><span>' + billingSupportRouteAccountCopy(hostedViewOnly, showSelfHostedCommercial) + '</span></div>' +
        '</div>' +
        '<div class="portal-support-actions">' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>' +
          '<a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + '</a>' +
        '</div>' +
      '</div>'
    )
    : (
      '<div class="portal-support-route-card">' +
        '<div class="account-panel-kicker">Self-hosted billing</div>' +
        '<h3>Self-hosted billing failed</h3>' +
        '<p>Escalate only if a self-hosted billing task still fails.</p>' +
        '<div class="portal-support-points">' +
          '<div class="portal-support-point"><strong>Billing job</strong><span>Billing, licenses, refunds, or privacy.</span></div>' +
          '<div class="portal-support-point"><strong>Account or email</strong><span>Include the commercial billing email and the failed step.</span></div>' +
        '</div>' +
        '<div class="portal-support-actions">' +
          '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>' +
          '<a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + '</a>' +
        '</div>' +
      '</div>'
    );
  var runbookSteps = hasHostedAccounts
    ? (
      '<div class="portal-support-runbook-step"><strong>1. Failed path</strong><span>' + supportRunbookPathCopy(hasHostedAccounts, hostedViewOnly, showSelfHostedCommercial) + '</span></div>' +
      '<div class="portal-support-runbook-step"><strong>2. Account or email</strong><span>' + supportRunbookAccountCopy(hasHostedAccounts, showSelfHostedCommercial) + '</span></div>' +
      '<div class="portal-support-runbook-step"><strong>3. Failed action</strong><span>Name the exact button, form, or billing step that failed and what happened next.</span></div>'
    )
    : (
      '<div class="portal-support-runbook-step"><strong>1. Billing job</strong><span>' + supportRunbookPathCopy(hasHostedAccounts, hostedViewOnly, showSelfHostedCommercial) + '</span></div>' +
      '<div class="portal-support-runbook-step"><strong>2. Account or email</strong><span>' + supportRunbookAccountCopy(hasHostedAccounts, showSelfHostedCommercial) + '</span></div>' +
      '<div class="portal-support-runbook-step"><strong>3. Failed action</strong><span>Name the exact button, form, or billing step that failed and what happened next.</span></div>'
    );
  return (
    '<section class="portal-support-panel">' +
      '<div class="account-panel-kicker">Support</div>' +
      '<h2>Support</h2>' +
      '<p>' + escapeHTML(supportLead) + '</p>' +
      renderSectionContextChips(supportChips) +
      '<div class="portal-support-layout">' +
        '<div class="portal-support-route-grid">' +
          routeCards +
        '</div>' +
        '<div class="portal-support-runbook">' +
          '<div class="account-panel-kicker">What to send</div>' +
          '<h3>Send these details</h3>' +
          '<p>Send the failed path, account or email, and failed action.</p>' +
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
  var shellSections = visibleShellSections(context.bootstrap);
  var preferredSection = context.activeSection || preferredPortalShellSection(context.bootstrap);
  var activeSection = shellSections.some(function(entry) {
    return entry.section === preferredSection;
  })
    ? preferredSection
    : (shellSections[0] ? shellSections[0].section : 'billing');
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
  var workspacesContent = accounts.length
    ? accounts.map(function(account) {
      return (
        '<section class="account-surface">' +
          renderAccountWorkspaceSection(account, context.accountAPIBasePath) +
        '</section>'
      );
    }).join('')
    : renderNoHostedWorkspacesSection();
  var accessContent = accounts.length
    ? accounts.map(function(account) {
      return (
        '<section class="account-surface">' +
          renderAccountAccessSection(account) +
        '</section>'
      );
    }).join('')
    : renderNoHostedAccessSection();

  return (
    '<div class="portal-shell" data-shell-section="' + activeSection + '">' +
      '<div class="portal-shell-main">' +
        renderPortalContextStrip(accounts, showSelfHostedCommercial) +
        renderShellNavigation(context.bootstrap, activeSection) +
        (hosted
          ? (
            '<section class="portal-content-panel portal-content-panel-workspaces">' +
              workspacesContent +
            '</section>' +
            '<section class="portal-content-panel portal-content-panel-access">' +
              accessContent +
            '</section>' +
            '<section class="portal-content-panel portal-content-panel-overview">' +
              renderShellOverviewSection(context) +
            '</section>'
          )
          : '') +
        '<section class="portal-content-panel portal-content-panel-billing billing-section" id="billing-section">' +
          '<div class="billing-header">' +
            '<div>' +
              '<div class="account-panel-kicker">Billing</div>' +
              '<h2>Billing</h2>' +
              renderSectionContextChips(billingHeaderChipLabels(hostedBillingCount, showSelfHostedCommercial)) +
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
                renderBillingActionRow('open-manage-billing', 'Manage subscriptions', 'Billing', 'Open Stripe for self-hosted plan, invoice, and payment changes.', 'manage-billing-panel', 'manage-inline-email', ['Plan changes', 'Invoices']) +
                renderBillingActionRow('open-retrieve-billing', 'Retrieve licenses', 'Licenses', 'Recover the latest active self-hosted license and invoice link.', 'retrieve-billing-panel', 'retrieve-inline-email', ['Latest active license', 'Invoice lookup']) +
                renderBillingActionRow('open-refund-billing', 'Refund requests', 'Refunds', 'Request a self-serve refund when the purchase is still eligible.', 'refund-billing-panel', 'refund-inline-email', ['Eligibility check', 'Revocation']) +
                renderBillingActionRow('open-data-billing', 'Data and privacy', 'Privacy', 'Request export or deletion for commercial account data.', 'data-billing-panel', 'data-export-email', ['Export', 'Deletion']) +
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
    '</div>'
  );
}

function renderAuthScopeRow(title: string, copy: string): string {
  return (
    '<article class="portal-auth-scope-row">' +
      '<h3>' + escapeHTML(title) + '</h3>' +
      '<p>' + escapeHTML(copy) + '</p>' +
    '</article>'
  );
}

export function renderSignedOutPortalHTML(context: ShellViewContext): string {
  var statusHTML = '';
  if (context.loginState.request.error) {
    statusHTML = '<div class="billing-status visible error">' + escapeHTML(context.loginState.request.error) + '</div>';
  } else if (context.loginState.success) {
    var successMessage = context.loginState.successMessage || 'If that email is registered, a sign-in link is on the way.';
    statusHTML =
      '<div class="billing-status visible success">' +
      escapeHTML(successMessage) +
      '<br><br><strong>Need another link?</strong> <a href="#" data-portal-action="resend-magic-link">Send it again</a>.' +
      '</div>';
  }
  return (
    '<section class="portal-auth-shell">' +
      '<div class="portal-auth-intro">' +
        '<div class="account-panel-kicker">Pulse Account</div>' +
        '<h1>Sign in to Pulse Account</h1>' +
        '<p>Use one commercial email address for hosted workspaces, account access, billing, licenses, refunds, and privacy requests.</p>' +
        '<div class="portal-auth-scope-list" aria-label="Pulse Account scope">' +
          renderAuthScopeRow('Workspaces', 'Open hosted workspaces and review lifecycle state.') +
          renderAuthScopeRow('Access', 'Review account access and manage roles when permitted.') +
          renderAuthScopeRow('Billing', 'Open hosted billing or self-hosted commercial tools when they apply.') +
        '</div>' +
      '</div>' +
      '<section class="portal-auth-panel" aria-labelledby="portal-auth-title">' +
        '<div class="portal-auth-card">' +
          '<div class="account-panel-kicker">Sign in</div>' +
          '<h2 id="portal-auth-title">Email sign-in link</h2>' +
          '<p>Enter the commercial email address for your Pulse account. A sign-in link will be sent to that address.</p>' +
          '<div class="form-group portal-auth-form-group">' +
            '<label for="portal-login-email">Commercial email</label>' +
            '<input id="portal-login-email" type="email" autocomplete="email" placeholder="you@example.com" value="' +
            escapeAttr(context.loginState.emailValue || '') +
            '" data-portal-input="login-email">' +
          '</div>' +
          '<div class="form-actions portal-auth-actions">' +
            '<button class="btn-primary" id="portal-login-send" type="button" data-portal-action="send-magic-link">' +
            (context.loginState.request.pending ? 'Sending…' : 'Send sign-in link') +
            '</button>' +
          '</div>' +
          '<p class="portal-auth-secondary-action">Need a new Pulse Account? <a href="' + escapeAttr(context.signupPath) + '">Create an account</a>.</p>' +
          statusHTML +
        '</div>' +
      '</section>' +
    '</section>'
  );
}
