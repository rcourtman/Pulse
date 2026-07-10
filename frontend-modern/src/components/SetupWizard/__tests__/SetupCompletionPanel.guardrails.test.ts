import { describe, expect, it } from 'vitest';
import setupCompletionPanelSource from '../SetupCompletionPanel.tsx?raw';
import setupCompletionModelSource from '../setupCompletionModel.ts?raw';
import runtimeHomeSource from '@/pages/RuntimeHome.tsx?raw';
import securityStepSource from '../steps/SecurityStep.tsx?raw';
import setupWizardSource from '../SetupWizard.tsx?raw';
import stepIndicatorSource from '../StepIndicator.tsx?raw';
import welcomeStepSource from '../steps/WelcomeStep.tsx?raw';

const migratedFirstSessionCopy = [
  'Opening workspace...',
  'Welcome to Pulse',
  'Unlock this Pulse server',
  'Create the admin account',
  'Choose the first source',
  'Unlock setup',
  'What this token does',
  'Paste your bootstrap token',
  'Verify bootstrap token',
  'Continue to Security',
  'Create Account & Continue',
  'On the next screen',
  'Choose your first infrastructure source',
  'First monitored system connected',
  'Credentials you must save now',
  'Shown during setup',
  'Save the admin login and API token before leaving this screen',
  'Recommended next step',
  'Source choices',
  'Inventory and health from Proxmox, TrueNAS, VMware, PBS, or PMG.',
  'Node-local telemetry for standalone hosts, services, Docker, and Kubernetes.',
  'Open Add infrastructure to choose a platform API, Pulse Agent, or both.',
];

const migratedSources = [
  setupCompletionPanelSource,
  setupCompletionModelSource,
  runtimeHomeSource,
  securityStepSource,
  setupWizardSource,
  stepIndicatorSource,
  welcomeStepSource,
];

describe('SetupCompletionPanel guardrails', () => {
  it('keeps setup completion aligned with the canonical add-infrastructure picker', () => {
    expect(setupCompletionPanelSource).toContain('buildInfrastructureOnboardingPath');
    expect(setupCompletionPanelSource).toContain(
      "const ADD_INFRASTRUCTURE_PATH = buildInfrastructureOnboardingPath('pick');",
    );
    expect(setupCompletionPanelSource).toContain(
      "const AGENT_INSTALL_PATH = buildInfrastructureOnboardingPath('linux-host');",
    );
    expect(setupCompletionPanelSource).toContain(
      'const INFRASTRUCTURE_WORKSPACE_PATH = buildInfrastructureWorkspacePath();',
    );
    expect(setupCompletionPanelSource).toContain('setup.completion.action.addInfrastructure');
    expect(setupCompletionPanelSource).toContain('setup.completion.action.installAgent');
    expect(setupCompletionPanelSource).toContain('setup.completion.credentials.title');
    expect(setupCompletionPanelSource).toContain('setup.completion.credentials.badge');
    expect(setupCompletionPanelSource).toContain('props.onComplete(ADD_INFRASTRUCTURE_PATH);');
    expect(setupCompletionPanelSource).toContain('props.onComplete(AGENT_INSTALL_PATH);');
    expect(setupCompletionPanelSource).toContain(
      'props.onComplete(INFRASTRUCTURE_WORKSPACE_PATH);',
    );
    expect(setupCompletionPanelSource).toContain('setup.completion.download.content');
    expect(setupCompletionModelSource).toContain('setup.completion.nextStep.summary.empty');
    expect(setupCompletionPanelSource).not.toContain('Use Add connection to connect');
    expect(setupCompletionPanelSource).not.toContain("from '@/stores/licenseCommercial';");
    expect(setupCompletionPanelSource).not.toContain('runStartProTrialAction');
    expect(setupCompletionPanelSource).not.toContain('loadCommercialPosture');
    expect(setupCompletionPanelSource).not.toContain('isCommercialTrialActive');
    expect(setupCompletionPanelSource).not.toContain('Monitor from Anywhere');
    expect(setupCompletionPanelSource).not.toContain("trackPaywallViewed('relay'");
    expect(setupCompletionPanelSource).not.toContain("trackUpgradeClicked('setup_wizard'");
    expect(setupCompletionPanelSource).not.toContain('infrastructureOnboardingMetrics');
    expect(setupCompletionPanelSource).not.toContain('trackAgentFirstConnected');
    expect(setupCompletionPanelSource).not.toContain('getUpgradeActionUrlOrFallback');
  });

  it('describes setup completion through one compact source-choice next-step surface', () => {
    expect(setupCompletionPanelSource).toContain('sourceStrategyOptions');
    expect(setupCompletionPanelSource).toContain('setup.completion.sourceOptions.title');
    expect(setupCompletionPanelSource).toContain('<ul class="mt-2 space-y-1.5 text-left">');
    expect(setupCompletionPanelSource).toContain(
      'setup.completion.sourceOptions.platformApi.title',
    );
    expect(setupCompletionPanelSource).toContain('setup.completion.sourceOptions.agent.title');
    expect(setupCompletionPanelSource).toContain('setup.completion.sourceOptions.both.title');
    expect(setupCompletionPanelSource).toContain(
      'setup.completion.sourceOptions.platformApi.description',
    );
    expect(setupCompletionModelSource).toContain('setup.completion.nextStep.detail.empty');
    expect(setupCompletionPanelSource).not.toContain("title: 'What happens next'");
    expect(setupCompletionPanelSource).not.toContain("title: 'Open Add infrastructure'");
    expect(setupCompletionPanelSource).not.toContain(
      "title: 'Save the source and confirm coverage'",
    );
    expect(setupCompletionPanelSource).not.toContain('What to expect');
    expect(setupCompletionPanelSource).not.toContain('First system first');
    expect(setupCompletionPanelSource).not.toContain('Smart Auto-Detection');
    expect(setupCompletionPanelSource).not.toContain('Agent Metrics');
    expect(setupCompletionPanelSource).not.toContain('ProxmoxIcon');
  });

  it('does not present setup guidance as extra credentials or clickable-looking source cards', () => {
    expect(setupCompletionPanelSource).not.toMatch(
      /<code[^>]*>\s*\{ADD_INFRASTRUCTURE_PATH\}\s*<\/code>/,
    );
    expect(setupCompletionPanelSource).not.toContain('grid gap-2 sm:grid-cols-3');
    expect(setupCompletionPanelSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-2.5',
    );
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
    expect(setupCompletionPanelSource).toContain('setup.completion.credentials.description');
    expect(setupCompletionPanelSource).toContain('setup.completion.nextStep.badge');
    expect(setupCompletionPanelSource).toContain('setup.completion.nextStep.ariaLabel');
    expect(setupCompletionPanelSource).toContain('setup.completion.action.openInfrastructure');
    expect(setupCompletionModelSource).toContain('setup.completion.hero.connected.title');
    expect(setupCompletionModelSource).toContain('setup.completion.hero.empty.title');
    expect(setupCompletionPanelSource).toContain(
      "completionViewModel().primaryAction === 'infrastructure'",
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
    expect(setupCompletionPanelSource).not.toContain(
      'Add infrastructure now owns the first source decision.',
    );
    expect(setupCompletionPanelSource).not.toContain(
      'then return to Add infrastructure when you want to connect the next API or Agent source.',
    );
  });

  it('prevents migrated first-session monitoring copy from reverting to hardcoded English', () => {
    for (const source of migratedSources) {
      for (const copy of migratedFirstSessionCopy) {
        expect(source).not.toContain(copy);
      }
    }

    expect(welcomeStepSource).toContain('setup.welcome.hero.title');
    expect(securityStepSource).toContain('setup.security.title');
    expect(setupWizardSource).toContain('setup.wizard.ariaLabel');
    expect(stepIndicatorSource).toContain('setup.progress.stepAriaLabel');
    expect(runtimeHomeSource).toContain('runtimeHome.openingWorkspace');
  });

  it('keeps setup preview copy actions accessible at phone widths', () => {
    expect(setupCompletionPanelSource).toContain('import { ActionIconButton }');
    expect(setupCompletionPanelSource).toContain('setup.completion.action.copyPassword');
    expect(setupCompletionPanelSource).toContain('setup.completion.action.copyAdminToken');
    expect(setupCompletionPanelSource).toContain('min-h-10 min-w-10');
  });
});
