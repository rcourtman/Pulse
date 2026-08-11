import { For, createSignal } from 'solid-js';

import {
  type ResourceLifecycleState,
  type ResourceMonitoringMode,
  type ResourceOperatorStateInput,
  getResourceOperatorState,
  setResourceOperatorState,
} from '@/api/resourceOperatorState';
import { notificationStore } from '@/stores/notifications';
import { describeResourceInventoryOwnership } from '@/utils/resourceMonitoringPolicy';

interface ResourceMonitoringPolicyActionProps {
  resourceId: string;
  resourceName: string;
  resourceType?: string;
  platformType?: string;
}

interface PolicyChoice {
  label: string;
  description: string;
  monitoringMode?: ResourceMonitoringMode;
  lifecycleState?: ResourceLifecycleState;
}

export function ResourceMonitoringPolicyAction(props: ResourceMonitoringPolicyActionProps) {
  const [saving, setSaving] = createSignal(false);
  let detailsRef: HTMLDetailsElement | undefined;
  const ownership = () =>
    describeResourceInventoryOwnership(props.resourceType, props.platformType);
  const choices = (): PolicyChoice[] => [
    {
      label: 'Normal monitoring',
      description: 'Resume all Alerts and Patrol attention.',
      monitoringMode: 'normal',
      lifecycleState: 'active',
    },
    {
      label: 'Expected offline',
      description: 'Hide availability noise but keep other monitoring active.',
      monitoringMode: 'expected_offline',
      lifecycleState: 'active',
    },
    {
      label: 'Mute all attention',
      description: 'Stop Alerts and Patrol while keeping the resource visible.',
      monitoringMode: 'muted',
      lifecycleState: 'active',
    },
    {
      label: 'Retire from monitoring',
      description: `${ownership().ownerLabel} keeps the inventory record. Pulse stops attention and automation.`,
      lifecycleState: 'retired',
    },
  ];

  const applyChoice = async (choice: PolicyChoice) => {
    if (saving()) return;
    setSaving(true);
    try {
      const current = await getResourceOperatorState(props.resourceId);
      const monitoringMode =
        choice.monitoringMode ??
        current?.monitoringMode ??
        (current?.intentionallyOffline ? 'expected_offline' : 'normal');
      const lifecycleState = choice.lifecycleState ?? current?.lifecycleState ?? 'active';
      const input: ResourceOperatorStateInput = {
        monitoringMode,
        lifecycleState,
        intentionallyOffline: monitoringMode === 'expected_offline',
        neverAutoRemediate: current?.neverAutoRemediate ?? false,
        autoRemediationPolicy: current?.autoRemediationPolicy ?? {
          enabled: false,
          capabilityNames: [],
        },
        maintenanceStartAt: current?.maintenanceStartAt,
        maintenanceEndAt: current?.maintenanceEndAt,
        maintenanceReason: current?.maintenanceReason,
        criticality: current?.criticality ?? '',
        note: current?.note,
      };
      await setResourceOperatorState(props.resourceId, input);
      if (detailsRef) detailsRef.open = false;
      notificationStore.success(`${choice.label} saved for ${props.resourceName}`);
    } catch (error) {
      notificationStore.error(
        error instanceof Error ? error.message : 'Failed to update resource monitoring',
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <details ref={detailsRef} class="relative" onClick={(event) => event.stopPropagation()}>
      <summary class="inline-flex min-h-9 cursor-pointer list-none items-center rounded-md border border-border bg-surface px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover">
        {saving() ? 'Saving…' : 'Monitoring'}
      </summary>
      <div class="absolute right-0 z-30 mt-1 w-72 rounded-md border border-border bg-surface p-1.5 shadow-lg">
        <p class="px-2 pb-1.5 text-[11px] text-muted">
          This changes Pulse policy only. {ownership().ownerLabel} remains the inventory owner.
        </p>
        <For each={choices()}>
          {(choice) => (
            <button
              type="button"
              disabled={saving()}
              class="block w-full rounded px-2 py-2 text-left hover:bg-surface-hover disabled:opacity-50"
              onClick={() => void applyChoice(choice)}
            >
              <span class="block text-xs font-medium text-base-content">{choice.label}</span>
              <span class="mt-0.5 block text-[11px] leading-tight text-muted">
                {choice.description}
              </span>
            </button>
          )}
        </For>
      </div>
    </details>
  );
}
