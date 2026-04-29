import { getSourcePlatformLabel } from '@/utils/sourcePlatforms';

export interface APIScopeOption {
  value: string;
  label: string;
  description?: string;
  group: 'Monitoring' | 'Agents' | 'Settings' | 'Security';
}

export const AGENT_REPORT_SCOPE = 'agent:report';
export const AGENT_CONFIG_READ_SCOPE = 'agent:config:read';
export const DOCKER_REPORT_SCOPE = 'docker:report';
export const DOCKER_MANAGE_SCOPE = 'docker:manage';
export const KUBERNETES_REPORT_SCOPE = 'kubernetes:report';
export const KUBERNETES_MANAGE_SCOPE = 'kubernetes:manage';
export const AGENT_EXEC_SCOPE = 'agent:exec';
export const MONITORING_READ_SCOPE = 'monitoring:read';
export const MONITORING_WRITE_SCOPE = 'monitoring:write';
export const SETTINGS_READ_SCOPE = 'settings:read';
export const SETTINGS_WRITE_SCOPE = 'settings:write';
export const AUDIT_READ_SCOPE = 'audit:read';

const DOCKER_PODMAN_SOURCE_LABEL = getSourcePlatformLabel('docker');

export const API_SCOPE_OPTIONS: APIScopeOption[] = [
  {
    value: MONITORING_READ_SCOPE,
    label: 'Monitoring & alerts (read)',
    description: 'View monitoring data, infrastructure, workloads, and alert history.',
    group: 'Monitoring',
  },
  {
    value: MONITORING_WRITE_SCOPE,
    label: 'Alert actions (write)',
    description: 'Acknowledge, silence, and clear alerts.',
    group: 'Monitoring',
  },
  {
    value: DOCKER_REPORT_SCOPE,
    label: `${DOCKER_PODMAN_SOURCE_LABEL} reporting`,
    description: `Allow ${DOCKER_PODMAN_SOURCE_LABEL} agents to submit host and runtime telemetry.`,
    group: 'Agents',
  },
  {
    value: DOCKER_MANAGE_SCOPE,
    label: `${DOCKER_PODMAN_SOURCE_LABEL} lifecycle management`,
    description: `Enable agent-triggered ${DOCKER_PODMAN_SOURCE_LABEL} commands and host actions.`,
    group: 'Agents',
  },
  {
    value: KUBERNETES_REPORT_SCOPE,
    label: 'Kubernetes agent reporting',
    description: 'Allow the Kubernetes agent to submit cluster, node, and workload telemetry.',
    group: 'Agents',
  },
  {
    value: KUBERNETES_MANAGE_SCOPE,
    label: 'Kubernetes cluster management',
    description: 'Allow administrative actions for Kubernetes cluster agents.',
    group: 'Agents',
  },
  {
    value: AGENT_REPORT_SCOPE,
    label: 'Agent reporting',
    description: 'Allow the agent to send OS, CPU, and disk metrics.',
    group: 'Agents',
  },
  {
    value: AGENT_CONFIG_READ_SCOPE,
    label: 'Agent config fetch',
    description: 'Allow the agent to retrieve its assigned configuration profile.',
    group: 'Agents',
  },
  {
    value: AGENT_EXEC_SCOPE,
    label: 'Agent exec connection',
    description: 'Allow the agent to establish WebSocket connections for remote execution.',
    group: 'Agents',
  },
  {
    value: SETTINGS_READ_SCOPE,
    label: 'Settings (read)',
    description: 'Fetch configuration snapshots such as nodes and security posture.',
    group: 'Settings',
  },
  {
    value: SETTINGS_WRITE_SCOPE,
    label: 'Settings (write)',
    description: 'Modify configuration, manage tokens, and trigger updates.',
    group: 'Settings',
  },
  {
    value: AUDIT_READ_SCOPE,
    label: 'Audit logs (read)',
    description: 'Read audit events, verification status, summaries, and export history.',
    group: 'Security',
  },
];

export const API_SCOPE_LABELS = API_SCOPE_OPTIONS.reduce<Record<string, string>>((acc, option) => {
  acc[option.value] = option.label;
  return acc;
}, {});
