import type {
  PortalAccountSummary,
  PortalBillingState,
  PortalBootstrapData,
  PortalLoginState,
  PortalShellSection,
  PortalWorkspaceSummary,
} from './types';
import { portalRoleLabel } from './account_roles';
import { preferredPortalShellSection } from './shell_section';
import {
  workspaceHealthLabel,
  workspaceHealthState,
  workspaceRowNote,
  workspaceSetupGuide,
  workspaceSetupLabel,
  workspaceSetupNextStep,
  workspaceSetupState,
  workspaceStatusCopy,
} from './workspace_presentation';

export interface ShellViewContext {
  bootstrap: PortalBootstrapData;
  billingState: PortalBillingState;
  loginState: PortalLoginState;
  signupPath: string;
  accountAPIBasePath: string;
  activeSection?: PortalShellSection;
}

interface WorkspaceSummaryEntry {
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

function hasSignupPath(signupPath: string): boolean {
  return String(signupPath || '').trim() !== '';
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

function accountCountLabel(count: number): string {
  return count === 1 ? '1 account' : String(count) + ' accounts';
}



function reviewWorkspaceHeadline(count: number): string {
  return count === 1 ? '1 workspace needs review' : String(count) + ' workspaces need review';
}

function readyWorkspaceHeadline(count: number): string {
  return count === 1 ? '1 workspace is ready to use' : String(count) + ' workspaces are ready to use';
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

function setupNeededWorkspaceChipLabel(count: number): string {
  return count === 1 ? '1 workspace in setup' : String(count) + ' workspaces in setup';
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

function normalizeUpgradeFeatureKey(featureKey: string): string {
  return String(featureKey || '').trim();
}

function isSelfHostedPlanUpgrade(featureKey: string): boolean {
  var normalized = normalizeUpgradeFeatureKey(featureKey);
  return normalized === 'self_hosted_plan' || normalized === 'max_monitored_systems';
}

function selfHostedUpgradeActionTitle(featureKey: string): string {
  return isSelfHostedPlanUpgrade(featureKey)
    ? 'Compare self-hosted plans'
    : 'Upgrade self-hosted plan';
}

function selfHostedUpgradeActionDescription(featureKey: string): string {
  return isSelfHostedPlanUpgrade(featureKey)
    ? 'Compare self-hosted plans as monitor, reach, or operate instead of by monitored-system volume.'
    : 'Compare self-hosted plans and continue into the commercial checkout path.';
}

function selfHostedUpgradeActionHighlights(featureKey: string): string[] {
  return isSelfHostedPlanUpgrade(featureKey)
    ? ['Plan comparison', 'Plan checkout']
    : ['Plan comparison', 'Checkout handoff'];
}

function renderSelfHostedUpgradeActionRow(context: ShellViewContext): string {
  var featureKey = normalizeUpgradeFeatureKey(context.billingState.upgradeFeatureKey);
  return renderBillingActionRow(
    'open-upgrade-billing',
    selfHostedUpgradeActionTitle(featureKey),
    'Open',
    selfHostedUpgradeActionDescription(featureKey),
    'upgrade-billing-panel',
    'upgrade-billing-link',
    selfHostedUpgradeActionHighlights(featureKey),
  );
}

function renderSelfHostedUpgradeBillingPanel(context: ShellViewContext): string {
  var featureKey = normalizeUpgradeFeatureKey(context.billingState.upgradeFeatureKey);
  var helperCopy = isSelfHostedPlanUpgrade(featureKey)
    ? 'Choose the self-hosted tier that fits how you run Pulse: Community monitors, Relay reaches anywhere, and Pro investigates and helps fix issues. Pulse Account will send completed checkout directly back to the Plans page in Pulse.'
    : 'Choose the self-hosted tier that fits this upgrade. Pulse Account will send completed checkout directly back to the Plans page in Pulse.';
  return renderBillingTaskPanel(
    selfHostedUpgradeActionTitle(featureKey),
    'Pulse Account owns self-hosted plan selection and checkout for self-hosted upgrades.',
    'upgrade-billing-panel',
    '<div id="upgrade-billing-root"></div>' +
    '<div class="helper-text">' + escapeHTML(helperCopy) + '</div>',
  );
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

function collectWorkspaceSummaryEntries(accounts: PortalAccountSummary[]): WorkspaceSummaryEntry[] {
  var results: WorkspaceSummaryEntry[] = [];
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
    if (String(workspaces[i].state || '') === 'active' && workspaceSetupState(workspaces[i]) === 'ready') {
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

function setupBadgeHTML(workspace: PortalWorkspaceSummary): string {
  var setup = workspaceSetupState(workspace);
  return '<span class="badge badge-setup-' + escapeHTML(setup) + '">' + escapeHTML(workspaceSetupLabel(workspace)) + '</span>';
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
  return '';
}

function renderFactLine(className: string, facts: string[]): string {
  if (!facts.length) return '';
  return (
    '<div class="' + className + '">' +
      facts.map(function(fact) {
        return '<span>' + escapeHTML(fact) + '</span>';
      }).join('<span class="portal-fact-separator">•</span>') +
    '</div>'
  );
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

function renderIdentityBar(accounts: PortalAccountSummary[], showSelfHostedCommercial: boolean): string {
  if (accounts.length === 1) {
    var account = accounts[0];
    return (
      '<div class="portal-identity-bar">' +
        '<h2>' + escapeHTML(account.name) + '</h2>' +
        '<span class="portal-identity-sep">·</span>' +
        '<span>' + escapeHTML(portalRoleLabel(account.role)) + '</span>' +
        '<span class="portal-identity-sep">·</span>' +
        '<span>' + escapeHTML(accountKindLabel(account)) + '</span>' +
      '</div>'
    );
  }
  if (accounts.length > 1) {
    return (
      '<div class="portal-identity-bar">' +
        '<h2>Pulse Account</h2>' +
        '<span class="portal-identity-sep">·</span>' +
        '<span>' + String(accounts.length) + ' accounts</span>' +
      '</div>'
    );
  }
  return (
    '<div class="portal-identity-bar">' +
      '<h2>Pulse Account</h2>' +
      '<span class="portal-identity-sep">·</span>' +
      '<span>' + escapeHTML(showSelfHostedCommercial ? 'Self-hosted billing' : 'Billing') + '</span>' +
    '</div>'
  );
}

interface ShellNavEntry {
  section: PortalShellSection;
  title: string;
}

function primaryShellSections(bootstrap: PortalBootstrapData): ShellNavEntry[] {
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
  return sections;
}

function utilityShellSections(_bootstrap: PortalBootstrapData): ShellNavEntry[] {
  return [{ section: 'support', title: 'Support' }];
}

function visibleShellSections(bootstrap: PortalBootstrapData): ShellNavEntry[] {
  return primaryShellSections(bootstrap).concat(utilityShellSections(bootstrap));
}

function renderTabBar(bootstrap: PortalBootstrapData, activeSection: PortalShellSection): string {
  var sections = visibleShellSections(bootstrap);
  return (
    '<nav class="portal-tab-bar" aria-label="Pulse Account sections">' +
      sections.map(function(entry) {
        var isActive = activeSection === entry.section;
        var cls = 'portal-tab' + (isActive ? ' active' : '') + (entry.section === 'support' ? ' portal-tab-utility' : '');
        return '<button class="' + cls + '" type="button" data-shell-action="activate-section" data-shell-section="' + entry.section + '">' + entry.title + '</button>';
      }).join('') +
    '</nav>'
  );
}

function workspaceListAnchorID(accountID: string): string {
  return 'workspace-list-' + accountID;
}

function workspaceRowAnchorID(accountID: string, workspaceID: string): string {
  return 'workspace-row-' + accountID + '-' + workspaceID;
}

const WORKSPACE_INSTALL_TARGET_PATH = '/settings/infrastructure?add=linux-host';
const WORKSPACE_REPORTING_TARGET_PATH = '/settings/support/reporting';

function workspaceHandoffActionPath(accountAPIBasePath: string, accountID: string, workspaceID: string, targetPath = ''): string {
  var path = accountAPIBasePath + '/' + encodeURIComponent(accountID) + '/tenants/' + encodeURIComponent(workspaceID) + '/handoff';
  if (!targetPath) return path;
  return path + '?target_path=' + encodeURIComponent(targetPath);
}

function renderWorkspaceCard(account: PortalAccountSummary, workspace: PortalWorkspaceSummary, accountAPIBasePath: string): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  var createdLabel = formatWorkspaceDate(workspace.created_at);
  var metaParts = [];
  if (state) {
    metaParts.push('<span class="workspace-meta-item">' + escapeHTML(titleCase(state)) + '</span>');
  }
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
      escapeAttr(workspaceHandoffActionPath(accountAPIBasePath, account.id, workspace.id)) +
      '">' +
      '<button type="submit" class="btn-primary">Open workspace</button>' +
      '</form>';
  }

  var installAction = '';
  if (state === 'active') {
    installAction =
      '<form method="POST" action="' +
      escapeAttr(workspaceHandoffActionPath(accountAPIBasePath, account.id, workspace.id, WORKSPACE_INSTALL_TARGET_PATH)) +
      '">' +
      '<button type="submit" class="btn-secondary">Install agents</button>' +
      '</form>';
  }

  var manageAction = '';
  if (account.can_manage && (state === 'active' || state === 'suspended' || state === 'failed')) {
    manageAction =
      '<button type="button" class="btn-secondary btn-workspace-manage" data-action="select-workspace" data-account-id="' +
      escapeAttr(account.id) +
      '" data-workspace-id="' +
      escapeAttr(workspace.id) +
      '">Setup checklist</button>';
  }

  return (
    '<article class="workspace-row workspace-row-health-' + escapeAttr(status) + ' workspace-row-state-' + escapeAttr(state || 'unknown') + '" id="' + escapeAttr(workspaceRowAnchorID(account.id, workspace.id)) + '" data-workspace-row="' + escapeAttr(workspace.id) + '">' +
      '<div class="workspace-row-primary">' +
        '<div class="workspace-row-heading">' +
          '<h4 class="workspace-name">' + escapeHTML(workspace.display_name) + '</h4>' +
          '<div class="workspace-meta">' + metaParts.join('') + '</div>' +
        '</div>' +
        '<div class="workspace-row-note">' + escapeHTML(workspaceRowNote(workspace)) + '</div>' +
      '</div>' +
      '<div class="workspace-row-status-cell workspace-row-status-cell-badge">' +
        setupBadgeHTML(workspace) +
      '</div>' +
      '<div class="workspace-row-status-cell workspace-row-status-cell-badge">' +
        healthBadgeHTML(workspace) +
      '</div>' +
      '<div class="workspace-actions">' +
        openAction +
        installAction +
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
    escapeAttr(workspaceHandoffActionPath(accountAPIBasePath, accountID, workspaceID)) +
    '">' +
    '<button type="submit" class="' + escapeAttr(buttonClassName) + '">' + escapeHTML(label) + '</button>' +
    '</form>'
  );
}

function renderWorkspaceInstallHandoffForm(accountID: string, workspaceID: string, accountAPIBasePath: string, label = 'Install agents', buttonClassName = 'btn-secondary btn-compact'): string {
  if (!accountAPIBasePath) {
    return '<button class="' + escapeAttr(buttonClassName) + '" type="button" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(label) + '</button>';
  }
  return (
    '<form method="POST" action="' +
    escapeAttr(workspaceHandoffActionPath(accountAPIBasePath, accountID, workspaceID, WORKSPACE_INSTALL_TARGET_PATH)) +
    '">' +
    '<button type="submit" class="' + escapeAttr(buttonClassName) + '">' + escapeHTML(label) + '</button>' +
    '</form>'
  );
}

function renderWorkspaceReportingHandoffForm(accountID: string, workspaceID: string, accountAPIBasePath: string, label = 'Open reports', buttonClassName = 'btn-secondary btn-compact'): string {
  if (!accountAPIBasePath) {
    return '<button class="' + escapeAttr(buttonClassName) + '" type="button" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(label) + '</button>';
  }
  return (
    '<form method="POST" action="' +
    escapeAttr(workspaceHandoffActionPath(accountAPIBasePath, accountID, workspaceID, WORKSPACE_REPORTING_TARGET_PATH)) +
    '">' +
    '<button type="submit" class="' + escapeAttr(buttonClassName) + '">' + escapeHTML(label) + '</button>' +
    '</form>'
  );
}

function attentionWorkspaceEntries(entries: WorkspaceSummaryEntry[]): WorkspaceSummaryEntry[] {
  var results: WorkspaceSummaryEntry[] = [];
  for (var i = 0; i < entries.length; i += 1) {
    var status = workspaceHealthState(entries[i].workspace);
    if (status === 'unhealthy' || status === 'checking') {
      results.push(entries[i]);
    }
  }
  return results;
}

function readyWorkspaceEntries(entries: WorkspaceSummaryEntry[]): WorkspaceSummaryEntry[] {
  var results: WorkspaceSummaryEntry[] = [];
  for (var i = 0; i < entries.length; i += 1) {
    if (String(entries[i].workspace.state || '') === 'active' && workspaceSetupState(entries[i].workspace) === 'ready') {
      results.push(entries[i]);
    }
  }
  return results;
}

function suspendedWorkspaceEntries(entries: WorkspaceSummaryEntry[]): WorkspaceSummaryEntry[] {
  var results: WorkspaceSummaryEntry[] = [];
  for (var i = 0; i < entries.length; i += 1) {
    if (String(entries[i].workspace.state || '') === 'suspended') {
      results.push(entries[i]);
    }
  }
  return results;
}

function setupNeededWorkspaceEntries(entries: WorkspaceSummaryEntry[]): WorkspaceSummaryEntry[] {
  var results: WorkspaceSummaryEntry[] = [];
  for (var i = 0; i < entries.length; i += 1) {
    if (String(entries[i].workspace.state || '') !== 'active') continue;
    if (workspaceHealthState(entries[i].workspace) !== 'healthy') continue;
    var setup = workspaceSetupState(entries[i].workspace);
    if (setup === 'install_agents' || setup === 'configure_outputs' || setup === 'setup_path') {
      results.push(entries[i]);
    }
  }
  return results;
}

function setupFactCountLabel(value: unknown, singular: string, plural: string): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 'Unknown ' + plural;
  return String(value) + ' ' + (value === 1 ? singular : plural);
}

function workspaceSetupFactsLine(workspace: PortalWorkspaceSummary): string {
  return [
    setupFactCountLabel(workspace.agent_count, 'agent', 'agents'),
    setupFactCountLabel(workspace.alert_route_count, 'alert route', 'alert routes'),
    setupFactCountLabel(workspace.report_schedule_count, 'report schedule', 'report schedules'),
  ].join(' · ');
}

function workspaceSetupDiagnosticsLine(workspace: PortalWorkspaceSummary): string {
  var guide = workspaceSetupGuide(workspace);
  return guide.diagnostics.length ? guide.diagnostics[0] : workspaceSetupNextStep(workspace);
}

function workspaceSummaryContext(entry: WorkspaceSummaryEntry, includeAccountName: boolean, note: string): string {
  if (!includeAccountName) return note;
  return entry.account.name + ' · ' + note;
}

function renderWorkspaceAnchorAction(anchorID: string, label: string, className = 'btn-secondary btn-compact workspace-summary-link'): string {
  return '<a class="' + escapeAttr(className) + '" href="#' + escapeAttr(anchorID) + '">' + escapeHTML(label) + '</a>';
}

interface WorkspaceSummaryDecision {
  title: string;
  description: string;
  primaryAction: string;
  secondaryAction: string;
}

function renderWorkspaceSummaryDecision(
  accounts: PortalAccountSummary[],
  entries: WorkspaceSummaryEntry[],
  accountAPIBasePath: string,
  showSelfHostedCommercial: boolean
): WorkspaceSummaryDecision {
  var attention = attentionWorkspaceEntries(entries);
  var suspended = suspendedWorkspaceEntries(entries);
  var setupNeeded = setupNeededWorkspaceEntries(entries);
  var ready = readyWorkspaceEntries(entries);
  var primaryAction = '';
  var secondaryAction = '';
  var title = '';
  var description = '';
  var totalWorkspaces = countWorkspaces(accounts);
  var creatableAccount = accounts.find(function(account) {
    return account.kind === 'msp' && account.can_manage;
  }) || null;
  var accessAccount = accounts.find(function(account) {
    return account.can_manage;
  }) || null;
  var hostedViewOnly = accounts.length > 0 && !accessAccount;

  if (attention.length) {
    var attentionEntry = attention[0];
    title = 'Review ' + attentionEntry.workspace.display_name;
    description = workspaceSummaryContext(attentionEntry, accounts.length > 1, workspaceStatusCopy(attentionEntry.workspace));
    primaryAction = renderWorkspaceAnchorAction(
      workspaceRowAnchorID(attentionEntry.account.id, attentionEntry.workspace.id),
      'Review workspace',
      'btn-primary btn-compact workspace-summary-link',
    );
    secondaryAction = attentionEntry.account.can_manage && (
      attentionEntry.workspace.state === 'active' ||
      attentionEntry.workspace.state === 'suspended' ||
      attentionEntry.workspace.state === 'failed'
    )
      ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' +
        escapeAttr(attentionEntry.account.id) +
        '" data-workspace-id="' +
        escapeAttr(attentionEntry.workspace.id) +
        '">Setup checklist</button>'
      : '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
  } else if (suspended.length) {
    var suspendedEntry = suspended[0];
    title = 'Review ' + suspendedEntry.workspace.display_name;
    description = workspaceSummaryContext(suspendedEntry, accounts.length > 1, workspaceStatusCopy(suspendedEntry.workspace));
    primaryAction = renderWorkspaceAnchorAction(
      workspaceRowAnchorID(suspendedEntry.account.id, suspendedEntry.workspace.id),
      'Review workspace',
      'btn-primary btn-compact workspace-summary-link',
    );
    secondaryAction = suspendedEntry.account.can_manage
      ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' +
        escapeAttr(suspendedEntry.account.id) +
        '" data-workspace-id="' +
        escapeAttr(suspendedEntry.workspace.id) +
        '">Setup checklist</button>'
      : '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
  } else if (setupNeeded.length) {
    var setupEntry = setupNeeded[0];
    var setupState = workspaceSetupState(setupEntry.workspace);
    title = setupState === 'configure_outputs'
      ? 'Configure outputs for ' + setupEntry.workspace.display_name
      : 'Set up ' + setupEntry.workspace.display_name;
    description = workspaceSummaryContext(setupEntry, accounts.length > 1, workspaceSetupNextStep(setupEntry.workspace));
    primaryAction = setupState === 'configure_outputs'
      ? renderWorkspaceReportingHandoffForm(
        setupEntry.account.id,
        setupEntry.workspace.id,
        accountAPIBasePath,
        'Open reports',
        'btn-primary btn-compact',
      )
      : renderWorkspaceInstallHandoffForm(
        setupEntry.account.id,
        setupEntry.workspace.id,
        accountAPIBasePath,
        'Install agents',
        'btn-primary btn-compact',
      );
    secondaryAction = setupEntry.account.can_manage
      ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' +
        escapeAttr(setupEntry.account.id) +
        '" data-workspace-id="' +
        escapeAttr(setupEntry.workspace.id) +
        '">Setup checklist</button>'
      : renderWorkspaceHandoffForm(setupEntry.account.id, setupEntry.workspace.id, accountAPIBasePath, 'Open workspace');
  } else if (ready.length) {
    var readyEntry = ready[0];
    title = 'Open ' + readyEntry.workspace.display_name;
    description = workspaceSummaryContext(readyEntry, accounts.length > 1, workspaceRowNote(readyEntry.workspace));
    primaryAction = renderWorkspaceHandoffForm(
      readyEntry.account.id,
      readyEntry.workspace.id,
      accountAPIBasePath,
      'Open workspace',
      'btn-primary btn-compact',
    );
    secondaryAction = ready.length > 1
      ? renderWorkspaceAnchorAction(workspaceListAnchorID(readyEntry.account.id), 'See all workspaces')
      : renderWorkspaceInstallHandoffForm(readyEntry.account.id, readyEntry.workspace.id, accountAPIBasePath);
  } else if (creatableAccount) {
    title = 'Create the first workspace';
    description = 'No hosted workspace is attached yet. Create the first workspace in ' + creatableAccount.name + '.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(creatableAccount.id) + '">Create workspace</button>';
    secondaryAction = accessAccount
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>'
      : '';
  } else if (hostedViewOnly) {
    if (entries.length > 0) {
      title = 'Review who can act';
      description = 'Hosted workspaces are attached here, but an owner or admin must make account-level changes.';
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
      secondaryAction = renderWorkspaceAnchorAction(workspaceListAnchorID(accounts[0].id), 'Review workspace list');
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
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
    secondaryAction = showSelfHostedCommercial
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>'
      : '';
  } else {
    title = 'Open billing or support';
    description = totalWorkspaces > 0
      ? 'Review the workspace list here, then use Billing for commercial work or Support only after a self-service path fails.'
      : 'Use Billing for commercial work. Use Support only after the billing path fails.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="billing">Open billing</button>';
    secondaryAction = '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="support">Escalate</button>';
  }

  return {
    title: title,
    description: description,
    primaryAction: primaryAction,
    secondaryAction: secondaryAction,
  };
}

function renderWorkspaceSummaryFacts(accounts: PortalAccountSummary[], entries: WorkspaceSummaryEntry[]): string[] {
  return [
    accountCountLabel(accounts.length),
    workspaceCountLabel(entries.length),
    readyWorkspaceChipLabel(readyWorkspaceEntries(entries).length),
    setupNeededWorkspaceChipLabel(setupNeededWorkspaceEntries(entries).length),
    reviewWorkspaceChipLabel(attentionWorkspaceEntries(entries).length),
    suspendedWorkspaceChipLabel(suspendedWorkspaceEntries(entries).length),
  ];
}

function renderWorkspaceSummaryInline(
  accounts: PortalAccountSummary[],
  entries: WorkspaceSummaryEntry[],
  accountAPIBasePath: string,
  showSelfHostedCommercial: boolean
): string {
  var decision = renderWorkspaceSummaryDecision(accounts, entries, accountAPIBasePath, showSelfHostedCommercial);
  return (
    '<section class="workspace-summary-inline">' +
      '<div class="workspace-summary-inline-copy">' +
        '<p><strong>Next:</strong> ' + escapeHTML(decision.title) + '</p>' +
        '<p>' + escapeHTML(decision.description) + '</p>' +
      '</div>' +
      '<div class="workspace-summary-actions">' +
        decision.primaryAction +
        decision.secondaryAction +
      '</div>' +
    '</section>'
  );
}

function renderWorkspaceSetupQueueAction(entry: WorkspaceSummaryEntry, accountAPIBasePath: string): string {
  var setup = workspaceSetupState(entry.workspace);
  if (setup === 'configure_outputs') {
    return renderWorkspaceReportingHandoffForm(
      entry.account.id,
      entry.workspace.id,
      accountAPIBasePath,
      'Configure outputs',
      'btn-primary btn-compact',
    );
  }
  return renderWorkspaceInstallHandoffForm(
    entry.account.id,
    entry.workspace.id,
    accountAPIBasePath,
    setup === 'install_agents' ? 'Install agents' : 'Open setup',
    'btn-primary btn-compact',
  );
}

function renderProviderSetupTemplates(accounts: PortalAccountSummary[]): string {
  var templateAccount = accounts.find(function(account) {
    return account.kind === 'msp' && Array.isArray(account.setup_templates) && account.setup_templates.length > 0;
  });
  if (!templateAccount || !templateAccount.setup_templates || !templateAccount.setup_templates.length) return '';
  var template = templateAccount.setup_templates[0];
  return (
    '<section class="workspace-template-panel" aria-label="Provider setup template">' +
      '<div class="workspace-template-heading">' +
        '<div>' +
          '<h3>' + escapeHTML(template.title || 'Provider setup template') + '</h3>' +
          '<p>Use the same setup shape for each client workspace, then finish the tenant-owned configuration inside that workspace.</p>' +
        '</div>' +
        '<span>' + escapeHTML(templateAccount.name) + '</span>' +
      '</div>' +
      '<div class="workspace-template-grid">' +
        '<div><strong>Agent naming</strong><span>' + escapeHTML(template.agent_naming) + '</span></div>' +
        '<div><strong>Alert routing</strong><span>' + escapeHTML(template.alert_routing) + '</span></div>' +
        '<div><strong>Reports</strong><span>' + escapeHTML(template.reporting) + '</span></div>' +
        '<div><strong>Access</strong><span>' + escapeHTML(template.access) + '</span></div>' +
      '</div>' +
    '</section>'
  );
}

function renderWorkspaceSetupQueue(entries: WorkspaceSummaryEntry[], accountAPIBasePath: string): string {
  var setupNeeded = setupNeededWorkspaceEntries(entries);
  if (!setupNeeded.length) return '';
  var visible = setupNeeded.slice(0, 5);
  return (
    '<section class="workspace-setup-queue" aria-label="Unfinished workspace setup">' +
      '<div class="workspace-setup-queue-header">' +
        '<div>' +
          '<h3>Unfinished setup</h3>' +
          '<p>Client workspaces stay here until agents, alert routing, and reports are in place.</p>' +
        '</div>' +
        '<span>' + escapeHTML(setupNeededWorkspaceChipLabel(setupNeeded.length)) + '</span>' +
      '</div>' +
      '<div class="workspace-setup-queue-list">' +
        visible.map(function(entry) {
          return (
            '<article class="workspace-setup-queue-row">' +
              '<div class="workspace-setup-queue-main">' +
                setupBadgeHTML(entry.workspace) +
                '<div>' +
                  '<strong>' + escapeHTML(entry.workspace.display_name) + '</strong>' +
                  '<span>' + escapeHTML(entry.account.name + ' · ' + workspaceSetupDiagnosticsLine(entry.workspace)) + '</span>' +
                  '<small>' + escapeHTML(workspaceSetupFactsLine(entry.workspace)) + '</small>' +
                '</div>' +
              '</div>' +
              '<div class="workspace-setup-queue-actions">' +
                renderWorkspaceSetupQueueAction(entry, accountAPIBasePath) +
                (entry.account.can_manage
                  ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' +
                    escapeAttr(entry.account.id) +
                    '" data-workspace-id="' +
                    escapeAttr(entry.workspace.id) +
                    '">Checklist</button>'
                  : renderWorkspaceHandoffForm(entry.account.id, entry.workspace.id, accountAPIBasePath, 'Open workspace')) +
              '</div>' +
            '</article>'
          );
        }).join('') +
      '</div>' +
    '</section>'
  );
}

function workspaceSectionHeaderCopy(accounts: PortalAccountSummary[], entries: WorkspaceSummaryEntry[]): string {
  var canManageAnyWorkspace = accounts.some(function(account) { return account.can_manage; });
  if (!entries.length) {
    return canManageAnyWorkspace
      ? 'Review hosted workspaces here, then create the next workspace when you are ready.'
      : 'Review hosted workspace state here. An owner or admin must create or change hosted workspaces.';
  }
  if (!canManageAnyWorkspace) {
    return 'Review hosted workspace health here and open ready workspaces. An owner or admin must handle setup and workspace changes.';
  }
  return 'Review client workspaces here, use the setup checklist for onboarding, and keep destructive workspace actions separate from daily workspace work.';
}

export function renderWorkspaceSummarySection(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var entries = collectWorkspaceSummaryEntries(accounts);
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);

  return (
    '<section class="workspace-summary-shell">' +
      '<div class="portal-page-header">' +
        '<h2>Workspaces</h2>' +
        '<p>' + escapeHTML(workspaceSectionHeaderCopy(accounts, entries)) + '</p>' +
      '</div>' +
      renderFactLine('workspace-summary-facts', renderWorkspaceSummaryFacts(accounts, entries)) +
      renderWorkspaceSummaryInline(accounts, entries, context.accountAPIBasePath, showSelfHostedCommercial) +
      renderProviderSetupTemplates(accounts) +
      renderWorkspaceSetupQueue(entries, context.accountAPIBasePath) +
    '</section>'
  );
}

function renderAccountBlockHeader(
  _account: PortalAccountSummary,
  actionsHTML = '',
  copy = ''
): string {
  return (
    (actionsHTML ? '<div class="portal-section-header"><div></div><div>' + actionsHTML + '</div></div>' : '') +
    (copy ? '<p class="portal-section-copy">' + escapeHTML(copy) + '</p>' : '')
  );
}

function renderNoHostedWorkspacesSection(): string {
  return (
    '<section class="account-content-panel account-content-panel-workspaces">' +
      '<div class="empty-state">' +
        '<p>No hosted workspaces are attached to this account. Use Billing for self-hosted subscriptions and licenses.</p>' +
      '</div>' +
    '</section>'
  );
}

function renderNoHostedAccessSection(): string {
  return (
    '<section class="account-content-panel account-content-panel-access">' +
      '<div class="empty-state">' +
        '<p>No hosted account roster is attached. Use Billing for commercial access to licenses, refunds, or privacy.</p>' +
      '</div>' +
    '</section>'
  );
}

function renderAccountWorkspaceSection(account: PortalAccountSummary, accountAPIBasePath: string): string {
  var workspaces = Array.isArray(account.workspaces) ? account.workspaces : [];
  var readyCount = countReadyWorkspaces(workspaces);
  var attentionCount = attentionWorkspaces(workspaces).length;
  var suspendedCount = countWorkspacesByState(workspaces, 'suspended');
  var workspaceListSummary = account.can_manage
    ? 'Open a workspace to work in it, or use Setup checklist when onboarding a client.'
    : 'Open a workspace here. An owner or admin must create or change hosted workspaces.';
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
            '<h3>Workspace setup checklist</h3>' +
            '<p>Finish the client setup steps without mixing client data across workspaces.</p>' +
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
                '<h4>Create a workspace</h4>' +
                '<p>Add a new hosted workspace for a customer or operating boundary.</p>' +
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
              '<span>Setup</span>' +
              '<strong id="workspace-management-setup-' + escapeAttr(account.id) + '"></strong>' +
            '</div>' +
            '<div class="workspace-management-fact">' +
              '<span>Agents</span>' +
              '<strong id="workspace-management-agents-' + escapeAttr(account.id) + '"></strong>' +
            '</div>' +
            '<div class="workspace-management-fact">' +
              '<span>Alert routes</span>' +
              '<strong id="workspace-management-alerts-' + escapeAttr(account.id) + '"></strong>' +
            '</div>' +
            '<div class="workspace-management-fact">' +
              '<span>Report schedules</span>' +
              '<strong id="workspace-management-reports-' + escapeAttr(account.id) + '"></strong>' +
            '</div>' +
            '<div class="workspace-management-fact">' +
              '<span>Created</span>' +
              '<strong id="workspace-management-created-' + escapeAttr(account.id) + '"></strong>' +
            '</div>' +
          '</div>' +
          '<div class="workspace-management-guidance" id="workspace-management-guidance-' +
          escapeAttr(account.id) +
          '"></div>' +
          '<div class="workspace-management-identity" id="workspace-management-identity-' +
          escapeAttr(account.id) +
          '"></div>' +
          '<div class="workspace-setup-guide" aria-label="Guided workspace setup">' +
            '<div class="workspace-setup-guide-copy">' +
              '<span>Current step</span>' +
              '<h4 id="workspace-management-guide-title-' +
              escapeAttr(account.id) +
              '"></h4>' +
              '<p id="workspace-management-guide-description-' +
              escapeAttr(account.id) +
              '"></p>' +
              '<ul id="workspace-management-guide-diagnostics-' +
              escapeAttr(account.id) +
              '"></ul>' +
            '</div>' +
            '<form method="POST" id="workspace-management-primary-form-' +
            escapeAttr(account.id) +
            '">' +
              '<button type="submit" class="btn-primary btn-compact" id="workspace-management-primary-' +
              escapeAttr(account.id) +
              '">Continue setup</button>' +
            '</form>' +
          '</div>' +
          '<div class="workspace-setup-checklist" aria-label="Workspace setup checklist">' +
            '<div class="workspace-setup-step workspace-setup-step-created">' +
              '<span class="workspace-setup-status" id="workspace-management-check-created-' + escapeAttr(account.id) + '"></span>' +
              '<div><strong>Workspace created</strong><span>This client has a separate account boundary.</span></div>' +
            '</div>' +
            '<div class="workspace-setup-step">' +
              '<span class="workspace-setup-status" id="workspace-management-check-install-' + escapeAttr(account.id) + '"></span>' +
              '<div><strong>Install the first agent</strong><span>Use the workspace-bound install path so data lands in this client.</span></div>' +
            '</div>' +
            '<div class="workspace-setup-step">' +
              '<span class="workspace-setup-status" id="workspace-management-check-alerts-' + escapeAttr(account.id) + '"></span>' +
              '<div><strong>Configure alert routes</strong><span>Keep notifications scoped to this client.</span></div>' +
            '</div>' +
            '<div class="workspace-setup-step">' +
              '<span class="workspace-setup-status" id="workspace-management-check-reports-' + escapeAttr(account.id) + '"></span>' +
              '<div><strong>Schedule reports</strong><span>Send client performance reports from this workspace.</span></div>' +
            '</div>' +
            '<div class="workspace-setup-step">' +
              '<span class="workspace-setup-status" id="workspace-management-check-access-' + escapeAttr(account.id) + '"></span>' +
              '<div><strong>Review access</strong><span>Invite provider staff or client users from Access.</span></div>' +
            '</div>' +
          '</div>' +
          '<div class="workspace-management-next-steps" id="workspace-management-next-steps-' +
          escapeAttr(account.id) +
          '">' +
            '<div class="workspace-next-step">' +
              '<div><strong>Open workspace</strong><span>Work inside this client boundary.</span></div>' +
              '<form method="POST" id="workspace-management-open-form-' +
              escapeAttr(account.id) +
              '">' +
                '<button type="submit" class="btn-primary btn-compact" id="workspace-management-open-' +
                escapeAttr(account.id) +
                '">Open workspace</button>' +
              '</form>' +
            '</div>' +
            '<div class="workspace-next-step">' +
              '<div><strong>Install agents</strong><span>Open the workspace-bound install commands.</span></div>' +
              '<form method="POST" id="workspace-management-install-form-' +
              escapeAttr(account.id) +
              '">' +
                '<button type="submit" class="btn-secondary btn-compact" id="workspace-management-install-' +
                escapeAttr(account.id) +
                '">Install agents</button>' +
              '</form>' +
            '</div>' +
            '<div class="workspace-next-step workspace-next-step-readonly">' +
              '<div><strong>Alerts and reports</strong><span>Alerts and performance reports stay inside the client workspace.</span></div>' +
              '<form method="POST" id="workspace-management-reporting-form-' +
              escapeAttr(account.id) +
              '">' +
                '<button type="submit" class="btn-secondary btn-compact" id="workspace-management-reporting-' +
                escapeAttr(account.id) +
                '">Open reports</button>' +
              '</form>' +
            '</div>' +
            '<div class="workspace-next-step workspace-next-step-readonly">' +
              '<div><strong>Access</strong><span>Invite people or adjust roles from the account access boundary.</span></div>' +
              '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">Open Access</button>' +
            '</div>' +
          '</div>' +
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
    ? '<div class="workspace-list-wrap" id="' + escapeAttr(workspaceListAnchorID(account.id)) + '">' +
          (workspaceHeaderActions ? '<div class="workspace-list-toolbar">' + workspaceHeaderActions + '</div>' : '') +
          '<div class="workspace-list-head">' +
            '<span>Workspace</span>' +
            '<span>Setup</span>' +
            '<span>Health</span>' +
            '<span>Actions</span>' +
          '</div>' +
          '<div class="workspace-list">' + workspaces.map(function(workspace) {
          return renderWorkspaceCard(account, workspace, accountAPIBasePath);
        }).join('') + '</div>' +
        '</div>'
    : '<div class="empty-state">' +
        '<p>' + escapeHTML(account.can_manage ? 'No hosted workspaces yet. Create one to get started.' : 'No hosted workspaces are attached yet. An owner or admin must create the first one.') + '</p>' +
        (workspaceHeaderActions ? '<div style="margin-top: 8px">' + workspaceHeaderActions + '</div>' : '') +
      '</div>';

  return (
    '<section class="account-content-panel account-content-panel-workspaces">' +
      '<div class="workspace-operations-shell workspace-operations-shell-idle" id="workspace-operations-shell-' +
        escapeAttr(account.id) +
        '">' +
        '<div class="workspace-operations-detail" id="workspace-operations-detail-' +
          escapeAttr(account.id) +
          '" hidden>' +
          workspaceManagement +
        '</div>' +
        '<div class="workspace-operations-main">' +
          workspaceHTML +
        '</div>' +
      '</div>' +
    '</section>'
  );
}

function renderAccountAccessSection(account: PortalAccountSummary): string {
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
        (!account.can_manage
          ? '<p class="portal-section-copy">' + escapeHTML('Review who has access. An owner or admin must make changes.') + '</p>'
          : '') +
        '<div class="access-management-stats" id="access-stats-' +
        escapeAttr(account.id) +
        '"></div>' +
        '<div class="access-shell access-shell-idle" id="access-shell-' + escapeAttr(account.id) + '">' +
          (account.can_manage
            ? (
              '<div class="access-shell-detail" id="access-detail-' + escapeAttr(account.id) + '" hidden>' +
                '<div class="access-task-panel" id="access-task-panel-' + escapeAttr(account.id) + '" hidden>' +
                  '<div class="access-task-header">' +
                    '<div>' +
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
          '<div class="access-shell-main">' +
            '<div class="access-roster-column">' +
              '<div class="access-roster">' +
                (account.can_manage
                  ? '<div class="access-roster-toolbar">' + accessTaskStrip + '</div>'
                  : '') +
                '<div class="access-roster-list" id="access-list-' +
                escapeAttr(account.id) +
                '">' +
                  '<div class="access-list-message">Loading…</div>' +
                '</div>' +
              '</div>' +
            '</div>' +
          '</div>' +
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
      '<section class="billing-surface-block billing-surface-block-empty">' +
        '<div class="billing-surface-header">' +
          '<h3>No hosted billing attached</h3>' +
        '</div>' +
        '<p>' + escapeHTML(showSelfHostedCommercial
          ? 'Use self-hosted billing tools below for self-hosted purchases.'
          : 'Hosted invoices and payment methods are not attached to this account.'
        ) + '</p>' +
      '</section>'
    );
  }

  return (
    '<section class="billing-surface-block">' +
      '<div class="billing-surface-header">' +
        '<h3>Hosted billing</h3>' +
      '</div>' +
      '<div class="billing-action-list billing-action-list-surface">' +
        hostedBillingAccounts.map(function(account) {
          var actionHTML = account.can_manage
            ? '<button type="button" class="btn-primary btn-compact" data-action="open-billing" data-account-id="' + escapeAttr(account.id) + '">Open hosted billing</button>'
            : '<div class="billing-task-note">An owner or admin on this account needs to open hosted billing.</div>';
          return (
            '<article class="billing-action-row billing-action-row-surface">' +
              '<div class="billing-action-main">' +
                '<div class="billing-action-copy">' +
                  '<h3>' + escapeHTML(account.name) + '</h3>' +
                  '<p>Invoices, payment methods, and hosted subscription changes for this account.</p>' +
                '</div>' +
                '<div class="billing-action-meta">Keep workspace changes in Workspaces and roster changes in Access.</div>' +
              '</div>' +
              '<div class="billing-action-cta">' + actionHTML + '</div>' +
            '</article>'
          );
        }).join('') +
      '</div>' +
    '</section>'
  );
}

function renderBillingTaskPanel(title: string, copy: string, panelID: string, bodyHTML: string): string {
  return (
    '<section class="billing-panel" id="' + escapeAttr(panelID) + '" hidden>' +
      '<div class="billing-task-header">' +
        '<div>' +
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
  var isHosted = accounts.length > 0;
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
  var supportEmail = context.bootstrap.support_email || '';
  var canManageHostedTasks = false;
  for (var i = 0; i < accounts.length; i += 1) {
    if (accounts[i].can_manage) {
      canManageHostedTasks = true;
      break;
    }
  }
  var hostedViewOnly = isHosted && !canManageHostedTasks;
  var retryCopy = isHosted
    ? (
      hostedViewOnly
        ? 'Review Workspaces or Access first. If billing is involved, hand it to an owner or admin before you escalate.'
        : 'Retry the same Workspaces, Access, or Billing step before you escalate.'
    )
    : 'Retry the same Billing step before you escalate.';
  var supportActions = isHosted
    ? (
      '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">Workspaces</button>' +
      '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">Access</button>' +
      '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Billing</button>'
    )
    : '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Billing</button>';
  return (
    '<section class="portal-support-panel">' +
      '<p>Use Support only after the self-service path fails. Retry the same step before you escalate.</p>' +
      '<div class="portal-support-simple">' +
        '<div class="portal-support-simple-card">' +
          '<div class="portal-support-simple-list">' +
            '<div class="portal-support-simple-row"><strong>Try first</strong><span>' + escapeHTML(retryCopy) + '</span></div>' +
            '<div class="portal-support-simple-row"><strong>Scope</strong><span>' + escapeHTML(supportRunbookPathCopy(isHosted, hostedViewOnly, showSelfHostedCommercial)) + '</span></div>' +
            '<div class="portal-support-simple-row"><strong>Include</strong><span>Account, email, and the exact action that failed.</span></div>' +
          '</div>' +
          '<div class="portal-support-simple-actions">' +
            supportActions +
            '<a class="portal-support-link" href="mailto:' + escapeAttr(supportEmail) + '">' + escapeHTML(supportEmail) + '</a>' +
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
  if (!hasSignupPath(context.signupPath)) {
    return '';
  }
  return '<a class="logout-btn link-button" href="' + escapeAttr(context.signupPath) + '">Create account</a>';
}

export function renderAuthenticatedPortalHTML(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var hosted = hasHostedAccounts(accounts);
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
  var showSelfHostedUpgradeHandoff =
    context.billingState.openBillingPanelID === 'upgrade-billing-panel' ||
    !!normalizeUpgradeFeatureKey(context.billingState.upgradeFeatureKey) ||
    !!String(context.billingState.upgradePortalHandoffID || '').trim();
  var showSelfHostedBillingShell = showSelfHostedCommercial || showSelfHostedUpgradeHandoff;
  var shellSections = visibleShellSections(context.bootstrap);
  var preferredSection = context.activeSection || preferredPortalShellSection(context.bootstrap);
  var activeSection = shellSections.some(function(entry) {
    return entry.section === preferredSection;
  })
    ? preferredSection
    : (shellSections[0] ? shellSections[0].section : 'billing');
  var billingNote = hosted
    ? (showSelfHostedCommercial
      ? 'Hosted billing by account. Self-hosted purchases stay separate below.'
      : 'Hosted billing by account.')
    : 'Self-hosted subscriptions, licenses, refunds, and privacy requests.';
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
  var workspaceSummaryContent = hosted ? renderWorkspaceSummarySection(context) : '';
  var accessContent = accounts.length
    ? accounts.map(function(account) {
      return (
        '<section class="account-surface">' +
          renderAccountAccessSection(account) +
        '</section>'
      );
    }).join('')
    : renderNoHostedAccessSection();
  var selfHostedBillingLeadCopy = showSelfHostedCommercial
    ? 'Use self-hosted billing only for self-hosted purchases.'
    : 'Pulse Account owns the commercial handoff for self-hosted upgrades from the app.';
  var selfHostedBillingActionsHTML = renderSelfHostedUpgradeActionRow(context);
  if (showSelfHostedCommercial) {
    selfHostedBillingActionsHTML +=
      renderBillingActionRow('open-manage-billing', 'Manage subscriptions', 'Open', 'Open Stripe for self-hosted plan, invoice, and payment changes.', 'manage-billing-panel', 'manage-inline-email', ['Plan changes', 'Invoices']) +
      renderBillingActionRow('open-retrieve-billing', 'Retrieve licenses', 'Open', 'Recover the latest active self-hosted license and invoice link.', 'retrieve-billing-panel', 'retrieve-inline-email', ['Latest active license', 'Invoice lookup']) +
      renderBillingActionRow('open-refund-billing', 'Refund requests', 'Open', 'Request a self-serve refund when the purchase is still eligible.', 'refund-billing-panel', 'refund-inline-email', ['Eligibility check', 'Revocation']) +
      renderBillingActionRow('open-data-billing', 'Data and privacy', 'Open', 'Request export or deletion for commercial account data.', 'data-billing-panel', 'data-export-email', ['Export', 'Deletion']);
  }
  var selfHostedBillingPanelsHTML = renderSelfHostedUpgradeBillingPanel(context);
  if (showSelfHostedCommercial) {
    selfHostedBillingPanelsHTML +=
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
      );
  }

  return (
    '<div class="portal-shell" data-shell-section="' + activeSection + '">' +
      '<div class="portal-shell-main">' +
        renderIdentityBar(accounts, showSelfHostedCommercial) +
        renderTabBar(context.bootstrap, activeSection) +
        (hosted
          ? (
            '<section class="portal-content-panel portal-content-panel-workspaces">' +
              workspaceSummaryContent +
              workspacesContent +
            '</section>' +
            '<section class="portal-content-panel portal-content-panel-access">' +
              accessContent +
            '</section>'
          )
          : '') +
        '<section class="portal-content-panel portal-content-panel-billing billing-section" id="billing-section">' +
          (hosted ? renderHostedBillingCards(accounts, showSelfHostedCommercial) : '') +
          (showSelfHostedBillingShell
            ? ('<div class="billing-shell billing-shell-idle">' +
            '<div class="billing-shell-main">' +
              '<div class="billing-shell-main-head">' +
                '<h3>Self-hosted billing</h3>' +
                '<p>' + escapeHTML(selfHostedBillingLeadCopy) + '</p>' +
              '</div>' +
              '<div class="billing-action-list">' + selfHostedBillingActionsHTML + '</div>' +
              '<div class="billing-inline-support">' +
                '<h4>Support</h4>' +
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
            '<div class="billing-shell-detail" id="billing-detail-shell" hidden>' + selfHostedBillingPanelsHTML + '</div>' +
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
  var signupHTML = hasSignupPath(context.signupPath)
    ? '<p class="portal-auth-secondary-action">Need a new Pulse Account? <a href="' + escapeAttr(context.signupPath) + '">Create an account</a>.</p>'
    : '';
  return (
    '<section class="portal-auth-shell">' +
      '<div class="portal-auth-intro">' +
        '<h1>Sign in to Pulse Account</h1>' +
        '<p>Use one commercial email address for hosted workspaces, account access, billing, licenses, refunds, and privacy requests.</p>' +
        '<div class="portal-auth-scope-list" aria-label="Pulse Account scope">' +
          renderAuthScopeRow('Workspaces', 'Open hosted workspaces and review workspace state.') +
          renderAuthScopeRow('Access', 'Review account access and manage roles when permitted.') +
          renderAuthScopeRow('Billing', 'Open hosted billing or self-hosted commercial tools when they apply.') +
        '</div>' +
      '</div>' +
      '<section class="portal-auth-panel" aria-labelledby="portal-auth-title">' +
        '<div class="portal-auth-card">' +
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
          signupHTML +
          statusHTML +
        '</div>' +
      '</section>' +
    '</section>'
  );
}
