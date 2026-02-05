import { Component, For, Show } from 'solid-js';
import { HostNetworkInterface } from '@/types/api';

interface NetworkInterfacesCardProps {
  interfaces?: HostNetworkInterface[];
}

export const NetworkInterfacesCard: Component<NetworkInterfacesCardProps> = (props) => {
  if (!props.interfaces || props.interfaces.length === 0) return null;

  return (
    <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
      <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Network</div>
      <div class="max-h-[140px] overflow-y-auto custom-scrollbar space-y-2">
        <For each={props.interfaces}>
          {(iface) => (
            <div class="rounded border border-dashed border-gray-200 p-2 dark:border-gray-700 overflow-hidden">
              <div class="flex items-center gap-2 text-[11px] font-medium text-gray-700 dark:text-gray-200 min-w-0">
                <span class="truncate min-w-0">{iface.name}</span>
                <Show when={iface.mac}>
                  <span class="text-[9px] text-gray-400 dark:text-gray-500 font-normal truncate shrink-0 max-w-[100px]" title={iface.mac}>{iface.mac}</span>
                </Show>
              </div>
              <Show when={iface.addresses && iface.addresses.length > 0}>
                <div class="flex flex-wrap gap-1 mt-1">
                  <For each={iface.addresses}>
                    {(ip) => (
                      <span class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200 max-w-full truncate" title={ip}>
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
    </div>
  );
};
