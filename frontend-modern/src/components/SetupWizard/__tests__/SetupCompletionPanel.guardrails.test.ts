import { describe, expect, it } from 'vitest';
import setupCompletionPanelSource from '../SetupCompletionPanel.tsx?raw';
import setupCompletionModelSource from '../setupCompletionModel.ts?raw';

describe('SetupCompletionPanel guardrails', () => {
  it('keeps setup completion aligned with the canonical add-infrastructure picker', () => {
    expect(setupCompletionPanelSource).toContain('buildInfrastructureOnboardingPath');
    expect(setupCompletionPanelSource).toContain(
      "const ADD_INFRASTRUCTURE_PATH = buildInfrastructureOnboardingPath('pick');",
    );
    expect(setupCompletionPanelSource).toContain(
      "const AGENT_INSTALL_PATH = buildInfrastructureOnboardingPath('agent');",
    );
    expect(setupCompletionPanelSource).toContain('Add infrastructure');
    expect(setupCompletionPanelSource).toContain('Install Pulse Agent');
    expect(setupCompletionPanelSource).toContain('Credentials you must save now');
    expect(setupCompletionPanelSource).toContain('Shown during setup');
    expect(setupCompletionPanelSource).toContain('props.onComplete(ADD_INFRASTRUCTURE_PATH);');
    expect(setupCompletionPanelSource).toContain('props.onComplete(AGENT_INSTALL_PATH);');
    expect(setupCompletionPanelSource).toContain(
      'Use Add infrastructure to choose a platform API, Pulse Agent, or both',
    );
    expect(setupCompletionPanelSource).toContain(
      'Open the picker to choose a platform API for inventory, Pulse Agent for host telemetry',
    );
    expect(setupCompletionPanelSource).not.toContain('Use Add connection to connect');
    expect(setupCompletionPanelSource).not.toContain("from '@/stores/licenseCommercial';");
    expect(setupCompletionPanelSource).not.toContain('runStartProTrialAction');
    expect(setupCompletionPanelSource).not.toContain('loadCommercialPosture');
    expect(setupCompletionPanelSource).not.toContain('isCommercialTrialActive');
    expect(setupCompletionPanelSource).not.toContain('Monitor from Anywhere');
    expect(setupCompletionPanelSource).not.toContain("trackPaywallViewed('relay'");
    expect(setupCompletionPanelSource).not.toContain("trackUpgradeClicked('setup_wizard'");
    expect(setupCompletionPanelSource).not.toContain('getUpgradeActionUrlOrFallback');
  });

  it('describes setup completion through the unified resource model instead of legacy install-command copy', () => {
    expect(setupCompletionPanelSource).toContain("title: 'What happens next'");
    expect(setupCompletionPanelSource).toContain('Pulse is now secured.');
    expect(setupCompletionPanelSource).toContain("title: 'Open Add infrastructure'");
    expect(setupCompletionPanelSource).toContain('Review the supported source types in one place');
    expect(setupCompletionPanelSource).toContain("title: 'Save the source and confirm coverage'");
    expect(setupCompletionPanelSource).toContain('What to expect');
    expect(setupCompletionPanelSource).toContain('First system first');
    expect(setupCompletionPanelSource).toContain(
      'Start with one source, then add more systems later from Settings',
    );
    expect(setupCompletionPanelSource).toContain(
      'Platform APIs own inventory and health. Pulse Agent owns host telemetry',
    );
    expect(setupCompletionModelSource).toContain(
      'Start with a platform API when a platform manages the estate.',
    );
    expect(setupCompletionPanelSource).not.toContain('Smart Auto-Detection');
    expect(setupCompletionPanelSource).not.toContain('Agent Metrics');
    expect(setupCompletionPanelSource).not.toContain('ProxmoxIcon');
  });

  it('keeps connected infrastructure classification on the canonical setup model', () => {
    expect(setupCompletionPanelSource).toContain('buildSetupCompletionConnectedSystems');
    expect(setupCompletionPanelSource).toContain('buildSetupCompletionViewModel');
    expect(setupCompletionPanelSource).toContain('props.connectedResourcesOverride !== undefined');
    expect(setupCompletionPanelSource).toContain(
      'setConnectedSystems(buildSetupCompletionConnectedSystems(props.connectedResourcesOverride));',
    );
    expect(setupCompletionModelSource).toContain('isAgentFacetInfrastructureResource');
    expect(setupCompletionModelSource).toContain('getPreferredInfrastructureDisplayName');
    expect(setupCompletionModelSource).toContain('getPreferredResourceHostname');
    expect(setupCompletionModelSource).toContain('getSourcePlatformManifestEntry');
    expect(setupCompletionModelSource).toContain('sourcePlatformSupportsOnboardingPath');
    expect(setupCompletionModelSource).toContain('displayTokens[displayTokens.length - 1]');
    expect(setupCompletionModelSource).not.toContain('PLATFORM_CONNECTION_PLATFORM_TYPES');
    expect(setupCompletionModelSource).not.toContain("resource.type === 'truenas'");
    expect(setupCompletionModelSource).not.toContain('getPreferredResourceDisplayName(resource)');
  });

  it('does not reintroduce a separate setup-wizard install command surface', () => {
    expect(setupCompletionPanelSource).not.toContain('buildUnixAgentInstallCommand');
    expect(setupCompletionPanelSource).not.toContain('buildWindowsAgentInstallCommand');
    expect(setupCompletionPanelSource).not.toContain('SecurityAPI');
    expect(setupCompletionPanelSource).not.toContain('SecurityStatus');
    expect(setupCompletionPanelSource).not.toContain('Connection URL (Agent → Pulse)');
    expect(setupCompletionPanelSource).not.toContain('Custom CA certificate path (optional)');
    expect(setupCompletionPanelSource).not.toContain('Windows (PowerShell as Administrator)');
    expect(setupCompletionPanelSource).not.toContain('Confirm without token');
    expect(setupCompletionPanelSource).not.toContain('Current Install Token');
    expect(setupCompletionPanelSource).not.toContain('Refresh Token');
    expect(setupCompletionPanelSource).not.toContain('Preview copied commands');
  });

  it('keeps setup completion on one primary next-step surface instead of repeated CTA sections', () => {
    expect(setupCompletionPanelSource).toContain(
      'const [showCredentials, setShowCredentials] = createSignal(true);',
    );
    expect(setupCompletionPanelSource).toContain(
      'Save the admin login and API token before leaving this screen',
    );
    expect(setupCompletionPanelSource).toContain('Recommended next step');
    expect(setupCompletionPanelSource).toContain('Go to Dashboard');
    expect(setupCompletionModelSource).toContain("heroTitle: 'First monitored system connected'");
    expect(setupCompletionModelSource).toContain(
      "heroTitle: 'Choose your first infrastructure source'",
    );
    expect(setupCompletionPanelSource).toContain(
      "completionViewModel().primaryAction === 'dashboard'",
    );
    expect(setupCompletionPanelSource).toContain(
      'completionViewModel().showAddInfrastructureAction',
    );
    expect(setupCompletionPanelSource).toContain('completionViewModel().showAgentInstallAction');
    expect(setupCompletionPanelSource).toContain('handleOpenAddInfrastructure');
    expect(setupCompletionPanelSource).not.toContain('hasConnectedAgents');
    expect(setupCompletionPanelSource).not.toContain('connectedAgents().length');
    expect(setupCompletionPanelSource).not.toContain(
      'You can return here later from Connections & Inventory if you skip install for now.',
    );
    expect(setupCompletionPanelSource).toContain(
      'Add infrastructure now owns the first source decision.',
    );
    expect(setupCompletionPanelSource).toContain(
      'then return to Add infrastructure when you want to connect the next API or Agent source.',
    );
  });
});
