import { describe, expect, it } from 'vitest';
import settingsSource from '../Settings.tsx?raw';
import settingsDialogsSource from '../SettingsDialogs.tsx?raw';
import settingsPageShellSource from '../SettingsPageShell.tsx?raw';
import settingsHeaderMetaSource from '../settingsHeaderMeta.ts?raw';
import settingsNavCatalogSource from '../settingsNavCatalog.ts?raw';
import settingsNavigationHookSource from '../useSettingsNavigation.ts?raw';
import settingsPanelRegistryContextSource from '../settingsPanelRegistryContext.tsx?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import infrastructureSourceManagerSource from '../InfrastructureSourceManager.tsx?raw';
import infrastructureSourcePickerSource from '../InfrastructureSourcePicker.tsx?raw';
import infrastructureWorkspaceModelSource from '../infrastructureWorkspaceModel.ts?raw';
import connectionsTableSource from '../ConnectionsTable.tsx?raw';
import connectionEditorSource from '../ConnectionEditor/ConnectionEditor.tsx?raw';
import addressProbeStepSource from '../ConnectionEditor/AddressProbeStep.tsx?raw';
import connectionEditorStateSource from '../ConnectionEditor/useConnectionEditor.ts?raw';
import nodeCredentialSlotSource from '../ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx?raw';
import trueNASCredentialSlotSource from '../ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx?raw';
import vmwareCredentialSlotSource from '../ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx?raw';
import diagnosticsResultsPanelSource from '../DiagnosticsResultsPanel.tsx?raw';
import diagnosticsModelSource from '../diagnosticsModel.ts?raw';
import infrastructureOnboardingPresentationSource from '../../../utils/infrastructureOnboardingPresentation.ts?raw';
import selfHostedBillingPresentationSource from '../selfHostedBillingPresentation.ts?raw';

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
    expect(settingsHeaderMetaSource).toContain('Manage infrastructure sources.');
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
    expect(selfHostedBillingPresentationSource).toContain("shellTitle: 'Plans & Activation'");
    expect(selfHostedBillingPresentationSource).toContain(
      "shellDescription:\n    'Review your current self-hosted plan, activation status, and unlocked capabilities.'",
    );
    expect(selfHostedBillingPresentationSource).toContain("planSectionTitle: 'Current plan'");
    expect(selfHostedBillingPresentationSource).toContain(
      "recoverySectionTitle: 'Activation & Recovery'",
    );

    expect(settingsNavCatalogSource).toContain(
      'label: SELF_HOSTED_PRO_BILLING_PRESENTATION.navLabel',
    );
    expect(settingsHeaderMetaSource).toContain(
      'title: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellTitle',
    );
    expect(settingsHeaderMetaSource).toContain(
      'description: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellDescription',
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
    expect(infrastructureWorkspaceSource).toContain('trackInitialCatalogSelection={');
    expect(infrastructureWorkspaceSource).toContain(
      "onDetectFromAddress={() => openAddFlow('detect')}",
    );
    expect(infrastructureWorkspaceSource).toContain("onBackToCatalog={() => openAddFlow('pick')}");
    expect(infrastructureWorkspaceSource).toContain('recordCatalogSelection(type);');
    expect(infrastructureWorkspaceSource).toContain('renderAgentConnectionDetails');
    expect(infrastructureWorkspaceSource).not.toContain('InfrastructureOperationsController');
    expect(infrastructureWorkspaceSource).not.toContain('PlatformConnectionsWorkspace');
    expect(infrastructureSourceManagerSource).toContain('Infrastructure systems');
    expect(infrastructureSourceManagerSource).toContain('Run discovery');
    expect(infrastructureSourceManagerSource).toContain('Discovery settings');
    expect(infrastructureSourceManagerSource).toContain(
      'Configured systems and discovered candidates grouped by platform or host type.',
    );
    expect(infrastructureSourceManagerSource).toContain('onReviewDiscoveredSource');
    expect(infrastructureSourceManagerSource).toContain('Discovered');
    expect(infrastructureSourceManagerSource).toContain('getInfrastructureSourceManagerProducts');
    expect(infrastructureSourceManagerSource).toContain('TableHeader');
    expect(infrastructureSourceManagerSource).toContain('aria-label={product.actionLabel}');
    expect(infrastructureSourceManagerSource).toContain('Review');
    expect(infrastructureSourceManagerSource).toContain('Edit');
    expect(infrastructureSourceManagerSource).not.toContain('Detect from address');
    expect(infrastructureSourceManagerSource).not.toContain('Connection types');
    expect(infrastructureSourcePickerSource).toContain('Choose a source type');
    expect(infrastructureSourcePickerSource).toContain('Detect from address');
    expect(infrastructureSourcePickerSource).toContain('getInfrastructureSourcePickerGroups');
    expect(infrastructureSourcePickerSource).toContain('group.label');
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
    expect(connectionEditorSource).toContain('Detect from address');
    expect(connectionEditorSource).toContain('Address probe');
    expect(connectionEditorSource).toContain('flex h-full min-h-0 flex-col');
    expect(connectionEditorSource).toContain('Back to source types');
    expect(connectionEditorSource).toContain('Back to detect');
    expect(connectionEditorSource).toContain('What happens next');
    expect(connectionEditorSource).not.toContain('buildConnectionEditorCatalogEntries');
    expect(connectionEditorSource).not.toContain('selectedFamilyId');
    expect(connectionEditorSource).not.toContain('Choose how Pulse should connect');
    expect(connectionEditorSource).not.toContain('Connect a supported platform');
    expect(connectionEditorSource).not.toContain('Choose a {family.label} product');
    expect(connectionEditorSource).not.toContain('Back to platforms');
    expect(connectionEditorSource).not.toContain('NodeModal');

    expect(addressProbeStepSource).toContain('Probe address');
    expect(addressProbeStepSource).toContain('install Pulse Agent instead');
    expect(addressProbeStepSource).toContain('Choose a source type instead');
    expect(addressProbeStepSource).toContain('bare-metal Linux');
    expect(addressProbeStepSource).toContain('supported API-backed platform');

    expect(connectionEditorStateSource).toContain('ConnectionsAPI.probe(value)');
    expect(connectionEditorStateSource).toContain('export const CONNECTION_TYPE_LABELS');
    expect(connectionEditorStateSource).not.toContain('DEFAULT_CONNECTION_EDITOR_CATALOG_ENTRIES');
    expect(connectionEditorStateSource).not.toContain('buildConnectionEditorCatalogEntries');
    expect(connectionEditorStateSource).not.toContain('getSourcePlatformFamily');
    expect(infrastructureOnboardingPresentationSource).toContain('getSourcePlatformManifestEntry');
    expect(infrastructureOnboardingPresentationSource).toContain(
      'getInfrastructureSourcePickerGroups',
    );
    expect(infrastructureOnboardingPresentationSource).toContain(
      'getInfrastructureSupportSummaryBadges',
    );
    expect(infrastructureOnboardingPresentationSource).toContain(
      'VMware vCenter is also available now.',
    );

    expect(nodeCredentialSlotSource).toContain('useNodeModalState(modalProps)');
    expect(nodeCredentialSlotSource).toContain('<NodeModalBasicInfoSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalAuthenticationSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalMonitoringSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalStatusFooter');
    expect(nodeCredentialSlotSource).not.toContain('<Dialog');

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

  it('keeps diagnostics funnel rendering on the shared results/model boundary', () => {
    expect(diagnosticsResultsPanelSource).toContain('Commercial Funnel');
    expect(diagnosticsResultsPanelSource).toContain('Infrastructure Onboarding');
    expect(diagnosticsResultsPanelSource).toContain('titleCaseDelimitedLabel');
    expect(diagnosticsResultsPanelSource).not.toContain("apiFetchJSON('/api/diagnostics')");

    expect(diagnosticsModelSource).toContain('export interface CommercialFunnelDiagnostic');
    expect(diagnosticsModelSource).toContain('export interface CommercialFunnelSummary');
    expect(diagnosticsModelSource).toContain('export interface InfrastructureOnboardingDiagnostic');
    expect(diagnosticsModelSource).toContain('export interface InfrastructureOnboardingSummary');
  });
});
