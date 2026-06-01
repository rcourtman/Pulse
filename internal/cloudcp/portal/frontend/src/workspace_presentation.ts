import type { PortalWorkspaceSummary } from './types';

export type WorkspaceSetupState = 'ready' | 'setup_path' | 'install_agents' | 'configure_outputs' | 'review';

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
  if (status === 'checking') return 'Health check pending';
  if (status === 'unhealthy') return 'Unhealthy';
  var setup = workspaceSetupState(workspace);
  if (setup === 'ready') return 'Ready';
  if (setup === 'install_agents') return 'Install first agent';
  if (setup === 'configure_outputs') return 'Configure alerts and reports';
  if (setup === 'setup_path') return 'Setup path ready';
  if (setup === 'review') return 'Review';
  return 'Ready';
}

function hasNumber(value: unknown): boolean {
  return typeof value === 'number' && Number.isFinite(value);
}

function positiveCount(value: unknown): boolean {
  return hasNumber(value) && Number(value) > 0;
}

function explicitSetupStatus(workspace: PortalWorkspaceSummary): WorkspaceSetupState | '' {
  var value = String(workspace.setup_status || '');
  if (
    value === 'ready' ||
    value === 'setup_path' ||
    value === 'install_agents' ||
    value === 'configure_outputs' ||
    value === 'review'
  ) {
    return value;
  }
  return '';
}

export function workspaceSetupState(workspace: PortalWorkspaceSummary): WorkspaceSetupState {
  var state = String(workspace.state || '');
  var health = workspaceHealthState(workspace);
  if (state === 'suspended' || state === 'failed' || health === 'unhealthy') return 'review';
  if (state !== 'active' || health === 'checking') return 'setup_path';

  var knowsAgentCount = hasNumber(workspace.agent_count);
  var knowsAlertCount = hasNumber(workspace.alert_route_count);
  var knowsReportCount = hasNumber(workspace.report_schedule_count);
  var hasAgents = positiveCount(workspace.agent_count);
  var hasAlerts = positiveCount(workspace.alert_route_count);
  var hasReports = positiveCount(workspace.report_schedule_count);

  if (knowsAgentCount && !hasAgents) return 'install_agents';
  if (hasAgents && ((knowsAlertCount && !hasAlerts) || (knowsReportCount && !hasReports))) {
    return 'configure_outputs';
  }
  if (hasAgents && (!knowsAlertCount || hasAlerts) && (!knowsReportCount || hasReports)) {
    return 'ready';
  }
  var explicit = explicitSetupStatus(workspace);
  if (explicit) return explicit;
  return 'setup_path';
}

export function workspaceSetupLabel(workspace: PortalWorkspaceSummary): string {
  switch (workspaceSetupState(workspace)) {
    case 'ready':
      return 'Ready';
    case 'install_agents':
      return 'Install agent';
    case 'configure_outputs':
      return 'Configure outputs';
    case 'review':
      return 'Review';
    default:
      return 'Setup path';
  }
}

export function workspaceSetupNextStep(workspace: PortalWorkspaceSummary): string {
  switch (workspaceSetupState(workspace)) {
    case 'ready':
      return 'Open the workspace when you need to work inside this client boundary.';
    case 'install_agents':
      return 'Install the first agent from this workspace so client data lands in the right boundary.';
    case 'configure_outputs':
      return 'Configure alert routing and reports before treating the client workspace as ready.';
    case 'review':
      return 'Review the workspace state before continuing setup.';
    default:
      return 'Open the workspace or install agents from the workspace-bound setup path.';
  }
}

export function workspaceGuidanceCopy(workspace: PortalWorkspaceSummary): string {
  var status = workspaceHealthState(workspace);
  var state = String(workspace.state || '');
  if (state === 'active' && status === 'healthy') {
    return workspaceSetupNextStep(workspace);
  }
  if (state === 'active' && status === 'checking') {
    return 'This workspace is active. The latest health check is still pending, but the workspace can still own agent install commands.';
  }
  if (status === 'unhealthy') {
    return 'The latest health check is unhealthy. Review the current state before suspending or deleting this workspace.';
  }
  if (state === 'suspended') {
    return 'This workspace is suspended. The remaining destructive action here is deletion.';
  }
  return 'Review the current workspace state before taking action on this workspace.';
}
