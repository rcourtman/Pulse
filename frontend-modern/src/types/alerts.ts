import type { FilterStack } from '@/utils/searchQuery';

export interface HysteresisThreshold {
  trigger: number;
  clear: number;
}

export interface AlertThresholds {
  cpu?: HysteresisThreshold;
  memory?: HysteresisThreshold;
  disk?: HysteresisThreshold;
  diskRead?: HysteresisThreshold;
  diskWrite?: HysteresisThreshold;
  networkIn?: HysteresisThreshold;
  networkOut?: HysteresisThreshold;
  // Legacy support for backward compatibility
  cpuLegacy?: number;
  memoryLegacy?: number;
  diskLegacy?: number;
  diskReadLegacy?: number;
  diskWriteLegacy?: number;
  networkInLegacy?: number;
  networkOutLegacy?: number;
  // Allow indexing with string
  [key: string]: HysteresisThreshold | number | undefined;
}

export interface CustomAlertRule {
  id: string;
  name: string;
  description?: string;
  filterConditions: FilterStack;
  thresholds: AlertThresholds;
  priority: number;
  enabled: boolean;
  notifications: {
    email?: {
      enabled: boolean;
      recipients: string[];
    };
    webhook?: {
      enabled: boolean;
      url: string;
    };
  };
  createdAt: string;
  updatedAt: string;
}

export interface AlertConfig {
  enabled: boolean;
  guestDefaults: AlertThresholds;
  nodeDefaults: AlertThresholds;
  storageDefault: HysteresisThreshold;
  customRules?: CustomAlertRule[];
  overrides: Record<string, AlertThresholds>; // key: resource ID
  minimumDelta?: number;
  suppressionWindow?: number;
  hysteresisMargin?: number;
  notifications?: {
    email?: {
      server: string;
      port: number;
      username: string;
      password: string;
      from: string;
      tls: boolean;
    };
    webhooks?: Array<{
      id: string;
      name: string;
      url: string;
      enabled: boolean;
    }>;
  };
  schedule?: {
    enabled?: boolean;
    quietHours?: {
      enabled: boolean;
      start: string;
      end: string;
      days: number[] | Record<string, boolean>;
    };
    cooldown?: number;
    groupingWindow?: number;
  };
}

// Priority levels:
// 0: Global defaults
// 1-99: Reserved for system rules
// 100+: Custom user rules
// 1000+: Guest-specific overrides