import { createSignal, Show, For, createMemo, createEffect, onMount } from 'solid-js';
import { EmailProviderSelect } from '@/components/Alerts/EmailProviderSelect';
import { WebhookConfig } from '@/components/Alerts/WebhookConfig';
import { ThresholdsTable } from '@/components/Alerts/ThresholdsTable';
import type { RawOverrideConfig } from '@/types/alerts';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import { useWebSocket } from '@/App';
import { showSuccess, showError } from '@/utils/toast';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI, Webhook } from '@/api/notifications';
import type { EmailConfig } from '@/api/notifications';
import type { HysteresisThreshold } from '@/types/alerts';
import type { Alert, State, VM, Container, DockerHost, DockerContainer } from '@/types/api';
import { useNavigate, useLocation } from '@solidjs/router';

type AlertTab = 'overview' | 'thresholds' | 'destinations' | 'schedule' | 'history';

const ALERT_HEADER_META: Record<AlertTab, { title: string; description: string }> = {
  overview: {
    title: 'Alerts Overview',
    description: 'Monitor active alerts, acknowledgements, and recent status changes across platforms.',
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
    description: 'Set quiet hours and maintenance windows to suppress alerts when expected changes occur.',
  },
  history: {
    title: 'Alert History',
    description: 'Review previously triggered alerts and their resolution timeline.',
  },
};

// Store reference interfaces
interface DestinationsRef {
  emailConfig?: () => EmailConfig;
}

// Override interface for both guests and nodes
type OverrideType =
  | 'guest'
  | 'node'
  | 'storage'
  | 'pbs'
  | 'dockerHost'
  | 'dockerContainer';

interface Override {
  id: string; // Full ID (e.g. "Main-node1-105" for guest, "node-node1" for node, "pbs-name" for PBS)
  name: string; // Display name
  type: OverrideType;
  resourceType?: string; // VM, CT, Node, Storage, or PBS
  vmid?: number; // Only for guests
  node?: string; // Node name (for guests and storage), undefined for nodes themselves
  instance?: string;
  disabled?: boolean; // Completely disable alerts for this guest/storage
  disableConnectivity?: boolean; // For nodes - disable offline/connectivity alerts
  thresholds: {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
    usage?: number; // For storage devices
  };
}

// Local email config with UI-specific fields
interface UIEmailConfig {
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

interface QuietHoursConfig {
  enabled: boolean;
  start: string;
  end: string;
  timezone: string;
  days: Record<string, boolean>;
}

interface CooldownConfig {
  enabled: boolean;
  minutes: number;
  maxAlerts: number;
}

interface GroupingConfig {
  enabled: boolean;
  window: number;
  maxGroupSize?: number;
  byNode?: boolean;
  byGuest?: boolean;
}

type EscalationNotifyTarget = 'email' | 'webhook' | 'all';

interface EscalationLevel {
  after: number;
  notify: EscalationNotifyTarget;
}

interface EscalationConfig {
  enabled: boolean;
  levels: EscalationLevel[];
}

const getLocalTimezone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

const createDefaultQuietHours = (): QuietHoursConfig => ({
  enabled: false,
  start: '22:00',
  end: '08:00',
  timezone: getLocalTimezone(),
  days: {
    monday: true,
    tuesday: true,
    wednesday: true,
    thursday: true,
    friday: true,
    saturday: false,
    sunday: false,
  },
});

const createDefaultCooldown = (): CooldownConfig => ({
  enabled: true,
  minutes: 30,
  maxAlerts: 3,
});

const createDefaultGrouping = (): GroupingConfig => ({
  enabled: true,
  window: 5,
  byNode: true,
  byGuest: false,
});

const createDefaultEscalation = (): EscalationConfig => ({
  enabled: false,
  levels: [],
});

export function Alerts() {
  const { state, activeAlerts, updateAlert, removeAlerts } = useWebSocket();
  const navigate = useNavigate();
  const location = useLocation();

  const tabSegments: Record<AlertTab, string> = {
    overview: 'overview',
    thresholds: 'thresholds',
    destinations: 'destinations',
    schedule: 'schedule',
    history: 'history',
  };

  const pathForTab = (tab: AlertTab) => {
    const segment = tabSegments[tab];
    return segment ? `/alerts/${segment}` : '/alerts';
  };

  const tabFromPath = (pathname: string): AlertTab => {
    const normalizedPath = pathname.replace(/\/+$/, '') || '/alerts';
    const segments = normalizedPath.split('/').filter(Boolean);

    if (segments[0] !== 'alerts') {
      return 'overview';
    }

    const segment = segments[1] ?? '';
    if (!segment) {
      return 'overview';
    }

    const entry = (Object.entries(tabSegments) as [AlertTab, string][])
      .find(([, value]) => value === segment);

    if (entry) {
      return entry[0];
    }

    if (segment === 'custom-rules') {
      return 'thresholds';
    }

    return 'overview';
  };

  const [activeTab, setActiveTab] = createSignal<AlertTab>(tabFromPath(location.pathname));

  const headerMeta = () =>
    ALERT_HEADER_META[activeTab()] ?? {
      title: 'Alerts',
      description: 'Manage alerting configuration.',
    };

  createEffect(() => {
    const currentPath = location.pathname;
    const tab = tabFromPath(currentPath);

    if (tab !== activeTab()) {
      setActiveTab(tab);
    }

    const expectedPath = pathForTab(tab);
    if (currentPath !== expectedPath) {
      navigate(expectedPath, { replace: true });
    }
  });

  const handleTabChange = (tab: AlertTab) => {
    const targetPath = pathForTab(tab);
    if (location.pathname !== targetPath) {
      navigate(targetPath);
    }
  };

  const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
  const [isReloadingConfig, setIsReloadingConfig] = createSignal(false);
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  // Quick tip visibility state
  const [showQuickTip, setShowQuickTip] = createSignal(
    localStorage.getItem('hideAlertsQuickTip') !== 'true',
  );

  const dismissQuickTip = () => {
    setShowQuickTip(false);
    localStorage.setItem('hideAlertsQuickTip', 'true');
  };

  // Store references to child component data
  let destinationsRef: DestinationsRef = {};

  const [overrides, setOverrides] = createSignal<Override[]>([]);
  const [rawOverridesConfig, setRawOverridesConfig] = createSignal<
    Record<string, RawOverrideConfig>
  >({}); // Store raw config

  // Email configuration state moved to parent to persist across tab changes
  const [emailConfig, setEmailConfig] = createSignal<UIEmailConfig>({
    enabled: false,
    provider: '',
    server: '', // Fixed: use 'server' not 'smtpHost'
    port: 587, // Fixed: use 'port' not 'smtpPort'
    username: '',
    password: '',
    from: '',
    to: [] as string[],
    tls: true,
    startTLS: false,
    replyTo: '',
    maxRetries: 3,
    retryDelay: 5,
    rateLimit: 60,
  });

  // Schedule configuration state moved to parent to persist across tab changes
  const [scheduleQuietHours, setScheduleQuietHours] =
    createSignal<QuietHoursConfig>(createDefaultQuietHours());

  const [scheduleCooldown, setScheduleCooldown] =
    createSignal<CooldownConfig>(createDefaultCooldown());

  const [scheduleGrouping, setScheduleGrouping] =
    createSignal<GroupingConfig>(createDefaultGrouping());

  const [scheduleEscalation, setScheduleEscalation] =
    createSignal<EscalationConfig>(createDefaultEscalation());

  // Set up destinationsRef.emailConfig function immediately
  destinationsRef.emailConfig = () => {
    const config = emailConfig();
    return {
      enabled: config.enabled,
      provider: config.provider,
      server: config.server, // Fixed: use correct property name
      port: config.port, // Fixed: use correct property name
      username: config.username,
      password: config.password,
      from: config.from,
      to: config.to,
      tls: config.tls,
      startTLS: config.startTLS,
    } as EmailConfig;
  };

  // Process raw overrides config when state changes
  createEffect(() => {
    // Skip this effect if there are unsaved changes to prevent losing focus
    if (hasUnsavedChanges()) {
      return;
    }

    const rawConfig = rawOverridesConfig();
    if (
      Object.keys(rawConfig).length > 0 &&
      state.nodes &&
      state.vms &&
      state.containers &&
      state.storage
    ) {
      // Convert overrides object to array format
      const overridesList: Override[] = [];

      const dockerHostsList: DockerHost[] = state.dockerHosts || [];
      const dockerHostMap = new Map<string, DockerHost>();
      const dockerContainerMap = new Map<string, { host: DockerHost; container: DockerContainer }>();

      dockerHostsList.forEach((host) => {
        dockerHostMap.set(host.id, host);
        (host.containers || []).forEach((container) => {
          const resourceId = `docker:${host.id}/${container.id}`;
          dockerContainerMap.set(resourceId, { host, container });
        });
      });

      Object.entries(rawConfig).forEach(([key, thresholds]) => {
        // Docker host override stored by host ID
        const dockerHost = dockerHostMap.get(key);
        if (dockerHost) {
          overridesList.push({
            id: key,
            name: dockerHost.displayName?.trim() || dockerHost.hostname || dockerHost.id,
            type: 'dockerHost',
            resourceType: 'Docker Host',
            disableConnectivity: thresholds.disableConnectivity || false,
            thresholds: extractTriggerValues(thresholds),
          });
          return;
        }

        // Docker container override stored as docker:hostId/containerId
        const dockerContainer = dockerContainerMap.get(key);
        if (dockerContainer) {
          const { host, container } = dockerContainer;
          const containerName = container.name?.replace(/^\/+/, '') || container.id;
          overridesList.push({
            id: key,
            name: containerName,
            type: 'dockerContainer',
            resourceType: 'Docker Container',
            node: host.hostname,
            instance: host.displayName,
            disabled: thresholds.disabled || false,
            disableConnectivity: thresholds.disableConnectivity || false,
            thresholds: extractTriggerValues(thresholds),
          });
          return;
        }

        if (key.startsWith('docker:')) {
          // Handle docker overrides where the host/container is no longer reporting
          const [, rest] = key.split(':', 2);
          const [hostId, containerId] = (rest || '').split('/', 2);

          if (containerId) {
            overridesList.push({
              id: key,
              name: containerId,
              type: 'dockerContainer',
              resourceType: 'Docker Container',
              node: hostId,
              disabled: thresholds.disabled || false,
              disableConnectivity: thresholds.disableConnectivity || false,
              thresholds: extractTriggerValues(thresholds),
            });
            return;
          }

          overridesList.push({
            id: hostId || key,
            name: hostId || key,
            type: 'dockerHost',
            resourceType: 'Docker Host',
            disableConnectivity: thresholds.disableConnectivity || false,
            thresholds: extractTriggerValues(thresholds),
          });
          return;
        }

        // Check if it's a PBS server override (starts with "pbs-")
        if (key.startsWith('pbs-')) {
          const pbs = (state.pbs || []).find((p) => p.id === key);
          if (pbs) {
            overridesList.push({
              id: key,
              name: pbs.name,
              type: 'pbs',
              resourceType: 'PBS',
              disableConnectivity: thresholds.disableConnectivity || false,
              thresholds: extractTriggerValues(thresholds),
            });
          }
        } else {
          // Check if it's a node override by looking for matching node
          const node = (state.nodes || []).find((n) => n.id === key);
          if (node) {
            overridesList.push({
              id: key,
              name: node.name,
              type: 'node',
              resourceType: 'Node',
              disableConnectivity: thresholds.disableConnectivity || false,
              thresholds: extractTriggerValues(thresholds),
            });
          } else {
            // Check if it's a storage device
            const storage = (state.storage || []).find((s) => s.id === key);
            if (storage) {
              overridesList.push({
                id: key,
                name: storage.name,
                type: 'storage',
                resourceType: 'Storage',
                node: storage.node,
                instance: storage.instance,
                disabled: thresholds.disabled || false,
                thresholds: extractTriggerValues(thresholds),
              });
            } else {
              // Find the guest by matching the full ID
              const vm = (state.vms || []).find((g) => g.id === key);
              const container = (state.containers || []).find((g) => g.id === key);
              const guest = vm || container;
              if (guest) {
                overridesList.push({
                  id: key,
                  name: guest.name,
                  type: 'guest',
                  resourceType: guest.type === 'qemu' ? 'VM' : 'CT',
                  vmid: guest.vmid,
                  node: guest.node,
                  instance: guest.instance,
                  disabled: thresholds.disabled || false,
                  thresholds: extractTriggerValues(thresholds),
                });
              }
            }
          }
        }
      });

      // Only update if there's an actual change to prevent losing edit state
      const currentOverrides = overrides();
      const hasChanged =
        overridesList.length !== currentOverrides.length ||
        overridesList.some((newOverride) => {
          const existing = currentOverrides.find((o) => o.id === newOverride.id);
          if (!existing) return true;
          // Check both thresholds and disableConnectivity for nodes/PBS
          const thresholdsChanged =
            JSON.stringify(newOverride.thresholds) !== JSON.stringify(existing.thresholds);
          const connectivityChanged =
            (newOverride.type === 'node' || newOverride.type === 'pbs') &&
            newOverride.disableConnectivity !== existing.disableConnectivity;
          const disabledChanged =
            (newOverride.type === 'guest' || newOverride.type === 'storage') &&
            newOverride.disabled !== existing.disabled;
          return thresholdsChanged || connectivityChanged || disabledChanged;
        });

      if (hasChanged) {
        setOverrides(overridesList);
      }
    }
  });

  const loadAlertConfiguration = async (options: { notify?: boolean } = {}) => {
    setIsReloadingConfig(true);
    setHasUnsavedChanges(false);

    // Reset to defaults before applying server state
    setGuestDefaults({
      cpu: 80,
      memory: 85,
      disk: 90,
      diskRead: -1,
      diskWrite: -1,
      networkIn: -1,
      networkOut: -1,
    });
    setGuestDisableConnectivity(false);
    setNodeDefaults({
      cpu: 80,
      memory: 85,
      disk: 90,
      temperature: 80,
    });
    setStorageDefault(85);
    setTimeThreshold(0);
    setTimeThresholds({ guest: 10, node: 15, storage: 30, pbs: 30 });
    setScheduleQuietHours(createDefaultQuietHours());
    setScheduleCooldown(createDefaultCooldown());
    setScheduleGrouping(createDefaultGrouping());
    setScheduleEscalation(createDefaultEscalation());

    setEmailConfig({
      enabled: false,
      provider: '',
      server: '',
      port: 587,
      username: '',
      password: '',
      from: '',
      to: [],
      tls: true,
      startTLS: false,
      replyTo: '',
      maxRetries: 3,
      retryDelay: 5,
      rateLimit: 60,
    });

    try {
      const config = await AlertsAPI.getConfig();

      if (config.guestDefaults) {
        setGuestDefaults({
          cpu: getTriggerValue(config.guestDefaults.cpu) ?? 80,
          memory: getTriggerValue(config.guestDefaults.memory) ?? 85,
          disk: getTriggerValue(config.guestDefaults.disk) ?? 90,
          diskRead: getTriggerValue(config.guestDefaults.diskRead) ?? -1,
          diskWrite: getTriggerValue(config.guestDefaults.diskWrite) ?? -1,
          networkIn: getTriggerValue(config.guestDefaults.networkIn) ?? -1,
          networkOut: getTriggerValue(config.guestDefaults.networkOut) ?? -1,
        });
        setGuestDisableConnectivity(Boolean(config.guestDefaults.disableConnectivity));
      } else {
        setGuestDisableConnectivity(false);
      }

      if (config.nodeDefaults) {
        setNodeDefaults({
          cpu: getTriggerValue(config.nodeDefaults.cpu) ?? 80,
          memory: getTriggerValue(config.nodeDefaults.memory) ?? 85,
          disk: getTriggerValue(config.nodeDefaults.disk) ?? 90,
          temperature: getTriggerValue(config.nodeDefaults.temperature) ?? 80,
        });
      }

      if (config.dockerDefaults) {
        setDockerDefaults({
          cpu: getTriggerValue(config.dockerDefaults.cpu) ?? 80,
          memory: getTriggerValue(config.dockerDefaults.memory) ?? 85,
          restartCount: config.dockerDefaults.restartCount ?? 3,
          restartWindow: config.dockerDefaults.restartWindow ?? 300,
          memoryWarnPct: config.dockerDefaults.memoryWarnPct ?? 90,
          memoryCriticalPct: config.dockerDefaults.memoryCriticalPct ?? 95,
        });
      }

      if (config.storageDefault) {
        setStorageDefault(getTriggerValue(config.storageDefault) ?? 85);
      }
      if (config.timeThreshold !== undefined) {
        setTimeThreshold(config.timeThreshold);
      }
      if (config.timeThresholds) {
        setTimeThresholds({
          guest: config.timeThresholds.guest ?? 10,
          node: config.timeThresholds.node ?? 15,
          storage: config.timeThresholds.storage ?? 30,
          pbs: config.timeThresholds.pbs ?? 30,
        });
      } else if (config.timeThreshold !== undefined && config.timeThreshold > 0) {
        setTimeThresholds({
          guest: config.timeThreshold,
          node: config.timeThreshold,
          storage: config.timeThreshold,
          pbs: config.timeThreshold,
        });
      }

      // Load global disable flags
      setDisableAllNodes(config.disableAllNodes ?? false);
      setDisableAllGuests(config.disableAllGuests ?? false);
      setDisableAllStorage(config.disableAllStorage ?? false);
      setDisableAllPBS(config.disableAllPBS ?? false);
      setDisableAllDockerHosts(config.disableAllDockerHosts ?? false);
      setDisableAllDockerContainers(config.disableAllDockerContainers ?? false);

      // Load global disable offline alerts flags
      setDisableAllNodesOffline(config.disableAllNodesOffline ?? false);
      setDisableAllGuestsOffline(config.disableAllGuestsOffline ?? false);
      setDisableAllPBSOffline(config.disableAllPBSOffline ?? false);
      setDisableAllDockerHostsOffline(config.disableAllDockerHostsOffline ?? false);

      setRawOverridesConfig(config.overrides || {});

      if (config.schedule) {
        if (config.schedule.quietHours) {
          const qh = config.schedule.quietHours;
          let days: Record<string, boolean>;
          if (Array.isArray(qh.days)) {
            days = {
              sunday: qh.days.includes(0),
              monday: qh.days.includes(1),
              tuesday: qh.days.includes(2),
              wednesday: qh.days.includes(3),
              thursday: qh.days.includes(4),
              friday: qh.days.includes(5),
              saturday: qh.days.includes(6),
            };
          } else {
            days = (qh.days as Record<string, boolean>) || createDefaultQuietHours().days;
          }

          setScheduleQuietHours({
            enabled: qh.enabled || false,
            start: qh.start || '22:00',
            end: qh.end || '08:00',
            timezone: qh.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
            days,
          });
        }

        if (config.schedule.cooldown !== undefined) {
          setScheduleCooldown({
            enabled: config.schedule.cooldown > 0,
            minutes: config.schedule.cooldown,
            maxAlerts: config.schedule.maxAlertsHour || 3,
          });
        }

        if (config.schedule.grouping) {
          setScheduleGrouping({
            enabled: config.schedule.grouping.enabled || false,
            window: Math.floor((config.schedule.grouping.window || 300) / 60),
            byNode:
              config.schedule.grouping.byNode !== undefined
                ? config.schedule.grouping.byNode
                : true,
            byGuest:
              config.schedule.grouping.byGuest !== undefined
                ? config.schedule.grouping.byGuest
                : false,
          });
        } else if (config.schedule.groupingWindow !== undefined) {
          setScheduleGrouping({
            enabled: config.schedule.groupingWindow > 0,
            window: Math.floor(config.schedule.groupingWindow / 60),
            byNode: true,
            byGuest: false,
          });
        }

        if (config.schedule.escalation) {
          const rawLevels = config.schedule.escalation.levels || [];
          const levels = rawLevels.map((level) => ({
            after: typeof level.after === 'number' ? level.after : 15,
            notify: (level.notify as EscalationNotifyTarget) || 'all',
          }));
          setScheduleEscalation({
            enabled: Boolean(config.schedule.escalation.enabled),
            levels,
          });
        }
      }

      try {
        const emailConfigData = await NotificationsAPI.getEmailConfig();
        setEmailConfig({
          enabled: emailConfigData.enabled,
          provider: emailConfigData.provider || '',
          server: emailConfigData.server || '',
          port: emailConfigData.port || 587,
          username: emailConfigData.username || '',
          password: emailConfigData.password || '',
          from: emailConfigData.from || '',
          to: emailConfigData.to || [],
          tls: emailConfigData.tls !== undefined ? emailConfigData.tls : true,
          startTLS: emailConfigData.startTLS || false,
          replyTo: '',
          maxRetries: 3,
          retryDelay: 5,
          rateLimit: 60,
        });
      } catch (emailErr) {
        console.error('Failed to load email configuration:', emailErr);
      }

      if (options.notify) {
        showSuccess('Changes discarded');
      }
    } catch (err) {
      console.error('Failed to load alert configuration:', err);
      if (options.notify) {
        showError('Failed to reload configuration');
      }
    } finally {
      setIsReloadingConfig(false);
    }
  };

  // Load existing alert configuration on mount (only once)
  onMount(() => {
    void loadAlertConfiguration();
  });

  // Reload email config when switching to destinations tab
  createEffect(() => {
    if (activeTab() === 'destinations') {
      // Reload email config from server when switching to destinations tab
      NotificationsAPI.getEmailConfig()
        .then((emailConfigData) => {
          setEmailConfig({
            enabled: emailConfigData.enabled,
            provider: emailConfigData.provider || '',
            server: emailConfigData.server || '',
            port: emailConfigData.port || 587,
            username: emailConfigData.username || '',
            password: emailConfigData.password || '',
            from: emailConfigData.from || '',
            to: emailConfigData.to || [],
            tls: emailConfigData.tls !== undefined ? emailConfigData.tls : true,
            startTLS: emailConfigData.startTLS || false,
            replyTo: '',
            maxRetries: 3,
            retryDelay: 5,
            rateLimit: 60,
          });
        })
        .catch((err) => {
          console.error('Failed to reload email configuration:', err);
        });
    }
  });

  // Get all guests from state - memoize to prevent unnecessary updates
  const allGuests = createMemo(
    () => {
      const vms = state.vms || [];
      const containers = state.containers || [];
      return [...vms, ...containers];
    },
    [],
    {
      equals: (prev, next) => {
        // Only update if the actual guest list changed
        if (prev.length !== next.length) return false;
        return prev.every((p, i) => p.vmid === next[i].vmid && p.name === next[i].name);
      },
    },
  );

  // Helper function to extract trigger value from threshold
  const getTriggerValue = (
    threshold: number | boolean | HysteresisThreshold | undefined,
  ): number => {
    if (typeof threshold === 'number') {
      return threshold; // Legacy format
    }
    if (typeof threshold === 'boolean') {
      return 0;
    }
    if (threshold && typeof threshold === 'object' && 'trigger' in threshold) {
      return threshold.trigger; // New hysteresis format
    }
    return 0; // Default fallback
  };

  // Helper to extract trigger values for all thresholds
  const extractTriggerValues = (thresholds: RawOverrideConfig): Record<string, number> => {
    const result: Record<string, number> = {};
    Object.entries(thresholds).forEach(([key, value]) => {
      // Skip non-threshold fields
      if (key === 'disabled' || key === 'disableConnectivity') return;
      result[key] = getTriggerValue(value);
    });
    return result;
  };

  // Threshold states - using trigger values for display
  const [guestDefaults, setGuestDefaults] = createSignal({
    cpu: 80,
    memory: 85,
    disk: 90,
    diskRead: -1,
    diskWrite: -1,
    networkIn: -1,
    networkOut: -1,
  });
  const [guestDisableConnectivity, setGuestDisableConnectivity] = createSignal(false);

  const [nodeDefaults, setNodeDefaults] = createSignal({
    cpu: 80,
    memory: 85,
    disk: 90,
    temperature: 80,
  });

  const [dockerDefaults, setDockerDefaults] = createSignal({
    cpu: 80,
    memory: 85,
    restartCount: 3,
    restartWindow: 300,
    memoryWarnPct: 90,
    memoryCriticalPct: 95,
  });

  const [storageDefault, setStorageDefault] = createSignal(85);
  const [timeThreshold, setTimeThreshold] = createSignal(0); // Legacy
  const [timeThresholds, setTimeThresholds] = createSignal({
    guest: 10,
    node: 15,
    storage: 30,
    pbs: 30,
  });

  // Global disable flags per resource type
  const [disableAllNodes, setDisableAllNodes] = createSignal(false);
  const [disableAllGuests, setDisableAllGuests] = createSignal(false);
  const [disableAllStorage, setDisableAllStorage] = createSignal(false);
  const [disableAllPBS, setDisableAllPBS] = createSignal(false);
  const [disableAllDockerHosts, setDisableAllDockerHosts] = createSignal(false);
  const [disableAllDockerContainers, setDisableAllDockerContainers] = createSignal(false);

  // Global disable offline alerts flags
  const [disableAllNodesOffline, setDisableAllNodesOffline] = createSignal(false);
  const [disableAllGuestsOffline, setDisableAllGuestsOffline] = createSignal(false);
  const [disableAllPBSOffline, setDisableAllPBSOffline] = createSignal(false);
  const [disableAllDockerHostsOffline, setDisableAllDockerHostsOffline] = createSignal(false);

  const tabIcons: Record<AlertTab, string> = {
    overview:
      'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6',
    thresholds: 'M13 7h8m0 0v8m0-8l-8 8-4-4-6 6',
    destinations:
      'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9',
    schedule:
      'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z',
    history: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z',
  };

  const tabGroups: {
    id: 'status' | 'configuration';
    label: string;
    items: { id: AlertTab; label: string }[];
  }[] = [
    {
      id: 'status',
      label: 'Status',
      items: [
        { id: 'overview', label: 'Overview' },
        { id: 'history', label: 'History' },
      ],
    },
    {
      id: 'configuration',
      label: 'Configuration',
      items: [
        { id: 'thresholds', label: 'Thresholds' },
        { id: 'destinations', label: 'Notifications' },
        { id: 'schedule', label: 'Schedule' },
      ],
    },
  ];

  const flatTabs = tabGroups.flatMap((group) => group.items);

  return (
    <div class="space-y-4">
      {/* Header with better styling */}
      <Card padding="md">
        <SectionHeader
          title={headerMeta().title}
          description={headerMeta().description}
          size="lg"
        />
      </Card>

      {/* Save notification bar - only show when there are unsaved changes */}
      <Show when={hasUnsavedChanges() && activeTab() !== 'overview' && activeTab() !== 'history'}>
        <Card tone="warning" padding="sm" class="border-yellow-200 dark:border-yellow-800 sm:p-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex items-center gap-2 text-yellow-800 dark:text-yellow-200">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="8" x2="12" y2="12"></line>
                <line x1="12" y1="16" x2="12.01" y2="16"></line>
              </svg>
              <span class="text-sm font-medium">You have unsaved changes</span>
            </div>
            <div class="flex w-full gap-2 sm:w-auto">
              <button
                class="flex-1 px-4 py-2 text-sm text-white transition-colors sm:flex-initial bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-60 disabled:cursor-not-allowed"
                disabled={isReloadingConfig()}
                onClick={async () => {
                  try {
                    // Save alert configuration with hysteresis format
                    const createHysteresisThreshold = (
                      trigger: number,
                      clearMargin: number = 5,
                    ) => ({
                      trigger,
                      clear: Math.max(0, trigger - clearMargin),
                    });

                    const alertConfig = {
                      enabled: true,
                      // Global disable flags per resource type
                      disableAllNodes: disableAllNodes(),
                      disableAllGuests: disableAllGuests(),
                      disableAllStorage: disableAllStorage(),
                      disableAllPBS: disableAllPBS(),
                      disableAllDockerHosts: disableAllDockerHosts(),
                      disableAllDockerContainers: disableAllDockerContainers(),
                      // Global disable offline alerts flags
                      disableAllNodesOffline: disableAllNodesOffline(),
                      disableAllGuestsOffline: disableAllGuestsOffline(),
                      disableAllPBSOffline: disableAllPBSOffline(),
                      disableAllDockerHostsOffline: disableAllDockerHostsOffline(),
                      guestDefaults: {
                        cpu: createHysteresisThreshold(guestDefaults().cpu),
                        memory: createHysteresisThreshold(guestDefaults().memory),
                        disk: createHysteresisThreshold(guestDefaults().disk),
                        diskRead: createHysteresisThreshold(guestDefaults().diskRead),
                        diskWrite: createHysteresisThreshold(guestDefaults().diskWrite),
                        networkIn: createHysteresisThreshold(guestDefaults().networkIn),
                        networkOut: createHysteresisThreshold(guestDefaults().networkOut),
                        disableConnectivity: guestDisableConnectivity(),
                      },
                      nodeDefaults: {
                        cpu: createHysteresisThreshold(nodeDefaults().cpu),
                        memory: createHysteresisThreshold(nodeDefaults().memory),
                        disk: createHysteresisThreshold(nodeDefaults().disk),
                        temperature: createHysteresisThreshold(nodeDefaults().temperature),
                      },
                      dockerDefaults: {
                        cpu: createHysteresisThreshold(dockerDefaults().cpu),
                        memory: createHysteresisThreshold(dockerDefaults().memory),
                        restartCount: dockerDefaults().restartCount,
                        restartWindow: dockerDefaults().restartWindow,
                        memoryWarnPct: dockerDefaults().memoryWarnPct,
                        memoryCriticalPct: dockerDefaults().memoryCriticalPct,
                      },
                      storageDefault: createHysteresisThreshold(storageDefault()),
                      minimumDelta: 2.0,
                      suppressionWindow: 5,
                      hysteresisMargin: 5.0,
                      timeThreshold: timeThreshold() || 0, // Legacy
                      timeThresholds: timeThresholds(),
                      // Use rawOverridesConfig which is already properly formatted with disabled flags
                      overrides: rawOverridesConfig(),
                      schedule: {
                        quietHours: scheduleQuietHours(),
                        cooldown: scheduleCooldown().enabled ? scheduleCooldown().minutes : 0,
                        groupingWindow:
                          scheduleGrouping().enabled && scheduleGrouping().window
                            ? scheduleGrouping().window * 60
                            : 30, // Convert minutes to seconds
                        maxAlertsHour: scheduleCooldown().maxAlerts || 10,
                        escalation: scheduleEscalation(),
                        grouping: {
                          enabled: scheduleGrouping().enabled,
                          window: scheduleGrouping().window * 60, // Convert minutes to seconds
                          byNode: scheduleGrouping().byNode,
                          byGuest: scheduleGrouping().byGuest,
                        },
                      },
                      // Add missing required fields
                      aggregation: {
                        enabled: true,
                        timeWindow: 10,
                        countThreshold: 3,
                        similarityWindow: 5.0,
                      },
                      flapping: {
                        enabled: true,
                        threshold: 5,
                        window: 10,
                        suppressionTime: 30,
                        minStability: 0.8,
                      },
                      ioNormalization: {
                        enabled: true,
                        vmDiskMax: 500.0,
                        containerDiskMax: 300.0,
                        networkMax: 1000.0,
                      },
                    };

                    await AlertsAPI.updateConfig(alertConfig);

                    // Save email config if it exists (regardless of active tab)
                    if (destinationsRef.emailConfig) {
                      const emailData = destinationsRef.emailConfig();
                      await NotificationsAPI.updateEmailConfig(emailData);
                    }

                    setHasUnsavedChanges(false);
                    showSuccess('Configuration saved successfully!');
                  } catch (err) {
                    console.error('Failed to save configuration:', err);
                    showError(err instanceof Error ? err.message : 'Failed to save configuration');
                  }
                }}
              >
                Save Changes
              </button>
              <button
                class="flex-1 px-4 py-2 text-sm transition-colors border border-gray-300 rounded-lg text-gray-700 dark:border-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 sm:flex-initial disabled:opacity-60 disabled:cursor-not-allowed"
                disabled={isReloadingConfig()}
                onClick={async () => {
                  await loadAlertConfiguration({ notify: true });
                }}
              >
                {isReloadingConfig() ? 'Discarding...' : 'Discard'}
              </button>
            </div>
          </div>
        </Card>
      </Show>

      <Card padding="none" class="lg:flex">
        <div class="hidden lg:inline-block lg:w-72 border-b border-gray-200 dark:border-gray-700 lg:border-b-0 lg:border-r lg:border-gray-200 dark:lg:border-gray-700 lg:align-top">
          <div class="sticky top-24 space-y-6 px-5 py-6">
            <For each={tabGroups}>
              {(group) => (
                <div class="space-y-2">
                  <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {group.label}
                  </p>
                  <div class="space-y-1.5">
                    <For each={group.items}>
                      {(item) => (
                        <button
                          type="button"
                          aria-current={activeTab() === item.id ? 'page' : undefined}
                          class={`flex w-full items-center rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                            activeTab() === item.id
                              ? 'bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-200'
                              : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700/60 dark:hover:text-gray-100'
                          }`}
                          onClick={() => handleTabChange(item.id)}
                        >
                          <span class="truncate">{item.label}</span>
                        </button>
                      )}
                    </For>
                  </div>
                </div>
              )}
            </For>
          </div>
        </div>

        <div class="flex-1">
          <Show when={flatTabs.length > 0}>
            <div class="lg:hidden border-b border-gray-200 dark:border-gray-700">
              <div class="p-1">
                <div
                  class="flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5 w-full overflow-x-auto"
                  style="-webkit-overflow-scrolling: touch;"
                >
                  <For each={flatTabs}>
                    {(tab) => (
                      <button
                        type="button"
                        class={`flex-1 px-3 py-2 text-xs font-medium rounded-md transition-all whitespace-nowrap ${
                          activeTab() === tab.id
                            ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                        }`}
                        onClick={() => handleTabChange(tab.id)}
                      >
                        {tab.label}
                      </button>
                    )}
                  </For>
                </div>
              </div>
            </div>
          </Show>

          {/* Tab Content */}
          <div class="p-3 sm:p-6">
            <Show when={activeTab() === 'overview'}>
              <OverviewTab
                overrides={overrides()}
                activeAlerts={activeAlerts}
                updateAlert={updateAlert}
              showQuickTip={showQuickTip}
              dismissQuickTip={dismissQuickTip}
              showAcknowledged={showAcknowledged}
              setShowAcknowledged={setShowAcknowledged}
            />
          </Show>

          <Show when={activeTab() === 'thresholds'}>
            <ThresholdsTab
              overrides={overrides}
              setOverrides={setOverrides}
              rawOverridesConfig={rawOverridesConfig}
              setRawOverridesConfig={setRawOverridesConfig}
              allGuests={allGuests}
              state={state}
              guestDefaults={guestDefaults}
              guestDisableConnectivity={guestDisableConnectivity}
              setGuestDefaults={setGuestDefaults}
              setGuestDisableConnectivity={setGuestDisableConnectivity}
              nodeDefaults={nodeDefaults}
              setNodeDefaults={setNodeDefaults}
              dockerDefaults={dockerDefaults}
              setDockerDefaults={setDockerDefaults}
              storageDefault={storageDefault}
              setStorageDefault={setStorageDefault}
              timeThreshold={timeThreshold}
              setTimeThreshold={setTimeThreshold}
              timeThresholds={timeThresholds}
              setTimeThresholds={setTimeThresholds}
              activeAlerts={activeAlerts}
              setHasUnsavedChanges={setHasUnsavedChanges}
              hasUnsavedChanges={hasUnsavedChanges}
              removeAlerts={removeAlerts}
              disableAllNodes={disableAllNodes}
              setDisableAllNodes={setDisableAllNodes}
              disableAllGuests={disableAllGuests}
              setDisableAllGuests={setDisableAllGuests}
              disableAllStorage={disableAllStorage}
              setDisableAllStorage={setDisableAllStorage}
              disableAllPBS={disableAllPBS}
              setDisableAllPBS={setDisableAllPBS}
              disableAllDockerHosts={disableAllDockerHosts}
              setDisableAllDockerHosts={setDisableAllDockerHosts}
              disableAllDockerContainers={disableAllDockerContainers}
              setDisableAllDockerContainers={setDisableAllDockerContainers}
              disableAllNodesOffline={disableAllNodesOffline}
              setDisableAllNodesOffline={setDisableAllNodesOffline}
              disableAllGuestsOffline={disableAllGuestsOffline}
              setDisableAllGuestsOffline={setDisableAllGuestsOffline}
              disableAllPBSOffline={disableAllPBSOffline}
              setDisableAllPBSOffline={setDisableAllPBSOffline}
              disableAllDockerHostsOffline={disableAllDockerHostsOffline}
              setDisableAllDockerHostsOffline={setDisableAllDockerHostsOffline}
            />
          </Show>

          <Show when={activeTab() === 'destinations'}>
            <DestinationsTab
              ref={destinationsRef}
              hasUnsavedChanges={hasUnsavedChanges}
              setHasUnsavedChanges={setHasUnsavedChanges}
              emailConfig={emailConfig}
              setEmailConfig={setEmailConfig}
            />
          </Show>

          <Show when={activeTab() === 'schedule'}>
            <ScheduleTab
              hasUnsavedChanges={hasUnsavedChanges}
              setHasUnsavedChanges={setHasUnsavedChanges}
              quietHours={scheduleQuietHours}
              setQuietHours={setScheduleQuietHours}
              cooldown={scheduleCooldown}
              setCooldown={setScheduleCooldown}
              grouping={scheduleGrouping}
              setGrouping={setScheduleGrouping}
              escalation={scheduleEscalation}
              setEscalation={setScheduleEscalation}
            />
          </Show>

          <Show when={activeTab() === 'history'}>
            <HistoryTab />
          </Show>
          </div>
        </div>
      </Card>
    </div>
  );
}

// Overview Tab - Shows current alert status
function OverviewTab(props: {
  overrides: Override[];
  activeAlerts: Record<string, Alert>;
  updateAlert: (alertId: string, updates: Partial<Alert>) => void;
  showQuickTip: () => boolean;
  dismissQuickTip: () => void;
  showAcknowledged: () => boolean;
  setShowAcknowledged: (value: boolean) => void;
}) {
  // Loading states for buttons
  const [processingAlerts, setProcessingAlerts] = createSignal<Set<string>>(new Set());

  // Get alert stats from actual active alerts
  const alertStats = createMemo(() => {
    // Access the store properly for reactivity
    const alertIds = Object.keys(props.activeAlerts);
    const alerts = alertIds.map((id) => props.activeAlerts[id]);
    return {
      active: alerts.filter((a) => !a.acknowledged).length,
      acknowledged: alerts.filter((a) => a.acknowledged).length,
      total24h: alerts.length, // In real app, would filter by time
      overrides: props.overrides.length,
    };
  });

  const filteredAlerts = createMemo(() => {
    const alerts = Object.values(props.activeAlerts);
    // Sort: unacknowledged first, then by start time (newest first)
    return alerts
      .filter((alert) => props.showAcknowledged() || !alert.acknowledged)
      .sort((a, b) => {
        // Acknowledged status comparison first
        if (a.acknowledged !== b.acknowledged) {
          return a.acknowledged ? 1 : -1; // Unacknowledged first
        }
        // Then by time
        return new Date(b.startTime).getTime() - new Date(a.startTime).getTime();
      });
  });

  return (
    <div class="space-y-6">
      {/* Stats Cards */}
      <div class="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <Card padding="sm" class="sm:p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Active Alerts</p>
              <p class="text-xl sm:text-2xl font-semibold text-gray-600 dark:text-gray-300">
                {alertStats().active}
              </p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-red-100 dark:bg-red-900/50 rounded-full flex items-center justify-center">
              <svg
                width="16"
                height="16"
                class="sm:w-5 sm:h-5 text-red-600 dark:text-red-400"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
                <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
              </svg>
            </div>
          </div>
        </Card>

        <Card padding="sm" class="sm:p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Acknowledged</p>
              <p class="text-xl sm:text-2xl font-semibold text-yellow-600 dark:text-yellow-400">
                {alertStats().acknowledged}
              </p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-yellow-100 dark:bg-yellow-900/50 rounded-full flex items-center justify-center">
              <svg
                width="16"
                height="16"
                class="sm:w-5 sm:h-5 text-yellow-600 dark:text-yellow-400"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M9 11L12 14L22 4"></path>
                <path d="M21 12V19C21 20.1046 20.1046 21 19 21H5C3.89543 21 3 20.1046 3 19V5C3 3.89543 3.89543 3 5 3H16"></path>
              </svg>
            </div>
          </div>
        </Card>

        <Card padding="sm" class="sm:p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Last 24 Hours</p>
              <p class="text-xl sm:text-2xl font-semibold text-gray-700 dark:text-gray-300">
                {alertStats().total24h}
              </p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-gray-200 dark:bg-gray-600 rounded-full flex items-center justify-center">
              <svg
                width="16"
                height="16"
                class="sm:w-5 sm:h-5 text-gray-600 dark:text-gray-400"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <polyline points="12 6 12 12 16 14"></polyline>
              </svg>
            </div>
          </div>
        </Card>

        <Card padding="sm" class="sm:p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Guest Overrides</p>
              <p class="text-xl sm:text-2xl font-semibold text-blue-600 dark:text-blue-400">
                {alertStats().overrides}
              </p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-blue-100 dark:bg-blue-900/50 rounded-full flex items-center justify-center">
              <svg
                width="16"
                height="16"
                class="sm:w-5 sm:h-5 text-blue-600 dark:text-blue-400"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
              </svg>
            </div>
          </div>
        </Card>
      </div>

      {/* Recent Alerts */}
      <div>
        <SectionHeader title="Active Alerts" size="md" class="mb-3" />
        <Show
          when={Object.keys(props.activeAlerts).length > 0}
          fallback={
            <div class="text-center py-8 text-gray-500 dark:text-gray-400">
              <p class="text-sm">No active alerts</p>
              <p class="text-xs mt-1">Alerts will appear here when thresholds are exceeded</p>
            </div>
          }
        >
          {/* Simple View Toggle - only show if there are acknowledged alerts */}
          <Show when={alertStats().acknowledged > 0}>
            <div class="flex justify-end p-2 bg-gray-50 dark:bg-gray-800 rounded-t-lg border border-gray-200 dark:border-gray-700">
              <button
                onClick={() => props.setShowAcknowledged(!props.showAcknowledged())}
                class="text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
              >
                {props.showAcknowledged() ? 'Hide' : 'Show'} acknowledged
              </button>
            </div>
          </Show>
          <div class="space-y-2">
            <Show when={filteredAlerts().length === 0}>
              <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                {props.showAcknowledged() ? 'No active alerts' : 'No unacknowledged alerts'}
              </div>
            </Show>
            <For each={filteredAlerts()}>
              {(alert) => (
                <div
                  class={`border rounded-lg p-4 transition-all ${
                    processingAlerts().has(alert.id) ? 'opacity-50' : ''
                  } ${
                    alert.acknowledged
                      ? 'opacity-60 border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/20'
                      : alert.level === 'critical'
                        ? 'border-red-300 dark:border-red-800 bg-red-50 dark:bg-red-900/20'
                        : 'border-yellow-300 dark:border-yellow-800 bg-yellow-50 dark:bg-yellow-900/20'
                  }`}
                >
                  <div class="flex flex-col sm:flex-row sm:items-start">
                    <div class="flex items-start flex-1">
                      {/* Status icon */}
                      <div
                        class={`mr-3 mt-0.5 transition-all ${
                          alert.acknowledged
                            ? 'text-green-600 dark:text-green-400'
                            : alert.level === 'critical'
                              ? 'text-red-600 dark:text-red-400'
                              : 'text-yellow-600 dark:text-yellow-400'
                        }`}
                      >
                        {alert.acknowledged ? (
                          // Checkmark for acknowledged
                          <svg
                            class="w-5 h-5"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                            />
                          </svg>
                        ) : (
                          // Warning/Alert icon
                          <svg
                            class="w-5 h-5"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                            />
                          </svg>
                        )}
                      </div>
                      <div class="flex-1 min-w-0">
                        <div class="flex flex-wrap items-center gap-2">
                          <span
                            class={`text-sm font-medium truncate ${
                              alert.level === 'critical'
                                ? 'text-red-700 dark:text-red-400'
                                : 'text-yellow-700 dark:text-yellow-400'
                            }`}
                          >
                            {alert.resourceName}
                          </span>
                          <span class="text-xs text-gray-600 dark:text-gray-400">
                            ({alert.type})
                          </span>
                          <Show when={alert.node}>
                            <span class="text-xs text-gray-500 dark:text-gray-500">
                              on {alert.node}
                            </span>
                          </Show>
                          <Show when={alert.acknowledged}>
                            <span class="px-2 py-0.5 text-xs bg-yellow-200 dark:bg-yellow-800 text-yellow-800 dark:text-yellow-200 rounded">
                              Acknowledged
                            </span>
                          </Show>
                        </div>
                        <p class="text-sm text-gray-700 dark:text-gray-300 mt-1 break-words">
                          {alert.message}
                        </p>
                        <p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                          Started: {new Date(alert.startTime).toLocaleString()}
                        </p>
                      </div>
                    </div>
                    <div class="flex gap-2 mt-3 sm:mt-0 sm:ml-4 self-end sm:self-start">
                      <button
                        class={`px-3 py-1.5 text-xs font-medium border rounded-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed ${
                          alert.acknowledged
                            ? 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600'
                            : 'bg-white dark:bg-gray-700 text-yellow-700 dark:text-yellow-300 border-yellow-300 dark:border-yellow-700 hover:bg-yellow-50 dark:hover:bg-yellow-900/20'
                        }`}
                        disabled={processingAlerts().has(alert.id)}
                        onClick={async (e) => {
                          e.preventDefault();
                          e.stopPropagation();

                          // Prevent double-clicks
                          if (processingAlerts().has(alert.id)) return;

                          setProcessingAlerts((prev) => new Set(prev).add(alert.id));

                          // Store current state to avoid race conditions
                          const wasAcknowledged = alert.acknowledged;

                          try {
                            if (wasAcknowledged) {
                              // Call API first, only update local state if successful
                              await AlertsAPI.unacknowledge(alert.id);
                              // Only update local state after successful API call
                              props.updateAlert(alert.id, {
                                acknowledged: false,
                                ackTime: undefined,
                                ackUser: undefined,
                              });
                              showSuccess('Alert restored');
                            } else {
                              // Call API first, only update local state if successful
                              await AlertsAPI.acknowledge(alert.id);
                              // Only update local state after successful API call
                              props.updateAlert(alert.id, {
                                acknowledged: true,
                                ackTime: new Date().toISOString(),
                              });
                              showSuccess('Alert acknowledged');
                            }
                          } catch (err) {
                            console.error(
                              `Failed to ${wasAcknowledged ? 'unacknowledge' : 'acknowledge'} alert:`,
                              err,
                            );
                            showError(
                              `Failed to ${wasAcknowledged ? 'restore' : 'acknowledge'} alert`,
                            );
                            // Don't update local state on error - let WebSocket keep the correct state
                          } finally {
                            // Keep button disabled for longer to prevent race conditions with WebSocket updates
                            setTimeout(() => {
                              setProcessingAlerts((prev) => {
                                const next = new Set(prev);
                                next.delete(alert.id);
                                return next;
                              });
                            }, 1500); // 1.5 seconds to allow server to process and WebSocket to sync
                          }
                        }}
                      >
                        {processingAlerts().has(alert.id)
                          ? 'Processing...'
                          : alert.acknowledged
                            ? 'Unacknowledge'
                            : 'Acknowledge'}
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>
      </div>
    </div>
  );
}

// Thresholds Tab - Improved design
interface ThresholdsTabProps {
  allGuests: () => (VM | Container)[];
  state: State;
  guestDefaults: () => Record<string, number>;
  nodeDefaults: () => Record<string, number>;
  dockerDefaults: () => { cpu: number; memory: number; restartCount: number; restartWindow: number; memoryWarnPct: number; memoryCriticalPct: number };
  storageDefault: () => number;
  timeThreshold: () => number;
  timeThresholds: () => { guest: number; node: number; storage: number; pbs: number };
  overrides: () => Override[];
  rawOverridesConfig: () => Record<string, RawOverrideConfig>;
  setGuestDefaults: (
    value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>),
  ) => void;
  guestDisableConnectivity: () => boolean;
  setGuestDisableConnectivity: (value: boolean) => void;
  setNodeDefaults: (
    value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>),
  ) => void;
  setDockerDefaults: (
    value: { cpu: number; memory: number; restartCount: number; restartWindow: number; memoryWarnPct: number; memoryCriticalPct: number } | ((prev: { cpu: number; memory: number; restartCount: number; restartWindow: number; memoryWarnPct: number; memoryCriticalPct: number }) => { cpu: number; memory: number; restartCount: number; restartWindow: number; memoryWarnPct: number; memoryCriticalPct: number }),
  ) => void;
  setStorageDefault: (value: number) => void;
  setTimeThreshold: (value: number) => void;
  setTimeThresholds: (value: { guest: number; node: number; storage: number; pbs: number }) => void;
  setOverrides: (value: Override[]) => void;
  setRawOverridesConfig: (value: Record<string, RawOverrideConfig>) => void;
  activeAlerts: Record<string, Alert>;
  setHasUnsavedChanges: (value: boolean) => void;
  hasUnsavedChanges: () => boolean;
  removeAlerts: (predicate: (alert: Alert) => boolean) => void;
  // Global disable flags
  disableAllNodes: () => boolean;
  setDisableAllNodes: (value: boolean) => void;
  disableAllGuests: () => boolean;
  setDisableAllGuests: (value: boolean) => void;
  disableAllStorage: () => boolean;
  setDisableAllStorage: (value: boolean) => void;
  disableAllPBS: () => boolean;
  setDisableAllPBS: (value: boolean) => void;
  disableAllDockerHosts: () => boolean;
  setDisableAllDockerHosts: (value: boolean) => void;
  disableAllDockerContainers: () => boolean;
  setDisableAllDockerContainers: (value: boolean) => void;
  // Global disable offline alerts flags
  disableAllNodesOffline: () => boolean;
  setDisableAllNodesOffline: (value: boolean) => void;
  disableAllGuestsOffline: () => boolean;
  setDisableAllGuestsOffline: (value: boolean) => void;
  disableAllPBSOffline: () => boolean;
  setDisableAllPBSOffline: (value: boolean) => void;
  disableAllDockerHostsOffline: () => boolean;
  setDisableAllDockerHostsOffline: (value: boolean) => void;
}

function ThresholdsTab(props: ThresholdsTabProps) {
  // Use the new table component for a cleaner, more information-dense layout
  return (
    <ThresholdsTable
      overrides={props.overrides}
      setOverrides={props.setOverrides}
      rawOverridesConfig={props.rawOverridesConfig}
      setRawOverridesConfig={props.setRawOverridesConfig}
      allGuests={props.allGuests}
      nodes={props.state.nodes || []}
      storage={props.state.storage || []}
      dockerHosts={props.state.dockerHosts || []}
      pbsInstances={props.state.pbs || []}
      guestDefaults={props.guestDefaults()}
      guestDisableConnectivity={props.guestDisableConnectivity()}
      setGuestDefaults={props.setGuestDefaults}
      setGuestDisableConnectivity={props.setGuestDisableConnectivity}
      nodeDefaults={props.nodeDefaults()}
      setNodeDefaults={props.setNodeDefaults}
      dockerDefaults={props.dockerDefaults()}
      setDockerDefaults={props.setDockerDefaults}
      storageDefault={props.storageDefault}
      setStorageDefault={props.setStorageDefault}
      timeThreshold={props.timeThreshold}
      setTimeThreshold={props.setTimeThreshold}
      timeThresholds={props.timeThresholds}
      setTimeThresholds={props.setTimeThresholds}
      setHasUnsavedChanges={props.setHasUnsavedChanges}
      activeAlerts={props.activeAlerts}
      removeAlerts={props.removeAlerts}
      disableAllNodes={props.disableAllNodes}
      setDisableAllNodes={props.setDisableAllNodes}
      disableAllGuests={props.disableAllGuests}
      setDisableAllGuests={props.setDisableAllGuests}
      disableAllStorage={props.disableAllStorage}
      setDisableAllStorage={props.setDisableAllStorage}
      disableAllPBS={props.disableAllPBS}
      setDisableAllPBS={props.setDisableAllPBS}
      disableAllDockerHosts={props.disableAllDockerHosts}
      setDisableAllDockerHosts={props.setDisableAllDockerHosts}
      disableAllDockerContainers={props.disableAllDockerContainers}
      setDisableAllDockerContainers={props.setDisableAllDockerContainers}
      disableAllNodesOffline={props.disableAllNodesOffline}
      setDisableAllNodesOffline={props.setDisableAllNodesOffline}
      disableAllGuestsOffline={props.disableAllGuestsOffline}
      setDisableAllGuestsOffline={props.setDisableAllGuestsOffline}
      disableAllPBSOffline={props.disableAllPBSOffline}
      setDisableAllPBSOffline={props.setDisableAllPBSOffline}
      disableAllDockerHostsOffline={props.disableAllDockerHostsOffline}
      setDisableAllDockerHostsOffline={props.setDisableAllDockerHostsOffline}
    />
  );
}

// Destinations Tab - Notification settings
interface DestinationsTabProps {
  ref: DestinationsRef;
  hasUnsavedChanges: () => boolean;
  setHasUnsavedChanges: (value: boolean) => void;
  emailConfig: () => UIEmailConfig;
  setEmailConfig: (config: UIEmailConfig) => void;
}

function DestinationsTab(props: DestinationsTabProps) {
  const [webhooks, setWebhooks] = createSignal<Webhook[]>([]);
  const [testingEmail, setTestingEmail] = createSignal(false);
  const [testingWebhook, setTestingWebhook] = createSignal<string | null>(null);

  // Load webhooks on mount (email config is now loaded in parent)
  onMount(async () => {
    try {
      const hooks = await NotificationsAPI.getWebhooks();
      // Map to local Webhook type - preserve the service type from backend
      setWebhooks(
        hooks.map((h) => ({
          ...h,
          service: h.service || 'generic', // Preserve service type or default to generic
        })),
      );
    } catch (err) {
      console.error('Failed to load webhooks:', err);
    }
  });

  const testEmailConfig = async () => {
    setTestingEmail(true);
    try {
      // Send the current form config for testing (including unsaved changes)
      const config = props.emailConfig();
      await NotificationsAPI.testNotification({
        type: 'email',
        config: { ...config } as Record<string, unknown>, // Send current form data, not saved config
      });
      showSuccess('Test email sent successfully!', 'Check your inbox.');
    } catch (err) {
      console.error('Failed to send test email:', err);
      showError('Failed to send test email', err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setTestingEmail(false);
    }
  };

  const testWebhook = async (webhookId: string, webhookData?: Omit<Webhook, 'id'>) => {
    setTestingWebhook(webhookId);
    try {
      if (webhookData) {
        // Test unsaved webhook with provided configuration
        await NotificationsAPI.testWebhook(webhookData);
      } else {
        // Test existing webhook by ID
        await NotificationsAPI.testNotification({ type: 'webhook', webhookId });
      }
      showSuccess('Test webhook sent successfully!');
    } catch (err) {
      showError(
        'Failed to send test webhook',
        err instanceof Error ? err.message : 'Unknown error',
      );
    } finally {
      setTestingWebhook(null);
    }
  };

  return (
    <div class="flex w-full max-w-full flex-col gap-6 md:gap-8">
      <SettingsPanel
        title="Email notifications"
        description="Configure SMTP delivery for alert emails."
        action={
          <Toggle
            checked={props.emailConfig().enabled}
            onChange={(e) => {
              props.setEmailConfig({ ...props.emailConfig(), enabled: e.currentTarget.checked });
              props.setHasUnsavedChanges(true);
            }}
            containerClass="sm:self-start"
            label={
              <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                {props.emailConfig().enabled ? 'Enabled' : 'Disabled'}
              </span>
            }
          />
        }
        class="min-w-0"
        bodyClass=""
      >
        <div
          class={`${!props.emailConfig().enabled ? 'pointer-events-none opacity-50 transition-opacity' : 'transition-opacity'}`}
        >
          <EmailProviderSelect
            config={props.emailConfig()}
            onChange={(config) => {
              props.setEmailConfig(config);
              props.setHasUnsavedChanges(true);
            }}
            onTest={testEmailConfig}
            testing={testingEmail()}
          />
        </div>
      </SettingsPanel>

      <SettingsPanel
        title="Webhooks"
        description="Push alerts to chat apps or automation systems."
        action={
          <span class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">
            {webhooks().length} configured
          </span>
        }
        class="min-w-0"
        bodyClass="space-y-4"
      >
        <WebhookConfig
          webhooks={webhooks()}
          onAdd={async (webhook) => {
            try {
              const created = await NotificationsAPI.createWebhook(webhook);
              setWebhooks([...webhooks(), created]);
              showSuccess('Webhook added successfully');
            } catch (err) {
              console.error('Failed to add webhook:', err);
              showError(err instanceof Error ? err.message : 'Failed to add webhook');
            }
          }}
          onUpdate={async (webhook) => {
            try {
              const updated = await NotificationsAPI.updateWebhook(webhook.id!, webhook);
              setWebhooks(webhooks().map((w) => (w.id === webhook.id ? updated : w)));
              showSuccess('Webhook updated successfully');
            } catch (err) {
              console.error('Failed to update webhook:', err);
              showError(err instanceof Error ? err.message : 'Failed to update webhook');
            }
          }}
          onDelete={async (id) => {
            try {
              await NotificationsAPI.deleteWebhook(id);
              setWebhooks(webhooks().filter((w) => w.id !== id));
              showSuccess('Webhook deleted successfully');
            } catch (err) {
              console.error('Failed to delete webhook:', err);
              showError(err instanceof Error ? err.message : 'Failed to delete webhook');
            }
          }}
          onTest={testWebhook}
          testing={testingWebhook()}
        />
      </SettingsPanel>
    </div>
  );
}

// History Tab - Alert history

// Schedule Tab - Quiet hours, cooldown, and grouping
interface ScheduleTabProps {
  hasUnsavedChanges: () => boolean;
  setHasUnsavedChanges: (value: boolean) => void;
  quietHours: () => QuietHoursConfig;
  setQuietHours: (value: QuietHoursConfig) => void;
  cooldown: () => CooldownConfig;
  setCooldown: (value: CooldownConfig) => void;
  grouping: () => GroupingConfig;
  setGrouping: (value: GroupingConfig) => void;
  escalation: () => EscalationConfig;
  setEscalation: (value: EscalationConfig) => void;
}

function ScheduleTab(props: ScheduleTabProps) {
  // Use props instead of local state
  const quietHours = props.quietHours;
  const setQuietHours = props.setQuietHours;
  const cooldown = props.cooldown;
  const setCooldown = props.setCooldown;
  const grouping = props.grouping;
  const setGrouping = props.setGrouping;
  const escalation = props.escalation;
  const setEscalation = props.setEscalation;
  const resetToDefaults = () => {
    setQuietHours(createDefaultQuietHours());
    setCooldown(createDefaultCooldown());
    setGrouping(createDefaultGrouping());
    setEscalation(createDefaultEscalation());
    props.setHasUnsavedChanges(true);
  };

  // Comprehensive list of common IANA timezones
  const timezones = [
    'UTC',
    // Africa
    'Africa/Cairo',
    'Africa/Johannesburg',
    'Africa/Lagos',
    'Africa/Nairobi',
    // Americas
    'America/Anchorage',
    'America/Argentina/Buenos_Aires',
    'America/Bogota',
    'America/Caracas',
    'America/Chicago',
    'America/Denver',
    'America/Halifax',
    'America/Lima',
    'America/Los_Angeles',
    'America/Mexico_City',
    'America/New_York',
    'America/Phoenix',
    'America/Santiago',
    'America/Sao_Paulo',
    'America/St_Johns',
    'America/Toronto',
    'America/Vancouver',
    // Asia
    'Asia/Bangkok',
    'Asia/Dhaka',
    'Asia/Dubai',
    'Asia/Hong_Kong',
    'Asia/Jakarta',
    'Asia/Jerusalem',
    'Asia/Karachi',
    'Asia/Kolkata',
    'Asia/Kuala_Lumpur',
    'Asia/Manila',
    'Asia/Riyadh',
    'Asia/Seoul',
    'Asia/Shanghai',
    'Asia/Singapore',
    'Asia/Taipei',
    'Asia/Tehran',
    'Asia/Tokyo',
    // Australia
    'Australia/Adelaide',
    'Australia/Brisbane',
    'Australia/Melbourne',
    'Australia/Perth',
    'Australia/Sydney',
    // Europe
    'Europe/Amsterdam',
    'Europe/Athens',
    'Europe/Berlin',
    'Europe/Brussels',
    'Europe/Budapest',
    'Europe/Copenhagen',
    'Europe/Dublin',
    'Europe/Helsinki',
    'Europe/Istanbul',
    'Europe/Lisbon',
    'Europe/London',
    'Europe/Madrid',
    'Europe/Moscow',
    'Europe/Oslo',
    'Europe/Paris',
    'Europe/Prague',
    'Europe/Rome',
    'Europe/Stockholm',
    'Europe/Vienna',
    'Europe/Warsaw',
    'Europe/Zurich',
    // Pacific
    'Pacific/Auckland',
    'Pacific/Fiji',
    'Pacific/Guam',
    'Pacific/Honolulu',
  ];

  const days = [
    { id: 'monday', label: 'M', fullLabel: 'Monday' },
    { id: 'tuesday', label: 'T', fullLabel: 'Tuesday' },
    { id: 'wednesday', label: 'W', fullLabel: 'Wednesday' },
    { id: 'thursday', label: 'T', fullLabel: 'Thursday' },
    { id: 'friday', label: 'F', fullLabel: 'Friday' },
    { id: 'saturday', label: 'S', fullLabel: 'Saturday' },
    { id: 'sunday', label: 'S', fullLabel: 'Sunday' },
  ];

  return (
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <SectionHeader
          title="Notification Schedule"
          description="Control when alerts are allowed to fire and how they are grouped."
          size="md"
        />
        <button
          type="button"
          onClick={resetToDefaults}
          class="inline-flex items-center gap-1 self-start rounded-md border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
          title="Restore quiet hours, cooldown, grouping, and escalation settings to their defaults"
        >
          <svg
            class="h-3 w-3"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
          Reset defaults
        </button>
      </div>

      <div class="grid gap-6 lg:grid-cols-2">
        {/* Quiet Hours */}
        <SettingsPanel
          title="Quiet hours"
          description="Pause non-critical alerts during specific times."
          action={
            <Toggle
              checked={quietHours().enabled}
              onChange={(e) => {
                setQuietHours({ ...quietHours(), enabled: e.currentTarget.checked });
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                  {quietHours().enabled ? 'Enabled' : 'Disabled'}
                </span>
              }
            />
          }
          class="space-y-4"
        >
          <Show when={quietHours().enabled}>
            <div class="space-y-4">
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    Start time
                  </label>
                  <input
                    type="time"
                    value={quietHours().start}
                    onChange={(e) => {
                      setQuietHours({ ...quietHours(), start: e.currentTarget.value });
                      props.setHasUnsavedChanges(true);
                    }}
                    class={controlClass('font-mono')}
                  />
                </div>
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>End time</label>
                  <input
                    type="time"
                    value={quietHours().end}
                    onChange={(e) => {
                      setQuietHours({ ...quietHours(), end: e.currentTarget.value });
                      props.setHasUnsavedChanges(true);
                    }}
                    class={controlClass('font-mono')}
                  />
                </div>
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>Timezone</label>
                  <select
                    value={quietHours().timezone}
                    onChange={(e) => {
                      setQuietHours({ ...quietHours(), timezone: e.currentTarget.value });
                      props.setHasUnsavedChanges(true);
                    }}
                    class={controlClass('pr-8')}
                  >
                    <For each={timezones}>{(tz) => <option value={tz}>{tz}</option>}</For>
                  </select>
                </div>
              </div>

              <div>
                <span class={`${labelClass('text-xs uppercase tracking-[0.08em]')} mb-2 block`}>
                  Quiet days
                </span>
                <div class="grid grid-cols-7 gap-1">
                  <For each={days}>
                    {(day) => (
                      <button
                        type="button"
                        onClick={() => {
                          const currentDays = quietHours().days;
                          setQuietHours({
                            ...quietHours(),
                            days: { ...currentDays, [day.id]: !currentDays[day.id] },
                          });
                          props.setHasUnsavedChanges(true);
                        }}
                        title={day.fullLabel}
                        class={`px-2 py-2 text-xs font-medium transition-all duration-200 ${
                          quietHours().days[day.id]
                            ? 'rounded-md bg-blue-500 text-white shadow-sm'
                            : 'rounded-md bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:hover:bg-gray-600'
                        }`}
                      >
                        {day.label}
                      </button>
                    )}
                  </For>
                </div>
                <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  <Show
                    when={
                      quietHours().days.monday &&
                      quietHours().days.tuesday &&
                      quietHours().days.wednesday &&
                      quietHours().days.thursday &&
                      quietHours().days.friday &&
                      !quietHours().days.saturday &&
                      !quietHours().days.sunday
                    }
                  >
                    Weekdays only
                  </Show>
                  <Show
                    when={
                      !quietHours().days.monday &&
                      !quietHours().days.tuesday &&
                      !quietHours().days.wednesday &&
                      !quietHours().days.thursday &&
                      !quietHours().days.friday &&
                      quietHours().days.saturday &&
                      quietHours().days.sunday
                    }
                  >
                    Weekends only
                  </Show>
                </p>
              </div>
            </div>
          </Show>
        </SettingsPanel>

        {/* Cooldown Period */}
        <SettingsPanel
          title="Alert cooldown"
          description="Limit alert frequency to prevent spam."
          action={
            <Toggle
              checked={cooldown().enabled}
              onChange={(e) => {
                setCooldown({ ...cooldown(), enabled: e.currentTarget.checked });
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                  {cooldown().enabled ? 'Enabled' : 'Disabled'}
                </span>
              }
            />
          }
          class="space-y-4"
        >
          <Show when={cooldown().enabled}>
            <div class="space-y-4">
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    Cooldown period
                  </label>
                  <div class="relative">
                    <input
                      type="number"
                      min="5"
                      max="120"
                      value={cooldown().minutes}
                      onChange={(e) => {
                        setCooldown({ ...cooldown(), minutes: parseInt(e.currentTarget.value) });
                        props.setHasUnsavedChanges(true);
                      }}
                      class={controlClass('pr-16')}
                    />
                    <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-500 dark:text-gray-400">
                      minutes
                    </span>
                  </div>
                  <p class={`${formHelpText} mt-1`}>
                    Minimum time between alerts for the same issue
                  </p>
                </div>

                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    Max alerts / hour
                  </label>
                  <div class="relative">
                    <input
                      type="number"
                      min="1"
                      max="10"
                      value={cooldown().maxAlerts}
                      onChange={(e) => {
                        setCooldown({ ...cooldown(), maxAlerts: parseInt(e.currentTarget.value) });
                        props.setHasUnsavedChanges(true);
                      }}
                      class={controlClass('pr-16')}
                    />
                    <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-500 dark:text-gray-400">
                      alerts
                    </span>
                  </div>
                  <p class={`${formHelpText} mt-1`}>Per guest/metric combination</p>
                </div>
              </div>
            </div>
          </Show>
        </SettingsPanel>

        {/* Alert Grouping */}
        <SettingsPanel
          title="Smart grouping"
          description="Bundle similar alerts together."
          action={
            <Toggle
              checked={grouping().enabled}
              onChange={(e) => {
                setGrouping({ ...grouping(), enabled: e.currentTarget.checked });
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                  {grouping().enabled ? 'Enabled' : 'Disabled'}
                </span>
              }
            />
          }
          class="space-y-4"
        >
          <Show when={grouping().enabled}>
            <div class="space-y-4">
              <div class={formField}>
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                  Grouping window
                </label>
                <div class="flex items-center gap-3">
                  <input
                    type="range"
                    min="1"
                    max="30"
                    value={grouping().window}
                    onChange={(e) => {
                      setGrouping({ ...grouping(), window: parseInt(e.currentTarget.value) });
                      props.setHasUnsavedChanges(true);
                    }}
                    class="flex-1"
                  />
                  <div class="w-16 rounded-md bg-gray-100 px-2 py-1 text-center text-sm text-gray-700 dark:bg-gray-700 dark:text-gray-200">
                    {grouping().window} min
                  </div>
                </div>
                <p class={`${formHelpText} mt-1`}>
                  Alerts within this window are grouped together.
                </p>
              </div>

              <div>
                <span class={`${labelClass('text-xs uppercase tracking-[0.08em]')} mb-2 block`}>
                  Grouping strategy
                </span>
                <div class="grid grid-cols-2 gap-2">
                  <label
                    class={`relative flex items-center gap-2 rounded-lg border-2 p-3 transition-all ${
                      grouping().byNode
                        ? 'border-blue-500 bg-blue-50 shadow-sm dark:bg-blue-900/20'
                        : 'border-gray-200 hover:bg-gray-50 dark:border-gray-600 dark:hover:bg-gray-700'
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={grouping().byNode}
                      onChange={(e) => {
                        setGrouping({ ...grouping(), byNode: e.currentTarget.checked });
                        props.setHasUnsavedChanges(true);
                      }}
                      class="sr-only"
                    />
                    <div
                      class={`flex h-4 w-4 items-center justify-center rounded border-2 ${
                        grouping().byNode
                          ? 'border-blue-500 bg-blue-500'
                          : 'border-gray-300 dark:border-gray-600'
                      }`}
                    >
                      <Show when={grouping().byNode}>
                        <svg
                          class="h-3 w-3 text-white"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="3"
                        >
                          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                        </svg>
                      </Show>
                    </div>
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      By Node
                    </span>
                  </label>

                  <label
                    class={`relative flex items-center gap-2 rounded-lg border-2 p-3 transition-all ${
                      grouping().byGuest
                        ? 'border-blue-500 bg-blue-50 shadow-sm dark:bg-blue-900/20'
                        : 'border-gray-200 hover:bg-gray-50 dark:border-gray-600 dark:hover:bg-gray-700'
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={grouping().byGuest}
                      onChange={(e) => {
                        setGrouping({ ...grouping(), byGuest: e.currentTarget.checked });
                        props.setHasUnsavedChanges(true);
                      }}
                      class="sr-only"
                    />
                    <div
                      class={`flex h-4 w-4 items-center justify-center rounded border-2 ${
                        grouping().byGuest
                          ? 'border-blue-500 bg-blue-500'
                          : 'border-gray-300 dark:border-gray-600'
                      }`}
                    >
                      <Show when={grouping().byGuest}>
                        <svg
                          class="h-3 w-3 text-white"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="3"
                        >
                          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                        </svg>
                      </Show>
                    </div>
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      By Guest
                    </span>
                  </label>
                </div>
              </div>
            </div>
          </Show>
        </SettingsPanel>

        {/* Escalation Rules */}
        <SettingsPanel
          title="Alert escalation"
          description="Notify additional contacts for persistent issues."
          action={
            <Toggle
              checked={escalation().enabled}
              onChange={(e) => {
                setEscalation({ ...escalation(), enabled: e.currentTarget.checked });
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                  {escalation().enabled ? 'Enabled' : 'Disabled'}
                </span>
              }
            />
          }
          class="space-y-4"
        >
          <Show when={escalation().enabled}>
            <div class="space-y-3">
              <p class={formHelpText}>Define escalation levels for unresolved alerts:</p>
              <For each={escalation().levels}>
                {(level, index) => (
                  <div class="flex items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-600 dark:bg-gray-700/40">
                    <div class="flex flex-1 flex-col gap-3 sm:grid sm:grid-cols-2 sm:items-center sm:gap-2">
                      <div class="flex items-center gap-2">
                        <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                          After
                        </span>
                        <input
                          type="number"
                          min="5"
                          max="180"
                          value={level.after}
                          onChange={(e) => {
                            const newLevels = [...escalation().levels];
                            const parsed = Number.parseInt(e.currentTarget.value, 10);
                            newLevels[index()] = {
                              ...level,
                              after: Number.isNaN(parsed) ? level.after : parsed,
                            };
                            setEscalation({ ...escalation(), levels: newLevels });
                            props.setHasUnsavedChanges(true);
                          }}
                          class={`${controlClass('px-2 py-1 text-sm')} w-20`}
                        />
                        <span class="text-xs text-gray-600 dark:text-gray-400">min</span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                          Notify
                        </span>
                        <select
                          value={level.notify}
                          onChange={(e) => {
                            const newLevels = [...escalation().levels];
                            newLevels[index()] = {
                              ...level,
                              notify: e.currentTarget.value as EscalationNotifyTarget,
                            };
                            setEscalation({ ...escalation(), levels: newLevels });
                            props.setHasUnsavedChanges(true);
                          }}
                          class={`${controlClass('px-2 py-1 text-sm')} flex-1`}
                        >
                          <option value="email">Email</option>
                          <option value="webhook">Webhooks</option>
                          <option value="all">All Channels</option>
                        </select>
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={() => {
                        const newLevels = escalation().levels.filter((_, i) => i !== index());
                        setEscalation({ ...escalation(), levels: newLevels });
                        props.setHasUnsavedChanges(true);
                      }}
                      class="rounded-md p-1.5 text-red-600 transition-colors hover:bg-red-100 dark:hover:bg-red-900/30"
                      title="Remove escalation level"
                    >
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                        />
                      </svg>
                    </button>
                  </div>
                )}
              </For>

              <button
                type="button"
                onClick={() => {
                  const lastLevel = escalation().levels[escalation().levels.length - 1];
                  const newAfter = typeof lastLevel?.after === 'number' ? lastLevel.after + 30 : 15;
                  setEscalation({
                    ...escalation(),
                    levels: [
                      ...escalation().levels,
                      { after: newAfter, notify: 'all' as EscalationNotifyTarget },
                    ],
                  });
                  props.setHasUnsavedChanges(true);
                }}
                class="flex w-full items-center justify-center gap-2 rounded-lg border-2 border-dashed border-gray-300 py-2 text-sm text-gray-600 transition-all hover:border-gray-400 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:border-gray-500 dark:hover:bg-gray-700"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 6v6m0 0v6m0-6h6m-6 0H6"
                  />
                </svg>
                Add Escalation Level
              </button>
            </div>
          </Show>
        </SettingsPanel>

        {/* Configuration Summary */}
        <SettingsPanel
          title="Configuration summary"
          description="Preview of the active schedule settings."
          tone="muted"
          padding="lg"
          bodyClass="space-y-1 text-sm text-blue-800 dark:text-blue-300"
          class="lg:col-span-2"
        >
          <Show when={quietHours().enabled}>
            <p>
              • Quiet hours active from {quietHours().start} to {quietHours().end} (
              {quietHours().timezone})
            </p>
          </Show>
          <Show when={cooldown().enabled}>
            <p>
              • {cooldown().minutes} minute cooldown between alerts, max {cooldown().maxAlerts}{' '}
              alerts per hour
            </p>
          </Show>
          <Show when={grouping().enabled}>
            <p>
              • Grouping alerts within {grouping().window} minute windows
              <Show when={grouping().byNode || grouping().byGuest}>
                {' '}
                by{' '}
                {[grouping().byNode && 'node', grouping().byGuest && 'guest']
                  .filter(Boolean)
                  .join(' and ')}
              </Show>
            </p>
          </Show>
          <Show when={escalation().enabled && escalation().levels.length > 0}>
            <p>
              • {escalation().levels.length} escalation level
              {escalation().levels.length > 1 ? 's' : ''} configured
            </p>
          </Show>
          <Show
            when={
              !quietHours().enabled &&
              !cooldown().enabled &&
              !grouping().enabled &&
              !escalation().enabled
            }
          >
            <p>• All notification controls are disabled - alerts will be sent immediately</p>
          </Show>
        </SettingsPanel>
      </div>
    </div>
  );
}
// History Tab - Comprehensive alert table
function HistoryTab() {
  const { state, activeAlerts } = useWebSocket();

  // Filter states with localStorage persistence
  const [timeFilter, setTimeFilter] = createSignal(
    localStorage.getItem('alertHistoryTimeFilter') || '7d',
  );
  const [severityFilter, setSeverityFilter] = createSignal(
    localStorage.getItem('alertHistorySeverityFilter') || 'all',
  );
  const [searchTerm, setSearchTerm] = createSignal('');
  const [alertHistory, setAlertHistory] = createSignal<Alert[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [selectedBarIndex, setSelectedBarIndex] = createSignal<number | null>(null);

  // Ref for search input
  let searchInputRef: HTMLInputElement | undefined;

  // Persist filter changes to localStorage
  createEffect(() => {
    localStorage.setItem('alertHistoryTimeFilter', timeFilter());
  });

  createEffect(() => {
    localStorage.setItem('alertHistorySeverityFilter', severityFilter());
  });

  // Load alert history on mount
  onMount(async () => {
    try {
      const history = await AlertsAPI.getHistory({ limit: 1000 });
      setAlertHistory(history);
    } catch (err) {
      console.error('Failed to load alert history:', err);
    } finally {
      setLoading(false);
    }

    // Add keyboard event listeners
    const handleKeydown = (e: KeyboardEvent) => {
      // If already focused on an input, select, or textarea, don't interfere
      const activeElement = document.activeElement;
      if (
        activeElement &&
        (activeElement.tagName === 'INPUT' ||
          activeElement.tagName === 'TEXTAREA' ||
          activeElement.tagName === 'SELECT')
      ) {
        // Handle Escape to clear and unfocus
        if (e.key === 'Escape' && activeElement === searchInputRef) {
          setSearchTerm('');
          searchInputRef.blur();
        }
        return;
      }

      // If typing a letter, number, or space, focus the search input
      if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        searchInputRef?.focus();
      }
    };

    document.addEventListener('keydown', handleKeydown);

    // Cleanup on unmount
    return () => {
      document.removeEventListener('keydown', handleKeydown);
    };
  });

  // Format duration for display
  const formatDuration = (startTime: string, endTime?: string) => {
    const start = new Date(startTime).getTime();
    const end = endTime ? new Date(endTime).getTime() : Date.now();
    const duration = end - start;

    // Handle negative durations (clock skew or timezone issues)
    if (duration < 0) {
      return '0m';
    }

    const minutes = Math.floor(duration / 60000);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}d ${hours % 24}h`;
    if (hours > 0) return `${hours}h ${minutes % 60}m`;
    return `${minutes}m`;
  };

  // Get resource type (VM, CT, Node, Storage)
  const getResourceType = (resourceName: string) => {
    // Check VMs and containers
    const vm = state.vms?.find((v) => v.name === resourceName);
    if (vm) return 'VM';

    const container = state.containers?.find((c) => c.name === resourceName);
    if (container) return 'CT';

    // Check nodes
    const node = state.nodes?.find((n) => n.name === resourceName);
    if (node) return 'Node';

    // Check storage
    const storage = state.storage?.find((s) => s.name === resourceName || s.id === resourceName);
    if (storage) return 'Storage';

    return 'Unknown';
  };

  // Extended alert type for display
  interface ExtendedAlert extends Alert {
    status?: string;
    duration?: string;
    resourceType?: string;
  }

  // Prepare all alerts without filtering
  const allAlertsData = createMemo(() => {
    // Combine active and historical alerts
    const allAlerts: ExtendedAlert[] = [];

    // Add active alerts
    Object.values(activeAlerts || {}).forEach((alert) => {
      allAlerts.push({
        ...alert,
        status: 'active',
        duration: formatDuration(alert.startTime),
        resourceType: getResourceType(alert.resourceName),
      });
    });

    // Create a set of active alert IDs for quick lookup
    const activeAlertIds = new Set(Object.keys(activeAlerts || {}));

    // Add historical alerts
    alertHistory().forEach((alert) => {
      // Skip if this alert is already in active alerts (avoid duplicates)
      if (activeAlertIds.has(alert.id)) {
        return;
      }

      allAlerts.push({
        ...alert,
        status: alert.acknowledged ? 'acknowledged' : 'resolved',
        duration: formatDuration(alert.startTime, alert.lastSeen),
        resourceType: getResourceType(alert.resourceName),
      });
    });

    return allAlerts;
  });

  // Apply filters to get the final alert data
  const alertData = createMemo(() => {
    let filtered = allAlertsData();

    // Selected bar filter (takes precedence over time filter)
    if (selectedBarIndex() !== null) {
      const trends = alertTrends();
      const index = selectedBarIndex()!;
      const bucketStart = trends.bucketTimes[index];
      const bucketEnd = bucketStart + trends.bucketSize * 60 * 60 * 1000;

      filtered = filtered.filter((alert) => {
        const alertTime = new Date(alert.startTime).getTime();
        return alertTime >= bucketStart && alertTime < bucketEnd;
      });
    } else {
      // Time filter
      if (timeFilter() !== 'all') {
        const now = Date.now();
        const cutoff = {
          '24h': now - 24 * 60 * 60 * 1000,
          '7d': now - 7 * 24 * 60 * 60 * 1000,
          '30d': now - 30 * 24 * 60 * 60 * 1000,
        }[timeFilter()];

        if (cutoff) {
          filtered = filtered.filter((a) => new Date(a.startTime).getTime() > cutoff);
        }
      }
    }

    // Severity filter
    if (severityFilter() !== 'all') {
      filtered = filtered.filter((a) => a.level === severityFilter());
    }

    // Search filter
    if (searchTerm()) {
      const term = searchTerm().toLowerCase();
      filtered = filtered.filter(
        (alert) =>
          alert.resourceName.toLowerCase().includes(term) ||
          alert.message.toLowerCase().includes(term) ||
          alert.type.toLowerCase().includes(term) ||
          alert.node.toLowerCase().includes(term),
      );
    }

    // Sort by start time (newest first)
    return filtered.sort(
      (a, b) => new Date(b.startTime).getTime() - new Date(a.startTime).getTime(),
    );
  });

  const monthNames = [
    'January',
    'February',
    'March',
    'April',
    'May',
    'June',
    'July',
    'August',
    'September',
    'October',
    'November',
    'December',
  ];

  const getDaySuffix = (day: number) => {
    if (day >= 11 && day <= 13) return 'th';
    switch (day % 10) {
      case 1:
        return 'st';
      case 2:
        return 'nd';
      case 3:
        return 'rd';
      default:
        return 'th';
    }
  };

  const formatAlertGroupLabel = (date: Date, todayStart: number, yesterdayStart: number) => {
    const month = monthNames[date.getMonth()];
    const day = date.getDate();
    const suffix = getDaySuffix(day);
    const absoluteDate = `${month} ${day}${suffix}`;

    if (date.getTime() === todayStart) {
      return `Today (${absoluteDate})`;
    }

    if (date.getTime() === yesterdayStart) {
      return `Yesterday (${absoluteDate})`;
    }

    return absoluteDate;
  };

  type AlertHistoryRow = ReturnType<typeof alertData>[number];

  // Group alerts by day for display
  const groupedAlerts = createMemo(() => {
    const now = new Date();
    const todayDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const todayStart = todayDate.getTime();
    const yesterdayDate = new Date(todayDate);
    yesterdayDate.setDate(yesterdayDate.getDate() - 1);
    const yesterdayStart = yesterdayDate.getTime();

    const groups = new Map<
      number,
      {
        date: Date;
        label: string;
        fullLabel: string;
        alerts: AlertHistoryRow[];
      }
    >();

    alertData().forEach((alert) => {
      const alertDate = new Date(alert.startTime);
      const dateOnly = new Date(
        alertDate.getFullYear(),
        alertDate.getMonth(),
        alertDate.getDate(),
      );
      const dateKey = dateOnly.getTime();

      if (!groups.has(dateKey)) {
        groups.set(dateKey, {
          date: dateOnly,
          label: formatAlertGroupLabel(dateOnly, todayStart, yesterdayStart),
          fullLabel: dateOnly.toLocaleDateString('en-US', {
            weekday: 'long',
            year: 'numeric',
            month: 'long',
            day: 'numeric',
          }),
          alerts: [],
        });
      }

      groups.get(dateKey)!.alerts.push(alert);
    });

    return Array.from(groups.values()).sort((a, b) => b.date.getTime() - a.date.getTime());
  });

  // Calculate alert trends for mini-chart
  const alertTrends = createMemo(() => {
    const now = Date.now();
    const timeRange =
      timeFilter() === '24h'
        ? 24
        : timeFilter() === '7d'
          ? 7 * 24
          : timeFilter() === '30d'
            ? 30 * 24
            : 90 * 24; // hours
    const bucketSize =
      timeFilter() === '24h' ? 1 : timeFilter() === '7d' ? 6 : timeFilter() === '30d' ? 24 : 72; // hours per bucket
    const numBuckets = Math.min(Math.floor(timeRange / bucketSize), 30); // Limit to 30 buckets max

    // Calculate start time for the chart
    const startTime = now - timeRange * 60 * 60 * 1000;

    // Initialize buckets
    const buckets = new Array(numBuckets).fill(0);
    // bucketTimes represents the START of each bucket
    const bucketTimes = new Array(numBuckets)
      .fill(0)
      .map((_, i) => startTime + i * bucketSize * 60 * 60 * 1000);

    // Filter alerts based on current time filter
    let alertsToCount = allAlertsData();
    if (timeFilter() !== 'all') {
      const cutoff = {
        '24h': now - 24 * 60 * 60 * 1000,
        '7d': now - 7 * 24 * 60 * 60 * 1000,
        '30d': now - 30 * 24 * 60 * 60 * 1000,
      }[timeFilter()];

      if (cutoff) {
        alertsToCount = alertsToCount.filter((a) => new Date(a.startTime).getTime() > cutoff);
      }
    }

    alertsToCount.forEach((alert) => {
      const alertTime = new Date(alert.startTime).getTime();
      if (alertTime >= startTime && alertTime <= now) {
        const bucketIndex = Math.floor((alertTime - startTime) / (bucketSize * 60 * 60 * 1000));
        if (bucketIndex >= 0 && bucketIndex < numBuckets) {
          buckets[bucketIndex]++;
        }
      }
    });

    // Find max for scaling
    const max = Math.max(...buckets, 1);

    return {
      buckets,
      max,
      bucketSize,
      bucketTimes,
    };
  });

  return (
    <div class="space-y-4">
      {/* Main section header */}
      <SectionHeader
        title="Alert History"
        description="View past and active alerts with trends and filtering options."
        size="md"
      />

      {/* Alert Trends Mini-Chart */}
      <Card padding="md">
        <div class="mb-3 flex items-start justify-between gap-3">
          <SectionHeader
            title="Alert frequency"
            description={
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {alertData().length} alerts
              </span>
            }
            size="sm"
            class="flex-1"
          />
          <div class="flex items-center gap-2">
            <Show when={selectedBarIndex() !== null}>
              <button
                type="button"
                onClick={() => setSelectedBarIndex(null)}
                class="px-2 py-0.5 text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800/50 transition-colors"
              >
                Clear filter
              </button>
            </Show>
            <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span class="flex items-center gap-1">
                <div class="w-2 h-2 bg-yellow-500 rounded-full"></div>
                {alertData().filter((a) => a.level === 'warning').length} warnings
              </span>
              <span class="flex items-center gap-1">
                <div class="w-2 h-2 bg-red-500 rounded-full"></div>
                {alertData().filter((a) => a.level === 'critical').length} critical
              </span>
            </div>
          </div>
        </div>

        {/* Mini sparkline chart */}
        <div class="text-[10px] text-gray-400 mb-1">
          Showing {alertTrends().buckets.length} time periods - Total: {alertData().length} alerts
        </div>

        {/* Alert frequency chart */}
        <div class="h-12 bg-gray-100 dark:bg-gray-800 rounded p-1 flex items-end gap-1">
          {alertTrends().buckets.map((val, i) => {
            const scaledHeight = val > 0 ? Math.min(100, Math.max(20, Math.log(val + 1) * 20)) : 0;
            const pixelHeight = val > 0 ? Math.max(8, (scaledHeight / 100) * 40) : 0; // 40px is roughly the inner height
            const isSelected = selectedBarIndex() === i;
            return (
              <div
                class="flex-1 relative flex items-end cursor-pointer"
                onClick={() => setSelectedBarIndex(i === selectedBarIndex() ? null : i)}
              >
                {/* Background track for all slots */}
                <div class="absolute bottom-0 w-full h-1 bg-gray-300 dark:bg-gray-600 opacity-30 rounded-full"></div>
                {/* Actual bar */}
                <div
                  class="w-full relative rounded-sm transition-all"
                  style={{
                    height: `${pixelHeight}px`,
                    'background-color':
                      val > 0 ? (isSelected ? '#2563eb' : '#3b82f6') : 'transparent',
                    opacity: isSelected ? '1' : '0.8',
                    'box-shadow': isSelected ? '0 0 0 2px rgba(37, 99, 235, 0.4)' : 'none',
                  }}
                  onMouseEnter={(e) => {
                    if (val <= 0) {
                      hideTooltip();
                      return;
                    }
                    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
                    const bucketHours = alertTrends().bucketSize;
                    const bucketLabel = (() => {
                      if (timeFilter() === '24h') return `${bucketHours} hour period`;
                      const bucketDays = bucketHours / 24;
                      return `${bucketDays} day period`;
                    })();
                    const timestamp = new Date(alertTrends().bucketTimes[i]).toLocaleString('en-US', {
                      month: 'short',
                      day: 'numeric',
                      hour: timeFilter() === '24h' ? 'numeric' : undefined,
                      minute: timeFilter() === '24h' ? '2-digit' : undefined,
                    });
                    const content = [
                      `${val} alert${val !== 1 ? 's' : ''}`,
                      bucketLabel,
                      timestamp,
                    ].join('\n');
                    showTooltip(content, rect.left + rect.width / 2, rect.top, {
                      align: 'center',
                      direction: 'up',
                    });
                  }}
                  onMouseLeave={() => hideTooltip()}
                />
              </div>
            );
          })}
        </div>

        {/* Time labels */}
        <div class="flex justify-between mt-1 text-[10px] text-gray-400 dark:text-gray-500">
          <span>
            {timeFilter() === '24h'
              ? '24h ago'
              : timeFilter() === '7d'
                ? '7d ago'
                : timeFilter() === '30d'
                  ? '30d ago'
                  : '90d ago'}
          </span>
          <span>Now</span>
        </div>
      </Card>

      {/* Filters */}
      <div class="flex flex-wrap gap-2 mb-4">
        <select
          value={timeFilter()}
          onChange={(e) => setTimeFilter(e.currentTarget.value)}
          class="px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
        >
          <option value="24h">Last 24h</option>
          <option value="7d">Last 7d</option>
          <option value="30d">Last 30d</option>
          <option value="all">All Time</option>
        </select>

        <select
          value={severityFilter()}
          onChange={(e) => setSeverityFilter(e.currentTarget.value)}
          class="px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600"
        >
          <option value="all">All Levels</option>
          <option value="critical">Critical Only</option>
          <option value="warning">Warning Only</option>
        </select>

        <div class="flex-1 max-w-xs">
          <input
            ref={searchInputRef}
            type="text"
            placeholder="Search alerts..."
            value={searchTerm()}
            onInput={(e) => setSearchTerm(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                setSearchTerm('');
                e.currentTarget.blur();
              }
            }}
            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600 
                   dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500"
          />
        </div>
      </div>

      {/* Alert History Table */}
      <Show
        when={loading()}
        fallback={
          <Show
            when={alertData().length > 0}
            fallback={
              <div class="text-center py-12 text-gray-500 dark:text-gray-400">
                <p class="text-sm">No alerts found</p>
                <p class="text-xs mt-1">Try adjusting your filters or check back later</p>
              </div>
            }
          >
            {/* Table */}
            <div class="mb-2 border border-gray-200 dark:border-gray-700 rounded overflow-hidden">
              <div class="overflow-x-auto">
                <table class="w-full min-w-[900px] text-xs sm:text-sm">
                  <thead>
                    <tr class="bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">
                        Timestamp
                      </th>
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">
                        Resource
                      </th>
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">
                        Type
                      </th>
                      <th class="p-1 px-2 text-center text-[10px] sm:text-xs font-medium uppercase tracking-wider">
                        Severity
                      </th>
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">
                        Message
                      </th>
                      <th class="p-1 px-2 text-center text-[10px] sm:text-xs font-medium uppercase tracking-wider">
                        Duration
                      </th>
                      <th class="p-1 px-2 text-center text-[10px] sm:text-xs font-medium uppercase tracking-wider">
                        Status
                      </th>
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">
                        Node
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <For each={groupedAlerts()}>
                      {(group) => (
                        <>
                          {/* Date divider */}
                          <tr class="bg-gray-50 dark:bg-gray-900/40">
                            <td
                              colspan="8"
                              class="py-1.5 pr-3 pl-4 text-[12px] sm:text-sm font-semibold text-slate-700 dark:text-slate-100"
                            >
                              <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
                                <span class="truncate" title={group.fullLabel}>
                                  {group.label}
                                </span>
                                <span class="text-[10px] font-medium text-slate-500 dark:text-slate-400">
                                  {group.alerts.length}{' '}
                                  {group.alerts.length === 1 ? 'alert' : 'alerts'}
                                </span>
                              </div>
                            </td>
                          </tr>

                          {/* Alerts for this day */}
                          <For each={group.alerts}>
                            {(alert) => (
                              <tr
                                class={`border-b border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 ${
                                  alert.status === 'active' ? 'bg-red-50 dark:bg-red-900/10' : ''
                                }`}
                              >
                                {/* Timestamp */}
                                <td class="p-1 px-2 text-gray-600 dark:text-gray-400 font-mono">
                                  {new Date(alert.startTime).toLocaleTimeString('en-US', {
                                    hour: '2-digit',
                                    minute: '2-digit',
                                  })}
                                </td>

                                {/* Resource */}
                                <td class="p-1 px-2 font-medium text-gray-900 dark:text-gray-100 truncate max-w-[150px]">
                                  {alert.resourceName}
                                </td>

                                {/* Type */}
                                <td class="p-1 px-2">
                                  <span
                                    class={`text-xs px-1 py-0.5 rounded ${
                                      alert.resourceType === 'VM'
                                        ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300'
                                        : alert.resourceType === 'CT'
                                          ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                                          : alert.resourceType === 'Node'
                                            ? 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300'
                                            : alert.resourceType === 'Storage'
                                              ? 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300'
                                              : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                    }`}
                                  >
                                    {alert.type}
                                  </span>
                                </td>

                                {/* Severity */}
                                <td class="p-1 px-2 text-center">
                                  <span
                                    class={`text-xs px-2 py-0.5 rounded font-medium ${
                                      alert.level === 'critical'
                                        ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
                                        : 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
                                    }`}
                                  >
                                    {alert.level}
                                  </span>
                                </td>

                                {/* Message */}
                                <td
                                  class="p-1 px-2 text-gray-700 dark:text-gray-300 truncate max-w-[300px]"
                                  title={alert.message}
                                >
                                  {alert.message}
                                </td>

                                {/* Duration */}
                                <td class="p-1 px-2 text-center text-gray-600 dark:text-gray-400">
                                  {alert.duration}
                                </td>

                                {/* Status */}
                                <td class="p-1 px-2 text-center">
                                  <span
                                    class={`text-xs px-2 py-0.5 rounded ${
                                      alert.status === 'active'
                                        ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 font-medium'
                                        : alert.status === 'acknowledged'
                                          ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
                                          : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                    }`}
                                  >
                                    {alert.status}
                                  </span>
                                </td>

                                {/* Node */}
                                <td class="p-1 px-2 text-gray-600 dark:text-gray-400 truncate">
                                  {alert.node || '—'}
                                </td>
                              </tr>
                            )}
                          </For>
                        </>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </div>
          </Show>
        }
      >
        <div class="text-center py-12 text-gray-500 dark:text-gray-400">
          <p class="text-sm">Loading alert history...</p>
        </div>
      </Show>

      {/* Administrative Actions - Only show if there's history to clear */}
      <Show when={alertHistory().length > 0}>
        <div class="mt-8 pt-6 border-t border-gray-200 dark:border-gray-700">
          <div class="bg-gray-50 dark:bg-gray-800/50 rounded-lg p-4">
            <div class="flex items-start justify-between">
              <div>
                <h4 class="text-sm font-medium text-gray-800 dark:text-gray-200 mb-1">
                  Administrative Actions
                </h4>
                <p class="text-xs text-gray-600 dark:text-gray-400">
                  Permanently clear all alert history. Use with caution - this action cannot be
                  undone.
                </p>
              </div>
              <button
                type="button"
                onClick={async () => {
                  if (
                    confirm(
                      'Are you sure you want to clear all alert history?\n\nThis will permanently delete all historical alert data and cannot be undone.\n\nThis is typically only used for system maintenance or when starting fresh with a new monitoring setup.',
                    )
                  ) {
                    try {
                      await AlertsAPI.clearHistory();
                      setAlertHistory([]);
                      // Alert history cleared successfully
                    } catch (err) {
                      console.error('Error clearing alert history:', err);
                      showError(
                        'Error clearing alert history',
                        'Please check your connection and try again.',
                      );
                    }
                  }
                }}
                class="px-3 py-2 text-xs border border-red-300 dark:border-red-600 text-red-600 dark:text-red-400 
                       rounded-md hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors flex-shrink-0"
              >
                Clear All History
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
}
