export const DIAGNOSTICS_PANEL_COPY = {
  title: 'System Diagnostics',
  description: 'Review connection health, configuration status, and troubleshooting tools.',
  summary: 'Test all connections and inspect runtime configuration.',
  runActionLabel: 'Run Diagnostics',
  runShortLabel: 'Run',
  runningActionLabel: 'Running...',
  exportFullLabel: 'Full',
  exportGithubLabel: 'GitHub',
  versionLabel: 'Version',
  uptimeLabel: 'Uptime',
  recommendedVersionLabel: 'Recommended version',
} as const;

export const DIAGNOSTICS_EMPTY_STATE_COPY = {
  title: 'No diagnostics data available',
  description: 'Run diagnostics to test connections and inspect system status.',
  actionLabel: 'Run Diagnostics',
} as const;

export const DIAGNOSTICS_EMPTY_PBS_MESSAGE = 'No Proxmox Backup Server instances configured.';
