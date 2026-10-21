import { Component, For, Show } from 'solid-js';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { HostNetworkInterface } from '@/types/api';

interface NetworkInterfacesCardProps {
  interfaces?: HostNetworkInterface[];
  /** Qualifies only this list's producer; host configuration is independent. */
  lastKnownReason?: string;
}

export const NetworkInterfacesCard: Component<NetworkInterfacesCardProps> = (props) => {
  return (
    <Show when={props.interfaces?.length}>
      <InfoCardFrame>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
          Network
        </div>
        <Show when={props.lastKnownReason}>
          <p class="mb-2 whitespace-normal wrap-break-word text-xs text-muted" role="status">
            Last known interface details, not a current connection. {props.lastKnownReason}.
          </p>
        </Show>
        <div class="max-h-[140px] overflow-y-auto custom-scrollbar space-y-2">
          <For each={props.interfaces}>
            {(iface) => (
              <div class="rounded-sm border border-dashed border-border p-2 overflow-hidden">
                <div class="flex items-center gap-2 text-[11px] font-medium text-base-content min-w-0">
                  <span class="truncate min-w-0">{iface.name}</span>
                  <Show when={iface.mac}>
                    <span
                      class="text-[9px] text-muted font-normal truncate shrink-0 max-w-[100px]"
                      title={iface.mac}
                    >
                      {iface.mac}
                    </span>
                  </Show>
                </div>
                <Show when={iface.addresses && iface.addresses.length > 0}>
                  <div class="flex flex-wrap gap-1 mt-1">
                    <For each={iface.addresses}>
                      {(ip) => (
                        <span
                          class={`inline-block rounded-sm px-1.5 py-0.5 text-[10px] max-w-full truncate ${props.lastKnownReason ? 'bg-surface-hover text-muted' : 'bg-blue-100 text-blue-700 dark:bg-blue-900/25 dark:text-blue-200'}`}
                          title={ip}
                        >
                          {ip}
                        </span>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
            )}
          </For>
        </div>
      </InfoCardFrame>
    </Show>
  );
};
