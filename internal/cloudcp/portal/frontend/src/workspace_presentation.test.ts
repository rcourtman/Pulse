import { describe, expect, it } from 'vitest';

import { workspaceGuidanceCopy, workspaceHealthLabel, workspaceHealthState, workspaceRowNote, workspaceStatusCopy } from './workspace_presentation';
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
    var healthy = createWorkspace();
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

  it('uses literal lifecycle guidance instead of commentary', function() {
    expect(workspaceGuidanceCopy(createWorkspace())).toBe('This workspace is active. Open it from the workspace list, or suspend it here if you intend to take it out of service.');
    expect(workspaceGuidanceCopy(createWorkspace({ healthy: false, health_status: 'checking' }))).toBe('This workspace is active. The latest health check is still pending.');
    expect(workspaceGuidanceCopy(createWorkspace({ healthy: false, health_status: 'unhealthy' }))).toBe('The latest health check is unhealthy. Review the current state before suspending or deleting this workspace.');
    expect(workspaceGuidanceCopy(createWorkspace({ state: 'suspended', health_status: 'healthy' }))).toBe('This workspace is suspended. The remaining lifecycle action here is deletion.');
  });
});
