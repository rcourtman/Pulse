import { Component, For, Show } from 'solid-js';
import type { HostRAIDArray } from '@/types/api';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  RAID_LAST_KNOWN_DEVICE_BADGE_CLASS,
  getRaidDeviceBadgeClass,
  getRaidLastKnownDeviceLabel,
  getRaidStateTextClass,
  getRaidStateVariant,
} from '@/utils/raidPresentation';

interface RaidCardProps {
  arrays?: HostRAIDArray[];
  title?: string;
  /**
   * Why the array states are retained rather than current, such as a host
   * agent past its reporting lease. The card keeps every state, since an
   * array that degraded before the machine went quiet is evidence, but reads
   * them as last known, without live status colour or rebuild progress.
   */
  lastKnownReason?: string;
}

export const RaidCard: Component<RaidCardProps> = (props) => {
  const lastKnownTitle = () => {
    const reason = props.lastKnownReason?.trim();
    return reason ? `Last known reading, not current: ${reason}` : undefined;
  };

  // Tracked, so a drawer opened before the first RAID report still shows it.
  return (
    <Show when={props.arrays?.length}>
      <InfoCardFrame>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
          {props.title || 'RAID'}
        </div>

        <div class="max-h-[160px] overflow-y-auto custom-scrollbar space-y-2">
          <For each={props.arrays}>
            {(array) => {
              const label = () =>
                (array.name || '').trim() || (array.device || '').trim() || 'RAID array';
              const variant = () => getRaidStateVariant(array.state);
              const stateText = () => (array.state || '').trim() || 'unknown';
              const levelText = () => (array.level || '').trim() || 'unknown';
              const rebuildPercent = () => array.rebuildPercent ?? 0;
              const rebuilding = () =>
                Number.isFinite(rebuildPercent()) && rebuildPercent() > 0 && rebuildPercent() < 100;

              return (
                <div
                  class="rounded-sm border border-dashed border-border p-2 overflow-hidden"
                  data-testid="raid-card-array"
                  data-raid-reading={lastKnownTitle() ? 'last-known' : 'current'}
                >
                  {/* The state drops to its own line rather than squeezing the
                    array name when both do not fit, as on a phone. */}
                  <div class="flex flex-wrap items-start justify-between gap-x-2 gap-y-1 min-w-0">
                    <div class="min-w-0">
                      <div
                        class="text-[11px] font-semibold text-base-content truncate"
                        title={label()}
                      >
                        {label()}
                      </div>
                      <div class="mt-0.5 text-[10px] text-muted truncate" title={levelText()}>
                        {levelText()}
                      </div>
                    </div>

                    <Show
                      when={!lastKnownTitle()}
                      fallback={
                        <div
                          class="ml-auto flex min-w-0 items-center gap-1.5"
                          title={lastKnownTitle()}
                          data-testid="raid-card-state"
                        >
                          <StatusDot
                            variant="muted"
                            size="xs"
                            ariaLabel={`Last known RAID state: ${stateText()}`}
                          />
                          <span class="text-[10px] font-medium text-muted">
                            {stateText()} (last known)
                          </span>
                        </div>
                      }
                    >
                      <div
                        class="ml-auto flex min-w-0 items-center gap-1.5"
                        title={stateText()}
                        data-testid="raid-card-state"
                      >
                        <StatusDot
                          variant={variant()}
                          size="xs"
                          ariaLabel={`RAID state: ${stateText()}`}
                        />
                        <span
                          class={`text-[10px] font-medium ${getRaidStateTextClass(array.state)}`}
                        >
                          {stateText()}
                        </span>
                      </div>
                    </Show>
                  </div>

                  <Show when={rebuilding()}>
                    <Show
                      when={!lastKnownTitle()}
                      fallback={
                        // A rebuild figure from the last report is not progress,
                        // so it reads in the past tense and drops the speed.
                        <div
                          class="mt-2 text-[10px] text-muted"
                          title={lastKnownTitle()}
                          data-testid="raid-card-rebuild"
                        >
                          Rebuild was at {Math.round(rebuildPercent())}%
                        </div>
                      }
                    >
                      <div class="mt-2 text-[10px] text-muted" data-testid="raid-card-rebuild">
                        Rebuild:{' '}
                        <span class="font-medium text-base-content">
                          {Math.round(rebuildPercent())}%
                        </span>
                        <Show when={array.rebuildSpeed}>
                          <span class="text-muted"> · </span>
                          <span class="font-medium text-base-content">{array.rebuildSpeed}</span>
                        </Show>
                      </div>
                    </Show>
                  </Show>

                  <Show when={array.devices && array.devices.length > 0}>
                    <div class="mt-2 flex flex-wrap gap-1">
                      <For each={array.devices}>
                        {(device) => (
                          <span
                            class={`inline-flex items-center rounded-sm border px-1.5 py-0.5 text-[10px] font-medium ${
                              lastKnownTitle()
                                ? RAID_LAST_KNOWN_DEVICE_BADGE_CLASS
                                : getRaidDeviceBadgeClass(device)
                            }`}
                            title={`slot ${device.slot} • ${device.state}`}
                            data-testid="raid-card-device"
                          >
                            {lastKnownTitle() ? getRaidLastKnownDeviceLabel(device) : device.device}
                          </span>
                        )}
                      </For>
                    </div>
                  </Show>
                </div>
              );
            }}
          </For>
        </div>
      </InfoCardFrame>
    </Show>
  );
};

export default RaidCard;
