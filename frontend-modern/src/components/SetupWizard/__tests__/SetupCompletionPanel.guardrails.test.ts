import { describe, expect, it } from 'vitest';
import setupCompletionPanelSource from '../SetupCompletionPanel.tsx?raw';

describe('SetupCompletionPanel guardrails', () => {
  it('keeps setup completion aligned with the canonical infrastructure install workspace', () => {
    expect(setupCompletionPanelSource).toContain(
      "const INFRASTRUCTURE_INSTALL_PATH = '/settings/infrastructure/install';",
    );
    expect(setupCompletionPanelSource).toContain('Open Infrastructure Install');
    expect(setupCompletionPanelSource).toContain('Infrastructure Install Workspace');
    expect(setupCompletionPanelSource).toContain('props.onComplete(INFRASTRUCTURE_INSTALL_PATH);');
    expect(setupCompletionPanelSource).toContain('Use the Infrastructure Install workspace to:');
    expect(setupCompletionPanelSource).toContain('generate Unified Agent tokens');
    expect(setupCompletionPanelSource).toContain('configure TLS and custom CA options');
  });

  it('describes setup completion through the unified resource model instead of legacy install-command copy', () => {
    expect(setupCompletionPanelSource).toContain("title: 'Unified Resource Inventory'");
    expect(setupCompletionPanelSource).toContain('Pulse v6 starts with the Unified Agent.');
    expect(setupCompletionPanelSource).toContain("title: 'Open Infrastructure Install'");
    expect(setupCompletionPanelSource).toContain("title: 'Bring Systems Into Pulse'");
    expect(setupCompletionPanelSource).toContain('What Pulse Builds');
    expect(setupCompletionPanelSource).toContain('Unified by default');
    expect(setupCompletionPanelSource).toContain('One install becomes one monitored system in Pulse.');
    expect(setupCompletionPanelSource).not.toContain('Smart Auto-Detection');
    expect(setupCompletionPanelSource).not.toContain('Agent Metrics');
    expect(setupCompletionPanelSource).not.toContain('ProxmoxIcon');
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
});
