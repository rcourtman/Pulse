import { For, Show, createMemo, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web';
import { useNavigate } from '@solidjs/router';
import type { JSX } from 'solid-js';
import type { Alert } from '@/types/api';
import type { AlertConfig, AlertThresholds, HysteresisThreshold } from '@/types/alerts';
import { notificationStore } from '@/stores/notifications';
import { formatAlertValue, formatAlertThreshold } from '@/utils/alertFormatters';

interface ActivationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onActivated?: () => Promise<void> | void;
  config: () => AlertConfig | null;
  activeAlerts: () => Alert[] | undefined;
  isLoading: () => boolean;
  activate: () => Promise<boolean>;
  refreshActiveAlerts: () => Promise<void>;
}

interface ThresholdSummary {
  heading: string;
  items: Array<{ label: string; value: string }>;
}

const extractTrigger = (
  threshold?: HysteresisThreshold | number,
  legacy?: number,
): number | undefined => {
  if (typeof threshold === 'number') {
    return threshold;
  }
  if (threshold && typeof threshold === 'object' && typeof threshold.trigger === 'number') {
    return threshold.trigger;
  }
  if (typeof legacy === 'number') {
    return legacy;
  }
  return undefined;
};

const formatThreshold = (value: number | undefined): string => {
  if (value === undefined || Number.isNaN(value)) {
    return 'Not configured';
  }
  if (value <= 0) {
    return 'Disabled';
  }
  return `${value}%`;
};

const summarizeThresholds = (config: AlertConfig | null): ThresholdSummary[] => {
  if (!config) {
    return [];
  }

  const summarize = (thresholds?: AlertThresholds): Array<{ label: string; value: string }> => {
    if (!thresholds) return [];
    return [
      {
        label: 'CPU',
        value: formatThreshold(extractTrigger(thresholds.cpu, thresholds.cpuLegacy)),
      },
      {
        label: 'Memory',
        value: formatThreshold(extractTrigger(thresholds.memory, thresholds.memoryLegacy)),
      },
      {
        label: 'Disk',
        value: formatThreshold(extractTrigger(thresholds.disk, thresholds.diskLegacy)),
      },
    ];
  };

  const guestItems = summarize(config.guestDefaults);
  const nodeItems = summarize(config.nodeDefaults);
  const storageValue = formatThreshold(extractTrigger(config.storageDefault));

  const summaries: ThresholdSummary[] = [];

  if (guestItems.length > 0) {
    summaries.push({ heading: 'Guest thresholds', items: guestItems });
  }
  if (nodeItems.length > 0) {
    const nodeWithTemperature = [
      ...nodeItems,
      {
        label: 'Temperature',
        value: formatAlertThreshold(extractTrigger(config.nodeDefaults?.temperature), 'temperature'),
      },
    ];
    summaries.push({ heading: 'Node thresholds', items: nodeWithTemperature });
  }
  summaries.push({
    heading: 'Storage',
    items: [
      {
        label: 'Usage',
        value: storageValue,
      },
    ],
  });

  return summaries;
};

const getChannelSummary = (config: AlertConfig | null): { status: 'configured' | 'missing'; message: string } => {
  if (!config || !config.notifications) {
    return {
      status: 'missing',
      message: 'Notification channels are not configured yet. Configure email or webhook destinations before activation.',
    };
  }

  const emailConfigured = Boolean(config.notifications.email?.server);
  const webhookConfigured = Boolean(config.notifications.webhooks?.some((hook) => hook.enabled));

  if (!emailConfigured && !webhookConfigured) {
    return {
      status: 'missing',
      message: 'Notification channels are not configured yet. Configure email or webhook destinations before activation.',
    };
  }

  if (emailConfigured && webhookConfigured) {
    return {
      status: 'configured',
      message: 'Email and webhook destinations are ready. You can fine-tune them under Notification Destinations.',
    };
  }

  if (emailConfigured) {
    return {
      status: 'configured',
      message: 'Email notifications are configured. Add additional webhook destinations if needed.',
    };
  }

  return {
    status: 'configured',
    message: 'Webhook notifications are configured. Add email fallbacks if needed.',
  };
};

export function ActivationModal(props: ActivationModalProps): JSX.Element {
  const navigate = useNavigate();
  const [isSubmitting, setIsSubmitting] = createSignal(false);

  const thresholdSummaries = createMemo(() => summarizeThresholds(props.config()));

  const violations = createMemo(() => props.activeAlerts() ?? []);
  const violationCount = createMemo(() => violations().length);

  const channelSummary = createMemo(() => getChannelSummary(props.config()));

  const observationHours = createMemo(() => props.config()?.observationWindowHours ?? 24);

  const handleActivate = async () => {
    if (isSubmitting()) {
      return;
    }
    setIsSubmitting(true);
    const success = await props.activate();

    if (success) {
      await props.refreshActiveAlerts();
      notificationStore.success('Notifications activated! You\'ll now receive alerts when issues are detected.');
      if (props.onActivated) {
        await props.onActivated();
      }
      props.onClose();
    } else {
      notificationStore.error('Unable to activate notifications. Please try again.');
    }

    setIsSubmitting(false);
  };

  const handleNavigateDestinations = () => {
    props.onClose();
    navigate('/alerts/destinations');
  };

  return (
    <Show when={props.isOpen}>
      <Portal>
        <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div class="absolute inset-0 bg-black/50 dark:bg-black/60" onClick={props.onClose} />
          <div class="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-hidden border border-gray-200 dark:border-gray-700 flex flex-col">
            <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Ready to activate notifications</h2>
                <p class="text-sm text-gray-600 dark:text-gray-400">
                  Review your alert thresholds and notification channels before turning on alerts.
                </p>
              </div>
              <button
                type="button"
                class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-500 dark:text-gray-400 transition-colors"
                onClick={props.onClose}
                aria-label="Close activation review"
              >
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>

            <div class="px-6 py-5 space-y-6 flex-1 overflow-y-auto">
              <section>
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200 uppercase tracking-wide">
                  Current thresholds
                </h3>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Thresholds determine when alerts fire. Adjust them under Alert Thresholds if needed before activating.
                </p>
                <div class="mt-4 grid gap-4 sm:grid-cols-2">
                  <For each={thresholdSummaries()}>
                    {(section) => (
                      <div class="rounded-md border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 p-3">
                        <h4 class="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase">
                          {section.heading}
                        </h4>
                        <ul class="mt-2 space-y-1">
                          <For each={section.items}>
                            {(item) => (
                              <li class="flex items-center justify-between text-sm text-gray-700 dark:text-gray-300">
                                <span>{item.label}</span>
                                <span class="font-medium text-gray-900 dark:text-gray-100">{item.value}</span>
                              </li>
                            )}
                          </For>
                        </ul>
                      </div>
                    )}
                  </For>
                </div>
              </section>

              <section>
                <div class="flex items-center justify-between">
                  <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200 uppercase tracking-wide">
                    Issues detected
                  </h3>
                  <span class="text-xs text-gray-500 dark:text-gray-400">
                    Observation window: {observationHours()}h
                  </span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  {violationCount() > 0
                    ? 'These alerts are currently active. When you activate, notifications will be sent to your configured channels.'
                    : 'No alerts triggered yet. When you activate, you\'ll be notified immediately if any issues are detected.'}
                </p>
                <Show
                  when={violationCount() > 0}
                  fallback={
                    <div class="mt-4 rounded-md border border-dashed border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-900/30 p-4 text-sm text-gray-600 dark:text-gray-400">
                      All systems healthy — no alerts triggered.
                    </div>
                  }
                >
                  <div class="mt-4 space-y-3">
                    <For each={violations()}>
                      {(alert) => (
                        <div
                          class={`border rounded-md p-3 text-sm transition-colors ${alert.level === 'critical'
                            ? 'border-red-300 dark:border-red-700 bg-red-50 dark:bg-red-900/20'
                            : 'border-yellow-300 dark:border-yellow-700 bg-yellow-50 dark:bg-yellow-900/20'
                            }`}
                        >
                          <div class="flex items-center justify-between">
                            <div class="flex items-center gap-2">
                              <span
                                class={`px-2 py-0.5 rounded-full text-xs font-semibold uppercase ${alert.level === 'critical'
                                  ? 'bg-red-600 text-white'
                                  : 'bg-yellow-500 text-gray-900'
                                  }`}
                              >
                                {alert.level}
                              </span>
                              <span class="font-medium text-gray-800 dark:text-gray-100">
                                {alert.resourceName || alert.resourceId}
                              </span>
                            </div>
                            <span class="text-xs text-gray-600 dark:text-gray-300">{alert.type}</span>
                          </div>
                          <p class="mt-2 text-xs text-gray-600 dark:text-gray-300">{alert.message}</p>
                          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                            Threshold {formatAlertValue(alert.threshold, alert.type)} • Current {formatAlertValue(alert.value, alert.type)} • Since{' '}
                            {new Date(alert.startTime).toLocaleString()}
                          </p>
                        </div>
                      )}
                    </For>
                  </div>
                </Show>
              </section>

              <section>
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-200 uppercase tracking-wide">
                  Notification channels
                </h3>
                <div
                  class={`mt-3 rounded-md border p-4 ${channelSummary().status === 'configured'
                    ? 'border-green-200 dark:border-green-700 bg-green-50 dark:bg-green-900/20'
                    : 'border-blue-200 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20'
                    }`}
                >
                  <p class="text-sm text-gray-800 dark:text-gray-100">{channelSummary().message}</p>
                  <button
                    type="button"
                    class="mt-3 inline-flex items-center gap-1 text-sm font-medium text-blue-600 dark:text-blue-300 hover:text-blue-700 dark:hover:text-blue-200 transition-colors"
                    onClick={handleNavigateDestinations}
                  >
                    Open Notification Destinations
                    <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
                      <path d="M12.293 2.293a1 1 0 011.414 0l4 4a1 1 0 010 1.414l-8 8a1 1 0 01-.497.263l-4 1a1 1 0 01-1.213-1.213l1-4a1 1 0 01.263-.497l8-8z" />
                    </svg>
                  </button>
                </div>
              </section>
            </div>

            <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/40 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p class="text-xs text-gray-600 dark:text-gray-400">
                You can snooze alerts later if you need a quiet period.
              </p>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                  onClick={props.onClose}
                >
                  Not now
                </button>
                <button
                  type="button"
                  class="inline-flex items-center justify-center px-4 py-2 text-sm font-semibold rounded-md bg-blue-600 hover:bg-blue-700 text-white transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
                  onClick={handleActivate}
                  disabled={isSubmitting() || props.isLoading()}
                >
                  {isSubmitting() || props.isLoading() ? 'Activating…' : 'Activate Notifications'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Portal>
    </Show>
  );
}
