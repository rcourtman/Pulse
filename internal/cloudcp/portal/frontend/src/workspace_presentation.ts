import type { PortalWorkspaceSummary } from './types';

export function workspaceHealthState(workspace: PortalWorkspaceSummary): 'healthy' | 'checking' | 'unhealthy' {
  if (workspace.health_status === 'healthy' || workspace.health_status === 'checking' || workspace.health_status === 'unhealthy') {
    return workspace.health_status;
  }
  if (workspace.healthy) return 'healthy';
  if (workspace.last_health_check) return 'unhealthy';
  return 'checking';
}

export function workspaceStatusCopy(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  if (state === 'suspended') return 'This workspace is suspended.';
  if (state === 'failed') return 'This workspace is in a failed state.';
  if (status === 'healthy') return 'Latest health check is healthy.';
  if (status === 'unhealthy') return 'Latest health check is unhealthy.';
  return 'Latest health check is still pending.';
}

export function workspaceHealthLabel(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  if (status === 'healthy') return 'Healthy';
  if (status === 'unhealthy') return 'Unhealthy';
  return 'Checking';
}

export function workspaceRowNote(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  if (state === 'suspended') return 'Suspended';
  if (state === 'failed') return 'Failed';
  if (status === 'healthy') return 'Ready';
  if (status === 'unhealthy') return 'Unhealthy';
  return 'Health check pending';
}

export function workspaceGuidanceCopy(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  if (state === 'active' && status === 'healthy') {
    return 'This workspace is active. Open it from the workspace list, or suspend it here if you intend to take it out of service.';
  }
  if (state === 'active' && status === 'checking') {
    return 'This workspace is active. The latest health check is still pending.';
  }
  if (status === 'unhealthy') {
    return 'The latest health check is unhealthy. Review the current state before suspending or deleting this workspace.';
  }
  if (state === 'suspended') {
    return 'This workspace is suspended. The remaining lifecycle action here is deletion.';
  }
  return 'Review the current lifecycle state before taking action on this workspace.';
}
