import { MIGRATION_GUIDE_DOC_URL, PRIVACY_DOC_URL } from '@/utils/docsLinks';

export interface WhatsNewFeatureCard {
  accent: string;
  description: string;
  icon: 'dashboard' | 'infrastructure' | 'workloads' | 'storage' | 'recovery';
  target: 'dashboard' | 'infrastructure' | 'workloads' | 'storage' | 'recovery';
  title: string;
}

export const WHATS_NEW_DOCS_URL = MIGRATION_GUIDE_DOC_URL;
export const WHATS_NEW_PRIVACY_URL = PRIVACY_DOC_URL;

export const WHATS_NEW_FEATURE_CARDS: WhatsNewFeatureCard[] = [
  {
    accent: 'border-indigo-200 bg-indigo-50 dark:border-indigo-800 dark:bg-indigo-900',
    description:
      'Start here for a problem-focused summary. This is the landing page now, not the old Proxmox overview.',
    icon: 'dashboard',
    target: 'dashboard',
    title: 'Dashboard',
  },
  {
    accent: 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900',
    description:
      'Platform roots live here: Proxmox nodes, Docker hosts, Kubernetes clusters, PBS, PMG, TrueNAS, and more.',
    icon: 'infrastructure',
    target: 'infrastructure',
    title: 'Infrastructure',
  },
  {
    accent: 'border-purple-200 bg-purple-50 dark:border-purple-800 dark:bg-purple-900',
    description:
      'VMs, containers, pods, and Docker update status now share one unified workloads surface.',
    icon: 'workloads',
    target: 'workloads',
    title: 'Workloads',
  },
  {
    accent: 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-900',
    description: 'Storage is now a top-level destination across all systems.',
    icon: 'storage',
    target: 'storage',
    title: 'Storage',
  },
  {
    accent: 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900',
    description:
      'Recovery events (backups, snapshots, and replication) are now first-class pages.',
    icon: 'recovery',
    target: 'recovery',
    title: 'Recovery',
  },
];

export const WHATS_NEW_TITLE = 'Welcome to Pulse v6';
export const WHATS_NEW_SUBTITLE =
  'Everything is now organized by what you want to do, not where the data comes from.';
export const WHATS_NEW_TELEMETRY_TITLE = 'Anonymous outbound telemetry';
export const WHATS_NEW_TELEMETRY_COPY = [
  'Pulse now sends a lightweight anonymous ping once a day — just a rotating install ID, normalized release identity, platform, resource counts, and feature flags. No hostnames, credentials, or personal information are sent, and IP addresses are not stored in telemetry rows.',
  'This helps the developer understand how Pulse is used and prioritise what to build next.',
];
export const WHATS_NEW_TELEMETRY_SETTINGS_PATH = 'Settings → System → General';
export const WHATS_NEW_TELEMETRY_ENV_VAR = 'PULSE_TELEMETRY=false';
export const WHATS_NEW_TELEMETRY_PRIVACY_LABEL = 'Full details';
export const WHATS_NEW_BACK_LABEL = 'Back';
export const WHATS_NEW_CLOSE_LABEL = 'Close';
export const WHATS_NEW_DOCS_LABEL = 'Migration guide';
export const WHATS_NEW_DO_NOT_SHOW_LABEL = "Don't show again";
export const WHATS_NEW_NEXT_LABEL = 'Next';
export const WHATS_NEW_PRIMARY_ACTION_LABEL = "Let's go";
export const WHATS_NEW_SKIP_LABEL = 'Skip tour';
