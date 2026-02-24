import type { IncidentEvent } from '@/types/api';
import type { EmailConfig, AppriseConfig } from '@/api/notifications';
import type { BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';
export type { HysteresisThreshold } from '@/types/alerts';
export type AlertTab = 'overview' | 'thresholds' | 'destinations' | 'schedule' | 'history';

export const ALERT_HEADER_META: Record<AlertTab, { title: string; description: string }> = {
  overview: {
    title: 'Alerts Overview',
    description:
      'Monitor active alerts, acknowledgements, and recent status changes across platforms.',
  },
  thresholds: {
    title: 'Alert Thresholds',
    description: 'Tune resource thresholds and override rules for nodes, guests, and containers.',
  },
  destinations: {
    title: 'Notification Destinations',
    description: 'Configure email, webhooks, and escalation paths for alert delivery.',
  },
  schedule: {
    title: 'Maintenance Schedule',
    description:
      'Set quiet hours and maintenance windows to suppress alerts when expected changes occur.',
  },
  history: {
    title: 'Alert History',
    description: 'Review previously triggered alerts and their resolution timeline.',
  },
};

export const ALERT_TAB_SEGMENTS: Record<AlertTab, string> = {
  overview: 'overview',
  thresholds: 'thresholds',
  destinations: 'destinations',
  schedule: 'schedule',
  history: 'history',
};

export const pathForTab = (
  tab: AlertTab,
  segments: Record<AlertTab, string> = ALERT_TAB_SEGMENTS,
): string => {
  const segment = segments[tab];
  return segment ? `/alerts/${segment}` : '/alerts';
};

export const tabFromPath = (
  pathname: string,
  segments: Record<AlertTab, string> = ALERT_TAB_SEGMENTS,
): AlertTab => {
  const normalizedPath = pathname.replace(/\/+$/, '') || '/alerts';
  const parts = normalizedPath.split('/').filter(Boolean);

  if (parts[0] !== 'alerts') {
    return 'overview';
  }

  const segment = parts[1] ?? '';
  if (!segment) {
    return 'overview';
  }

  const entry = (Object.entries(segments) as [AlertTab, string][]).find(
    ([, value]) => value === segment,
  );

  if (entry) {
    return entry[0];
  }

  if (segment === 'custom-rules') {
    return 'thresholds';
  }

  return 'overview';
};

export const INCIDENT_EVENT_TYPES = [
  'alert_fired',
  'alert_acknowledged',
  'alert_unacknowledged',
  'alert_resolved',
  'ai_analysis',
  'command',
  'runbook',
  'note',
] as const;

export const INCIDENT_EVENT_LABELS: Record<(typeof INCIDENT_EVENT_TYPES)[number], string> = {
  alert_fired: 'Fired',
  alert_acknowledged: 'Ack',
  alert_unacknowledged: 'Unack',
  alert_resolved: 'Resolved',
  ai_analysis: 'Patrol',
  command: 'Cmd',
  runbook: 'Runbook',
  note: 'Note',
};
// Store reference interfaces
export interface DestinationsRef {
  emailConfig?: () => EmailConfig;
  appriseConfig?: () => AppriseConfig;
}

// Override interface for both guests and nodes
export type OverrideType =
  | 'guest'
  | 'node'
  | 'hostAgent'
  | 'hostDisk'
  | 'storage'
  | 'pbs'
  | 'pmg'
  | 'dockerHost'
  | 'dockerContainer';

export interface Override {
  id: string; // Full ID (e.g. "Main-node1-105" for guest, "node-node1" for node, "pbs-name" for PBS)
  name: string; // Display name
  type: OverrideType;
  resourceType?: string; // VM, CT, Node, Storage, or PBS
  vmid?: number; // Only for guests
  node?: string; // Node name (for guests and storage), undefined for nodes themselves
  instance?: string;
  disabled?: boolean; // Completely disable alerts for this guest/storage
  disableConnectivity?: boolean; // For nodes/hosts - disable offline/connectivity alerts
  poweredOffSeverity?: 'warning' | 'critical';
  backup?: BackupAlertConfig;
  snapshot?: SnapshotAlertConfig;
  thresholds: {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
    usage?: number; // For storage devices
    temperature?: number;
  };
}

// Local email config with UI-specific fields
export interface UIEmailConfig {
  enabled: boolean;
  provider: string;
  server: string; // Fixed: use 'server' not 'smtpHost'
  port: number; // Fixed: use 'port' not 'smtpPort'
  username: string;
  password: string;
  from: string;
  to: string[];
  tls: boolean;
  startTLS: boolean;
  replyTo: string;
  maxRetries: number;
  retryDelay: number;
  rateLimit: number;
}

export interface UIAppriseConfig {
  enabled: boolean;
  mode: 'cli' | 'http';
  targetsText: string;
  cliPath: string;
  timeoutSeconds: number;
  serverUrl: string;
  configKey: string;
  apiKey: string;
  apiKeyHeader: string;
  skipTlsVerify: boolean;
}

export interface QuietHoursConfig {
  enabled: boolean;
  start: string;
  end: string;
  timezone: string;
  days: Record<string, boolean>;
  suppress: {
    performance: boolean;
    storage: boolean;
    offline: boolean;
  };
}

export interface CooldownConfig {
  enabled: boolean;
  minutes: number;
  maxAlerts: number;
}

export interface GroupingConfig {
  enabled: boolean;
  window: number;
  maxGroupSize?: number;
  byNode?: boolean;
  byGuest?: boolean;
}

export type EscalationNotifyTarget = 'email' | 'webhook' | 'all';

export interface EscalationLevel {
  after: number;
  notify: EscalationNotifyTarget;
}

export interface EscalationConfig {
  enabled: boolean;
  levels: EscalationLevel[];
}

const COOLDOWN_MIN_MINUTES = 5;
const COOLDOWN_MAX_MINUTES = 120;
export const COOLDOWN_DEFAULT_MINUTES = 30;
export const MAX_ALERTS_MIN = 1;
export const MAX_ALERTS_MAX = 10;
export const MAX_ALERTS_DEFAULT = 3;
export const GROUPING_WINDOW_DEFAULT_SECONDS = 30; // Keep in sync with backend default in internal/alerts/alerts.go
export const GROUPING_WINDOW_DEFAULT_MINUTES = Math.max(
  0,
  Math.round(GROUPING_WINDOW_DEFAULT_SECONDS / 60),
);

export const clampCooldownMinutes = (value?: number): number => {
  const numericValue = typeof value === 'number' ? value : Number.NaN;
  if (!Number.isFinite(numericValue)) {
    return COOLDOWN_MIN_MINUTES;
  }
  return Math.min(COOLDOWN_MAX_MINUTES, Math.max(COOLDOWN_MIN_MINUTES, numericValue));
};

export const fallbackCooldownMinutes = (value?: number): number => {
  const numericValue = typeof value === 'number' ? value : Number.NaN;
  if (!Number.isFinite(numericValue) || numericValue <= 0) {
    return COOLDOWN_DEFAULT_MINUTES;
  }
  return clampCooldownMinutes(numericValue);
};

export const filterIncidentEvents = (
  events: IncidentEvent[] | undefined,
  filters: Set<string>,
): IncidentEvent[] => {
  if (!events || events.length === 0) {
    return [];
  }
  if (filters.size === 0 || filters.size === INCIDENT_EVENT_TYPES.length) {
    return events;
  }
  return events.filter((event) => filters.has(event.type));
};
