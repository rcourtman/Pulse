import { MIGRATION_GUIDE_DOC_URL, PRIVACY_DOC_URL } from '@/utils/docsLinks';

export interface WhatsNewFeatureCard {
  accent: string;
  description: string;
  icon: 'infrastructure' | 'workloads' | 'storage' | 'recovery';
  target: 'infrastructure' | 'workloads' | 'storage' | 'recovery';
  title: string;
}

export const WHATS_NEW_DOCS_URL = MIGRATION_GUIDE_DOC_URL;
export const WHATS_NEW_PRIVACY_URL = PRIVACY_DOC_URL;

export const WHATS_NEW_FEATURE_CARDS: WhatsNewFeatureCard[] = [
  {
    accent: 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900',
    description:
      'Start here to add, inspect, and manage infrastructure sources: platform API inventory, Pulse Agent telemetry, and discovered candidates.',
    icon: 'infrastructure',
    target: 'infrastructure',
    title: 'Infrastructure',
  },
  {
    accent: 'border-rose-200 bg-rose-50 dark:border-rose-800 dark:bg-rose-900',
    description: 'Use this for VMs, containers, pods, and other running workloads.',
    icon: 'workloads',
    target: 'workloads',
    title: 'Workloads',
  },
  {
    accent: 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-900',
    description: 'Use this for pools, datastores, disks, datasets, and capacity.',
    icon: 'storage',
    target: 'storage',
    title: 'Storage',
  },
  {
    accent: 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900',
    description: 'Use this for backup coverage, snapshots, replication, and restore readiness.',
    icon: 'recovery',
    target: 'recovery',
    title: 'Recovery',
  },
];

export const WHATS_NEW_REOPEN_EVENT = 'pulse:reopen-nav-guide';

export const WHATS_NEW_KICKER_LABEL = 'Nav guide';
export const WHATS_NEW_TITLE = 'Pulse navigation guide';
export const WHATS_NEW_PROGRESS_PREFIX = 'Step';
export const WHATS_NEW_BACK_LABEL = 'Back';
export const WHATS_NEW_CLOSE_LABEL = 'Close';
export const WHATS_NEW_DOCS_LABEL = 'Navigation guide';
export const WHATS_NEW_DO_NOT_SHOW_LABEL = "Don't show again";
export const WHATS_NEW_NEXT_LABEL = 'Next';
export const WHATS_NEW_PRIMARY_ACTION_LABEL = 'Done';
export const WHATS_NEW_TELEMETRY_LINK_LABEL = 'Telemetry details';
