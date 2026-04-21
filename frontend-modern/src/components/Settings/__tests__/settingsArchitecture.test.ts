import { describe, expect, it } from 'vitest';
import settingsSource from '../Settings.tsx?raw';
import settingsDialogsSource from '../SettingsDialogs.tsx?raw';
import settingsPageShellSource from '../SettingsPageShell.tsx?raw';
import settingsHeaderMetaSource from '../settingsHeaderMeta.ts?raw';
import settingsNavigationHookSource from '../useSettingsNavigation.ts?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import infrastructureWorkspaceModelSource from '../infrastructureWorkspaceModel.ts?raw';
import connectionEditorSource from '../ConnectionEditor/ConnectionEditor.tsx?raw';
import addressProbeStepSource from '../ConnectionEditor/AddressProbeStep.tsx?raw';
import connectionEditorStateSource from '../ConnectionEditor/useConnectionEditor.ts?raw';
import nodeCredentialSlotSource from '../ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx?raw';
import diagnosticsResultsPanelSource from '../DiagnosticsResultsPanel.tsx?raw';
import diagnosticsModelSource from '../diagnosticsModel.ts?raw';

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
      'Review monitored systems and add new infrastructure to Pulse',
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

  it('keeps the infrastructure add flow inline on ConnectionEditor instead of retired overlays', () => {
    expect(infrastructureWorkspaceSource).toContain(
      "import { ConnectionEditor } from './ConnectionEditor/ConnectionEditor';",
    );
    expect(infrastructureWorkspaceSource).toContain('NodeCredentialSlot');
    expect(infrastructureWorkspaceSource).toContain('TrueNASCredentialSlot');
    expect(infrastructureWorkspaceSource).toContain('VMwareCredentialSlot');
    expect(infrastructureWorkspaceSource).toContain(
      'navigate(buildInfrastructureWorkspacePath(), { replace: true });',
    );
    expect(infrastructureWorkspaceSource).toContain(
      'const [, setSearchParams] = useSearchParams();',
    );
    expect(infrastructureWorkspaceSource).toContain(
      'setSearchParams({ [INFRASTRUCTURE_ADD_QUERY_PARAM]: null }, { replace: true });',
    );
    expect(infrastructureWorkspaceSource).not.toContain('InfrastructureOperationsController');
    expect(infrastructureWorkspaceSource).not.toContain('PlatformConnectionsWorkspace');
    expect(infrastructureWorkspaceSource).not.toContain('NodeModal');
    expect(infrastructureWorkspaceSource).not.toContain('layout="drawer-right"');
  });

  it('keeps the platform-first add landing and inline node credentials on the shared editor model', () => {
    expect(connectionEditorSource).toContain(
      "import { AddressProbeStep } from './AddressProbeStep';",
    );
    expect(connectionEditorSource).toContain('buildConnectionEditorCatalogEntries');
    expect(connectionEditorSource).toContain('selectedFamilyId');
    expect(connectionEditorSource).toContain('<AddressProbeStep');
    expect(connectionEditorSource).toContain('Connect a platform');
    expect(connectionEditorSource).toContain('Choose a {family.label} product');
    expect(connectionEditorSource).toContain('Back to platforms');
    expect(connectionEditorSource).toContain('Install on a host instead');
    expect(connectionEditorSource).toContain('other supported services');
    expect(connectionEditorSource).toContain('connects them when available');
    expect(connectionEditorSource).not.toContain('On supported Proxmox hosts');
    expect(connectionEditorSource).not.toContain('register them automatically');
    expect(connectionEditorSource).not.toContain('auto-registers the node');
    expect(connectionEditorSource).not.toContain('Recommended');
    expect(connectionEditorSource).not.toContain('NodeModal');

    expect(addressProbeStepSource).toContain('Probe address');
    // The no-match branch must name the agent alternative so a user who
    // probed bare-metal Linux / Unraid / FreeBSD is not left in a
    // Platform-API-only dead end.
    expect(addressProbeStepSource).toContain('install the Unified Agent instead');
    expect(addressProbeStepSource).toContain('bare-metal Linux');

    expect(connectionEditorStateSource).toContain('ConnectionsAPI.probe(value)');
    expect(connectionEditorStateSource).toContain('export const CONNECTION_TYPE_LABELS');
    expect(connectionEditorStateSource).toContain('DEFAULT_INFRASTRUCTURE_SOURCE_ORDER');
    expect(connectionEditorStateSource).toContain('getSourcePlatformFamily');
    expect(connectionEditorStateSource).toContain(
      'export const DEFAULT_CONNECTION_EDITOR_CATALOG_ENTRIES',
    );
    expect(connectionEditorStateSource).toContain(
      'export function buildConnectionEditorCatalogEntries',
    );
    expect(connectionEditorStateSource).not.toContain('PROXMOX_FAMILY_TYPES');

    expect(nodeCredentialSlotSource).toContain('useNodeModalState(modalProps)');
    expect(nodeCredentialSlotSource).toContain('<NodeModalBasicInfoSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalAuthenticationSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalMonitoringSection');
    expect(nodeCredentialSlotSource).toContain('<NodeModalStatusFooter');
    expect(nodeCredentialSlotSource).not.toContain('<Dialog');
  });

  it('keeps diagnostics commercial funnel rendering on the shared results/model boundary', () => {
    expect(diagnosticsResultsPanelSource).toContain('Commercial Funnel');
    expect(diagnosticsResultsPanelSource).toContain('titleCaseDelimitedLabel');
    expect(diagnosticsResultsPanelSource).not.toContain("apiFetchJSON('/api/diagnostics')");

    expect(diagnosticsModelSource).toContain('export interface CommercialFunnelDiagnostic');
    expect(diagnosticsModelSource).toContain('export interface CommercialFunnelSummary');
  });
});
