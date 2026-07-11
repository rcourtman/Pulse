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
  workspaceActiveAlertLabel,
  workspaceActiveAlertsUpdatedLabel,
  workspaceActiveAlertState,
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

function accountUsesClientLanguage(account: PortalAccountSummary): boolean {
  return account.kind === 'msp';
}

function accountsUseClientLanguage(accounts: PortalAccountSummary[]): boolean {
  return accounts.length > 0 && accounts.every(accountUsesClientLanguage);
}

function workspaceEntityName(clientLanguage: boolean, plural = false): string {
  if (clientLanguage) return plural ? 'clients' : 'client';
  return plural ? 'workspaces' : 'workspace';
}

function workspaceCountLabel(count: number, clientLanguage = false): string {
  return count === 1
    ? '1 ' + workspaceEntityName(clientLanguage)
    : String(count) + ' ' + workspaceEntityName(clientLanguage, true);
}

function accountCountLabel(count: number): string {
  return count === 1 ? '1 account' : String(count) + ' accounts';
}



function reviewWorkspaceHeadline(count: number, clientLanguage = false): string {
  return count === 1
    ? '1 ' + workspaceEntityName(clientLanguage) + ' needs review'
    : String(count) + ' ' + workspaceEntityName(clientLanguage, true) + ' need review';
}

function readyWorkspaceHeadline(count: number, clientLanguage = false): string {
  return count === 1
    ? '1 ' + workspaceEntityName(clientLanguage) + ' is ready to use'
    : String(count) + ' ' + workspaceEntityName(clientLanguage, true) + ' are ready to use';
}



function reviewWorkspaceChipLabel(count: number, clientLanguage = false): string {
  return count === 1
    ? '1 ' + workspaceEntityName(clientLanguage) + ' to review'
    : String(count) + ' ' + workspaceEntityName(clientLanguage, true) + ' to review';
}

function readyWorkspaceChipLabel(count: number, clientLanguage = false): string {
  return count === 1
    ? '1 ready ' + workspaceEntityName(clientLanguage)
    : String(count) + ' ready ' + workspaceEntityName(clientLanguage, true);
}

function suspendedWorkspaceChipLabel(count: number, clientLanguage = false): string {
  return count === 1
    ? '1 suspended ' + workspaceEntityName(clientLanguage)
    : String(count) + ' suspended ' + workspaceEntityName(clientLanguage, true);
}

function setupNeededWorkspaceChipLabel(count: number, clientLanguage = false): string {
  return count === 1
    ? '1 ' + workspaceEntityName(clientLanguage) + ' in setup'
    : String(count) + ' ' + workspaceEntityName(clientLanguage, true) + ' in setup';
}

function criticalWorkspaceChipLabel(count: number, clientLanguage = false): string {
  return count === 1
    ? '1 ' + workspaceEntityName(clientLanguage) + ' with critical alerts'
    : String(count) + ' ' + workspaceEntityName(clientLanguage, true) + ' with critical alerts';
}



function supportRunbookPathCopy(hasHostedAccounts: boolean, hostedViewOnly: boolean, showSelfHostedCommercial: boolean, hasHostedBilling: boolean, clientLanguage = false): string {
  var primarySection = clientLanguage ? 'Clients' : 'Workspaces';
  if (!hasHostedAccounts) return 'Billing, licenses, refunds, or privacy.';
  if (hostedViewOnly) {
    if (showSelfHostedCommercial && hasHostedBilling) return primarySection + ', Access review, owner/admin handoff, hosted billing, licenses, refunds, or privacy.';
    if (showSelfHostedCommercial) return primarySection + ', Access review, owner/admin handoff, licenses, refunds, or privacy.';
    if (hasHostedBilling) return primarySection + ', Access review, owner/admin handoff, or hosted billing.';
    return primarySection + ', Access review, or owner/admin handoff.';
  }
  if (showSelfHostedCommercial && hasHostedBilling) return primarySection + ', Access, hosted billing, licenses, refunds, or privacy.';
  if (showSelfHostedCommercial) return primarySection + ', Access, licenses, refunds, or privacy.';
  if (hasHostedBilling) return primarySection + ', Access, or hosted billing.';
  return primarySection + ' or Access.';
}

function hasHostedAccounts(accounts: PortalAccountSummary[]): boolean {
  return accounts.length > 0;
}

function hasHostedBillingAccounts(accounts: PortalAccountSummary[]): boolean {
  return accounts.some(function(account) {
    return account.has_billing === true;
  });
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

function alertBadgeHTML(workspace: PortalWorkspaceSummary): string {
  var state = workspaceActiveAlertState(workspace);
  var label = state === 'unknown' ? '-' : workspaceActiveAlertLabel(workspace);
  var title = workspaceActiveAlertLabel(workspace);
  if (state !== 'unknown') {
    title += ' · ' + workspaceActiveAlertsUpdatedLabel(workspace);
  }
  return '<span class="badge badge-alert-' + escapeAttr(state) + '" title="' + escapeAttr(title) + '">' + escapeHTML(label) + '</span>';
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
    if (workspaceActiveAlertState(workspaces[i]) === 'critical' || status === 'unhealthy' || status === 'checking') {
      results.push(workspaces[i]);
    }
  }
  return results;
}

function hasCriticalAlerts(workspace: PortalWorkspaceSummary): boolean {
  return workspaceActiveAlertState(workspace) === 'critical';
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
  var hasHostedBilling = hasHostedBillingAccounts(accounts);

  var sections: ShellNavEntry[] = [];
  if (hosted) {
    sections.push({ section: 'workspaces', title: accountsUseClientLanguage(accounts) ? 'Clients' : 'Workspaces' });
    sections.push({ section: 'access', title: 'Access' });
  }
  if (hasHostedBilling || showSelfHostedCommercial) {
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
  var clientLanguage = accountUsesClientLanguage(account);
  var createdLabel = formatWorkspaceDate(workspace.created_at);
  var metaParts = [];
  if (state) {
    metaParts.push('<span class="workspace-meta-item">' + escapeHTML(titleCase(state)) + '</span>');
  }
  if (createdLabel) {
    metaParts.push('<span class="workspace-meta-item">Created ' + escapeHTML(createdLabel) + '</span>');
  }

  var openAction = '';
  if (state === 'active') {
    openAction =
      '<form method="POST" action="' +
      escapeAttr(workspaceHandoffActionPath(accountAPIBasePath, account.id, workspace.id)) +
      '">' +
      '<button type="submit" class="btn-primary">' + escapeHTML(clientLanguage ? 'Open client' : 'Open workspace') + '</button>' +
      '</form>';
  }

  var installAction = '';
  if (account.can_manage && state === 'active') {
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
      '">' + escapeHTML(clientLanguage ? 'Client onboarding' : 'Setup checklist') + '</button>';
  }

  return (
    '<article class="workspace-row workspace-row-health-' + escapeAttr(status) + ' workspace-row-state-' + escapeAttr(state || 'unknown') + '" id="' + escapeAttr(workspaceRowAnchorID(account.id, workspace.id)) + '" data-workspace-row="' + escapeAttr(workspace.id) + '">' +
      '<div class="workspace-row-primary">' +
        '<div class="workspace-row-heading">' +
          '<h4 class="workspace-name">' + escapeHTML(workspace.display_name) + '</h4>' +
        '</div>' +
        (metaParts.length ? '<div class="workspace-meta">' + metaParts.join('') + '</div>' : '') +
      '</div>' +
      '<div class="workspace-row-status-cell workspace-row-status-cell-badge">' +
        setupBadgeHTML(workspace) +
      '</div>' +
      '<div class="workspace-row-status-cell workspace-row-status-cell-badge">' +
        alertBadgeHTML(workspace) +
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
    if (hasCriticalAlerts(entries[i].workspace) || status === 'unhealthy' || status === 'checking') {
      results.push(entries[i]);
    }
  }
  return results;
}

function criticalAlertWorkspaceEntries(entries: WorkspaceSummaryEntry[]): WorkspaceSummaryEntry[] {
  var results: WorkspaceSummaryEntry[] = [];
  for (var i = 0; i < entries.length; i += 1) {
    if (hasCriticalAlerts(entries[i].workspace)) {
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

function workspaceSummaryStatusCopy(workspace: PortalWorkspaceSummary, clientLanguage: boolean): string {
  if (!clientLanguage) return workspaceStatusCopy(workspace);
  var state = String(workspace.state || '');
  if (state === 'suspended') return 'This client is suspended.';
  if (state === 'failed') return 'This client is in a failed state.';
  return workspaceStatusCopy(workspace);
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
  var clientLanguage = accountsUseClientLanguage(accounts);
  var critical = criticalAlertWorkspaceEntries(entries);
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

  if (critical.length) {
    var criticalEntry = critical[0];
    title = 'Review ' + criticalEntry.workspace.display_name;
    description = workspaceSummaryContext(criticalEntry, accounts.length > 1, workspaceActiveAlertLabel(criticalEntry.workspace));
    primaryAction = renderWorkspaceAnchorAction(
      workspaceRowAnchorID(criticalEntry.account.id, criticalEntry.workspace.id),
      clientLanguage ? 'Review client' : 'Review workspace',
      'btn-primary btn-compact workspace-summary-link',
    );
    secondaryAction = renderWorkspaceHandoffForm(
      criticalEntry.account.id,
      criticalEntry.workspace.id,
      accountAPIBasePath,
      clientLanguage ? 'Open client' : 'Open workspace',
    );
  } else if (attention.length) {
    var attentionEntry = attention[0];
    title = 'Review ' + attentionEntry.workspace.display_name;
    description = workspaceSummaryContext(attentionEntry, accounts.length > 1, workspaceSummaryStatusCopy(attentionEntry.workspace, clientLanguage));
    primaryAction = renderWorkspaceAnchorAction(
      workspaceRowAnchorID(attentionEntry.account.id, attentionEntry.workspace.id),
      clientLanguage ? 'Review client' : 'Review workspace',
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
        '">' + escapeHTML(clientLanguage ? 'Client onboarding' : 'Setup checklist') + '</button>'
      : '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
  } else if (suspended.length) {
    var suspendedEntry = suspended[0];
    title = 'Review ' + suspendedEntry.workspace.display_name;
    description = workspaceSummaryContext(suspendedEntry, accounts.length > 1, workspaceSummaryStatusCopy(suspendedEntry.workspace, clientLanguage));
    primaryAction = renderWorkspaceAnchorAction(
      workspaceRowAnchorID(suspendedEntry.account.id, suspendedEntry.workspace.id),
      clientLanguage ? 'Review client' : 'Review workspace',
      'btn-primary btn-compact workspace-summary-link',
    );
    secondaryAction = suspendedEntry.account.can_manage
      ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' +
        escapeAttr(suspendedEntry.account.id) +
        '" data-workspace-id="' +
        escapeAttr(suspendedEntry.workspace.id) +
        '">' + escapeHTML(clientLanguage ? 'Client onboarding' : 'Setup checklist') + '</button>'
      : '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
  } else if (setupNeeded.length) {
    var setupEntry = setupNeeded[0];
    var setupState = workspaceSetupState(setupEntry.workspace);
    title = setupState === 'configure_outputs'
      ? 'Configure outputs for ' + setupEntry.workspace.display_name
      : 'Set up ' + setupEntry.workspace.display_name;
    description = workspaceSummaryContext(setupEntry, accounts.length > 1, workspaceSetupNextStep(setupEntry.workspace));
    if (!setupEntry.account.can_manage) {
      primaryAction = renderWorkspaceHandoffForm(setupEntry.account.id, setupEntry.workspace.id, accountAPIBasePath, clientLanguage ? 'Open client' : 'Open workspace', 'btn-primary btn-compact');
    } else if (setupState === 'configure_outputs') {
      primaryAction = renderWorkspaceReportingHandoffForm(
        setupEntry.account.id,
        setupEntry.workspace.id,
        accountAPIBasePath,
        'Open reports',
        'btn-primary btn-compact',
      );
    } else {
      primaryAction = renderWorkspaceInstallHandoffForm(
        setupEntry.account.id,
        setupEntry.workspace.id,
        accountAPIBasePath,
        'Install agents',
        'btn-primary btn-compact',
      );
    }
    secondaryAction = setupEntry.account.can_manage
      ? '<button type="button" class="btn-secondary btn-compact" data-action="select-workspace" data-account-id="' +
        escapeAttr(setupEntry.account.id) +
        '" data-workspace-id="' +
        escapeAttr(setupEntry.workspace.id) +
        '">' + escapeHTML(clientLanguage ? 'Client onboarding' : 'Setup checklist') + '</button>'
      : renderWorkspaceHandoffForm(setupEntry.account.id, setupEntry.workspace.id, accountAPIBasePath, clientLanguage ? 'Open client' : 'Open workspace');
  } else if (ready.length) {
    var readyEntry = ready[0];
    title = 'Open ' + readyEntry.workspace.display_name;
    description = workspaceSummaryContext(readyEntry, accounts.length > 1, workspaceRowNote(readyEntry.workspace));
    primaryAction = renderWorkspaceHandoffForm(
      readyEntry.account.id,
      readyEntry.workspace.id,
      accountAPIBasePath,
      clientLanguage ? 'Open client' : 'Open workspace',
      'btn-primary btn-compact',
    );
    secondaryAction = ready.length > 1
      ? renderWorkspaceAnchorAction(workspaceListAnchorID(readyEntry.account.id), clientLanguage ? 'See all clients' : 'See all workspaces')
      : readyEntry.account.can_manage
        ? renderWorkspaceInstallHandoffForm(readyEntry.account.id, readyEntry.workspace.id, accountAPIBasePath)
        : '';
  } else if (creatableAccount) {
    title = clientLanguage ? 'Add the first client' : 'Create the first workspace';
    description = clientLanguage
      ? 'No client is attached yet. Add the first client in ' + creatableAccount.name + '.'
      : 'No hosted workspace is attached yet. Create the first workspace in ' + creatableAccount.name + '.';
    primaryAction = '<button class="btn-primary btn-compact" type="button" data-action="toggle-add-workspace" data-account-id="' + escapeAttr(creatableAccount.id) + '">' + escapeHTML(clientLanguage ? 'Add client' : 'Create workspace') + '</button>';
    secondaryAction = accessAccount
      ? '<button class="btn-secondary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>'
      : '';
  } else if (hostedViewOnly) {
    if (entries.length > 0) {
      title = 'Review who can act';
      description = clientLanguage
        ? 'Clients are attached here, but an owner or admin must make account-level changes.'
        : 'Hosted workspaces are attached here, but an owner or admin must make account-level changes.';
      primaryAction = '<button class="btn-primary btn-compact" type="button" data-shell-action="activate-section" data-shell-section="access">Open Access</button>';
      secondaryAction = renderWorkspaceAnchorAction(workspaceListAnchorID(accounts[0].id), clientLanguage ? 'Review client list' : 'Review workspace list');
    } else {
      title = 'Review who can act';
      description = clientLanguage
        ? 'No client is attached yet. Review Access to see who can add or manage clients on this account.'
        : showSelfHostedCommercial
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
      ? 'Review the ' + workspaceEntityName(clientLanguage) + ' list here, then use Billing for commercial work or Support only after a self-service path fails.'
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
  var clientLanguage = accountsUseClientLanguage(accounts);
  return [
    accountCountLabel(accounts.length),
    workspaceCountLabel(entries.length, clientLanguage),
    readyWorkspaceChipLabel(readyWorkspaceEntries(entries).length, clientLanguage),
    setupNeededWorkspaceChipLabel(setupNeededWorkspaceEntries(entries).length, clientLanguage),
    criticalWorkspaceChipLabel(criticalAlertWorkspaceEntries(entries).length, clientLanguage),
    reviewWorkspaceChipLabel(attentionWorkspaceEntries(entries).length, clientLanguage),
    suspendedWorkspaceChipLabel(suspendedWorkspaceEntries(entries).length, clientLanguage),
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
      '<details class="workspace-template-details">' +
        '<summary class="workspace-template-summary">' +
          '<span class="workspace-template-summary-title">' + escapeHTML(template.title || 'Provider setup template') + '</span>' +
          '<span class="workspace-template-summary-meta">' + escapeHTML(templateAccount.name) + '</span>' +
        '</summary>' +
        '<p class="workspace-template-intro">Use the same onboarding shape for each client, then finish the workspace-owned configuration inside that isolated client boundary.</p>' +
        '<div class="workspace-template-grid">' +
          '<div><strong>Agent naming</strong><span>' + escapeHTML(template.agent_naming) + '</span></div>' +
          '<div><strong>Alert routing</strong><span>' + escapeHTML(template.alert_routing) + '</span></div>' +
          '<div><strong>Reports</strong><span>' + escapeHTML(template.reporting) + '</span></div>' +
          '<div><strong>Access</strong><span>' + escapeHTML(template.access) + '</span></div>' +
        '</div>' +
      '</details>' +
    '</section>'
  );
}

function renderWorkspaceSetupQueue(entries: WorkspaceSummaryEntry[], accountAPIBasePath: string, clientLanguage = false): string {
  var setupNeeded = setupNeededWorkspaceEntries(entries);
  if (!setupNeeded.length) return '';
  var visible = setupNeeded.slice(0, 5);
  return (
    '<section class="workspace-setup-queue" aria-label="' + escapeAttr(clientLanguage ? 'Client onboarding queue' : 'Unfinished workspace setup') + '">' +
      '<div class="workspace-setup-queue-header">' +
        '<div>' +
          '<h3>' + escapeHTML(clientLanguage ? 'Clients in setup' : 'Unfinished setup') + '</h3>' +
          '<p>' + escapeHTML(clientLanguage ? 'Clients stay here until agents, alert routing, and reports are in place.' : 'Client workspaces stay here until agents, alert routing, and reports are in place.') + '</p>' +
        '</div>' +
        '<span>' + escapeHTML(setupNeededWorkspaceChipLabel(setupNeeded.length, clientLanguage)) + '</span>' +
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
                    '">' + escapeHTML(clientLanguage ? 'Client onboarding' : 'Setup checklist') + '</button>'
                  : renderWorkspaceHandoffForm(entry.account.id, entry.workspace.id, accountAPIBasePath, clientLanguage ? 'Open client' : 'Open workspace')) +
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
  var clientLanguage = accountsUseClientLanguage(accounts);
  if (!entries.length) {
    if (clientLanguage) {
      return canManageAnyWorkspace
        ? 'Add clients here, then use onboarding to finish agents, alert routing, and reports.'
        : 'Review client state here. An owner or admin must add or change clients.';
    }
    return canManageAnyWorkspace
      ? 'Review hosted workspaces here, then create the next workspace when you are ready.'
      : 'Review hosted workspace state here. An owner or admin must create or change hosted workspaces.';
  }
  if (!canManageAnyWorkspace) {
    return clientLanguage
      ? 'Review client health here and open ready clients. An owner or admin must handle onboarding and client changes.'
      : 'Review hosted workspace health here and open ready workspaces. An owner or admin must handle setup and workspace changes.';
  }
  return clientLanguage
    ? 'Review clients here, use Client onboarding for setup, and keep destructive client actions separate from daily monitoring. Each client remains an isolated workspace.'
    : 'Review client workspaces here, use the setup checklist for onboarding, and keep destructive workspace actions separate from daily workspace work.';
}

export function renderWorkspaceSummarySection(context: ShellViewContext): string {
  var accounts = Array.isArray(context.bootstrap.accounts) ? context.bootstrap.accounts : [];
  var entries = collectWorkspaceSummaryEntries(accounts);
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
  var clientLanguage = accountsUseClientLanguage(accounts);

  return (
    '<section class="workspace-summary-shell">' +
      '<div class="portal-page-header">' +
        '<h2>' + escapeHTML(clientLanguage ? 'Clients' : 'Workspaces') + '</h2>' +
        '<p>' + escapeHTML(workspaceSectionHeaderCopy(accounts, entries)) + '</p>' +
      '</div>' +
      renderFactLine('workspace-summary-facts', renderWorkspaceSummaryFacts(accounts, entries)) +
      renderWorkspaceSummaryInline(accounts, entries, context.accountAPIBasePath, showSelfHostedCommercial) +
      renderProviderSetupTemplates(accounts) +
      renderWorkspaceSetupQueue(entries, context.accountAPIBasePath, clientLanguage) +
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

function renderAccountSurfaceHeader(account: PortalAccountSummary, showHeader: boolean): string {
  if (!showHeader) return '';
  return (
    '<div class="portal-section-header account-surface-header">' +
      '<div>' +
        '<h3>' + escapeHTML(account.name) + '</h3>' +
        '<p class="portal-section-copy">' + escapeHTML(accountKindLabel(account) + ' · ' + portalRoleLabel(account.role)) + '</p>' +
      '</div>' +
    '</div>'
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
  var clientLanguage = accountUsesClientLanguage(account);
  var addEntityLabel = clientLanguage ? 'Add client' : 'Create workspace';
  var openEntityLabel = clientLanguage ? 'Open client' : 'Open workspace';
  var onboardingLabel = clientLanguage ? 'Client onboarding' : 'Setup checklist';
  var readyCount = countReadyWorkspaces(workspaces);
  var attentionCount = attentionWorkspaces(workspaces).length;
  var suspendedCount = countWorkspacesByState(workspaces, 'suspended');
  var workspaceListSummary = account.can_manage
    ? (clientLanguage ? 'Open a client to work inside its isolated workspace boundary, or use Client onboarding to finish setup.' : 'Open a workspace to work in it, or use Setup checklist when onboarding a client.')
    : 'Open a workspace here. An owner or admin must create or change hosted workspaces.';
  var workspaceManagementEmptyNote = account.has_billing
    ? 'Access changes stay in Access. Billing changes stay in Billing.'
    : (clientLanguage
      ? 'Access changes stay in Access. Client runtime changes stay inside the client workspace.'
      : 'Access changes stay in Access. Workspace runtime changes stay inside the workspace.');
  var workspaceManagement = '';
  var addWorkspaceForm = '';
  var workspaceHeaderActions = '';
  if (account.can_manage) {
    if (account.kind === 'msp') {
      workspaceHeaderActions +=
        '<button type="button" class="btn-secondary btn-compact" data-action="toggle-add-workspace" data-account-id="' +
        escapeAttr(account.id) +
        '">' + escapeHTML(addEntityLabel) + '</button>';
    }

    if (account.kind === 'msp') {
      addWorkspaceForm =
        '<div class="add-workspace-form" id="add-ws-form-' +
        escapeAttr(account.id) +
        '">' +
          '<label for="ws-name-' +
          escapeAttr(account.id) +
          '">' + escapeHTML(clientLanguage ? 'Client name' : 'Workspace name (for example, a client name)') + '</label>' +
          '<input type="text" id="ws-name-' +
          escapeAttr(account.id) +
          '" placeholder="Acme Corp" maxlength="80" autocomplete="off">' +
          '<div class="form-actions">' +
            '<button type="button" class="btn-primary" data-action="create-workspace" data-account-id="' +
            escapeAttr(account.id) +
            '">' + escapeHTML(addEntityLabel) + '</button>' +
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
            '<h3>' + escapeHTML(onboardingLabel) + '</h3>' +
            '<p>' + escapeHTML(clientLanguage ? 'Finish setup while each client keeps a separate workspace boundary.' : 'Finish the client setup steps without mixing client data across workspaces.') + '</p>' +
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
                '<h4>' + escapeHTML(clientLanguage ? 'Add a client' : 'Create a workspace') + '</h4>' +
                '<p>' + escapeHTML(clientLanguage ? 'Create an isolated client workspace for a customer.' : 'Add a new hosted workspace for a customer or operating boundary.') + '</p>' +
              '</div>' +
              addWorkspaceForm +
            '</div>' +
            '<div class="workspace-management-empty-note">' + escapeHTML(workspaceManagementEmptyNote) + '</div>' +
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
          '<div class="workspace-setup-guide" aria-label="' + escapeAttr(clientLanguage ? 'Guided client onboarding' : 'Guided workspace setup') + '">' +
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
          '<div class="workspace-setup-checklist" aria-label="' + escapeAttr(clientLanguage ? 'Client onboarding checklist' : 'Workspace setup checklist') + '">' +
            '<div class="workspace-setup-step workspace-setup-step-created">' +
              '<span class="workspace-setup-status" id="workspace-management-check-created-' + escapeAttr(account.id) + '"></span>' +
              '<div><strong>' + escapeHTML(clientLanguage ? 'Client added' : 'Workspace created') + '</strong><span>' + escapeHTML(clientLanguage ? 'A separate workspace boundary keeps this client isolated.' : 'This client has a separate workspace boundary.') + '</span></div>' +
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
              '<div><strong>' + escapeHTML(openEntityLabel) + '</strong><span>' + escapeHTML(clientLanguage ? 'Work inside this client workspace boundary.' : 'Work inside this client boundary.') + '</span></div>' +
              '<form method="POST" id="workspace-management-open-form-' +
              escapeAttr(account.id) +
              '">' +
                '<button type="submit" class="btn-primary btn-compact" id="workspace-management-open-' +
                escapeAttr(account.id) +
                '">' + escapeHTML(openEntityLabel) + '</button>' +
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
            '">' + escapeHTML(clientLanguage ? 'Manage client' : 'Manage workspace') + '</button>' +
          '</div>' +
        '</div>' +
      '</section>';

  }

  var workspaceHTML = workspaces.length
    ? '<div class="workspace-list-wrap" id="' + escapeAttr(workspaceListAnchorID(account.id)) + '">' +
          (workspaceHeaderActions ? '<div class="workspace-list-toolbar">' + workspaceHeaderActions + '</div>' : '') +
          '<div class="workspace-list-head">' +
            '<span>' + escapeHTML(clientLanguage ? 'Client' : 'Workspace') + '</span>' +
            '<span>Setup</span>' +
            '<span>Alerts</span>' +
            '<span>Health</span>' +
            '<span>Actions</span>' +
          '</div>' +
          '<div class="workspace-list">' + workspaces.map(function(workspace) {
          return renderWorkspaceCard(account, workspace, accountAPIBasePath);
        }).join('') + '</div>' +
        '</div>'
    : '<div class="empty-state">' +
        '<p>' + escapeHTML(account.can_manage
          ? (clientLanguage ? 'No clients yet. Add one to get started.' : 'No hosted workspaces yet. Create one to get started.')
          : (clientLanguage ? 'No clients are attached yet. An owner or admin must add the first one.' : 'No hosted workspaces are attached yet. An owner or admin must create the first one.')
        ) + '</p>' +
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

function renderAccountAccessSection(account: PortalAccountSummary, emailSignInAvailable = true): string {
  var clientLanguage = accountUsesClientLanguage(account);
  var hasBilling = account.has_billing === true;
  var accessRoleCopy = {
    admin: hasBilling
      ? (clientLanguage ? 'Client control, billing, and roster management.' : 'Workspace control, billing, and roster management.')
      : (clientLanguage ? 'Client control and roster management.' : 'Workspace control and roster management.'),
    tech: hasBilling
      ? (clientLanguage ? 'Client control without billing or roster ownership.' : 'Workspace control without billing or roster ownership.')
      : (clientLanguage ? 'Client control without roster ownership.' : 'Workspace control without roster ownership.'),
    readOnly: clientLanguage ? 'Review client status without control-plane changes.' : 'Review access without control-plane changes.',
  };
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
        '<div class="access-policy-row"><strong>Owner</strong><span>' + escapeHTML(hasBilling ? 'Full account, billing, and access control.' : 'Full account and access control.') + '</span></div>' +
        '<div class="access-policy-row"><strong>Admin</strong><span>' + escapeHTML(accessRoleCopy.admin) + '</span></div>' +
        '<div class="access-policy-row"><strong>Tech</strong><span>' + escapeHTML(accessRoleCopy.tech) + '</span></div>' +
        '<div class="access-policy-row"><strong>Read-only</strong><span>' + escapeHTML(accessRoleCopy.readOnly) + '</span></div>' +
      '</div>' +
    '</div>';
  var inviteDeliveryNote = emailSignInAvailable
    ? ''
    : '<p class="access-invite-delivery-note">No email provider is configured, so invitation emails are not sent. After inviting, print their one-time sign-in link on the control plane host: <code>docker compose run --rm control-plane provider-msp portal-link --email their@address</code></p>';
  var accessInvitePanel = account.can_manage
    ? (
      '<div class="access-invite-panel">' +
        '<div class="access-panel-heading">' +
          '<h4>Invite people</h4>' +
          '<p>Add one person with the minimum role they need on this account.</p>' +
          inviteDeliveryNote +
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
          '"><option value="read_only">Read-only</option><option value="tech">Tech</option><option value="admin">Admin</option></select></div>' +
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
      '" data-client-language="' +
      escapeAttr(clientLanguage ? 'true' : 'false') +
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
  var clientLanguage = accountsUseClientLanguage(accounts);
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
                '<div class="billing-action-meta">' + escapeHTML(clientLanguage ? 'Keep client onboarding in Clients and roster changes in Access.' : 'Keep workspace changes in Workspaces and roster changes in Access.') + '</div>' +
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
  var clientLanguage = accountsUseClientLanguage(accounts);
  var primarySectionLabel = clientLanguage ? 'Clients' : 'Workspaces';
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
  var hasBillingSection = hasHostedBillingAccounts(accounts) || showSelfHostedCommercial;
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
        ? (
          hasBillingSection
            ? 'Review ' + primarySectionLabel + ' or Access first. If billing is involved, hand it to an owner or admin before you escalate.'
            : 'Review ' + primarySectionLabel + ' or Access first. If account ownership is involved, hand it to an owner or admin before you escalate.'
        )
        : 'Retry the same ' + primarySectionLabel + (hasBillingSection ? ', Access, or Billing' : ' or Access') + ' step before you escalate.'
    )
    : 'Retry the same Billing step before you escalate.';
  var supportActions = isHosted
    ? (
      '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="workspaces">' + escapeHTML(primarySectionLabel) + '</button>' +
      '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="access">Access</button>' +
      (hasBillingSection ? '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Billing</button>' : '')
    )
    : '<button type="button" class="btn-secondary btn-compact" data-shell-action="activate-section" data-shell-section="billing">Billing</button>';
  var providerOpsRow = context.bootstrap.provider_hosted_mode === true
    ? '<div class="portal-support-simple-row"><strong>Operations guide</strong><span>Backups, upgrades, firewall baseline, and the validation checklist live in <a href="https://github.com/rcourtman/Pulse" target="_blank" rel="noopener">docs/MSP.md</a> in the Pulse repository.</span></div>'
    : '';
  return (
    '<section class="portal-support-panel">' +
      '<p>Most issues clear on a retry of the same step. If it fails twice, email the details below and we will pick it up from there.</p>' +
      '<div class="portal-support-simple">' +
        '<div class="portal-support-simple-card">' +
          '<div class="portal-support-simple-list">' +
            '<div class="portal-support-simple-row"><strong>Try first</strong><span>' + escapeHTML(retryCopy) + '</span></div>' +
            '<div class="portal-support-simple-row"><strong>Scope</strong><span>' + escapeHTML(supportRunbookPathCopy(isHosted, hostedViewOnly, showSelfHostedCommercial, hasHostedBillingAccounts(accounts), clientLanguage)) + '</span></div>' +
            '<div class="portal-support-simple-row"><strong>Include</strong><span>Account, email, and the exact action that failed.</span></div>' +
            providerOpsRow +
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
  var hasHostedBilling = hasHostedBillingAccounts(accounts);
  var showSelfHostedCommercial = hasSelfHostedCommercial(context.bootstrap);
  var showSelfHostedUpgradeHandoff =
    context.billingState.openBillingPanelID === 'upgrade-billing-panel' ||
    !!normalizeUpgradeFeatureKey(context.billingState.upgradeFeatureKey) ||
    !!String(context.billingState.upgradePortalHandoffID || '').trim();
  var showSelfHostedBillingShell = showSelfHostedCommercial || showSelfHostedUpgradeHandoff;
  var showBillingPanel = hasHostedBilling || showSelfHostedBillingShell;
  var shellSections = visibleShellSections(context.bootstrap);
  var preferredSection = context.activeSection || preferredPortalShellSection(context.bootstrap);
  var activeSection = shellSections.some(function(entry) {
    return entry.section === preferredSection;
  })
    ? preferredSection
    : (shellSections[0] ? shellSections[0].section : 'billing');
  var selfHostedBillingEscalationCopy = hosted
    ? 'Escalate with the same hosted billing action or self-hosted path and the exact failed step.'
    : 'Escalate with the same self-hosted billing path and the exact failed step.';
  var workspacesContent = accounts.length
    ? accounts.map(function(account) {
      return (
        '<section class="account-surface">' +
          renderAccountSurfaceHeader(account, accounts.length > 1) +
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
          renderAccountSurfaceHeader(account, accounts.length > 1) +
          renderAccountAccessSection(account, context.bootstrap.email_sign_in_available !== false) +
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
        (showBillingPanel
          ? '<section class="portal-content-panel portal-content-panel-billing billing-section" id="billing-section">' +
          (hosted && hasHostedBilling ? renderHostedBillingCards(accounts, showSelfHostedCommercial) : '') +
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
        '</section>'
          : '') +
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
  var providerMode = context.bootstrap.provider_hosted_mode === true;
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
  var introHTML = providerMode
    ? '<h1>Sign in to Pulse Account</h1>' +
      '<p>This portal manages the client workspaces on your Pulse control plane.</p>' +
      '<div class="portal-auth-scope-list" aria-label="Pulse Account scope">' +
        renderAuthScopeRow('Clients', 'Add clients, follow onboarding, and review client health and alerts.') +
        renderAuthScopeRow('Access', 'Invite provider staff and keep every person on the smallest useful role.') +
        renderAuthScopeRow('Isolation', 'Each client runs in its own workspace boundary; opening a client hands you into that boundary.') +
      '</div>'
    : '<h1>Sign in to Pulse Account</h1>' +
      '<p>Use one commercial email address for hosted workspaces, account access, billing, licenses, refunds, and privacy requests.</p>' +
      '<div class="portal-auth-scope-list" aria-label="Pulse Account scope">' +
        renderAuthScopeRow('Workspaces', 'Open hosted workspaces and review workspace state.') +
        renderAuthScopeRow('Access', 'Review account access and manage roles when permitted.') +
        renderAuthScopeRow('Billing', 'Open hosted billing or self-hosted commercial tools when they apply.') +
      '</div>';
  var cardHTML;
  if (context.bootstrap.email_sign_in_available === false) {
    // No transactional email provider is configured, so a "we sent you a
    // link" form would be a false promise. Point at the operator command
    // that actually produces a sign-in link.
    cardHTML =
      '<h2 id="portal-auth-title">Sign-in links come from your control plane host</h2>' +
      '<p>This control plane has no email provider configured, so it cannot send sign-in links. Generate a one-time link on the host where the control plane runs, then open it in this browser:</p>' +
      '<pre class="portal-auth-command"><code>docker compose run --rm control-plane \\\n  provider-msp portal-link --email you@example.com</code></pre>' +
      '<p>Run it from your deploy directory. The email address must already be an account member or hold a pending invitation.</p>' +
      '<p>To enable email sign-in, set <code>RESEND_API_KEY</code> in the control plane <code>.env</code> and restart it.</p>';
  } else {
    var emailLabel = providerMode ? 'Email' : 'Commercial email';
    var emailIntro = providerMode
      ? 'Enter the email address for your Pulse account. A sign-in link will be sent to that address.'
      : 'Enter the commercial email address for your Pulse account. A sign-in link will be sent to that address.';
    cardHTML =
      '<h2 id="portal-auth-title">Email sign-in link</h2>' +
      '<p>' + escapeHTML(emailIntro) + '</p>' +
      '<div class="form-group portal-auth-form-group">' +
        '<label for="portal-login-email">' + escapeHTML(emailLabel) + '</label>' +
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
      statusHTML;
  }
  return (
    '<section class="portal-auth-shell">' +
      '<div class="portal-auth-intro">' +
        introHTML +
      '</div>' +
      '<section class="portal-auth-panel" aria-labelledby="portal-auth-title">' +
        '<div class="portal-auth-card">' +
          cardHTML +
        '</div>' +
      '</section>' +
    '</section>'
  );
}
