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
      'Start here first when you want the overall picture: health, alerts, capacity, and recent activity across your estate.',
    icon: 'dashboard',
    target: 'dashboard',
    title: 'Dashboard',
  },
  {
    accent: 'border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900',
    description:
      'Use this when you want the systems themselves: Proxmox nodes, Docker hosts, Kubernetes clusters, PBS, PMG, TrueNAS, and other platform roots.',
    icon: 'infrastructure',
    target: 'infrastructure',
    title: 'Infrastructure',
  },
  {
    accent: 'border-purple-200 bg-purple-50 dark:border-purple-800 dark:bg-purple-900',
    description:
      'VMs, containers, and pods live here now. If you used to drill into guests or Docker workloads in v5, this is the new starting point.',
    icon: 'workloads',
    target: 'workloads',
    title: 'Workloads',
  },
  {
    accent: 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-900',
    description:
      'Datastores, pools, disks, and capacity moved here so storage is one destination across platforms.',
    icon: 'storage',
    target: 'storage',
    title: 'Storage',
  },
  {
    accent: 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900',
    description:
      'Backups, snapshots, and replication moved here. Open this when you want restore posture or recent recovery activity.',
    icon: 'recovery',
    target: 'recovery',
    title: 'Recovery',
  },
];

export const WHATS_NEW_KICKER_LABEL = 'V5 to V6 Guide';
export const WHATS_NEW_CURRENT_STEP_LABEL = 'Now showing';
export const WHATS_NEW_STEP_MAP_LABEL = 'Where Things Moved';
export const WHATS_NEW_STEP_MAP_HELPER = 'Jump ahead or follow the highlighted path.';
export const WHATS_NEW_TITLE = 'Welcome to Pulse v6';
export const WHATS_NEW_SUBTITLE =
  "If you're coming from v5, nothing is gone. Pulse is now grouped by task so you can find things faster.";
export const WHATS_NEW_TELEMETRY_LABEL = 'Telemetry note';
export const WHATS_NEW_TELEMETRY_TITLE = 'Anonymous telemetry';
export const WHATS_NEW_TELEMETRY_COPY = [
  'Pulse also sends a lightweight anonymous daily ping. No hostnames, credentials, or personal information are sent.',
  'You can turn it off any time in Settings → System → General or with PULSE_TELEMETRY=false.',
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
