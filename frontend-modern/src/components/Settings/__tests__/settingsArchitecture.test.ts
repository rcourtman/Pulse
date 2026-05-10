import { describe, expect, it } from 'vitest';
import settingsSource from '../Settings.tsx?raw';
import settingsDialogsSource from '../SettingsDialogs.tsx?raw';
import settingsPageShellSource from '../SettingsPageShell.tsx?raw';
import aiSettingsDialogsSource from '../AISettingsDialogs.tsx?raw';
import aiChatMaintenanceSectionSource from '../AIChatMaintenanceSection.tsx?raw';
import aiModelSelectionSectionSource from '../AIModelSelectionSection.tsx?raw';
import aiRuntimeControlsSectionSource from '../AIRuntimeControlsSection.tsx?raw';
import aiSettingsModelSource from '../aiSettingsModel.ts?raw';
import generalSettingsPanelSource from '../GeneralSettingsPanel.tsx?raw';
import dockerRuntimeSettingsCardSource from '../DockerRuntimeSettingsCard.tsx?raw';
import settingsHeaderMetaSource from '../settingsHeaderMeta.ts?raw';
import settingsNavCatalogSource from '../settingsNavCatalog.ts?raw';
import settingsNavigationHookSource from '../useSettingsNavigation.ts?raw';
import aiSettingsStateSource from '../useAISettingsState.ts?raw';
import settingsPanelRegistryContextSource from '../settingsPanelRegistryContext.tsx?raw';
import apiAccessPanelSource from '../APIAccessPanel.tsx?raw';
import apiTokenManagerSource from '../APITokenManager.tsx?raw';
import apiTokenManagerModelSource from '../apiTokenManagerModel.ts?raw';
import apiTokenManagerStateSource from '../useAPITokenManagerState.ts?raw';
import dataHandlingPanelSource from '../DataHandlingPanel.tsx?raw';
import auditLogPanelSource from '../AuditLogPanel.tsx?raw';
import auditWebhookPanelSource from '../AuditWebhookPanel.tsx?raw';
import reportingPanelSource from '../ReportingPanel.tsx?raw';
import rbacFeatureGateSectionSource from '../RBACFeatureGateSection.tsx?raw';
import recoverySettingsPanelSource from '../RecoverySettingsPanel.tsx?raw';
import systemLogsPanelSource from '../SystemLogsPanel.tsx?raw';
import updatesSettingsPanelSource from '../UpdatesSettingsPanel.tsx?raw';
import agentProfilesPanelSource from '../AgentProfilesPanel.tsx?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import infrastructureInstallerSectionSource from '../InfrastructureInstallerSection.tsx?raw';
import infrastructureOperationsModelSource from '../infrastructureOperationsModel.tsx?raw';
import infrastructureSourceManagerSource from '../InfrastructureSourceManager.tsx?raw';
import infrastructureSourcePickerSource from '../InfrastructureSourcePicker.tsx?raw';
import infrastructureWorkspaceModelSource from '../infrastructureWorkspaceModel.ts?raw';
import agentProfileSettingsSource from '../agentProfileSettings.ts?raw';
import connectionsTableSource from '../ConnectionsTable.tsx?raw';
import monitoredSystemImpactPreviewSource from '../MonitoredSystemImpactPreview.tsx?raw';
import connectionEditorSource from '../ConnectionEditor/ConnectionEditor.tsx?raw';
import addressProbeStepSource from '../ConnectionEditor/AddressProbeStep.tsx?raw';
import connectionEditorStateSource from '../ConnectionEditor/useConnectionEditor.ts?raw';
import nodeCredentialSlotSource from '../ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx?raw';
import networkBoundarySettingsSectionSource from '../NetworkBoundarySettingsSection.tsx?raw';
import nodeModalBasicInfoSectionSource from '../NodeModalBasicInfoSection.tsx?raw';
import nodeModalAuthenticationSectionSource from '../NodeModalAuthenticationSection.tsx?raw';
import nodeModalMonitoringSectionSource from '../NodeModalMonitoringSection.tsx?raw';
import nodeModalSetupGuideSectionSource from '../NodeModalSetupGuideSection.tsx?raw';
import nodeModalStatusFooterSource from '../NodeModalStatusFooter.tsx?raw';
import nodeModalStateSource from '../useNodeModalState.ts?raw';
import trueNASCredentialSlotSource from '../ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx?raw';
import vmwareCredentialSlotSource from '../ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx?raw';
import organizationAccessManagementSectionSource from '../OrganizationAccessManagementSection.tsx?raw';
import organizationAccessMembersSectionSource from '../OrganizationAccessMembersSection.tsx?raw';
import organizationSharingCreateSectionSource from '../OrganizationSharingCreateSection.tsx?raw';
import rolesEditorDialogSource from '../RolesEditorDialog.tsx?raw';
import diagnosticsResultsPanelSource from '../DiagnosticsResultsPanel.tsx?raw';
import diagnosticsModelSource from '../diagnosticsModel.ts?raw';
import agentProfilesStateSource from '../useAgentProfilesPanelState.ts?raw';
import auditLogStateSource from '../useAuditLogPanelState.ts?raw';
import auditWebhookStateSource from '../useAuditWebhookPanelState.ts?raw';
import rbacFeatureGateStateSource from '../useRBACFeatureGateState.ts?raw';
import reportingStateSource from '../useReportingPanelState.ts?raw';
import ssoProvidersPanelSource from '../SSOProvidersPanel.tsx?raw';
import ssoProvidersStateSource from '../useSSOProvidersState.ts?raw';
import infrastructureOnboardingPresentationSource from '../../../utils/infrastructureOnboardingPresentation.ts?raw';
import selfHostedBillingPresentationSource from '../selfHostedBillingPresentation.ts?raw';
import systemSettingsPresentationSource from '../../../utils/systemSettingsPresentation.ts?raw';
import auditLogPresentationSource from '../../../utils/auditLogPresentation.ts?raw';

const settingsRuntimeSources = import.meta.glob(['../*.tsx', '../ConnectionEditor/**/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;

describe('settings architecture guardrails', () => {
  it('keeps Settings on the canonical page shell boundary', () => {
    expect(settingsSource).toContain("import { SettingsDialogs } from './SettingsDialogs';");
    expect(settingsSource).toContain("import { SettingsPageShell } from './SettingsPageShell';");
    expect(settingsSource).toContain('const {');
    expect(settingsSource).toContain('useSettingsNavigation({');
    expect(settingsSource).toContain('<SettingsPageShell');
    expect(settingsSource).toContain('<SettingsDialogs');
    expect(settingsSource).not.toContain('<PageHeader');

    expect(settingsPageShellSource).toContain('import { PageHeader } from');
    expect(settingsPageShellSource).toContain(
      '<PageHeader title={props.headerMeta().title} description={props.headerMeta().description} />',
    );
    expect(settingsDialogsSource).toContain('export const SettingsDialogs');
  });

  it('keeps infrastructure onboarding route-backed under the shared settings shell', () => {
    expect(settingsHeaderMetaSource).toContain("'infrastructure-systems': {");
    expect(settingsHeaderMetaSource).toContain(
      'Add, discover, and verify the infrastructure Pulse monitors.',
    );
    expect(settingsHeaderMetaSource).toContain("'organization-access': {");
    expect(settingsHeaderMetaSource).toContain(
      'Manage organization invitations, member roles, and ownership transfers.',
    );

    expect(settingsNavigationHookSource).toContain('deriveAddStepFromLegacyPath(path)');
    expect(settingsNavigationHookSource).toContain(
      'navigate(buildInfrastructureOnboardingPath(infrastructureOnboardingStep), {',
    );
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
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function deriveAddStepFromLegacyPath(',
    );
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function deriveAddStepFromSearch(',
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
      '|| params.securityStatus()?.ssoSessionUsername',
    );
    expect(settingsPanelRegistryContextSource).toContain(
      '|| params.securityStatus()?.authUsername;',
    );
    expect(settingsPanelRegistryContextSource).toContain('getOrganizationOverviewPanelProps');
    expect(settingsPanelRegistryContextSource).toContain('getOrganizationAccessPanelProps');
    expect(settingsPanelRegistryContextSource).toContain('getOrganizationSharingPanelProps');
  });

  it('keeps self-hosted commercial settings plan-owned under one shared presentation model', () => {
    expect(selfHostedBillingPresentationSource).toContain("navLabel: 'Plans'");
    expect(selfHostedBillingPresentationSource).toContain("shellTitle: 'Self-hosted plan'");
    expect(selfHostedBillingPresentationSource).toContain(
      "shellDescription:\n    'Review the plan this instance is using and the optional capabilities connected to it.'",
    );
    expect(selfHostedBillingPresentationSource).toContain("planSectionTitle: 'Current plan'");
    expect(selfHostedBillingPresentationSource).toContain(
      "recoverySectionTitle: 'Existing purchases'",
    );

    expect(settingsNavCatalogSource).toContain(
      'label: SELF_HOSTED_PRO_BILLING_PRESENTATION.navLabel',
    );
    expect(settingsNavCatalogSource).toContain('hideFromSidebar: true');
    expect(settingsNavCatalogSource).toContain("features: ['rbac']");
    expect(settingsNavCatalogSource).toContain("features: ['audit_logging']");
    expect(settingsNavCatalogSource).toContain("features: ['relay']");
    expect(settingsNavCatalogSource).toContain("features: ['advanced_reporting']");
    expect(settingsNavCatalogSource).toContain('hideWhenUnavailable: true');
    expect(settingsHeaderMetaSource).toContain(
      'title: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellTitle',
    );
    expect(settingsHeaderMetaSource).toContain(
      'description: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellDescription',
    );
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
    expect(dataHandlingPanelSource).not.toContain('Pro');
    expect(auditWebhookPanelSource).toContain('getAuditWebhookFeatureGateCopy({');
    expect(auditWebhookPanelSource).toContain('paidRuntimeRequired: paidRuntimeRequired()');
    expect(auditWebhookPanelSource).not.toContain('Audit Webhooks (Pro)');
    expect(reportingPanelSource).toContain('if (!catalog || !state || showUpgradePrompts())');
    expect(reportingPanelSource).toContain('title: catalog.title');
    expect(reportingPanelSource).not.toContain('Advanced Reporting (Pro)');
    expect(aiRuntimeControlsSectionSource).toContain('showAutonomousControlOption');
    expect(aiRuntimeControlsSectionSource).toContain("state.form.controlLevel === 'autonomous'");
    expect(aiRuntimeControlsSectionSource).toContain(
      'Controlled - Pulse Assistant asks before actions',
    );
    expect(aiRuntimeControlsSectionSource).toContain(
      'Autonomous - Pulse Assistant can run eligible actions',
    );
    expect(aiRuntimeControlsSectionSource).not.toContain('without approval (Pro)');
    expect(aiRuntimeControlsSectionSource).not.toContain(
      'Pulse Assistant executes without approval',
    );
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

  it('keeps Patrol tool-call preflight wired through the canonical settings state', () => {
    // The Verify Patrol button must drive the canonical
    // /api/ai/patrol/preflight endpoint via the typed runPatrolPreflight
    // client and surface the result through state, not via inline fetch
    // calls in the section component.
    expect(aiSettingsStateSource).toContain('runPatrolPreflight');
    expect(aiSettingsStateSource).toContain('runPatrolToolPreflight');
    expect(aiSettingsStateSource).toContain('patrolPreflightResult');
    expect(aiSettingsStateSource).toContain('patrolPreflightRunning');
    expect(aiModelSelectionSectionSource).toContain('PatrolPreflightControl');
    expect(aiModelSelectionSectionSource).toContain('runPatrolToolPreflight');
    // The amber soft-warning tone is the operator's signal that the
    // provider accepted the request but the model did not call the tool.
    expect(aiModelSelectionSectionSource).toContain('model_tool_support_unverified');
    expect(aiModelSelectionSectionSource).not.toContain('fetch(\'/api/ai/patrol/preflight');
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
    expect(auditWebhookPanelSource).not.toContain('Audit Webhooks (Pro)');
    expect(reportingPanelSource).not.toContain('Advanced Reporting (Pro)');
    expect(rbacFeatureGateSectionSource).not.toContain('Custom Roles (Pro)');
    expect(rbacFeatureGateSectionSource).not.toContain('Centralized Access Control (Pro)');
    expect(ssoProvidersPanelSource).not.toContain('Add SAML (Pro)');
  });

  it('keeps SAML SSO available without a self-hosted Pro upsell boundary', () => {
    expect(ssoProvidersPanelSource).toContain("openAddModal('saml')");
    expect(ssoProvidersPanelSource).toContain('getSSOProviderAddButtonLabel');
    expect(ssoProvidersPanelSource).toContain('Groups Claim');
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
    expect(dockerRuntimeSettingsCardSource).toContain('DOCKER_UPDATE_ACTIONS_SECTION_TITLE');
    expect(dockerRuntimeSettingsCardSource).toContain('DOCKER_UPDATE_ACTIONS_ENV_VAR');
    expect(dockerRuntimeSettingsCardSource).not.toContain('Container Updates');
    expect(dockerRuntimeSettingsCardSource).not.toContain('container update actions');

    expect(systemSettingsPresentationSource).toContain("getSourcePlatformLabel('docker')");
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

  it('keeps API token Docker and Podman copy on shared presentation helpers', () => {
    expect(apiAccessPanelSource).toContain('API_TOKEN_ACCESS_PANEL_DESCRIPTION');
    expect(apiTokenManagerModelSource).toContain('API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION');
    expect(apiTokenManagerModelSource).toContain('API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION');
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
      updatesSettingsPanelSource,
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
    expect(settingsHeaderMetaSource).toContain(
      'Manage appearance, layout, and default monitoring cadence.',
    );
    expect(generalSettingsPanelSource).not.toContain('Dashboard, Infrastructure');
    expect(networkBoundarySettingsSectionSource).toContain('Pulse URL for Notifications');
    expect(networkBoundarySettingsSectionSource).not.toContain('Dashboard URL for Notifications');
    expect(nodeModalBasicInfoSectionSource).toContain('monitoring views');
    expect(nodeModalBasicInfoSectionSource).not.toContain('dashboards');
    expect(nodeModalMonitoringSectionSource).toContain('trim workload noise');
    expect(nodeModalMonitoringSectionSource).toContain('Existing monitoring readings');
    expect(nodeModalMonitoringSectionSource).not.toContain('dashboard readings');
    expect(recoverySettingsPanelSource).toContain('Required for workload backup status');
    expect(recoverySettingsPanelSource).not.toContain('dashboard backup status');
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
    expect(aiModelSelectionSectionSource).not.toContain('<select');
    expect(aiModelSelectionSectionSource).not.toContain('<optgroup');

    expect(aiSettingsModelSource).toContain('AIProviderTestResult');
    expect(aiSettingsModelSource).toContain('AISettings as AISettingsType');
    expect(aiSettingsModelSource).toContain('export type AIAvailableModel = ModelInfo;');
    expect(aiSettingsStateSource).toContain(
      'const [availableModels, setAvailableModels] = createSignal<ModelInfo[]>([]);',
    );
  });

  it('keeps system AI save feedback tied to provider and Patrol readiness context', () => {
    expect(aiSettingsStateSource).toContain(
      "import { apiErrorDetails } from '@/api/responseUtils';",
    );
    expect(aiSettingsStateSource).toContain('resolveAISettingsSaveProviderFailure');
    expect(aiSettingsStateSource).toContain('getAISettingsPatrolReadinessSaveMessage(');
    expect(aiSettingsStateSource).toContain(
      "getAISettingsPatrolReadinessSaveMessage(\n        updated.patrol_readiness,\n        'Assistant & Patrol enabled',",
    );
    expect(aiSettingsStateSource).toContain('notificationStore.warning(patrolReadinessMessage);');
    expect(aiSettingsStateSource).toContain('getAISettingsSaveProviderFailureMessage(');
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
    expect(infrastructureWorkspaceSource).toContain(
      ": (type) => openAddFlow(type === 'agent' ? 'agent' : (type as ManagedAddTypeStep))",
    );
    expect(infrastructureWorkspaceSource).toContain('reviewDiscoveredSource');
    expect(infrastructureWorkspaceSource).toContain('selectedDiscoveredSource');
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
    // Discovery actions are now icon-only ghost buttons; the human copy
    // still lives in their title and aria-label attributes.
    expect(infrastructureSourceManagerSource).toContain('Run discovery');
    expect(infrastructureSourceManagerSource).toContain('discovery settings');
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
    expect(infrastructureSourceManagerSource).not.toContain("'Install agent'");
    expect(infrastructureSourceManagerSource).toContain('Add infrastructure');
    expect(infrastructureSourceManagerSource).toContain('getInfrastructureEmptyStateSummary');
    expect(infrastructureSourceManagerSource).toContain('Setup status');
    expect(infrastructureSourceManagerSource).toContain('Systems');
    expect(infrastructureSourceManagerSource).toContain('Live');
    expect(infrastructureSourceManagerSource).toContain('Needs attention');
    expect(infrastructureSourceManagerSource).toContain('Needs agent');
    expect(infrastructureSourceManagerSource).toContain('setupConfidenceAction');
    expect(infrastructureSourceManagerSource).not.toContain('Infrastructure coverage');
    expect(infrastructureSourceManagerSource).not.toContain('Fleet governance');
    expect(infrastructureSourceManagerSource).not.toContain('Connection types');
    expect(infrastructureSourcePickerSource).toContain('Detect API platform');
    expect(infrastructureSourcePickerSource).toContain('getInfrastructureSourcePickerItems');
    expect(infrastructureSourcePickerSource).toContain('itemMatchesQuery');
    expect(infrastructureSourcePickerSource).toContain('catalogDescription');
    expect(infrastructureSourcePickerSource).not.toContain(
      'getInfrastructureSourceStrategyPresentation',
    );
    expect(settingsHeaderMetaSource).toContain(
      "description: 'Configure the public URL, CORS, embedding, and webhook network boundaries.'",
    );
  });

  it('keeps the detect-first editor and inline credential bodies on the shared editor model', () => {
    expect(connectionEditorSource).toContain(
      "import { AddressProbeStep } from './AddressProbeStep';",
    );
    expect(connectionEditorSource).toContain(
      "from '@/utils/infrastructureOnboardingPresentation';",
    );
    expect(connectionsTableSource).toContain(
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
      'Supported source types include VMware vCenter',
    );

    expect(nodeCredentialSlotSource).toContain('useNodeModalState(modalProps)');
    expect(nodeCredentialSlotSource).toContain('<NodeModalBasicInfoSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalAuthenticationSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalMonitoringSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalStatusFooter');
    expect(nodeCredentialSlotSource).not.toContain('<Dialog');
    expect(nodeModalAuthenticationSectionSource).toContain(
      "state.formData().setupMode === 'manual'",
    );
    expect(nodeModalAuthenticationSectionSource).toContain('Advanced manual token path');
    expect(nodeModalSetupGuideSectionSource).toContain('Source strategy');
    expect(nodeModalSetupGuideSectionSource).toContain('Host telemetry agent');
    expect(nodeModalSetupGuideSectionSource).toContain('API inventory');
    expect(nodeModalSetupGuideSectionSource).toContain('Manual API token');
    expect(nodeModalSetupGuideSectionSource).toContain('Recommended least-privilege path');
    expect(nodeModalStatusFooterSource).toContain('guidedSetupOnlyMode');
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

  it('keeps internal analytics off the user diagnostics boundary', () => {
    expect(diagnosticsResultsPanelSource).not.toContain('Commercial Funnel');
    expect(diagnosticsResultsPanelSource).not.toContain('Infrastructure Onboarding');
    expect(diagnosticsResultsPanelSource).not.toContain('commercialFunnel');
    expect(diagnosticsResultsPanelSource).not.toContain('infrastructureOnboarding');
    expect(diagnosticsResultsPanelSource).not.toContain("apiFetchJSON('/api/diagnostics')");

    expect(diagnosticsModelSource).toContain('stripInternalAnalyticsDiagnosticsFields');
    expect(diagnosticsModelSource).not.toContain('export interface CommercialFunnelDiagnostic');
    expect(diagnosticsModelSource).not.toContain('export interface CommercialFunnelSummary');
    expect(diagnosticsModelSource).not.toContain(
      'export interface InfrastructureOnboardingDiagnostic',
    );
    expect(diagnosticsModelSource).not.toContain(
      'export interface InfrastructureOnboardingSummary',
    );
  });
});
