import { describe, expect, it } from 'vitest';
import settingsSource from '../Settings.tsx?raw';
import { EN_MESSAGES } from '@/i18n/messages';
import settingsDialogsSource from '../SettingsDialogs.tsx?raw';
import settingsPageShellSource from '../SettingsPageShell.tsx?raw';
import aiSettingsSource from '../AISettings.tsx?raw';
import aiSettingsDialogsSource from '../AISettingsDialogs.tsx?raw';
import aiSettingsStatusAndActionsSource from '../AISettingsStatusAndActions.tsx?raw';
import aiChatMaintenanceSectionSource from '../AIChatMaintenanceSection.tsx?raw';
import aiModelSelectionSectionSource from '../AIModelSelectionSection.tsx?raw';
import aiProviderConfigurationSectionSource from '../AIProviderConfigurationSection.tsx?raw';
import aiRuntimeControlsSectionSource from '../AIRuntimeControlsSection.tsx?raw';
import aiSettingsModelSource from '../aiSettingsModel.ts?raw';
import generalSettingsPanelSource from '../GeneralSettingsPanel.tsx?raw';
import dockerRuntimeSettingsCardSource from '../DockerRuntimeSettingsCard.tsx?raw';
import settingsHeaderMetaSource from '../settingsHeaderMeta.ts?raw';
import settingsNavCatalogSource from '../settingsNavCatalog.ts?raw';
import settingsNavVisibilitySource from '../settingsNavVisibility.ts?raw';
import settingsNavigationModelSource from '../settingsNavigationModel.ts?raw';
import settingsNavigationHookSource from '../useSettingsNavigation.ts?raw';
import settingsAccessSource from '../useSettingsAccess.ts?raw';
import aiSettingsStateSource from '../useAISettingsState.ts?raw';
import settingsPanelRegistryContextSource from '../settingsPanelRegistryContext.tsx?raw';
import apiAccessPanelSource from '../APIAccessPanel.tsx?raw';
import apiTokenManagerSource from '../APITokenManager.tsx?raw';
import apiTokenManagerModelSource from '../apiTokenManagerModel.ts?raw';
import apiTokenManagerStateSource from '../useAPITokenManagerState.ts?raw';
import agentIntegrationsPanelSource from '../AgentIntegrationsPanel.tsx?raw';
import agentCapabilitiesApiSource from '@/api/agentCapabilities.ts?raw';
import dataHandlingPanelSource from '../DataHandlingPanel.tsx?raw';
import auditLogPanelSource from '../AuditLogPanel.tsx?raw';
import auditWebhookPanelSource from '../AuditWebhookPanel.tsx?raw';
import reportingPanelSource from '../ReportingPanel.tsx?raw';
import rbacFeatureGateSectionSource from '../RBACFeatureGateSection.tsx?raw';
import recoverySettingsPanelSource from '../RecoverySettingsPanel.tsx?raw';
import securityAuthPanelSource from '../SecurityAuthPanel.tsx?raw';
import securityOverviewPanelSource from '../SecurityOverviewPanel.tsx?raw';
import systemLogsPanelSource from '../SystemLogsPanel.tsx?raw';
import updatesSettingsPanelSource from '../UpdatesSettingsPanel.tsx?raw';
import updateHistorySectionSource from '../UpdateHistorySection.tsx?raw';
import updateInstallGuideSource from '../UpdateInstallGuide.tsx?raw';
import agentProfilesPanelSource from '../AgentProfilesPanel.tsx?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import infrastructureInstallerSectionSource from '../InfrastructureInstallerSection.tsx?raw';
import infrastructureOperationsModelSource from '../infrastructureOperationsModel.tsx?raw';
import connectionsTableModelSource from '../connectionsTableModel.ts?raw';
import connectionsApiSource from '@/api/connections.ts?raw';
import infrastructureSourceManagerSource from '../InfrastructureSourceManager.tsx?raw';
import infrastructureSourcePickerSource from '../InfrastructureSourcePicker.tsx?raw';
import discoverySettingsFormSource from '../DiscoverySettingsForm.tsx?raw';
import availabilitySettingsPanelSource from '../AvailabilitySettingsPanel.tsx?raw';
import availabilitySettingsModelSource from '../availabilitySettingsModel.ts?raw';
import infrastructureWorkspaceModelSource from '../infrastructureWorkspaceModel.ts?raw';
import useConnectionsLedgerSource from '../useConnectionsLedger.ts?raw';
import agentProfileSettingsSource from '../agentProfileSettings.ts?raw';
import monitoredSystemImpactPreviewSource from '../MonitoredSystemImpactPreview.tsx?raw';
import infrastructureImportPlanModelSource from '../infrastructureImportPlanModel.ts?raw';
import connectionEditorSource from '../ConnectionEditor/ConnectionEditor.tsx?raw';
import addressProbeStepSource from '../ConnectionEditor/AddressProbeStep.tsx?raw';
import connectionEditorStateSource from '../ConnectionEditor/useConnectionEditor.ts?raw';
import nodeCredentialSlotSource from '../ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx?raw';
import nodeCandidateImportPlanSource from '../ConnectionEditor/CredentialSlots/NodeCandidateImportPlan.tsx?raw';
import networkBoundarySettingsSectionSource from '../NetworkBoundarySettingsSection.tsx?raw';
import nodeModalBasicInfoSectionSource from '../NodeModalBasicInfoSection.tsx?raw';
import nodeModalClusterMembersSectionSource from '../NodeModalClusterMembersSection.tsx?raw';
import nodeModalAuthenticationSectionSource from '../NodeModalAuthenticationSection.tsx?raw';
import nodeModalMonitoringSectionSource from '../NodeModalMonitoringSection.tsx?raw';
import nodeModalSetupGuideSectionSource from '../NodeModalSetupGuideSection.tsx?raw';
import nodeModalStatusFooterSource from '../NodeModalStatusFooter.tsx?raw';
import nodeModalModelSource from '../nodeModalModel.ts?raw';
import nodeModalStateSource from '../useNodeModalState.ts?raw';
import availabilityTargetSlotSource from '../ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx?raw';
import trueNASCredentialSlotSource from '../ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx?raw';
import vmwareCredentialSlotSource from '../ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx?raw';
import organizationAccessInvitationsSectionSource from '../OrganizationAccessInvitationsSection.tsx?raw';
import organizationAccessManagementSectionSource from '../OrganizationAccessManagementSection.tsx?raw';
import organizationAccessMembersSectionSource from '../OrganizationAccessMembersSection.tsx?raw';
import organizationIncomingSharesSectionSource from '../OrganizationIncomingSharesSection.tsx?raw';
import organizationOutgoingSharesSectionSource from '../OrganizationOutgoingSharesSection.tsx?raw';
import organizationOverviewDetailsSectionSource from '../OrganizationOverviewDetailsSection.tsx?raw';
import organizationSharingCreateSectionSource from '../OrganizationSharingCreateSection.tsx?raw';
import rolesEditorDialogSource from '../RolesEditorDialog.tsx?raw';
import rolesPanelSource from '../RolesPanel.tsx?raw';
import userAssignmentsPanelSource from '../UserAssignmentsPanel.tsx?raw';
import diagnosticsResultsPanelSource from '../DiagnosticsResultsPanel.tsx?raw';
import diagnosticsModelSource from '../diagnosticsModel.ts?raw';
import agentProfilesStateSource from '../useAgentProfilesPanelState.ts?raw';
import auditLogStateSource from '../useAuditLogPanelState.ts?raw';
import auditWebhookStateSource from '../useAuditWebhookPanelState.ts?raw';
import rbacFeatureGateStateSource from '../useRBACFeatureGateState.ts?raw';
import reportingStateSource from '../useReportingPanelState.ts?raw';
import ssoProvidersPanelSource from '../SSOProvidersPanel.tsx?raw';
import ssoProvidersStateSource from '../useSSOProvidersState.ts?raw';
import useInfrastructureInstallStateSource from '../useInfrastructureInstallState.tsx?raw';
import infrastructureOnboardingPresentationSource from '../../../utils/infrastructureOnboardingPresentation.ts?raw';
import selfHostedBillingPresentationSource from '../selfHostedBillingPresentation.ts?raw';
import systemSettingsPresentationSource from '../../../utils/systemSettingsPresentation.ts?raw';
import aiSettingsPresentationSource from '../../../utils/aiSettingsPresentation.ts?raw';
import auditLogPresentationSource from '../../../utils/auditLogPresentation.ts?raw';
import statusIndicatorBadgeSource from '../../shared/StatusIndicatorBadge.tsx?raw';

const settingsRuntimeSources = import.meta.glob(['../*.tsx', '../ConnectionEditor/**/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

describe('settings architecture guardrails', () => {
  it('keeps Unified Agent lifecycle failures on the shared connections contract', () => {
    expect(connectionsApiSource).toContain("| 'failed'");
    expect(connectionsApiSource).toContain('agentUpdate?: ConnectionAgentUpdateStatus;');
    expect(connectionsApiSource).toContain('agentModules?: ConnectionAgentModuleStatus[];');
    expect(connectionsTableModelSource).toContain("key: 'module-health'");
    expect(connectionsTableModelSource).toContain("label: 'Agent update failed'");
    expect(connectionsTableModelSource).toContain('update?.lastError');
  });

  it('keeps Settings on the canonical page shell boundary', () => {
    expect(settingsSource).toContain("import { SettingsDialogs } from './SettingsDialogs';");
    expect(settingsSource).toContain("import { SettingsPageShell } from './SettingsPageShell';");
    expect(settingsSource).toContain('useSettingsNavigation({');
    expect(settingsSource).toContain('<SettingsPageShell');
    expect(settingsSource).toContain('<SettingsDialogs');
    expect(settingsSource).toContain('const SettingsPanelLoadingFallback');
    expect(settingsSource).not.toContain('border-dashed border-border bg-surface-alt py-12');
    expect(settingsSource).not.toContain('<PageHeader');

    expect(settingsPageShellSource).toContain('import { PageHeader } from');
    expect(settingsPageShellSource).toContain(
      '<PageHeader title={props.headerMeta().title} description={props.headerMeta().description} />',
    );
    expect(settingsPageShellSource).toContain('max-h-[calc(100dvh-12rem)]');
    expect(settingsPageShellSource).toContain('overflow-y-auto overscroll-contain');
    expect(settingsPageShellSource).toContain('lg:min-h-[600px]');
    expect(settingsPageShellSource).not.toContain('min-h-[600px]">');
    expect(settingsDialogsSource).toContain('export const SettingsDialogs');
  });

  it('keeps infrastructure onboarding query-backed under the shared settings shell', () => {
    expect(settingsHeaderMetaSource).toContain("'infrastructure-systems': {");
    expect(settingsHeaderMetaSource).toContain(
      'Add, discover, and verify the infrastructure Pulse monitors.',
    );
    expect(settingsHeaderMetaSource).toContain("'monitoring-availability': {");
    expect(settingsHeaderMetaSource).toContain(
      'Monitor endpoint-only devices and services with ping, TCP, and HTTP probes.',
    );
    expect(settingsHeaderMetaSource).toContain("'organization-access': {");
    expect(settingsHeaderMetaSource).toContain(
      'Manage organization invitations, member roles, and ownership transfers.',
    );

    expect(settingsSource).toContain("import NotFound from '@/pages/NotFound';");
    expect(settingsSource).toContain(
      'isRouteableSettingsLocation(location.pathname, location.search)',
    );
    expect(settingsNavigationHookSource).not.toContain('deriveAddStepFromLegacyPath');
    expect(settingsNavigationHookSource).not.toContain('buildInfrastructureOnboardingPath');
    expect(settingsNavigationHookSource).toContain(
      'navigate(buildInfrastructureWorkspacePath(), {',
    );
    expect(settingsNavigationHookSource).toContain('resolveCanonicalSettingsPath(path)');

    expect(infrastructureWorkspaceModelSource).toContain(
      "const INFRASTRUCTURE_BASE_PATH = '/settings/infrastructure';",
    );
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function buildInfrastructureOnboardingPath(',
    );
    expect(infrastructureWorkspaceModelSource).not.toContain("| 'availability'");
    expect(infrastructureWorkspaceModelSource).not.toContain('deriveAddStepFromLegacyPath');
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function deriveAddStepFromSearch(',
    );
    expect(availabilitySettingsModelSource).toContain(
      "export const AVAILABILITY_SETTINGS_PATH = '/settings/monitoring/availability';",
    );
    expect(availabilitySettingsModelSource).toContain(
      'export function buildAvailabilityTargetAddPath(',
    );
  });

  it('keeps Assistant OAuth callback compatibility on the shared settings shell', () => {
    expect(settingsNavigationModelSource).toContain(
      'export const SETTINGS_PROVIDER_MODELS_PATH = `${SETTINGS_PULSE_INTELLIGENCE_PATH}/provider`;',
    );
    expect(settingsNavigationModelSource).toContain(
      "const LEGACY_SYSTEM_AI_PREFIX = '/settings/system-ai';",
    );
    expect(settingsNavigationModelSource).toContain('normalizedPath === LEGACY_SYSTEM_AI_PREFIX');
    expect(settingsNavigationModelSource).toContain(
      "case 'system-ai':\n      return PULSE_INTELLIGENCE_PROVIDER_PREFIX;",
    );
    expect(settingsNavigationModelSource).toContain(
      'export function isAISettingsOAuthCallbackQuery(search: string): boolean',
    );
    expect(settingsNavigationModelSource).toContain(
      "return params.has('ai_oauth_success') || params.has('ai_oauth_error');",
    );
    expect(settingsNavigationModelSource).toContain(
      "if (isAISettingsOAuthCallbackQuery(search)) return 'system-ai';",
    );
    expect(settingsNavigationHookSource).toContain(
      "resolvedTab === 'system-ai' && isAISettingsOAuthCallbackQuery(search)",
    );
    expect(settingsNavigationHookSource).toContain(
      'const targetHref = shouldPreserveQuery ? `${target}${search}${hash}` : target;',
    );
  });

  it('keeps retired operations paths out of the settings routing model', () => {
    expect(settingsNavigationModelSource).toContain(
      "const RETIRED_SETTINGS_OPERATIONS_PREFIX = '/settings/operations';",
    );
    expect(settingsNavigationModelSource).not.toContain('buildLegacyOperationsSettingsPath');
    expect(settingsNavigationModelSource).not.toContain('LEGACY_SETTINGS_OPERATIONS_PREFIX');
    expect(settingsNavigationModelSource).not.toContain("normalizedPath === '/operations/logs'");
    expect(settingsNavigationModelSource).not.toContain(
      "normalizedPath.startsWith('/operations/logs/')",
    );
    expect(settingsNavigationModelSource).not.toContain(
      "normalizedPath === '/operations/reporting'",
    );
    expect(settingsNavigationModelSource).not.toContain(
      "normalizedPath.startsWith('/operations/reporting/')",
    );
  });

  it('keeps allowed organization deep links on the canonical settings shell', () => {
    expect(settingsSource).toContain("import { useSettingsAccess } from './useSettingsAccess';");
    expect(settingsSource).toContain('const activeSettingsPanelEntry = createMemo(() => {');
    expect(settingsSource).toContain('if (!flatTabs().some((tab) => tab.id === currentTab)) {');
    expect(settingsSource).toContain('return settingsPanelRegistry()[currentTab];');
    expect(settingsPanelRegistryContextSource).toContain(
      'params.securityStatus()?.proxyAuthUsername',
    );
    expect(settingsPanelRegistryContextSource).toContain(
      'params.securityStatus()?.ssoSessionUsername',
    );
    expect(settingsPanelRegistryContextSource).toContain('params.securityStatus()?.authUsername;');
    expect(settingsPanelRegistryContextSource).toContain('getOrganizationOverviewPanelProps');
    expect(settingsPanelRegistryContextSource).toContain('getOrganizationAccessPanelProps');
    expect(settingsPanelRegistryContextSource).toContain('getOrganizationSharingPanelProps');
  });

  it('keeps self-hosted commercial settings plan-owned under one shared presentation model', () => {
    expect(selfHostedBillingPresentationSource).toContain("navLabel: 'Plans & Billing'");
    expect(selfHostedBillingPresentationSource).toContain("shellTitle: 'Plans & Billing'");
    expect(selfHostedBillingPresentationSource).toContain(
      'Plan, license, and Patrol mode for this instance.',
    );
    expect(selfHostedBillingPresentationSource).toContain("planSectionTitle: 'Current plan'");
    expect(selfHostedBillingPresentationSource).toContain(
      "recoverySectionTitle: 'License recovery'",
    );

    expect(settingsNavCatalogSource).toContain(
      'label: SELF_HOSTED_PRO_BILLING_PRESENTATION.navLabel',
    );
    const pulseIntelligenceNavBlock = settingsNavCatalogSource.match(
      /id: 'pulse-intelligence',[\s\S]*?id: 'organization',/,
    );
    expect(pulseIntelligenceNavBlock?.[0]).toContain("id: 'system-billing'");
    const systemBillingNavBlock = settingsNavCatalogSource.match(
      /id: 'system-billing',[\s\S]*?hideWhenCommercialHidden: true,\n\s*},/,
    );
    expect(systemBillingNavBlock?.[0]).toContain(
      'label: SELF_HOSTED_PRO_BILLING_PRESENTATION.navLabel',
    );
    expect(systemBillingNavBlock?.[0]).not.toContain('hideFromSidebar');
    const systemNavBlock = settingsNavCatalogSource.match(/id: 'system',[\s\S]*?id: 'support',/);
    expect(systemNavBlock?.[0]).not.toContain("id: 'system-billing'");
    expect(settingsNavCatalogSource).toContain("features: ['rbac']");
    expect(settingsNavCatalogSource).toContain("features: ['audit_logging']");
    expect(settingsNavCatalogSource).toContain("features: ['relay']");
    expect(settingsNavCatalogSource).toContain("features: ['advanced_reporting']");
    expect(settingsNavCatalogSource).toContain('hideWhenUnavailable: true');
    // system-relay deliberately stays visible without the relay feature so
    // Relay remains discoverable to free installs; the panel-owned upgrade
    // gate communicates the paid boundary instead of nav hiding.
    const systemRelayNavBlock = settingsNavCatalogSource.match(
      /id: 'system-relay',[\s\S]*?requiredCapability: 'relayRead',\n\s*},/,
    );
    expect(systemRelayNavBlock?.[0]).toContain("features: ['relay']");
    expect(systemRelayNavBlock?.[0]).not.toContain('hideWhenUnavailable');
    expect(settingsHeaderMetaSource).toContain(
      'title: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellTitle',
    );
    expect(settingsHeaderMetaSource).toContain(
      'description: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellDescription',
    );
  });

  it('keeps the external-agent (MCP) connector setup findable from sidebar search', () => {
    // The Assistant page hosts the pulse-mcp connector setup, but its label and
    // copy can't carry every term users search for. The nav item's search-only
    // keywords are the canonical bridge; losing them regresses "mcp"/"claude"
    // searches back to "No settings found".
    const assistantNavBlock = settingsNavCatalogSource.match(
      /id: 'system-ai-assistant',[\s\S]*?},/,
    );
    for (const keyword of ['mcp', 'external agent', 'claude', 'opencode', 'connector']) {
      expect(assistantNavBlock?.[0]).toContain(`'${keyword}'`);
    }
    expect(settingsHeaderMetaSource).toContain(
      'Configure Assistant chat behavior, chat action permissions, sessions, and external agent (MCP) connectors.',
    );
  });

  it('keeps Agent Doctor route-backed under the infrastructure workspace', () => {
    expect(settingsNavigationModelSource).toContain(
      'normalizedPath !== INFRASTRUCTURE_AGENT_DOCTOR_PATH',
    );
    expect(settingsNavigationModelSource).toContain(
      'canonicalPath === INFRASTRUCTURE_AGENT_DOCTOR_PATH',
    );
    expect(settingsNavigationModelSource).toContain('INFRASTRUCTURE_AGENT_DOCTOR_PATH,');
    expect(settingsNavigationHookSource).toContain('isLegacyAgentDoctorLocation(path, search)');
    expect(settingsNavigationHookSource).toContain(
      'buildInfrastructureAgentDoctorPath(deriveAgentDoctorScopeFromLocation(path, search))',
    );
  });

  it('keeps Pulse server updates separate from Agent Doctor lifecycle triage', () => {
    const updatesNavBlock = settingsNavCatalogSource.match(
      /id: 'system-updates',[\s\S]*?id: 'system-recovery',/,
    );
    expect(updatesNavBlock?.[0]).toContain("label: 'Pulse server updates'");
    expect(settingsHeaderMetaSource).toContain("title: 'Pulse server updates'");
    expect(settingsNavCatalogSource).not.toContain("label: 'Agent Doctor'");
  });

  it('keeps resource privacy route-backed instead of sidebar-promoted', () => {
    expect(settingsNavCatalogSource).toMatch(
      /id: 'security-data-handling',[\s\S]*label: 'Resource Privacy',[\s\S]*hideFromSidebar: true/,
    );
    expect(settingsHeaderMetaSource).toContain("title: 'Resource Privacy'");
    expect(settingsHeaderMetaSource).toContain(
      'See which monitored resource details can be summarized, must stay local, or are redacted.',
    );
    expect(dataHandlingPanelSource).toContain('title="Resource Data Policy"');
    expect(dataHandlingPanelSource).toContain('Read-only resource privacy posture');
    expect(dataHandlingPanelSource).toContain('<PolicyScopeSummary />');
    expect(dataHandlingPanelSource).toContain('<EmptyPolicyPostureState />');
  });

  it('keeps default self-hosted commercial copy opt-in from shared settings primitives', () => {
    expect(aiSettingsDialogsSource).not.toContain('Open hosted handoff');
    expect(aiSettingsDialogsSource).not.toContain(
      'Hosted quickstart routes policy-redacted prompts',
    );
    expect(aiSettingsDialogsSource).not.toContain('quickstartBlockedReason');
    expect(aiSettingsStateSource).not.toContain('hasQuickstartAvailable');
    expect(aiSettingsDialogsSource).not.toContain('Start Trial');
    expect(aiSettingsDialogsSource).not.toContain('RELAY_ONBOARDING_TRIAL_STARTING_LABEL');
    expect(generalSettingsPanelSource).not.toContain('Disable local-only commercial events');
    expect(generalSettingsPanelSource).not.toContain('commercial handoff events');
    expect(generalSettingsPanelSource).not.toContain('PULSE_DISABLE_LOCAL_UPGRADE_METRICS');
    expect(generalSettingsPanelSource).not.toContain('Disable local-only upgrade events');
    expect(systemSettingsPresentationSource).not.toContain(
      'Unable to update local-only commercial events.',
    );
    expect(systemSettingsPresentationSource).not.toContain(
      'Unable to update commercial prompt preferences.',
    );
    expect(dataHandlingPanelSource).not.toContain('Start Trial');
    expect(dataHandlingPanelSource).not.toContain('higher limits');
    expect(dataHandlingPanelSource).not.toContain('Upgrade');
    expect(auditWebhookPanelSource).toContain('getAuditWebhookFeatureGateCopy({');
    expect(auditWebhookPanelSource).toContain('paidRuntimeRequired: paidRuntimeRequired()');
    expect(auditWebhookPanelSource).not.toContain('Audit Webhooks (Pro)');
    expect(reportingPanelSource).toContain('if (!catalog || !state || showUpgradePrompts())');
    expect(reportingPanelSource).toContain('title: `${state.title} unavailable`');
    expect(reportingPanelSource).toContain(
      'Reporting is locked for this session. The report builder appears when advanced reporting is available.',
    );
    expect(reportingPanelSource).not.toContain('Advanced Reporting (Pro)');
    expect(aiRuntimeControlsSectionSource).toContain('showAutonomousControlOption');
    expect(aiRuntimeControlsSectionSource).toContain("state.form.controlLevel === 'autonomous'");
    expect(aiRuntimeControlsSectionSource).toContain(
      'Ask first - Assistant asks before chat-only actions',
    );
    expect(aiRuntimeControlsSectionSource).toContain(
      'Allow chat-only actions - Assistant may take eligible chat actions',
    );
    expect(aiRuntimeControlsSectionSource).toContain(
      'This controls actions started from Assistant chat only',
    );
    expect(aiRuntimeControlsSectionSource).toContain('Patrol handles infrastructure');
    expect(aiRuntimeControlsSectionSource).not.toContain('Command auto-run');
    expect(aiRuntimeControlsSectionSource).not.toContain('without per-command approval');
    expect(aiRuntimeControlsSectionSource).not.toContain('Legal Disclaimer');
    expect(aiRuntimeControlsSectionSource).not.toContain('Patrol uses the mode configured on');
    expect(aiRuntimeControlsSectionSource).toContain('label="Chat action mode"');
    expect(aiRuntimeControlsSectionSource).not.toContain('label="Command mode"');
    expect(aiSettingsPresentationSource).toContain('Assistant chat actions');
    expect(aiSettingsPresentationSource).not.toContain('Assistant chat command access');
    expect(aiSettingsPresentationSource).not.toContain('Assistant command access');
    expect(aiSettingsSource).toContain('AI_SETTINGS_PAGE_META');
    expect(aiSettingsSource).toContain("saveLabel: 'Save provider settings'");
    expect(aiSettingsSource).toContain("saveLabel: 'Save Patrol settings'");
    expect(aiSettingsSource).toContain("saveLabel: 'Save Assistant settings'");
    expect(aiSettingsSource).toContain("saveLabel: 'Save service context settings'");
    expect(aiSettingsStatusAndActionsSource).toContain('props.saveLabel');
    expect(aiSettingsStatusAndActionsSource).not.toContain('Save changes</button>');
    expect(aiSettingsStateSource).toContain('options.savedLabel');
    expect(aiSettingsStateSource).toContain('notificationStore.success(savedLabel)');
    expect(aiRuntimeControlsSectionSource).not.toContain('without approval (Pro)');
    expect(aiRuntimeControlsSectionSource).not.toContain(
      'Pulse Assistant executes without approval',
    );
  });

  it('keeps settings command actions on the shared button primitive', () => {
    expect(generalSettingsPanelSource).toContain(
      "import { Button } from '@/components/shared/Button';",
    );
    expect(generalSettingsPanelSource).toContain('size="settingsActionXs"');
    expect(generalSettingsPanelSource).not.toContain(
      'inline-flex items-center rounded-md border border-border bg-surface px-3 py-2 text-xs font-medium text-base-content transition hover:bg-surface-hover',
    );

    expect(reportingPanelSource).toContain("import { Button } from '@/components/shared/Button';");
    expect(reportingPanelSource).toContain('<Button');
    expect(reportingPanelSource).toContain('variant="primary"');
    expect(reportingPanelSource).toContain('variant="success"');
    expect(reportingPanelSource).toContain('isLoading={generating()}');
    expect(reportingPanelSource).toContain('isLoading={exportingInventory()}');
    expect(reportingPanelSource).not.toContain(
      'flex w-full items-center justify-center gap-2 rounded-md px-6 py-3 font-semibold transition-all sm:w-auto',
    );
    expect(reportingPanelSource).not.toContain('bg-blue-600 text-white hover:bg-blue-700');
    expect(reportingPanelSource).not.toContain('bg-emerald-600 text-white hover:bg-emerald-700');

    expect(infrastructureInstallerSectionSource).toContain(
      "import { Button, CommandCopyButton } from '@/components/shared/Button';",
    );
    expect(infrastructureInstallerSectionSource).toContain('variant="success"');
    expect(infrastructureInstallerSectionSource).toContain('variant="successOutline"');
    expect(infrastructureInstallerSectionSource).toContain('variant="successGhost"');
    expect(infrastructureInstallerSectionSource).not.toContain(
      'inline-flex items-center justify-center rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-700',
    );
    expect(infrastructureInstallerSectionSource).not.toContain(
      'inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 transition-colors hover:bg-emerald-100',
    );
    expect(infrastructureInstallerSectionSource).not.toContain(
      'inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100',
    );

    expect(ssoProvidersPanelSource).toContain(
      "import { ActionIconButton, Button, CopyValueButton } from '@/components/shared/Button';",
    );
    expect(ssoProvidersPanelSource).toContain('size="settingsAction"');
    expect(ssoProvidersPanelSource).toContain('ActionIconButton');
    expect(ssoProvidersPanelSource).toContain('CopyValueButton');
    expect(ssoProvidersPanelSource).not.toContain('<button');
    expect(ssoProvidersPanelSource).not.toContain('lucide-solid/icons/copy');
    expect(ssoProvidersPanelSource).not.toContain(
      'min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium bg-blue-600',
    );
    expect(ssoProvidersPanelSource).not.toContain('p-2 text-slate-500 hover:text-blue-600');
    expect(ssoProvidersPanelSource).not.toContain(
      'text-blue-600 hover:underline flex items-center gap-1',
    );

    expect(securityAuthPanelSource).toContain(
      "import { Button } from '@/components/shared/Button';",
    );
    expect(securityAuthPanelSource).toContain('variant="warning"');
    expect(securityAuthPanelSource).toContain('variant="primary"');
    expect(securityAuthPanelSource).toContain('size="settingsAction"');
    expect(securityAuthPanelSource).not.toContain(
      'w-full sm:w-auto px-3 py-2 text-xs font-medium rounded-md border border-amber-300',
    );
    expect(securityAuthPanelSource).not.toContain(
      'w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium bg-blue-600',
    );
    expect(securityAuthPanelSource).not.toContain(
      'w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium border border-border',
    );

    expect(rolesPanelSource).toContain(
      "import { ActionIconButton, Button } from '@/components/shared/Button';",
    );
    expect(rolesPanelSource).toContain('variant="primary"');
    expect(rolesPanelSource).toContain('ActionIconButton');
    expect(rolesPanelSource).not.toContain(
      'inline-flex w-full sm:w-auto min-h-10 sm:min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600',
    );
    expect(rolesPanelSource).not.toContain('p-1.5 rounded-md text-slate-500 hover:text-blue-600');
    expect(rolesPanelSource).not.toContain('p-1.5 rounded-md text-slate-500 hover:text-red-600');

    expect(userAssignmentsPanelSource).toContain(
      "import { Button } from '@/components/shared/Button';",
    );
    expect(userAssignmentsPanelSource).toContain('variant="ghost"');
    expect(userAssignmentsPanelSource).toContain('size="settingsAction"');
    expect(userAssignmentsPanelSource).not.toContain(
      'inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-1.5 rounded-md',
    );

    for (const organizationActionSource of [
      organizationAccessInvitationsSectionSource,
      organizationAccessManagementSectionSource,
      organizationAccessMembersSectionSource,
      organizationIncomingSharesSectionSource,
      organizationOutgoingSharesSectionSource,
      organizationOverviewDetailsSectionSource,
      organizationSharingCreateSectionSource,
    ]) {
      expect(organizationActionSource).toContain('@/components/shared/Button');
      expect(organizationActionSource).toContain('<Button');
    }
    expect(organizationAccessInvitationsSectionSource).toContain('variant="dangerOutline"');
    expect(organizationIncomingSharesSectionSource).toContain('variant="successGhost"');
    expect(organizationIncomingSharesSectionSource).toContain('variant="dangerGhost"');
    expect(organizationAccessMembersSectionSource).toContain('variant="dangerGhost"');
    expect(organizationOutgoingSharesSectionSource).toContain('variant="dangerGhost"');
    expect(organizationSharingCreateSectionSource).toContain('variant="ghost"');
    for (const source of [
      organizationAccessInvitationsSectionSource,
      organizationAccessManagementSectionSource,
      organizationAccessMembersSectionSource,
      organizationIncomingSharesSectionSource,
      organizationOutgoingSharesSectionSource,
      organizationOverviewDetailsSectionSource,
      organizationSharingCreateSectionSource,
    ]) {
      expect(source).not.toContain(
        'inline-flex items-center justify-center rounded-md bg-blue-600',
      );
      expect(source).not.toContain('hover:bg-red-50 dark:text-red-300');
      expect(source).not.toContain('hover:bg-emerald-50 dark:text-emerald-300');
    }
  });

  it('keeps settings callouts on the shared CalloutCard primitive', () => {
    for (const source of [
      aiProviderConfigurationSectionSource,
      diagnosticsResultsPanelSource,
      discoverySettingsFormSource,
      monitoredSystemImpactPreviewSource,
      reportingPanelSource,
      securityAuthPanelSource,
      securityOverviewPanelSource,
      addressProbeStepSource,
      availabilityTargetSlotSource,
      trueNASCredentialSlotSource,
      vmwareCredentialSlotSource,
    ]) {
      expect(source).toContain('CalloutCard');
    }

    expect(discoverySettingsFormSource).toContain('scale="compact"');
    expect(aiProviderConfigurationSectionSource).toContain('scale="compact"');
    expect(diagnosticsResultsPanelSource).toContain('scale="compact"');
    for (const source of [
      addressProbeStepSource,
      availabilityTargetSlotSource,
      trueNASCredentialSlotSource,
      vmwareCredentialSlotSource,
    ]) {
      expect(source).toContain('scale="compact"');
      expect(source).toContain('padding="sm"');
    }
    expect(discoverySettingsFormSource).not.toContain(
      'rounded-md border border-amber-200 bg-amber-50/80',
    );
    expect(aiProviderConfigurationSectionSource).not.toContain('rounded border border-red-200');
    expect(diagnosticsResultsPanelSource).not.toContain(
      'rounded-md border border-amber-200 bg-amber-50',
    );
    expect(addressProbeStepSource).not.toContain('rounded-md border border-red-300 bg-red-50');
    expect(addressProbeStepSource).not.toContain('rounded-md border border-amber-300 bg-amber-50');
    expect(availabilityTargetSlotSource).not.toContain('testToneClass');
    expect(availabilityTargetSlotSource).not.toContain('border-green-300 bg-green-50');
    expect(availabilityTargetSlotSource).not.toContain(
      'rounded-md border border-rose-300 bg-rose-50',
    );
    expect(trueNASCredentialSlotSource).not.toContain(
      'rounded-md border border-amber-300 bg-amber-50',
    );
    expect(trueNASCredentialSlotSource).not.toContain(
      'rounded-md border border-rose-300 bg-rose-50',
    );
    expect(vmwareCredentialSlotSource).not.toContain(
      'rounded-md border border-amber-300 bg-amber-50',
    );
    expect(vmwareCredentialSlotSource).not.toContain(
      'rounded-md border border-rose-300 bg-rose-50',
    );
  });

  it('keeps settings loading skeletons on the shared skeleton primitive', () => {
    for (const source of [dataHandlingPanelSource, securityOverviewPanelSource]) {
      expect(source).toContain('@/components/shared/SettingsLoadingSkeleton');
      expect(source).toContain('SettingsLoadingSkeleton');
      expect(source).not.toContain('animate-pulse');
    }
  });

  it('normalizes sparse security status before deriving privileged posture', () => {
    expect(securityAuthPanelSource).toContain('apiTokenConfigured?: boolean;');
    expect(securityOverviewPanelSource).toContain(
      'const postureStatus = createMemo<SecurityPostureStatus | null>',
    );
    expect(securityOverviewPanelSource).toContain(
      'apiTokenConfigured: status.apiTokenConfigured === true',
    );
    expect(securityOverviewPanelSource).toContain(
      'exportProtected: status.exportProtected === true',
    );
    expect(securityOverviewPanelSource).toContain(
      'hasAuditLogging: status.hasAuditLogging === true',
    );
    expect(securityOverviewPanelSource).toContain(
      'postureStatus() ? getSecurityHardeningActions(postureStatus()!) : []',
    );
  });

  it('keeps settings loading spinners on the shared spinner primitive', () => {
    const spinnerConsumers: Array<[string, string[]]> = [
      [
        '../AISettings.tsx',
        ['h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin'],
      ],
      [
        '../AISettingsDialogs.tsx',
        ['h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin'],
      ],
      ['../APITokenManager.tsx', ['<svg class="h-4 w-4 animate-spin"']],
      [
        '../AgentProfilesPanel.tsx',
        ['animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500'],
      ],
      ['../RolesPanel.tsx', ['animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500']],
      [
        '../SSOProvidersPanel.tsx',
        ['h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin'],
      ],
      [
        '../UpdateInstallGuide.tsx',
        ['h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent'],
      ],
      [
        '../UpdatesSettingsPanel.tsx',
        ['animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full'],
      ],
      [
        '../UserAssignmentsDialog.tsx',
        ['animate-spin rounded-full h-4 w-4 border-b-2 border-blue-500'],
      ],
      [
        '../UserAssignmentsPanel.tsx',
        ['animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500'],
      ],
    ];

    for (const [path, retiredPatterns] of spinnerConsumers) {
      const source = settingsRuntimeSources[path];
      expect(source).toContain('@/components/shared/LoadingSpinner');
      expect(source).toContain('LoadingSpinner');
      for (const retiredPattern of retiredPatterns) {
        expect(source).not.toContain(retiredPattern);
      }
    }
  });

  it('keeps embedded settings panel empty states on the shared primitive', () => {
    const panelEmptyStateConsumers = [
      agentProfilesPanelSource,
      auditWebhookPanelSource,
      auditLogPanelSource,
      availabilitySettingsPanelSource,
      diagnosticsResultsPanelSource,
      ssoProvidersPanelSource,
    ];

    for (const source of panelEmptyStateConsumers) {
      expect(source).toContain('@/components/shared/EmptyState');
      expect(source).toContain('<EmptyState');
    }

    expect(agentProfilesPanelSource.match(/variant="panel"/g) ?? []).toHaveLength(2);
    expect(auditWebhookPanelSource).toContain('variant="panel"');
    expect(auditLogPanelSource).toContain('variant="panel"');
    expect(availabilitySettingsPanelSource).toContain('variant="panel"');
    expect(diagnosticsResultsPanelSource).toContain('variant="panel"');
    expect(ssoProvidersPanelSource).toContain('variant="panel"');
    expect(agentProfilesPanelSource).not.toContain('text-center py-8 text-muted');
    expect(auditWebhookPanelSource).not.toContain(
      'py-10 flex flex-col items-center justify-center text-muted border-2 border-dashed border-border rounded-md',
    );
    expect(auditLogPanelSource).not.toContain(
      'text-center py-12 px-4 bg-surface-alt rounded-md border border-dashed border-border',
    );
    expect(availabilitySettingsPanelSource).not.toContain(
      'flex flex-col items-center justify-center gap-3 px-4 py-12 text-center',
    );
    expect(diagnosticsResultsPanelSource).not.toContain(
      'inline-flex min-h-10 items-center gap-2 rounded-md bg-blue-600',
    );
    expect(ssoProvidersPanelSource).not.toContain('text-center py-8 text-muted');
  });

  it('keeps telemetry disclosure aligned with the security privacy contract', () => {
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'aggregate self-hosted adoption',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'aggregate self-hosted adoption counts, coarse feature flags, and coarse Patrol, Assistant, and external-agent usage counters',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).not.toContain(
      'Pulse Intelligence loop adoption',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'identifiers, prompts, chat messages, command text, action output, token values, names, email addresses, or IP addresses',
    );
    expect(generalSettingsPanelSource).toContain('settings.general.telemetry.payloadAriaLabel');
    expect(generalSettingsPanelSource).toContain('settings.general.telemetry.resetId');
    expect(generalSettingsPanelSource).not.toContain('license_tier');
    expect(generalSettingsPanelSource).not.toContain('api_tokens');
  });

  it('keeps panel-owned gated settings routes separate from sidebar visibility', () => {
    expect(settingsNavVisibilitySource).toContain('const PANEL_OWNED_FEATURE_GATE_TABS');
    expect(settingsNavVisibilitySource).toContain('shouldBlockSettingsRouteItem');
    expect(settingsNavVisibilitySource).toContain('!PANEL_OWNED_FEATURE_GATE_TABS.has(tab)');
    expect(settingsAccessSource).toContain('const routeTabGroups = createMemo');
    expect(settingsAccessSource).toContain('const navTabGroups = createMemo');
    expect(settingsAccessSource).toContain('!shouldBlockSettingsRouteItem');
    expect(settingsAccessSource).toContain('!shouldHideSettingsNavItem');
  });

  it('keeps Assistant and Patrol provider diagnostics backend-owned in settings state', () => {
    expect(aiSettingsModelSource).toContain(
      'export type ProviderTestResult = AIProviderTestResult',
    );
    expect(aiSettingsModelSource).toContain(
      "{ value: 'deepseek', title: 'DeepSeek', description: 'V4' }",
    );
    expect(aiSettingsModelSource).not.toContain("description: 'V3'");
    expect(aiSettingsStateSource).toContain('getProviderTestDiagnosticMessage(result)');
    expect(aiSettingsStateSource).toContain('recommendation: result.recommendation');
    expect(aiSettingsStateSource).toContain('providerHealth[erroredCandidate].message');
    expect(aiSettingsStateSource).not.toContain('OpenRouter returned 401');
  });

  it('reveals the Assistant after AI settings saves without a page reload', () => {
    // Every successful AI settings save path must re-derive the session
    // assistantEnabled capability so the launcher and handoff buttons
    // appear (or disappear) mid-session, and first-time setup must open
    // the Assistant drawer at the moment of maximum intent.
    const refreshCalls = aiSettingsStateSource.match(/aiChatStore\.refreshEnabledFromServer\(\)/g);
    expect(refreshCalls?.length ?? 0).toBeGreaterThanOrEqual(3);
    expect(aiSettingsStateSource).toContain('aiChatStore.open()');
    expect(aiSettingsStateSource).not.toContain(
      'Pulse Intelligence enabled. You can customize settings below.',
    );
  });

  it('keeps Assistant and Patrol provider-specific fields model-driven', () => {
    expect(aiSettingsModelSource).toContain('export const AI_PROVIDERS: AIProvider[] = [');
    for (const provider of ['zai', 'groq', 'mistral', 'cerebras', 'together', 'fireworks']) {
      expect(aiSettingsModelSource).toContain(`provider: '${provider}'`);
      expect(aiSettingsStateSource).toContain(`${provider}_api_key`);
      expect(aiSettingsStateSource).toContain(`clear_${provider}_key`);
      expect(aiSettingsStateSource).toContain(`${provider}_configured`);
    }
    expect(aiSettingsModelSource).toContain('extraFields: [');
    expect(aiSettingsModelSource).toContain("inputField: 'ollamaKeepAlive'");
    expect(aiSettingsModelSource).toContain("helpContentId: 'ai.ollama.keepAlive'");
    expect(aiSettingsModelSource).toContain("inputField: 'zaiBaseUrl'");
    expect(aiProviderConfigurationSectionSource).toContain('<For each={config.extraFields || []}>');
    expect(aiProviderConfigurationSectionSource).toContain(
      'aria-label={`${getAIProviderDisplayName(config.provider)} ${extraField.label}`}',
    );
    expect(aiProviderConfigurationSectionSource).not.toContain('<Show when={config.extraField}>');
    expect(aiProviderConfigurationSectionSource).not.toContain('extraField()');
  });

  it('keeps local subscription agents explicit and credential-free in settings', () => {
    for (const provider of ['codex-subscription', 'claude-subscription']) {
      expect(aiSettingsModelSource).toContain(`provider: '${provider}'`);
    }
    expect(aiSettingsModelSource).toContain("inputType: 'toggle'");
    expect(aiSettingsModelSource).toContain("inputField: 'codexSubscriptionEnabled'");
    expect(aiSettingsModelSource).toContain("inputField: 'claudeSubscriptionEnabled'");
    expect(aiSettingsDialogsSource).toContain("props.setupProvider() === 'codex-subscription'");
    expect(aiSettingsDialogsSource).toContain("props.setupProvider() === 'claude-subscription'");
    expect(aiSettingsDialogsSource).toContain('key or OAuth token is stored in Pulse');
  });

  it('keeps the Ollama quickstart on the server-authored suggested-model projection', () => {
    // The blessed Patrol model is registry-owned backend metadata; the
    // provider row renders the projection and must not hardcode model IDs.
    expect(aiProviderConfigurationSectionSource).toContain(
      "registryDefinition()?.suggested_model ?? ''",
    );
    expect(aiProviderConfigurationSectionSource).toContain(
      'command={`ollama pull ${suggestedModel()}`}',
    );
    expect(aiProviderConfigurationSectionSource).toContain('suggested_model_equivalents');
    expect(aiProviderConfigurationSectionSource).not.toContain('qwen');
    expect(aiSettingsModelSource).not.toContain('qwen');
  });

  it('keeps Assistant session maintenance limited to Pulse-owned session actions', () => {
    expect(aiChatMaintenanceSectionSource).toContain('Summarize session');
    expect(aiChatMaintenanceSectionSource).toContain('handleSessionSummarize');
    expect(aiChatMaintenanceSectionSource).toContain(
      'Summarize a specific Pulse Assistant session',
    );

    expect(aiChatMaintenanceSectionSource).not.toContain('View session changes');
    expect(aiChatMaintenanceSectionSource).not.toContain('Revert changes');
    expect(aiChatMaintenanceSectionSource).not.toContain('handleSessionDiff');
    expect(aiChatMaintenanceSectionSource).not.toContain('handleSessionRevert');
    expect(aiSettingsStateSource).not.toContain('handleSessionDiff');
    expect(aiSettingsStateSource).not.toContain('handleSessionRevert');
    expect(aiSettingsStateSource).not.toContain('showDiffModal');
    expect(aiSettingsStateSource).not.toContain('diffFiles');
    expect(aiSettingsDialogsSource).not.toContain('Session file changes');
  });

  it('keeps the Patrol model readiness advisor wired through canonical settings state', () => {
    expect(aiSettingsStateSource).toContain('runPatrolModelReadiness');
    expect(aiSettingsStateSource).toContain('runPatrolModelReadinessAdvisor');
    expect(aiSettingsStateSource).toContain('patrolModelReadinessResult');
    expect(aiSettingsStateSource).toContain('patrolModelReadinessRunning');
    expect(aiSettingsStateSource).toContain('AbortController');
    expect(aiModelSelectionSectionSource).toContain('PatrolModelReadinessControl');
    expect(aiModelSelectionSectionSource).toContain('runPatrolModelReadinessAdvisor');
    expect(aiModelSelectionSectionSource).toContain('Autonomy suitability');
    expect(aiModelSelectionSectionSource).not.toContain("fetch('/api/ai/patrol/readiness");
  });

  it('keeps service context manual refresh on the canonical settings state', () => {
    expect(aiSettingsStateSource).toContain(
      "import { runDiscoveryRefresh } from '@/api/discovery';",
    );
    expect(aiSettingsStateSource).toContain('const handleRunDiscoveryRefresh = async () => {');
    expect(aiSettingsStateSource).toContain('const result = await runDiscoveryRefresh();');
    expect(aiSettingsStateSource).toContain('discoveryRunRunning');
    expect(aiRuntimeControlsSectionSource).toContain('state.handleRunDiscoveryRefresh()');
    expect(aiRuntimeControlsSectionSource).toContain('Run context scan');
    expect(aiRuntimeControlsSectionSource).toContain('Auto ${state.form.discoveryIntervalHours}h');
    expect(aiRuntimeControlsSectionSource).toContain('Manual only');
    expect(aiRuntimeControlsSectionSource).toContain(
      'Recurring service context scans will run at this interval.',
    );
    expect(aiRuntimeControlsSectionSource).toContain(
      'Recurring service context scans are off. Only manual refreshes will run.',
    );
    expect(aiRuntimeControlsSectionSource).toContain('Runs the same scan used by the schedule.');
    expect(aiRuntimeControlsSectionSource).toContain(
      'Manual-only mode: runs one scan without enabling recurring scans.',
    );
    expect(aiRuntimeControlsSectionSource).not.toContain("fetch('/api/discovery/run");
  });

  it('hydrates the Patrol readiness advisor from the persisted settings snapshot', () => {
    expect(aiSettingsStateSource).toContain('hydratePatrolModelReadinessFromSettings');
    expect(aiSettingsStateSource).toContain('patrol_model_readiness');
    expect(aiModelSelectionSectionSource).toContain('formatRecordedAt');
    expect(aiModelSelectionSectionSource).toContain('last evaluated');
  });

  it("passes the form's pending patrolModel to the advisor so the model check tests the unsaved selection", () => {
    // Without this, clicking Check Patrol model after changing the model
    // dropdown silently tested the previously-saved model and the
    // operator would believe their pending selection was verified.
    expect(aiSettingsStateSource).toContain('form.patrolModel');
    expect(aiSettingsStateSource).toContain('pendingModel');
    expect(aiSettingsStateSource).toContain('runPatrolModelReadiness(');
    expect(aiSettingsStateSource).toContain('pendingModel ? { model: pendingModel } : {}');
  });

  it("flags the inline preflight panel as stale when the cached result is for a different model than the form's current selection", () => {
    // Cache may hold a green result for the previously-saved model
    // while the operator has changed the dropdown. Show a warning-tone
    // panel with copy that names both models so the green badge
    // doesn't silently mislead.
    expect(aiModelSelectionSectionSource).toContain('isStaleAgainstFormSelection');
    expect(aiModelSelectionSectionSource).toContain('pendingFormModel');
    expect(aiModelSelectionSectionSource).toContain('cachedResultModel');
    expect(aiModelSelectionSectionSource).toContain('Evaluation result is for');
    expect(aiModelSelectionSectionSource).toContain(
      'Click Check Patrol model to test the pending selection',
    );
  });

  it('keeps contextual settings feature gates free of retired commercial telemetry wrappers', () => {
    for (const source of [
      agentProfilesPanelSource,
      agentProfilesStateSource,
      aiRuntimeControlsSectionSource,
      auditLogPanelSource,
      auditLogStateSource,
      auditWebhookPanelSource,
      auditWebhookStateSource,
      rbacFeatureGateSectionSource,
      rbacFeatureGateStateSource,
      reportingPanelSource,
      reportingStateSource,
      ssoProvidersPanelSource,
      ssoProvidersStateSource,
    ]) {
      expect(source).not.toContain('upgradeMetrics');
      expect(source).not.toContain('conversionEvents');
      expect(source).not.toContain('infrastructureOnboardingMetrics');
      expect(source).not.toContain('trackPaywallViewed');
      expect(source).not.toContain('trackUpgradeClicked');
      expect(source).not.toContain('/api/upgrade-metrics/events');
    }

    expect(agentProfilesPanelSource).not.toContain('Pro feature');
    expect(agentProfilesPanelSource).toContain('UpgradeButtonLink');
    expect(agentProfilesPanelSource).not.toContain('getUpgradeActionButtonClass');
    expect(auditLogPanelSource).toContain('UpgradeButtonLink');
    expect(auditLogPanelSource).not.toContain('getUpgradeActionButtonClass');
    expect(auditWebhookPanelSource).not.toContain('Audit Webhooks (Pro)');
    expect(reportingPanelSource).not.toContain('Advanced Reporting (Pro)');
    expect(rbacFeatureGateSectionSource).not.toContain('Custom Roles (Pro)');
    expect(rbacFeatureGateSectionSource).not.toContain('Centralized Access Control (Pro)');
    expect(ssoProvidersPanelSource).not.toContain('Add SAML (Pro)');
  });

  it('keeps SAML SSO available without a self-hosted Pro upsell boundary', () => {
    expect(ssoProvidersPanelSource).toContain('@/components/shared/Button');
    expect(ssoProvidersPanelSource).toContain('ActionIconButton');
    expect(ssoProvidersPanelSource).toContain('CopyValueButton');
    expect(ssoProvidersPanelSource).toContain("openAddModal('saml')");
    expect(ssoProvidersPanelSource).toContain('getSSOProviderAddButtonLabel');
    expect(ssoProvidersPanelSource).toContain('Groups Claim');
    expect(ssoProvidersPanelSource).toContain('FormTextarea');
    expect(ssoProvidersPanelSource).not.toContain(['<', 'textarea'].join(''));
    expect(ssoProvidersPanelSource).toContain(
      'Claim used for OIDC allowed groups and role mappings.',
    );
    expect(ssoProvidersPanelSource).not.toContain('showSamlUpsell');
    expect(ssoProvidersPanelSource).not.toContain('UpgradeLink');
    expect(ssoProvidersPanelSource).not.toContain('Add SAML (Pro)');
    expect(ssoProvidersStateSource).not.toContain('advanced_sso');
    expect(ssoProvidersStateSource).not.toContain('getUpgradeActionDestination');
    expect(ssoProvidersStateSource).not.toContain('loadRuntimeCapabilities');
  });

  it('keeps Docker and Podman update-action copy on the system settings presentation owner', () => {
    expect(dockerRuntimeSettingsCardSource).toContain('getDockerUpdateActionsPresentation');
    expect(dockerRuntimeSettingsCardSource).toContain('DOCKER_UPDATE_ACTIONS_ENV_VAR');
    expect(dockerRuntimeSettingsCardSource).toContain('TogglePrimitive');
    expect(dockerRuntimeSettingsCardSource).not.toContain('role="switch"');
    expect(dockerRuntimeSettingsCardSource).not.toContain('aria-checked');
    expect(dockerRuntimeSettingsCardSource).not.toContain('Container Updates');
    expect(dockerRuntimeSettingsCardSource).not.toContain('container update actions');

    expect(systemSettingsPresentationSource).toContain("getSourcePlatformLabel('docker')");
    expect(systemSettingsPresentationSource).toContain('getDockerUpdateActionsPresentation');
    expect(systemSettingsPresentationSource).toContain('DOCKER_UPDATE_ACTIONS_SECTION_TITLE');
    expect(systemSettingsPresentationSource).toContain('DOCKER_UPDATE_ACTIONS_ENV_VAR');
  });

  it('keeps audit-log filter labels on the audit presentation owner', () => {
    expect(auditLogPanelSource).toContain('AUDIT_EVENT_FILTER_ALL_LABEL');
    expect(auditLogPanelSource).toContain('AUDIT_EVENT_CONFIG_CHANGE_LABEL');
    expect(auditLogPanelSource).toContain('AUDIT_SUCCESS_FILTER_SUCCESS_ONLY_LABEL');
    expect(auditLogPanelSource).toContain('AUDIT_VERIFICATION_FILTER_ALL_LABEL');
    expect(auditLogPanelSource).not.toContain('All Events');
    expect(auditLogPanelSource).not.toContain('All Verification');
    expect(auditLogPanelSource).not.toContain('Success Only');
    expect(auditLogPresentationSource).toContain(
      "AUDIT_EVENT_FILTER_ALL_LABEL = getAllFilterOptionLabel('events')",
    );
    expect(auditLogPresentationSource).toContain(
      "AUDIT_VERIFICATION_FILTER_ALL_LABEL = getAllFilterOptionLabel('verification')",
    );
  });

  it('keeps audit-log fetch errors and page-size normalization on shared owners', () => {
    expect(auditLogStateSource).toContain('apiErrorFromResponse');
    expect(auditLogStateSource).toContain('getAuditLogFetchErrorMessage');
    expect(auditLogStateSource).toContain('normalizeAuditPageSize');
    expect(auditLogStateSource).toContain('ALLOWED_AUDIT_PAGE_SIZES');
    expect(auditLogPresentationSource).toContain("code === 'audit_store_busy'");
    expect(auditLogPresentationSource).toContain("code === 'audit_store_unavailable'");
    expect(auditLogPresentationSource).toContain("code === 'query_failed'");
  });

  it('keeps API token Docker and Podman copy on shared presentation helpers', () => {
    expect(apiAccessPanelSource).toContain('API_TOKEN_ACCESS_PANEL_DESCRIPTION');
    expect(apiAccessPanelSource).toContain('ButtonLink');
    expect(apiAccessPanelSource).toContain('variant="info"');
    expect(apiAccessPanelSource).not.toContain('rel="noreferrer"');
    expect(apiAccessPanelSource).not.toContain(
      'inline-flex min-h-10 sm:min-h-10 w-fit items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700',
    );
    expect(apiTokenManagerModelSource).toContain('API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION');
    expect(apiTokenManagerModelSource).toContain('API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION');
    expect(apiTokenManagerSource).toContain('ExternalTextLink');
    expect(apiTokenManagerSource).not.toContain('rel="noreferrer"');
    expect(apiTokenManagerSource).toContain('getAPITokenDockerPodmanUsageSummary');
    expect(apiTokenManagerSource).toContain('getAPITokenDockerPodmanUsageTitle');
    expect(apiTokenManagerStateSource).toContain('getAPITokenDockerPodmanUsageCountLabel');

    for (const source of [
      apiAccessPanelSource,
      apiTokenManagerSource,
      apiTokenManagerModelSource,
      apiTokenManagerStateSource,
    ]) {
      expect(source).not.toContain('container runtime');
      expect(source).not.toContain('Container runtime');
      expect(source).not.toContain('container runtimes');
      expect(source).not.toContain('Container runtimes');
    }
  });

  it('keeps infrastructure Docker and Podman settings copy on shared source-platform labels', () => {
    expect(infrastructureOperationsModelSource).toContain("getSourcePlatformLabel('docker')");
    expect(agentProfileSettingsSource).toContain("getSourcePlatformLabel('docker')");
    expect(diagnosticsResultsPanelSource).toContain("getSourcePlatformLabel('docker')");

    for (const source of [
      infrastructureOperationsModelSource,
      agentProfileSettingsSource,
      diagnosticsResultsPanelSource,
    ]) {
      expect(source).not.toContain('Container Runtime Agents');
      expect(source).not.toContain('Agent-backed container runtime monitoring');
      expect(source).not.toContain('Force container runtime monitoring');
      expect(source).not.toContain('Force a specific container runtime');
      expect(source).not.toContain('Docker Runtime');
    }
  });

  it('keeps settings native select controls on the shared labelled primitive', () => {
    const rawSelectUsers = Object.entries(settingsRuntimeSources)
      .filter(([, source]) => source.includes('<select'))
      .map(([path]) => path)
      .sort();

    expect(rawSelectUsers).toEqual([]);

    for (const source of [
      aiChatMaintenanceSectionSource,
      aiRuntimeControlsSectionSource,
      auditLogPanelSource,
      recoverySettingsPanelSource,
      systemLogsPanelSource,
      agentProfilesPanelSource,
      infrastructureInstallerSectionSource,
      nodeModalMonitoringSectionSource,
      trueNASCredentialSlotSource,
      organizationAccessManagementSectionSource,
      organizationAccessMembersSectionSource,
      organizationSharingCreateSectionSource,
      rolesEditorDialogSource,
    ]) {
      expect(source).toContain("import { FormSelect } from '@/components/shared/FormSelect';");
      expect(source).toContain('<FormSelect');
    }
  });

  it('keeps settings copy aligned with Infrastructure as the default workspace', () => {
    // Copy phrases are matched against whitespace-normalized source so
    // prettier line-wrapping inside JSX text cannot break the guard.
    const normalize = (source: string) => source.replace(/\s+/g, ' ');
    expect(normalize(settingsHeaderMetaSource)).toContain(
      'Manage appearance, layout, and default monitoring cadence.',
    );
    expect(normalize(generalSettingsPanelSource)).not.toContain('Dashboard, Infrastructure');
    expect(normalize(networkBoundarySettingsSectionSource)).toContain(
      'Pulse URL for Notifications',
    );
    expect(normalize(networkBoundarySettingsSectionSource)).not.toContain(
      'Dashboard URL for Notifications',
    );
    expect(normalize(nodeModalBasicInfoSectionSource)).toContain('monitoring views');
    expect(normalize(nodeModalBasicInfoSectionSource)).not.toContain('dashboards');
    expect(normalize(nodeModalMonitoringSectionSource)).toContain('trim workload noise');
    expect(normalize(nodeModalMonitoringSectionSource)).toContain('Existing monitoring readings');
    expect(normalize(nodeModalMonitoringSectionSource)).not.toContain('dashboard readings');
    expect(normalize(recoverySettingsPanelSource)).toContain('Required for workload backup status');
    expect(normalize(recoverySettingsPanelSource)).not.toContain('dashboard backup status');
  });

  it('keeps cluster member address edits on the durable IPOverride boundary', () => {
    // Existing PVE clusters expose per-member connection addresses in the
    // node editor; the discovered Host and IP are rebuilt on re-discovery,
    // so the editor must write ClusterEndpoint.ipOverride, never host.
    expect(nodeCredentialSlotSource).toContain('<NodeModalClusterMembersSection');
    expect(nodeModalClusterMembersSectionSource).toContain('updateClusterEndpointOverride(');
    expect(nodeModalClusterMembersSectionSource).toContain('Connection address for ');
    // Only changed members ride the write-only PUT payload.
    expect(nodeModalStateSource).toContain('buildClusterEndpointOverridesPayload(');
    expect(nodeModalModelSource).toContain('export const buildClusterEndpointOverridesPayload = (');
  });

  it('keeps system AI model catalogs on the shared searchable picker boundary', () => {
    expect(aiModelSelectionSectionSource).toContain(
      "import { AIModelPicker } from '@/components/shared/AIModelPicker';",
    );
    expect(aiModelSelectionSectionSource).toContain('const selectableModels =');
    expect(aiModelSelectionSectionSource).toContain(
      'isModelProviderConfigured(model.id, state.settings()) || model.id === selected',
    );
    expect(aiModelSelectionSectionSource).toContain(
      'searchPlaceholder="Search configured provider models"',
    );
    expect(aiModelSelectionSectionSource).toContain('formatAIModelRouteLabel(match || trimmed)');
    expect(aiSettingsStatusAndActionsSource).toContain('formatAIModelRouteLabel(match || modelId)');
    expect(aiSettingsStatusAndActionsSource).not.toContain("model?.split(':').pop()");
    expect(aiModelSelectionSectionSource).not.toContain('<select');
    expect(aiModelSelectionSectionSource).not.toContain('<optgroup');

    expect(aiModelSelectionSectionSource).toContain('type AIModelOverrideKind');
    expect(aiModelSelectionSectionSource).toContain("formKey: 'chatModel' | 'patrolModel'");
    expect(aiModelSelectionSectionSource).toContain("label: 'Pulse Assistant model'");
    expect(aiModelSelectionSectionSource).toContain(
      'Used for chat, explanations, and review. Patrol handles infrastructure work.',
    );
    expect(aiModelSelectionSectionSource).not.toContain('approved fix execution');
    expect(aiModelSelectionSectionSource).toContain("label: 'Patrol model'");
    expect(aiModelSelectionSectionSource).toContain(
      'Used when Patrol checks, investigates, and verifies work.',
    );
    expect(aiModelSelectionSectionSource).toContain("label: 'Service context model'");
    expect(aiModelSelectionSectionSource).toContain('export const AIModelOverrideField');
    expect(aiSettingsSource).toContain(
      '<AIModelOverrideField state={props.state} kind="assistant" />',
    );
    expect(aiSettingsSource).toContain(
      '<AIModelOverrideField state={props.state} kind="patrol" includePatrolReadiness />',
    );
    expect(aiSettingsSource).toContain(
      '<AIModelOverrideField state={props.state} kind="discovery" />',
    );
    expect(aiSettingsStateSource).toContain('discoveryModel:');
    expect(aiSettingsStateSource).toContain('payload.discovery_model = form.discoveryModel');

    expect(aiSettingsModelSource).toContain('AIProviderTestResult');
    expect(aiSettingsModelSource).toContain('AISettings as AISettingsType');
    expect(aiSettingsModelSource).toContain('export type AIAvailableModel = ModelInfo;');
    expect(aiSettingsStateSource).toContain(
      'const [availableModels, setAvailableModels] = createSignal<ModelInfo[]>([]);',
    );
  });

  it('keeps system AI save feedback tied to provider and Patrol readiness context', () => {
    expect(aiSettingsSource).toContain('Choose a Patrol mode on the Patrol page.');
    expect(aiSettingsSource).toContain('Open Patrol');
    expect(aiSettingsSource).toContain("const isProviderPage = () => page() === 'provider';");
    expect(aiSettingsSource).toContain('showConnectionControls={isProviderPage()}');
    expect(aiSettingsStatusAndActionsSource).toContain('showConnectionControls?: boolean');
    expect(aiSettingsStatusAndActionsSource).toContain(
      'props.showConnectionControls && state.settings()?.configured',
    );
    expect(aiSettingsSource).not.toContain(
      'Set how much Patrol can do on its own from the Patrol page',
    );
    expect(aiSettingsSource).not.toContain('Patrol autonomy is the operations policy');
    expect(aiSettingsStateSource).toContain(
      "import { apiErrorDetails } from '@/api/responseUtils';",
    );
    expect(aiSettingsStateSource).toContain('resolveAISettingsSaveProviderFailure');
    expect(aiSettingsStateSource).toContain('getAISettingsPatrolReadinessSaveMessage(');
    expect(aiSettingsStateSource).toContain(
      "getAISettingsPatrolReadinessSaveMessage(\n        updated.patrol_readiness,\n        'Pulse Intelligence enabled',",
    );
    expect(aiSettingsStateSource).toContain('notificationStore.warning(patrolReadinessMessage);');
    expect(aiSettingsStateSource).toContain('getAISettingsSaveProviderFailureMessage(');
  });

  it('keeps update readiness checks on the shared install guide boundary', () => {
    expect(updateInstallGuideSource).toContain('UpdateReadinessPanel');
    expect(updateInstallGuideSource).toContain("props.updatePlan?.readiness?.status === 'blocked'");
    expect(updateInstallGuideSource).toContain('Install blocked');
    expect(updateInstallGuideSource).toContain(
      'disabled={props.isInstalling || readinessBlocked()}',
    );
  });

  it('keeps Pro-runtime docker updates off the community image commands', () => {
    // A Pro-runtime container updates with digest-pinned commands from the
    // license server broker; pulling the community rcourtman/pulse image
    // would silently downgrade it to the community build. The panel keys the
    // suppression off the compiled runtime identity, and the install guide
    // routes both the available-update steps and the idle Docker box through
    // that flag.
    expect(updatesSettingsPanelSource).toContain("runtimeCapabilities()?.runtime?.build === 'pro'");
    expect(updatesSettingsPanelSource).toContain('isProRuntime={isProRuntime()}');
    expect(updateInstallGuideSource).toContain('isProRuntime: boolean');
    expect(updateInstallGuideSource).toContain('IDLE_DOCKER_PRO_NOTICE');
  });

  it('keeps update history and rollback on the dedicated section boundary', () => {
    // The panel shell mounts the history section; it must not inline history
    // rows or rollback confirmation itself.
    expect(updatesSettingsPanelSource).toContain(
      "import { UpdateHistorySection } from '@/components/Settings/UpdateHistorySection';",
    );
    expect(updatesSettingsPanelSource).toContain('<UpdateHistorySection />');
    expect(updatesSettingsPanelSource).not.toContain('listUpdateHistory');
    expect(updatesSettingsPanelSource).not.toContain('rollbackUpdate');
    // The section starts rollbacks through the shared store action, never its
    // own POST, and always behind an explicit confirmation dialog gated to
    // successful updates whose backup is still retained.
    expect(updateHistorySectionSource).toContain('updateStore.rollbackUpdate');
    expect(updateHistorySectionSource).not.toContain('apiFetchJSON');
    expect(updateHistorySectionSource).toContain('ariaLabel="Confirm rollback"');
    expect(updateHistorySectionSource).toContain(
      "entry.action === 'update' && entry.status === 'success' && Boolean(entry.backup_path)",
    );
  });

  it('keeps infrastructure on a source-manager landing with route-backed dialogs', () => {
    expect(infrastructureWorkspaceSource).toContain(
      "import { ConnectionEditor } from './ConnectionEditor/ConnectionEditor';",
    );
    expect(infrastructureWorkspaceSource).toContain(
      "import { InfrastructureSourceManager } from './InfrastructureSourceManager';",
    );
    expect(infrastructureWorkspaceSource).toContain(
      "import { InfrastructureSourcePicker } from './InfrastructureSourcePicker';",
    );
    expect(infrastructureWorkspaceSource).toContain(
      "import { Dialog } from '@/components/shared/Dialog';",
    );
    expect(infrastructureWorkspaceSource).toContain('NodeCredentialSlot');
    expect(infrastructureWorkspaceSource).toContain('TrueNASCredentialSlot');
    expect(infrastructureWorkspaceSource).toContain('VMwareCredentialSlot');
    expect(infrastructureWorkspaceSource).not.toContain('AvailabilityTargetSlot');
    expect(infrastructureWorkspaceSource).toContain(
      ": (type) => openAddFlow(type === 'agent' ? 'agent' : (type as ManagedAddTypeStep))",
    );
    expect(infrastructureWorkspaceSource).toContain('reviewDiscoveredSource');
    expect(infrastructureWorkspaceSource).toContain('selectedDiscoveredSource');
    expect(infrastructureWorkspaceSource).toContain('selectedProbeCandidate');
    expect(infrastructureWorkspaceSource).toContain('importCandidateForNodeType');
    expect(infrastructureWorkspaceSource).toContain('importCandidate={importCandidate ?? null}');
    expect(infrastructureWorkspaceSource).toContain(
      "return { kind: 'discovery', server: discovered };",
    );
    expect(infrastructureWorkspaceSource).toContain("return { kind: 'probe', candidate: probed };");
    expect(infrastructureWorkspaceSource).toContain(
      "import { InfrastructureDiscoverySettingsDialog } from './InfrastructureDiscoverySettingsDialog';",
    );
    expect(infrastructureWorkspaceSource).toContain(
      'const [showDiscoverySettings, setShowDiscoverySettings] = createSignal(false);',
    );
    expect(infrastructureWorkspaceSource).toContain('<InfrastructureDiscoverySettingsDialog');
    expect(infrastructureWorkspaceSource).toContain('onReviewDiscoveredSource');
    expect(infrastructureWorkspaceSource).toContain('void props.loadDiscoveredNodes();');
    expect(infrastructureWorkspaceSource).toContain('<InfrastructureSourceManager');
    expect(infrastructureWorkspaceSource).toContain('<InfrastructureSourcePicker');
    expect(infrastructureWorkspaceSource).not.toContain('<ConnectionsTable rows={rows} />');
    expect(infrastructureWorkspaceSource).not.toContain("from './ConnectionsTable'");
    expect(infrastructureWorkspaceSource).toContain('flex h-full min-h-0 flex-col');
    expect(infrastructureWorkspaceSource).toContain('showSlotHeader={false}');
    expect(infrastructureWorkspaceSource).toContain(
      "onDetectApiPlatform={() => openAddFlow('detect')}",
    );
    expect(infrastructureWorkspaceSource).toContain("onBackToCatalog={() => openAddFlow('pick')}");
    expect(infrastructureWorkspaceSource).not.toContain('trackInitialCatalogSelection');
    expect(infrastructureWorkspaceSource).not.toContain('recordCatalogSelection');
    expect(infrastructureWorkspaceSource).not.toContain('onboardingMetricsTracker');
    expect(infrastructureWorkspaceSource).toContain('renderAgentConnectionDetails');
    expect(infrastructureWorkspaceSource).not.toContain('InfrastructureOperationsController');
    expect(infrastructureWorkspaceSource).not.toContain('PlatformConnectionsWorkspace');
    // Card title shifted from 'Infrastructure systems' (which duplicated
    // the page header) to 'Connected systems' which describes the card
    // content distinctly.
    expect(infrastructureSourceManagerSource).toContain('Connected systems');
    // Discovery owns an explicit status/action band inside Connected systems
    // so manual scans are observable without opening the settings dialog.
    expect(infrastructureSourceManagerSource).toContain('Run discovery');
    // Scan scope choices must be full-row controls; a narrow native radio input
    // regressed the custom subnet path into a hard-to-click target.
    expect(discoverySettingsFormSource).toContain('role="radiogroup"');
    expect(discoverySettingsFormSource).toContain('role="radio"');
    expect(discoverySettingsFormSource).toContain('selectDiscoveryMode');
    expect(infrastructureSourceManagerSource).toContain('Discover Proxmox systems');
    expect(infrastructureSourceManagerSource).toContain('Configure discovery');
    expect(monitoredSystemImpactPreviewSource).toContain('getMonitoredSystemImpactPreviewTitle');
    expect(monitoredSystemImpactPreviewSource).toContain(
      'formatMonitoredSystemImpactPreviewSummary',
    );
    expect(monitoredSystemImpactPreviewSource).not.toContain('Current usage');
    expect(monitoredSystemImpactPreviewSource).not.toContain(' / ');
    expect(monitoredSystemImpactPreviewSource).not.toContain(
      'reuses your current monitored-system allowance',
    );
    expect(monitoredSystemImpactPreviewSource).not.toContain('frees monitored-system allowance');
    expect(nodeCandidateImportPlanSource).toContain('aria-label="Candidate import plan"');
    expect(nodeCandidateImportPlanSource).toContain('MonitoredSystemImpactPreview');
    expect(nodeCandidateImportPlanSource).toContain('Preview impact');
    expect(nodeCandidateImportPlanSource).toContain('Approve this import plan');
    // Card-level description was dropped; the page-level subtitle from
    // SETTINGS_HEADER_META carries the same intent without duplication.
    expect(infrastructureSourceManagerSource).toContain('onReviewDiscoveredSource');
    expect(infrastructureSourceManagerSource).toContain('Discovered');
    expect(infrastructureSourceManagerSource).toContain('getInfrastructureSourceManagerProducts');
    expect(infrastructureSourceManagerSource).toContain('TableHeader');
    expect(infrastructureSourceManagerSource).toContain('getGroupedTableRowClass');
    expect(infrastructureSourceManagerSource).not.toContain('bg-base hover:bg-base');
    expect(infrastructureSourceManagerSource).toContain('aria-label={group.actionLabel}');
    expect(infrastructureWorkspaceSource).toContain('onAddSourceStep');
    expect(infrastructureSourceManagerSource).toContain('getAgentHostProfileFamily');
    expect(infrastructureSourceManagerSource).toContain('visibleSourceGroupsForProduct');
    expect(infrastructureSourceManagerSource).toContain(
      'group.id === product.type && discoveredRows().length > 0',
    );
    expect(infrastructureSourceManagerSource).toContain('Review');
    expect(infrastructureSourceManagerSource).toContain('Manage');
    expect(infrastructureSourceManagerSource).not.toContain('Detect address');
    expect(infrastructureSourceManagerSource).toContain("'Install agent'");
    expect(infrastructureSourceManagerSource).toContain('Add infrastructure');
    expect(infrastructureSourceManagerSource).not.toContain('Monitor endpoint');
    expect(infrastructureSourceManagerSource).toContain('getInfrastructureEmptyStateSummary');
    expect(infrastructureSourceManagerSource).toContain('Connection posture');
    expect(infrastructureSourceManagerSource).toContain("'system')} connected");
    expect(infrastructureSourceManagerSource).toContain('All active');
    expect(infrastructureSourceManagerSource).toContain('needs attention');
    expect(infrastructureSourceManagerSource).toContain('has limited coverage');
    expect(infrastructureSourceManagerSource).toContain('setupConfidenceAction');
    expect(infrastructureSourceManagerSource).not.toContain('Infrastructure coverage');
    expect(infrastructureSourceManagerSource).not.toContain('Fleet governance');
    expect(infrastructureSourceManagerSource).not.toContain('Connection types');
    expect(infrastructureSourcePickerSource).toContain('Detect API platform');
    expect(infrastructureSourcePickerSource).toContain('getInfrastructureSourcePickerItems');
    expect(infrastructureSourcePickerSource).toContain('itemMatchesQuery');
    expect(infrastructureSourcePickerSource).toContain('catalogDescription');
    expect(infrastructureSourcePickerSource).not.toContain('Monitor network endpoint');
    expect(infrastructureSourcePickerSource).not.toContain(
      'getInfrastructureSourceStrategyPresentation',
    );
    expect(availabilitySettingsPanelSource).toContain('AvailabilityTargetSlot');
    expect(availabilitySettingsPanelSource).toContain('buildAvailabilityTargetAddPath');
    expect(availabilitySettingsPanelSource).toContain('Availability checks');
    expect(availabilitySettingsPanelSource).toContain('MQTT broker');
    expect(settingsHeaderMetaSource).toContain(
      "description: 'Configure the public URL, CORS, embedding, and webhook network boundaries.'",
    );
    expect(useConnectionsLedgerSource).toContain('createNonSuspendingQuery');
    expect(useConnectionsLedgerSource).toContain('pollMs: POLL_INTERVAL_MS');
    expect(useConnectionsLedgerSource).not.toContain('createResource');

    expect(connectionsTableModelSource).toContain(
      "'inline-flex items-center rounded-full bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-base-content'",
    );
    expect(connectionsTableModelSource).toContain(
      "'inline-flex items-center rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-medium text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'",
    );
    expect(connectionsTableModelSource).not.toContain(['bg', 'slate', '100'].join('-'));
    expect(connectionsTableModelSource).not.toContain(['text', 'slate', '700'].join('-'));
  });

  it('keeps setup-handoff token cleanup from masking install failures', () => {
    expect(useInfrastructureInstallStateSource).toContain(
      [
        '} finally {',
        '      if (!disposed) {',
        '        setIsGeneratingToken(false);',
        "        if (source === 'setup_handoff') {",
        '          setSetupHandoffAutoTokenPending(false);',
        '        }',
        '      }',
        '    }',
      ].join('\n'),
    );
    expect(useInfrastructureInstallStateSource).not.toMatch(
      /finally \{[\s\S]*?if \(disposed\) \{[\s\S]*?return;/,
    );
  });

  it('keeps the detect-first editor and inline credential bodies on the shared editor model', () => {
    expect(connectionEditorSource).toContain(
      "import { AddressProbeStep } from './AddressProbeStep';",
    );
    expect(connectionEditorSource).toContain(
      "from '@/utils/infrastructureOnboardingPresentation';",
    );
    expect(infrastructureSourceManagerSource).toContain(
      "from '@/utils/infrastructureOnboardingPresentation';",
    );
    expect(connectionEditorSource).toContain('<AddressProbeStep');
    expect(connectionEditorSource).toContain('API platform probe');
    expect(connectionEditorSource).toContain('flex h-full min-h-0 flex-col');
    expect(connectionEditorSource).toContain('Back to source types');
    expect(connectionEditorSource).toContain('Back to API probe');
    expect(connectionEditorSource).toContain('Install Pulse Agent');
    expect(connectionEditorSource).not.toContain('buildConnectionEditorCatalogEntries');
    expect(connectionEditorSource).not.toContain('selectedFamilyId');
    expect(connectionEditorSource).not.toContain('Choose how Pulse should connect');
    expect(connectionEditorSource).not.toContain('Connect a supported platform');
    expect(connectionEditorSource).not.toContain('Choose a {family.label} product');
    expect(connectionEditorSource).not.toContain('Back to platforms');
    expect(connectionEditorSource).not.toContain('NodeModal');

    expect(addressProbeStepSource).toContain('Probe API endpoint');
    expect(addressProbeStepSource).toContain('install Pulse Agent instead');
    expect(addressProbeStepSource).toContain('Choose a source type instead');
    expect(addressProbeStepSource).toContain('getInfrastructureAgentHostProfileSupportText');
    expect(addressProbeStepSource).toContain('supported API-backed platform');

    expect(connectionEditorStateSource).toContain('ConnectionsAPI.probe(value)');
    expect(connectionEditorStateSource).toContain('export const CONNECTION_TYPE_LABELS');
    expect(connectionEditorStateSource).not.toContain('DEFAULT_CONNECTION_EDITOR_CATALOG_ENTRIES');
    expect(connectionEditorStateSource).not.toContain('buildConnectionEditorCatalogEntries');
    expect(connectionEditorStateSource).not.toContain('getSourcePlatformFamily');
    expect(infrastructureOnboardingPresentationSource).toContain('getSourcePlatformManifestEntry');
    expect(infrastructureOnboardingPresentationSource).toContain(
      'getInfrastructureSourcePickerItems',
    );
    expect(infrastructureOnboardingPresentationSource).toContain(
      'getInfrastructureSourceStrategyPresentation',
    );
    expect(infrastructureOnboardingPresentationSource).toContain('API first');
    expect(infrastructureOnboardingPresentationSource).toContain(
      'getInfrastructureSupportSummaryBadges',
    );
    expect(infrastructureOnboardingPresentationSource).toContain(
      'VMware vCenter is available as a preview platform',
    );

    expect(nodeCredentialSlotSource).toContain('useNodeModalState(modalProps)');
    expect(nodeCredentialSlotSource).toContain('<NodeModalBasicInfoSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalAuthenticationSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalMonitoringSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalStatusFooter');
    expect(nodeCredentialSlotSource).not.toContain('<Dialog');
    expect(nodeCredentialSlotSource).toContain('buildNodeImportPlan');
    expect(nodeCredentialSlotSource).toContain('MonitoredSystemLedgerAPI.preview');
    expect(nodeCredentialSlotSource).toContain('<NodeCandidateImportPlan');
    expect(nodeCredentialSlotSource).toContain('setupHandoffDisabled');
    expect(nodeCredentialSlotSource).toContain('saveDisabled={saveBlockedByImportPlan()}');
    expect(infrastructureImportPlanModelSource).toContain('MonitoredSystemLedgerPreviewRequest');
    expect(infrastructureImportPlanModelSource).toContain('nodeTypeToMonitoredSource');
    expect(infrastructureImportPlanModelSource).toContain('previewRequest');
    expect(infrastructureImportPlanModelSource).toContain('resource_id: name');
    expect(nodeModalAuthenticationSectionSource).toContain(
      "state.formData().setupMode === 'manual'",
    );
    expect(nodeModalAuthenticationSectionSource).toContain('Advanced manual token path');
    expect(nodeModalSetupGuideSectionSource).toContain('Source strategy');
    expect(nodeModalSetupGuideSectionSource).toContain('Host telemetry agent');
    expect(nodeModalSetupGuideSectionSource).toContain('API inventory');
    expect(nodeModalSetupGuideSectionSource).toContain('Manual API token');
    expect(nodeModalSetupGuideSectionSource).toContain('Recommended least-privilege path');
    expect(nodeModalSetupGuideSectionSource).toContain('Existing source repair');
    expect(nodeModalSetupGuideSectionSource).toContain('Audit/Repair');
    expect(nodeModalSetupGuideSectionSource).toContain('without rotating the current API token');
    expect(nodeModalSetupGuideSectionSource).toContain('VM.GuestAgent.Audit/FileRead');
    expect(nodeModalModelSource).toContain('HAS_GUEST_AGENT_AUDIT');
    expect(nodeModalModelSource).toContain('VM.GuestAgent.FileRead');
    expect(nodeModalModelSource).toContain('PulseTmpVMMonitor');
    const guestAgentBranch = nodeModalModelSource.indexOf(
      'if [ "$HAS_GUEST_AGENT_AUDIT" = true ]; then',
    );
    const monitorFallback = nodeModalModelSource.indexOf(
      'pveum role add PulseTmpVMMonitor -privs VM.Monitor',
    );
    expect(guestAgentBranch).toBeGreaterThanOrEqual(0);
    expect(monitorFallback).toBeGreaterThanOrEqual(0);
    expect(guestAgentBranch).toBeLessThan(monitorFallback);
    expect(nodeModalSetupGuideSectionSource).toContain('Docker inside Proxmox LXCs');
    expect(nodeModalSetupGuideSectionSource).toContain(
      'PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true',
    );
    expect(nodeModalSetupGuideSectionSource).toContain('setupCommandButtonTitle');
    expect(nodeModalSetupGuideSectionSource).toContain('disabled={setupHandoffDisabled()}');
    expect(nodeModalSetupGuideSectionSource).toContain('Credentialed command ready');
    expect(nodeModalSetupGuideSectionSource).toContain(
      'one-time setup token is intentionally not shown on this page',
    );
    expect(nodeModalStateSource).toContain('data.commandWithEnv');
    expect(nodeModalStateSource).not.toContain('quickSetupPreviewCommand');
    expect(nodeModalStatusFooterSource).toContain('guidedSetupOnlyMode');
    expect(nodeModalStatusFooterSource).toContain('props.saveDisabled');
    expect(nodeModalStateSource).toContain("enableCommands: type === 'pve'");
    expect(nodeModalStateSource).toContain('data.setupMode !==');

    expect(vmwareCredentialSlotSource).toContain('TlsVerificationWarningBanner');
    expect(vmwareCredentialSlotSource).toContain('subject="this vCenter connection"');
    expect(vmwareCredentialSlotSource).toContain(
      'Install a trusted certificate for vCenter before using this in production.',
    );

    expect(trueNASCredentialSlotSource).toContain('TlsVerificationWarningBanner');
    expect(trueNASCredentialSlotSource).toContain('subject="this TrueNAS connection"');
    expect(trueNASCredentialSlotSource).toContain(
      'Install a trusted certificate or configure the TLS fingerprint before using this in production.',
    );
  });

  it('exposes external-agent setup from Pulse Intelligence while API Access stays token-only', () => {
    // External agents are a Pulse Intelligence access path over the same
    // governed Patrol policy, not a generic Security API feature. API Access
    // remains the place to mint the scoped credential when setup needs one.
    expect(aiSettingsSource).toContain(
      "import AgentIntegrationsPanel from '@/components/Settings/AgentIntegrationsPanel';",
    );
    expect(aiSettingsSource).toContain(
      "const showExternalAgentAccess = () => page() === 'assistant';",
    );
    expect(aiSettingsSource).toContain('<AgentIntegrationsPanel />');
    expect(apiAccessPanelSource).not.toContain('AgentIntegrationsPanel');
    expect(apiAccessPanelSource).not.toContain('isExternalAgentSetupHash');
    expect(apiAccessPanelSource).not.toContain('api-access-external-agent-section');
    expect(apiAccessPanelSource).toContain('api-access-token-section');
    expect(agentIntegrationsPanelSource).toContain('ExternalTextLink');
    expect(agentIntegrationsPanelSource).not.toContain('rel="noreferrer"');
    expect(agentIntegrationsPanelSource).toContain('External agents');
    expect(agentIntegrationsPanelSource).toContain('Connect external tools');
    expect(agentIntegrationsPanelSource).toContain('read Pulse context');
    expect(agentIntegrationsPanelSource).toContain('request Patrol work');
    expect(agentIntegrationsPanelSource).toContain('Patrol mode and scoped tokens control');
    expect(agentIntegrationsPanelSource).not.toContain(
      'Optional connector access for Claude Desktop',
    );
    expect(agentIntegrationsPanelSource).not.toContain('Patrol remains the operator');
    expect(agentIntegrationsPanelSource).not.toContain('Connected tools');
    expect(agentIntegrationsPanelSource).toContain('setupOpen');
    expect(agentIntegrationsPanelSource).toContain('Show connector setup');
    expect(agentIntegrationsPanelSource).toContain('Hide connector setup');
    expect(agentIntegrationsPanelSource).toContain('<Show when={!setupOpen()}>');
    expect(agentIntegrationsPanelSource).not.toContain(
      [
        'Choose',
        'what',
        'Patrol',
        'may',
        'handle',
        'automatically',
        'before',
        'any',
        'external',
        'agent',
        'can',
      ].join(' '),
    );
    expect(agentIntegrationsPanelSource).toContain(
      'Required before agents can request Patrol work.',
    );
    expect(agentIntegrationsPanelSource).not.toContain(
      'Set the Patrol mode before connected agents can request work.',
    );
    expect(agentIntegrationsPanelSource).toContain('Choose Patrol mode');
    expect(agentIntegrationsPanelSource).toContain('PATROL_CONTROL_PATH');
    expect(agentIntegrationsPanelSource).not.toContain("settingsTabPath('system-ai')");
    expect(agentIntegrationsPanelSource).toContain('EXTERNAL_AGENT_SETUP_ANCHOR');
    expect(agentIntegrationsPanelSource).toContain('PULSE_MCP_SETUP_ANCHOR');
    expect(agentIntegrationsPanelSource).toContain('focusPanelUntilLayoutSettles');
    expect(agentIntegrationsPanelSource).toContain('findScrollableAncestor');
    expect(agentIntegrationsPanelSource).toContain(
      'document.getElementById(EXTERNAL_AGENT_SETUP_ANCHOR)',
    );
    expect(agentIntegrationsPanelSource).toContain('window.history.replaceState');
    expect(agentIntegrationsPanelSource).not.toContain('Use Pulse MCP only');
    expect(agentIntegrationsPanelSource).not.toContain('outside client');
    expect(agentIntegrationsPanelSource).not.toContain('Pulse Intelligence surfaces');
    expect(agentIntegrationsPanelSource).toContain('Connector setup');
    expect(agentIntegrationsPanelSource).toContain('Connect the agent');
    expect(agentIntegrationsPanelSource).toContain('Developer details');
    expect(agentIntegrationsPanelSource).toContain('advancedClientDetailsOpen');
    expect(agentIntegrationsPanelSource).toContain(
      'Only open this when you are building or debugging a client',
    );
    expect(agentIntegrationsPanelSource).toContain('Patrol access model');
    expect(agentIntegrationsPanelSource).toContain(
      'Built-in Pulse views and connected clients all sit behind',
    );
    expect(agentIntegrationsPanelSource).toContain('Live manifest details');
    expect(agentIntegrationsPanelSource).not.toContain('Build a custom client');
    expect(agentIntegrationsPanelSource).not.toContain('Instance manifest details');
    expect(agentIntegrationsPanelSource).not.toContain('MCP client config');
    expect(agentIntegrationsPanelSource).toContain('normalizeAgentMCPAdapter');
    expect(agentIntegrationsPanelSource).not.toContain('getAgentMCPClientExamples');
    expect(agentIntegrationsPanelSource).toContain('getAgentWorkflowPrompts');
    expect(agentIntegrationsPanelSource).toContain('Agent starting points');
    expect(agentIntegrationsPanelSource).toContain('Patrol');
    expect(agentIntegrationsPanelSource).not.toContain('Operations loop');
    expect(agentIntegrationsPanelSource).toContain('OpenCode');
    expect(agentIntegrationsPanelSource).toContain(
      "import { Button, ButtonLink } from '@/components/shared/Button';",
    );
    expect(agentIntegrationsPanelSource).toContain(
      "import KeyRoundIcon from 'lucide-solid/icons/key-round';",
    );
    expect(agentIntegrationsPanelSource).toContain('API_TOKEN_PATROL_EXTERNAL_AGENT_PRESET_LABEL');
    expect(agentIntegrationsPanelSource).toContain('PULSE_MCP_TOKEN_SETUP_PATH');
    expect(agentIntegrationsPanelSource).toContain('hardNavigation');
    expect(apiTokenManagerSource).toContain('API_TOKEN_CREATE_ANCHOR');
    expect(agentIntegrationsPanelSource).toContain('formatAgentOpenCodeMCPConfig');
    expect(agentIntegrationsPanelSource).toContain('formatAgentMCPServersConfig');
    expect(agentIntegrationsPanelSource).toContain('Installer commands');
    expect(agentIntegrationsPanelSource).toContain('installerCommandsOpen');
    expect(agentIntegrationsPanelSource).toContain('mcpInstallShellCommand');
    expect(agentIntegrationsPanelSource).toContain('mcpInstallPowerShellCommand');
    expect(agentIntegrationsPanelSource).toContain('Client config');
    expect(agentIntegrationsPanelSource).toContain('clientConfigOpen');
    expect(agentIntegrationsPanelSource).toContain('Install the connector');
    expect(agentIntegrationsPanelSource).toContain('paste the client config');
    expect(agentIntegrationsPanelSource).not.toContain('The installer and');
    expect(agentIntegrationsPanelSource).not.toContain(
      'on the machine that runs the external tool',
    );
    expect(agentIntegrationsPanelSource).not.toContain('Install the Pulse MCP bridge');
    expect(agentIntegrationsPanelSource).not.toContain(
      'Use the OpenCode or Claude-style snippet below',
    );
    expect(agentIntegrationsPanelSource).toContain('opencode.json');
    expect(agentCapabilitiesApiSource).toContain("type: 'local'");
    expect(agentCapabilitiesApiSource).toContain('environment');
    expect(agentIntegrationsPanelSource).toContain('mcpServersConfigFamily');
    expect(agentCapabilitiesApiSource).toContain('custom clients');
    expect(agentIntegrationsPanelSource).toContain('requiredScopes');
    expect(agentIntegrationsPanelSource).toContain('Manifest scopes');
    expect(agentIntegrationsPanelSource).not.toContain('published contract');
    expect(agentIntegrationsPanelSource).toContain('External agents expose');
    expect(agentCapabilitiesApiSource).toContain('surfaceLabel,');
    expect(agentIntegrationsPanelSource).toContain('capability below shows its required scope');
    expect(agentIntegrationsPanelSource).toContain('getAgentSurfaceContractEntries');
    expect(agentIntegrationsPanelSource).toContain('surfaceContractEntries');
    expect(agentIntegrationsPanelSource).toContain('Built-in Pulse views');
    expect(agentCapabilitiesApiSource).toContain('AgentSurfaceAffordanceContract');
    expect(agentCapabilitiesApiSource).toContain('AgentSurfaceToolContract');
    expect(agentCapabilitiesApiSource).toContain('normalizeSurfaceAffordances');
    expect(agentCapabilitiesApiSource).toContain('surfaceAffordanceLabels');
    expect(agentCapabilitiesApiSource).toContain('AGENT_SURFACE_ID_PULSE_MCP');
    expect(agentCapabilitiesApiSource).toContain('getAgentManifestSurfaceToolContract');
    expect(agentCapabilitiesApiSource).toContain('getAgentManifestSurfaceToolContracts');
    expect(agentIntegrationsPanelSource).toContain('AGENT_SURFACE_ID_PULSE_MCP');
    expect(agentIntegrationsPanelSource).toContain('getAgentManifestSurfaceToolContract');
    expect(agentIntegrationsPanelSource).toContain('getAgentSurfaceToolPosturePresentation');
    expect(agentIntegrationsPanelSource).toContain('mcpSurfaceToolPosture');
    expect(agentIntegrationsPanelSource).toContain('data-testid="agent-mcp-tool-posture"');
    expect(agentIntegrationsPanelSource).toContain('External agents expose {posture().label}');
    expect(agentCapabilitiesApiSource).toContain('capability availability');
    expect(agentCapabilitiesApiSource).toContain("if (affordances.tools) labels.push('Actions')");
    expect(agentCapabilitiesApiSource).not.toContain('runtime tool availability');
    expect(agentIntegrationsPanelSource).toContain('getAgentCapabilityErrorCodeSummaries');
    expect(agentIntegrationsPanelSource).toContain('errorCodeSummaries');
    expect(agentIntegrationsPanelSource).toContain('Failure codes');
    expect(agentIntegrationsPanelSource).toContain('from the live manifest');
    expect(agentIntegrationsPanelSource).toContain('groupAgentCapabilitiesByManifestCategories');
    expect(agentIntegrationsPanelSource).toContain("from '@/api/agentCapabilities'");
    expect(agentIntegrationsPanelSource).not.toContain("fetch('/api/agent/capabilities'");
    expect(agentCapabilitiesApiSource).toContain(
      "export const AGENT_CAPABILITIES_PATH = '/api/agent/capabilities'",
    );
    expect(agentCapabilitiesApiSource).toContain("from './generated/agentCapabilities'");
    expect(agentCapabilitiesApiSource).toContain('skipAuth: true');
    expect(agentCapabilitiesApiSource).toContain('manifest.categories');
    expect(agentCapabilitiesApiSource).toContain('AgentMCPAdapterContract');
    expect(agentCapabilitiesApiSource).toContain('manifest?.surfaceToolContracts');
    expect(agentCapabilitiesApiSource).toContain('manifest?.mcpAdapter');
    expect(agentCapabilitiesApiSource).toContain('getAgentMCPClientExamples');
    expect(agentCapabilitiesApiSource).toContain('formatAgentOpenCodeMCPConfig');
    expect(agentCapabilitiesApiSource).toContain('formatAgentMCPServersConfig');
    expect(agentCapabilitiesApiSource).toContain('getAgentCapabilityErrorCodeSummaries');
    expect(agentCapabilitiesApiSource).not.toContain('export interface AgentCapability {');
    expect(agentCapabilitiesApiSource).not.toContain(
      'export interface AgentCapabilitiesManifest {',
    );
    expect(agentCapabilitiesApiSource).not.toContain('getAgentMCPSurfaceToolContract');
    expect(agentIntegrationsPanelSource).not.toContain('MCP_CLIENT_EXAMPLES');
    expect(agentIntegrationsPanelSource).not.toContain('function formatClaudeMcpConfig');
    expect(agentIntegrationsPanelSource).not.toContain('function formatOpenCodeMcpConfig');
    expect(agentIntegrationsPanelSource).not.toContain('CATEGORY_ORDER');
    expect(agentIntegrationsPanelSource).not.toContain('Claude Desktop / Claude Code config');
    expect(agentIntegrationsPanelSource).not.toContain('any MCP client that accepts');
    expect(agentIntegrationsPanelSource).not.toContain('getAgentMCPSurfaceToolContract');
    expect(agentIntegrationsPanelSource).not.toContain('subscribe_events');
    expect(agentIntegrationsPanelSource).not.toContain('capability.name');
    expect(agentIntegrationsPanelSource).not.toContain('Interactive questions');
    expect(agentIntegrationsPanelSource).not.toContain('Capability metadata');
    expect(agentIntegrationsPanelSource).not.toContain(
      'monitoring:write for the operator-state write tools',
    );
  });

  it('keeps internal analytics off the user diagnostics boundary', () => {
    expect(diagnosticsResultsPanelSource).not.toContain('Commercial Funnel');
    expect(diagnosticsResultsPanelSource).not.toContain('Infrastructure Onboarding');
    expect(diagnosticsResultsPanelSource).not.toContain('commercialFunnel');
    expect(diagnosticsResultsPanelSource).not.toContain('infrastructureOnboarding');
    expect(diagnosticsResultsPanelSource).not.toContain("apiFetchJSON('/api/diagnostics')");
    expect(diagnosticsResultsPanelSource).toContain('Assistant runtime');
    expect(diagnosticsResultsPanelSource).toContain('assistantRuntimeConnected');
    expect(diagnosticsResultsPanelSource).not.toContain('MCP Connection');
    expect(diagnosticsResultsPanelSource).not.toContain('mcpConnected');

    expect(diagnosticsModelSource).toContain('stripInternalAnalyticsDiagnosticsFields');
    expect(diagnosticsModelSource).toContain('assistantRuntimeConnected');
    expect(diagnosticsModelSource).not.toContain('mcpConnected');
    expect(diagnosticsModelSource).not.toContain('mcpToolCount');
    expect(diagnosticsModelSource).not.toContain('export interface CommercialFunnelDiagnostic');
    expect(diagnosticsModelSource).not.toContain('export interface CommercialFunnelSummary');
    expect(diagnosticsModelSource).not.toContain(
      'export interface InfrastructureOnboardingDiagnostic',
    );
    expect(diagnosticsModelSource).not.toContain(
      'export interface InfrastructureOnboardingSummary',
    );
  });

  it('keeps read-only settings status indicators on the shared badge primitive', () => {
    expect(statusIndicatorBadgeSource).toContain('getSimpleStatusIndicator');
    expect(statusIndicatorBadgeSource).toContain('getStatusIndicatorBadgeToneClasses');
    expect(statusIndicatorBadgeSource).toContain('StatusDot');

    expect(agentProfilesPanelSource).toContain('StatusIndicatorBadge');
    expect(agentProfilesPanelSource).not.toContain('getStatusIndicatorBadgeToneClasses');

    expect(diagnosticsResultsPanelSource).toContain('StatusIndicatorBadge');
    expect(diagnosticsResultsPanelSource).not.toContain('const StatusBadge');
    expect(diagnosticsResultsPanelSource).not.toContain('getStatusIndicatorBadgeToneClasses');
  });
});
