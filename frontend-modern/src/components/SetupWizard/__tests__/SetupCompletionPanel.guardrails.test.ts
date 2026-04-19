import { describe, expect, it } from 'vitest';
import setupCompletionPanelSource from '../SetupCompletionPanel.tsx?raw';
import setupCompletionModelSource from '../setupCompletionModel.ts?raw';

describe('SetupCompletionPanel guardrails', () => {
  it('keeps setup completion aligned with the canonical infrastructure install workspace', () => {
    expect(setupCompletionPanelSource).toContain('buildInfrastructureOnboardingPath');
    expect(setupCompletionPanelSource).toContain(
      "const INFRASTRUCTURE_INSTALL_PATH = buildInfrastructureOnboardingPath('agent');",
    );
    expect(setupCompletionPanelSource).toContain(
      "const PLATFORM_CONNECTIONS_PATH = buildInfrastructureOnboardingPath('pick');",
    );
    expect(setupCompletionPanelSource).toContain('Open Infrastructure Install');
    expect(setupCompletionPanelSource).toContain('Open Platform connections');
    expect(setupCompletionPanelSource).toContain('Credentials you must save now');
    expect(setupCompletionPanelSource).toContain('Shown during setup');
    expect(setupCompletionPanelSource).toContain('props.onComplete(INFRASTRUCTURE_INSTALL_PATH);');
    expect(setupCompletionPanelSource).toContain('props.onComplete(PLATFORM_CONNECTIONS_PATH);');
    expect(setupCompletionPanelSource).toContain(
      'Use Add connection to connect your first host or API-backed platform',
    );
    expect(setupCompletionPanelSource).toContain(
      'continue with the first-host install token Pulse prepares from setup',
    );
    expect(setupCompletionPanelSource).not.toContain(
      'Use Add infrastructure to connect your first host or API-backed platform',
    );
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
    expect(setupCompletionPanelSource).toContain("title: 'Open Infrastructure Install'");
    expect(setupCompletionPanelSource).toContain(
      'Pulse prepares the first-host install token from setup',
    );
    expect(setupCompletionPanelSource).toContain("title: 'Run it on the first host you want to monitor'");
    expect(setupCompletionPanelSource).toContain('What to expect');
    expect(setupCompletionPanelSource).toContain('First system first');
    expect(setupCompletionPanelSource).toContain('Start with one host, then add more systems later from the same install workspace.');
    expect(setupCompletionPanelSource).toContain(
      'API-backed platforms like Proxmox, TrueNAS, and VMware use Platform connections instead of a dedicated install profile in Infrastructure Install.',
    );
    expect(setupCompletionModelSource).toContain(
      'If the first system is API-backed, use Platform connections instead of starting with host install.',
    );
    expect(setupCompletionPanelSource).not.toContain('Smart Auto-Detection');
    expect(setupCompletionPanelSource).not.toContain('Agent Metrics');
    expect(setupCompletionPanelSource).not.toContain('ProxmoxIcon');
  });

  it('keeps connected infrastructure classification on the canonical setup model', () => {
    expect(setupCompletionPanelSource).toContain('buildSetupCompletionConnectedSystems');
    expect(setupCompletionPanelSource).toContain('buildSetupCompletionViewModel');
    expect(setupCompletionPanelSource).toContain("props.connectedResourcesOverride !== undefined");
    expect(setupCompletionPanelSource).toContain("setConnectedSystems(buildSetupCompletionConnectedSystems(props.connectedResourcesOverride));");
    expect(setupCompletionModelSource).toContain('isAgentFacetInfrastructureResource');
    expect(setupCompletionModelSource).toContain('getPreferredInfrastructureDisplayName');
    expect(setupCompletionModelSource).toContain('getPreferredResourceHostname');
    expect(setupCompletionModelSource).toContain('getSourcePlatformManifestEntry');
    expect(setupCompletionModelSource).toContain("sourcePlatformSupportsOnboardingPath");
    expect(setupCompletionModelSource).toContain("displayTokens[displayTokens.length - 1]");
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
    expect(setupCompletionPanelSource).toContain("const [showCredentials, setShowCredentials] = createSignal(true);");
    expect(setupCompletionPanelSource).toContain('Save the admin login and API token before leaving this screen');
    expect(setupCompletionPanelSource).toContain('Recommended next step');
    expect(setupCompletionPanelSource).toContain('Go to Dashboard');
    expect(setupCompletionModelSource).toContain("heroTitle: 'First monitored system connected'");
    expect(setupCompletionModelSource).toContain("heroTitle: 'Connect your first monitored system'");
    expect(setupCompletionPanelSource).toContain(
      "completionViewModel().primaryAction === 'dashboard'",
    );
    expect(setupCompletionPanelSource).toContain(
      'completionViewModel().showPlatformConnectionsAction',
    );
    expect(setupCompletionPanelSource).toContain('completionViewModel().showInstallAction');
    expect(setupCompletionPanelSource).toContain('handleOpenPlatformConnections');
    expect(setupCompletionPanelSource).not.toContain('hasConnectedAgents');
    expect(setupCompletionPanelSource).not.toContain('connectedAgents().length');
    expect(setupCompletionPanelSource).not.toContain(
      'You can return here later from Connections & Inventory if you skip install for now.',
    );
    expect(setupCompletionPanelSource).toContain(
      'The canonical install flow now lives in Infrastructure.',
    );
    expect(setupCompletionPanelSource).toContain(
      'then return to Infrastructure when you want to continue with the next system path.',
    );
  });
});
