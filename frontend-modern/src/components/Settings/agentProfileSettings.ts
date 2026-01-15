export interface BooleanSetting {
    key: string;
    type: 'boolean';
    label: string;
    description: string;
}

export interface SelectSetting {
    key: string;
    type: 'select';
    label: string;
    description: string;
    options: string[];
}

export interface DurationSetting {
    key: string;
    type: 'duration';
    label: string;
    description: string;
}

export interface StringSetting {
    key: string;
    type: 'string';
    label: string;
    description: string;
    placeholder?: string;
}

export type KnownSetting = BooleanSetting | SelectSetting | DurationSetting | StringSetting;

// Settings that the agent actually supports (from applyRemoteSettings in cmd/pulse-agent/main.go)
export const KNOWN_SETTINGS: KnownSetting[] = [
    // Core monitoring
    { key: 'enable_host', type: 'boolean', label: 'Enable Host Monitoring', description: 'Collect host metrics and allow command execution' },
    { key: 'enable_docker', type: 'boolean', label: 'Enable Docker Monitoring', description: 'Monitor Docker or Podman containers on this agent' },
    { key: 'docker_runtime', type: 'select', label: 'Docker Runtime', description: 'Force a specific container runtime', options: ['auto', 'docker', 'podman'] },
    { key: 'enable_kubernetes', type: 'boolean', label: 'Enable Kubernetes Monitoring', description: 'Monitor Kubernetes workloads' },
    { key: 'kube_include_all_pods', type: 'boolean', label: 'Include All Pods', description: 'Include all non-succeeded pods in reports' },
    { key: 'kube_include_all_deployments', type: 'boolean', label: 'Include All Deployments', description: 'Include all deployments, not just problem ones' },
    { key: 'enable_proxmox', type: 'boolean', label: 'Enable Proxmox Mode', description: 'Auto-detect and configure Proxmox API access' },
    { key: 'proxmox_type', type: 'select', label: 'Proxmox Type', description: 'Force PVE or PBS mode', options: ['auto', 'pve', 'pbs'] },
    { key: 'disable_ceph', type: 'boolean', label: 'Disable Ceph Monitoring', description: 'Skip local Ceph status polling' },
    // Timing
    { key: 'interval', type: 'duration', label: 'Reporting Interval', description: 'How often the agent reports metrics (e.g., 30s, 1m)' },
    // Network
    { key: 'report_ip', type: 'string', label: 'Report IP Override', description: 'Override the reported IP address', placeholder: '' },
    // Operations
    { key: 'disable_auto_update', type: 'boolean', label: 'Disable Auto Updates', description: 'Stop the unified agent from auto-updating' },
    { key: 'disable_docker_update_checks', type: 'boolean', label: 'Disable Docker Update Checks', description: 'Skip Docker image update detection (avoid registry rate limits)' },
    { key: 'log_level', type: 'select', label: 'Log Level', description: 'Agent logging verbosity', options: ['debug', 'info', 'warn', 'error'] },
];

export const KNOWN_SETTINGS_BY_KEY = new Map(KNOWN_SETTINGS.map(setting => [setting.key, setting]));
