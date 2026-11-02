export const DIAGNOSTICS_PANEL_COPY = {
  title: 'System Diagnostics',
  description: 'Review connection health, configuration status, and troubleshooting tools.',
  summary: 'Collect runtime information and check configured Proxmox and PBS connections.',
  runSafety:
    'Diagnostics can make live API and guest-agent requests. Do not run it during a backup, freeze/thaw or an unresponsive-host incident. Keep the existing evidence instead. A successful check does not prove ongoing collection has recovered.',
  exportSafety:
    'Downloads use the displayed result without running diagnostics again. Nothing is uploaded. Keep the full file private. Review even a sanitised file for credentials, secret URLs and private host or personal information before sharing.',
  runActionLabel: 'Run Diagnostics',
  runShortLabel: 'Run',
  runningActionLabel: 'Running...',
  runSuccess: 'Diagnostics completed',
  runError:
    'Diagnostics could not be loaded. Previous results have been cleared. Check your access and try again only when live checks are safe.',
  accessUnavailable: 'Diagnostics is not available to this session.',
  exportRequiresResult: 'Run diagnostics first, only when live checks are safe.',
  exportError: 'Diagnostics could not be downloaded. Nothing was uploaded. Try downloading again.',
  exportFullLabel: 'Full (private)',
  exportGithubLabel: 'GitHub (review first)',
  exportFullSuccess: 'Full diagnostics downloaded — keep this file private',
  exportGithubSuccess: 'Sanitised diagnostics downloaded — review before sharing',
  versionLabel: 'Version',
  uptimeLabel: 'Uptime',
  recommendedVersionLabel: 'Recommended version',
} as const;

export const DIAGNOSTICS_EMPTY_STATE_COPY = {
  title: 'No diagnostics data available',
  description:
    'Collect diagnostics only when live checks are safe. Keep existing evidence if the host is unresponsive or a backup is running.',
  actionLabel: 'Run Diagnostics',
} as const;

export const DIAGNOSTICS_EMPTY_PBS_MESSAGE = 'No Proxmox Backup Server instances configured.';
