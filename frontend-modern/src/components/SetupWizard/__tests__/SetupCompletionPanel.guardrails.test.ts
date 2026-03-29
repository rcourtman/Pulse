import { describe, expect, it } from 'vitest';
import setupCompletionPanelSource from '../SetupCompletionPanel.tsx?raw';

describe('SetupCompletionPanel guardrails', () => {
  it('keeps setup completion aligned with the canonical infrastructure install workspace', () => {
    expect(setupCompletionPanelSource).toContain(
      "const INFRASTRUCTURE_INSTALL_PATH = '/settings/infrastructure/install';",
    );
    expect(setupCompletionPanelSource).toContain('Open Infrastructure Install');
    expect(setupCompletionPanelSource).toContain('Infrastructure Install Workspace');
    expect(setupCompletionPanelSource).toContain('Credentials you must save now');
    expect(setupCompletionPanelSource).toContain('Shown during setup');
    expect(setupCompletionPanelSource).toContain('props.onComplete(INFRASTRUCTURE_INSTALL_PATH);');
    expect(setupCompletionPanelSource).toContain('Use the Infrastructure Install workspace to:');
    expect(setupCompletionPanelSource).toContain(
      'continue with the first-host install token Pulse prepares from setup',
    );
    expect(setupCompletionPanelSource).toContain('configure TLS and custom CA options');
    expect(setupCompletionPanelSource).toContain('runStartProTrialAction({');
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
    expect(setupCompletionPanelSource).toContain('First host first');
    expect(setupCompletionPanelSource).toContain('Start with one host, then add more systems later from the same install workspace.');
    expect(setupCompletionPanelSource).toContain(
      'Platform connections remains available for API-backed platforms like Proxmox and TrueNAS when you need it.',
    );
    expect(setupCompletionPanelSource).not.toContain('Smart Auto-Detection');
    expect(setupCompletionPanelSource).not.toContain('Agent Metrics');
    expect(setupCompletionPanelSource).not.toContain('ProxmoxIcon');
  });

  it('keeps connected infrastructure labels on the canonical local identity helper', () => {
    expect(setupCompletionPanelSource).toContain('getPreferredInfrastructureDisplayName');
    expect(setupCompletionPanelSource).not.toContain('getPreferredResourceDisplayName(resource)');
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
    expect(setupCompletionPanelSource).toContain('First monitored host connected');
    expect(setupCompletionPanelSource).toContain(
      'hasConnectedAgents() ? handleGoToDashboard() : handleOpenInstallWorkspace()',
    );
    expect(setupCompletionPanelSource).toContain(
      "{hasConnectedAgents() ? 'Go to Dashboard' : 'Open Infrastructure Install'}",
    );
    expect(setupCompletionPanelSource).not.toContain(
      "connectedAgents().length > 0 ? 'Go to Dashboard' : 'Open Infrastructure Install'",
    );
    expect(setupCompletionPanelSource).not.toContain(
      'You can return here later from Infrastructure Operations if you skip install for now.',
    );
  });
});
