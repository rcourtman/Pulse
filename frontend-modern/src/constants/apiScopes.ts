export interface APIScopeOption {
  value: string;
  label: string;
  description?: string;
  group: 'Monitoring' | 'Agents' | 'Settings';
}

export const HOST_AGENT_SCOPE = 'host-agent:report';
export const HOST_AGENT_CONFIG_READ_SCOPE = 'host-agent:config:read';
export const DOCKER_REPORT_SCOPE = 'docker:report';
export const DOCKER_MANAGE_SCOPE = 'docker:manage';
export const KUBERNETES_REPORT_SCOPE = 'kubernetes:report';
export const KUBERNETES_MANAGE_SCOPE = 'kubernetes:manage';
export const MONITORING_READ_SCOPE = 'monitoring:read';
export const MONITORING_WRITE_SCOPE = 'monitoring:write';
export const SETTINGS_READ_SCOPE = 'settings:read';
export const SETTINGS_WRITE_SCOPE = 'settings:write';

export const API_SCOPE_OPTIONS: APIScopeOption[] = [
  {
    value: MONITORING_READ_SCOPE,
    label: 'Dashboards & alerts (read)',
    description: 'View monitoring data, dashboards, and alert history.',
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
    label: 'Container agent reporting',
    description: 'Allow the container agent to submit host and runtime telemetry.',
    group: 'Agents',
  },
  {
    value: DOCKER_MANAGE_SCOPE,
    label: 'Container lifecycle management',
    description: 'Enable agent-triggered runtime commands and host actions.',
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
    value: HOST_AGENT_SCOPE,
    label: 'Host agent reporting',
    description: 'Allow the host agent to send OS, CPU, and disk metrics.',
    group: 'Agents',
  },
  {
    value: HOST_AGENT_CONFIG_READ_SCOPE,
    label: 'Host agent config fetch',
    description: 'Allow the host agent to retrieve its assigned configuration profile.',
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
];

export const API_SCOPE_LABELS = API_SCOPE_OPTIONS.reduce<Record<string, string>>((acc, option) => {
  acc[option.value] = option.label;
  return acc;
}, {});
