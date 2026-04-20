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
    expect(settingsNavigationHookSource).toContain('navigate(buildInfrastructureWorkspacePath(), {');
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
    expect(infrastructureWorkspaceSource).toContain('navigate(buildInfrastructureWorkspacePath(), { replace: true });');
    expect(infrastructureWorkspaceSource).toContain('const [, setSearchParams] = useSearchParams();');
    expect(infrastructureWorkspaceSource).toContain(
      'setSearchParams({ [INFRASTRUCTURE_ADD_QUERY_PARAM]: null }, { replace: true });',
    );
    expect(infrastructureWorkspaceSource).not.toContain('InfrastructureOperationsController');
    expect(infrastructureWorkspaceSource).not.toContain('PlatformConnectionsWorkspace');
    expect(infrastructureWorkspaceSource).not.toContain('NodeModal');
    expect(infrastructureWorkspaceSource).not.toContain('layout="drawer-right"');
  });

  it('keeps probe-first connection setup and inline node credentials on the shared editor model', () => {
    expect(connectionEditorSource).toContain("import { AddressProbeStep } from './AddressProbeStep';");
    // Platform integrations render as peer tiles; the agent lives in its own
    // section below so it can explain what host-level telemetry adds instead
    // of being mistaken for one more peer of Proxmox / VMware / TrueNAS.
    expect(connectionEditorSource).toContain('const DEFAULT_PLATFORM_TYPES: ConnectionType[] =');
    expect(connectionEditorSource).not.toContain("'agent'] =");
    expect(connectionEditorSource).toContain('<AddressProbeStep');
    expect(connectionEditorSource).toContain('Or connect a platform API directly');
    // The agent card is the *primary* path on the Add landing — it sits above
    // the probe input AND the platform grid so a user doesn't read the probe
    // as the lead action. The probe + platforms collapse into one "connect
    // via API instead" fallback block below the agent. Guarding ordering
    // here prevents regression to "agent as an alternative to the probe."
    const agentPrimaryIdx = connectionEditorSource.indexOf('On a Proxmox host, this is the');
    const apiSectionIdx = connectionEditorSource.indexOf('Or connect a platform API directly');
    const probeIdx = connectionEditorSource.indexOf('<AddressProbeStep');
    expect(agentPrimaryIdx).toBeGreaterThan(-1);
    expect(apiSectionIdx).toBeGreaterThan(-1);
    expect(probeIdx).toBeGreaterThan(-1);
    expect(agentPrimaryIdx).toBeLessThan(apiSectionIdx);
    expect(apiSectionIdx).toBeLessThan(probeIdx);
    expect(connectionEditorSource).not.toContain('Or install the agent on the host');
    expect(connectionEditorSource).toContain('auto-registers the node');
    expect(connectionEditorSource).toContain('Recommended');
    // The agent install path is a first-class ledger-header action, not a
    // subtext offramp inside the editor — make sure it doesn't drift back.
    expect(connectionEditorSource).not.toContain('Install the Unified Agent on a host');
    expect(connectionEditorSource).not.toContain('NodeModal');

    expect(addressProbeStepSource).toContain('Probe address');
    // The no-match branch must name the agent alternative so a user who
    // probed bare-metal Linux / Unraid / FreeBSD is not left in a
    // Platform-API-only dead end.
    expect(addressProbeStepSource).toContain('install the Unified Agent instead');
    expect(addressProbeStepSource).toContain('bare-metal Linux');

    expect(connectionEditorStateSource).toContain('ConnectionsAPI.probe(value)');
    expect(connectionEditorStateSource).toContain('export const CONNECTION_TYPE_LABELS');

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
