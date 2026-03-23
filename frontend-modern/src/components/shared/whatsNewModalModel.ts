export interface WhatsNewFeatureCard {
  accent: string;
  description: string;
  icon: 'infrastructure' | 'workloads' | 'storage' | 'recovery';
  title: string;
}

export const WHATS_NEW_DOCS_URL = 'https://github.com/rcourtman/Pulse/blob/main/docs/README.md';
export const WHATS_NEW_PRIVACY_URL =
  'https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md';

export const WHATS_NEW_FEATURE_CARDS: WhatsNewFeatureCard[] = [
  {
    accent: 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900',
    description: 'Proxmox nodes, agents, and container runtimes live together in one unified view.',
    icon: 'infrastructure',
    title: 'Infrastructure',
  },
  {
    accent: 'border-purple-200 bg-purple-50 dark:border-purple-800 dark:bg-purple-900',
    description: 'All VMs, containers, and Kubernetes workloads now share a single list.',
    icon: 'workloads',
    title: 'Workloads',
  },
  {
    accent: 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-900',
    description: 'Storage is now a top-level destination across all systems.',
    icon: 'storage',
    title: 'Storage',
  },
  {
    accent: 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900',
    description:
      'Recovery events (backups, snapshots, and replication) are now first-class pages.',
    icon: 'recovery',
    title: 'Recovery',
  },
];

export const WHATS_NEW_TITLE = 'Welcome to the New Navigation!';
export const WHATS_NEW_SUBTITLE =
  'Everything is now organized by what you want to do, not where the data comes from.';
export const WHATS_NEW_TELEMETRY_TITLE = 'Anonymous telemetry';
export const WHATS_NEW_TELEMETRY_COPY = [
  'Pulse now sends a lightweight anonymous ping once a day — just a random install ID, version, platform, resource counts, and feature flags. No hostnames, credentials, IP addresses, or personal information is ever sent.',
  'This helps the developer understand how Pulse is used and prioritise what to build next.',
];
export const WHATS_NEW_TELEMETRY_SETTINGS_PATH = 'Settings → System → General';
export const WHATS_NEW_TELEMETRY_ENV_VAR = 'PULSE_TELEMETRY=false';
export const WHATS_NEW_TELEMETRY_PRIVACY_LABEL = 'Full details';
export const WHATS_NEW_PRIMARY_ACTION_LABEL = "Let's go";
export const WHATS_NEW_CLOSE_LABEL = 'Close';
export const WHATS_NEW_DOCS_LABEL = 'Documentation';
export const WHATS_NEW_RECOVERY_LINK_LABEL = 'Recovery events';
export const WHATS_NEW_DO_NOT_SHOW_LABEL = "Don't show again";
