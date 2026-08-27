import { For, Show, createEffect, createSignal, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';

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

const MENU_WIDTH = 288;
const ESTIMATED_MENU_HEIGHT = 240;

export function ResourceMonitoringPolicyAction(props: ResourceMonitoringPolicyActionProps) {
  const [saving, setSaving] = createSignal(false);
  const [menuOpen, setMenuOpen] = createSignal(false);
  const [menuPosition, setMenuPosition] = createSignal({ left: 0, top: 0 });
  let triggerRef: HTMLButtonElement | undefined;
  let menuRef: HTMLDivElement | undefined;

  // The menu renders through a portal with fixed positioning so it can extend
  // past the alert card's scroll container: rendered in place, the last card's
  // menu is clipped at the container edge (#1737). The rendered height is
  // measured once mounted because the choice descriptions wrap, so a fixed
  // estimate under-clamps at the viewport bottom.
  const updateMenuPosition = () => {
    if (!triggerRef || typeof window === 'undefined') return;
    const rect = triggerRef.getBoundingClientRect();
    const menuHeight = menuRef?.offsetHeight || ESTIMATED_MENU_HEIGHT;
    setMenuPosition({
      left: Math.max(8, Math.min(window.innerWidth - MENU_WIDTH - 8, rect.right - MENU_WIDTH)),
      top: Math.max(8, Math.min(window.innerHeight - menuHeight - 8, rect.bottom + 4)),
    });
  };

  createEffect(() => {
    if (!menuOpen() || typeof window === 'undefined') return;
    updateMenuPosition();
    const closeOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Element && target.closest('[data-monitoring-policy-menu-root]')) {
        return;
      }
      setMenuOpen(false);
    };
    window.addEventListener('resize', updateMenuPosition);
    window.addEventListener('scroll', updateMenuPosition, true);
    document.addEventListener('pointerdown', closeOnOutsidePointer);
    onCleanup(() => {
      window.removeEventListener('resize', updateMenuPosition);
      window.removeEventListener('scroll', updateMenuPosition, true);
      document.removeEventListener('pointerdown', closeOnOutsidePointer);
    });
  });

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
        maintenanceRecurrence: current?.maintenanceRecurrence,
        maintenanceScope: current?.maintenanceScope ?? 'resource',
        maintenanceReason: current?.maintenanceReason,
        criticality: current?.criticality ?? '',
        note: current?.note,
      };
      await setResourceOperatorState(props.resourceId, input);
      setMenuOpen(false);
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
    <div
      class="relative"
      data-monitoring-policy-menu-root
      onClick={(event) => event.stopPropagation()}
    >
      <button
        type="button"
        ref={triggerRef}
        class="inline-flex min-h-9 cursor-pointer items-center rounded-md border border-border bg-surface px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
        onClick={(event) => {
          event.stopPropagation();
          updateMenuPosition();
          setMenuOpen((open) => !open);
        }}
      >
        {saving() ? 'Saving…' : 'Monitoring'}
      </button>
      <Show when={menuOpen()}>
        <Portal mount={document.body}>
          <div
            ref={menuRef}
            data-monitoring-policy-menu-root
            role="menu"
            class="fixed z-[9999] w-72 rounded-md border border-border bg-surface p-1.5 shadow-lg"
            style={{ left: `${menuPosition().left}px`, top: `${menuPosition().top}px` }}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => {
              event.stopPropagation();
              if (event.key === 'Escape') setMenuOpen(false);
            }}
          >
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
        </Portal>
      </Show>
    </div>
  );
}
