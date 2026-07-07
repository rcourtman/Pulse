import type { PortalWorkspaceSummary } from './types';

export type WorkspaceSetupState = 'ready' | 'setup_path' | 'install_agents' | 'configure_outputs' | 'review';
export type WorkspaceSetupStepID = 'workspace' | 'agent' | 'alerts' | 'reports' | 'access';
export type WorkspaceSetupStepTone = 'done' | 'next' | 'pending' | 'blocked' | 'available';
export type WorkspaceAlertState = 'critical' | 'warning' | 'quiet' | 'unknown';

export interface WorkspaceSetupStepModel {
  id: WorkspaceSetupStepID;
  title: string;
  detail: string;
  tone: WorkspaceSetupStepTone;
  label: string;
}

export interface WorkspaceSetupGuide {
  title: string;
  description: string;
  primaryAction: 'open' | 'install' | 'outputs' | 'access' | 'review';
  primaryLabel: string;
  diagnostics: string[];
}

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

export function workspaceActiveAlertState(workspace: PortalWorkspaceSummary): WorkspaceAlertState {
  if (!hasNumber(workspace.active_critical_alert_count) || !hasNumber(workspace.active_warning_alert_count)) {
    return 'unknown';
  }
  if (Number(workspace.active_critical_alert_count) > 0) return 'critical';
  if (Number(workspace.active_warning_alert_count) > 0) return 'warning';
  return 'quiet';
}

export function workspaceActiveAlertLabel(workspace: PortalWorkspaceSummary): string {
  var state = workspaceActiveAlertState(workspace);
  if (state === 'unknown') return 'No alert data yet';
  var critical = Number(workspace.active_critical_alert_count || 0);
  var warning = Number(workspace.active_warning_alert_count || 0);
  if (state === 'quiet') return '0 active alerts';
  var parts = [];
  if (critical > 0) parts.push(String(critical) + ' critical');
  if (warning > 0) parts.push(String(warning) + ' warning');
  return parts.join(', ');
}

export function workspaceActiveAlertsUpdatedLabel(workspace: PortalWorkspaceSummary, now = new Date()): string {
  if (!workspace.active_alerts_updated_at) return 'no alert data yet';
  var updated = new Date(workspace.active_alerts_updated_at);
  if (Number.isNaN(updated.getTime())) return 'no alert data yet';
  var minutes = Math.max(0, Math.round((now.getTime() - updated.getTime()) / 60000));
  if (minutes <= 1) return 'as of 1 min ago';
  return 'as of ' + String(minutes) + ' min ago';
}

function positiveCount(value: unknown): boolean {
  return hasNumber(value) && Number(value) > 0;
}

function zeroCount(value: unknown): boolean {
  return hasNumber(value) && Number(value) <= 0;
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
      return 'Install the first agent from this workspace so client data lands in the isolated workspace boundary.';
    case 'configure_outputs':
      return 'Configure alert routing and reports before treating the client workspace as ready.';
    case 'review':
      return 'Review the workspace state before continuing setup.';
    default:
      return 'Open the workspace or install agents from the workspace-bound setup path.';
  }
}

export function workspaceIdentityCopy(workspace: PortalWorkspaceSummary): string {
  return 'Client workspace boundary: ' + workspace.display_name + '. Hostnames can repeat across clients because agents, alerts, and reports stay scoped to this workspace.';
}

export function workspaceSetupDiagnostics(workspace: PortalWorkspaceSummary): string[] {
  var setup = workspaceSetupState(workspace);
  var diagnostics: string[] = [];
  if (setup === 'ready') {
    diagnostics.push('Reporting agent, enabled alert route, and enabled report schedule are present.');
    return diagnostics;
  }
  if (setup === 'review') {
    diagnostics.push(workspaceStatusCopy(workspace));
    return diagnostics;
  }

  if (zeroCount(workspace.agent_count)) {
    if (positiveCount(workspace.unused_agent_token_count) || positiveCount(workspace.agent_token_count)) {
      diagnostics.push('Agent install token exists, but no reporting agent has checked in yet.');
    } else {
      diagnostics.push('No reporting agent has checked in yet.');
    }
  }

  if (positiveCount(workspace.agent_count) && zeroCount(workspace.alert_route_count)) {
    diagnostics.push(
      positiveCount(workspace.disabled_alert_route_count)
        ? 'Alert route configuration exists, but no route is enabled.'
        : 'No enabled alert route is configured yet.',
    );
  }

  if (positiveCount(workspace.agent_count) && zeroCount(workspaceReportScheduleCount(workspace))) {
    diagnostics.push(
      positiveCount(workspace.disabled_report_schedule_count)
        ? 'Report schedule exists, but no schedule is enabled.'
        : 'No enabled report schedule is configured yet.',
    );
  }

  if (!diagnostics.length) {
    diagnostics.push(workspaceSetupNextStep(workspace));
  }
  return diagnostics;
}

function workspaceReportScheduleCount(workspace: PortalWorkspaceSummary): unknown {
  return workspace.report_schedule_count;
}

export function workspaceSetupGuide(workspace: PortalWorkspaceSummary): WorkspaceSetupGuide {
  var setup = workspaceSetupState(workspace);
  if (setup === 'ready') {
    return {
      title: 'Ready',
      description: 'Agents, alert routing, and reports are in place for this client workspace.',
      primaryAction: 'open',
      primaryLabel: 'Open workspace',
      diagnostics: workspaceSetupDiagnostics(workspace),
    };
  }
  if (setup === 'review') {
    return {
      title: 'Review workspace state',
      description: 'Resolve the workspace state before continuing client setup.',
      primaryAction: 'review',
      primaryLabel: 'Review workspace',
      diagnostics: workspaceSetupDiagnostics(workspace),
    };
  }
  if (setup === 'install_agents') {
    return {
      title: 'Install the first agent',
      description: 'Start inside this isolated client workspace so the first reporting token and future hostnames stay scoped to the client.',
      primaryAction: 'install',
      primaryLabel: 'Install agents',
      diagnostics: workspaceSetupDiagnostics(workspace),
    };
  }
  if (setup === 'configure_outputs') {
    var needsAlerts = zeroCount(workspace.alert_route_count);
    var needsReports = zeroCount(workspaceReportScheduleCount(workspace));
    return {
      title: needsAlerts && needsReports ? 'Configure alerts and reports' : needsAlerts ? 'Configure alert routes' : 'Schedule reports',
      description: 'Finish the output side before this workspace leaves onboarding.',
      primaryAction: 'outputs',
      primaryLabel: needsReports && !needsAlerts ? 'Open reports' : 'Configure outputs',
      diagnostics: workspaceSetupDiagnostics(workspace),
    };
  }
  return {
    title: 'Follow the setup path',
    description: 'Open the client workspace and continue the next setup task from there.',
    primaryAction: 'install',
    primaryLabel: 'Open setup',
    diagnostics: workspaceSetupDiagnostics(workspace),
  };
}

export function workspaceSetupSteps(workspace: PortalWorkspaceSummary): WorkspaceSetupStepModel[] {
  var setup = workspaceSetupState(workspace);
  var state = String(workspace.state || '');
  var isActive = state === 'active';
  var hasAgents = positiveCount(workspace.agent_count) || setup === 'configure_outputs' || setup === 'ready';
  var hasAlerts = positiveCount(workspace.alert_route_count);
  var hasReports = positiveCount(workspaceReportScheduleCount(workspace));

  return [
    {
      id: 'workspace',
      title: 'Create workspace',
      detail: 'Separate client boundary created.',
      tone: state ? 'done' : 'pending',
      label: state ? 'Done' : 'Pending',
    },
    {
      id: 'agent',
      title: 'Install first agent',
      detail: 'First reporting agent checks in inside this workspace.',
      tone: hasAgents ? 'done' : setup === 'review' || !isActive ? 'blocked' : 'next',
      label: hasAgents ? 'Done' : setup === 'review' || !isActive ? 'Review' : 'Next',
    },
    {
      id: 'alerts',
      title: 'Configure alert routes',
      detail: 'Enabled notification route exists for this client.',
      tone: hasAlerts ? 'done' : setup === 'review' ? 'blocked' : isActive && hasAgents ? 'next' : 'pending',
      label: hasAlerts ? 'Done' : setup === 'review' ? 'Review' : isActive && hasAgents ? 'Next' : 'Pending',
    },
    {
      id: 'reports',
      title: 'Schedule reports',
      detail: 'Enabled report schedule exists for client reporting.',
      tone: hasReports ? 'done' : setup === 'review' ? 'blocked' : isActive && hasAgents && hasAlerts ? 'next' : 'pending',
      label: hasReports ? 'Done' : setup === 'review' ? 'Review' : isActive && hasAgents && hasAlerts ? 'Next' : 'Pending',
    },
    {
      id: 'access',
      title: 'Review access',
      detail: 'Provider staff and client users are handled from Access.',
      tone: 'available',
      label: 'Available',
    },
  ];
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
