import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';

interface TimeThresholdSettingsProps {
  timeThresholds: () => { guest: number; node: number; storage: number; pbs: number };
  setTimeThresholds: (value: { guest: number; node: number; storage: number; pbs: number }) => void;
  setHasUnsavedChanges: (value: boolean) => void;
}

export function TimeThresholdSettings(props: TimeThresholdSettingsProps) {
  const thresholdConfigs = [
    {
      key: 'guest' as const,
      label: 'VMs & Containers',
      description: 'Delay before triggering alerts for virtual machines and containers',
      min: 0,
      max: 300,
      step: 5,
    },
    {
      key: 'node' as const,
      label: 'Proxmox Nodes',
      description: 'Delay before triggering alerts for Proxmox nodes',
      min: 0,
      max: 300,
      step: 5,
    },
    {
      key: 'storage' as const,
      label: 'Storage Devices',
      description: 'Delay before triggering alerts for storage devices',
      min: 0,
      max: 300,
      step: 5,
    },
    {
      key: 'pbs' as const,
      label: 'PBS Servers',
      description: 'Delay before triggering alerts for Proxmox Backup Server instances',
      min: 0,
      max: 300,
      step: 5,
    },
  ];

  return (
    <Card>
      <div class="space-y-4">
        <div>
          <SectionHeader
            title="Alert Delay Thresholds"
            size="md"
            class="mb-2"
          />
          <p class="text-sm text-gray-600 dark:text-gray-400">
            Configure how long a metric must remain above threshold before triggering an alert.
            This prevents alerts from being triggered by brief spikes.
          </p>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          {thresholdConfigs.map((config) => (
            <div class={formField}>
              <label class={labelClass()}>
                {config.label}
              </label>
              <div class="relative">
                <input
                  type="number"
                  min={config.min}
                  max={config.max}
                  step={config.step}
                  value={props.timeThresholds()[config.key]}
                  onInput={(e) => {
                    const value = parseInt(e.currentTarget.value);
                    if (!isNaN(value) && value >= config.min && value <= config.max) {
                      props.setTimeThresholds({
                        ...props.timeThresholds(),
                        [config.key]: value,
                      });
                      props.setHasUnsavedChanges(true);
                    }
                  }}
                  class={controlClass('pr-20')}
                />
                <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-500 dark:text-gray-400">
                  seconds
                </span>
              </div>
              <p class={formHelpText}>
                {config.description}
                {props.timeThresholds()[config.key] === 0 && (
                  <span class="ml-1 font-medium text-amber-600 dark:text-amber-400">
                    (Alerts trigger immediately)
                  </span>
                )}
              </p>
            </div>
          ))}
        </div>

        <div class="rounded-md bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 p-3">
          <div class="flex gap-2">
            <svg
              class="h-5 w-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <div class="text-sm text-blue-800 dark:text-blue-200">
              <p class="font-medium mb-1">How Alert Delays Work</p>
              <p>
                When a metric exceeds its threshold, Pulse waits for the configured delay period before triggering an alert.
                If the metric drops below the threshold during this waiting period, no alert is created.
                This helps reduce false alarms from temporary spikes.
              </p>
            </div>
          </div>
        </div>
      </div>
    </Card>
  );
}
