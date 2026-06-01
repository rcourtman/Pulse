import { describe, expect, it } from 'vitest';

import {
  workspaceGuidanceCopy,
  workspaceHealthLabel,
  workspaceHealthState,
  workspaceRowNote,
  workspaceSetupLabel,
  workspaceSetupNextStep,
  workspaceSetupState,
  workspaceStatusCopy,
} from './workspace_presentation';
import type { PortalWorkspaceSummary } from './types';

function createWorkspace(overrides: Partial<PortalWorkspaceSummary> = {}): PortalWorkspaceSummary {
  return {
    id: 'ws_1',
    display_name: 'Alpha Workspace',
    state: 'active',
    healthy: true,
    health_status: 'healthy',
    ...overrides,
  };
}

describe('workspace presentation', function() {
  it('derives consistent literal copy for healthy, pending, unhealthy, and suspended workspaces', function() {
    var healthy = createWorkspace({ setup_status: 'ready' });
    var checking = createWorkspace({ healthy: false, health_status: 'checking' });
    var unhealthy = createWorkspace({ healthy: false, health_status: 'unhealthy' });
    var suspended = createWorkspace({ state: 'suspended', health_status: 'healthy' });

    expect(workspaceHealthState(healthy)).toBe('healthy');
    expect(workspaceStatusCopy(healthy)).toBe('Latest health check is healthy.');
    expect(workspaceHealthLabel(healthy)).toBe('Healthy');
    expect(workspaceRowNote(healthy)).toBe('Ready');

    expect(workspaceHealthState(checking)).toBe('checking');
    expect(workspaceStatusCopy(checking)).toBe('Latest health check is still pending.');
    expect(workspaceHealthLabel(checking)).toBe('Checking');
    expect(workspaceRowNote(checking)).toBe('Health check pending');

    expect(workspaceHealthState(unhealthy)).toBe('unhealthy');
    expect(workspaceStatusCopy(unhealthy)).toBe('Latest health check is unhealthy.');
    expect(workspaceHealthLabel(unhealthy)).toBe('Unhealthy');
    expect(workspaceRowNote(unhealthy)).toBe('Unhealthy');

    expect(workspaceStatusCopy(suspended)).toBe('This workspace is suspended.');
    expect(workspaceRowNote(suspended)).toBe('Suspended');
  });

  it('uses literal setup guidance instead of commentary', function() {
    expect(workspaceGuidanceCopy(createWorkspace({ setup_status: 'ready' }))).toBe('Open the workspace when you need to work inside this client boundary.');
    expect(workspaceGuidanceCopy(createWorkspace())).toBe('Open the workspace or install agents from the workspace-bound setup path.');
    expect(workspaceGuidanceCopy(createWorkspace({ healthy: false, health_status: 'checking' }))).toBe('This workspace is active. The latest health check is still pending, but the workspace can still own agent install commands.');
    expect(workspaceGuidanceCopy(createWorkspace({ healthy: false, health_status: 'unhealthy' }))).toBe('The latest health check is unhealthy. Review the current state before suspending or deleting this workspace.');
    expect(workspaceGuidanceCopy(createWorkspace({ state: 'suspended', health_status: 'healthy' }))).toBe('This workspace is suspended. The remaining destructive action here is deletion.');
  });

  it('derives setup state from explicit status and optional setup counts', function() {
    expect(workspaceSetupState(createWorkspace())).toBe('setup_path');
    expect(workspaceSetupLabel(createWorkspace())).toBe('Setup path');
    expect(workspaceRowNote(createWorkspace())).toBe('Setup path ready');
    expect(workspaceSetupNextStep(createWorkspace({ setup_status: 'install_agents' }))).toBe('Install the first agent from this workspace so client data lands in the right boundary.');
    expect(workspaceSetupState(createWorkspace({ agent_count: 0 }))).toBe('install_agents');
    expect(workspaceSetupState(createWorkspace({ setup_status: 'setup_path', agent_count: 0 }))).toBe('install_agents');
    expect(workspaceSetupState(createWorkspace({ agent_count: 1, alert_route_count: 0, report_schedule_count: 0 }))).toBe('configure_outputs');
    expect(workspaceSetupState(createWorkspace({ agent_count: 1, alert_route_count: 1, report_schedule_count: 1 }))).toBe('ready');
    expect(workspaceSetupState(createWorkspace({ state: 'failed', healthy: false, health_status: 'unhealthy' }))).toBe('review');
  });
});
