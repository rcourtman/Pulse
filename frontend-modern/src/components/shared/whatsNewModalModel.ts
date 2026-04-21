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
    description: 'Start here for health, alerts, capacity, and recent activity.',
    icon: 'dashboard',
    target: 'dashboard',
    title: 'Dashboard',
  },
  {
    accent: 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900',
    description: 'Use this for nodes, hosts, clusters, and other platform roots.',
    icon: 'infrastructure',
    target: 'infrastructure',
    title: 'Infrastructure',
  },
  {
    accent: 'border-purple-200 bg-purple-50 dark:border-purple-800 dark:bg-purple-900',
    description: 'Use this for VMs, containers, and pods.',
    icon: 'workloads',
    target: 'workloads',
    title: 'Workloads',
  },
  {
    accent: 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-900',
    description: 'Use this for datastores, pools, disks, and capacity.',
    icon: 'storage',
    target: 'storage',
    title: 'Storage',
  },
  {
    accent: 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900',
    description: 'Use this for backups, snapshots, and replication.',
    icon: 'recovery',
    target: 'recovery',
    title: 'Recovery',
  },
];

export const WHATS_NEW_KICKER_LABEL = 'Quick Tour';
export const WHATS_NEW_TITLE = 'Welcome to Pulse v6';
export const WHATS_NEW_PROGRESS_PREFIX = 'Step';
export const WHATS_NEW_BACK_LABEL = 'Back';
export const WHATS_NEW_CLOSE_LABEL = 'Close';
export const WHATS_NEW_DOCS_LABEL = 'Navigation guide';
export const WHATS_NEW_DO_NOT_SHOW_LABEL = "Don't show again";
export const WHATS_NEW_NEXT_LABEL = 'Next';
export const WHATS_NEW_PRIMARY_ACTION_LABEL = 'Done';
export const WHATS_NEW_TELEMETRY_LINK_LABEL = 'Telemetry details';
